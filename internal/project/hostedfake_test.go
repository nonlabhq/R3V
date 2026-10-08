//go:build nightly

package project

import (
	"testing"

	"github.com/nonlabhq/r3v/internal/remote/cloudtest"
)

// newHostedFake is a fake hosted team (see package cloudtest).
func newHostedFake(t *testing.T) string { return cloudtest.New(t) }
