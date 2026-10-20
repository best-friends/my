// SPDX-License-Identifier: AGPL-3.0-only

package native

import (
	"context"
	"fmt"

	"github.com/best-friends/my/internal/app/media"
)

// InspectAV implements media.AVProcessor. It parses the whole container, which reads no
// samples, and so costs about as much as Remux without the copy.
func (p *Processor) InspectAV(ctx context.Context, data []byte, limits media.AVLimits) (media.AVProbe, error) {
	out, err := p.av(ctx, data, limits, false)
	return out.AVProbe, err
}

// Remux implements media.AVProcessor.
func (p *Processor) Remux(ctx context.Context, data []byte, limits media.AVLimits) (media.AVOutput, error) {
	return p.av(ctx, data, limits, true)
}

func (p *Processor) av(ctx context.Context, data []byte, limits media.AVLimits, write bool) (out media.AVOutput, err error) {
	if err := ctx.Err(); err != nil {
		return media.AVOutput{}, err
	}
	limits = limits.WithDefaults()
	if int64(len(data)) > limits.MaxBytes {
		return media.AVOutput{}, fmt.Errorf("%w: %d bytes, at most %d", media.ErrTooLarge, len(data), limits.MaxBytes)
	}
	select {
	case p.sem <- struct{}{}:
	case <-ctx.Done():
		return media.AVOutput{}, ctx.Err()
	}
	defer func() { <-p.sem }()
	defer func() {
		if r := recover(); r != nil {
			out, err = media.AVOutput{}, fmt.Errorf("%w: parser panicked: %v", media.ErrCorrupt, r)
		}
	}()
	switch media.SniffAV(data) {
	case media.FormatMP4:
		f, err := parseMP4(data, limits)
		if err != nil {
			return media.AVOutput{}, err
		}
		out = media.AVOutput{AVProbe: f.probe, Cover: f.cover}
		if write {
			out.Data, err = f.write(data)
		}
		return out, err
	case media.FormatWebM:
		f, err := parseWebM(data, limits)
		if err != nil {
			return media.AVOutput{}, err
		}
		out = media.AVOutput{AVProbe: f.probe}
		if write {
			out.Data, err = f.write()
		}
		return out, err
	case media.FormatMP3:
		out, err = remuxMP3(data, limits)
	case media.FormatOgg:
		out, err = remuxOgg(data, limits)
	default:
		return media.AVOutput{}, media.ErrUnsupportedFormat
	}
	if !write {
		out.Data = nil
	}
	return out, err
}
