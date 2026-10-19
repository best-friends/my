<!-- Code generated from internal/platform/config/schema.go by "make config-docs". DO NOT EDIT. -->

# Configuration reference

The server reads each setting from these sources, highest precedence first (ADR-0010):

1. The environment variable `MY_<KEY>` (the key in upper case).
2. The TOML file named by `MY_CONFIG` (or `my config check -config <path>`), with the keys below at the top level.
3. The default.

Secrets are read only from the environment: either the variable itself or `<NAME>_FILE`, the path of a file that holds the value
(trailing newlines are removed), for Docker and Kubernetes secrets (ADR-0019 §6). Setting a secret in the TOML file is an error.

Adapters are always selected by an explicit key; nothing is inferred from a URL scheme.
Values marked "not in v0" are decided in an ADR but not shipped in v0, and are rejected.

`my config check` validates the configuration and lists every problem at once; the server refuses to start on any of them.
The problems are described in the language of the process locale (`LANGUAGE`, `LC_ALL`, `LC_MESSAGES`, `LANG`; English for `C` or an unsupported language),
followed by the technical detail in English.

## Configuration and instance settings

The settings on this page configure the deployment: what a process runs, which adapters it uses, where they are, and its secrets.
These values are fixed for the life of a process and change only with a restart.
The policies of the instance that administrators change while it runs (registration mode, maximum post length,
default locale, which timelines are enabled) and its localized texts (name, descriptions, terms, privacy policy)
are instance settings instead: they are stored in the database with every change kept as a revision (data-model §4.9),
and are changed with `my instance set` ([instance settings](../operations/instance-settings.md)).

The two never share a key, so neither overrides the other: no environment variable or config file key sets an
instance setting, and the database never holds a setting from this page. A test keeps the two key sets disjoint.
An instance setting that has never been set reads as its default, which is defined in code.

## Summary

| Key | Environment variable | Default | Required | Secret |
| --- | --- | --- | --- | --- |
| [`host`](#host) | `MY_HOST` | none | yes | no |
| [`account_domain`](#account_domain) | `MY_ACCOUNT_DOMAIN` | value of `host` | no | no |
| [`source_url`](#source_url) | `MY_SOURCE_URL` | `https://github.com/best-friends/my` | no | no |
| [`web_origin`](#web_origin) | `MY_WEB_ORIGIN` | `https://{host}` | no | no |
| [`role`](#role) | `MY_ROLE` | `all` | no | no |
| [`listen`](#listen) | `MY_LISTEN` | `:8080` | no | no |
| [`trusted_proxies`](#trusted_proxies) | `MY_TRUSTED_PROXIES` | none | no | no |
| [`trusted_proxy_header`](#trusted_proxy_header) | `MY_TRUSTED_PROXY_HEADER` | `x-forwarded-for` | no | no |
| [`db`](#db) | `MY_DB` | `sqlite` | no | no |
| [`database_url`](#database_url) | `MY_DATABASE_URL` | none | yes | yes |
| [`queue`](#queue) | `MY_QUEUE` | `db` | no | no |
| [`blob`](#blob) | `MY_BLOB` | `fs` | no | no |
| [`media`](#media) | `MY_MEDIA` | `native` | no | no |
| [`blob_fs_dir`](#blob_fs_dir) | `MY_BLOB_FS_DIR` | `blobs` | no | no |
| [`media_max_image_bytes`](#media_max_image_bytes) | `MY_MEDIA_MAX_IMAGE_BYTES` | `16777216` | no | no |
| [`media_max_image_side`](#media_max_image_side) | `MY_MEDIA_MAX_IMAGE_SIDE` | `8192` | no | no |
| [`media_max_image_pixels`](#media_max_image_pixels) | `MY_MEDIA_MAX_IMAGE_PIXELS` | `33177600` | no | no |
| [`media_unattached_ttl`](#media_unattached_ttl) | `MY_MEDIA_UNATTACHED_TTL` | `24h` | no | no |
| [`remote_media_cache_max_bytes`](#remote_media_cache_max_bytes) | `MY_REMOTE_MEDIA_CACHE_MAX_BYTES` | `1073741824` | no | no |
| [`kv`](#kv) | `MY_KV` | `db` | no | no |
| [`key_manager`](#key_manager) | `MY_KEY_MANAGER` | `local` | no | no |
| [`kek`](#kek) | `MY_KEK` | none | when `key_manager = local` | yes |
| [`kek_previous`](#kek_previous) | `MY_KEK_PREVIOUS` | none | no | yes |
| [`mail`](#mail) | `MY_MAIL` | `none` | no | no |
| [`mail_from`](#mail_from) | `MY_MAIL_FROM` | none | when `mail` is not `none` | no |
| [`smtp_host`](#smtp_host) | `MY_SMTP_HOST` | none | when `mail = smtp` | no |
| [`smtp_port`](#smtp_port) | `MY_SMTP_PORT` | `587` | no | no |
| [`smtp_username`](#smtp_username) | `MY_SMTP_USERNAME` | none | no | no |
| [`smtp_password`](#smtp_password) | `MY_SMTP_PASSWORD` | none | no | yes |
| [`captcha`](#captcha) | `MY_CAPTCHA` | `none` | no | no |
| [`captcha_site_key`](#captcha_site_key) | `MY_CAPTCHA_SITE_KEY` | none | when `captcha` is not `none` | no |
| [`captcha_secret_key`](#captcha_secret_key) | `MY_CAPTCHA_SECRET_KEY` | none | when `captcha` is not `none` | yes |
| [`profile`](#profile) | `MY_PROFILE` | `production` | no | no |
| [`http_allow_private_networks`](#http_allow_private_networks) | `MY_HTTP_ALLOW_PRIVATE_NETWORKS` | `false` | no | no |
| [`remote_status_retention`](#remote_status_retention) | `MY_REMOTE_STATUS_RETENTION` | `2160h` | no | no |
| [`remote_status_max_retention`](#remote_status_max_retention) | `MY_REMOTE_STATUS_MAX_RETENTION` | none | no | no |

## Settings

### `host`

Domain that serves the instance. Every ActivityPub `id` is under `https://{host}/` (ADR-0018). A port is allowed for local development. Recorded in the database on first start and must not change afterwards.

- Environment variable: `MY_HOST`
- Default: none
- Required: yes

### `account_domain`

Domain of user handles (`@alice@{account_domain}`). Defaults to `host`. When it differs, `account_domain` must redirect `/.well-known/webfinger`, `/.well-known/host-meta`, and `/.well-known/nodeinfo` to `host` (ADR-0018). Recorded in the database on first start and must not change afterwards.

- Environment variable: `MY_ACCOUNT_DOMAIN`
- Default: value of `host`
- Required: no

### `source_url`

Where users get the source code of the running server, as an `https` URL. NodeInfo publishes it as `software.repository` (AGPL-3.0 section 13, ADR-0011); point it at your own repository when you run a modified version.

- Environment variable: `MY_SOURCE_URL`
- Default: `https://github.com/best-friends/my`
- Required: no

### `web_origin`

Origin (`scheme://host[:port]`) of the web client. The client API allows only this origin by CORS (ADR-0019 §4). Set it when the web client is served from another origin, for example a development server at `http://localhost:5173`. An origin on `host` is where the server itself serves the web client, and the client served there calls the API at it: set `http://localhost:8080` (with `host` = `localhost:8080`) to run the server over plain HTTP on localhost, where the sign-in screens, the OAuth issuer, the client API, the links to the instance's pages in its responses, media URLs, and email and invite links are all on `http://localhost:8080`. ActivityPub ids stay under `https://{host}/` in every setup.

- Environment variable: `MY_WEB_ORIGIN`
- Default: `https://{host}`
- Required: no

### `role`

What this process runs.

- Environment variable: `MY_ROLE`
- Default: `all`
- Required: no

| Value | In v0 | Description |
| --- | --- | --- |
| `all` | yes | HTTP server and job worker in one process. |
| `web` | yes | HTTP server only. |
| `worker` | yes | Job worker only; the HTTP server answers only the health checks (docs/operations/running.md). |

### `listen`

TCP address the HTTP server listens on, as `host:port`.

- Environment variable: `MY_LISTEN`
- Default: `:8080`
- Required: no

### `trusted_proxies`

Reverse proxies and load balancers in front of the server whose forwarding header (`trusted_proxy_header`) tells the client IP, as comma-separated CIDRs or addresses such as `10.0.0.0/8, 2001:db8::/32, 192.0.2.10` (ADR-0019 §5). The client IP keys the rate limits of sign-in, registration, the client API, and the inbox. When the connection's peer is in this list, the client IP is the rightmost address of the header that is not in it; otherwise the header is ignored and the client IP is the peer. Empty trusts no one: behind a proxy every request then shares the proxy's limits. List only addresses that nothing but your proxies can connect from, and only proxies that append to the header rather than pass a client's value through (docs/architecture/deployment-targets.md).

- Environment variable: `MY_TRUSTED_PROXIES`
- Default: none
- Required: no

### `trusted_proxy_header`

Forwarding header that the `trusted_proxies` append the client IP to. Only this header is read, so that a client cannot add the other one past a proxy that does not know it.

- Environment variable: `MY_TRUSTED_PROXY_HEADER`
- Default: `x-forwarded-for`
- Required: no

| Value | In v0 | Description |
| --- | --- | --- |
| `x-forwarded-for` | yes | `X-Forwarded-For`, which nginx, Caddy, Traefik, Google Cloud load balancers and Cloud Run, and AWS Application Load Balancers append to. |
| `forwarded` | yes | The `for` parameters of `Forwarded` (RFC 7239). |

### `db`

Database adapter (ADR-0003). The adapter is never inferred from `database_url`.

- Environment variable: `MY_DB`
- Default: `sqlite`
- Required: no

| Value | In v0 | Description |
| --- | --- | --- |
| `sqlite` | yes | Local SQLite file (`modernc.org/sqlite`). |
| `libsql` | yes | libSQL / Turso. |
| `postgres` | yes | PostgreSQL and compatible services (`pgx/v5`). |

### `database_url`

Connection string passed to the `db` adapter, in that adapter's format.

- Environment variable: `MY_DATABASE_URL` or `MY_DATABASE_URL_FILE`
- Default: none
- Required: yes

### `queue`

Job queue adapter (ADR-0004).

- Environment variable: `MY_QUEUE`
- Default: `db`
- Required: no

| Value | In v0 | Description |
| --- | --- | --- |
| `db` | yes | Queue table in the database. |
| `inline` | yes | Runs jobs synchronously in the enqueuing process; for tests and development. Requires `role = all`. |
| `sqs` | yes | Amazon SQS. |
| `cloudtasks` | yes | Google Cloud Tasks. |

### `blob`

Blob storage adapter for media (ADR-0005).

- Environment variable: `MY_BLOB`
- Default: `fs`
- Required: no

| Value | In v0 | Description |
| --- | --- | --- |
| `fs` | yes | Local file system. |
| `s3` | yes | S3-compatible API (AWS S3, Cloudflare R2, GCS XML API, MinIO). |
| `gcs` | not in v0 | Native Google Cloud Storage API. |

### `media`

Media processor adapter (ADR-0005).

- Environment variable: `MY_MEDIA`
- Default: `native`
- Required: no

| Value | In v0 | Description |
| --- | --- | --- |
| `native` | yes | Pure Go image processing. |
| `vips` | yes | libvips; needs a cgo build with the adapter's build tag. |
| `none` | yes | No processing except EXIF removal. |

### `blob_fs_dir`

Directory of the `fs` blob store, relative to the working directory or absolute. It holds the uploads as received (private) and the processed media, which the server serves under `/media/`. Share it between the web and worker processes, and back it up with the database.

- Environment variable: `MY_BLOB_FS_DIR`
- Default: `blobs`
- Required: no

### `media_max_image_bytes`

Largest accepted image upload in bytes (ADR-0019 §8).

- Environment variable: `MY_MEDIA_MAX_IMAGE_BYTES`
- Default: `16777216`
- Required: no

### `media_max_image_side`

Largest accepted image width and height in pixels, read from the file's header before decoding (ADR-0019 §8).

- Environment variable: `MY_MEDIA_MAX_IMAGE_SIDE`
- Default: `8192`
- Required: no

### `media_max_image_pixels`

Largest accepted image area (width × height, and for an animated GIF the frames together), read from the file's header before decoding (ADR-0019 §8). The default is 33 MP (7680 × 4320).

- Environment variable: `MY_MEDIA_MAX_IMAGE_PIXELS`
- Default: `33177600`
- Required: no

### `media_unattached_ttl`

How long an upload may stay unattached to any post or profile before the hourly cleanup deletes it (FR-803), as a Go duration such as `24h`.

- Environment variable: `MY_MEDIA_UNATTACHED_TTL`
- Default: `24h`
- Required: no

### `remote_media_cache_max_bytes`

Size limit in bytes of the copies of remote media that the media proxy keeps when the instance setting `remote_media_policy` is `avatars` or `all` (FR-409). The hourly cleanup deletes the expired copies, then the least recently accessed ones over this limit (FR-803).

- Environment variable: `MY_REMOTE_MEDIA_CACHE_MAX_BYTES`
- Default: `1073741824`
- Required: no

### `kv`

Key-value store adapter for caches and rate limits (ADR-0003, ADR-0019 §5).

- Environment variable: `MY_KV`
- Default: `db`
- Required: no

| Value | In v0 | Description |
| --- | --- | --- |
| `db` | yes | Table in the database. |
| `memory` | yes | In-process memory; limits are per process. |
| `redis` | not in v0 | Redis. |

### `key_manager`

Key manager adapter that encrypts actor private keys (ADR-0019 §6).

- Environment variable: `MY_KEY_MANAGER`
- Default: `local`
- Required: no

| Value | In v0 | Description |
| --- | --- | --- |
| `local` | yes | Uses the key-encryption key from `kek`. |
| `gcpkms` | not in v0 | Google Cloud KMS. |
| `awskms` | not in v0 | AWS KMS. |

### `kek`

Key-encryption key for actor private keys: 32 random bytes in standard base64 (for example `openssl rand -base64 32`). Required when `key_manager = local`. Back it up separately from the database; losing it loses every actor key (ADR-0019 §6, docs/operations/key-management.md).

- Environment variable: `MY_KEK` or `MY_KEK_FILE`
- Default: none
- Required: when `key_manager = local`

### `kek_previous`

Previous key-encryption keys, comma-separated, each in the format of `kek`. They only decrypt; new secrets are always encrypted with `kek`. Set the replaced key here while rotating `kek` (docs/operations/key-management.md).

- Environment variable: `MY_KEK_PREVIOUS` or `MY_KEK_PREVIOUS_FILE`
- Default: none
- Required: no

### `mail`

Mailer adapter for verification emails (FR-002). With `none`, no email reaches users, so an unverified address does not keep a user out and sign-ups in the `open` registration mode wait for a moderator's approval instead.

- Environment variable: `MY_MAIL`
- Default: `none`
- Required: no

| Value | In v0 | Description |
| --- | --- | --- |
| `none` | yes | Logs each message, including its links, instead of sending it; for development and instances without email. |
| `smtp` | yes | SMTP submission with mandatory STARTTLS (`smtp_host`, `smtp_port`, `smtp_username`, `smtp_password`). |
| `ses` | not in v0 | Amazon SES. |

### `mail_from`

Sender address of emails, as a bare address such as `noreply@social.example`.

- Environment variable: `MY_MAIL_FROM`
- Default: none
- Required: when `mail` is not `none`

### `smtp_host`

Host name of the SMTP server. The server must offer STARTTLS with a certificate valid for this name; the connection is refused otherwise.

- Environment variable: `MY_SMTP_HOST`
- Default: none
- Required: when `mail = smtp`

### `smtp_port`

TCP port of the SMTP server (submission).

- Environment variable: `MY_SMTP_PORT`
- Default: `587`
- Required: no

### `smtp_username`

User name for SMTP AUTH PLAIN, sent only after STARTTLS. Leave it empty for a server that needs no authentication.

- Environment variable: `MY_SMTP_USERNAME`
- Default: none
- Required: no

### `smtp_password`

Password for SMTP AUTH PLAIN.

- Environment variable: `MY_SMTP_PASSWORD` or `MY_SMTP_PASSWORD_FILE`
- Default: none
- Required: no

### `captcha`

CAPTCHA service that the sign-in and registration forms check (ADR-0042). The service receives the visitor's IP address, browser details, and cookies, and the two pages load its script; say so in the instance's privacy policy. Only `none` keeps every request on this server.

- Environment variable: `MY_CAPTCHA`
- Default: `none`
- Required: no

| Value | In v0 | Description |
| --- | --- | --- |
| `none` | yes | No CAPTCHA; nothing is sent to a third party. |
| `turnstile` | yes | Cloudflare Turnstile. |
| `hcaptcha` | yes | hCaptcha. |
| `recaptcha` | yes | Google reCAPTCHA v2 with the checkbox (not v3 or Enterprise). |

### `captcha_site_key`

Public site key of the `captcha` service, which the widget on the forms carries.

- Environment variable: `MY_CAPTCHA_SITE_KEY`
- Default: none
- Required: when `captcha` is not `none`

### `captcha_secret_key`

Secret key of the `captcha` service, which the server sends with each token to the service's siteverify API. The server does not try the key at start: a key that the service refuses is logged as an error on the first check, and until it is fixed registrations are refused and sign-ins skip the CAPTCHA (ADR-0042 §4).

- Environment variable: `MY_CAPTCHA_SECRET_KEY` or `MY_CAPTCHA_SECRET_KEY_FILE`
- Default: none
- Required: when `captcha` is not `none`

### `profile`

Whether this is a production instance. Settings that weaken a security default for development, such as `http_allow_private_networks`, are refused in the `production` profile (ADR-0019).

- Environment variable: `MY_PROFILE`
- Default: `production`
- Required: no

| Value | In v0 | Description |
| --- | --- | --- |
| `production` | yes | A production instance. |
| `development` | yes | A development or test instance. |

### `http_allow_private_networks`

Lets outbound HTTP (federation, media, link previews) reach loopback, private, and other non-public addresses and host names, over plain `http` and on any port. For development and tests against local servers only: the server logs a warning at start, and the `production` profile refuses it (ADR-0019 §1).

- Environment variable: `MY_HTTP_ALLOW_PRIVATE_NETWORKS`
- Default: `false`
- Required: no

| Value | In v0 | Description |
| --- | --- | --- |
| `false` | yes | Only public `https` destinations on ports 443 and 80. |
| `true` | yes | Every destination; requires `profile = development`. |

### `remote_status_retention`

How long a remote post is kept after its publication when nothing on this instance refers to it (a local reply, boost, favourite, bookmark, notification, or a thread with a local post); the hourly purge then deletes it (ADR-0034, FR-803). A Go duration such as `2160h` (90 days).

- Environment variable: `MY_REMOTE_STATUS_RETENTION`
- Default: `2160h`
- Required: no

### `remote_status_max_retention`

The longest a remote post is kept after its publication, even when this instance refers to it; the hourly purge then deletes it, while local accounts' replies, boosts, favourites, and bookmarks stay and show it as deleted (ADR-0034). A Go duration such as `8760h` (365 days); empty keeps referred posts without limit.

- Environment variable: `MY_REMOTE_STATUS_MAX_RETENTION`
- Default: none
- Required: no
