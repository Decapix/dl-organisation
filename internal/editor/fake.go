package editor

// Fake is a test double for Editor. It lives in the production package rather
// than a _test.go file because the actions package needs it too.
type Fake struct {
	// Result is what Edit returns.
	Result string
	// Err, when non-nil, is what Edit returns instead.
	Err error
	// Seen records the content Edit was handed, so tests can assert that the
	// current note was passed in.
	Seen string
	// Calls counts invocations.
	Calls int
}

// Edit implements Editor.
func (f *Fake) Edit(initial string) (string, error) {
	f.Calls++
	f.Seen = initial
	if f.Err != nil {
		return "", f.Err
	}
	return f.Result, nil
}
