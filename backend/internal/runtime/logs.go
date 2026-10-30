package runtime

import (
	"regexp"
	"strings"
)

// Container logs as a screen can read them.
//
// A build tool writes for a terminal: colour, and progress bars that rewrite
// the line they sit on. Served raw to a browser, the escape character itself
// does not render and what is left is its arguments - "[36mINFO[0m[0000]"
// where a terminal shows a cyan "INFO" - and a pip download bar becomes one
// line holding every percentage it passed through.
//
// So the sequences are removed, and a line the tool rewrote keeps what a
// terminal would have left on it. Nothing else is touched: the tool's own
// words, kaniko's "[0000]" elapsed seconds included, are what somebody pastes
// into a bug report.
var (
	terminalEscape = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)
	terminalOSC    = regexp.MustCompile(`\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)`)
)

// ReadableLogs strips terminal formatting from container output.
func ReadableLogs(raw string) string {
	cleaned := terminalOSC.ReplaceAllString(raw, "")
	cleaned = terminalEscape.ReplaceAllString(cleaned, "")
	if !strings.ContainsRune(cleaned, '\r') {
		return cleaned
	}
	lines := strings.Split(cleaned, "\n")
	for i, line := range lines {
		if index := strings.LastIndex(line, "\r"); index >= 0 {
			lines[i] = line[index+1:]
		}
	}
	return strings.Join(lines, "\n")
}
