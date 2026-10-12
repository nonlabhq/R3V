package redact

import (
	"strings"
	"testing"
)

func TestText(t *testing.T) {
	for _, secret := range []string{
		"r3v-s3:eyJlbmRwb2ludCI6Imh0dHBzOi8vZXhhbXBsZSJ9",
		"AKIAIOSFODNN7EXAMPLE",
		"sk-ant-api03-AbCdEfGhIjKlMnOpQrStUvWxYz012345",
		"ghp_1234567890abcdefghijABCDEFGHIJ123456",
		"xoxb-123456789012-abcdefghij",
		"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjMifQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV",
		"Zq8Lm2Vx9Rt4Kp7Wn3Hs6Jd1Fb5Gc0Ya",
	} {
		got := Text("use " + secret + " for it")
		if strings.Contains(got, secret) || !strings.Contains(got, Mark) || !strings.HasPrefix(got, "use ") {
			t.Errorf("%s: %q", secret, got)
		}
	}
	if got := Text("password: hunter2 and then"); strings.Contains(got, "hunter2") {
		t.Errorf("password: %q", got)
	}
	if got := Text("-----BEGIN OPENSSH PRIVATE KEY-----\nabc\n-----END OPENSSH PRIVATE KEY-----"); strings.Contains(got, "abc") {
		t.Errorf("private key: %q", got)
	}
	// What a prompt says about the work stays.
	for _, plain := range []string{
		"Make the arena lighting warmer and move the spawn points off the walls",
		"go to version 9e3cd7f1a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d",
		"see src/renderer/postprocessing/ToneMappingPass.ts and apps/editor/src-tauri/target",
		"rename SpawnPointControllerComponentForArenaLevels",
	} {
		if got := Text(plain); got != plain {
			t.Errorf("changed: %q", got)
		}
	}
}
