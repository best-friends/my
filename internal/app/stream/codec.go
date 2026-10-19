// SPDX-License-Identifier: AGPL-3.0-only

package stream

import (
	"encoding/json"
	"errors"
	"fmt"
)

// signalVersion is the version of the payload format that encodeSignal writes.
const signalVersion = 1

// Errors of decodeSignal.
var (
	// errUnknownVersion reports a payload of a version this process does not know, written by
	// a newer process during a rolling deployment; the payload is dropped.
	errUnknownVersion = errors.New("stream: unknown signal version")
	// errMalformedSignal reports a payload that is not a signal of a known kind.
	errMalformedSignal = errors.New("stream: malformed signal")
)

// signal is the wire format of a signal on a Bus (ADR-0050 §1):
// {"v":1,"o":"<process id>","k":"home_entry","e":"<entry id>"} or
// {"v":1,"o":"<process id>","k":"notifications","a":"<account id>"}.
type signal struct {
	Version   int    `json:"v"`
	Origin    string `json:"o"`
	Kind      Kind   `json:"k"`
	AccountID string `json:"a,omitempty"`
	EntryID   string `json:"e,omitempty"`
}

// encodeSignal encodes e as published by the process origin. It fails for a payload over
// MaxPayloadSize, which no ID of the server reaches.
func encodeSignal(origin string, e Event) ([]byte, error) {
	b, err := json.Marshal(signal{Version: signalVersion, Origin: origin, Kind: e.Kind, AccountID: e.AccountID, EntryID: e.EntryID})
	if err != nil {
		return nil, err
	}
	if len(b) > MaxPayloadSize {
		return nil, fmt.Errorf("stream: a %s signal of %d bytes exceeds %d bytes", e.Kind, len(b), MaxPayloadSize)
	}
	return b, nil
}

// decodeSignal decodes a payload into the signal and the process that published it. A payload
// of another version reports errUnknownVersion; one that is not a known kind with its ID,
// errMalformedSignal.
func decodeSignal(payload []byte) (origin string, e Event, err error) {
	var s signal
	if err := json.Unmarshal(payload, &s); err != nil {
		return "", Event{}, errors.Join(errMalformedSignal, err)
	}
	if s.Version != signalVersion {
		return "", Event{}, fmt.Errorf("%w: %d", errUnknownVersion, s.Version)
	}
	switch {
	case s.Kind == KindHomeEntry && s.EntryID != "":
		return s.Origin, Event{Kind: KindHomeEntry, EntryID: s.EntryID}, nil
	case s.Kind == KindNotifications && s.AccountID != "":
		return s.Origin, Event{Kind: KindNotifications, AccountID: s.AccountID}, nil
	}
	return "", Event{}, fmt.Errorf("%w: kind %q", errMalformedSignal, s.Kind)
}
