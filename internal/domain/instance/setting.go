// SPDX-License-Identifier: AGPL-3.0-only

package instance

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

// ErrUnknownKey reports a setting or text key that the code does not define.
var ErrUnknownKey = errors.New("instance: unknown key")

// ErrInvalidValue reports a setting value or text body that its key rejects.
var ErrInvalidValue = errors.New("instance: invalid value")

// SettingKey names an instance setting (data-model §4.9, instance_setting_revisions.key).
// Every key holds one scalar; a compound setting is split into several keys.
type SettingKey string

// Setting keys. Adding one means adding its Definition below and its value to the CHECK
// constraint of instance_setting_revisions.key in schema/<dialect>.sql.
const (
	SettingRegistrationMode         SettingKey = "registration_mode"
	SettingMaxStatusLength          SettingKey = "max_status_length"
	SettingDefaultLocale            SettingKey = "default_locale"
	SettingLocalTimelineEnabled     SettingKey = "local_timeline_enabled"
	SettingFederatedTimelineEnabled SettingKey = "federated_timeline_enabled"
	SettingPreviewImage             SettingKey = "preview_image"
	SettingRemoteMediaPolicy        SettingKey = "remote_media_policy"
	SettingRemoteMediaCacheTTL      SettingKey = "remote_media_cache_ttl"
	SettingAuthorizedFetch          SettingKey = "authorized_fetch"
	// SettingDomainActionsVisibility and SettingDomainActionsRationaleVisibility decide who
	// may read the public list of domain actions and their public comments (FR-718).
	SettingDomainActionsVisibility          SettingKey = "domain_actions_visibility"
	SettingDomainActionsRationaleVisibility SettingKey = "domain_actions_rationale_visibility"
	// SettingAppealWindow is how many days after an action on an account the account may
	// appeal it (FR-707, ADR-0038).
	SettingAppealWindow SettingKey = "appeal_window"
)

// RegistrationMode is the value of SettingRegistrationMode (FR-001).
type RegistrationMode string

// Registration modes.
const (
	RegistrationOpen     RegistrationMode = "open"     // anyone may sign up
	RegistrationApproval RegistrationMode = "approval" // sign-ups wait for a moderator's approval
	RegistrationInvite   RegistrationMode = "invite"   // sign-up needs an invite
	RegistrationClosed   RegistrationMode = "closed"   // nobody may sign up
)

// RemoteMediaPolicy is the value of SettingRemoteMediaPolicy (FR-409, ADR-0005): which remote
// media files the instance keeps a copy of.
type RemoteMediaPolicy string

// Remote media policies.
const (
	RemoteMediaNone    RemoteMediaPolicy = "none"    // keep nothing; the media proxy relays every file
	RemoteMediaAvatars RemoteMediaPolicy = "avatars" // keep avatars and headers for remote_media_cache_ttl
	RemoteMediaAll     RemoteMediaPolicy = "all"     // keep every remote file for remote_media_cache_ttl
)

// Audience is the value of the settings that decide who may read something public about the
// instance (SettingDomainActionsVisibility, SettingDomainActionsRationaleVisibility).
type Audience string

// Audiences.
const (
	AudienceNone  Audience = "none"  // nobody
	AudienceUsers Audience = "users" // the instance's logged-in users
	AudienceAll   Audience = "all"   // everyone, anonymous visitors included
)

// Allows reports whether a reader may read what a reaches; authenticated is whether the reader
// is a logged-in user of the instance.
func (a Audience) Allows(authenticated bool) bool {
	return a == AudienceAll || (a == AudienceUsers && authenticated)
}

var audienceChoices = []string{string(AudienceNone), string(AudienceUsers), string(AudienceAll)}

// Bounds of SettingRemoteMediaCacheTTL, in seconds: a minute to a year.
const (
	MinRemoteMediaCacheTTL = 60
	MaxRemoteMediaCacheTTL = 365 * 24 * 60 * 60
)

// Bounds of SettingMaxStatusLength, in characters.
const (
	MinMaxStatusLength = 1
	MaxMaxStatusLength = 100000
)

// Bounds and default of SettingAppealWindow, in days (ADR-0038).
const (
	MinAppealWindow     = 20
	MaxAppealWindow     = 365
	DefaultAppealWindow = 180
)

// Kind is the scalar type of a setting's value.
type Kind string

// Kinds of setting values.
const (
	KindBool   Kind = "bool"   // "true" or "false"
	KindInt    Kind = "int"    // a decimal integer in [Min, Max]
	KindEnum   Kind = "enum"   // one of Choices
	KindLocale Kind = "locale" // a canonical BCP 47 language tag
	// KindMedia is the ID of a media file (a lowercase UUID), or "" for none.
	KindMedia Kind = "media"
)

// Definition describes a setting key: its type, allowed values, and default. A key that has
// never been set reads as its default.
type Definition struct {
	Key     SettingKey
	Kind    Kind
	Default string
	// Min and Max bound a KindInt value, inclusive.
	Min, Max int
	// Choices are the values of a KindEnum key.
	Choices []string
	Doc     string
}

var definitions = []Definition{
	{
		Key: SettingRegistrationMode, Kind: KindEnum, Default: string(RegistrationApproval),
		Choices: []string{string(RegistrationOpen), string(RegistrationApproval), string(RegistrationInvite), string(RegistrationClosed)},
		Doc:     "Who may sign up (FR-001).",
	},
	{
		Key: SettingMaxStatusLength, Kind: KindInt, Default: "500", Min: MinMaxStatusLength, Max: MaxMaxStatusLength,
		Doc: "Maximum length of a post, in characters (FR-101).",
	},
	{
		Key: SettingDefaultLocale, Kind: KindLocale, Default: "en",
		Doc: "Initial UI locale of new users, and the locale for visitors whose preferences match no catalog and for instance texts missing in theirs (ADR-0009).",
	},
	{
		Key: SettingLocalTimelineEnabled, Kind: KindBool, Default: "true",
		Doc: "Whether the local timeline is available (FR-202).",
	},
	{
		Key: SettingFederatedTimelineEnabled, Kind: KindBool, Default: "true",
		Doc: "Whether the federated timeline is available (FR-202).",
	},
	{
		Key: SettingPreviewImage, Kind: KindMedia, Default: "",
		Doc: "Media ID of the link preview image of profiles and posts without images; empty means the built-in image (ADR-0028).",
	},
	{
		Key: SettingRemoteMediaPolicy, Kind: KindEnum, Default: string(RemoteMediaNone),
		Choices: []string{string(RemoteMediaNone), string(RemoteMediaAvatars), string(RemoteMediaAll)},
		Doc:     "Which remote media files the instance keeps a copy of; none relays every file through the media proxy (FR-409, ADR-0005).",
	},
	{
		Key: SettingRemoteMediaCacheTTL, Kind: KindInt, Default: "86400", Min: MinRemoteMediaCacheTTL, Max: MaxRemoteMediaCacheTTL,
		Doc: "How long a fetched remote media file may be kept, in seconds: the max-age of the media proxy's responses for the CDN and browsers (FR-409).",
	},
	{
		Key: SettingAuthorizedFetch, Kind: KindBool, Default: "false",
		Doc: "Whether ActivityPub objects, collections, and full actor documents are served only to requests with a verified HTTP signature (authorized fetch, FR-408).",
	},
	{
		Key: SettingDomainActionsVisibility, Kind: KindEnum, Default: string(AudienceNone), Choices: audienceChoices,
		Doc: "Who may read the list of the limited and suspended domains: nobody, logged-in users, or everyone (FR-718).",
	},
	{
		Key: SettingDomainActionsRationaleVisibility, Kind: KindEnum, Default: string(AudienceNone), Choices: audienceChoices,
		Doc: "Who may read the public comments of the listed domain actions: nobody, logged-in users, or everyone (FR-718).",
	},
	{
		Key: SettingAppealWindow, Kind: KindInt, Default: strconv.Itoa(DefaultAppealWindow), Min: MinAppealWindow, Max: MaxAppealWindow,
		Doc: "How many days after a moderation action a local account may appeal it, once (FR-707, ADR-0038).",
	},
}

// Definitions returns the definitions of every setting key, in a fixed order.
func Definitions() []Definition {
	out := slices.Clone(definitions)
	for i := range out {
		out[i].Choices = slices.Clone(out[i].Choices)
	}
	return out
}

// Lookup returns the definition of key, or an error wrapping ErrUnknownKey.
func Lookup(key SettingKey) (Definition, error) {
	for _, d := range definitions {
		if d.Key == key {
			return d, nil
		}
	}
	return Definition{}, fmt.Errorf("setting %q: %w", key, ErrUnknownKey)
}

// Parse validates raw as a value of d and returns its canonical string form, which is what the
// database stores: "true"/"false" for KindBool and a decimal without sign or leading zeros
// for KindInt. A KindLocale value must already be canonical (the application layer
// canonicalizes it with internal/i18n); Parse checks only its shape.
func (d Definition) Parse(raw string) (string, error) {
	invalid := func(why string) error {
		return fmt.Errorf("setting %s: %q %s: %w", d.Key, raw, why, ErrInvalidValue)
	}
	switch d.Kind {
	case KindBool:
		switch strings.ToLower(raw) {
		case "true":
			return "true", nil
		case "false":
			return "false", nil
		}
		return "", invalid("is not true or false")
	case KindInt:
		n, err := strconv.Atoi(raw)
		if err != nil {
			return "", invalid("is not an integer")
		}
		if n < d.Min || n > d.Max {
			return "", invalid(fmt.Sprintf("is outside %d..%d", d.Min, d.Max))
		}
		return strconv.Itoa(n), nil
	case KindEnum:
		if slices.Contains(d.Choices, raw) {
			return raw, nil
		}
		return "", invalid("is not one of " + strings.Join(d.Choices, ", "))
	case KindLocale:
		if !wellFormedTag(raw) {
			return "", invalid("is not a BCP 47 language tag")
		}
		return raw, nil
	case KindMedia:
		id := strings.ToLower(raw)
		if id != "" && !wellFormedUUID(id) {
			return "", invalid("is not a media ID")
		}
		return id, nil
	}
	return "", invalid("has an unknown kind " + string(d.Kind))
}

// wellFormedTag reports whether tag consists of hyphen-separated subtags of 1 to 8 ASCII
// letters or digits, starting with a 2 or 3 letter language subtag.
func wellFormedTag(tag string) bool {
	parts := strings.Split(tag, "-")
	if l := len(parts[0]); l < 2 || l > 3 {
		return false
	}
	for i, p := range parts {
		if p == "" || len(p) > 8 {
			return false
		}
		for _, r := range p {
			letter := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
			if !letter && (i == 0 || r < '0' || r > '9') {
				return false
			}
		}
	}
	return true
}

// wellFormedUUID reports whether id is a UUID in lowercase hexadecimal, 8-4-4-4-12.
func wellFormedUUID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, r := range id {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
				return false
			}
		}
	}
	return true
}

// SettingRevision is one change of a setting (instance_setting_revisions). Value is canonical
// (Definition.Parse). The revision with the greatest ID is the current value of its key.
type SettingRevision struct {
	ID    string
	Key   SettingKey
	Value string
	// RevisedByUserID is the user who made the change, or "" for a change from the CLI.
	RevisedByUserID string
	RevisedAt       time.Time
}

// CurrentSetting is the current value of a key (the instance_settings projection).
type CurrentSetting struct {
	Key        SettingKey
	RevisionID string
	Value      string
}

// Supersedes reports whether r should replace the current value c, which is the zero value
// when the key has no current value yet. IDs are UUIDv7, so the later revision wins.
func (r SettingRevision) Supersedes(c CurrentSetting) bool {
	return c.RevisionID == "" || r.ID > c.RevisionID
}

// Settings are the values of every setting key: the stored value, or the default for a key
// that has never been set. The zero value reads every key as its default.
type Settings struct {
	stored map[SettingKey]string
}

// NewSettings returns the settings whose stored values are current. Values of unknown keys
// are ignored.
func NewSettings(current []CurrentSetting) Settings {
	s := Settings{stored: map[SettingKey]string{}}
	for _, c := range current {
		if _, err := Lookup(c.Key); err == nil {
			s.stored[c.Key] = c.Value
		}
	}
	return s
}

// Value returns the value of key and whether it is stored (false: the default applies).
// It returns "" and false for an unknown key.
func (s Settings) Value(key SettingKey) (value string, stored bool) {
	if v, ok := s.stored[key]; ok {
		return v, true
	}
	d, err := Lookup(key)
	if err != nil {
		return "", false
	}
	return d.Default, false
}

func (s Settings) boolValue(key SettingKey) bool {
	v, _ := s.Value(key)
	return v == "true"
}

// RegistrationMode returns the value of SettingRegistrationMode.
func (s Settings) RegistrationMode() RegistrationMode {
	v, _ := s.Value(SettingRegistrationMode)
	return RegistrationMode(v)
}

// intValue returns the value of a KindInt key, or its default when the stored value does not
// parse.
func (s Settings) intValue(key SettingKey) int {
	v, _ := s.Value(key)
	n, err := strconv.Atoi(v)
	if err != nil {
		d, _ := Lookup(key)
		n, _ = strconv.Atoi(d.Default)
	}
	return n
}

// MaxStatusLength returns the value of SettingMaxStatusLength.
func (s Settings) MaxStatusLength() int { return s.intValue(SettingMaxStatusLength) }

// DefaultLocale returns the value of SettingDefaultLocale.
func (s Settings) DefaultLocale() string {
	v, _ := s.Value(SettingDefaultLocale)
	return v
}

// LocalTimelineEnabled returns the value of SettingLocalTimelineEnabled.
func (s Settings) LocalTimelineEnabled() bool { return s.boolValue(SettingLocalTimelineEnabled) }

// FederatedTimelineEnabled returns the value of SettingFederatedTimelineEnabled.
func (s Settings) FederatedTimelineEnabled() bool {
	return s.boolValue(SettingFederatedTimelineEnabled)
}

// PreviewImageMediaID returns the value of SettingPreviewImage, "" when unset.
func (s Settings) PreviewImageMediaID() string {
	v, _ := s.Value(SettingPreviewImage)
	return v
}

// RemoteMediaPolicy returns the value of SettingRemoteMediaPolicy.
func (s Settings) RemoteMediaPolicy() RemoteMediaPolicy {
	v, _ := s.Value(SettingRemoteMediaPolicy)
	return RemoteMediaPolicy(v)
}

// RemoteMediaCacheTTL returns the value of SettingRemoteMediaCacheTTL.
func (s Settings) RemoteMediaCacheTTL() time.Duration {
	return time.Duration(s.intValue(SettingRemoteMediaCacheTTL)) * time.Second
}

// AuthorizedFetch returns the value of SettingAuthorizedFetch.
func (s Settings) AuthorizedFetch() bool { return s.boolValue(SettingAuthorizedFetch) }

// DomainActionsVisibility returns the value of SettingDomainActionsVisibility.
func (s Settings) DomainActionsVisibility() Audience {
	v, _ := s.Value(SettingDomainActionsVisibility)
	return Audience(v)
}

// DomainActionsRationaleVisibility returns the value of
// SettingDomainActionsRationaleVisibility.
func (s Settings) DomainActionsRationaleVisibility() Audience {
	v, _ := s.Value(SettingDomainActionsRationaleVisibility)
	return Audience(v)
}

// AppealWindow returns the value of SettingAppealWindow as a duration.
func (s Settings) AppealWindow() time.Duration {
	return time.Duration(s.intValue(SettingAppealWindow)) * 24 * time.Hour
}

// MediaKeys returns the setting keys whose values are media IDs.
func MediaKeys() []SettingKey {
	var out []SettingKey
	for _, d := range definitions {
		if d.Kind == KindMedia {
			out = append(out, d.Key)
		}
	}
	return out
}
