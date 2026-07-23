package duration

import (
	"os"
	"testing"
)

// openTestFile opens path for reading, closing it automatically when the
// test ends — every parser in this package takes an already-open file
// since the scanner shares one file handle between tag and duration
// extraction.
func openTestFile(t *testing.T, path string) *os.File {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}
