// Copyright 2022 Canonical Ltd.

package version_test

import (
	"testing"

	"github.com/canonical/ofga/internal/version"
	qt "github.com/frankban/quicktest"
)

func TestInfo(t *testing.T) {
	qt.Assert(t, version.Info(), qt.DeepEquals, version.Version{
		GitCommit: version.GitCommit,
	})
}
