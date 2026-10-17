// SPDX-License-Identifier: AGPL-3.0-only

package httpclient

import (
	"encoding/binary"
	"net/netip"
	"strconv"
	"strings"
)

// deniedIPv4 lists the IPv4 special-purpose ranges that ADR-0019 §1 refuses on top of
// addresses that are not global unicast (IANA IPv4 Special-Purpose Address Registry).
var deniedIPv4 = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("255.255.255.255/32"),
}

// deniedIPv6 lists the IPv6 special-purpose ranges that ADR-0019 §1 refuses (IANA IPv6
// Special-Purpose Address Registry). The ranges that embed an IPv4 address are not here:
// allowedAddr checks the embedded address against deniedIPv4 instead.
var deniedIPv6 = []netip.Prefix{
	// ::/96 also covers ::/128, ::1/128, and the deprecated IPv4-compatible addresses
	// (RFC 4291 §2.5.5.1), which no public host uses.
	netip.MustParsePrefix("::/96"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("fec0::/10"),
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("ff00::/8"),
}

var (
	nat64Prefix      = netip.MustParsePrefix("64:ff9b::/96")
	nat64LocalPrefix = netip.MustParsePrefix("64:ff9b:1::/48")
	sixToFourPrefix  = netip.MustParsePrefix("2002::/16")
)

// allowedAddr reports whether a is a public address: global unicast, outside the denied
// ranges, and, for an IPv6 address that embeds an IPv4 address, embedding a public one.
func allowedAddr(a netip.Addr) bool {
	if a.Zone() != "" {
		return false
	}
	a = a.Unmap()
	if !a.IsGlobalUnicast() {
		return false
	}
	if a.Is4() {
		return !inAny(a, deniedIPv4)
	}
	if v4, ok := embeddedIPv4(a); ok {
		return allowedAddr(v4)
	}
	return !inAny(a, deniedIPv6)
}

func inAny(a netip.Addr, prefixes []netip.Prefix) bool {
	for _, p := range prefixes {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// embeddedIPv4 returns the IPv4 address embedded in a NAT64 (RFC 6052 §2.2) or 6to4
// (RFC 3056 §2) address.
func embeddedIPv4(a netip.Addr) (netip.Addr, bool) {
	b := a.As16()
	switch {
	case nat64Prefix.Contains(a):
		return netip.AddrFrom4([4]byte{b[12], b[13], b[14], b[15]}), true
	case nat64LocalPrefix.Contains(a):
		// A /48 prefix puts the IPv4 address in bits 48-63 and 72-87; bits 64-71 are the u octet.
		return netip.AddrFrom4([4]byte{b[6], b[7], b[9], b[10]}), true
	case sixToFourPrefix.Contains(a):
		return netip.AddrFrom4([4]byte{b[2], b[3], b[4], b[5]}), true
	}
	return netip.Addr{}, false
}

// deniedHostNames are refused before name resolution (ADR-0019 §1): the cloud metadata
// services and the names that resolve only inside a network.
var deniedHostNames = []string{"localhost", "metadata", "metadata.google.internal"}

var deniedHostSuffixes = []string{".internal", ".local", ".localhost"}

// deniedHostName reports whether the lower-cased host name without a trailing dot is
// refused. Single-label names are refused too: resolvers complete them with the search
// domains of the local network (for example a Kubernetes service name).
func deniedHostName(name string) bool {
	for _, n := range deniedHostNames {
		if name == n {
			return true
		}
	}
	for _, s := range deniedHostSuffixes {
		if strings.HasSuffix(name, s) {
			return true
		}
	}
	return !strings.Contains(name, ".")
}

// parseHostAddr parses a URL host that is an IP address: an IPv6 literal (without brackets),
// a dotted-decimal IPv4 address, or one of the IPv4 forms that inet_aton and the WHATWG URL
// standard accept (http://2130706433/, http://0x7f.1/, http://0177.0.0.1/). numeric reports
// whether host is meant as an address at all: its last label is a number. A numeric host that
// does not parse is malformed.
func parseHostAddr(host string) (addr netip.Addr, numeric, ok bool) {
	if a, err := netip.ParseAddr(host); err == nil {
		return a, true, true
	}
	labels := strings.Split(strings.TrimSuffix(host, "."), ".")
	if !numericLabel(labels[len(labels)-1]) {
		return netip.Addr{}, false, false
	}
	if len(labels) > 4 {
		return netip.Addr{}, true, false
	}
	parts := make([]uint64, len(labels))
	for i, l := range labels {
		n, ok := parseIPv4Part(l)
		if !ok {
			return netip.Addr{}, true, false
		}
		parts[i] = n
	}
	var v uint64
	for i, p := range parts[:len(parts)-1] {
		if p > 0xff {
			return netip.Addr{}, true, false
		}
		v |= p << (24 - 8*i)
	}
	last := parts[len(parts)-1]
	if last >= uint64(1)<<(8*(5-len(parts))) {
		return netip.Addr{}, true, false
	}
	v |= last
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], uint32(v)) // #nosec G115 -- v < 1<<32: the parts are bounded above
	return netip.AddrFrom4(b), true, true
}

// numericLabel reports whether l is a number in the sense of the WHATWG URL standard's
// "ends in a number": decimal digits, or 0x followed by hexadecimal digits.
func numericLabel(l string) bool {
	if len(l) >= 2 && (l[:2] == "0x" || l[:2] == "0X") {
		return strings.Trim(l[2:], "0123456789abcdefABCDEF") == ""
	}
	return l != "" && strings.Trim(l, "0123456789") == ""
}

// parseIPv4Part parses one part of an inet_aton address: decimal, octal with a leading 0,
// or hexadecimal with a leading 0x.
func parseIPv4Part(s string) (uint64, bool) {
	base := 10
	switch {
	case len(s) >= 2 && (s[:2] == "0x" || s[:2] == "0X"):
		s, base = s[2:], 16
		if s == "" {
			return 0, true
		}
	case len(s) >= 2 && s[0] == '0':
		s, base = s[1:], 8
	}
	if s == "" || strings.ContainsAny(s, "+-_") {
		return 0, false
	}
	n, err := strconv.ParseUint(s, base, 32)
	return n, err == nil
}
