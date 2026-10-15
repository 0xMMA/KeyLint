package clipboard

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// offsets reads the four byte offsets out of a CF_HTML header.
func offsets(t *testing.T, data []byte) map[string]int {
	t.Helper()
	got := map[string]int{}
	for _, m := range regexp.MustCompile(`(StartHTML|EndHTML|StartFragment|EndFragment):(\d+)\r\n`).FindAllSubmatch(data, -1) {
		n, err := strconv.Atoi(string(m[2]))
		if err != nil {
			t.Fatalf("offset %s: %v", m[1], err)
		}
		got[string(m[1])] = n
	}
	if len(got) != 4 {
		t.Fatalf("header has %d offsets, want 4:\n%s", len(got), data)
	}
	return got
}

func TestCFHTMLOffsetsPointAtTheFragment(t *testing.T) {
	for _, fragment := range []string{
		"<p><strong>Bold</strong></p>",
		// Offsets count UTF-8 bytes, not characters: ü, – and € are 2, 3 and 3.
		"<p>Grüße – 5 €</p><ul><li>eins</li></ul>",
		"",
	} {
		data := cfHTML(fragment)
		o := offsets(t, data)

		if got := string(data[o["StartFragment"]:o["EndFragment"]]); got != fragment {
			t.Errorf("fragment = %q, want %q", got, fragment)
		}
		if !strings.HasSuffix(string(data[:o["StartFragment"]]), "<!--StartFragment-->") {
			t.Errorf("StartFragment does not follow the marker:\n%s", data)
		}
		if !strings.HasPrefix(string(data[o["EndFragment"]:]), "<!--EndFragment-->") {
			t.Errorf("EndFragment is not at the marker:\n%s", data)
		}
		doc := string(data[o["StartHTML"]:o["EndHTML"]])
		if !strings.HasPrefix(doc, "<html>") || !strings.HasSuffix(doc, "</html>") {
			t.Errorf("StartHTML..EndHTML = %q, want the whole document", doc)
		}
		if o["EndHTML"] != len(data) {
			t.Errorf("EndHTML = %d, want the end of the data (%d)", o["EndHTML"], len(data))
		}
		if !strings.Contains(doc, `<meta charset="utf-8">`) {
			t.Errorf("document does not declare UTF-8:\n%s", doc)
		}
		if !strings.HasPrefix(string(data), "Version:0.9\r\n") {
			t.Errorf("data does not start with the version line:\n%s", data)
		}
	}
}
