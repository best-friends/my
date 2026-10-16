// SPDX-License-Identifier: AGPL-3.0-only

package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// EnvPrefix is prepended to the upper-cased key to form a setting's environment variable.
const EnvPrefix = "MY_"

// EnvConfigFile names the environment variable that points at the optional TOML file.
const EnvConfigFile = EnvPrefix + "CONFIG"

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
		key: "role",
		doc: "What this process runs.",
		def: string(RoleAll),
		choices: []choice{
			{value: string(RoleAll), inV0: true, doc: "HTTP server and job worker in one process."},
			{value: string(RoleWeb), inV0: true, doc: "HTTP server only."},
			{value: string(RoleWorker), inV0: true, doc: "Job worker only."},
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
		key: "kv",
		doc: "Key-value store adapter for caches and rate limits (ADR-0003, ADR-0019 §5).",
		def: "db",
		choices: []choice{
			{value: "db", inV0: true, doc: "Table in the database."},
			{value: "memory", inV0: true, doc: "In-process memory; limits are per process."},
			{value: "redis", doc: "Redis."},
		},
		set: func(c *Config, v string) { c.KV = v },
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
}

// crossChecks rejects missing conditional settings and contradictory combinations.
// vals holds every setting that passed its own validation; present holds every
// setting that was given a non-empty value, valid or not.
func crossChecks(vals map[string]string, present map[string]bool) []Issue {
	var issues []Issue
	if vals["key_manager"] == "local" {
		if !present["kek"] {
			issues = append(issues, Issue{Key: "kek", Message: "required when key_manager is local"})
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
	if vals["queue"] == "inline" && vals["role"] != "" && vals["role"] != string(RoleAll) {
		issues = append(issues, Issue{Key: "queue", Message: fmt.Sprintf("inline runs jobs in the enqueuing process and requires role = all, not %s", vals["role"])})
	}
	return issues
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

func checkPreviousKEKs(v string) error {
	for i, k := range strings.Split(v, ",") {
		if err := checkKEK(strings.TrimSpace(k)); err != nil {
			return fmt.Errorf("key #%d %w", i+1, err)
		}
	}
	return nil
}
