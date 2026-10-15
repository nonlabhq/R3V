package cloud

// R3V signs in to R3V Cloud (every build since 0.2). The test service,
// api-dev.r3v.so, is for the end-to-end runs (R3V_CLOUD_URL).
func init() { Default = "https://api.r3v.so" }
