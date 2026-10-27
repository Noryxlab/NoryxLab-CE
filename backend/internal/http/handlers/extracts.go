package handlers

import (
	"encoding/json"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/platformrole"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	extractdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/extract"
	ontologydomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
)

// Extracts: a named selection of files, frozen when it is declared.
//
// The question a study starts from is "the subjects with a corneal wavefront,
// first visit" - and the honest way to answer it twice is to resolve it once
// and keep the list. An extract that re-ran its filter would return a different
// study every month, which is how an n stops being reproducible.
//
// Nothing is copied. The members are keys in the dataset where the data already
// lives; a mount builds a tree of links over them. The source bucket is read
// only to the platform, and stays that way.

const extractMemberPageSize = 500

type extractRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	ProjectID   string   `json:"projectId"`
	Subjects    []string `json:"subjects"`
	Modalities  []string `json:"modalities"`
	Visits      []string `json:"visits"`
	// Formats narrows to kinds of file - DICOM, CSV, PNG.
	//
	// It is the axis that makes a disclosure smaller than the folder it lives
	// in. On PREMYOM1000's ANTERION, the measurements are 349 CSV files
	// totalling 37 MB and the images are 20,115 files totalling 41 GB; before
	// this, somebody who needed the numbers was handed the images too,
	// because the modality was the smallest thing anybody could ask for.
	Formats []string `json:"formats"`
	// Tables narrows to measurement tables - Cornea_Basics, Cornea_Ectasia.
	// The axis below the modality, and the one somebody means when they ask
	// for a fourth level: on PREMYOM1000's ANTERION the fourth path segment
	// holds three folder names and 349 file names, while the tables are
	// seven, each held by all thirty subjects.
	Tables []string `json:"tables"`
	// DataTypes narrows to the folders a modality files below itself -
	// CorneaMaps, SegmentOverlays, DICOM.
	//
	// It is the axis the other two cannot express. SELENA's ANTERION holds
	// 175 segmentation overlays and, inside its DICOM export, five images
	// belonging to the web viewer: both are PNG, so selecting by kind hands
	// over the viewer's furniture with the measurements. The folder is what
	// the instrument itself separated, and what the study was re-filed to
	// make selectable.
	DataTypes []string `json:"dataTypes"`
	// Layout is the order of the directory levels the mount builds. Empty is
	// the default, subject first, which is what every extract carried before
	// the order could be chosen.
	Layout []string `json:"layout"`
}

func (h Handlers) CreateExtract(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	// La matrice decide (ADR-049).
	if !h.requireCapability(w, identity, platformrole.CreateExtract) {
		return
	}
	if h.extractStore == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "extracts require a persistent store"})
		return
	}
	ontologyID := strings.TrimSpace(r.PathValue("ontologyID"))
	item, found, err := h.ontologyStore.GetByID(ontologyID)
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "ontology not found"})
		return
	}
	if !h.isGlobalAdmin(identity) && h.ontologyRole(item, identity) == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "ontology not found"})
		return
	}

	var req extractRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	members, err := h.ontologyStore.ListObjects(ontologyID, ontologydomain.ObjectFilter{
		Subjects:   req.Subjects,
		Modalities: req.Modalities,
		Visits:     req.Visits,
		Formats:    req.Formats,
		Tables:     req.Tables,
		DataTypes:  req.DataTypes,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to resolve the extract"})
		return
	}
	// An ontology scanned before the platform kept its file list resolves to
	// nothing, and the fix is a rescan, not an extract of zero files that looks
	// like a legitimate empty result.
	if len(members) == 0 {
		stored, countErr := h.ontologyStore.CountObjects(ontologyID)
		if countErr == nil && stored == 0 {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "this ontology was scanned before file paths were kept; rescan it to build extracts from it"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no object matches this selection"})
		return
	}

	// A project is optional, and attaching is a separate gesture.
	//
	// It used to be required: an extract that named no project was refused,
	// and the catalogue screen had to ask for one before it could freeze a
	// selection. But what an extract describes has nothing to do with which
	// projects mount it - that is a later question, and one with more than one
	// answer. Naming a project here simply attaches it straight away, which is
	// the common case and saves a second click.
	projectID := strings.TrimSpace(req.ProjectID)
	if projectID == "" && h.projectResourceStore != nil {
		// Attached to exactly one project, the ontology answers for itself.
		linked, err := h.projectResourceStore.ListOntologyProjectIDs(ontologyID)
		if err == nil && len(linked) == 1 {
			projectID = linked[0]
		}
	}

	layout, probleme := extractdomain.NormaliseLayout(req.Layout)
	if probleme != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": probleme})
		return
	}
	// A layout that names fewer than three levels merges what it leaves out,
	// and the mount builds links: two files on one path means one of them is
	// not there. Checked against the selection, because only the files can
	// say - dropping the visit is harmless on a study with one visit per
	// patient and destructive on the next study along.
	if conflit, collides := layoutCollision(layout, members); collides {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": conflit})
		return
	}

	object := extractdomain.New(identity.UserID(), ontologyID, projectID, req.Name, req.Description, req.Subjects, req.Modalities, req.Visits, req.Formats, req.Tables, req.DataTypes)
	// La paternite suit l'ontologie decoupee, comme celle de l'ontologie suit
	// le dataset lu : declarer une vue sur l'etude de quelqu'un n'en fait pas
	// la sienne.
	object.InheritOwner(item.OwnerType, item.OwnerID)
	object.Layout = layout
	frozen := make([]extractdomain.Member, 0, len(members))
	for _, member := range members {
		object.TotalBytes += member.SizeBytes
		frozen = append(frozen, extractdomain.Member{
			ExtractID: object.ID,
			Path:      member.Path,
			SubjectID: member.SubjectID,
			Visit:     member.Visit,
			Modality:  member.Modality,
			SizeBytes: member.SizeBytes,
		})
	}
	object.ObjectCount = len(frozen)

	if err := h.extractStore.Create(object, frozen); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create the extract"})
		return
	}
	if projectID != "" && h.projectResourceStore != nil {
		// Un echec de rattachement ne doit pas perdre l extrait : il existe,
		// il est gele, et il se rattache en un geste de plus.
		if err := h.projectResourceStore.AttachExtract(projectID, object.ID); err != nil {
			log.Printf("extract %s created but not attached to project %s: %v", object.ID, projectID, err)
		}
	}
	h.emitAudit(r, identity.UserID(), "extract.create", "extract", object.ID, object.ProjectID, "success", "", map[string]any{
		"ontologyId":  ontologyID,
		"objectCount": object.ObjectCount,
	})
	writeJSON(w, http.StatusCreated, object)
}

func (h Handlers) ListOntologyExtracts(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	ontologyID := strings.TrimSpace(r.PathValue("ontologyID"))
	item, found, err := h.ontologyStore.GetByID(ontologyID)
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "ontology not found"})
		return
	}
	// Voir n'est pas lire : l'administrateur de l'objet entre ici, et
	// n'ouvre pas pour autant un seul fichier.
	if !h.canSeeOntology(item, identity) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "ontology not found"})
		return
	}
	if h.extractStore == nil {
		writeJSON(w, http.StatusOK, []extractdomain.Extract{})
		return
	}
	items, err := h.extractStore.ListByOntology(ontologyID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list extracts"})
		return
	}
	h.nameExtractOwners(items)
	// La liste des sujets figes est du contenu, pas une etiquette.
	h.redactExtractsForReader(items, identity)
	writeJSON(w, http.StatusOK, items)
}

// GetExtractMembers is the frozen list itself: what a mount reads, and what a
// reviewer asks for when they want to know which files an n was computed over.
func (h Handlers) GetExtractMembers(w http.ResponseWriter, r *http.Request) {
	identity, item, ok := h.requireExtractContent(w, r)
	if !ok {
		return
	}
	limit := extractMemberPageSize
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	members, err := h.extractStore.ListMembers(item.ID, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list the extract's files"})
		return
	}
	subjects := map[string]bool{}
	for _, member := range members {
		subjects[member.SubjectID] = true
	}
	names := make([]string, 0, len(subjects))
	for name := range subjects {
		names = append(names, name)
	}
	sort.Strings(names)
	_ = identity
	// Lire la liste figee laisse une trace : ce sont des chemins qui nomment
	// des patients, leurs visites et leurs examens.
	h.emitAudit(r, identity.UserID(), "extract.members.listed", "extract", item.ID, item.ProjectID,
		"success", "", map[string]any{
			"extract": item.Name, "ontologyId": item.OntologyID,
			"members": len(members), "subjects": len(names),
		})
	writeJSON(w, http.StatusOK, map[string]any{
		"extract":  item,
		"members":  members,
		"subjects": names,
		// The page, not the extract: a reader must not mistake 500 shown files
		// for the size of the study.
		"shown": len(members),
		"total": item.ObjectCount,
	})
}

func (h Handlers) DeleteExtract(w http.ResponseWriter, r *http.Request) {
	identity, item, ok := h.requireExtract(w, r)
	if !ok {
		return
	}
	if !h.canManageExtract(item, identity) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "only the extract's owner can delete it"})
		return
	}
	if err := h.extractStore.Delete(item.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete the extract"})
		return
	}
	h.emitAudit(r, identity.UserID(), "extract.delete", "extract", item.ID, item.ProjectID, "success", "", nil)
	w.WriteHeader(http.StatusNoContent)
}

// An extract is visible to whoever can read the ontology it came from, and to
// whoever it was granted to.
//
// The second half is what makes an extract a disclosure rather than a
// bookmark: handing a selection to a partner used to mean handing them the
// whole study, or transferring the extract and losing it.
func (h Handlers) requireExtract(w http.ResponseWriter, r *http.Request) (auth.Identity, extractdomain.Extract, bool) {
	return h.resolveExtract(w, r, false)
}

// requireExtractContent is the same lookup for the handlers that hand back the
// frozen list itself - the file paths, which name patients.
//
// Separate from requireExtract because administering an extract is not reading
// it: the organization that owns one decides who it goes to, and that decision
// does not open the list. Both still answer 404, because whether an extract
// exists is itself information about regulated data.
func (h Handlers) requireExtractContent(w http.ResponseWriter, r *http.Request) (auth.Identity, extractdomain.Extract, bool) {
	return h.resolveExtract(w, r, true)
}

func (h Handlers) resolveExtract(w http.ResponseWriter, r *http.Request, contenu bool) (identity auth.Identity, item extractdomain.Extract, ok bool) {
	resolved, authorised := h.requireIdentity(w, r)
	if !authorised {
		return identity, item, false
	}
	if h.extractStore == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "extract not found"})
		return identity, item, false
	}
	found := false
	var err error
	item, found, err = h.extractStore.GetByID(strings.TrimSpace(r.PathValue("extractID")))
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "extract not found"})
		return identity, item, false
	}
	ontology, foundOntology, err := h.ontologyStore.GetByID(item.OntologyID)
	if err != nil || !foundOntology {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "extract not found"})
		return identity, item, false
	}
	// Un lecteur de l'extrait entre ici sans rien devoir a l'ontologie : c'est
	// tout l'objet d'une divulgation plus petite que l'etude.
	admis := h.canReadExtract(item, ontology, resolved)
	if !contenu {
		admis = admis || h.canManageExtract(item, resolved)
	}
	if !admis {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "extract not found"})
		return identity, item, false
	}
	return resolved, item, true
}

// canReadExtract is the right to open the selection: the extract was lent to
// this caller, or they may read the study it was cut from.
func (h Handlers) canReadExtract(item extractdomain.Extract, ontology ontologydomain.Ontology, identity auth.Identity) bool {
	if h.extractRole(item, identity) != "" {
		return true
	}
	return h.isGlobalAdmin(identity) || h.ontologyRole(ontology, identity) != ""
}

// extractRole answers what this person may do with the extract itself.
//
// "owner" for whoever it belongs to, "reader" for a subject it was granted
// to, empty for everybody else. Separate from the ontology's role on purpose:
// an extract exists to be handed over without its study, so being allowed to
// read a selection must not require being allowed to read what it was cut
// from.
func (h Handlers) extractRole(item extractdomain.Extract, identity auth.Identity) string {
	// Les sujets "donnee" seulement - la personne et ses equipes.
	//
	// Cette fonction appelait canManageExtract, qui compte les organisations
	// depuis que celles-ci administrent ce qu'elles possedent. Un seul "if"
	// transformait donc "j'administre" en "je lis", et chaque membre de
	// l'organisation proprietaire ouvrait la liste des fichiers de l'extrait
	// - la porte qu'on venait de fermer un etage plus haut, laissee ouverte
	// un etage plus bas.
	ownerType, ownerID := item.OwnerType, item.OwnerID
	if strings.TrimSpace(ownerType) == "" {
		ownerType, ownerID = "user", item.OwnerUserID
	}
	for _, subject := range h.ontologySubjects(identity) {
		if strings.EqualFold(ownerType, subject.Type) && strings.EqualFold(ownerID, subject.ID) {
			return "owner"
		}
	}
	if h.extractStore == nil {
		return ""
	}
	// Les octrois explicites comptent les organisations, la propriete non.
	//
	// Un octroi est un acte : quelqu'un a decide que telle organisation lit
	// cette donnee, et l'ecran le propose. Il ne se lisait plus - la liste
	// des sujets avait perdu les organisations pour fermer l'heritage de
	// propriete - donc la plateforme acceptait un droit, l'affichait dans la
	// matrice, et ne l'appliquait pas. L'heritage ferme est celui de la
	// propriete ; ce qui est donne nommement s'applique.
	for _, subject := range h.ontologyGrantSubjects(identity) {
		access, found, err := h.extractStore.GetAccess(item.ID, subject.Type, subject.ID)
		if err == nil && found && strings.TrimSpace(access.Role) != "" {
			return "reader"
		}
	}
	return ""
}

// canManageExtract reports whether this person may hand the extract on, or
// delete it.
//
// Read from the owner, not from the author. The check used to compare
// OwnerUserID - whoever declared the extract - which was the same thing while
// an extract could not be transferred. The moment it can, the two part company:
// hand an extract to an organization and its author would keep the right to
// delete it while a member of the owning organization would not.
//
// The author keeps the extract only until somebody moves it, which is exactly
// what the default owner says.
func (h Handlers) canManageExtract(item extractdomain.Extract, identity auth.Identity) bool {
	if h.isGlobalAdmin(identity) {
		return true
	}
	// Les sujets d'administration : l'organisation proprietaire administre ce
	// qu'elle possede, sans que cela lui ouvre la donnee.
	for _, subject := range h.ontologyAdminSubjects(identity) {
		if strings.EqualFold(item.OwnerType, subject.Type) && strings.EqualFold(item.OwnerID, subject.ID) {
			return true
		}
	}
	// Les lignes anterieures a la migration n ont pas de proprietaire : leur
	// auteur garde la main plutot que personne.
	return strings.TrimSpace(item.OwnerType) == "" && item.OwnerUserID == identity.UserID()
}

// UpdateExtractOwner hands an extract to a person or an organization, in the
// same shape and the same words as a dataset or an ontology.
func (h Handlers) UpdateExtractOwner(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.extractStore.GetByID(strings.TrimSpace(r.PathValue("extractID")))
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "extract not found"})
		return
	}
	if !h.canManageExtract(item, identity) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "extract owner or global admin required"})
		return
	}
	var req setDatasetOwnerRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid ownerType and ownerId are required"})
		return
	}
	ownerType, ownerID, status, probleme := h.normaliseOwner(req.OwnerType, req.OwnerID, identity)
	if status != 0 {
		writeJSON(w, status, map[string]string{"error": probleme})
		return
	}
	req.OwnerType, req.OwnerID = ownerType, ownerID
	if err := h.extractStore.SetOwner(item.ID, req.OwnerType, req.OwnerID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to transfer the extract"})
		return
	}
	// A frozen selection: the former owner keeps reading, and an owning
	// organization's members read at most.
	h.settleOwnership(firstNonEmpty(item.OwnerType, "user"), firstNonEmpty(item.OwnerID, item.OwnerUserID), req.OwnerType, req.OwnerID, "reader",
		func(subjectType, subjectID, role string) error {
			now := time.Now().UTC()
			return h.extractStore.SetAccess(extractdomain.Access{ExtractID: item.ID, SubjectType: subjectType, SubjectID: subjectID, Role: "reader", CreatedAt: now, UpdatedAt: now})
		})
	h.emitAudit(r, identity.UserID(), "extract.owner.changed", "extract", item.ID, item.ProjectID,
		"success", "", map[string]any{"ownerType": req.OwnerType, "ownerId": req.OwnerID,
			"previousOwnerType": item.OwnerType, "previousOwnerId": item.OwnerID})
	updated, _, _ := h.extractStore.GetByID(item.ID)
	writeJSON(w, http.StatusOK, updated)
}

// UpdateExtractMetadata corrects the label a person typed.
//
// The name is typed at declaration - "anterion-3-sujets" - and a typed name is
// a name that gets typed wrong. It could not be changed, so the only way to
// fix one was to declare the selection again: a different frozen list over a
// bucket that keeps growing, which means a rename produced a different study.
// The ontology got the same screen for the same reason on 2026-10-01.
//
// The label moves and nothing else does. The frozen file list, the author and
// the dates are what the extract is, and an n already published against this
// name still describes the same files.
func (h Handlers) UpdateExtractMetadata(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.extractStore.GetByID(strings.TrimSpace(r.PathValue("extractID")))
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "extract not found"})
		return
	}
	if !h.canManageExtract(item, identity) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "extract owner or global admin required"})
		return
	}
	var req updateDatasetMetadataRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid name and description are required"})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if err := h.extractStore.UpdateMetadata(item.ID, req.Name, req.Description); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to rename the extract"})
		return
	}
	h.emitAudit(r, identity.UserID(), "extract.renamed", "extract", item.ID, item.ProjectID,
		"success", "", map[string]any{"name": strings.TrimSpace(req.Name), "previousName": item.Name})
	updated, _, _ := h.extractStore.GetByID(item.ID)
	writeJSON(w, http.StatusOK, updated)
}

// AttachProjectExtract mounts an extract in a project's workspaces.
//
// The same gesture as attaching a dataset or an ontology, and for the same
// reason: what an extract describes has nothing to do with which projects use
// it. It used to be decided once, when the extract was declared, and never
// again - so an extract made in the wrong project simply never appeared.
func (h Handlers) AttachProjectExtract(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	extractID := strings.TrimSpace(r.PathValue("extractID"))
	if projectID == "" || extractID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectID and extractID are required"})
		return
	}
	if !h.requireProjectRole(w, projectID, identity.UserID(), actionAttachOntology, "extract attach") {
		return
	}
	item, found, err := h.extractStore.GetByID(extractID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read extract"})
		return
	}
	// Not found rather than forbidden when it is not yours: naming an
	// identifier should not confirm that it exists.
	// Le lecteur de l'extrait compte, lui aussi : c'est ce que le pret
	// promet. Une equipe a qui on confie une selection doit pouvoir la
	// monter dans son propre projet, sans quoi on lui a donne un droit
	// qu'elle ne peut exercer nulle part.
	if !found || (h.extractRole(item, identity) == "" &&
		!h.canManageExtract(item, identity) &&
		!h.canReadOntologyObjectID(item.OntologyID, identity)) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "extract not found"})
		return
	}
	// A selection drawn from regulated data is attached by an administrator.
	//
	// Attaching an extract now delivers its bytes: since the project no longer
	// has to attach the bucket, the filler reads it on the workload's behalf
	// and the selection appears under /extracts. That is the feature, and it
	// moves where the decision has to be taken. Before it, reaching an HDS
	// selection required a grant on the dataset, and the grant was the
	// decision; after it, attaching the extract is the decision.
	//
	// So the two are kept at the same level. A project role is enough for
	// ordinary data. For data classified hds, the person who attaches it
	// carries platform authority - which is what Cyril and Cedric already hold
	// - and the audit entry names them.
	if classification, err := h.extractSourceClassification(item.OntologyID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read what this extract is drawn from"})
		return
	} else if classification == "hds" && !h.requireCapability(w, identity, platformrole.AttachRegulatedData) {
		// Drawn from regulated data: the matrix says who attaches it (ADR-051).
		// The administrator row carries the box; an installation ticks it on
		// a row of its own for the people who answer for the data.
		return
	}
	if err := h.projectResourceStore.AttachExtract(projectID, extractID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to attach the extract"})
		return
	}
	h.emitAudit(r, identity.UserID(), "extract.attached", "extract", extractID, projectID, "success", "", nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "attached"})
}

// DetachProjectExtract stops mounting it. The extract itself is untouched: a
// frozen selection outlives the projects that used it.
func (h Handlers) DetachProjectExtract(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	extractID := strings.TrimSpace(r.PathValue("extractID"))
	if projectID == "" || extractID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectID and extractID are required"})
		return
	}
	if !h.requireProjectRole(w, projectID, identity.UserID(), actionAttachOntology, "extract detach") {
		return
	}
	if err := h.projectResourceStore.DetachExtract(projectID, extractID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to detach the extract"})
		return
	}
	h.emitAudit(r, identity.UserID(), "extract.detached", "extract", extractID, projectID, "success", "", nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "detached"})
}

// ListProjectExtracts is what this project mounts.
//
// The screen that attaches needs to show what is already attached, and the
// link table is the only thing that knows: an extract no longer carries the
// project that mounts it.
func (h Handlers) ListProjectExtracts(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectID is required"})
		return
	}
	if !h.requireProjectRole(w, projectID, identity.UserID(), actionRead, "extract list") {
		return
	}
	items, err := h.projectExtracts(projectID)
	if err != nil {
		h.nameExtractOwners(items)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read extracts"})
		return
	}
	// Ce que cette personne peut lire ou administrer, comme partout ailleurs.
	//
	// Le seul role de projet gardait cette liste : elle nommait les extraits
	// attaches, leur n et leur etude, a tout membre du projet - y compris a
	// qui n'a aucun droit sur l'etude dont ils sont tires.
	visibles := make([]extractdomain.Extract, 0, len(items))
	for _, item := range items {
		ontology, found, err := h.ontologyStore.GetByID(item.OntologyID)
		if err != nil || !found {
			continue
		}
		if h.canReadExtract(item, ontology, identity) || h.canManageExtract(item, identity) {
			visibles = append(visibles, item)
		}
	}
	items = visibles
	h.nameExtractOwners(items)
	h.redactExtractsForReader(items, identity)
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// ListExtracts is the catalogue entry: every extract the caller can reach,
// without naming an ontology first.
//
// Visibility is the ontology's, plus what was lent.
//
// An extract drawn from an ontology somebody cannot read stays invisible to
// them - a selection over a study they have no business seeing. The exception
// is the one the object exists for: an extract granted to them, which they
// must find in their own catalogue, because a right with no path to exercise
// it is not a right. Ownership still decides who may transfer or delete it.
func (h Handlers) ListExtracts(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	if h.extractStore == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []extractdomain.Extract{}})
		return
	}
	ontologies, err := h.ontologiesVisibleTo(identity)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list extracts"})
		return
	}
	items := []extractdomain.Extract{}
	for _, ontology := range ontologies {
		listed, err := h.extractStore.ListByOntology(ontology.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list extracts"})
			return
		}
		items = append(items, listed...)
	}
	// Et ce qui leur a ete prete, sans doublon avec ce qui precede.
	vus := map[string]bool{}
	for _, item := range items {
		vus[item.ID] = true
	}
	for _, subject := range h.ontologySubjects(identity) {
		pretes, err := h.extractStore.ListGrantedTo(subject.Type, subject.ID)
		if err != nil {
			continue
		}
		for _, item := range pretes {
			if !vus[item.ID] {
				vus[item.ID] = true
				items = append(items, item)
			}
		}
	}
	h.nameExtractOwners(items)
	h.redactExtractsForReader(items, identity)
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// ListExtractAccess shows who may read this extract besides its owner.
func (h Handlers) ListExtractAccess(w http.ResponseWriter, r *http.Request) {
	identity, item, ok := h.requireExtract(w, r)
	if !ok {
		return
	}
	// Voir la liste n'est pas la changer : un lecteur a le droit de savoir
	// avec qui il partage, le proprietaire seul decide.
	grants, err := h.extractStore.ListAccess(item.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list extract permissions"})
		return
	}
	// Le proprietaire en tete, comme sur un dataset : la liste repond "qui
	// peut lire", et le proprietaire en fait partie.
	tous := append([]extractdomain.Access{{
		ExtractID: item.ID, SubjectType: item.OwnerType, SubjectID: item.OwnerID,
		Role: "owner", CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}}, grants...)
	h.nameExtractAccessSubjects(tous)
	writeJSON(w, http.StatusOK, map[string]any{
		"items":     tous,
		"canManage": h.canManageExtract(item, identity),
	})
}

// SetExtractAccess grants a read on this extract to a person, a team or an
// organization.
func (h Handlers) SetExtractAccess(w http.ResponseWriter, r *http.Request) {
	identity, item, ok := h.requireExtract(w, r)
	if !ok {
		return
	}
	if !h.canManageExtract(item, identity) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "extract owner or global admin required"})
		return
	}
	subjectType, subjectID, status, probleme := h.grantSubject(r)
	if status != 0 {
		writeJSON(w, status, map[string]string{"error": probleme})
		return
	}
	if strings.EqualFold(subjectType, item.OwnerType) && strings.EqualFold(subjectID, item.OwnerID) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "owner role cannot be changed"})
		return
	}
	// Un extrait est gele : il n'y a rien a y ecrire, donc un seul role.
	now := time.Now().UTC()
	access := extractdomain.Access{ExtractID: item.ID, SubjectType: subjectType, SubjectID: subjectID, Role: "reader", CreatedAt: now, UpdatedAt: now}
	if existing, exists, _ := h.extractStore.GetAccess(item.ID, subjectType, subjectID); exists {
		access.CreatedAt = existing.CreatedAt
	}
	if err := h.extractStore.SetAccess(access); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to set extract permission"})
		return
	}
	h.emitAdvancedAudit(r, identity.UserID(), "extract.access.grant", "extract", item.ID, item.ProjectID, "success", "", map[string]any{
		"subjectType": subjectType, "subjectId": subjectID, "role": access.Role, "name": item.Name,
	})
	writeJSON(w, http.StatusOK, access)
}

// DeleteExtractAccess takes that read back.
func (h Handlers) DeleteExtractAccess(w http.ResponseWriter, r *http.Request) {
	identity, item, ok := h.requireExtract(w, r)
	if !ok {
		return
	}
	if !h.canManageExtract(item, identity) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "extract owner or global admin required"})
		return
	}
	subjectType, subjectID, status, probleme := h.grantSubject(r)
	if status != 0 {
		writeJSON(w, status, map[string]string{"error": probleme})
		return
	}
	if err := h.extractStore.DeleteAccess(item.ID, subjectType, subjectID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete extract permission"})
		return
	}
	h.emitAdvancedAudit(r, identity.UserID(), "extract.access.revoke", "extract", item.ID, item.ProjectID, "success", "", map[string]any{
		"subjectType": subjectType, "subjectId": subjectID, "name": item.Name,
	})
	w.WriteHeader(http.StatusNoContent)
}

// grantSubject reads who a grant is about, and resolves a name to its
// identifier - a team is granted by the name somebody reads on a screen, not
// by a UUID they would have to look up first.
func (h Handlers) grantSubject(r *http.Request) (string, string, int, string) {
	subjectType := strings.TrimSpace(r.PathValue("subjectType"))
	subjectID := strings.TrimSpace(r.PathValue("subjectID"))
	if subjectType == "" {
		subjectType = "user"
		subjectID = strings.TrimSpace(r.PathValue("userID"))
	}
	if !isGrantableSubjectType(subjectType) || subjectID == "" {
		return "", "", http.StatusBadRequest, "valid subjectType and subjectID are required"
	}
	switch strings.ToLower(subjectType) {
	case "organization":
		organization, found := h.resolveOrganization(subjectID)
		if !found {
			return "", "", http.StatusBadRequest, "no organization named " + subjectID
		}
		subjectID = organization.ID
	case "team":
		team, found := h.resolveTeam(subjectID)
		if !found {
			return "", "", http.StatusBadRequest, "no team with identifier " + subjectID
		}
		subjectID = team.ID
	}
	return subjectType, subjectID, 0, ""
}
