package remote

// File locks (R3V Cloud teams), in every build since 0.2.
func init() {
	FileLocks = true
	RegisterFeature(FeatureLocks)
}
