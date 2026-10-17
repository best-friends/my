// SPDX-License-Identifier: AGPL-3.0-only

// Package config defines the typed configuration schema and loads it at startup.
//
// Each setting is resolved from, in order of precedence, its environment
// variable (EnvPrefix + upper-cased key), the optional TOML file named by
// MY_CONFIG, and its default. Secrets come only from the environment, either
// directly or through NAME_FILE. Load reports every problem at once.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// Role selects what a process runs.
type Role string

// Roles.
const (
	RoleAll    Role = "all"
	RoleWeb    Role = "web"
	RoleWorker Role = "worker"
)

// Config is the validated configuration. Adapter fields hold the adapter name.
type Config struct {
	Host          string
	AccountDomain string
	SourceURL     string
	// WebOrigin is the origin of the web client that the client API allows by CORS.
	WebOrigin   string
	Role        Role
	Listen      string
	DB          string
	DatabaseURL string
	Queue       string
	Blob        string
	Media       string
	// BlobFSDir is the directory of the fs blob store.
	BlobFSDir string
	// Limits of image uploads and how long an upload may stay unattached.
	MediaMaxImageBytes  int64
	MediaMaxImageSide   int
	MediaMaxImagePixels int64
	MediaUnattachedTTL  time.Duration
	// RemoteMediaCacheMaxBytes bounds the total size of the remote media cache.
	RemoteMediaCacheMaxBytes int64
	KV                       string
	KeyManager               string
	KEK                      []byte
	PreviousKEKs             [][]byte
	// Mail is the Mailer adapter; MailFrom and the SMTP fields configure it.
	Mail         string
	MailFrom     string
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	// Profile is "production" or "development".
	Profile string
	// HTTPAllowPrivateNetworks lets the HTTPClient reach non-public destinations (development only).
	HTTPAllowPrivateNetworks bool
	// RemoteStatusRetention is how long an unreferenced remote post is kept after its publication;
	// RemoteStatusMaxRetention bounds how long any remote post is kept, zero for no bound (ADR-0034).
	RemoteStatusRetention    time.Duration
	RemoteStatusMaxRetention time.Duration
}

// LogValue logs the configuration with its secrets redacted (ADR-0019 §6).
func (c *Config) LogValue() slog.Value {
	redact := func(set bool) string {
		if set {
			return "(redacted)"
		}
		return ""
	}
	return slog.GroupValue(
		slog.String("host", c.Host),
		slog.String("account_domain", c.AccountDomain),
		slog.String("source_url", c.SourceURL),
		slog.String("web_origin", c.WebOrigin),
		slog.String("role", string(c.Role)),
		slog.String("listen", c.Listen),
		slog.String("db", c.DB),
		slog.String("database_url", redact(c.DatabaseURL != "")),
		slog.String("queue", c.Queue),
		slog.String("blob", c.Blob),
		slog.String("media", c.Media),
		slog.String("blob_fs_dir", c.BlobFSDir),
		slog.Int64("media_max_image_bytes", c.MediaMaxImageBytes),
		slog.Int("media_max_image_side", c.MediaMaxImageSide),
		slog.Int64("media_max_image_pixels", c.MediaMaxImagePixels),
		slog.Duration("media_unattached_ttl", c.MediaUnattachedTTL),
		slog.Int64("remote_media_cache_max_bytes", c.RemoteMediaCacheMaxBytes),
		slog.String("kv", c.KV),
		slog.String("key_manager", c.KeyManager),
		slog.String("kek", redact(len(c.KEK) > 0)),
		slog.Int("kek_previous", len(c.PreviousKEKs)),
		slog.String("mail", c.Mail),
		slog.String("mail_from", c.MailFrom),
		slog.String("smtp_host", c.SMTPHost),
		slog.Int("smtp_port", c.SMTPPort),
		slog.String("smtp_username", c.SMTPUsername),
		slog.String("smtp_password", redact(c.SMTPPassword != "")),
		slog.String("profile", c.Profile),
		slog.Bool("http_allow_private_networks", c.HTTPAllowPrivateNetworks),
		slog.Duration("remote_status_retention", c.RemoteStatusRetention),
		slog.Duration("remote_status_max_retention", c.RemoteStatusMaxRetention),
	)
}

// Origin describes where a resolved value came from.
type Origin string

// Origins.
const (
	OriginUnset   Origin = "unset"
	OriginDefault Origin = "default"
	OriginFile    Origin = "file"
	OriginEnv     Origin = "env"
)

// Setting is one resolved setting, for display by `my config check`.
type Setting struct {
	Key    string
	Env    string
	Value  string // "" when unset; redacted for secrets
	Secret bool
	Origin Origin
}

// Issue is one validation problem.
type Issue struct {
	// Key is the setting the problem is about, or "" for a problem with the config file itself.
	Key string
	// Missing reports that the setting Key is required but not set; otherwise an issue
	// with a Key is about a value that is set but invalid.
	Missing bool
	// Message is the technical detail in English; it may be empty for a Missing issue.
	Message string
}

// Env returns the environment variable of the setting Key, or "" when there is none.
func (i Issue) Env() string {
	for k := range schema {
		if schema[k].key == i.Key {
			return schema[k].env()
		}
	}
	return ""
}

func (i Issue) String() string {
	if i.Key == "" {
		return i.Message
	}
	if i.Missing && i.Message == "" {
		i.Message = "required"
	}
	if env := i.Env(); env != "" {
		return fmt.Sprintf("%s (%s): %s", i.Key, env, i.Message)
	}
	return fmt.Sprintf("%s: %s", i.Key, i.Message)
}

// ValidationError lists every problem found while loading.
type ValidationError struct {
	Issues []Issue
}

func (e *ValidationError) Error() string {
	lines := make([]string, len(e.Issues))
	for i, is := range e.Issues {
		lines[i] = is.String()
	}
	return "invalid configuration:\n  " + strings.Join(lines, "\n  ")
}

// Source supplies the raw inputs. Zero fields fall back to the process environment and file system.
type Source struct {
	// LookupEnv reads an environment variable; defaults to os.LookupEnv.
	LookupEnv func(string) (string, bool)
	// ReadFile reads a file; defaults to os.ReadFile.
	ReadFile func(string) ([]byte, error)
	// File is the TOML file path; when empty, MY_CONFIG is used, and no file is read if that is unset too.
	File string
}

// Load resolves, validates, and returns the configuration together with the
// resolved settings. On failure the error is a *ValidationError listing every
// issue, and the settings are still returned for display.
func Load(src Source) (*Config, []Setting, error) {
	if src.LookupEnv == nil {
		src.LookupEnv = os.LookupEnv
	}
	if src.ReadFile == nil {
		src.ReadFile = os.ReadFile
	}
	var issues []Issue

	file := map[string]any{}
	path := src.File
	if path == "" {
		path, _ = src.LookupEnv(EnvConfigFile)
	}
	if path != "" {
		data, err := src.ReadFile(path)
		if err != nil {
			issues = append(issues, Issue{Message: fmt.Sprintf("config file: %v", err)})
		} else if err := toml.Unmarshal(data, &file); err != nil {
			issues = append(issues, Issue{Message: fmt.Sprintf("config file %s: %v", path, err)})
		}
	}
	for _, k := range unknownKeys(file) {
		issues = append(issues, Issue{Message: fmt.Sprintf("config file %s: unknown key %q", path, k)})
	}

	cfg := &Config{}
	settings := make([]Setting, 0, len(schema))
	valid := map[string]string{}
	present := map[string]bool{}
	for i := range schema {
		f := &schema[i]
		v, origin, srcIssues := resolve(f, src, file)
		issues = append(issues, srcIssues...)
		s := Setting{Key: f.key, Env: f.env(), Value: v, Secret: f.secret, Origin: origin}
		if f.secret && v != "" {
			s.Value = "(redacted)"
		}
		settings = append(settings, s)
		if len(srcIssues) > 0 {
			present[f.key] = true
			continue
		}
		if v == "" {
			if f.required {
				issues = append(issues, Issue{Key: f.key, Missing: true})
			}
			continue
		}
		present[f.key] = true
		if err := validate(f, v); err != nil {
			issues = append(issues, Issue{Key: f.key, Message: err.Error()})
			continue
		}
		valid[f.key] = v
		f.set(cfg, v)
	}
	issues = append(issues, crossChecks(valid, present)...)
	if cfg.AccountDomain == "" {
		cfg.AccountDomain = cfg.Host
	}
	if cfg.WebOrigin == "" {
		cfg.WebOrigin = "https://" + cfg.Host
	}
	if len(issues) > 0 {
		return nil, settings, &ValidationError{Issues: issues}
	}
	return cfg, settings, nil
}

// resolve returns a setting's raw value and origin, following the precedence env > file > default.
func resolve(f *field, src Source, file map[string]any) (string, Origin, []Issue) {
	var issues []Issue
	fileVal, inFile := file[f.key]
	if f.secret {
		if inFile {
			issues = append(issues, Issue{Key: f.key, Message: fmt.Sprintf("is a secret; set %s or %s_FILE instead of the config file", f.env(), f.env())})
		}
		v, ok := src.LookupEnv(f.env())
		pathV, pathOK := src.LookupEnv(f.env() + "_FILE")
		switch {
		case ok && pathOK:
			issues = append(issues, Issue{Key: f.key, Message: fmt.Sprintf("set only one of %s and %s_FILE", f.env(), f.env())})
		case pathOK:
			data, err := src.ReadFile(pathV)
			if err != nil {
				issues = append(issues, Issue{Key: f.key, Message: fmt.Sprintf("%s_FILE: %v", f.env(), err)})
				break
			}
			return strings.TrimRight(string(data), "\r\n"), OriginEnv, issues
		case ok:
			return v, OriginEnv, issues
		}
		return "", OriginUnset, issues
	}
	if v, ok := src.LookupEnv(f.env()); ok {
		return v, OriginEnv, nil
	}
	if inFile {
		s, ok := fileVal.(string)
		if !ok {
			return "", OriginFile, []Issue{{Key: f.key, Message: fmt.Sprintf("must be a string in the config file, got %T", fileVal)}}
		}
		return s, OriginFile, nil
	}
	if f.def != "" {
		return f.def, OriginDefault, nil
	}
	return "", OriginUnset, nil
}

func validate(f *field, v string) error {
	if len(f.choices) > 0 {
		i := slices.IndexFunc(f.choices, func(c choice) bool { return c.value == v })
		if i < 0 {
			return fmt.Errorf("unknown value %q; want one of %s", v, strings.Join(v0Values(f), ", "))
		}
		if !f.choices[i].inV0 {
			return fmt.Errorf("%q is not available in v0; want one of %s", v, strings.Join(v0Values(f), ", "))
		}
	}
	if f.check != nil {
		return f.check(v)
	}
	return nil
}

func v0Values(f *field) []string {
	var vs []string
	for _, c := range f.choices {
		if c.inV0 {
			vs = append(vs, c.value)
		}
	}
	return vs
}

func unknownKeys(file map[string]any) []string {
	var unknown []string
	for k := range file {
		if !slices.ContainsFunc(schema, func(f field) bool { return f.key == k }) {
			unknown = append(unknown, k)
		}
	}
	slices.Sort(unknown)
	return unknown
}
