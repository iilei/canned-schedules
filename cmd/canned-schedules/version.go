package main

import (
	"regexp"
	"runtime/debug"
	"strings"
)

// Set by goreleaser via -ldflags. A release build prints them verbatim.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// semverPattern is the official regular expression from semver.org (2.0.0), anchored.
var semverPattern = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)` +
	`(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][a-zA-Z0-9-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][a-zA-Z0-9-]*))*))?` +
	`(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

// stamp is what --version prints and what the Sphinx kit records.
type stamp struct {
	version, commit, date string
}

// buildStamp resolves the stamp of the running binary.
func buildStamp() stamp {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		info = nil
	}

	return resolveStamp(stamp{version, commit, date}, info)
}

// resolveStamp fills in what -ldflags left at its default. Values set via -ldflags win and pass through verbatim.
// Without them (go install, go build), the module version from the build info is used when it is valid semver after
// its "v" (so "(devel)" stays "dev"), and the VCS stamp supplies commit and date when the build has one.
func resolveStamp(ld stamp, info *debug.BuildInfo) stamp {
	v, c, d := ld.version, ld.commit, ld.date
	if info == nil {
		return ld
	}

	if v == "dev" {
		if mod := strings.TrimPrefix(info.Main.Version, "v"); semverPattern.MatchString(mod) {
			v = mod
		}
	}

	for _, s := range info.Settings {
		switch {
		case s.Key == "vcs.revision" && c == "none":
			c = s.Value
		case s.Key == "vcs.time" && d == "unknown":
			d = s.Value
		}
	}

	return stamp{v, c, d}
}
