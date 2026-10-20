// SPDX-License-Identifier: AGPL-3.0-only

package federation

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"math/big"
	"strings"
)

// MultikeyType is the type of a FEP-521a key object (Controlled Identifiers §2.2.2).
const MultikeyType = "Multikey"

// MultikeyDocument is a public key as an actor's assertionMethod publishes it (FEP-521a).
type MultikeyDocument struct {
	ID                 string `json:"id"`
	Type               string `json:"type"`
	Controller         IRI    `json:"controller"`
	PublicKeyMultibase string `json:"publicKeyMultibase"`
}

// multicodecEd25519 is the multicodec prefix of an Ed25519 public key (0xed, varint-encoded).
var multicodecEd25519 = []byte{0xed, 0x01}

// EncodeEd25519Multikey returns the publicKeyMultibase of an Ed25519 public key: base58btc
// ("z") of the Ed25519 multicodec prefix and the 32-byte key, which starts with "z6Mk".
func EncodeEd25519Multikey(pub ed25519.PublicKey) string {
	return "z" + encodeBase58(append(append([]byte{}, multicodecEd25519...), pub...))
}

// ParseEd25519Multikey decodes a publicKeyMultibase of an Ed25519 key (EncodeEd25519Multikey).
func ParseEd25519Multikey(s string) (ed25519.PublicKey, error) {
	rest, ok := strings.CutPrefix(s, "z")
	if !ok {
		return nil, errors.New("not base58btc")
	}
	b, err := decodeBase58(rest)
	if err != nil {
		return nil, err
	}
	if len(b) != len(multicodecEd25519)+ed25519.PublicKeySize || b[0] != multicodecEd25519[0] || b[1] != multicodecEd25519[1] {
		return nil, errors.New("not an Ed25519 multikey")
	}
	return ed25519.PublicKey(b[len(multicodecEd25519):]), nil
}

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// maxBase58Length bounds what decodeBase58 reads: a 64-byte signature is at most 88 characters.
const maxBase58Length = 128

// encodeBase58 encodes b in the Bitcoin base58 alphabet; leading zero bytes are leading "1"s.
func encodeBase58(b []byte) string {
	n := new(big.Int).SetBytes(b)
	var out []byte
	radix, mod := big.NewInt(58), new(big.Int)
	for n.Sign() > 0 {
		n.DivMod(n, radix, mod)
		out = append(out, base58Alphabet[mod.Int64()])
	}
	for _, c := range b {
		if c != 0 {
			break
		}
		out = append(out, '1')
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return string(out)
}

// decodeBase58 decodes the Bitcoin base58 alphabet; leading "1"s are leading zero bytes.
func decodeBase58(s string) ([]byte, error) {
	if len(s) > maxBase58Length {
		return nil, errors.New("base58 value too long")
	}
	n := new(big.Int)
	radix := big.NewInt(58)
	zeros := 0
	for zeros < len(s) && s[zeros] == '1' {
		zeros++
	}
	for _, c := range []byte(s) {
		i := strings.IndexByte(base58Alphabet, c)
		if i < 0 {
			return nil, fmt.Errorf("invalid base58 character %q", c)
		}
		n.Mul(n, radix)
		n.Add(n, big.NewInt(int64(i)))
	}
	return append(make([]byte, zeros), n.Bytes()...), nil
}
