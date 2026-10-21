#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only
#
# Checks the build tags that leave adapters out of the server (NFR-PORT-4, ADR-0010, ADR-0015 Build matrix;
# docs/reference/build-tags.md). For each representative set of tags, it builds every package with
# CGO_ENABLED=0, runs go vet, and fails when cmd/my still depends on a package that a tag of the set leaves out.
#
#   scripts/go-build-tags.sh          every set below
#   scripts/go-build-tags.sh SET...   the given comma-separated sets of tags
set -euo pipefail

# The representative sets: together they use every tag.
sets=(
  # A self-hosted instance on SQLite, without a broker or a metrics exporter.
  "no_postgres,no_valkey,no_sns,no_pubsub,no_otel,no_billing_aws"
  # AWS: PostgreSQL, the SNS broker, the EMF metrics.
  "no_sqlite,no_valkey,no_pubsub,no_otel"
  # Google Cloud: PostgreSQL, the Pub/Sub broker.
  "no_sqlite,no_valkey,no_sns,no_billing_aws"
)
if (($# > 0)); then
  sets=("$@")
fi

# excluded_packages TAG prints the packages that TAG leaves out of cmd/my: the adapters and their client library.
excluded_packages() {
  local m=github.com/best-friends/my/internal/adapter
  case "$1" in
    no_sqlite) echo "$m/db/sqlite modernc.org/sqlite" ;;
    no_postgres) echo "$m/db/postgres $m/stream/postgres github.com/jackc/pgx/v5" ;;
    no_valkey) echo "$m/valkey $m/kv/valkey $m/stream/valkey github.com/valkey-io/valkey-go" ;;
    no_sns) echo "$m/stream/sns github.com/aws/aws-sdk-go-v2/service/sns" ;;
    no_pubsub) echo "$m/stream/pubsub cloud.google.com/go/pubsub/v2" ;;
    no_billing_aws) echo "$m/billing/aws github.com/aws/aws-sdk-go-v2/service/costexplorer" ;;
    no_otel) echo "$m/telemetry/otel $m/telemetry/otel/otlptest go.opentelemetry.io/otel/sdk/metric go.opentelemetry.io/otel/sdk/trace go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp" ;;
    *)
      echo "unknown build tag: $1 (see docs/reference/build-tags.md)" >&2
      return 1
      ;;
  esac
}

status=0
for set in "${sets[@]}"; do
  echo "--- -tags $set"
  CGO_ENABLED=0 go build -tags "$set" ./...
  go vet -tags "$set" ./...
  deps="$(go list -deps -tags "$set" ./cmd/my)"
  IFS=, read -ra tags <<<"$set"
  for tag in "${tags[@]}"; do
    pkgs="$(excluded_packages "$tag")"
    for pkg in $pkgs; do
      if grep -qFx "$pkg" <<<"$deps"; then
        echo "cmd/my built with -tags $set still depends on $pkg, which $tag leaves out" >&2
        status=1
      fi
    done
  done
done
exit "$status"
