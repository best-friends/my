// SPDX-License-Identifier: AGPL-3.0-only

package identity

import (
	"context"
	"errors"
	"fmt"
)

// ErrWrongPassword reports a password that does not match the user's current one, given to
// confirm an operation of a signed-in user.
var ErrWrongPassword = errors.New("identity: the password is wrong")

// DeleteOwnAccount deletes the local account that a user logs in as at the user's request
// (FR-008), once password matches the user's current password: the account is deleted at once
// (account.Service.DeleteBySelf), which signs the user out everywhere, sends the Delete of the
// actor to the remote servers, and leaves the data to the erasure (data-model §8). A wrong
// password fails with ErrWrongPassword and changes nothing.
func (s *Service) DeleteOwnAccount(ctx context.Context, userID, password string) error {
	cred, err := s.d.Repository.FindCurrentPasswordCredential(ctx, userID)
	if errors.Is(err, ErrNotFound) {
		return ErrWrongPassword
	}
	if err != nil {
		return err
	}
	ok, _, err := s.d.Passwords.Verify(ctx, password, cred.Hash)
	if err != nil {
		return fmt.Errorf("identity: verify the password of user %s: %w", userID, err)
	}
	if !ok {
		return ErrWrongPassword
	}
	st, err := s.d.Accounts.UserLifecycle(ctx, userID)
	if err != nil {
		return err
	}
	return s.d.Accounts.DeleteBySelf(ctx, st.AccountID)
}
