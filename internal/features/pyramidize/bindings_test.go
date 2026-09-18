package pyramidize

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"testing"
)

// Wails turns every exported method of a registered service into a function the
// webview can call. main.go registers *Service, so adding an exported method
// here widens the RPC surface — silently, because the generated bindings are
// committed and nothing checks them against the Go side.
//
// That is not a style question: an eval helper that spends the user's API key
// became webview-callable this way, and the only visible symptom was a
// generated file nobody regenerated.
//
// CI also runs `wails3 generate bindings` and fails on a diff. This test is the
// fast local version, and it needs no toolchain.
func TestTheExportedMethodSetMatchesTheCommittedBindings(t *testing.T) {
	bindings := filepath.Join("..", "..", "..", "frontend", "bindings", "keylint",
		"internal", "features", "pyramidize", "service.js")
	data, err := os.ReadFile(bindings)
	if err != nil {
		t.Skipf("generated bindings not present: %v", err)
	}

	var generated []string
	for _, m := range regexp.MustCompile(`(?m)^export function ([A-Za-z0-9_]+)`).FindAllStringSubmatch(string(data), -1) {
		generated = append(generated, m[1])
	}
	sort.Strings(generated)

	var exported []string
	typ := reflect.TypeOf(&Service{})
	for i := 0; i < typ.NumMethod(); i++ {
		exported = append(exported, typ.Method(i).Name)
	}
	sort.Strings(exported)

	if !reflect.DeepEqual(exported, generated) {
		t.Errorf("exported methods and generated bindings disagree.\n  Go:       %v\n  bindings: %v\n"+
			"Run `wails3 generate bindings` if the method is meant to be callable from the UI, "+
			"or unexport it if it is not.", exported, generated)
	}
}
