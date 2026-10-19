// SPDX-License-Identifier: AGPL-3.0-only

package config

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io/fs"
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

var (
	testKEK  = base64.StdEncoding.EncodeToString(make([]byte, kekSize))
	testKEK1 = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, kekSize))
	testKEK2 = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{2}, kekSize))
)

// minimal is the smallest valid environment.
func minimal() map[string]string {
	return map[string]string{
		"MY_HOST":         "social.example.org",
		"MY_DATABASE_URL": "file:my.db",
		"MY_KEK":          testKEK,
	}
}

func with(base map[string]string, kv ...string) map[string]string {
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i+1] == unset {
			delete(base, kv[i])
			continue
		}
		base[kv[i]] = kv[i+1]
	}
	return base
}

const unset = "\x00"

func source(env map[string]string, files map[string]string) Source {
	return Source{
		LookupEnv: func(k string) (string, bool) { v, ok := env[k]; return v, ok },
		ReadFile: func(p string) ([]byte, error) {
			if s, ok := files[p]; ok {
				return []byte(s), nil
			}
			return nil, &fs.PathError{Op: "open", Path: p, Err: fs.ErrNotExist}
		},
	}
}

func TestLoadValid(t *testing.T) {
	kek := make([]byte, kekSize)
	tests := []struct {
		name  string
		env   map[string]string
		files map[string]string
		want  Config
	}{
		{
			name: "defaults",
			env:  minimal(),
			want: Config{
				Host: "social.example.org", AccountDomain: "social.example.org", SourceURL: DefaultSourceURL, WebOrigin: "https://social.example.org", Role: RoleAll, Listen: ":8080",
				DB: "sqlite", DatabaseURL: "file:my.db", Queue: "db", Blob: "fs", Media: "native", KV: "db",
				BlobFSDir: "blobs", MediaMaxImageBytes: 16 << 20, MediaMaxImageSide: 8192, MediaMaxImagePixels: 7680 * 4320, MediaUnattachedTTL: 24 * time.Hour, RemoteMediaCacheMaxBytes: 1 << 30, RemoteStatusRetention: 90 * 24 * time.Hour,
				KeyManager: "local", KEK: kek, Mail: "none", SMTPPort: 587, Captcha: "none", Profile: "production",
			},
		},
		{
			name: "file overrides defaults and env overrides file",
			env:  with(minimal(), "MY_CONFIG", "my.toml", "MY_QUEUE", "sqs"),
			files: map[string]string{"my.toml": `
account_domain = "example.org"
source_url = "https://git.example.org/fork/my"
web_origin = "http://localhost:5173"
role = "web"
queue = "db"
db = "postgres"
listen = "127.0.0.1:9000"
`},
			want: Config{
				Host: "social.example.org", AccountDomain: "example.org", SourceURL: "https://git.example.org/fork/my", WebOrigin: "http://localhost:5173", Role: RoleWeb, Listen: "127.0.0.1:9000",
				DB: "postgres", DatabaseURL: "file:my.db", Queue: "sqs", Blob: "fs", Media: "native", KV: "db",
				BlobFSDir: "blobs", MediaMaxImageBytes: 16 << 20, MediaMaxImageSide: 8192, MediaMaxImagePixels: 7680 * 4320, MediaUnattachedTTL: 24 * time.Hour, RemoteMediaCacheMaxBytes: 1 << 30, RemoteStatusRetention: 90 * 24 * time.Hour,
				KeyManager: "local", KEK: kek, Mail: "none", SMTPPort: 587, Captcha: "none", Profile: "production",
			},
		},
		{
			name:  "secrets from _FILE",
			env:   with(minimal(), "MY_DATABASE_URL", unset, "MY_DATABASE_URL_FILE", "/mnt/db-url", "MY_KEK", unset, "MY_KEK_FILE", "/mnt/kek"),
			files: map[string]string{"/mnt/db-url": "postgres://db/my\n", "/mnt/kek": testKEK + "\n"},
			want: Config{
				Host: "social.example.org", AccountDomain: "social.example.org", SourceURL: DefaultSourceURL, WebOrigin: "https://social.example.org", Role: RoleAll, Listen: ":8080",
				DB: "sqlite", DatabaseURL: "postgres://db/my", Queue: "db", Blob: "fs", Media: "native", KV: "db",
				BlobFSDir: "blobs", MediaMaxImageBytes: 16 << 20, MediaMaxImageSide: 8192, MediaMaxImagePixels: 7680 * 4320, MediaUnattachedTTL: 24 * time.Hour, RemoteMediaCacheMaxBytes: 1 << 30, RemoteStatusRetention: 90 * 24 * time.Hour,
				KeyManager: "local", KEK: kek, Mail: "none", SMTPPort: 587, Captcha: "none", Profile: "production",
			},
		},
		{
			name: "previous KEKs",
			env:  with(minimal(), "MY_KEK_PREVIOUS", testKEK1+", "+testKEK2),
			want: Config{
				Host: "social.example.org", AccountDomain: "social.example.org", SourceURL: DefaultSourceURL, WebOrigin: "https://social.example.org", Role: RoleAll, Listen: ":8080",
				DB: "sqlite", DatabaseURL: "file:my.db", Queue: "db", Blob: "fs", Media: "native", KV: "db",
				BlobFSDir: "blobs", MediaMaxImageBytes: 16 << 20, MediaMaxImageSide: 8192, MediaMaxImagePixels: 7680 * 4320, MediaUnattachedTTL: 24 * time.Hour, RemoteMediaCacheMaxBytes: 1 << 30, RemoteStatusRetention: 90 * 24 * time.Hour,
				KeyManager: "local", KEK: kek, Mail: "none", SMTPPort: 587, Captcha: "none", Profile: "production",
				PreviousKEKs: [][]byte{bytes.Repeat([]byte{1}, kekSize), bytes.Repeat([]byte{2}, kekSize)},
			},
		},
		{
			name: "smtp mailer",
			env: with(minimal(), "MY_MAIL", "smtp", "MY_MAIL_FROM", "noreply@social.example.org", "MY_SMTP_HOST", "smtp.example.org",
				"MY_SMTP_PORT", "2525", "MY_SMTP_USERNAME", "mailer", "MY_SMTP_PASSWORD", "hunter3"),
			want: Config{
				Host: "social.example.org", AccountDomain: "social.example.org", SourceURL: DefaultSourceURL, WebOrigin: "https://social.example.org", Role: RoleAll, Listen: ":8080",
				DB: "sqlite", DatabaseURL: "file:my.db", Queue: "db", Blob: "fs", Media: "native", KV: "db",
				BlobFSDir: "blobs", MediaMaxImageBytes: 16 << 20, MediaMaxImageSide: 8192, MediaMaxImagePixels: 7680 * 4320, MediaUnattachedTTL: 24 * time.Hour, RemoteMediaCacheMaxBytes: 1 << 30, RemoteStatusRetention: 90 * 24 * time.Hour,
				KeyManager: "local", KEK: kek, Mail: "smtp", MailFrom: "noreply@social.example.org", SMTPHost: "smtp.example.org",
				SMTPPort: 2525, SMTPUsername: "mailer", SMTPPassword: "hunter3", Captcha: "none", Profile: "production",
			},
		},
		{
			name:  "captcha with the secret key from _FILE",
			env:   with(minimal(), "MY_CAPTCHA", "turnstile", "MY_CAPTCHA_SITE_KEY", "0x4AAA-site", "MY_CAPTCHA_SECRET_KEY_FILE", "/mnt/captcha"),
			files: map[string]string{"/mnt/captcha": "0x4AAA-secret\n"},
			want: Config{
				Host: "social.example.org", AccountDomain: "social.example.org", SourceURL: DefaultSourceURL, WebOrigin: "https://social.example.org", Role: RoleAll, Listen: ":8080",
				DB: "sqlite", DatabaseURL: "file:my.db", Queue: "db", Blob: "fs", Media: "native", KV: "db",
				BlobFSDir: "blobs", MediaMaxImageBytes: 16 << 20, MediaMaxImageSide: 8192, MediaMaxImagePixels: 7680 * 4320, MediaUnattachedTTL: 24 * time.Hour, RemoteMediaCacheMaxBytes: 1 << 30, RemoteStatusRetention: 90 * 24 * time.Hour,
				KeyManager: "local", KEK: kek, Mail: "none", SMTPPort: 587,
				Captcha: "turnstile", CaptchaSiteKey: "0x4AAA-site", CaptchaSecretKey: "0x4AAA-secret", Profile: "production",
			},
		},
		{
			name: "host with port",
			env:  with(minimal(), "MY_HOST", "localhost:3000"),
			want: Config{
				Host: "localhost:3000", AccountDomain: "localhost:3000", SourceURL: DefaultSourceURL, WebOrigin: "https://localhost:3000", Role: RoleAll, Listen: ":8080",
				DB: "sqlite", DatabaseURL: "file:my.db", Queue: "db", Blob: "fs", Media: "native", KV: "db",
				BlobFSDir: "blobs", MediaMaxImageBytes: 16 << 20, MediaMaxImageSide: 8192, MediaMaxImagePixels: 7680 * 4320, MediaUnattachedTTL: 24 * time.Hour, RemoteMediaCacheMaxBytes: 1 << 30, RemoteStatusRetention: 90 * 24 * time.Hour,
				KeyManager: "local", KEK: kek, Mail: "none", SMTPPort: 587, Captcha: "none", Profile: "production",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := Load(source(tt.env, tt.files))
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if !reflect.DeepEqual(*got, tt.want) {
				t.Errorf("Load() =\n%+v\nwant\n%+v", *got, tt.want)
			}
		})
	}
}

func TestLoadInvalid(t *testing.T) {
	tests := []struct {
		name  string
		env   map[string]string
		files map[string]string
		file  string
		want  []string // every issue, in order
	}{
		{
			name: "all required missing",
			env:  map[string]string{},
			want: []string{
				"host (MY_HOST): required",
				"database_url (MY_DATABASE_URL): required",
				"kek (MY_KEK): required when key_manager is local",
			},
		},
		{
			name: "invalid values are all listed",
			env: with(minimal(),
				"MY_HOST", "https://Social.example.org/",
				"MY_ACCOUNT_DOMAIN", "10.0.0.1",
				"MY_SOURCE_URL", "http://example.org/my",
				"MY_WEB_ORIGIN", "http://localhost:5173/",
				"MY_ROLE", "both",
				"MY_LISTEN", "8080",
				"MY_DB", "mysql",
				"MY_KEK", "c2hvcnQ=",
			),
			want: []string{
				"host (MY_HOST): must be a domain name with an optional :port, without scheme or path",
				"account_domain (MY_ACCOUNT_DOMAIN): must be a domain name, not an IP address",
				"source_url (MY_SOURCE_URL): must be an absolute https URL without user info or fragment",
				"web_origin (MY_WEB_ORIGIN): must be an origin: http or https, a host, and an optional port, without path or trailing slash",
				`role (MY_ROLE): unknown value "both"; want one of all, web, worker`,
				"listen (MY_LISTEN): must be host:port, for example :8080 or 127.0.0.1:8080",
				`db (MY_DB): unknown value "mysql"; want one of sqlite, libsql, postgres`,
				"kek (MY_KEK): must decode to 32 bytes, got 5",
			},
		},
		{
			name: "upper case host and bad port",
			env:  with(minimal(), "MY_HOST", "Social.example.org", "MY_ACCOUNT_DOMAIN", "example.org:99999"),
			want: []string{
				"host (MY_HOST): must be lower case",
				`account_domain (MY_ACCOUNT_DOMAIN): port "99999" must be a number from 1 to 65535`,
			},
		},
		{
			name: "adapters outside v0",
			env:  with(minimal(), "MY_BLOB", "gcs", "MY_KV", "redis", "MY_KEY_MANAGER", "awskms", "MY_KEK", unset),
			want: []string{
				`blob (MY_BLOB): "gcs" is not available in v0; want one of fs, s3`,
				`kv (MY_KV): "redis" is not available in v0; want one of db, memory`,
				`key_manager (MY_KEY_MANAGER): "awskms" is not available in v0; want one of local`,
			},
		},
		{
			name: "media limits",
			env:  with(minimal(), "MY_MEDIA_MAX_IMAGE_BYTES", "0", "MY_MEDIA_MAX_IMAGE_SIDE", "big", "MY_MEDIA_UNATTACHED_TTL", "1 day"),
			want: []string{
				"media_max_image_bytes (MY_MEDIA_MAX_IMAGE_BYTES): must be a whole number from 1 to 2147483647",
				"media_max_image_side (MY_MEDIA_MAX_IMAGE_SIDE): must be a whole number from 1 to 2147483647",
				"media_unattached_ttl (MY_MEDIA_UNATTACHED_TTL): must be a positive Go duration, for example 24h or 90m",
			},
		},
		{
			name: "malformed previous KEK",
			env:  with(minimal(), "MY_KEK_PREVIOUS", testKEK1+",c2hvcnQ="),
			want: []string{"kek_previous (MY_KEK_PREVIOUS): key #2 must decode to 32 bytes, got 5"},
		},
		{
			name: "previous KEK repeats the current one",
			env:  with(minimal(), "MY_KEK_PREVIOUS", testKEK1+","+testKEK),
			want: []string{"kek_previous (MY_KEK_PREVIOUS): must not repeat a key or contain the current kek"},
		},
		{
			name: "previous KEK given twice",
			env:  with(minimal(), "MY_KEK_PREVIOUS", testKEK1+", "+testKEK1),
			want: []string{"kek_previous (MY_KEK_PREVIOUS): must not repeat a key or contain the current kek"},
		},
		{
			name: "smtp without a sender or a host",
			env:  with(minimal(), "MY_MAIL", "smtp"),
			want: []string{
				"mail_from (MY_MAIL_FROM): required when mail is not none",
				"smtp_host (MY_SMTP_HOST): required when mail is smtp",
			},
		},
		{
			name: "malformed mail settings",
			env:  with(minimal(), "MY_MAIL", "smtp", "MY_MAIL_FROM", "My <noreply@example.org>", "MY_SMTP_HOST", "smtp.example.org:587", "MY_SMTP_PORT", "0"),
			want: []string{
				"mail_from (MY_MAIL_FROM): must be a bare email address such as noreply@example.com",
				"smtp_host (MY_SMTP_HOST): must be a host name or an IP address, without scheme or port",
				`smtp_port (MY_SMTP_PORT): port "0" must be a number from 1 to 65535`,
			},
		},
		{
			name: "captcha without keys",
			env:  with(minimal(), "MY_CAPTCHA", "recaptcha"),
			want: []string{
				"captcha_site_key (MY_CAPTCHA_SITE_KEY): required when captcha is not none",
				"captcha_secret_key (MY_CAPTCHA_SECRET_KEY): required when captcha is not none",
			},
		},
		{
			name:  "captcha secret key in the config file",
			env:   with(minimal(), "MY_CONFIG", "my.toml"),
			files: map[string]string{"my.toml": "captcha = \"hcaptcha\"\ncaptcha_site_key = \"site\"\ncaptcha_secret_key = \"0x5eCrE7\"\n"},
			want: []string{
				"captcha_secret_key (MY_CAPTCHA_SECRET_KEY): is a secret; set MY_CAPTCHA_SECRET_KEY or MY_CAPTCHA_SECRET_KEY_FILE instead of the config file",
			},
		},
		{
			name: "unknown captcha service",
			env:  with(minimal(), "MY_CAPTCHA", "recaptcha-v3"),
			want: []string{`captcha (MY_CAPTCHA): unknown value "recaptcha-v3"`},
		},
		{
			name: "inline queue with a split role",
			env:  with(minimal(), "MY_QUEUE", "inline", "MY_ROLE", "worker"),
			want: []string{"queue (MY_QUEUE): inline runs jobs in the enqueuing process and requires role = all, not worker"},
		},
		{
			name: "private networks in the production profile",
			env:  with(minimal(), "MY_HTTP_ALLOW_PRIVATE_NETWORKS", "true"),
			want: []string{"http_allow_private_networks (MY_HTTP_ALLOW_PRIVATE_NETWORKS): true is refused in the production profile; set profile = development"},
		},
		{
			name: "secret sources",
			env:  with(minimal(), "MY_DATABASE_URL_FILE", "/x", "MY_KEK", unset, "MY_KEK_FILE", "/missing"),
			want: []string{
				"database_url (MY_DATABASE_URL): set only one of MY_DATABASE_URL and MY_DATABASE_URL_FILE",
				"kek (MY_KEK): MY_KEK_FILE: open /missing: file does not exist",
			},
		},
		{
			name: "file problems",
			env:  minimal(),
			file: "my.toml",
			files: map[string]string{"my.toml": `
hots = "typo.example"
role = 1
kek = "nope"
`},
			want: []string{
				`config file my.toml: unknown key "hots"`,
				"role (MY_ROLE): must be a string in the config file, got int64",
				"kek (MY_KEK): is a secret; set MY_KEK or MY_KEK_FILE instead of the config file",
			},
		},
		{
			name: "file not found",
			env:  with(minimal(), "MY_CONFIG", "nope.toml"),
			want: []string{"config file: open nope.toml: file does not exist"},
		},
		{
			name:  "file syntax error",
			env:   minimal(),
			file:  "bad.toml",
			files: map[string]string{"bad.toml": "role = "},
			want:  []string{"config file bad.toml: toml: "},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := source(tt.env, tt.files)
			src.File = tt.file
			cfg, _, err := Load(src)
			if cfg != nil {
				t.Errorf("Load() config = %+v, want nil", cfg)
			}
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("Load() error = %v, want *ValidationError", err)
			}
			got := make([]string, len(ve.Issues))
			for i, is := range ve.Issues {
				got[i] = is.String()
			}
			if len(got) != len(tt.want) {
				t.Fatalf("issues =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(tt.want, "\n"))
			}
			for i := range got {
				if !strings.HasPrefix(got[i], tt.want[i]) {
					t.Errorf("issue %d = %q, want prefix %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestLoadSettingsRedactSecrets(t *testing.T) {
	_, settings, err := Load(source(with(minimal(), "MY_KEK_PREVIOUS", testKEK1, "MY_SMTP_PASSWORD", "hunter3",
		"MY_CAPTCHA", "turnstile", "MY_CAPTCHA_SITE_KEY", "site", "MY_CAPTCHA_SECRET_KEY", "0x5eCrE7"), nil))
	if err != nil {
		t.Fatal(err)
	}
	keys := make([]string, len(settings))
	for i, s := range settings {
		keys[i] = s.Key
		if s.Secret && s.Value != "(redacted)" {
			t.Errorf("%s = %q, want redacted", s.Key, s.Value)
		}
	}
	for i := range schema {
		if !slices.Contains(keys, schema[i].key) {
			t.Errorf("settings miss %s", schema[i].key)
		}
	}
}

func TestSchemaWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for i := range schema {
		f := &schema[i]
		if seen[f.key] {
			t.Errorf("duplicate key %s", f.key)
		}
		seen[f.key] = true
		if f.doc == "" || f.set == nil {
			t.Errorf("%s: doc and set are required", f.key)
		}
		if f.def != "" {
			if err := validate(f, f.def); err != nil {
				t.Errorf("%s: default %q is invalid: %v", f.key, f.def, err)
			}
		}
		if f.secret && f.def != "" {
			t.Errorf("%s: secrets have no default", f.key)
		}
	}
}

// TestServerOrigin checks that the server is reached over plain HTTP only when web_origin says so
// on host, and over HTTPS in every other setup.
func TestServerOrigin(t *testing.T) {
	for _, c := range []struct{ host, webOrigin, want string }{
		{"social.example.org", unset, "https://social.example.org"},
		{"social.example.org", "https://social.example.org", "https://social.example.org"},
		{"localhost:8080", "http://localhost:8080", "http://localhost:8080"},
		// A web client served elsewhere (a development server) says nothing about the server's scheme.
		{"localhost:8080", "http://localhost:5173", "https://localhost:8080"},
		{"social.example.org", "http://localhost:5173", "https://social.example.org"},
	} {
		cfg, _, err := Load(source(with(minimal(), "MY_HOST", c.host, "MY_WEB_ORIGIN", c.webOrigin), nil))
		if err != nil {
			t.Fatal(err)
		}
		if got := cfg.ServerOrigin(); got != c.want {
			t.Errorf("host %q, web origin %q: ServerOrigin() = %q, want %q", c.host, c.webOrigin, got, c.want)
		}
	}
}

func TestConfigLogValueRedactsSecrets(t *testing.T) {
	cfg, _, err := Load(source(with(minimal(), "MY_DATABASE_URL", "postgres://user:hunter2@db/my", "MY_KEK_PREVIOUS", testKEK1, "MY_SMTP_PASSWORD", "hunter3",
		"MY_CAPTCHA", "hcaptcha", "MY_CAPTCHA_SITE_KEY", "site", "MY_CAPTCHA_SECRET_KEY", "0x5eCrE7"), nil))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	slog.New(slog.NewTextHandler(&buf, nil)).Info("config", "config", cfg)
	out := buf.String()
	for _, secret := range []string{"hunter2", "hunter3", "0x5eCrE7", string(cfg.KEK), string(cfg.PreviousKEKs[0]), testKEK, testKEK1} {
		if strings.Contains(out, secret) {
			t.Errorf("log output %q contains secret %q", out, secret)
		}
	}
	for _, want := range []string{"config.host=social.example.org", "config.database_url=(redacted)", "config.kek=(redacted)", "config.kek_previous=1", "config.captcha_secret_key=(redacted)"} {
		if !strings.Contains(out, want) {
			t.Errorf("log output %q does not contain %q", out, want)
		}
	}
}
