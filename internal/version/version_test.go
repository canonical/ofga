// Copyright 2023 Canonical Ltd.
// Licensed under the AGPL license, see LICENSE file for details.

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
