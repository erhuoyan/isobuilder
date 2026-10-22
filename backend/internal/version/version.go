// Package version exposes build-time metadata injected via ldflags.
//
//	go build -ldflags "-X isobuilder/internal/version.Version=0.5.0
//	  -X isobuilder/internal/version.GitCommit=$(git rev-parse --short HEAD)
//	  -X isobuilder/internal/version.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
package version

import (
	"fmt"
	"runtime"
)

// Set by ldflags at build time; fallback to sensible dev defaults.
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

// GoVersion returns the Go toolchain version.
func GoVersion() string { return runtime.Version() }

// Short returns a one-line summary, e.g. "0.5.0 (abc1234)".
func Short() string {
	return fmt.Sprintf("%s (%s)", Version, GitCommit)
}

// Full returns a multi-line build info string for the -v flag.
func Full() string {
	return fmt.Sprintf("isobuilder %s\n  commit:  %s\n  built:   %s\n  go:      %s\n  os/arch: %s/%s",
		Version, GitCommit, BuildTime, GoVersion(), runtime.GOOS, runtime.GOARCH)
}

// Info returns the structured build metadata for API responses.
func Info() map[string]string {
	return map[string]string{
		"version":    Version,
		"git_commit": GitCommit,
		"build_time": BuildTime,
		"go_version": GoVersion(),
	}
}
