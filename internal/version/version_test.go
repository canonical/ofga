// Copyright 2023 Canonical Ltd.

package version_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/canonical/ofga/internal/version"
)

func TestInfo(t *testing.T) {
	qt.Assert(t, version.Info(), qt.DeepEquals, version.Version{
		GitCommit: version.GitCommit,
	})
}
