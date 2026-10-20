// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	appcontent "github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/domain/content"
	"github.com/best-friends/my/internal/platform"
)

// stringList is a flag that may be given more than once.
type stringList []string

func (l *stringList) String() string { return strings.Join(*l, ",") }

func (l *stringList) Set(v string) error {
	*l = append(*l, v)
	return nil
}

// runAdminEmoji implements `my admin emoji <subcommand>`: the custom emoji (ADR-0051 §5). The
// CLI acts as no user: its operations are audited as the CLI's, and the images it registers are
// uploads of the instance actor.
func runAdminEmoji(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		adminEmojiUsage(stderr)
		return 2
	}
	sub, code := dispatch("admin emoji", args, []string{"create", "list", "enable", "disable", "delete"}, stderr, adminEmojiUsage)
	if code >= 0 {
		return code
	}
	var (
		shortcode, category, domain, before string
		aliases                             stringList
		local, remote, hidden               bool
		limit                               = 50
	)
	nargs := 1
	if sub == "list" {
		nargs = 0
	}
	c, code := parseSubcommand("my admin emoji "+sub, args[1:], nargs, stderr, adminEmojiUsage, func(fs *flag.FlagSet) {
		switch sub {
		case "create":
			fs.StringVar(&shortcode, "shortcode", "", "2 to 30 letters, digits, and underscores (required)")
			fs.StringVar(&category, "category", "", "the category of the picker")
			fs.Var(&aliases, "alias", "another name that the picker finds the emoji by; repeat for more")
			fs.BoolVar(&hidden, "hidden", false, "keep the emoji out of the picker; texts can still use it")
		case "list":
			fs.BoolVar(&local, "local", false, "show only the emoji of this instance")
			fs.BoolVar(&remote, "remote", false, "show only the emoji of other servers")
			fs.StringVar(&domain, "domain", "", "show only the emoji of this remote domain")
			fs.StringVar(&shortcode, "shortcode", "", "show only the emoji whose shortcode contains this text")
			fs.StringVar(&before, "before", "", "show the emoji older than this emoji ID")
			fs.IntVar(&limit, "limit", limit, "number of emoji to show, newest first")
		}
	})
	if code >= 0 {
		return code
	}
	switch {
	case sub == "create" && shortcode == "":
		_, _ = fmt.Fprintf(stderr, "%s: -shortcode is required\n", c.name)
		return 2
	case local && remote:
		_, _ = fmt.Fprintf(stderr, "%s: -local and -remote cannot be given together\n", c.name)
		return 2
	case limit < 1:
		_, _ = fmt.Fprintf(stderr, "%s: -limit must be at least 1\n", c.name)
		return 2
	}
	ctx := context.Background()
	app, code := c.open(ctx, stderr)
	if code >= 0 {
		return code
	}
	defer func() { _ = app.Close() }()
	var err error
	switch sub {
	case "create":
		var e appcontent.AdminCustomEmoji
		e, err = createEmoji(ctx, app, c.fs.Arg(0), appcontent.CreateCustomEmojiInput{
			Shortcode: shortcode, Category: category, Aliases: aliases, VisibleInPicker: !hidden,
		})
		if err == nil {
			_, _ = fmt.Fprintf(stdout, "created :%s: (emoji %s)\n", e.Emoji.Shortcode, e.Emoji.ID)
		}
	case "list":
		q := appcontent.CustomEmojiQuery{Domain: domain, Shortcode: shortcode, Before: before}
		switch {
		case local:
			q.Locality = appcontent.LocalEmoji
		case remote:
			q.Locality = appcontent.RemoteEmoji
		}
		var list []appcontent.AdminCustomEmoji
		if list, err = app.CustomEmojis.List(ctx, q, limit); err == nil {
			err = printEmojis(stdout, list)
		}
	case "enable", "disable":
		enabled := sub == "enable"
		var e appcontent.AdminCustomEmoji
		e, err = app.CustomEmojis.Update(ctx, c.fs.Arg(0), appcontent.CustomEmojiUpdate{Enabled: &enabled})
		if err == nil {
			_, _ = fmt.Fprintf(stdout, "%sd %s (emoji %s)\n", sub, e.Emoji.Label(), e.Emoji.ID)
		}
	case "delete":
		var e appcontent.AdminCustomEmoji
		if e, err = app.CustomEmojis.Get(ctx, c.fs.Arg(0)); err == nil {
			err = app.CustomEmojis.Delete(ctx, e.Emoji.ID, "")
		}
		if err == nil {
			_, _ = fmt.Fprintf(stdout, "deleted %s (emoji %s)\n", e.Emoji.Label(), e.Emoji.ID)
		}
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%s: %s\n", c.name, describeEmojiError(err, c.fs.Arg(0)))
		return 1
	}
	return 0
}

// createEmoji registers the image file at path as a local emoji: it uploads it as the instance
// actor's, processes it at once for the emoji's variants, and registers it.
func createEmoji(ctx context.Context, app *platform.App, path string, in appcontent.CreateCustomEmojiInput) (appcontent.AdminCustomEmoji, error) {
	if !content.ValidLocalShortcode(in.Shortcode) {
		return appcontent.AdminCustomEmoji{}, &content.CustomEmojiError{Reason: content.EmojiInvalidShortcode, Value: in.Shortcode}
	}
	f, err := os.Open(path) //nolint:gosec // the operator names the image to register
	if err != nil {
		return appcontent.AdminCustomEmoji{}, err
	}
	defer func() { _ = f.Close() }()
	if st, err := f.Stat(); err == nil && st.Size() > app.CustomEmojis.MaxBytes() {
		return appcontent.AdminCustomEmoji{}, &content.CustomEmojiError{Reason: content.EmojiTooLarge, Value: path}
	}
	owner, err := app.Accounts.EnsureInstanceActor(ctx)
	if err != nil {
		return appcontent.AdminCustomEmoji{}, err
	}
	m, err := app.Media.Upload(ctx, appcontent.UploadInput{
		OwnerAccountID: owner.ID, Content: f, Purpose: content.PurposeEmoji,
	})
	if err != nil {
		return appcontent.AdminCustomEmoji{}, err
	}
	if res := app.Media.Process(ctx, m.File.ID, content.PurposeEmoji); !res.IsDone() {
		return appcontent.AdminCustomEmoji{}, fmt.Errorf("process the image: %v", res)
	}
	in.MediaID, in.OwnerAccountID = m.File.ID, owner.ID
	return app.CustomEmojis.Create(ctx, in)
}

// describeEmojiError explains a failure to the operator.
func describeEmojiError(err error, arg string) string {
	var ce *content.CustomEmojiError
	switch {
	case errors.As(err, &ce):
		return map[content.CustomEmojiReason]string{
			content.EmojiInvalidShortcode:  "a shortcode is 2 to 30 letters (A to Z), digits, and underscores",
			content.EmojiShortcodeTaken:    "a custom emoji of this instance already has the shortcode",
			content.EmojiInvalidAlias:      "an alias is 2 to 30 letters (A to Z), digits, and underscores, and an emoji has at most 10",
			content.EmojiInvalidCategory:   "a category is at most 64 characters on one line",
			content.EmojiUnsupportedFormat: "the image must be a PNG, a GIF, or a WebP",
			content.EmojiTooLarge:          "the image is larger than custom_emoji_max_bytes",
			content.EmojiTooBig:            "the image must be at most 1,024 pixels on each side",
			content.EmojiRemote:            "this can be done only with the custom emoji of this instance",
		}[ce.Reason] + "; nothing was changed"
	case errors.Is(err, appcontent.ErrNotFound):
		return fmt.Sprintf("no custom emoji has the ID %q", arg)
	}
	return err.Error()
}

func printEmojis(w io.Writer, list []appcontent.AdminCustomEmoji) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tSHORTCODE\tDOMAIN\tSTATE\tPICKER\tCATEGORY\tALIASES\tUPDATED")
	for _, e := range list {
		domain, state, picker, category, aliases := e.Emoji.Domain, "enabled", "shown", e.Settings.Category, strings.Join(e.Aliases, ",")
		if domain == "" {
			domain = "-"
		}
		if !e.Settings.Enabled {
			state = "disabled"
		}
		if !e.Settings.VisibleInPicker {
			picker = "hidden"
		}
		if !e.Emoji.Local() {
			picker = "-"
		}
		if category == "" {
			category = "-"
		}
		if aliases == "" {
			aliases = "-"
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", e.Emoji.ID, e.Emoji.Shortcode, domain, state, picker,
			oneLine(category), aliases, e.Image.RevisedAt.UTC().Format(time.RFC3339))
	}
	return tw.Flush()
}

func adminEmojiUsage(w io.Writer) {
	_, _ = fmt.Fprint(w, `Usage:
  my admin emoji create -shortcode <name> [-category <text>] [-alias <name>]... [-hidden]
                        [-config <file>] <image file>
  my admin emoji list [-local | -remote] [-domain <domain>] [-shortcode <text>] [-before <id>]
                      [-limit <n>] [-config <file>]
  my admin emoji enable|disable [-config <file>] <emoji id>
  my admin emoji delete [-config <file>] <emoji id>

The custom emoji of this instance (ADR-0051), which posts and profiles use as
:shortcode:, and those of other servers that posts and profiles brought in.

create registers a local emoji with an image: a PNG, a GIF, or a WebP of at most
custom_emoji_max_bytes (256 KiB by default) and 1,024 pixels on each side. The
image is re-encoded without metadata into variants of up to 256 pixels; an APNG
shows only its first frame. GoToSocial does not fetch emoji larger than 100 KiB
by default. -hidden keeps the emoji out of the picker.

disable shows the emoji nowhere (its :shortcode: stays text) until enable;
remote emoji can only be enabled and disabled. delete deletes a local emoji with
its images for good; the posts that used it show its :shortcode: as text.

Every operation is recorded in the audit log as the CLI's. The admin screen and
the admin API (/api/v1/admin/custom_emojis) do the same and more.
`)
}
