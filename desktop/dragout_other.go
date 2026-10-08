//go:build !windows || server

package desktop

// StartDrag drags files out of the app: only the Windows app can (the
// browser build can't hand files to other programs), so nothing starts.
func (a *App) StartDrag(root string, files []string) (bool, error) { return false, nil }
