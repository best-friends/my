// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"golang.org/x/term"

	appaccount "github.com/best-friends/my/internal/app/account"
	appidentity "github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/platform"
	"github.com/best-friends/my/internal/platform/config"
)

// runAdmin implements `my admin <subcommand>`. stdin supplies the password.
func runAdmin(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	switch {
	case len(args) == 0:
		adminUsage(stderr)
		return 2
	case args[0] == "-h" || args[0] == "-help" || args[0] == "--help":
		adminUsage(stderr)
		return 0
	case args[0] != "create":
		_, _ = fmt.Fprintf(stderr, "my admin: unknown subcommand %q\n\n", args[0])
		adminUsage(stderr)
		return 2
	}
	fs := flag.NewFlagSet("my admin create", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { adminUsage(stderr) }
	username := fs.String("username", "", "username of the new admin (required)")
	email := fs.String("email", "", "email address of the new admin (required)")
	file := fs.String("config", "", "TOML config file (default: $"+config.EnvConfigFile+")")
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() > 0 {
		_, _ = fmt.Fprintf(stderr, "my admin create: unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	if *username == "" || *email == "" {
		_, _ = fmt.Fprintln(stderr, "my admin create: -username and -email are required")
		return 2
	}

	cfg, _, err := config.Load(config.Source{File: *file})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	password, err := readPassword(stdin, stderr)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "my admin create: %v\n", err)
		return 1
	}

	ctx := context.Background()
	cli, err := platform.OpenAdminCLI(ctx, cfg)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "my admin create: %v\n", err)
		return 1
	}
	defer cli.Close()
	reg, err := cli.Identity.CreateAdmin(ctx, appidentity.CreateAdminInput{Username: *username, Email: *email, Password: password})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "my admin create: %s\n", describeAdminError(err, *username, *email))
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "created admin @%s@%s (user %s, account %s)\n", *username, cfg.AccountDomain, reg.User.ID, reg.AccountID)
	return 0
}

// describeAdminError turns a CreateAdmin failure into a message for the operator. None of the
// errors it describes carries the password.
func describeAdminError(err error, username, email string) string {
	switch {
	case errors.Is(err, appaccount.ErrUsernameTaken):
		return fmt.Sprintf("username %q is already taken; nothing was changed", username)
	case errors.Is(err, appidentity.ErrEmailTaken):
		return fmt.Sprintf("email address %q is already in use; nothing was changed", email)
	default:
		return err.Error()
	}
}

// maxPasswordInput bounds what readPassword reads from a pipe; it is far above the longest
// valid password (identity.MaxPasswordLength characters of up to 4 bytes each).
const maxPasswordInput = 4096

// readPassword reads the new password: from an interactive prompt without echo, asked twice,
// when stdin is a terminal, and otherwise from the first line of stdin.
func readPassword(stdin io.Reader, stderr io.Writer) (string, error) {
	if f, ok := stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		return promptPassword(int(f.Fd()), stderr)
	}
	line, err := bufio.NewReaderSize(io.LimitReader(stdin, maxPasswordInput+1), maxPasswordInput+1).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read the password from standard input: %w", err)
	}
	if len(line) > maxPasswordInput {
		return "", errors.New("the password on standard input is too long")
	}
	line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	if line == "" {
		return "", errors.New("no password on standard input; pipe it in or run in a terminal to be prompted")
	}
	return line, nil
}

func promptPassword(fd int, stderr io.Writer) (string, error) {
	// Ctrl-C while echo is off would leave the terminal without echo; restore it first.
	if state, err := term.GetState(fd); err == nil {
		interrupts := make(chan os.Signal, 1)
		signal.Notify(interrupts, os.Interrupt, syscall.SIGTERM)
		done := make(chan struct{})
		defer func() { signal.Stop(interrupts); close(done) }()
		go func() {
			select {
			case <-interrupts:
				_ = term.Restore(fd, state)
				_, _ = fmt.Fprintln(stderr)
				os.Exit(130)
			case <-done:
			}
		}()
	}
	read := func(prompt string) (string, error) {
		_, _ = fmt.Fprint(stderr, prompt)
		b, err := term.ReadPassword(fd)
		_, _ = fmt.Fprintln(stderr)
		if err != nil {
			return "", fmt.Errorf("read the password: %w", err)
		}
		return string(b), nil
	}
	first, err := read("Password: ")
	if err != nil {
		return "", err
	}
	second, err := read("Confirm password: ")
	if err != nil {
		return "", err
	}
	if first != second {
		return "", errors.New("the passwords do not match")
	}
	return first, nil
}

func adminUsage(w io.Writer) {
	_, _ = fmt.Fprint(w, `Usage: my admin create -username <name> -email <address> [-config <file>]

Creates an administrator: a user with the password, the email address as primary
address, a local account with the username, and the built-in admin role, in one
transaction. The database schema must already be applied with Atlas.

The password is prompted for twice without echo when standard input is a
terminal; otherwise the first line of standard input is the password. It is
never printed or logged.

If the username or the email address is already in use, the command fails with
exit code 1 and changes nothing; it never modifies an existing user. A failed
run writes nothing, so it is safe to run again.
`)
}
