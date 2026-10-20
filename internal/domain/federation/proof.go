// SPDX-License-Identifier: AGPL-3.0-only

package federation

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"
)

// This file holds Object Integrity Proofs (FEP-8b32): a DataIntegrityProof of the
// eddsa-jcs-2022 cryptosuite (W3C Data Integrity EdDSA Cryptosuites v1.0 §3.3) under the key
// proof of an activity, made with the Ed25519 key that the actor's assertionMethod publishes
// (FEP-521a).

// The values of an Object Integrity Proof.
const (
	// DataIntegrityContext is the JSON-LD context that defines proof and its terms.
	DataIntegrityContext = "https://w3id.org/security/data-integrity/v2"
	// ControlledIdentifierContext is the JSON-LD context that defines assertionMethod and
	// Multikey (FEP-521a).
	ControlledIdentifierContext = "https://www.w3.org/ns/cid/v1"
	// ProofType is the type of every proof (Data Integrity §3.1).
	ProofType = "DataIntegrityProof"
	// CryptosuiteEdDSAJCS2022 is the cryptosuite that FEP-8b32 recommends: JCS, SHA-256, and
	// Ed25519.
	CryptosuiteEdDSAJCS2022 = "eddsa-jcs-2022"
	// ProofPurposeAssertionMethod is the purpose of every proof (FEP-8b32).
	ProofPurposeAssertionMethod = "assertionMethod"
)

var (
	// ErrNoProof reports a document without a proof that FEP-8b32 tells a verifier to check:
	// no proof, more than one, or one of another type, cryptosuite, or purpose, or past its
	// expiry. Verifiers ignore such a proof and rely on the HTTP signature.
	ErrNoProof = errors.New("federation: no usable integrity proof")
	// ErrProofMismatch reports a proof that does not verify with the key it names.
	ErrProofMismatch = errors.New("federation: integrity proof does not verify")
)

// Proof is the proof of a document as FEP-8b32 reads it.
type Proof struct {
	VerificationMethod string
	Created            string
	// value is the decoded proofValue, options the proof without it, and context its @context
	// (nil when it has none).
	value   []byte
	options map[string]json.RawMessage
	context json.RawMessage
}

// SignEdDSAJCS2022 returns document with an eddsa-jcs-2022 proof made with key, named by
// verificationMethod, created at created (Data Integrity §4.2 Add Proof, vc-di-eddsa §3.3.1).
// It adds DataIntegrityContext to the document's @context when it is missing, and the proof
// carries the same @context, as the examples of FEP-8b32 do. A proof the document had is
// replaced. The output has its members in JCS order; the signature does not depend on it.
func SignEdDSAJCS2022(document []byte, key ed25519.PrivateKey, verificationMethod string, created time.Time) ([]byte, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(document, &doc); err != nil || doc == nil {
		return nil, fmt.Errorf("federation: sign a document that is not a JSON object: %w", err)
	}
	delete(doc, "proof")
	ctx, err := withDataIntegrityContext(doc["@context"])
	if err != nil {
		return nil, err
	}
	doc["@context"] = ctx
	options := map[string]json.RawMessage{"@context": ctx}
	for k, v := range map[string]string{
		"type": ProofType, "cryptosuite": CryptosuiteEdDSAJCS2022, "verificationMethod": verificationMethod,
		"proofPurpose": ProofPurposeAssertionMethod, "created": FormatTime(created),
	} {
		options[k], _ = json.Marshal(v)
	}
	hash, err := proofHash(doc, options)
	if err != nil {
		return nil, err
	}
	options["proofValue"], _ = json.Marshal("z" + encodeBase58(ed25519.Sign(key, hash)))
	if doc["proof"], err = marshalRaw(options); err != nil {
		return nil, err
	}
	return marshalRaw(doc)
}

// marshalRaw encodes v as JSON without escaping HTML characters, so that the content of a
// signed document stays as it was built.
func marshalRaw(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(b.Bytes(), []byte("\n")), nil
}

// withDataIntegrityContext returns the @context ctx with DataIntegrityContext appended unless it
// has it: a missing context becomes [DataIntegrityContext], one value becomes an array.
func withDataIntegrityContext(ctx json.RawMessage) (json.RawMessage, error) {
	var entries []json.RawMessage
	if len(ctx) > 0 {
		if err := json.Unmarshal(ctx, &entries); err != nil {
			entries = []json.RawMessage{ctx}
		}
	}
	for _, e := range entries {
		var s string
		if json.Unmarshal(e, &s) == nil && s == DataIntegrityContext {
			return ctx, nil
		}
	}
	di, _ := json.Marshal(DataIntegrityContext)
	return json.Marshal(append(entries, di))
}

// proofHash returns the data that eddsa-jcs-2022 signs: the SHA-256 of the canonical proof
// options followed by the SHA-256 of the canonical document (vc-di-eddsa §3.3.4).
func proofHash(doc, options map[string]json.RawMessage) ([]byte, error) {
	canonical := func(v any) ([]byte, error) {
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		return CanonicalJSON(b)
	}
	opts, err := canonical(options)
	if err != nil {
		return nil, err
	}
	data, err := canonical(doc)
	if err != nil {
		return nil, err
	}
	o, d := sha256.Sum256(opts), sha256.Sum256(data)
	return append(o[:], d[:]...), nil
}

// rawProof is the part of a proof that ReadProof checks.
type rawProof struct {
	Type               string `json:"type"`
	Cryptosuite        string `json:"cryptosuite"`
	VerificationMethod string `json:"verificationMethod"`
	ProofPurpose       string `json:"proofPurpose"`
	ProofValue         string `json:"proofValue"`
	Created            string `json:"created"`
	Expires            string `json:"expires"`
}

// ReadProof returns the top-level proof of document that a verifier checks (FEP-8b32): one
// DataIntegrityProof of the eddsa-jcs-2022 cryptosuite for assertionMethod, with a
// verificationMethod and a base58btc proofValue, not expired at now. It fails with ErrNoProof
// otherwise, and with ErrProofMismatch when its proofValue is not a signature.
func ReadProof(document []byte, now time.Time) (Proof, error) {
	var doc struct {
		Proof json.RawMessage `json:"proof"`
	}
	if err := json.Unmarshal(document, &doc); err != nil || len(doc.Proof) == 0 || doc.Proof[0] != '{' {
		return Proof{}, ErrNoProof
	}
	var raw rawProof
	var options map[string]json.RawMessage
	if json.Unmarshal(doc.Proof, &raw) != nil || json.Unmarshal(doc.Proof, &options) != nil {
		return Proof{}, ErrNoProof
	}
	if raw.Type != ProofType || raw.Cryptosuite != CryptosuiteEdDSAJCS2022 || raw.ProofPurpose != ProofPurposeAssertionMethod ||
		raw.VerificationMethod == "" || raw.ProofValue == "" {
		return Proof{}, ErrNoProof
	}
	if raw.Expires != "" {
		if exp, err := time.Parse(time.RFC3339, raw.Expires); err == nil && exp.Before(now) {
			return Proof{}, ErrNoProof
		}
	}
	rest, ok := cutMultibase(raw.ProofValue)
	if !ok {
		return Proof{}, fmt.Errorf("%w: proofValue is not base58btc", ErrProofMismatch)
	}
	value, err := decodeBase58(rest)
	if err != nil || len(value) != ed25519.SignatureSize {
		return Proof{}, fmt.Errorf("%w: proofValue is not an Ed25519 signature", ErrProofMismatch)
	}
	delete(options, "proofValue")
	return Proof{
		VerificationMethod: raw.VerificationMethod, Created: raw.Created,
		value: value, options: options, context: options["@context"],
	}, nil
}

func cutMultibase(s string) (string, bool) {
	if len(s) < 2 || s[0] != 'z' {
		return "", false
	}
	return s[1:], true
}

// Verify checks that p, read from document by ReadProof, is a signature of document by pub
// (vc-di-eddsa §3.3.2 Verify Proof). The document is taken without its proof and without the
// linked data signature (signature) that FEP-8b32 tells a verifier to remove first. When the
// proof has an @context, the document's @context must start with it, and the document is
// hashed with the proof's @context. It fails with ErrProofMismatch.
func (p Proof) Verify(document []byte, pub ed25519.PublicKey) error {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(document, &doc); err != nil || doc == nil {
		return fmt.Errorf("%w: not a JSON object", ErrProofMismatch)
	}
	delete(doc, "proof")
	delete(doc, "signature")
	if p.context != nil {
		if !contextStartsWith(doc["@context"], p.context) {
			return fmt.Errorf("%w: the document's @context does not start with the proof's", ErrProofMismatch)
		}
		doc["@context"] = p.context
	}
	hash, err := proofHash(doc, p.options)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrProofMismatch, err)
	}
	if !ed25519.Verify(pub, hash, p.value) {
		return ErrProofMismatch
	}
	return nil
}

// contextStartsWith reports whether the @context doc starts with the values of the @context
// prefix, in order, compared by their canonical JSON.
func contextStartsWith(doc, prefix json.RawMessage) bool {
	list := func(raw json.RawMessage) ([][]byte, bool) {
		var entries []json.RawMessage
		if err := json.Unmarshal(raw, &entries); err != nil {
			entries = []json.RawMessage{raw}
		}
		out := make([][]byte, len(entries))
		for i, e := range entries {
			c, err := CanonicalJSON(e)
			if err != nil {
				return nil, false
			}
			out[i] = c
		}
		return out, true
	}
	d, ok := list(doc)
	if !ok || len(doc) == 0 {
		return false
	}
	p, ok := list(prefix)
	if !ok || len(p) > len(d) {
		return false
	}
	return slices.EqualFunc(p, d[:len(p)], func(a, b []byte) bool { return string(a) == string(b) })
}
