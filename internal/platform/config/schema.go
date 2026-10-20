// SPDX-License-Identifier: AGPL-3.0-only

package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"net"
	"net/mail"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	snsstream "github.com/best-friends/my/internal/adapter/stream/sns"
	"github.com/best-friends/my/internal/adapter/valkey"
)

// EnvPrefix is prepended to the upper-cased key to form a setting's environment variable.
const EnvPrefix = "MY_"

// EnvConfigFile names the environment variable that points at the optional TOML file.
const EnvConfigFile = EnvPrefix + "CONFIG"

// DefaultSourceURL is the repository of the unmodified server, the default of source_url.
const DefaultSourceURL = "https://github.com/best-friends/my"

// kekSize is the length in bytes of the AES-256 key-encryption key (ADR-0019 §6).
const kekSize = 32

// choice is one allowed value of an enumerated setting.
type choice struct {
	value string
	// inV0 reports whether v0 ships this value. Values outside v0 are documented
	// (they are decided in an ADR) but rejected by validation.
	inV0 bool
	doc  string
}

// field describes one setting. The schema is the single source of truth for
// loading, validation, and the generated reference (docs/reference/configuration.md).
type field struct {
	// key is the TOML key; the environment variable is EnvPrefix + upper(key).
	key string
	doc string
	// def is the default value; empty means no default.
	def string
	// defDoc documents a default computed from other settings (def is then empty).
	defDoc   string
	required bool
	// requiredWhen documents a condition under which the setting is required (checked in crossChecks).
	requiredWhen string
	// secret settings are read only from the environment (NAME or NAME_FILE)
	// and are never printed (ADR-0010, ADR-0019 §6).
	secret  bool
	choices []choice
	// check validates a non-empty value beyond the choices.
	check func(string) error
	// set stores a validated value in the Config.
	set func(*Config, string)
}

// env returns the environment variable that sets f.
func (f *field) env() string { return EnvPrefix + strings.ToUpper(f.key) }

// Keys returns the key of every setting, in documentation order.
func Keys() []string {
	keys := make([]string, len(schema))
	for i := range schema {
		keys[i] = schema[i].key
	}
	return keys
}

// schema lists every setting in documentation order.
var schema = []field{
	{
		key:      "host",
		doc:      "Domain that serves the instance. Every ActivityPub `id` is under `https://{host}/` (ADR-0018). A port is allowed for local development. Recorded in the database on first start and must not change afterwards.",
		required: true,
		check:    checkDomain,
		set:      func(c *Config, v string) { c.Host = v },
	},
	{
		key:    "account_domain",
		defDoc: "value of `host`",
		doc:    "Domain of user handles (`@alice@{account_domain}`). Defaults to `host`. When it differs, `account_domain` must redirect `/.well-known/webfinger`, `/.well-known/host-meta`, and `/.well-known/nodeinfo` to `host` (ADR-0018). Recorded in the database on first start and must not change afterwards.",
		check:  checkDomain,
		set:    func(c *Config, v string) { c.AccountDomain = v },
	},
	{
		key:   "source_url",
		doc:   "Where users get the source code of the running server, as an `https` URL. NodeInfo publishes it as `software.repository` (AGPL-3.0 section 13, ADR-0011); point it at your own repository when you run a modified version.",
		def:   DefaultSourceURL,
		check: checkSourceURL,
		set:   func(c *Config, v string) { c.SourceURL = v },
	},
	{
		key:    "web_origin",
		defDoc: "`https://{host}`",
		doc:    "Origin (`scheme://host[:port]`) of the web client. The client API allows only this origin by CORS (ADR-0019 §4). Set it when the web client is served from another origin, for example a development server at `http://localhost:5173`. An origin on `host` is where the server itself serves the web client, and the client served there calls the API at it: set `http://localhost:8080` (with `host` = `localhost:8080`) to run the server over plain HTTP on localhost, where the sign-in screens, the OAuth issuer, the client API, the links to the instance's pages in its responses, media URLs, and email and invite links are all on `http://localhost:8080`. ActivityPub ids stay under `https://{host}/` in every setup.",
		check:  checkOrigin,
		set:    func(c *Config, v string) { c.WebOrigin = v },
	},
	{
		key: "role",
		doc: "What this process runs.",
		def: string(RoleAll),
		choices: []choice{
			{value: string(RoleAll), inV0: true, doc: "HTTP server and job worker in one process."},
			{value: string(RoleWeb), inV0: true, doc: "HTTP server only."},
			{value: string(RoleWorker), inV0: true, doc: "Job worker only; the HTTP server answers only the health checks (docs/operations/running.md)."},
		},
		set: func(c *Config, v string) { c.Role = Role(v) },
	},
	{
		key:   "listen",
		doc:   "TCP address the HTTP server listens on, as `host:port`.",
		def:   ":8080",
		check: checkListen,
		set:   func(c *Config, v string) { c.Listen = v },
	},
	{
		key:   "trusted_proxies",
		doc:   "Reverse proxies and load balancers in front of the server whose forwarding header (`trusted_proxy_header`) tells the client IP, as comma-separated CIDRs or addresses such as `10.0.0.0/8, 2001:db8::/32, 192.0.2.10` (ADR-0019 §5). The client IP keys the rate limits of sign-in, registration, the client API, and the inbox. When the connection's peer is in this list, the client IP is the rightmost address of the header that is not in it; otherwise the header is ignored and the client IP is the peer. Empty trusts no one: behind a proxy every request then shares the proxy's limits. List only addresses that nothing but your proxies can connect from, and only proxies that append to the header rather than pass a client's value through (docs/architecture/deployment-targets.md).",
		check: checkTrustedProxies,
		set:   func(c *Config, v string) { c.TrustedProxies = parseTrustedProxies(v) },
	},
	{
		key: "trusted_proxy_header",
		doc: "Forwarding header that the `trusted_proxies` append the client IP to. Only this header is read, so that a client cannot add the other one past a proxy that does not know it.",
		def: "x-forwarded-for",
		choices: []choice{
			{value: "x-forwarded-for", inV0: true, doc: "`X-Forwarded-For`, which nginx, Caddy, Traefik, Google Cloud load balancers and Cloud Run, and AWS Application Load Balancers append to."},
			{value: "forwarded", inV0: true, doc: "The `for` parameters of `Forwarded` (RFC 7239)."},
		},
		set: func(c *Config, v string) { c.TrustedProxyHeader = v },
	},
	{
		key: "db",
		doc: "Database adapter (ADR-0003). The adapter is never inferred from `database_url`.",
		def: "sqlite",
		choices: []choice{
			{value: "sqlite", inV0: true, doc: "Local SQLite file (`modernc.org/sqlite`)."},
			{value: "libsql", inV0: true, doc: "libSQL / Turso."},
			{value: "postgres", inV0: true, doc: "PostgreSQL and compatible services (`pgx/v5`)."},
		},
		set: func(c *Config, v string) { c.DB = v },
	},
	{
		key:      "database_url",
		doc:      "Connection string passed to the `db` adapter, in that adapter's format.",
		required: true,
		secret:   true,
		set:      func(c *Config, v string) { c.DatabaseURL = v },
	},
	{
		key: "queue",
		doc: "Job queue adapter (ADR-0004).",
		def: "db",
		choices: []choice{
			{value: "db", inV0: true, doc: "Queue table in the database."},
			{value: "inline", inV0: true, doc: "Runs jobs synchronously in the enqueuing process; for tests and development. Requires `role = all`."},
			{value: "sqs", inV0: true, doc: "Amazon SQS."},
			{value: "cloudtasks", inV0: true, doc: "Google Cloud Tasks."},
		},
		set: func(c *Config, v string) { c.Queue = v },
	},
	{
		key: "blob",
		doc: "Blob storage adapter for media (ADR-0005).",
		def: "fs",
		choices: []choice{
			{value: "fs", inV0: true, doc: "Local file system."},
			{value: "s3", inV0: true, doc: "S3-compatible API (AWS S3, Cloudflare R2, GCS XML API, MinIO)."},
			{value: "gcs", doc: "Native Google Cloud Storage API."},
		},
		set: func(c *Config, v string) { c.Blob = v },
	},
	{
		key: "media",
		doc: "Media processor adapter (ADR-0005).",
		def: "native",
		choices: []choice{
			{value: "native", inV0: true, doc: "Pure Go image processing."},
			{value: "vips", inV0: true, doc: "libvips; needs a cgo build with the adapter's build tag."},
			{value: "none", inV0: true, doc: "No processing except EXIF removal."},
		},
		set: func(c *Config, v string) { c.Media = v },
	},
	{
		key:   "blob_fs_dir",
		doc:   "Directory of the `fs` blob store, relative to the working directory or absolute. It holds the uploads as received (private) and the processed media, which the server serves under `/media/`. Share it between the web and worker processes, and back it up with the database.",
		def:   "blobs",
		check: checkNotBlank,
		set:   func(c *Config, v string) { c.BlobFSDir = v },
	},
	{
		key:   "media_max_image_bytes",
		doc:   "Largest accepted image upload in bytes (ADR-0019 §8).",
		def:   strconv.Itoa(16 << 20),
		check: checkPositiveInt,
		set:   func(c *Config, v string) { c.MediaMaxImageBytes, _ = strconv.ParseInt(v, 10, 64) },
	},
	{
		key:   "media_max_image_side",
		doc:   "Largest accepted image width and height in pixels, read from the file's header before decoding (ADR-0019 §8).",
		def:   "8192",
		check: checkPositiveInt,
		set:   func(c *Config, v string) { c.MediaMaxImageSide, _ = strconv.Atoi(v) },
	},
	{
		key:   "media_max_image_pixels",
		doc:   "Largest accepted image area (width × height, and for an animated GIF the frames together), read from the file's header before decoding (ADR-0019 §8). The default is 33 MP (7680 × 4320).",
		def:   strconv.Itoa(7680 * 4320),
		check: checkPositiveInt,
		set:   func(c *Config, v string) { c.MediaMaxImagePixels, _ = strconv.ParseInt(v, 10, 64) },
	},
	{
		key:   "media_unattached_ttl",
		doc:   "How long an upload may stay unattached to any post or profile before the hourly cleanup deletes it (FR-803), as a Go duration such as `24h`.",
		def:   "24h",
		check: checkPositiveDuration,
		set:   func(c *Config, v string) { c.MediaUnattachedTTL, _ = time.ParseDuration(v) },
	},
	{
		key:   "remote_media_cache_max_bytes",
		doc:   "Size limit in bytes of the copies of remote media that the media proxy keeps when the instance setting `remote_media_policy` is `avatars` or `all` (FR-409). The hourly cleanup deletes the expired copies, then the least recently accessed ones over this limit (FR-803).",
		def:   strconv.Itoa(1 << 30),
		check: checkPositiveInt,
		set:   func(c *Config, v string) { c.RemoteMediaCacheMaxBytes, _ = strconv.ParseInt(v, 10, 64) },
	},
	{
		key: "kv",
		doc: "Key-value store adapter for caches and rate limits (ADR-0003, ADR-0019 §5).",
		def: "db",
		choices: []choice{
			{value: "db", inV0: true, doc: "Table in the database."},
			{value: "memory", inV0: true, doc: "In-process memory; limits are per process."},
			{value: "valkey", inV0: true, doc: "Valkey or another server that speaks the Redis protocol (`valkey_url`); entries are shared by every process on the server."},
		},
		set: func(c *Config, v string) { c.KV = v },
	},
	{
		key:          "valkey_url",
		doc:          "Valkey server of the adapters that use one (`kv = valkey`, `stream = valkey`):`valkey://[user:password@]host[:port][/db]` (`redis://` is the same), `valkeys://` or `rediss://` for TLS, or `unix:///path/to/socket`. The path selects the database number. A cluster is found by itself.",
		secret:       true,
		requiredWhen: "when `kv = valkey` or `stream = valkey`",
		check:        valkey.CheckURL,
		set:          func(c *Config, v string) { c.ValkeyURL = v },
	},
	{
		key:   "valkey_tls_ca_file",
		doc:   "PEM file of the certificate authorities that the TLS certificate of `valkey_url` chains to, for a managed service with a private CA. Empty uses the system roots; it needs a `valkeys://` or `rediss://` URL.",
		check: checkNotBlank,
		set:   func(c *Config, v string) { c.ValkeyTLSCAFile = v },
	},
	{
		key:   "valkey_key_prefix",
		doc:   "Prefix of every key that this server writes to `valkey_url`, so that several instances or other applications can share the server.",
		def:   "my:",
		check: checkNotBlank,
		set:   func(c *Config, v string) { c.ValkeyKeyPrefix = v },
	},
	{
		key: "stream",
		doc: "Broker that carries the signals of the real-time updates (`GET /api/v1/streaming`) between processes (ADR-0050). It is never inferred from the other settings. With `hub`, a stream receives only the changes made in its own process, so run one process with `role = all`; a process with another role warns at start. The other brokers reach every process of the instance, and a process connects to them only while it has open streams (docs/architecture/deployment-targets.md).",
		def: "hub",
		choices: []choice{
			{value: "hub", inV0: true, doc: "In-process only; for one process with `role = all` (SQLite, or one PostgreSQL server process)."},
			{value: "postgres", inV0: true, doc: "PostgreSQL `LISTEN` / `NOTIFY` on the database of `database_url` (or `stream_postgres_url`). Requires `db = postgres`."},
			{value: "valkey", inV0: true, doc: "Valkey Pub/Sub on `valkey_url`, with the channel `{valkey_key_prefix}stream`."},
			{value: "sns", inV0: true, doc: "An Amazon SNS standard topic (`stream_sns_topic_arn`) that fans out to one SQS queue per listening process (`stream_sqs_queue_prefix`); credentials and settings from the AWS SDK's default chain."},
		},
		set: func(c *Config, v string) { c.Stream = v },
	},
	{
		key:    "stream_postgres_url",
		defDoc: "value of `database_url`",
		doc:    "Connection string of the `LISTEN` connections of `stream = postgres`, in the format of `database_url`. Set it when `database_url` goes through a pooler in transaction mode (PgBouncer's transaction pooling, Neon's pooled connection string, Supabase's port 6543), where `LISTEN` does not work: it must reach the same database directly or through a pooler in session mode.",
		secret: true,
		set:    func(c *Config, v string) { c.StreamPostgresURL = v },
	},
	{
		key:          "stream_sns_topic_arn",
		doc:          "ARN of the Amazon SNS standard topic of `stream = sns`, which the deployment template creates. The queues are created in its account and region.",
		requiredWhen: "when `stream = sns`",
		check:        snsstream.CheckTopicARN,
		set:          func(c *Config, v string) { c.StreamSNSTopicARN = v },
	},
	{
		key:          "stream_sqs_queue_prefix",
		doc:          "Start of the names of the per-process SQS queues of `stream = sns`, `{prefix}-{process id}-{n}`: at most 40 letters, digits, hyphens, and underscores. The `stream.sweep` job deletes the queues under it that ended processes left behind, so it belongs to one instance.",
		requiredWhen: "when `stream = sns`",
		check:        snsstream.CheckQueuePrefix,
		set:          func(c *Config, v string) { c.StreamSQSQueuePrefix = v },
	},
	{
		key: "key_manager",
		doc: "Key manager adapter that encrypts actor private keys (ADR-0019 §6).",
		def: "local",
		choices: []choice{
			{value: "local", inV0: true, doc: "Uses the key-encryption key from `kek`."},
			{value: "gcpkms", doc: "Google Cloud KMS."},
			{value: "awskms", doc: "AWS KMS."},
		},
		set: func(c *Config, v string) { c.KeyManager = v },
	},
	{
		key:          "kek",
		doc:          "Key-encryption key for actor private keys: 32 random bytes in standard base64 (for example `openssl rand -base64 32`). Required when `key_manager = local`. Back it up separately from the database; losing it loses every actor key (ADR-0019 §6, docs/operations/key-management.md).",
		secret:       true,
		requiredWhen: "when `key_manager = local`",
		check:        checkKEK,
		set: func(c *Config, v string) {
			c.KEK, _ = base64.StdEncoding.DecodeString(v) // validated by checkKEK
		},
	},
	{
		key:    "kek_previous",
		doc:    "Previous key-encryption keys, comma-separated, each in the format of `kek`. They only decrypt; new secrets are always encrypted with `kek`. Set the replaced key here while rotating `kek` (docs/operations/key-management.md).",
		secret: true,
		check:  checkPreviousKEKs,
		set: func(c *Config, v string) {
			for k := range strings.SplitSeq(v, ",") {
				b, _ := base64.StdEncoding.DecodeString(strings.TrimSpace(k)) // validated by checkPreviousKEKs
				c.PreviousKEKs = append(c.PreviousKEKs, b)
			}
		},
	},
	{
		key: "mail",
		doc: "Mailer adapter for verification emails (FR-002). With `none`, no email reaches users, so an unverified address does not keep a user out and sign-ups in the `open` registration mode wait for a moderator's approval instead.",
		def: "none",
		choices: []choice{
			{value: "none", inV0: true, doc: "Logs each message, including its links, instead of sending it; for development and instances without email."},
			{value: "smtp", inV0: true, doc: "SMTP submission with mandatory STARTTLS (`smtp_host`, `smtp_port`, `smtp_username`, `smtp_password`)."},
			{value: "ses", doc: "Amazon SES."},
		},
		set: func(c *Config, v string) { c.Mail = v },
	},
	{
		key:          "mail_from",
		doc:          "Sender address of emails, as a bare address such as `noreply@social.example`.",
		requiredWhen: "when `mail` is not `none`",
		check:        checkBareAddress,
		set:          func(c *Config, v string) { c.MailFrom = v },
	},
	{
		key:          "smtp_host",
		doc:          "Host name of the SMTP server. The server must offer STARTTLS with a certificate valid for this name; the connection is refused otherwise.",
		requiredWhen: "when `mail = smtp`",
		check:        checkHostName,
		set:          func(c *Config, v string) { c.SMTPHost = v },
	},
	{
		key:   "smtp_port",
		doc:   "TCP port of the SMTP server (submission).",
		def:   "587",
		check: checkPort,
		set: func(c *Config, v string) {
			c.SMTPPort, _ = strconv.Atoi(v) // validated by checkPort
		},
	},
	{
		key: "smtp_username",
		doc: "User name for SMTP AUTH PLAIN, sent only after STARTTLS. Leave it empty for a server that needs no authentication.",
		set: func(c *Config, v string) { c.SMTPUsername = v },
	},
	{
		key:    "smtp_password",
		doc:    "Password for SMTP AUTH PLAIN.",
		secret: true,
		set:    func(c *Config, v string) { c.SMTPPassword = v },
	},
	{
		key: "captcha",
		doc: "CAPTCHA service that the sign-in and registration forms check (ADR-0042). The service receives the visitor's IP address, browser details, and cookies, and the two pages load its script; say so in the instance's privacy policy. Only `none` keeps every request on this server.",
		def: "none",
		choices: []choice{
			{value: "none", inV0: true, doc: "No CAPTCHA; nothing is sent to a third party."},
			{value: "turnstile", inV0: true, doc: "Cloudflare Turnstile."},
			{value: "hcaptcha", inV0: true, doc: "hCaptcha."},
			{value: "recaptcha", inV0: true, doc: "Google reCAPTCHA v2 with the checkbox (not v3 or Enterprise)."},
		},
		set: func(c *Config, v string) { c.Captcha = v },
	},
	{
		key:          "captcha_site_key",
		doc:          "Public site key of the `captcha` service, which the widget on the forms carries.",
		requiredWhen: "when `captcha` is not `none`",
		check:        checkNotBlank,
		set:          func(c *Config, v string) { c.CaptchaSiteKey = v },
	},
	{
		key:          "captcha_secret_key",
		doc:          "Secret key of the `captcha` service, which the server sends with each token to the service's siteverify API. The server does not try the key at start: a key that the service refuses is logged as an error on the first check, and until it is fixed registrations are refused and sign-ins skip the CAPTCHA (ADR-0042 §4).",
		secret:       true,
		requiredWhen: "when `captcha` is not `none`",
		check:        checkNotBlank,
		set:          func(c *Config, v string) { c.CaptchaSecretKey = v },
	},
	{
		key: "profile",
		doc: "Whether this is a production instance. Settings that weaken a security default for development, such as `http_allow_private_networks`, are refused in the `production` profile (ADR-0019).",
		def: "production",
		choices: []choice{
			{value: "production", inV0: true, doc: "A production instance."},
			{value: "development", inV0: true, doc: "A development or test instance."},
		},
		set: func(c *Config, v string) { c.Profile = v },
	},
	{
		key: "http_allow_private_networks",
		doc: "Lets outbound HTTP (federation, media, link previews) reach loopback, private, and other non-public addresses and host names, over plain `http` and on any port. For development and tests against local servers only: the server logs a warning at start, and the `production` profile refuses it (ADR-0019 §1).",
		def: "false",
		choices: []choice{
			{value: "false", inV0: true, doc: "Only public `https` destinations on ports 443 and 80."},
			{value: "true", inV0: true, doc: "Every destination; requires `profile = development`."},
		},
		set: func(c *Config, v string) { c.HTTPAllowPrivateNetworks = v == "true" },
	},
	{
		key:   "remote_status_retention",
		doc:   "How long a remote post is kept after its publication when nothing on this instance refers to it (a local reply, boost, favourite, bookmark, notification, or a thread with a local post); the hourly purge then deletes it (ADR-0034, FR-803). A Go duration such as `2160h` (90 days).",
		def:   "2160h",
		check: checkPositiveDuration,
		set:   func(c *Config, v string) { c.RemoteStatusRetention, _ = time.ParseDuration(v) },
	},
	{
		key:   "remote_status_max_retention",
		doc:   "The longest a remote post is kept after its publication, even when this instance refers to it; the hourly purge then deletes it, while local accounts' replies, boosts, favourites, and bookmarks stay and show it as deleted (ADR-0034). A Go duration such as `8760h` (365 days); empty keeps referred posts without limit.",
		check: checkPositiveDuration,
		set:   func(c *Config, v string) { c.RemoteStatusMaxRetention, _ = time.ParseDuration(v) },
	},
}

// crossChecks rejects missing conditional settings and contradictory combinations.
// vals holds every setting that passed its own validation; present holds every
// setting that was given a non-empty value, valid or not.
func crossChecks(vals map[string]string, present map[string]bool) []Issue {
	var issues []Issue
	if vals["key_manager"] == "local" {
		if !present["kek"] {
			issues = append(issues, Issue{Key: "kek", Missing: true, Message: "required when key_manager is local"})
		}
	}
	if prev, ok := vals["kek_previous"]; ok {
		seen := map[string]bool{}
		if kek, ok := vals["kek"]; ok {
			seen[kek] = true
		}
		for k := range strings.SplitSeq(prev, ",") {
			k = strings.TrimSpace(k)
			if seen[k] {
				issues = append(issues, Issue{Key: "kek_previous", Message: "must not repeat a key or contain the current kek"})
				break
			}
			seen[k] = true
		}
	}
	if vals["kv"] == "valkey" && !present["valkey_url"] {
		issues = append(issues, Issue{Key: "valkey_url", Missing: true, Message: "required when kv is valkey"})
	} else if vals["stream"] == "valkey" && !present["valkey_url"] {
		issues = append(issues, Issue{Key: "valkey_url", Missing: true, Message: "required when stream is valkey"})
	}
	if db, ok := vals["db"]; ok && vals["stream"] == "postgres" && db != "postgres" {
		issues = append(issues, Issue{Key: "stream", Message: "postgres requires db = postgres, not " + db})
	}
	if vals["stream"] == "sns" {
		for _, k := range []string{"stream_sns_topic_arn", "stream_sqs_queue_prefix"} {
			if !present[k] {
				issues = append(issues, Issue{Key: k, Missing: true, Message: "required when stream is sns"})
			}
		}
	}
	if vals["valkey_tls_ca_file"] != "" && vals["valkey_url"] != "" && !strings.HasPrefix(vals["valkey_url"], "valkeys://") && !strings.HasPrefix(vals["valkey_url"], "rediss://") {
		issues = append(issues, Issue{Key: "valkey_tls_ca_file", Message: "needs a valkeys:// or rediss:// valkey_url"})
	}
	if mail, ok := vals["mail"]; ok && mail != "none" && !present["mail_from"] {
		issues = append(issues, Issue{Key: "mail_from", Missing: true, Message: "required when mail is not none"})
	}
	if vals["mail"] == "smtp" && !present["smtp_host"] {
		issues = append(issues, Issue{Key: "smtp_host", Missing: true, Message: "required when mail is smtp"})
	}
	if captcha, ok := vals["captcha"]; ok && captcha != "none" {
		for _, k := range []string{"captcha_site_key", "captcha_secret_key"} {
			if !present[k] {
				issues = append(issues, Issue{Key: k, Missing: true, Message: "required when captcha is not none"})
			}
		}
	}
	if vals["http_allow_private_networks"] == "true" && vals["profile"] != "development" {
		issues = append(issues, Issue{Key: "http_allow_private_networks", Message: "true is refused in the production profile; set profile = development"})
	}
	if vals["queue"] == "inline" && vals["role"] != "" && vals["role"] != string(RoleAll) {
		issues = append(issues, Issue{Key: "queue", Message: fmt.Sprintf("inline runs jobs in the enqueuing process and requires role = all, not %s", vals["role"])})
	}
	return issues
}

func checkNotBlank(v string) error {
	if strings.TrimSpace(v) == "" {
		return errors.New("must not be blank")
	}
	return nil
}

func checkPositiveInt(v string) error {
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 || n > math.MaxInt32 {
		return fmt.Errorf("must be a whole number from 1 to %d", math.MaxInt32)
	}
	return nil
}

func checkPositiveDuration(v string) error {
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return errors.New("must be a positive Go duration, for example 24h or 90m")
	}
	return nil
}

// checkDomain accepts a lower-case DNS name with an optional port.
func checkDomain(v string) error {
	errSyntax := errors.New("must be a domain name with an optional :port, without scheme or path")
	if strings.ContainsAny(v, "/?#@") {
		return errSyntax
	}
	name := v
	if strings.Contains(v, ":") {
		h, p, err := net.SplitHostPort(v)
		if err != nil {
			return errSyntax
		}
		if err := checkPort(p); err != nil {
			return err
		}
		name = h
	}
	if name != strings.ToLower(name) {
		return errors.New("must be lower case")
	}
	if net.ParseIP(name) != nil {
		return errors.New("must be a domain name, not an IP address")
	}
	if len(name) > 253 {
		return errors.New("must be at most 253 characters")
	}
	for label := range strings.SplitSeq(name, ".") {
		if !validLabel(label) {
			return errSyntax
		}
	}
	return nil
}

func validLabel(l string) bool {
	if l == "" || len(l) > 63 || l[0] == '-' || l[len(l)-1] == '-' {
		return false
	}
	for _, r := range l {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
	}
	return true
}

func checkSourceURL(v string) error {
	u, err := url.Parse(v)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" {
		return errors.New("must be an absolute https URL without user info or fragment")
	}
	return nil
}

func checkOrigin(v string) error {
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil ||
		u.Path != "" || u.RawQuery != "" || u.Fragment != "" || v != u.Scheme+"://"+u.Host {
		return errors.New("must be an origin: http or https, a host, and an optional port, without path or trailing slash")
	}
	if u.Host != strings.ToLower(u.Host) {
		return errors.New("must be lower case")
	}
	return nil
}

func checkListen(v string) error {
	_, p, err := net.SplitHostPort(v)
	if err != nil {
		return errors.New("must be host:port, for example :8080 or 127.0.0.1:8080")
	}
	return checkPort(p)
}

func checkPort(p string) error {
	n, err := strconv.Atoi(p)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("port %q must be a number from 1 to 65535", p)
	}
	return nil
}

func checkKEK(v string) error {
	b, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return errors.New("must be standard base64")
	}
	if len(b) != kekSize {
		return fmt.Errorf("must decode to %d bytes, got %d", kekSize, len(b))
	}
	return nil
}

// checkBareAddress accepts an email address without a display name or angle brackets.
func checkBareAddress(v string) error {
	a, err := mail.ParseAddress(v)
	if err != nil || a.Name != "" || a.Address != v {
		return errors.New("must be a bare email address such as noreply@example.com")
	}
	return nil
}

// checkHostName accepts a lower-case DNS name or an IP address, without a port.
func checkHostName(v string) error {
	if net.ParseIP(v) != nil {
		return nil
	}
	if v != strings.ToLower(v) {
		return errors.New("must be lower case")
	}
	if len(v) > 253 {
		return errors.New("must be at most 253 characters")
	}
	for label := range strings.SplitSeq(v, ".") {
		if !validLabel(label) {
			return errors.New("must be a host name or an IP address, without scheme or port")
		}
	}
	return nil
}

func checkPreviousKEKs(v string) error {
	for i, k := range strings.Split(v, ",") {
		if err := checkKEK(strings.TrimSpace(k)); err != nil {
			return fmt.Errorf("key #%d %w", i+1, err)
		}
	}
	return nil
}

// parseProxy parses one entry of trusted_proxies: a CIDR, or an address as its single-address
// prefix. IPv4-mapped IPv6 entries are unmapped, like the addresses they are compared with.
func parseProxy(v string) (netip.Prefix, error) {
	if strings.Contains(v, "/") {
		p, err := netip.ParsePrefix(v)
		if err != nil {
			return netip.Prefix{}, err
		}
		if a := p.Addr(); a.Is4In6() {
			bits := p.Bits() - 96
			if bits < 0 {
				return netip.Prefix{}, errors.New("an IPv4-mapped prefix must be at least /96")
			}
			p = netip.PrefixFrom(a.Unmap(), bits)
		}
		return p.Masked(), nil
	}
	a, err := netip.ParseAddr(v)
	if err != nil {
		return netip.Prefix{}, err
	}
	if a.Zone() != "" {
		return netip.Prefix{}, errors.New("must not have a zone")
	}
	a = a.Unmap()
	return netip.PrefixFrom(a, a.BitLen()), nil
}

func checkTrustedProxies(v string) error {
	for i, p := range strings.Split(v, ",") {
		p = strings.TrimSpace(p)
		if _, err := parseProxy(p); err != nil {
			return fmt.Errorf("entry #%d %q must be a CIDR such as 10.0.0.0/8 or an IP address", i+1, p)
		}
	}
	return nil
}

func parseTrustedProxies(v string) []netip.Prefix {
	var ps []netip.Prefix
	for p := range strings.SplitSeq(v, ",") {
		pfx, _ := parseProxy(strings.TrimSpace(p)) // validated by checkTrustedProxies
		ps = append(ps, pfx)
	}
	return ps
}
