//go:build nightly

package cloud

// Nightly builds sign in to the test service while R3V-Cloud is built.
func init() { Default = "https://api-dev.r3v.so" }
