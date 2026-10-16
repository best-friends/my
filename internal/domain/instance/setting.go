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

// Bounds of SettingMaxStatusLength, in characters.
const (
	MinMaxStatusLength = 1
	MaxMaxStatusLength = 100000
)

// Kind is the scalar type of a setting's value.
type Kind string

// Kinds of setting values.
const (
	KindBool   Kind = "bool"   // "true" or "false"
	KindInt    Kind = "int"    // a decimal integer in [Min, Max]
	KindEnum   Kind = "enum"   // one of Choices
	KindLocale Kind = "locale" // a canonical BCP 47 language tag
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
		Doc: "Locale for visitors whose preferences match no catalog, and for instance texts missing in theirs (ADR-0009).",
	},
	{
		Key: SettingLocalTimelineEnabled, Kind: KindBool, Default: "true",
		Doc: "Whether the local timeline is available (FR-202).",
	},
	{
		Key: SettingFederatedTimelineEnabled, Kind: KindBool, Default: "true",
		Doc: "Whether the federated timeline is available (FR-202).",
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

// MaxStatusLength returns the value of SettingMaxStatusLength.
func (s Settings) MaxStatusLength() int {
	v, _ := s.Value(SettingMaxStatusLength)
	n, err := strconv.Atoi(v)
	if err != nil {
		d, _ := Lookup(SettingMaxStatusLength)
		n, _ = strconv.Atoi(d.Default)
	}
	return n
}

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
