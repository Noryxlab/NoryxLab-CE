package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/extract"

// ExtractStore persists a named selection of files and the list it froze.
type ExtractStore interface {
	ListByProject(projectID string) ([]extract.Extract, error)
	// ListAll sert la matrice des acces : "qui peut lire quoi" ne peut pas
	// repondre pour un type d'objet qu'elle ne sait pas enumerer.
	ListAll() ([]extract.Extract, error)
	ListByOntology(ontologyID string) ([]extract.Extract, error)
	GetByID(id string) (extract.Extract, bool, error)
	Create(item extract.Extract, members []extract.Member) error
	ListMembers(extractID string, limit int) ([]extract.Member, error)
	// UpdateMetadata corrects the label, and only the label.
	//
	// The name is typed by a person at declaration - "anterion-3-sujets" - and
	// a typed name is a name that gets typed wrong. It was unchangeable, so
	// the only way to fix one was to declare the selection again, which
	// produces a different frozen list over a bucket that keeps growing: a
	// rename turned into a different study.
	UpdateMetadata(id, name, description string) error
	// SetOwner hands the extract to somebody else. Only the owner moves: the
	// frozen file list, its author and its dates are what the extract is, and
	// a transfer does not rewrite history.
	SetOwner(id, ownerType, ownerID string) error
	// Les lectures accordees sur un extrait, a cote de sa propriete.
	ListAccess(extractID string) ([]extract.Access, error)
	GetAccess(extractID, subjectType, subjectID string) (extract.Access, bool, error)
	SetAccess(item extract.Access) error
	DeleteAccess(extractID, subjectType, subjectID string) error
	// Les extraits pretes a un sujet, pour qu'un lecteur les trouve dans son
	// catalogue : un droit sans chemin pour l'exercer n'est pas un droit.
	ListGrantedTo(subjectType, subjectID string) ([]extract.Extract, error)
	Delete(id string) error
}
