//go:build nightly

package cloud

// Nightly builds sign in to R3V Cloud (Stable doesn't reach it). The test
// service, api-dev.r3v.so, is for the end-to-end runs (R3V_CLOUD_URL).
func init() { Default = "https://api.r3v.so" }
