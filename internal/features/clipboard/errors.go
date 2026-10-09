package clipboard

import (
	"errors"
	"time"
)

// ErrNothingCopied is a copy shortcut that did not change the clipboard: the
// foreground window had nothing selected, or ignored the keystroke (an
// elevated window drops input from a normal process without saying so). The
// clipboard still holds whatever was there before, which is not what the user
// meant — a silent fix that went ahead would send that old text to the
// provider and paste its "fix" at the caret.
var ErrNothingCopied = errors.New("the copy did not change the clipboard")

// PasteSettle is how long to let a clipboard write settle before the paste
// keystroke. A caller that has to check the target window right before the
// keystroke waits this itself and then calls SendPaste.
const PasteSettle = 150 * time.Millisecond
