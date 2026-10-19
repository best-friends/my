// SPDX-License-Identifier: AGPL-3.0-only

package clientapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/queue"
	"github.com/best-friends/my/internal/app/stream"
	"github.com/best-friends/my/internal/i18n"
)

// BasePath is the path prefix of every client API endpoint, the server URL of api/openapi.yaml.
const BasePath = "/api/v1"

// Options configure a Server.
type Options struct {
	// Host is the domain that serves the instance.
	Host string
	// AccountDomain is the domain of local handles (@alice@AccountDomain) when it differs
	// from Host; "" means Host.
	AccountDomain string
	// Origin is the origin at which browsers reach the server's own pages, for the links that
	// responses carry to them (Account.url of a local account, Status.url of a local status,
	// StatusTag.url); "" means https://Host. The ActivityPub ids stay under https://Host.
	Origin string
	// SoftwareVersion is the server version.
	SoftwareVersion string
	// SourceURL is where users get the server's source code (ADR-0011).
	SourceURL string
	// Instance reads the instance settings and texts; nil reads every setting as its default
	// and no text.
	Instance InstanceInfo
	// SendsEmail tells whether the instance sends email (the setting mail is not none), for
	// Instance.sends_email.
	SendsEmail bool

	// Authenticator authenticates bearer access tokens; nil accepts none.
	Authenticator Authenticator
	// Accounts reads accounts.
	Accounts Accounts
	// RemoteProfiles reads the profile pages of remote accounts for Account.url; nil links
	// remote accounts to their uri.
	RemoteProfiles RemoteProfiles
	// Resolver fetches unknown remote accounts for lookupAccount's resolve; nil resolves none
	// (lookups find only stored accounts).
	Resolver AccountResolver
	// Profiles changes the viewer's profile.
	Profiles Profiles
	// Posting reads what the viewer's restrictions let them post, for CredentialAccount; nil
	// presents every account as unrestricted.
	Posting PostingPolicy
	// Preferences reads and changes the viewer's preferences.
	Preferences UserPreferences
	// Media runs the media use cases and presents profile images; nil serves no media
	// operation (they fail with 500) and presents no images.
	Media MediaFiles
	// Graph reads and changes follows, blocks, and mutes.
	Graph Graph
	// Notifications runs the use cases of the viewer's notifications.
	Notifications Notifications
	// Statuses posts, edits, and deletes statuses; StatusReader presents them; Interactions
	// boosts, favourites, bookmarks, and pins them (the statuses operations).
	Statuses     Statuses
	StatusReader StatusReader
	Interactions Interactions
	// Threads starts fetching the remote parts of a thread that a viewer opens (getStatusContext);
	// nil fetches none.
	Threads ThreadFetcher
	// Outboxes starts fetching the recent posts of a remote account whose profile a logged-in
	// viewer opens (the first page of listAccountStatuses); nil fetches none.
	Outboxes OutboxFetcher
	// Timelines reads the timelines, and TimelineReader the statuses they list.
	Timelines      Timelines
	TimelineReader TimelineReader
	// TimelineLanguages returns the languages that a user's timelines show, none for every
	// language (FR-208); nil shows every language.
	TimelineLanguages func(ctx context.Context, userID string) ([]string, error)
	// Streams opens the subscriptions of the streaming endpoint; nil answers it with 503.
	// HomeEntries tells the streams whether a home timeline lists a new entry; nil streams no
	// home timeline entries. StreamHeartbeat overrides DefaultStreamHeartbeat when positive.
	Streams         stream.Subscriber
	HomeEntries     HomeEntries
	StreamHeartbeat time.Duration
	// Permissions reads the permissions of the viewer's roles, which the admin operations need
	// (admin.go); nil grants none. InstanceHealth reads and clears the health of remote servers,
	// and DeadJobs lists and requeues the dead jobs, for the admin operations.
	Permissions    Permissions
	InstanceHealth InstanceHealth
	DeadJobs       queue.DeadJobs
	// Moderation runs the moderators' use cases on domain actions and reports for the admin
	// operations (admin_moderation.go); UserAccounts presents the moderator of a report by
	// account, and nil leaves it out.
	Moderation   Moderation
	UserAccounts UserAccounts
	// AccountModeration runs the moderators' use cases on the actions on accounts for the admin
	// operations (admin_account_actions.go).
	AccountModeration AccountModeration
	// AccountAdmin runs the account management of the admin operations, and AccountReports
	// reads the reports of an account for its details (admin_accounts.go).
	AccountAdmin   AccountAdmin
	AccountReports AccountReports
	// OwnModeration reads the actions on the viewer's own account, for GET /moderation/actions
	// and the moderation notices (moderation.go); nil leaves the action out of the notices.
	OwnModeration OwnModeration
	// OwnAppeals files the viewer's appeals of the actions on their account and reads the action
	// of an appeal_decided notice (appeals.go); nil refuses appeals with 404 and leaves the action
	// out of those notices. AppealModeration runs the moderators' use cases on the appeals
	// (admin_appeals.go) and counts the pending ones for GET /admin/counts.
	OwnAppeals       OwnAppeals
	AppealModeration AppealModeration
	// ProfileRestrictions hides the accounts whose profiles moderation hides (profile_view)
	// from everyone but themselves; nil hides none. Moderators see them through the admin
	// operations.
	ProfileRestrictions ProfileRestrictions
	// AuditLog lists the audit log for the admin operations (admin_audit.go).
	AuditLog AuditLog
	// Rules runs the use cases of the instance rules: the admin operations (admin_rules.go)
	// and the rules of GET /instance, which lists none when it is nil.
	Rules InstanceRules
	// InstanceTexts reads and writes the instance texts for the admin operations
	// (admin_instance_texts.go).
	InstanceTexts InstanceTexts
	// TermsPath and PrivacyPolicyPath are the paths, on Origin, of the public pages of the terms
	// of service and the privacy policy, for Instance.terms_url and Instance.privacy_policy_url;
	// empty leaves the link out.
	TermsPath, PrivacyPolicyPath string
	// Roles lists, grants, and revokes the roles of users for the admin operations
	// (admin_roles.go).
	Roles Roles
	// Reports files the reports of local users (reports.go).
	Reports Reports
	// FiledReports reads the reports that the viewer filed (listOwnReports) and that their
	// report_resolved notifications tell of; nil leaves the report out of the notifications.
	FiledReports FiledReports
	// Severances reads the follows that the viewer's relationships_severed notifications tell
	// of; nil leaves them out of the notifications.
	Severances Severances
	// ReportCounts and RegistrationCounts count what awaits the moderators (admin_counts.go);
	// nil leaves the count out.
	ReportCounts       ReportCounts
	RegistrationCounts RegistrationCounts
	// Registrations runs the registration review (admin_registrations.go).
	Registrations Registrations
	// SignupBlocks runs the sign-up block list (admin_signup_blocks.go).
	SignupBlocks SignupBlocks
	// RateLimiter applies AnonymousLimit and AccountLimit; nil applies no limit.
	RateLimiter *kernel.RateLimiter
	// AnonymousLimit and AccountLimit override the package's limits of the same names when
	// their Max is set.
	AnonymousLimit kernel.Limit
	AccountLimit   kernel.Limit
	// ClientIP returns the client IP of a request for its rate limit; nil means the
	// connection's peer.
	ClientIP func(r *http.Request) string

	// Locales resolves the locale of problem documents (ADR-0009 §2). Its UserLocale step
	// finds the authenticated user with ViewerFrom.
	Locales *i18n.Resolver
	// RequestID returns the ID of the request in ctx, echoed in problem documents; nil omits it.
	RequestID func(ctx context.Context) string
	// Logger logs unexpected errors; nil means slog.Default().
	Logger *slog.Logger
}

// Server implements the generated StrictServerInterface by calling application use cases.
type Server struct {
	o Options
}

var _ StrictServerInterface = (*Server)(nil)

// New returns a Server on o.
func New(o Options) *Server {
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.RequestID == nil {
		o.RequestID = func(context.Context) string { return "" }
	}
	if o.AnonymousLimit.Max == 0 {
		o.AnonymousLimit = AnonymousLimit
	}
	if o.AccountLimit.Max == 0 {
		o.AccountLimit = AccountLimit
	}
	if o.ClientIP == nil {
		o.ClientIP = remoteIP
	}
	if o.Authenticator == nil {
		o.Authenticator = noTokens{}
	}
	return &Server{o: o}
}

// Router registers handlers for http.ServeMux patterns.
type Router interface {
	Handle(pattern string, h http.Handler)
}

// Register registers every operation of the spec on r under BasePath, the generated ones and
// the hand-written streaming endpoint (streaming.go), behind authentication and rate limiting, with the security of each operation enforced (auth.go). Requests that
// match no operation are left to r, which answers them with its own 404 or 405 problem.
func (s *Server) Register(r Router) {
	strict := NewStrictHandlerWithOptions(s, []StrictMiddlewareFunc{s.rejectInvalidBodyText}, StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  s.requestError,
		ResponseErrorHandlerFunc: s.responseError,
	})
	HandlerWithOptions(strict, StdHTTPServerOptions{
		BaseURL:          BasePath,
		BaseRouter:       routerMux{r: r, wrap: s.guard},
		Middlewares:      []MiddlewareFunc{s.localize},
		ErrorHandlerFunc: s.requestError,
	})
	s.registerStreaming(r)
}

// routerMux adapts a Router to the ServeMux that the generated code registers on, wrapping
// every operation's handler by its pattern.
type routerMux struct {
	r    Router
	wrap func(pattern string, h http.Handler) http.Handler
}

func (m routerMux) HandleFunc(pattern string, h func(http.ResponseWriter, *http.Request)) {
	m.r.Handle(pattern, m.wrap(pattern, http.HandlerFunc(h)))
}

// ServeHTTP is never called: the Router serves the registered handlers.
func (routerMux) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNotFound)
}
