// SPDX-License-Identifier: AGPL-3.0-only

package federation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// maxJCSDepth bounds the nesting of a document that CanonicalJSON accepts; the inbox already
// bounds the size (1 MiB).
const maxJCSDepth = 64

// ErrNotCanonicalizable reports a JSON text that the JSON Canonicalization Scheme does not
// accept: invalid JSON or UTF-8, a duplicate member name, a number outside IEEE 754 double
// precision, or nesting deeper than maxJCSDepth.
var ErrNotCanonicalizable = errors.New("federation: JSON cannot be canonicalized")

// CanonicalJSON returns the JSON Canonicalization Scheme (RFC 8785) form of a JSON text: no
// whitespace, object members sorted by the UTF-16 code units of their names, strings with only
// the required escapes, and numbers serialized as ECMAScript does. It is the canonicalization
// of the eddsa-jcs-2022 cryptosuite (FEP-8b32).
func CanonicalJSON(data []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var out bytes.Buffer
	if err := canonicalValue(dec, &out, 0); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: data after the value", ErrNotCanonicalizable)
	}
	return out.Bytes(), nil
}

// canonicalValue writes the canonical form of the next value of dec.
func canonicalValue(dec *json.Decoder, out *bytes.Buffer, depth int) error {
	if depth > maxJCSDepth {
		return fmt.Errorf("%w: nested deeper than %d", ErrNotCanonicalizable, maxJCSDepth)
	}
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrNotCanonicalizable, err)
	}
	switch t := tok.(type) {
	case json.Delim:
		if t == '[' {
			return canonicalArray(dec, out, depth)
		}
		if t == '{' {
			return canonicalObject(dec, out, depth)
		}
		return fmt.Errorf("%w: unexpected %v", ErrNotCanonicalizable, t)
	case string:
		return canonicalString(out, t)
	case json.Number:
		return canonicalNumber(out, t)
	case bool:
		out.WriteString(strconv.FormatBool(t))
	case nil:
		out.WriteString("null")
	}
	return nil
}

func canonicalArray(dec *json.Decoder, out *bytes.Buffer, depth int) error {
	out.WriteByte('[')
	for i := 0; dec.More(); i++ {
		if i > 0 {
			out.WriteByte(',')
		}
		if err := canonicalValue(dec, out, depth+1); err != nil {
			return err
		}
	}
	if _, err := dec.Token(); err != nil {
		return fmt.Errorf("%w: %w", ErrNotCanonicalizable, err)
	}
	out.WriteByte(']')
	return nil
}

func canonicalObject(dec *json.Decoder, out *bytes.Buffer, depth int) error {
	type member struct {
		name  string
		key   []uint16
		value []byte
	}
	var members []member
	seen := map[string]bool{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("%w: %w", ErrNotCanonicalizable, err)
		}
		name, ok := tok.(string)
		if !ok {
			return fmt.Errorf("%w: member name %v", ErrNotCanonicalizable, tok)
		}
		if seen[name] {
			return fmt.Errorf("%w: duplicate member %q", ErrNotCanonicalizable, name)
		}
		seen[name] = true
		var v bytes.Buffer
		if err := canonicalValue(dec, &v, depth+1); err != nil {
			return err
		}
		members = append(members, member{name: name, key: utf16.Encode([]rune(name)), value: v.Bytes()})
	}
	if _, err := dec.Token(); err != nil {
		return fmt.Errorf("%w: %w", ErrNotCanonicalizable, err)
	}
	slices.SortFunc(members, func(a, b member) int { return slices.Compare(a.key, b.key) })
	out.WriteByte('{')
	for i, m := range members {
		if i > 0 {
			out.WriteByte(',')
		}
		if err := canonicalString(out, m.name); err != nil {
			return err
		}
		out.WriteByte(':')
		out.Write(m.value)
	}
	out.WriteByte('}')
	return nil
}

// canonicalString writes s as RFC 8785 §3.2.2.2 serializes strings: the short escapes for
// backspace, tab, line feed, form feed, carriage return, quotation mark, and reverse solidus,
// \u00xx for the other control characters, and every other character as it is.
func canonicalString(out *bytes.Buffer, s string) error {
	if !utf8.ValidString(s) {
		return fmt.Errorf("%w: invalid UTF-8", ErrNotCanonicalizable)
	}
	out.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\b':
			out.WriteString(`\b`)
		case '\t':
			out.WriteString(`\t`)
		case '\n':
			out.WriteString(`\n`)
		case '\f':
			out.WriteString(`\f`)
		case '\r':
			out.WriteString(`\r`)
		case '"':
			out.WriteString(`\"`)
		case '\\':
			out.WriteString(`\\`)
		default:
			if r < 0x20 {
				fmt.Fprintf(out, `\u%04x`, r)
				continue
			}
			out.WriteRune(r)
		}
	}
	out.WriteByte('"')
	return nil
}

// canonicalNumber writes n as an IEEE 754 double serialized by ECMAScript's Number.prototype.
// toString (RFC 8785 §3.2.2.3).
func canonicalNumber(out *bytes.Buffer, n json.Number) error {
	f, err := strconv.ParseFloat(string(n), 64)
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
		return fmt.Errorf("%w: number %s", ErrNotCanonicalizable, n)
	}
	out.WriteString(ecmaNumber(f))
	return nil
}

// ecmaNumber formats a finite f as ECMAScript's Number::toString does (ECMA-262 §6.1.6.1.20):
// the shortest digits that round-trip, in positional notation for exponents in [-7, 21) and in
// exponential notation otherwise.
func ecmaNumber(f float64) string {
	if f == 0 {
		return "0" // also for -0
	}
	sign := ""
	if f < 0 {
		sign, f = "-", -f
	}
	// d.ddde±x: digits and the exponent of the shortest representation.
	e := strconv.FormatFloat(f, 'e', -1, 64)
	mantissa, exp, _ := strings.Cut(e, "e")
	digits := strings.Replace(mantissa, ".", "", 1)
	x, _ := strconv.Atoi(exp)
	k := len(digits)
	n := x + 1 // the position of the decimal point after the first n digits
	var s string
	switch {
	case k <= n && n <= 21:
		s = digits + strings.Repeat("0", n-k)
	case 0 < n && n <= 21:
		s = digits[:n] + "." + digits[n:]
	case -6 < n && n <= 0:
		s = "0." + strings.Repeat("0", -n) + digits
	default:
		s = digits[:1]
		if k > 1 {
			s += "." + digits[1:]
		}
		if n-1 >= 0 {
			s += "e+" + strconv.Itoa(n-1)
		} else {
			s += "e-" + strconv.Itoa(1-n)
		}
	}
	return sign + s
}
