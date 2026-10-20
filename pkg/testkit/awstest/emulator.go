// SPDX-License-Identifier: AGPL-3.0-only

// Package awstest gives the tests of the adapters on Amazon SNS and SQS a broker: Fake, SNS and
// SQS in memory, for every run, and Emulator, an AWS emulator, for the runs that ask for one.
//
// Environment variables choose the emulator:
//
//   - TEST_AWS_ENDPOINT: an emulator that is already running, as a URL (http://localhost:4566).
//   - TEST_AWS=container: a container of TEST_AWS_IMAGE (default DefaultImage) started with
//     testcontainers-go on the first Emulator call (make go-test-stream).
//   - Without either, Emulator skips the test.
//   - TEST_AWS_REQUIRED: when non-empty, an emulator that cannot start because Docker is
//     unavailable fails the tests instead of skipping them. CI sets it.
//
// The emulator is Floci (MIT): goaws v0.5.4 rejects the Policy attribute of CreateQueue and
// SetQueueAttributes in the AWS JSON protocol of the SQS client and returns no CreatedTimestamp,
// and MiniStack keeps a long poll of a deleted queue waiting until its wait time ends. The
// TestMain of a package that uses Emulator calls os.Exit(awstest.Main(m)) to stop the container.
package awstest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// The environment variables that choose the emulator; see the package documentation.
const (
	EnvEndpoint = "TEST_AWS_ENDPOINT"
	EnvMode     = "TEST_AWS"
	EnvImage    = "TEST_AWS_IMAGE"
	EnvRequired = "TEST_AWS_REQUIRED"
)

// DefaultImage is the emulator's container image.
const DefaultImage = "floci/floci:2.1.0"

var emulator struct {
	once     sync.Once
	endpoint string // empty when there is no emulator
	skip     string // why endpoint is empty, when the tests skip
	err      error  // why endpoint is empty, when the tests fail
	stop     func()
}

// Main runs the tests of m and then stops the emulator, if Emulator started one.
func Main(m *testing.M) int {
	code := m.Run()
	if emulator.stop != nil {
		emulator.stop()
		emulator.stop = nil
	}
	return code
}

// Emulator returns the configuration of the SNS and SQS clients of the emulator: its endpoint,
// region FakeRegion, and static credentials. It skips t when no emulator is chosen or available.
// Tests share the emulator, so each one uses topics and queue names of its own.
func Emulator(t testing.TB) aws.Config {
	t.Helper()
	emulator.once.Do(startEmulator)
	if emulator.err != nil {
		t.Fatalf("awstest: %v", emulator.err)
	}
	if emulator.endpoint == "" {
		t.Skip(emulator.skip)
	}
	return Config(emulator.endpoint)
}

// Config returns the configuration of SNS and SQS clients on endpoint, with region FakeRegion and
// static credentials.
func Config(endpoint string) aws.Config {
	return aws.Config{
		Region:       FakeRegion,
		BaseEndpoint: aws.String(endpoint),
		Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			return aws.Credentials{AccessKeyID: "test", SecretAccessKey: "test", Source: "awstest"}, nil
		}),
	}
}

func startEmulator() {
	if e := os.Getenv(EnvEndpoint); e != "" {
		emulator.endpoint = e
		return
	}
	if os.Getenv(EnvMode) != "container" {
		emulator.skip = fmt.Sprintf("no AWS emulator: set %s=container (make go-test-stream) or %s", EnvMode, EnvEndpoint)
		return
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		reason := fmt.Sprintf("Docker is unavailable for the AWS emulator container (%v)", err)
		if os.Getenv(EnvRequired) != "" {
			emulator.err = fmt.Errorf("%s, but %s is set", reason, EnvRequired)
			return
		}
		emulator.skip = reason
		return
	}
	image := os.Getenv(EnvImage)
	if image == "" {
		image = DefaultImage
	}
	ctx := context.Background()
	c, err := testcontainers.Run(ctx, image,
		testcontainers.WithExposedPorts("4566/tcp"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("4566/tcp").WithStartupTimeout(2*time.Minute)),
	)
	if c != nil {
		emulator.stop = func() { _ = testcontainers.TerminateContainer(c) }
	}
	if err != nil {
		emulator.err = fmt.Errorf("start container %s: %w", image, err)
		return
	}
	endpoint, err := c.PortEndpoint(ctx, "4566/tcp", "http")
	if err != nil {
		emulator.err = fmt.Errorf("container %s endpoint: %w", image, err)
		return
	}
	emulator.endpoint = endpoint
}
