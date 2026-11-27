package handlers

import "strings"

// How a caller's command becomes shell words, in one place.
//
// Apps worked this out first, and the reasoning is theirs: the contract is an
// argv in words, but whoever calls the API by hand writes the line they would
// have typed, and a single word full of spaces has to reach a shell or it is
// read as the name of one very long program. Jobs joined their argv with
// spaces instead, which lost the word boundaries the moment an argument
// contained one - invisible for as long as the jobs screen bypassed this path
// altogether, and live for everybody now that it does not.

// commandWords keeps the words a caller actually gave.
func commandWords(argv []string) []string {
	words := make([]string, 0, len(argv))
	for _, word := range argv {
		if trimmed := strings.TrimSpace(word); trimmed != "" {
			words = append(words, trimmed)
		}
	}
	return words
}

// launchWords renders those words for a shell, each still a word.
//
// Two exceptions, and both are the same situation seen from different ends:
// the caller wrote a command line, not an argv.
//
//   - a lone word containing whitespace, which is somebody calling the API by
//     hand with the line they would have typed;
//   - words that include a shell operator, which is somebody typing that same
//     line into the interface, where it is split on spaces before it arrives.
//
// The second cost a researcher an afternoon on 2026-11-27. The command was
// `apt-get update && apt-get install -y libusb-1.0-0 && python process.py
// --source ... --processed ...`, written exactly as it would be in a
// terminal. Split into words and quoted one by one, `&&` stops being an
// operator and becomes an argument: apt received the whole line and answered
// "the command line option --source is not recognized", which names neither
// the `&&` nor the splitting nor anything the person could act on.
//
// Quoting each word stays the default, and it is not a detail to give up: it
// is what keeps `--title 'my study'` from becoming two arguments. What changes
// is that a line which cannot possibly be an argv is run as what it is.
func launchWords(words []string) []string {
	if len(words) == 1 && strings.ContainsAny(words[0], " \t") {
		return []string{"sh", "-c", shellQuote(words[0])}
	}
	if containsShellOperator(words) {
		return []string{"sh", "-c", shellQuote(shellLineFrom(words))}
	}
	rendered := make([]string, 0, len(words))
	for _, word := range words {
		rendered = append(rendered, shellQuote(word))
	}
	return rendered
}

// containsShellOperator reports whether a word is one of the operators that
// only mean anything to a shell.
//
// Whole words only. A word that *contains* `&&` - a regular expression, a URL
// with a `;` - is an argument and must keep being quoted; a word that *is*
// `&&` was typed as an operator, because nothing else is spelled that way on
// its own.
// shellLineFrom rebuilds the line the caller typed: the operators bare, so
// the shell reads them, and everything else quoted, so it does not.
//
// Joining the words with plain spaces would have been simpler and wrong in
// the same way the original defect was: `python -c "import sys; print(x)"`
// comes apart again the moment the shell re-reads it. Proven on a cluster
// rather than reasoned about - the first version of this fix joined raw, and
// the job died on `sh: 1: Syntax error: word unexpected`.
func shellLineFrom(words []string) string {
	parts := make([]string, 0, len(words))
	for _, word := range words {
		if isShellOperator(word) {
			parts = append(parts, word)
			continue
		}
		parts = append(parts, shellQuote(word))
	}
	return strings.Join(parts, " ")
}

func containsShellOperator(words []string) bool {
	for _, word := range words {
		if isShellOperator(word) {
			return true
		}
	}
	return false
}

func isShellOperator(word string) bool {
	switch word {
	case "&&", "||", "|", ";", "&", ">", ">>", "<", "2>", "2>&1":
		return true
	}
	return false
}
