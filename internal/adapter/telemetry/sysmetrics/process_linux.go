// SPDX-License-Identifier: AGPL-3.0-only

//go:build linux

package sysmetrics

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
	"syscall"
	"time"

	"github.com/best-friends/my/internal/app/kernel"
)

// registerOSProcess reports the CPU time, the resident memory, and the file descriptors from
// getrusage(2), getrlimit(2), and /proc/self.
func registerOSProcess(m kernel.Meter) {
	m.ObservableCounter(ProcessCPUTime, func(_ context.Context, o kernel.GaugeObserver) error {
		var ru syscall.Rusage
		if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
			return fmt.Errorf("getrusage: %w", err)
		}
		o.Observe(timevalSeconds(ru.Utime), kernel.NewLabels(LabelCPUMode, "user"))
		o.Observe(timevalSeconds(ru.Stime), kernel.NewLabels(LabelCPUMode, "system"))
		return nil
	})
	m.Gauge(ProcessMemoryUsage, func(_ context.Context, o kernel.GaugeObserver) error {
		rss, err := residentBytes()
		if err != nil {
			return err
		}
		o.Observe(float64(rss), kernel.Labels{})
		return nil
	})
	m.Gauge(ProcessFileDescriptors, func(_ context.Context, o kernel.GaugeObserver) error {
		entries, err := os.ReadDir("/proc/self/fd")
		if err != nil {
			return err
		}
		// The directory being read holds a descriptor of its own.
		o.Observe(float64(max(len(entries)-1, 0)), kernel.Labels{})
		return nil
	})
	m.Gauge(ProcessFileDescriptorLimit, func(_ context.Context, o kernel.GaugeObserver) error {
		var l syscall.Rlimit
		if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &l); err != nil {
			return fmt.Errorf("getrlimit: %w", err)
		}
		o.Observe(float64(l.Cur), kernel.Labels{})
		return nil
	})
}

func timevalSeconds(tv syscall.Timeval) float64 {
	return (time.Duration(tv.Sec)*time.Second + time.Duration(tv.Usec)*time.Microsecond).Seconds()
}

// residentBytes reads the resident set size from /proc/self/statm, whose second field counts
// pages.
func residentBytes() (int64, error) {
	b, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		return 0, err
	}
	fields := bytes.Fields(b)
	if len(fields) < 2 {
		return 0, fmt.Errorf("/proc/self/statm: %q", b)
	}
	pages, err := strconv.ParseInt(string(fields[1]), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("/proc/self/statm: %w", err)
	}
	return pages * int64(os.Getpagesize()), nil
}
