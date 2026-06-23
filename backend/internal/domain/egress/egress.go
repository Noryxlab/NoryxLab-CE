package egress

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Rule struct {
	ID            string     `json:"id"`
	ProjectID     string     `json:"projectId"`
	RequesterID   string     `json:"requesterId"`
	SubjectType   string     `json:"subjectType"`
	SubjectID     string     `json:"subjectId"`
	Profile       string     `json:"profile"`
	Destination   string     `json:"destination"`
	Port          int        `json:"port"`
	Protocol      string     `json:"protocol"`
	WorkloadTypes []string   `json:"workloadTypes"`
	Justification string     `json:"justification"`
	Status        string     `json:"status"`
	ReviewerID    string     `json:"reviewerId"`
	DecisionNote  string     `json:"decisionNote"`
	ExpiresAt     *time.Time `json:"expiresAt,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

type Profile struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Default     bool   `json:"default"`
	HDSAllowed  bool   `json:"hdsAllowed"`
	AdminOnly   bool   `json:"adminOnly"`
}

func New(projectID, requesterID, subjectType, subjectID, profile, destination string, port int, protocol string, workloadTypes []string, justification string, expiresAt *time.Time) Rule {
	now := time.Now().UTC()
	subjectType = normalizeSubjectType(subjectType)
	subjectID = strings.TrimSpace(subjectID)
	if subjectID == "" {
		subjectID = strings.TrimSpace(requesterID)
	}
	return Rule{
		ID:            "egress-" + randomID(),
		ProjectID:     strings.TrimSpace(projectID),
		RequesterID:   strings.TrimSpace(requesterID),
		SubjectType:   subjectType,
		SubjectID:     subjectID,
		Profile:       normalizeProfile(profile),
		Destination:   strings.ToLower(strings.TrimSpace(destination)),
		Port:          port,
		Protocol:      normalizeProtocol(protocol),
		WorkloadTypes: normalizeWorkloadTypes(workloadTypes),
		Justification: strings.TrimSpace(justification),
		Status:        "pending",
		ExpiresAt:     expiresAt,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func normalizeSubjectType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "organization" {
		return value
	}
	return "user"
}

func Profiles() []Profile {
	return []Profile{
		{ID: "isolated", Name: "Isolé", Description: "Aucun accès Internet direct. Profil par défaut pour les workloads sensibles.", Default: true, HDSAllowed: true},
		{ID: "internal-only", Name: "Interne uniquement", Description: "Accès aux services internes autorisés de la plateforme uniquement.", HDSAllowed: true},
		{ID: "package-access", Name: "Paquets et Git", Description: "Accès à des miroirs Git/package approuvés.", HDSAllowed: false},
		{ID: "approved-egress", Name: "Destination approuvée", Description: "Destination externe explicite, limitée, auditable et expirante.", HDSAllowed: false},
		{ID: "hds-regulated", Name: "HDS régulé", Description: "Connecteurs régulés sans Internet direct pour données HDS.", HDSAllowed: true, AdminOnly: true},
		{ID: "unrestricted", Name: "Exception temporaire", Description: "Accès exceptionnel administrateur, strictement borné dans le temps.", HDSAllowed: false, AdminOnly: true},
	}
}

func ValidProfile(value string) bool {
	profile := normalizeProfile(value)
	for _, item := range Profiles() {
		if item.ID == profile {
			return true
		}
	}
	return false
}

func normalizeProfile(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "isolated"
	}
	return value
}

func normalizeProtocol(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return "HTTPS"
	}
	return value
}

func normalizeWorkloadTypes(values []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, value := range values {
		v := strings.ToLower(strings.TrimSpace(value))
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	if len(out) == 0 {
		return []string{"workspace", "job", "app"}
	}
	return out
}

func randomID() string {
	return strings.ReplaceAll(uuid.NewString(), "-", "")
}
