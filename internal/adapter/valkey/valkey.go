// SPDX-License-Identifier: AGPL-3.0-only

// Package valkey connects to a Valkey server (or another server that speaks the Redis protocol)
// for the adapters that keep their data there, such as the key-value store of
// internal/adapter/kv/valkey. The adapters share one connection setting (valkey_url,
// valkey_tls_ca_file, valkey_key_prefix) and may share one Client.
//
// The client is valkey-go (Apache-2.0), which is pure Go. It finds out by itself whether the
// server is standalone or a cluster; the adapters keep each of their data sets in one hash slot,
// so that their scripts work on both.
package valkey

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"

	valkeygo "github.com/valkey-io/valkey-go"
)

// Client is a connection pool to the server; it is safe for concurrent use.
type Client = valkeygo.Client

// Options say how to reach the server.
type Options struct {
	// URL is valkey://[user:password@]host[:port][/db] (redis:// is the same), valkeys:// or
	// rediss:// for TLS, or unix:///path/to/socket. The path selects the database number.
	URL string
	// TLSCAFile is a PEM file of the certificate authorities that the server's TLS certificate
	// chains to, for a managed service with a private CA; empty uses the system roots.
	TLSCAFile string
}

// CheckURL reports whether raw is a URL that Open accepts. It does not connect.
func CheckURL(raw string) error {
	_, err := parseURL(raw)
	return err
}

func parseURL(raw string) (valkeygo.ClientOption, error) {
	opt, err := valkeygo.ParseURL(raw)
	if err != nil {
		return opt, err
	}
	if opt.TLSConfig != nil && opt.TLSConfig.InsecureSkipVerify {
		return opt, errors.New("valkey: skip_verify is refused; trust the server's CA with valkey_tls_ca_file instead")
	}
	return opt, nil
}

// Open connects to the server and returns the client; Close it to release the connections.
func Open(ctx context.Context, opts Options) (Client, error) {
	opt, err := parseURL(opts.URL)
	if err != nil {
		return nil, err
	}
	if opts.TLSCAFile != "" {
		if opt.TLSConfig == nil {
			return nil, errors.New("valkey: a TLS CA file needs a valkeys:// or rediss:// URL")
		}
		pem, err := os.ReadFile(opts.TLSCAFile)
		if err != nil {
			return nil, fmt.Errorf("valkey: TLS CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("valkey: TLS CA file %s holds no PEM certificate", opts.TLSCAFile)
		}
		opt.TLSConfig.RootCAs = pool
		opt.TLSConfig.MinVersion = tls.VersionTLS12
	}
	// The adapters read every value from the server; client-side caching would need CLIENT
	// TRACKING, which some managed services do not offer.
	opt.DisableCache = true
	c, err := valkeygo.NewClient(opt)
	if err != nil {
		return nil, fmt.Errorf("valkey: connect: %w", err)
	}
	if err := Ping(ctx, c); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

// Ping reports whether the server answers.
func Ping(ctx context.Context, c Client) error {
	if err := c.Do(ctx, c.B().Ping().Build()).Error(); err != nil {
		return fmt.Errorf("valkey: ping: %w", err)
	}
	return nil
}
