package handlers

import "strings"

// What a workload may write while it also reads regulated data.
//
// The access control around regulated data was real and still is: mounting it
// needs the AttachRegulatedData capability, the mount honours Owner/Write/Read
// (ADR-051), and a bulk download of an HDS dataset is refused. None of that
// closes the shortest path out, and the path was open in production: a pod
// mounting `hds-for` read-only and `dataset-local-stef` writable needs one
// `cp` to move patient data into a dataset with no classification, and that
// dataset is downloadable through the platform by its owner, legitimately.
//
// It is how 17 GB of PREMYOM1000 came to sit in a personal dataset on EMSE.
// That copy was found by chance, not by a journal, because an fuse write
// leaves no trace in the API's audit trail - the only place writes are
// recorded.
//
// Stephane approved the rule on 2026-11-17, "sur tous les workloads": if a
// regulated dataset is mounted, no other dataset is mounted writable, and the
// refusal says so.
//
// The rule is applied where the mount list is built rather than where each
// workload is launched, so a workspace, a job, an app and a scheduled job are
// covered by construction - a new call site cannot forget it.
//
// Writing from one regulated dataset into another stays allowed, and that is
// the one deliberate narrowing of the sentence above. A regulated destination
// is inside the same perimeter: it carries the same classification, so every
// project that mounts it is closed in turn and its bulk download is refused
// the same way. The rule is therefore closed under itself, which is what makes
// it a confinement rule rather than a nuisance - a researcher writing results
// next to the cohort they came from is the normal case, and breaking it would
// push people towards the unclassified dataset this exists to protect.
const readOnlyReasonRegulatedCoMount = "regulated-co-mount"

// applyRegulatedCoMountRule demotes to read-only every mount that is not
// itself regulated, when at least one regulated dataset is in the list.
//
// exempt lifts it, for a role holding platformrole.WriteBesideRegulatedData.
// It is passed in rather than read here because this function is the rule and
// has to stay answerable from a test with no identity, no store and no
// platform - and because the decision belongs to the one place that already
// knows who is launching.
//
// An exempt workload is not an unwatched one: the nightly fingerprint
// crossing (regulated_copy_scan.go) looks for regulated objects reappearing
// in the unclassified datasets of the same project, and it is what makes
// handing this box out defensible. Prevention for everybody, detection for
// those who were given the room to work.
func applyRegulatedCoMountRule(items []workspaceAttachedDataset, exempt bool) []workspaceAttachedDataset {
	if exempt || !anyRegulatedDataset(items) {
		return items
	}
	for index := range items {
		if items[index].isRegulated() || items[index].ReadOnly {
			continue
		}
		items[index].ReadOnly = true
		items[index].ReadOnlyReason = readOnlyReasonRegulatedCoMount
	}
	return items
}

// readOnlyForCoMount names what the rule took, for the launch message and the
// audit entry.
//
// Named rather than counted: "one dataset is read-only" sends somebody looking
// for which, and the person who notices is usually the one who was about to
// write there.
func readOnlyForCoMount(items []workspaceAttachedDataset) []string {
	names := []string{}
	for _, item := range items {
		if item.ReadOnlyReason == readOnlyReasonRegulatedCoMount {
			names = append(names, strings.TrimSpace(item.Name))
		}
	}
	return names
}
