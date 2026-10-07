package remote

import (
	"errors"
	"net/url"
	"strings"
	"syscall"
	"testing"
)

// A presigned address is a key to the storage for a while: a transfer's
// error doesn't carry it, and is told to be tried again as before.
func TestTransferErrorsKeepPresignedAddressesOut(t *testing.T) {
	in := &url.Error{Op: "Put", URL: "https://bucket.example/projects/p/objects/ab?X-Amz-Credential=AKIA&X-Amz-Signature=deadbeef",
		Err: syscall.ECONNRESET}
	out := withoutAddress(in)
	if s := out.Error(); strings.Contains(s, "X-Amz") || strings.Contains(s, "bucket.example") {
		t.Errorf("error %q", s)
	}
	if Transient(in) != Transient(out) {
		t.Error("whether to try again changed")
	}
	if !errors.Is(out, syscall.ECONNRESET) {
		t.Error("the cause is lost")
	}
}

// Contents kept per project (a hosted team's storage) aren't cleaned up
// from here: this cleanup would read them as one store.
func TestCleanupRefusesPerProjectStorage(t *testing.T) {
	b, err := NewBroker("https://cloud.example/v1/teams/t1", "tok")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewBucketBackend(b).CollectGarbage(true); !errors.Is(err, ErrCleanupPerProject) {
		t.Errorf("cleanup: %v", err)
	}
}
