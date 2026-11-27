package handlers

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/attestation"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/objectstore"
)

// Training attestations: a document somebody uploads to say they were trained
// to handle confidential data.
//
// Nothing here refuses anything. The platform records the document and the
// date it stops being valid, shows an administrator who is covered, and stops
// there - "rien de coercitif pour l'instant", 2026-11-27. Collecting before
// enforcing is the order that works: an installation that starts requiring a
// field nobody has filled locks everybody out on the first day.
const (
	// Enough for a scanned certificate, small enough that nobody stores a
	// training video here.
	attestationMaxBytes = 10 << 20
	// The bucket is the platform's own and never a dataset's. These are
	// documents about people - a name, an employer, sometimes a date of
	// birth - and they have no business sitting next to research data where
	// a project's members can list them.
	attestationBucket = "noryx-attestations"
)

type attestationView struct {
	attestation.Attestation
	// Status is computed rather than stored: a document does not expire by
	// being written to, it expires by the date passing.
	Status string `json:"status"`
	// UserName is filled for an administrator reading the whole list, where
	// a column of identifiers answers nothing.
	UserName string `json:"userName,omitempty"`
}

func attestationViewOf(item attestation.Attestation, now time.Time) attestationView {
	return attestationView{Attestation: item, Status: item.Status(now)}
}

// ListMyAttestations is what a person sees about themselves.
func (h Handlers) ListMyAttestations(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	if h.attestationStore == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []attestationView{}})
		return
	}
	items, err := h.attestationStore.ListByUser(userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read your attestations"})
		return
	}
	now := time.Now().UTC()
	views := make([]attestationView, 0, len(items))
	for _, item := range items {
		views = append(views, attestationViewOf(item, now))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": views})
}

// UploadAttestation takes the document itself.
//
// Multipart rather than a JSON body with base64: the file is the point, it is
// megabytes, and every HTTP client and every browser form already knows how
// to send one.
func (h Handlers) UploadAttestation(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	if h.attestationStore == nil || h.minioClient == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{
			"error": "attestations need the database and the platform's object storage"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, attestationMaxBytes+(1<<20))
	if err := r.ParseMultipartForm(attestationMaxBytes); err != nil {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{
			"error": fmt.Sprintf("the document must be a PDF of at most %d MiB", attestationMaxBytes>>20)})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a file is required"})
		return
	}
	defer file.Close()

	content, err := io.ReadAll(io.LimitReader(file, attestationMaxBytes+1))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "the document could not be read"})
		return
	}
	if int64(len(content)) > attestationMaxBytes {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{
			"error": fmt.Sprintf("the document must be at most %d MiB", attestationMaxBytes>>20)})
		return
	}
	// Checked on the bytes, not on the name. A file called .pdf that is not
	// one is the ordinary way this goes wrong - somebody exports a Word
	// document and renames it - and the person who finds out otherwise is an
	// auditor opening it a year later.
	if !bytes.HasPrefix(content, []byte("%PDF-")) {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "the document must be a PDF (its contents do not start like one)"})
		return
	}

	validUntil, err := parseOptionalDate(r.FormValue("validUntil"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validUntil must be a date, as 2027-03-31"})
		return
	}

	record := attestation.Attestation{
		ID:          uuid.NewString(),
		UserID:      userID,
		Subject:     strings.TrimSpace(r.FormValue("subject")),
		FileName:    sanitizeAttestationName(header.Filename),
		ContentType: "application/pdf",
		Size:        int64(len(content)),
		ValidUntil:  validUntil,
		UploadedAt:  time.Now().UTC(),
	}
	record.StorageKey = record.UserID + "/" + record.ID + ".pdf"

	store := objectstore.NewS3(h.minioClient, h.minioRegion)
	exists, err := store.BucketExists(r.Context(), attestationBucket)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "the platform's storage did not answer"})
		return
	}
	if !exists {
		if err := store.MakeBucket(r.Context(), attestationBucket, ""); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "the attestation store could not be created"})
			return
		}
	}
	if _, err := store.Put(r.Context(), attestationBucket, record.StorageKey,
		bytes.NewReader(content), record.Size, record.ContentType); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "the document could not be stored"})
		return
	}
	// The row last: an object with no row is invisible and harmless, a row
	// with no object is a download that fails for somebody who was told it
	// had worked.
	if err := h.attestationStore.Create(record); err != nil {
		_ = store.Remove(r.Context(), attestationBucket, record.StorageKey)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "the document could not be recorded"})
		return
	}
	h.emitAudit(r, userID, "attestation.uploaded", "user", userID, "", "success", "",
		map[string]any{"attestationId": record.ID, "subject": record.Subject,
			"fileName": record.FileName, "validUntil": record.ValidUntil})
	writeJSON(w, http.StatusCreated, attestationViewOf(record, time.Now().UTC()))
}

// DownloadAttestation hands back the document, to its owner or to a platform
// administrator and to nobody else.
//
// Not to the person's colleagues, not to a project administrator: it carries
// a name, an employer and sometimes a date of birth, and the fact that
// somebody is on a team says nothing about their right to read that.
func (h Handlers) DownloadAttestation(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	record, allowed := h.attestationFor(w, r, identity.UserID())
	if !allowed {
		return
	}
	store := objectstore.NewS3(h.minioClient, h.minioRegion)
	reader, err := store.Get(r.Context(), attestationBucket, record.StorageKey)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "the document is no longer in storage"})
		return
	}
	defer reader.Close()
	w.Header().Set("Content-Type", record.ContentType)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+record.FileName+"\"")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", record.Size))
	h.emitAudit(r, identity.UserID(), "attestation.downloaded", "user", record.UserID, "", "success", "",
		map[string]any{"attestationId": record.ID})
	_, _ = io.Copy(w, reader)
}

// DeleteAttestation removes a document and its row.
func (h Handlers) DeleteAttestation(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	record, allowed := h.attestationFor(w, r, identity.UserID())
	if !allowed {
		return
	}
	store := objectstore.NewS3(h.minioClient, h.minioRegion)
	// The object first here, and the row second, for the same reason as the
	// upload in reverse: a row pointing at nothing is worse than an object
	// nothing points at.
	_ = store.Remove(r.Context(), attestationBucket, record.StorageKey)
	if _, err := h.attestationStore.Delete(record.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "the attestation could not be removed"})
		return
	}
	h.emitAudit(r, identity.UserID(), "attestation.deleted", "user", record.UserID, "", "success", "",
		map[string]any{"attestationId": record.ID, "fileName": record.FileName})
	w.WriteHeader(http.StatusNoContent)
}

// ListAllAttestations is the administrator's view: who has produced what.
func (h Handlers) ListAllAttestations(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminModule(w, r, "users"); !ok {
		return
	}
	if h.attestationStore == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []attestationView{}})
		return
	}
	items, err := h.attestationStore.ListAll()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read the attestations"})
		return
	}
	// The directory is asked once, not once per row: a column of identifiers
	// answers nothing, and thirty lookups to produce thirty names is how an
	// administration screen becomes slow enough that nobody opens it.
	users, organizations, _ := h.dataUsageIdentityMaps()
	now := time.Now().UTC()
	views := make([]attestationView, 0, len(items))
	for _, item := range items {
		view := attestationViewOf(item, now)
		view.UserName = subjectName("user", item.UserID, users, organizations)
		views = append(views, view)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": views})
}

// attestationFor reads the attestation named by the request and decides
// whether this caller may see it at all.
func (h Handlers) attestationFor(w http.ResponseWriter, r *http.Request, callerID string) (attestation.Attestation, bool) {
	if h.attestationStore == nil || h.minioClient == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "attestations are not configured"})
		return attestation.Attestation{}, false
	}
	record, found, err := h.attestationStore.GetByID(strings.TrimSpace(r.PathValue("attestationID")))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read the attestation"})
		return attestation.Attestation{}, false
	}
	// Not found rather than forbidden for somebody else's document: a 403
	// confirms that a given identifier exists, and these are personal.
	if !found || (record.UserID != callerID && !h.isGlobalAdminUserID(callerID)) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "attestation not found"})
		return attestation.Attestation{}, false
	}
	return record, true
}

// parseOptionalDate reads a plain date, or nothing.
//
// A date rather than an instant: nobody knows what time of day a training
// certificate expires, and asking for one would invite a guess that is wrong
// by a timezone.
func parseOptionalDate(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, err
	}
	// The end of the named day, so a certificate valid "until 31 March" is
	// valid on 31 March.
	moment := parsed.UTC().Add(24*time.Hour - time.Second)
	return &moment, nil
}

// sanitizeAttestationName keeps something recognisable for the download and
// nothing that can escape a path or confuse a header.
func sanitizeAttestationName(raw string) string {
	raw = strings.TrimSpace(raw)
	if index := strings.LastIndexAny(raw, `/\`); index >= 0 {
		raw = raw[index+1:]
	}
	raw = strings.Map(func(r rune) rune {
		switch {
		case r == '"' || r == '\\' || r < 0x20:
			return -1
		}
		return r
	}, raw)
	if raw == "" {
		return "attestation.pdf"
	}
	if len(raw) > 120 {
		raw = raw[:120]
	}
	return raw
}
