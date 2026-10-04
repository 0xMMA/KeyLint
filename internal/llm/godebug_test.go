package llm

import (
	"runtime/debug"
	"strings"
	"testing"
)

// TestCertStoreOverrideStaysOff pins the `godebug x509sslcertoverrideplatform=0`
// line in go.mod. Every provider call goes through crypto/x509; if the line is
// dropped, Go 1.27's default lets SSL_CERT_FILE / SSL_CERT_DIR replace the
// Windows certificate store, and nothing else in the suite would notice.
func TestCertStoreOverrideStaysOff(t *testing.T) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		t.Skip("no build info in this binary")
	}
	for _, s := range info.Settings {
		if s.Key == "DefaultGODEBUG" && strings.Contains(s.Value, "x509sslcertoverrideplatform=0") {
			return
		}
	}
	t.Fatal("x509sslcertoverrideplatform=0 is not in DefaultGODEBUG — was the godebug line removed from go.mod?")
}
