package pyramidize

import "strings"

// knownApps maps lowercase keyword substrings to their default Pyramidize document type.
// When LookupDocType scans a source app name, it checks whether any of these keywords
// is a substring of the (lowercased) app name/window title.
var knownApps = map[string]string{
	// Email clients
	"outlook":     "email",
	"thunderbird": "email",
	"apple mail":  "email",
	"mail":        "email",

	// Messaging apps that commonly carry email-style communication
	"teams": "email",
	"slack": "email",

	// Code editors and text editors → informal memos / notes
	"vscode":    "memo",
	"code":      "memo",
	"notepad++": "memo",
	"notepad":   "memo",
	"sublime":   "memo",

	// Word processors → memo / document style
	"word": "memo",
	"docs": "memo",

	// Presentation tools
	"powerpoint": "powerpoint",
	"keynote":    "powerpoint",
	"impress":    "powerpoint",
}

// LookupDocType returns the default document type for the given source application name.
// It performs a case-insensitive substring search against known application keywords.
// Returns an empty string if no match is found.
func LookupDocType(sourceApp string) string {
	lower := strings.ToLower(sourceApp)
	for keyword, docType := range knownApps {
		if strings.Contains(lower, keyword) {
			return docType
		}
	}
	return ""
}
