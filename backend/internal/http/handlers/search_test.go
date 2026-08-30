package handlers

import "testing"

// Ranking decides what a user sees first, and the label is what they typed
// against: an exact name must beat the same word buried in a description.
func TestMatchScorePrefersTheLabel(t *testing.T) {
	exact, ok := matchScore("ventes", []string{"ventes"})
	if !ok {
		t.Fatal("an exact label must match")
	}
	prefix, _ := matchScore("vent", []string{"ventes 2026"})
	contains, _ := matchScore("2026", []string{"ventes 2026"})
	inDescription, _ := matchScore("ventes", []string{"analyse", "rapport sur les ventes"})

	if !(exact < prefix && prefix < contains && contains < inDescription) {
		t.Fatalf("expected exact < prefix < contains < description, got %d %d %d %d",
			exact, prefix, contains, inDescription)
	}
}

func TestMatchScoreIgnoresEmptyAndMisses(t *testing.T) {
	if _, ok := matchScore("ventes", []string{"", "   "}); ok {
		t.Fatal("blank haystacks must not match")
	}
	if _, ok := matchScore("ventes", []string{"achats", "stocks"}); ok {
		t.Fatal("an absent term must not match")
	}
}

// One crowded collection must not push every other kind off the list: a user
// with 200 jobs still needs to find their project.
func TestRankingCapsEachKind(t *testing.T) {
	results := []searchResult{}
	for i := range 50 {
		results = append(results, searchResult{Kind: "job", ID: string(rune('a' + i%26)), Label: "job", Score: 0})
	}
	results = append(results, searchResult{Kind: "project", ID: "p1", Label: "projet", Score: 2})

	ranked := rankSearchResults(results)

	jobs, projects := 0, 0
	for _, result := range ranked {
		switch result.Kind {
		case "job":
			jobs++
		case "project":
			projects++
		}
	}
	if jobs > searchMaxPerKind {
		t.Fatalf("expected at most %d jobs, got %d", searchMaxPerKind, jobs)
	}
	if projects != 1 {
		t.Fatal("the project must survive a flood of jobs")
	}
}

func TestRankingOrdersByScoreThenLabel(t *testing.T) {
	ranked := rankSearchResults([]searchResult{
		{Kind: "dataset", ID: "3", Label: "zebre", Score: 0},
		{Kind: "dataset", ID: "1", Label: "alpha", Score: 2},
		{Kind: "dataset", ID: "2", Label: "beta", Score: 0},
	})
	if ranked[0].Label != "beta" || ranked[1].Label != "zebre" || ranked[2].Label != "alpha" {
		t.Fatalf("unexpected order: %s, %s, %s", ranked[0].Label, ranked[1].Label, ranked[2].Label)
	}
}
