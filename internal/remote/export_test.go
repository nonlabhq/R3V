package remote

// ForgetFeature undoes RegisterFeature (tests that turn a feature on).
func ForgetFeature(name string) {
	featuresMu.Lock()
	delete(known, name)
	featuresMu.Unlock()
}
