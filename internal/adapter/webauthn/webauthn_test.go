// SPDX-License-Identifier: AGPL-3.0-only

package webauthn_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"testing"

	"github.com/best-friends/my/internal/adapter/webauthn"
	appidentity "github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/domain/identity"
)

const (
	rpID   = "social.example"
	origin = "https://social.example"
)

var b64 = base64.RawURLEncoding

// cbor encodes the few CBOR items (RFC 8949) that an attestation object and a COSE key use.
type cbor []byte

func (c cbor) head(major byte, n uint64) cbor {
	switch {
	case n < 24:
		return append(c, major<<5|byte(n))
	case n < 1<<8:
		return append(c, major<<5|24, byte(n))
	default:
		return binary.BigEndian.AppendUint16(append(c, major<<5|25), uint16(n)) //nolint:gosec // G115: test data under 64 KiB
	}
}

func (c cbor) int(n int) cbor {
	if n < 0 {
		return c.head(1, uint64(-1-n))
	}
	return c.head(0, uint64(n))
}

func (c cbor) bytes(b []byte) cbor { return append(c.head(2, uint64(len(b))), b...) }
func (c cbor) text(s string) cbor  { return append(c.head(3, uint64(len(s))), s...) }
func (c cbor) mapOf(n int) cbor    { return c.head(5, uint64(n)) } //nolint:gosec // G115: small maps

// authenticator is a software passkey: one P-256 key of one credential.
type authenticator struct {
	key    *ecdsa.PrivateKey
	credID []byte
	flags  byte
	count  uint32
}

// Authenticator data flags (WebAuthn §6.1).
const (
	flagUP = 0x01
	flagUV = 0x04
	flagBE = 0x08
	flagBS = 0x10
	flagAT = 0x40
)

func newAuthenticator(t *testing.T) *authenticator {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id := make([]byte, 16)
	_, _ = rand.Read(id)
	return &authenticator{key: key, credID: id, flags: flagUP | flagUV | flagBE | flagBS}
}

func (a *authenticator) coseKey() []byte {
	pub, err := a.key.PublicKey.Bytes() // 0x04 || X || Y
	if err != nil {
		panic(err)
	}
	// kty EC2, alg ES256, crv P-256, x, y (RFC 9053 §7.1).
	return cbor{}.mapOf(5).int(1).int(2).int(3).int(-7).int(-1).int(1).int(-2).bytes(pub[1:33]).int(-3).bytes(pub[33:])
}

func (a *authenticator) authData(flags byte, attested bool) []byte {
	rp := sha256.Sum256([]byte(rpID))
	d := append(rp[:], flags)
	a.count++
	d = binary.BigEndian.AppendUint32(d, a.count)
	if attested {
		d = append(d, make([]byte, 16)...)                          // AAGUID
		d = binary.BigEndian.AppendUint16(d, uint16(len(a.credID))) //nolint:gosec // G115: 16 bytes
		d = append(d, a.credID...)
		d = append(d, a.coseKey()...)
	}
	return d
}

func clientData(typ, challenge, from string) []byte {
	b, _ := json.Marshal(map[string]any{"type": typ, "challenge": challenge, "origin": from, "crossOrigin": false})
	return b
}

// challengeOf reads the challenge of the options.
func challengeOf(t *testing.T, options []byte) string {
	t.Helper()
	var o struct {
		Challenge string `json:"challenge"`
	}
	if err := json.Unmarshal(options, &o); err != nil || o.Challenge == "" {
		t.Fatalf("options %s have no challenge: %v", options, err)
	}
	return o.Challenge
}

// register answers a registration's options as the browser's PublicKeyCredential.toJSON does.
func (a *authenticator) register(t *testing.T, options []byte, from string) []byte {
	t.Helper()
	cd := clientData("webauthn.create", challengeOf(t, options), from)
	att := cbor{}.mapOf(3).text("fmt").text("none").text("attStmt").mapOf(0).text("authData").bytes(a.authData(a.flags|flagAT, true))
	b, _ := json.Marshal(map[string]any{
		"id": b64.EncodeToString(a.credID), "rawId": b64.EncodeToString(a.credID), "type": "public-key",
		"response": map[string]any{
			"clientDataJSON": b64.EncodeToString(cd), "attestationObject": b64.EncodeToString(att), "transports": []string{"internal"},
		},
		"clientExtensionResults": map[string]any{}, "authenticatorAttachment": "platform",
	})
	return b
}

// signIn answers a sign-in's options for the user handle.
func (a *authenticator) signIn(t *testing.T, options []byte, handle []byte, flags byte, from string) []byte {
	t.Helper()
	cd := clientData("webauthn.get", challengeOf(t, options), from)
	ad := a.authData(flags, false)
	hash := sha256.Sum256(cd)
	digest := sha256.Sum256(append(append([]byte{}, ad...), hash[:]...))
	sig, err := ecdsa.SignASN1(rand.Reader, a.key, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(map[string]any{
		"id": b64.EncodeToString(a.credID), "rawId": b64.EncodeToString(a.credID), "type": "public-key",
		"response": map[string]any{
			"clientDataJSON": b64.EncodeToString(cd), "authenticatorData": b64.EncodeToString(ad),
			"signature": b64.EncodeToString(sig), "userHandle": b64.EncodeToString(handle),
		},
		"clientExtensionResults": map[string]any{}, "authenticatorAttachment": "platform",
	})
	return b
}

func newCeremonies(t *testing.T) *webauthn.Ceremonies {
	t.Helper()
	c, err := webauthn.New(webauthn.Options{RPID: rpID, DisplayName: rpID, Origins: []string{origin}})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

var alice = appidentity.PasskeyUser{Handle: []byte("019a0000-0000-7000-8000-000000000001"), Name: "alice@social.example"}

// registered registers a's credential for alice and returns it as the identity module stores it.
func registered(t *testing.T, c *webauthn.Ceremonies, a *authenticator) identity.PasskeyCredential {
	t.Helper()
	options, state, err := c.BeginRegistration(alice, nil)
	if err != nil {
		t.Fatal(err)
	}
	created, err := c.FinishRegistration(alice, state, a.register(t, options, origin))
	if err != nil {
		t.Fatalf("FinishRegistration = %v", err)
	}
	return identity.PasskeyCredential{
		ID: "p1", UserID: string(alice.Handle), CredentialID: created.CredentialID, PublicKey: created.PublicKey,
		BackupEligible: created.BackupEligible, Name: "phone",
	}
}

func TestRegistrationOptionsAskForADiscoverableVerifiedCredential(t *testing.T) {
	c := newCeremonies(t)
	excluded := []byte{1, 2, 3, 4}
	options, _, err := c.BeginRegistration(alice, [][]byte{excluded})
	if err != nil {
		t.Fatal(err)
	}
	var o struct {
		RP   struct{ ID, Name string }
		User struct {
			ID   string
			Name string
		}
		AuthenticatorSelection struct {
			ResidentKey      string
			UserVerification string
		}
		Attestation        string
		ExcludeCredentials []struct{ ID, Type string }
		Timeout            int
	}
	if err := json.Unmarshal(options, &o); err != nil {
		t.Fatal(err)
	}
	if o.RP.ID != rpID || o.User.ID != b64.EncodeToString(alice.Handle) || o.User.Name != alice.Name ||
		o.AuthenticatorSelection.ResidentKey != "required" || o.AuthenticatorSelection.UserVerification != "required" ||
		o.Attestation != "none" || len(o.ExcludeCredentials) != 1 || o.ExcludeCredentials[0].ID != b64.EncodeToString(excluded) ||
		o.Timeout != int(appidentity.PasskeyCeremonyTTL.Milliseconds()) {
		t.Errorf("registration options = %s", options)
	}
}

func TestRegistersAndSignsIn(t *testing.T) {
	c := newCeremonies(t)
	a := newAuthenticator(t)
	stored := registered(t, c, a)
	if !stored.BackupEligible || len(stored.PublicKey) == 0 || string(stored.CredentialID) != string(a.credID) {
		t.Fatalf("registered credential = %+v", stored)
	}

	options, state, err := c.BeginSignIn()
	if err != nil {
		t.Fatal(err)
	}
	var o struct {
		RPID             string `json:"rpId"`
		UserVerification string `json:"userVerification"`
		AllowCredentials []any  `json:"allowCredentials"`
	}
	if err := json.Unmarshal(options, &o); err != nil || o.RPID != rpID || o.UserVerification != "required" || len(o.AllowCredentials) != 0 {
		t.Fatalf("sign-in options = %s, %v", options, err)
	}
	var looked []byte
	got, err := c.FinishSignIn(state, a.signIn(t, options, alice.Handle, flagUP|flagUV|flagBE|flagBS, origin), func(id []byte) (identity.PasskeyCredential, error) {
		looked = id
		return stored, nil
	})
	if err != nil || got.ID != stored.ID || string(looked) != string(a.credID) {
		t.Fatalf("FinishSignIn = %+v, %v (looked up %x)", got, err, looked)
	}
}

func TestSignInRejectsWhatDoesNotVerify(t *testing.T) {
	c := newCeremonies(t)
	a := newAuthenticator(t)
	stored := registered(t, c, a)
	find := func([]byte) (identity.PasskeyCredential, error) { return stored, nil }
	for _, tc := range []struct {
		name   string
		answer func(options []byte) []byte
		find   func([]byte) (identity.PasskeyCredential, error)
	}{
		{"another origin", func(o []byte) []byte {
			return a.signIn(t, o, alice.Handle, flagUP|flagUV|flagBE|flagBS, "https://evil.example")
		}, find},
		{"no user verification", func(o []byte) []byte { return a.signIn(t, o, alice.Handle, flagUP|flagBE|flagBS, origin) }, find},
		{"another user's handle", func(o []byte) []byte {
			return a.signIn(t, o, []byte("someone-else"), flagUP|flagUV|flagBE|flagBS, origin)
		}, find},
		{"another key", func(o []byte) []byte {
			other := newAuthenticator(t)
			other.credID = a.credID
			return other.signIn(t, o, alice.Handle, flagUP|flagUV|flagBE|flagBS, origin)
		}, find},
		{"a changed backup eligibility", func(o []byte) []byte { return a.signIn(t, o, alice.Handle, flagUP|flagUV, origin) }, find},
		{"an unknown credential", func(o []byte) []byte {
			return a.signIn(t, o, alice.Handle, flagUP|flagUV|flagBE|flagBS, origin)
		}, func([]byte) (identity.PasskeyCredential, error) {
			return identity.PasskeyCredential{}, appidentity.ErrNotFound
		}},
		{"not JSON", func([]byte) []byte { return []byte("{") }, find},
	} {
		t.Run(tc.name, func(t *testing.T) {
			options, state, err := c.BeginSignIn()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := c.FinishSignIn(state, tc.answer(options), tc.find); !errors.Is(err, appidentity.ErrInvalidPasskey) {
				t.Errorf("FinishSignIn = %v, want ErrInvalidPasskey", err)
			}
		})
	}

	// Another sign-in's challenge.
	options, _, err := c.BeginSignIn()
	if err != nil {
		t.Fatal(err)
	}
	_, state, err := c.BeginSignIn()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.FinishSignIn(state, a.signIn(t, options, alice.Handle, flagUP|flagUV|flagBE|flagBS, origin), find); !errors.Is(err, appidentity.ErrInvalidPasskey) {
		t.Errorf("FinishSignIn with another challenge = %v, want ErrInvalidPasskey", err)
	}

	// An error of the lookup is not the response's fault.
	options, state, err = c.BeginSignIn()
	if err != nil {
		t.Fatal(err)
	}
	broken := errors.New("database down")
	_, err = c.FinishSignIn(state, a.signIn(t, options, alice.Handle, flagUP|flagUV|flagBE|flagBS, origin), func([]byte) (identity.PasskeyCredential, error) {
		return identity.PasskeyCredential{}, broken
	})
	if !errors.Is(err, broken) || errors.Is(err, appidentity.ErrInvalidPasskey) {
		t.Errorf("FinishSignIn with a failing lookup = %v, want %v", err, broken)
	}
}

func TestRegistrationRejectsWhatDoesNotVerify(t *testing.T) {
	c := newCeremonies(t)
	for _, tc := range []struct {
		name   string
		answer func(a *authenticator, options []byte) []byte
	}{
		{"another origin", func(a *authenticator, o []byte) []byte { return a.register(t, o, "https://evil.example") }},
		{"no user verification", func(a *authenticator, o []byte) []byte {
			a.flags = flagUP
			return a.register(t, o, origin)
		}},
		{"not JSON", func(*authenticator, []byte) []byte { return []byte("[]") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			options, state, err := c.BeginRegistration(alice, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := c.FinishRegistration(alice, state, tc.answer(newAuthenticator(t), options)); !errors.Is(err, appidentity.ErrInvalidPasskey) {
				t.Errorf("FinishRegistration = %v, want ErrInvalidPasskey", err)
			}
		})
	}
	// A registration begun for another user.
	options, state, err := c.BeginRegistration(appidentity.PasskeyUser{Handle: []byte("bob"), Name: "bob@social.example"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.FinishRegistration(alice, state, newAuthenticator(t).register(t, options, origin)); !errors.Is(err, appidentity.ErrInvalidPasskey) {
		t.Errorf("FinishRegistration for another user = %v, want ErrInvalidPasskey", err)
	}
}

// A development server or the E2E run on localhost over plain HTTP, which browsers treat as
// a secure context, with the web client on another port.
func TestRunsOnLocalhost(t *testing.T) {
	c, err := webauthn.New(webauthn.Options{RPID: "localhost", DisplayName: "localhost:8080", Origins: []string{"http://localhost:8080", "http://localhost:5173"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.BeginSignIn(); err != nil {
		t.Fatal(err)
	}
}

func TestNewRejectsAnInvalidRelyingParty(t *testing.T) {
	if _, err := webauthn.New(webauthn.Options{RPID: "", DisplayName: "x", Origins: []string{origin}}); err == nil {
		t.Error("New without a relying party ID succeeded")
	}
}
