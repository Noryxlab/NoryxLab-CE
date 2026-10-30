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

// The profile identifiers, named rather than retyped: the network policies in
// deploy/k8s/base select on these exact strings, so a typo in one of them is
// an open door rather than a compile error.
//
// Closed and open is the distinction that matters at the network layer.
// Isolated, internal-only and HDS-regulated deny direct internet access and
// are applied by a NetworkPolicy. Package-access and approved-egress name
// domains, which a NetworkPolicy cannot select on, so they remain recorded
// rather than applied - see ADR-023 and the gateway it calls for.
const (
	ProfileIsolated       = "isolated"
	ProfileInternalOnly   = "internal-only"
	ProfilePackageAccess  = "package-access"
	ProfileApprovedEgress = "approved-egress"
	ProfileHDSRegulated   = "hds-regulated"
	ProfileUnrestricted   = "unrestricted"
)

func Profiles() []Profile {
	return []Profile{
		{ID: ProfileIsolated, Name: "Isolé", Description: "Aucun accès Internet direct. Profil par défaut pour les workloads sensibles.", Default: true, HDSAllowed: true},
		{ID: ProfileInternalOnly, Name: "Interne uniquement", Description: "Accès aux services internes autorisés de la plateforme uniquement.", HDSAllowed: true},
		{ID: ProfilePackageAccess, Name: "Paquets et Git", Description: "Accès à des miroirs Git/package approuvés.", HDSAllowed: false},
		{ID: ProfileApprovedEgress, Name: "Destination approuvée", Description: "Destination externe explicite, limitée, auditable et expirante.", HDSAllowed: false},
		{ID: ProfileHDSRegulated, Name: "HDS régulé", Description: "Connecteurs régulés sans Internet direct pour données HDS.", HDSAllowed: true, AdminOnly: true},
		{ID: ProfileUnrestricted, Name: "Exception temporaire", Description: "Accès exceptionnel administrateur, strictement borné dans le temps.", HDSAllowed: false, AdminOnly: true},
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
		return ProfileIsolated
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
