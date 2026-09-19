package enhance

// The text is handed to the model between markers so that it has a name for
// what it is looking at. Without them a message that reads as addressed to a
// reader is indistinguishable from one addressed to the assistant, and two of
// three runs of the first baseline answered a chat message instead of
// correcting it.
const (
	inputOpen  = "<text-to-correct>"
	inputClose = "</text-to-correct>"
)

func buildUserMessage(text string) string {
	return inputOpen + "\n" + text + "\n" + inputClose
}
