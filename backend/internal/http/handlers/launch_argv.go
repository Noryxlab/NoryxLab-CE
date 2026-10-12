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
// The exception is the one case above: a lone word containing whitespace is a
// command line, so a shell is interposed for it and only for it.
func launchWords(words []string) []string {
	if len(words) == 1 && strings.ContainsAny(words[0], " \t") {
		return []string{"sh", "-c", shellQuote(words[0])}
	}
	rendered := make([]string, 0, len(words))
	for _, word := range words {
		rendered = append(rendered, shellQuote(word))
	}
	return rendered
}
