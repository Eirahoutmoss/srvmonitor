package config

import (
	"bytes"
	"io"
	"strings"
)

// newCommentStripper returns a reader over the config bytes with whole-line
// // comments removed, so the JSON file can carry human notes. Only lines
// whose first non-space characters are "//" are dropped; inline "//" inside a
// value (such as a URL) is left untouched.
func newCommentStripper(b []byte) io.Reader {
	var out bytes.Buffer
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	return &out
}
