package domain

// Tag is a user-visible label in the library.
type Tag struct {
	ID   string
	Name string
	// CanonicalKey is CanonicalTagKey(Name). It is derived, never assigned from
	// input: it carries the depluralization rules described in tagkey.go, so it
	// is what enforces "these two names are the same tag". It is stored (and
	// uniquely indexed) rather than recomputed on read because SQLite has no
	// equivalent function.
	CanonicalKey string
	// Aliases records names this tag absorbed through a merge, so a later
	// suggestion of an absorbed name resolves here instead of minting a
	// near-duplicate.
	Aliases []string
	// ParentID is the owning tag for the two-level taxonomy. Empty means no
	// parent; storage writes SQL NULL for it, because an empty string would
	// violate the self-referencing foreign key.
	//
	// No screen sets or shows a theme any more: it was never used, so the entry
	// points went away. The field and its storage stay because backups and
	// imports still carry parents, and a merge repairs them rather than dropping
	// the relation; the schema's triggers still bound the depth at two.
	ParentID  string
	CreatedAt string
	UpdatedAt string
}
