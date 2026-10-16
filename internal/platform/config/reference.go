// SPDX-License-Identifier: AGPL-3.0-only

package config

import (
	"fmt"
	"strings"
)

// ReferencePath is the generated reference, relative to the repository root.
const ReferencePath = "docs/reference/configuration.md"

// Reference renders the configuration reference from the schema as Markdown.
func Reference() string {
	var b strings.Builder
	b.WriteString(`<!-- Code generated from internal/platform/config/schema.go by "make config-docs". DO NOT EDIT. -->

# Configuration reference

The server reads each setting from these sources, highest precedence first (ADR-0010):

1. The environment variable ` + "`" + EnvPrefix + "<KEY>`" + ` (the key in upper case).
2. The TOML file named by ` + "`" + EnvConfigFile + "`" + ` (or ` + "`my config check -config <path>`" + `), with the keys below at the top level.
3. The default.

Secrets are read only from the environment: either the variable itself or ` + "`<NAME>_FILE`" + `, the path of a file that holds the value
(trailing newlines are removed), for Docker and Kubernetes secrets (ADR-0019 §6). Setting a secret in the TOML file is an error.

Adapters are always selected by an explicit key; nothing is inferred from a URL scheme.
Values marked "not in v0" are decided in an ADR but not shipped in v0, and are rejected.

` + "`my config check`" + ` validates the configuration and lists every problem at once; the server refuses to start on any of them.

## Summary

| Key | Environment variable | Default | Required | Secret |
| --- | --- | --- | --- | --- |
`)
	for i := range schema {
		f := &schema[i]
		fmt.Fprintf(&b, "| [`%s`](#%s) | `%s` | %s | %s | %s |\n",
			f.key, f.key, f.env(), defaultCell(f), requiredCell(f), yesNo(f.secret))
	}
	b.WriteString("\n## Settings\n")
	for i := range schema {
		f := &schema[i]
		fmt.Fprintf(&b, "\n### `%s`\n\n%s\n\n", f.key, f.doc)
		fmt.Fprintf(&b, "- Environment variable: `%s`", f.env())
		if f.secret {
			fmt.Fprintf(&b, " or `%s_FILE`", f.env())
		}
		fmt.Fprintf(&b, "\n- Default: %s\n- Required: %s\n", defaultCell(f), requiredCell(f))
		if len(f.choices) > 0 {
			b.WriteString("\n| Value | In v0 | Description |\n| --- | --- | --- |\n")
			for _, c := range f.choices {
				inV0 := "yes"
				if !c.inV0 {
					inV0 = "not in v0"
				}
				fmt.Fprintf(&b, "| `%s` | %s | %s |\n", c.value, inV0, c.doc)
			}
		}
	}
	return b.String()
}

func defaultCell(f *field) string {
	switch {
	case f.def != "":
		return "`" + f.def + "`"
	case f.defDoc != "":
		return f.defDoc
	default:
		return "none"
	}
}

func requiredCell(f *field) string {
	if f.requiredWhen != "" {
		return f.requiredWhen
	}
	return yesNo(f.required)
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
