//go:build !windows

package settings

// isTransientRenameError is always false off Windows: a rename there does not
// fail because another process has the file open, so there is nothing to wait
// out and an error is returned at once.
var isTransientRenameError = func(error) bool { return false }
