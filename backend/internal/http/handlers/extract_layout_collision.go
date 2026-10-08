package handlers

import (
	"fmt"
	"strings"

	extractdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/extract"
	ontologydomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
)

// Whether a layout would file two of the selected files in the same place.
//
// A layout may now name one or two levels instead of three, which is what
// somebody wants on a study with a single visit per patient: the middle
// directory holds one entry per patient and exists only to be walked through.
// But dropping a level merges whatever it used to separate, and the mount
// builds a tree of links - two files landing on the same link path means one
// of them is not there.
//
// That failure has happened once already, by a different route: placing each
// file under its basename lost every repeated name, and 21 730 objects landed
// as 18 825 files on PREMYOM1000's ANTERION modality. A study silently 13%
// smaller is the worst way to be wrong about data, and nothing on screen said
// so. A layout is refused before it can do it again.
//
// Checked against the selection and not against the rule, because only the
// files can answer: dropping the visit is harmless on a study with one visit
// per patient and destructive on the next study along.

// layoutCollision reports the first two members a layout would place on the
// same path, and which levels would separate them again.
func layoutCollision(layout []string, members []ontologydomain.Object) (string, bool) {
	// The same composition the mount performs: the directory from the layout,
	// then the part of the source path below the modality. Anything else here
	// would check a tree nobody builds.
	seen := make(map[string]string, len(members))
	for _, member := range members {
		place := strings.Join(
			append(
				extractdomain.DirectoryFor(layout, member.SubjectID, member.Visit, member.Modality),
				extractLeaf(member.Path, member.Modality),
			),
			"/",
		)
		if first, already := seen[place]; already {
			return layoutCollisionMessage(layout, first, member.Path), true
		}
		seen[place] = member.Path
	}
	return "", false
}

// layoutCollisionMessage says what collided and what to add back.
//
// It names the two source paths because the person reading has to recognise
// their own data to believe the refusal, and it names the missing levels
// because "this layout collides" is not something anybody can act on.
func layoutCollisionMessage(layout []string, first, second string) string {
	missing := extractdomain.Omitted(layout)
	if len(missing) == 0 {
		// Three distinct levels cannot collide: two files in the same
		// subject, visit and modality keep their own path below it. Reaching
		// here means the leaves themselves repeat, which is a scan to look at
		// rather than a layout to fix.
		return fmt.Sprintf(
			"two files of this selection share a place in the tree (%s and %s); the layout is not the cause",
			first, second,
		)
	}
	return fmt.Sprintf(
		"this layout would file %s and %s in the same place, so one of them would be missing; add %s to the layout",
		first, second, strings.Join(missing, " and "),
	)
}
