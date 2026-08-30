package handlers

import (
	"net/http"
	"sort"
	"strings"
)

// Global search.
//
// Finding a dataset previously meant knowing which screen held it. That is
// survivable with three projects and crippling at fifty (ADR-034).
//
// The one thing search must not do is widen access. Every result here comes
// from the same scoping the corresponding listing endpoint uses -
// listProjectsForUser, datasetSubjects, ontologySubjects, hasProjectMembership -
// rather than from a parallel query. A search index maintained beside the
// authorisation model is an index that eventually disagrees with it, and the
// direction it disagrees in is a data leak.

const (
	searchMaxPerKind = 8
	searchMaxTotal   = 40
	searchMinQuery   = 2
)

type searchResult struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	// Label is what the user typed against; Sublabel gives it context.
	Label    string `json:"label"`
	Sublabel string `json:"sublabel,omitempty"`
	// ProjectID lets the interface build a link into the project scope.
	ProjectID string `json:"projectId,omitempty"`
	// Score orders results; lower is better.
	Score int `json:"-"`
}

func (h Handlers) Search(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	userID := identity.UserID()

	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if len([]rune(query)) < searchMinQuery {
		// Below two characters every result matches, which is noise rather
		// than an answer.
		writeJSON(w, http.StatusOK, map[string]any{"items": []searchResult{}, "query": query})
		return
	}

	results := []searchResult{}
	add := func(kind, id, label, sublabel, projectID string, haystacks ...string) {
		score, matched := matchScore(query, append([]string{label}, haystacks...))
		if !matched {
			return
		}
		results = append(results, searchResult{
			Kind: kind, ID: id, Label: label, Sublabel: sublabel, ProjectID: projectID, Score: score,
		})
	}

	// Projects, and the set of ids used to scope everything project-bound.
	accessible := map[string]string{}
	if projects, err := h.listProjectsForUser(userID); err == nil {
		for _, item := range projects {
			accessible[item.ID] = item.Name
			add("project", item.ID, item.Name, item.Description, item.ID, item.Description)
		}
	}

	if datasets, err := h.datasetStore.ListBySubjects(h.datasetSubjects(identity)); err == nil {
		for _, item := range h.filterDatasetsForEdition(datasets) {
			add("dataset", item.ID, item.Name, item.Description, "", item.Description, item.Bucket)
		}
	}

	if ontologies, err := h.ontologyStore.ListBySubjects(h.ontologySubjects(identity)); err == nil {
		for _, item := range ontologies {
			add("ontology", item.ID, item.Name, item.Description, "", item.Description, item.SourceName)
		}
	}

	if datasources, err := h.datasourceStore.ListByUser(userID); err == nil {
		for _, item := range datasources {
			add("datasource", item.ID, item.Name, item.Type, "", item.Host, item.Database)
		}
	}

	if repositories, err := h.repositoryStore.ListByUser(userID); err == nil {
		for _, item := range repositories {
			add("repository", item.ID, item.Name, item.URL, "", item.URL)
		}
	}

	// Secrets match on name only: a secret's value must never take part in a
	// search, and neither must anything derived from it.
	if secrets, err := h.secretStore.ListByUser(userID); err == nil {
		for _, item := range secrets {
			add("secret", item.Name, item.Name, "", "")
		}
	}

	if workspaces, err := h.workspaceStore.List(); err == nil {
		for _, item := range workspaces {
			if _, allowed := accessible[item.ProjectID]; !allowed {
				continue
			}
			add("workspace", item.ID, item.Name, accessible[item.ProjectID], item.ProjectID, item.Kind)
		}
	}

	if jobs, err := h.jobStore.List(); err == nil {
		for _, item := range jobs {
			if _, allowed := accessible[item.ProjectID]; !allowed {
				continue
			}
			add("job", item.ID, jobDisplayName(item), accessible[item.ProjectID], item.ProjectID)
		}
	}

	if apps, err := h.appStore.List(); err == nil {
		for _, item := range apps {
			if _, allowed := accessible[item.ProjectID]; !allowed {
				continue
			}
			add("app", item.ID, item.Name, accessible[item.ProjectID], item.ProjectID, item.Slug)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"query": query,
		"items": rankSearchResults(results),
	})
}

// matchScore ranks how well a query matches, so an exact name beats a
// substring buried in a description. Lower is better.
func matchScore(query string, haystacks []string) (int, bool) {
	best := -1
	for index, haystack := range haystacks {
		value := strings.ToLower(strings.TrimSpace(haystack))
		if value == "" {
			continue
		}
		var score int
		switch {
		case value == query:
			score = 0
		case strings.HasPrefix(value, query):
			score = 1
		case strings.Contains(value, query):
			score = 2
		default:
			continue
		}
		// A match on a later field is weaker than the same match on the label.
		score += index * 3
		if best == -1 || score < best {
			best = score
		}
	}
	return best, best >= 0
}

// rankSearchResults orders by relevance and caps each kind, so one crowded
// collection cannot push every other kind off the list.
func rankSearchResults(results []searchResult) []searchResult {
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score < results[j].Score
		}
		return results[i].Label < results[j].Label
	})

	perKind := map[string]int{}
	out := make([]searchResult, 0, searchMaxTotal)
	for _, result := range results {
		if perKind[result.Kind] >= searchMaxPerKind {
			continue
		}
		perKind[result.Kind]++
		out = append(out, result)
		if len(out) >= searchMaxTotal {
			break
		}
	}
	return out
}
