package clipboard

import "fmt"

// cfHTML wraps an HTML fragment in the clipboard's "HTML Format" (CF_HTML):
// a header of byte offsets, then a document whose fragment sits between
// <!--StartFragment--> and <!--EndFragment-->. The offsets count UTF-8 bytes
// from the start of the data, and they are fixed-width, so the header's length
// does not depend on their values.
// https://learn.microsoft.com/en-us/windows/win32/dataxchg/html-clipboard-format
//
// It lives outside the Windows files so the offsets are tested on every
// platform.
func cfHTML(fragment string) []byte {
	const header = "Version:0.9\r\nStartHTML:%010d\r\nEndHTML:%010d\r\nStartFragment:%010d\r\nEndFragment:%010d\r\n"
	const pre = "<html>\r\n<body>\r\n<!--StartFragment-->"
	const post = "<!--EndFragment-->\r\n</body>\r\n</html>"

	startHTML := len(fmt.Sprintf(header, 0, 0, 0, 0))
	startFragment := startHTML + len(pre)
	endFragment := startFragment + len(fragment)
	endHTML := endFragment + len(post)
	return []byte(fmt.Sprintf(header, startHTML, endHTML, startFragment, endFragment) + pre + fragment + post)
}
