// SPDX-License-Identifier: AGPL-3.0-only

// Package webauthn implements identity.PasskeyCeremonies with go-webauthn
// (github.com/go-webauthn/webauthn, BSD-3-Clause, pure Go): the relying party's side of the
// registration and the sign-in of passkeys (FR-004), following W3C Web Authentication Level 3
// §7. The library stays inside this package (.golangci.yml, depguard).
//
// Every ceremony asks for a discoverable credential and user verification, and no attestation:
// the instance trusts any authenticator that the user trusts, as the password it stands in for
// trusts any password manager. The options are the JSON that the browser's
// PublicKeyCredential.parseCreationOptionsFromJSON and parseRequestOptionsFromJSON read, the
// state the library's SessionData as JSON, which the identity module keeps sealed until the
// response comes and whose lifetime it enforces with its own clock.
package webauthn

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	appidentity "github.com/mycroblog/my/internal/app/identity"
	"github.com/mycroblog/my/internal/domain/identity"
)

// Options configure Ceremonies.
type Options struct {
	// RPID is the relying party ID: the host name of the instance, without a port.
	RPID string
	// DisplayName names the instance in the browser's prompts.
	DisplayName string
	// Origins are the origins that may run the ceremonies (scheme://host[:port]): the server's
	// and the web client's.
	Origins []string
	// Timeout is how long the browser waits for the user, sent as the options' timeout; zero
	// means appidentity.PasskeyCeremonyTTL.
	Timeout time.Duration
}

// Ceremonies implements identity.PasskeyCeremonies.
type Ceremonies struct {
	w *webauthn.WebAuthn
}

var _ appidentity.PasskeyCeremonies = (*Ceremonies)(nil)

// New returns Ceremonies on o. It fails when the relying party ID or an origin is invalid.
func New(o Options) (*Ceremonies, error) {
	timeout := o.Timeout
	if timeout <= 0 {
		timeout = appidentity.PasskeyCeremonyTTL
	}
	// The library accepts a configuration without one and fails each ceremony instead.
	if err := protocol.ValidateRPID(o.RPID); err != nil {
		return nil, fmt.Errorf("webauthn: the relying party ID %q: %w", o.RPID, err)
	}
	limit := webauthn.TimeoutConfig{Timeout: timeout, TimeoutUVD: timeout}
	w, err := webauthn.New(&webauthn.Config{
		RPID:                  o.RPID,
		RPDisplayName:         o.DisplayName,
		RPOrigins:             o.Origins,
		AttestationPreference: protocol.PreferNoAttestation,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			RequireResidentKey: protocol.ResidentKeyRequired(),
			ResidentKey:        protocol.ResidentKeyRequirementRequired,
			UserVerification:   protocol.VerificationRequired,
		},
		Timeouts: webauthn.TimeoutsConfig{Login: limit, Registration: limit},
		// Password managers may answer extensions that were not asked for; nothing reads them.
		ExtensionsUnsolicitedOutputPolicy: protocol.UnsolicitedOutputPolicyIgnore,
	})
	if err != nil {
		return nil, fmt.Errorf("webauthn: %w", err)
	}
	return &Ceremonies{w: w}, nil
}

// user is a user of the library: its handle, its name, and the credential of a sign-in.
type user struct {
	handle      []byte
	name        string
	credentials []webauthn.Credential
}

func (u user) WebAuthnID() []byte                         { return u.handle }
func (u user) WebAuthnName() string                       { return u.name }
func (u user) WebAuthnDisplayName() string                { return u.name }
func (u user) WebAuthnCredentials() []webauthn.Credential { return u.credentials }

// invalid wraps an error of the library that a response caused.
func invalid(err error) error {
	var perr *protocol.Error
	if errors.As(err, &perr) && perr.DevInfo != "" {
		return fmt.Errorf("%w: %w (%s)", appidentity.ErrInvalidPasskey, err, perr.DevInfo)
	}
	return fmt.Errorf("%w: %w", appidentity.ErrInvalidPasskey, err)
}

// BeginRegistration implements identity.PasskeyCeremonies.
func (c *Ceremonies) BeginRegistration(u appidentity.PasskeyUser, exclude [][]byte) (options, state []byte, err error) {
	excluded := make([]protocol.CredentialDescriptor, len(exclude))
	for i, id := range exclude {
		excluded[i] = protocol.CredentialDescriptor{Type: protocol.PublicKeyCredentialType, CredentialID: id}
	}
	creation, session, err := c.w.BeginRegistration(user{handle: u.Handle, name: u.Name}, webauthn.WithExclusions(excluded))
	if err != nil {
		return nil, nil, fmt.Errorf("webauthn: begin a registration: %w", err)
	}
	return marshal(creation.Response, session)
}

// FinishRegistration implements identity.PasskeyCeremonies.
func (c *Ceremonies) FinishRegistration(u appidentity.PasskeyUser, state, response []byte) (appidentity.NewPasskey, error) {
	var session webauthn.SessionData
	if err := json.Unmarshal(state, &session); err != nil {
		return appidentity.NewPasskey{}, fmt.Errorf("webauthn: read the state of a registration: %w", err)
	}
	parsed, err := protocol.ParseCredentialCreationResponseBytes(response)
	if err != nil {
		return appidentity.NewPasskey{}, invalid(err)
	}
	cred, err := c.w.CreateCredential(user{handle: u.Handle, name: u.Name}, session, parsed)
	if err != nil {
		return appidentity.NewPasskey{}, invalid(err)
	}
	return appidentity.NewPasskey{CredentialID: cred.ID, PublicKey: cred.PublicKey, BackupEligible: cred.Flags.BackupEligible}, nil
}

// BeginSignIn implements identity.PasskeyCeremonies.
func (c *Ceremonies) BeginSignIn() (options, state []byte, err error) {
	assertion, session, err := c.w.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return nil, nil, fmt.Errorf("webauthn: begin a sign-in: %w", err)
	}
	return marshal(assertion.Response, session)
}

// FinishSignIn implements identity.PasskeyCeremonies.
func (c *Ceremonies) FinishSignIn(state, response []byte, find func(credentialID []byte) (identity.PasskeyCredential, error)) (identity.PasskeyCredential, error) {
	var session webauthn.SessionData
	if err := json.Unmarshal(state, &session); err != nil {
		return identity.PasskeyCredential{}, fmt.Errorf("webauthn: read the state of a sign-in: %w", err)
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(response)
	if err != nil {
		return identity.PasskeyCredential{}, invalid(err)
	}
	stored, err := find(parsed.RawID)
	if errors.Is(err, appidentity.ErrNotFound) {
		return identity.PasskeyCredential{}, invalid(err)
	}
	if err != nil {
		return identity.PasskeyCredential{}, err
	}
	owner := user{handle: []byte(stored.UserID), credentials: []webauthn.Credential{{
		ID:        stored.CredentialID,
		PublicKey: stored.PublicKey,
		Flags:     webauthn.CredentialFlags{BackupEligible: stored.BackupEligible},
	}}}
	_, _, err = c.w.ValidatePasskeyLogin(func(_, _ []byte) (webauthn.User, error) { return owner, nil }, session, parsed)
	if err != nil {
		return identity.PasskeyCredential{}, invalid(err)
	}
	return stored, nil
}

func marshal(options any, session *webauthn.SessionData) ([]byte, []byte, error) {
	o, err := json.Marshal(options)
	if err != nil {
		return nil, nil, fmt.Errorf("webauthn: encode the options: %w", err)
	}
	s, err := json.Marshal(session)
	if err != nil {
		return nil, nil, fmt.Errorf("webauthn: encode the state: %w", err)
	}
	return o, s, nil
}
