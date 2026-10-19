// SPDX-License-Identifier: AGPL-3.0-only

// Package authpages serves the server-rendered sign-in, sign-out, registration, email
// verification, and OAuth authorization (consent) screens (ADR-0030): plain HTML forms from
// html/template that work without JavaScript, worded from the server message catalog and styled
// by one stylesheet that the web client's build provides. With a CAPTCHA (Options.HumanCheck),
// the sign-in and registration forms also load the service's widget (humancheck.go).
//
// A sign-in or sign-up starts a browser session carried by the __Host-my_session cookie
// (Secure, HttpOnly, SameSite=Lax, Path=/) with a new token every time (ADR-0019 §4). Every POST
// passes net/http.CrossOriginProtection and must carry the anti-forgery token of the
// __Host-my_csrf cookie in its csrf_token field, for browsers that send neither Sec-Fetch-Site
// nor Origin (ADR-0019 §4). Failed sign-ins answer with one message whether or not the account
// exists.
//
// A signed-in user whose account cannot be used yet (a registration waiting for approval or
// rejected, an email address waiting for confirmation, or moderation actions that keep the
// account out of the app, identity.Access) sees a page that says so, and lists those actions
// with a form to appeal each, instead of continuing to where they were going.
//
// The authorization endpoint (RFC 6749 §3.1) asks a signed-in user to consent to a third-party
// client and answers the client by a 303 redirect to its registered redirect URI with the code
// or the error, the state, and the issuer (RFC 9207). First-party clients skip the consent.
// Without a session, it sends the browser to the sign-in form and back; a user whose account
// cannot be used yet gets the status page and no code.
package authpages

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"errors"
	"html/template"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	appidentity "github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/domain/identity"
	"github.com/best-friends/my/internal/i18n"
)

// Paths of the screens.
const (
	PathSignIn  = "/auth/sign_in"
	PathSignOut = "/auth/sign_out"
	PathSignUp  = "/auth/sign_up"
	// PathVerifyEmail is the page that verification emails link to.
	PathVerifyEmail = appidentity.PathVerifyEmail
	// PathResendVerification sends a signed-in user a new verification email.
	PathResendVerification = "/auth/verify_email/resend"
	// PathAppeal takes the status page's appeals of the moderation actions that keep the user
	// out of the app (moderation.go).
	PathAppeal = "/auth/appeal"
	// PathAuthorize is the OAuth authorization endpoint.
	PathAuthorize = "/oauth/authorize"
)

// Cookie names. The __Host- prefix makes browsers accept the cookies only when Secure, without
// a Domain, and with Path=/, so that no other host can set or overwrite them.
const (
	CookieSession = "__Host-my_session"
	CookieCSRF    = "__Host-my_csrf"
)

// contentSecurityPolicy allows the pages nothing but their own stylesheet, images, and forms.
// Browsers check form-action on the redirects that follow a form, so it lists the origins a
// form may end up at: this site, Options.FormActionOrigins, and the consent page's redirect URI.
// A page with the CAPTCHA widget adds the service's sources in widget (humancheck.go); the
// others pass none.
func contentSecurityPolicy(formAction []string, widget kernel.HumanCheckCSP) string {
	var b strings.Builder
	b.WriteString("default-src 'none'")
	if len(widget.ScriptSrc) > 0 {
		b.WriteString("; script-src " + strings.Join(widget.ScriptSrc, " "))
	}
	if len(widget.FrameSrc) > 0 {
		b.WriteString("; frame-src " + strings.Join(widget.FrameSrc, " "))
	}
	if len(widget.ConnectSrc) > 0 {
		b.WriteString("; connect-src " + strings.Join(widget.ConnectSrc, " "))
	}
	b.WriteString("; style-src " + strings.Join(append([]string{"'self'"}, widget.StyleSrc...), " "))
	b.WriteString("; img-src 'self'; form-action " + strings.Join(append([]string{"'self'"}, formAction...), " "))
	b.WriteString("; frame-ancestors 'none'; base-uri 'none'")
	return b.String()
}

// maxFormBytes bounds a form body; the largest are the registration form, whose reason field
// holds up to identity.MaxRegistrationReasonLength characters, and the appeal form, whose text
// holds up to moderation.MaxAppealText.
const maxFormBytes = 32 << 10

// maxReturnTo bounds the return_to path.
const maxReturnTo = 2048

// Identity is what the screens need from the identity module; *identity.Service implements it.
type Identity interface {
	SignIn(ctx context.Context, in appidentity.SignInInput) (appidentity.SignedIn, error)
	Authenticate(ctx context.Context, token string) (identity.Session, error)
	SignOut(ctx context.Context, token string) error
	SignUp(ctx context.Context, in appidentity.SignUpInput) (appidentity.SignedUp, error)
	RegistrationMode(ctx context.Context) (identity.RegistrationMode, error)
	UserAccess(ctx context.Context, userID string) (appidentity.Access, error)
	VerifyEmail(ctx context.Context, token string) (string, error)
	ResendVerificationEmail(ctx context.Context, userID string) error
}

// OAuth is what the authorization endpoint needs from the identity module; *identity.Service
// implements it.
type OAuth interface {
	ValidateAuthorization(ctx context.Context, req appidentity.AuthorizationRequest) (appidentity.Authorization, error)
	IssueAuthorizationCode(ctx context.Context, a appidentity.Authorization, userID string) (string, error)
}

// Options configure a Handler.
type Options struct {
	Identity Identity
	// OAuth serves the authorization endpoint; nil leaves it out.
	OAuth OAuth
	// Issuer is the authorization server's identifier, sent as the iss parameter of the
	// authorization response (RFC 9207), e.g. https://social.example.
	Issuer string
	// FormActionOrigins are the origins besides this site that a sign-in may be redirected to,
	// such as the web client's origin for its callback.
	FormActionOrigins []string
	// Locales resolves the locale of the screens (ADR-0009 §2).
	Locales *i18n.Resolver
	// InstanceName returns the name of the instance for readers of locale: its display name, or
	// its domain when it has none. The pages show it above their card and in their title; nil
	// leaves it out.
	InstanceName func(ctx context.Context, locale string) string
	// Stylesheet is the URL of the pages' stylesheet, the web build's server pages entry
	// (web.Handler.Stylesheet); empty leaves the pages unstyled.
	Stylesheet string
	// Logger logs unexpected errors; nil means slog.Default().
	Logger *slog.Logger
	// Moderation lists the actions that keep a signed-in user out of the app on the status
	// page; nil lists none. UserLocales words the status page in the user's saved locale; nil
	// words it in the request's.
	Moderation  Moderation
	UserLocales UserLocales
	// Documents serves the pages of the terms of service and the privacy policy (documents.go);
	// nil leaves them unpublished.
	Documents Documents
	// HumanCheck is the CAPTCHA widget of the sign-in and registration forms (humancheck.go);
	// nil, as with the captcha setting none, renders none.
	HumanCheck *kernel.HumanCheckWidget
}

// Handler serves the screens.
type Handler struct {
	o     Options
	pages map[string]*template.Template
	cop   *http.CrossOriginProtection
}

//go:embed templates/*.html
var templateFS embed.FS

// Page templates, each rendered inside templates/layout.html.
const (
	pageSignIn      = "sign_in"
	pageSignedIn    = "signed_in"
	pageError       = "error"
	pageSignUp      = "sign_up"
	pageStatus      = "status"
	pageVerifyEmail = "verify_email"
	pageVerified    = "verified"
	pageAuthorize   = "authorize"
	pageDocument    = "document"
)

// New returns a Handler on o.
func New(o Options) *Handler {
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	h := &Handler{o: o, pages: map[string]*template.Template{}, cop: http.NewCrossOriginProtection()}
	for _, name := range []string{pageSignIn, pageSignedIn, pageError, pageSignUp, pageStatus, pageVerifyEmail, pageVerified, pageAuthorize, pageDocument} {
		h.pages[name] = template.Must(template.ParseFS(templateFS, "templates/layout.html", "templates/human_check.html", "templates/"+name+".html"))
	}
	h.cop.SetDenyHandler(http.HandlerFunc(h.crossOrigin))
	return h
}

// Router registers handlers for http.ServeMux patterns.
type Router interface {
	Handle(pattern string, h http.Handler)
}

// Register registers the screens on r.
func (h *Handler) Register(r Router) {
	r.Handle("GET "+PathSignIn, http.HandlerFunc(h.signInPage))
	r.Handle("POST "+PathSignIn, h.cop.Handler(http.HandlerFunc(h.signIn)))
	r.Handle("POST "+PathSignOut, h.cop.Handler(http.HandlerFunc(h.signOut)))
	r.Handle("GET "+PathSignUp, http.HandlerFunc(h.signUpPage))
	r.Handle("POST "+PathSignUp, h.cop.Handler(http.HandlerFunc(h.signUp)))
	r.Handle("GET "+PathVerifyEmail, http.HandlerFunc(h.verifyEmailPage))
	r.Handle("POST "+PathVerifyEmail, h.cop.Handler(http.HandlerFunc(h.verifyEmail)))
	r.Handle("POST "+PathResendVerification, h.cop.Handler(http.HandlerFunc(h.resendVerification)))
	r.Handle("POST "+PathAppeal, h.cop.Handler(http.HandlerFunc(h.appeal)))
	for i, d := range documentPages {
		r.Handle("GET "+d.path, h.documentPage(i))
	}
	if h.o.OAuth != nil {
		r.Handle("GET "+PathAuthorize, http.HandlerFunc(h.authorizePage))
		r.Handle("POST "+PathAuthorize, h.cop.Handler(http.HandlerFunc(h.authorize)))
	}
}

// page is the data of a template.
type page struct {
	L         *i18n.Localizer
	Lang, Dir string
	// Title is the page's title; DocumentTitle adds the instance's name to it.
	Title string
	// InstanceName names the instance in the page's locale; "" leaves it out.
	InstanceName string
	Stylesheet   string
	CSRFToken    string
	ReturnTo     string
	Email        string
	// Error is an alert about the whole form; Notice reports success.
	Error  string
	Notice string

	// CanSignUp shows the link to the registration page.
	CanSignUp bool
	// SignUp is the registration form.
	SignUp *signUpForm
	// Status is the state of an account that cannot be used yet.
	Status *statusView
	// Token is the email verification token of the confirmation form.
	Token string
	// Consent is the authorization request on the consent page.
	Consent *consent
	// Document is the document on its page.
	Document *documentView
	// HumanCheck is the CAPTCHA widget of the form; the page's Content-Security-Policy admits
	// the service when it is set.
	HumanCheck *humanCheckView
	// Documents are the links to the published documents, in the footer of every page and on
	// the registration form; render fills them.
	Documents []documentLink
	// cache is the Cache-Control of the page; empty means no-store.
	cache string
	// formAction lists origins the page's forms may be redirected to besides
	// Options.FormActionOrigins.
	formAction []string
}

// DocumentTitle is the <title> of the page: its title and the instance's name.
func (p page) DocumentTitle() string {
	if p.InstanceName == "" {
		return p.Title
	}
	return p.L.ServerAuthDocumentTitle(p.InstanceName, p.Title)
}

func (h *Handler) newPage(r *http.Request, title func(*i18n.Localizer) string) page {
	l := h.o.Locales.RequestLocalizer(r)
	return page{L: l, Lang: l.Locale(), Dir: l.Direction(), Title: title(l), Stylesheet: h.o.Stylesheet}
}

func (h *Handler) render(w http.ResponseWriter, r *http.Request, status int, name string, p page) {
	hd := w.Header()
	hd.Set("Content-Type", "text/html; charset=utf-8")
	hd.Set("Content-Language", p.Lang)
	var widget kernel.HumanCheckCSP
	if p.HumanCheck != nil {
		widget = h.humanCheckCSP()
	}
	hd.Set("Content-Security-Policy", contentSecurityPolicy(append(slices.Clone(h.o.FormActionOrigins), p.formAction...), widget))
	cache := p.cache
	if cache == "" {
		cache = "no-store"
	}
	hd.Set("Cache-Control", cache)
	hd.Add("Vary", "Accept-Language, Cookie")
	if h.o.InstanceName != nil {
		p.InstanceName = h.o.InstanceName(r.Context(), p.Lang)
	}
	p.Documents = h.documentLinks(r, p.L)
	w.WriteHeader(status)
	if err := h.pages[name].ExecuteTemplate(w, "layout", p); err != nil {
		h.o.Logger.ErrorContext(r.Context(), "render auth page", slog.String("page", name), slog.Any("error", err))
	}
}

func (h *Handler) renderError(w http.ResponseWriter, r *http.Request, status int, message func(*i18n.Localizer) string) {
	p := h.newPage(r, (*i18n.Localizer).ServerAuthErrorTitle)
	p.Error = message(p.L)
	h.render(w, r, status, pageError, p)
}

func (h *Handler) internalError(w http.ResponseWriter, r *http.Request, err error) {
	h.o.Logger.ErrorContext(r.Context(), "auth page failed", slog.Any("error", err))
	h.renderError(w, r, http.StatusInternalServerError, (*i18n.Localizer).ServerErrorsInternal)
}

// crossOrigin answers a POST that CrossOriginProtection rejected.
func (h *Handler) crossOrigin(w http.ResponseWriter, r *http.Request) {
	h.renderError(w, r, http.StatusForbidden, (*i18n.Localizer).ServerAuthCrossOrigin)
}

// signInPage shows the sign-in form; when the browser has a session, it shows the signed-in
// page, or the status page of an account that cannot be used yet.
func (h *Handler) signInPage(w http.ResponseWriter, r *http.Request) {
	returnTo := safeReturnTo(r.URL.Query().Get("return_to"))
	session, err := h.o.Identity.Authenticate(r.Context(), cookieValue(r, CookieSession))
	switch {
	case err == nil:
		access, err := h.o.Identity.UserAccess(r.Context(), session.UserID)
		if err != nil {
			h.internalError(w, r, err)
			return
		}
		if !access.Allowed() {
			h.renderStatus(w, r, http.StatusOK, session.UserID, access, nil, nil)
			return
		}
		h.renderSignedIn(w, r, returnTo)
	case errors.Is(err, appidentity.ErrInvalidSession):
		h.renderSignIn(w, r, http.StatusOK, returnTo, "", nil)
	default:
		h.internalError(w, r, err)
	}
}

func (h *Handler) renderSignedIn(w http.ResponseWriter, r *http.Request, returnTo string) {
	p := h.newPage(r, (*i18n.Localizer).ServerAuthSignedInTitle)
	p.CSRFToken = h.csrfToken(w, r)
	p.ReturnTo = returnTo
	if p.ReturnTo == "" {
		p.ReturnTo = "/"
	}
	h.render(w, r, http.StatusOK, pageSignedIn, p)
}

func (h *Handler) renderSignIn(w http.ResponseWriter, r *http.Request, status int, returnTo, email string, message func(*i18n.Localizer) string) {
	p := h.newPage(r, (*i18n.Localizer).ServerAuthSignInTitle)
	p.CSRFToken = h.csrfToken(w, r)
	p.ReturnTo, p.Email = returnTo, email
	if message != nil {
		p.Error = message(p.L)
	}
	mode, err := h.o.Identity.RegistrationMode(r.Context())
	if err != nil {
		h.o.Logger.WarnContext(r.Context(), "read the registration mode", slog.Any("error", err))
	}
	p.CanSignUp = err == nil && mode != identity.RegistrationClosed
	p.HumanCheck = h.humanCheck(w, r, p.L, kernel.HumanCheckActionSignIn)
	h.render(w, r, status, pageSignIn, p)
}

// signIn handles the sign-in form.
func (h *Handler) signIn(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := r.ParseForm(); err != nil {
		h.renderError(w, r, http.StatusBadRequest, (*i18n.Localizer).ServerErrorsBadRequest)
		return
	}
	returnTo := safeReturnTo(r.PostForm.Get("return_to"))
	email := strings.TrimSpace(r.PostForm.Get("email"))
	if !h.validCSRF(r) {
		h.renderSignIn(w, r, http.StatusForbidden, returnTo, email, (*i18n.Localizer).ServerAuthFormExpired)
		return
	}
	password := r.PostForm.Get("password")
	if email == "" || password == "" {
		h.renderSignIn(w, r, http.StatusUnprocessableEntity, returnTo, email, (*i18n.Localizer).ServerAuthSignInMissing)
		return
	}
	signedIn, err := h.o.Identity.SignIn(r.Context(), appidentity.SignInInput{
		Email:           email,
		Password:        password,
		IP:              clientIP(r),
		UserAgent:       r.UserAgent(),
		CurrentToken:    cookieValue(r, CookieSession),
		HumanCheckToken: h.humanCheckToken(r),
	})
	var limited *kernel.RateLimitedError
	switch {
	case errors.As(err, &limited):
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(limited.RetryAfter.Seconds()))))
		minutes := int(math.Ceil(limited.RetryAfter.Minutes()))
		h.renderSignIn(w, r, http.StatusTooManyRequests, returnTo, email, func(l *i18n.Localizer) string {
			return l.ServerAuthSignInRateLimited(minutes)
		})
		return
	case errors.Is(err, appidentity.ErrInvalidCredentials):
		h.renderSignIn(w, r, http.StatusUnprocessableEntity, returnTo, email, (*i18n.Localizer).ServerAuthSignInFailed)
		return
	case errors.Is(err, kernel.ErrHumanCheckFailed):
		h.renderSignIn(w, r, http.StatusUnprocessableEntity, returnTo, email, (*i18n.Localizer).ServerAuthHumanCheckFailed)
		return
	case err != nil:
		h.internalError(w, r, err)
		return
	}
	setSessionCookie(w, signedIn)
	access, err := h.o.Identity.UserAccess(r.Context(), signedIn.Session.UserID)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	switch {
	case !access.Allowed():
		returnTo = PathSignIn // the page that says why the account cannot be used yet
	case returnTo == "":
		returnTo = "/"
	}
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, returnTo, http.StatusSeeOther) //nolint:gosec // G710: safeReturnTo admits only paths on this site
}

// setSessionCookie gives the browser the session of s.
func setSessionCookie(w http.ResponseWriter, s appidentity.SignedIn) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieSession,
		Value:    s.Token,
		Path:     "/",
		Expires:  s.Session.ExpiresAt,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// signOut ends the browser's session and returns to the sign-in form.
func (h *Handler) signOut(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := r.ParseForm(); err != nil {
		h.renderError(w, r, http.StatusBadRequest, (*i18n.Localizer).ServerErrorsBadRequest)
		return
	}
	if !h.validCSRF(r) {
		h.renderError(w, r, http.StatusForbidden, (*i18n.Localizer).ServerAuthFormExpired)
		return
	}
	if err := h.o.Identity.SignOut(r.Context(), cookieValue(r, CookieSession)); err != nil {
		h.internalError(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: CookieSession, Value: "", Path: "/", MaxAge: -1, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, PathSignIn, http.StatusSeeOther)
}

// csrfToken returns the browser's anti-forgery token, issuing a new one in the CSRF cookie when
// the browser has none (the double-submit pattern; the __Host- prefix keeps other hosts from
// planting the cookie).
func (h *Handler) csrfToken(w http.ResponseWriter, r *http.Request) string {
	if t := cookieValue(r, CookieCSRF); validTokenSyntax(t) {
		return t
	}
	b := make([]byte, 32)
	_, _ = rand.Read(b) // crypto/rand.Read never returns an error
	t := base64.RawURLEncoding.EncodeToString(b)
	http.SetCookie(w, &http.Cookie{
		Name: CookieCSRF, Value: t, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	return t
}

// validCSRF reports whether the form's csrf_token matches the CSRF cookie.
func (h *Handler) validCSRF(r *http.Request) bool {
	cookie := cookieValue(r, CookieCSRF)
	form := r.PostForm.Get("csrf_token")
	return validTokenSyntax(cookie) && subtle.ConstantTimeCompare([]byte(cookie), []byte(form)) == 1
}

// validTokenSyntax accepts what csrfToken issues: 43 characters of unpadded base64url.
func validTokenSyntax(t string) bool {
	if len(t) != 43 {
		return false
	}
	_, err := base64.RawURLEncoding.DecodeString(t)
	return err == nil
}

func cookieValue(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}

// clientIP returns the address of the connection's peer. Forwarding headers are not trusted:
// reading them needs the trusted proxy setting of ADR-0019 §5.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// safeReturnTo returns s when it is a path on this site to go back to after signing in, and ""
// otherwise, so that the form cannot be turned into an open redirect.
func safeReturnTo(s string) string {
	if s == "" || len(s) > maxReturnTo || s[0] != '/' || strings.HasPrefix(s, "//") || strings.ContainsAny(s, "\\\x00\r\n\t") {
		return ""
	}
	u, err := url.Parse(s)
	if err != nil || u.Scheme != "" || u.Host != "" || u.User != nil {
		return ""
	}
	return s
}
