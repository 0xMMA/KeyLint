//go:build windows

package settings

import (
	"errors"
	"syscall"
)

// Windows error codes for a file another process has open. Spelled out because
// the syscall package does not export ERROR_SHARING_VIOLATION.
const (
	errorAccessDenied     syscall.Errno = 5
	errorSharingViolation syscall.Errno = 32
)

// isTransientRenameError reports whether a rename failed because another
// process briefly holds the file, which is worth waiting out.
var isTransientRenameError = func(err error) bool {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return false
	}
	return errno == errorAccessDenied || errno == errorSharingViolation
}
