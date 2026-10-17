// SPDX-License-Identifier: AGPL-3.0-only

package federation

import (
	"strings"
	"time"
	"unicode/utf8"
)

// MaxSoftwareLength bounds the software name and version kept from a NodeInfo document, in
// bytes.
const MaxSoftwareLength = 255

// NodeInfoObservation is the software a remote server reported in its NodeInfo
// (the instance_nodeinfo_observations event). Domain is a canonical host.
type NodeInfoObservation struct {
	ID              string
	Domain          string
	SoftwareName    string
	SoftwareVersion string
	ObservedAt      time.Time
}

// SameSoftware reports whether o reports the same software as p.
func (o NodeInfoObservation) SameSoftware(p NodeInfoObservation) bool {
	return o.SoftwareName == p.SoftwareName && o.SoftwareVersion == p.SoftwareVersion
}

// NormalizeSoftware returns the stored form of a NodeInfo software name or version: trimmed,
// without control characters, at most MaxSoftwareLength bytes of valid UTF-8. Names are also
// lower-cased, as NodeInfo 2.x requires them to be.
func NormalizeSoftware(s string, name bool) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r == utf8.RuneError {
			return -1
		}
		return r
	}, strings.TrimSpace(s))
	if name {
		s = strings.ToLower(s)
	}
	for len(s) > MaxSoftwareLength {
		_, size := utf8.DecodeLastRuneInString(s)
		s = s[:len(s)-size]
	}
	return s
}
