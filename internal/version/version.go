// Copyright 2022 Canonical Ltd.

// Package version provides server version information.
package version

// GitCommit holds the git commit, and it is populated while building the
// server using ldflags.
var GitCommit = "development"

// Version holds server version info.
type Version struct {
	GitCommit string
}

// Info returns information about the server version.
func Info() Version {
	return Version{
		GitCommit: GitCommit,
	}
}
