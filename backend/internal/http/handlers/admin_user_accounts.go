package handlers

import (
	"crypto/rand"
	"encoding/json"
	"math/big"
	"net/http"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/iam/keycloak"
)

// Creating accounts and resetting passwords from the administration screen.
//
// Both were possible only through the Keycloak console, which means handing an
// administrator a second product, a second set of credentials and a surface far
// wider than the task. A platform that manages projects, datasets and access
// but sends someone elsewhere to add a colleague is not finished.
//
// Keycloak remains the source of truth for identity. Nothing here stores a
// user; these endpoints ask Keycloak, and Noryx decides only who may ask.

// passwordAlphabet excludes the characters people misread when a password is
// dictated or copied from a screen: no O/0, no l/1/I. A temporary password is
// read aloud or pasted into a chat far more often than anyone admits.
const passwordAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"

const temporaryPasswordLength = 20

type createUserRequest struct {
	Username  string `json:"username"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	// OrganizationID adds the account to an organization as it is created.
	//
	// On an Enterprise installation membership is mandatory, so an account
	// created without one can sign in and then do nothing - the phantom-user
	// shape that once left backups refused for three nights. The interface
	// therefore asks for it up front rather than leaving it to be discovered.
	OrganizationID string `json:"organizationId"`
}

func (h Handlers) CreateUserAccount(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireAdminModule(w, r, "users")
	if !ok {
		return
	}
	if h.keycloak == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "keycloak admin client is not configured"})
		return
	}

	var req createUserRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	username := strings.TrimSpace(req.Username)
	if username == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a username is required"})
		return
	}
	if h.organizationRequired && strings.TrimSpace(req.OrganizationID) == "" {
		// Refused rather than created half-formed: this installation requires
		// organization membership, so an account without one is an account that
		// signs in and can do nothing.
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "this installation requires organization membership; choose an organization",
			"code":  "organization_required",
		})
		return
	}

	userID, err := h.keycloak.CreateUser(keycloak.User{
		Username:  username,
		Email:     strings.TrimSpace(req.Email),
		FirstName: strings.TrimSpace(req.FirstName),
		LastName:  strings.TrimSpace(req.LastName),
	})
	if err != nil {
		h.writeKeycloakUserError(w, "create the account", err)
		return
	}

	password, err := temporaryPassword()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate a password"})
		return
	}
	if err := h.keycloak.SetTemporaryPassword(userID, password); err != nil {
		// The account exists and cannot be signed into. Said plainly, with the
		// identifier, so an administrator can finish the job rather than
		// wondering whether to create it again.
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error":  "the account was created but its password could not be set; reset it from the users list",
			"userId": userID,
		})
		return
	}

	if organizationID := strings.TrimSpace(req.OrganizationID); organizationID != "" {
		if err := h.keycloak.AddOrganizationMember(organizationID, userID); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{
				"error":  "the account was created but could not be added to the organization",
				"userId": userID,
			})
			return
		}
	}

	h.emitAudit(r, identity.UserID(), "user.created", "user", username, "", "success", "",
		map[string]any{"organizationId": strings.TrimSpace(req.OrganizationID)})

	// The password is in this response and nowhere else. It is temporary, so
	// the window in which the administrator's copy is valid ends at the user's
	// next sign-in.
	writeJSON(w, http.StatusCreated, map[string]any{
		"userId":            userID,
		"username":          username,
		"temporaryPassword": password,
		"note":              "shown once; the user must change it at first sign-in",
	})
}

func (h Handlers) ResetUserPassword(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireAdminModule(w, r, "users")
	if !ok {
		return
	}
	if h.keycloak == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "keycloak admin client is not configured"})
		return
	}
	userID := strings.TrimSpace(r.PathValue("userID"))
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a user is required"})
		return
	}

	password, err := temporaryPassword()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate a password"})
		return
	}
	if err := h.keycloak.SetTemporaryPassword(userID, password); err != nil {
		h.writeKeycloakUserError(w, "reset the password", err)
		return
	}

	h.emitAudit(r, identity.UserID(), "user.password.reset", "user", userID, "", "success", "", nil)

	writeJSON(w, http.StatusOK, map[string]any{
		"userId":            userID,
		"temporaryPassword": password,
		"note":              "shown once; the user must change it at first sign-in",
	})
}

func (h Handlers) writeKeycloakUserError(w http.ResponseWriter, action string, err error) {
	if keycloak.IsNotFound(err) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such user"})
		return
	}
	if keycloak.IsConflict(err) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "a user with that name or email already exists"})
		return
	}
	// Keycloak's own response body stays in the log: it is another system's
	// internals and the caller cannot act on it.
	h.writeKeycloakError(w, action, err)
}

// temporaryPassword generates one the platform chooses rather than the
// administrator.
//
// An administrator inventing passwords picks weak and reused ones, and does it
// under time pressure while somebody waits. Twenty characters from an
// unambiguous alphabet is about 115 bits, which is beyond guessing and still
// short enough to read out loud.
func temporaryPassword() (string, error) {
	out := make([]byte, temporaryPasswordLength)
	limit := big.NewInt(int64(len(passwordAlphabet)))
	for index := range out {
		pick, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", err
		}
		out[index] = passwordAlphabet[pick.Int64()]
	}
	return string(out), nil
}
