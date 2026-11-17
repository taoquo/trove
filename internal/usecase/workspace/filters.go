package workspace

import (
	"context"
	"errors"
	"fmt"
	"github.com/taoquo/trove/internal/domain"
	"strings"
)

// CreateTag creates a tag.
func (s *Service) CreateTag(ctx context.Context, name string) (domain.Tag, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	display := domain.NormalizeTagDisplayName(name)
	key := domain.CanonicalTagKey(display)
	if key == "" {
		return domain.Tag{}, errors.New("workspace: a tag needs a name")
	}

	// Reusing an existing tag is the right answer here: the user asked for a tag
	// by that name, and one already exists.
	if existing, found, err := s.repos.Tags.FindByName(ctx, display); err != nil {
		return domain.Tag{}, err
	} else if found {
		return existing, nil
	}

	now := s.clock.Now()
	tag := domain.Tag{
		ID:           s.newID("tag"),
		Name:         display,
		CanonicalKey: key,
		CreatedAt:    domain.FormatTimestamp(now),
		UpdatedAt:    domain.FormatTimestamp(now),
	}
	if err := s.repos.Tags.Create(ctx, tag); err != nil {
		return domain.Tag{}, err
	}
	_ = s.bumpGeneration()
	return tag, nil
}

// TagNameTakenError is a rename refused because another tag already answers to the name.
//
// It names the tag that got in the way, not the wanted name twice: renaming "Rustlang" to
// "Rust" would otherwise read as `"Rust" is already used by "Rust"`, which says nothing.
type TagNameTakenError struct {
	// Renaming is the tag the caller tried to rename.
	Renaming string
	// Wanted is the name that was asked for.
	Wanted string
	// TakenBy is the tag that already answers to it.
	TakenBy string
}

func (e *TagNameTakenError) Error() string {
	return fmt.Sprintf("workspace: cannot rename %q to %q: %q already uses that name",
		e.Renaming, e.Wanted, e.TakenBy)
}

// RenameTag renames a tag.
//
// A name another tag already answers to is refused rather than merged into it: merging
// would move a second tag's relations and delete that tag, which is not what a rename
// asks for. Merging has its own entry point.
func (s *Service) RenameTag(ctx context.Context, tagID, newName string) (domain.Tag, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tag, found, err := s.repos.Tags.Get(ctx, tagID)
	if err != nil {
		return domain.Tag{}, err
	}
	if !found {
		return domain.Tag{}, fmt.Errorf("workspace: no tag %q", tagID)
	}

	display := domain.NormalizeTagDisplayName(newName)
	key := domain.CanonicalTagKey(display)
	if key == "" {
		return domain.Tag{}, errors.New("workspace: a tag needs a name")
	}

	// A rename onto a name another tag already answers to is refused, not merged.
	//
	// This used to merge, on the reasoning that the relations then move instead of being
	// orphaned. Refusing avoids that too, and does it without moving anything: the user
	// asked to rename one tag, and merging would also have absorbed a second tag's
	// relations and deleted it. Merging is a separate action with its own entry point
	// (MergeTags), and the two are kept apart for the same reason.
	//
	// The lookup covers aliases as well as names, because FindByName does -- an alias
	// another tag owns is a collision for the same reason a name is.
	if other, exists, err := s.repos.Tags.FindByName(ctx, display); err != nil {
		return domain.Tag{}, err
	} else if exists && other.ID != tagID {
		return domain.Tag{}, &TagNameTakenError{Renaming: tag.Name, Wanted: display, TakenBy: other.Name}
	}

	tag.Name = display
	tag.CanonicalKey = key
	tag.UpdatedAt = domain.FormatTimestamp(s.clock.Now())
	if err := s.repos.Tags.Update(ctx, tag); err != nil {
		return domain.Tag{}, err
	}
	_ = s.bumpGeneration()
	return tag, nil
}

// DeleteTag removes a tag and rejects its pending suggestions.
//
// Rejecting them is not optional: a leftover pending suggestion would recreate
// the tag the user just deleted on the next accept.
func (s *Service) DeleteTag(ctx context.Context, tagID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tag, found, err := s.repos.Tags.Get(ctx, tagID)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}

	now := s.clock.Now()
	if _, err := s.repos.Suggestions.RejectByTagNames(ctx, []string{tag.Name}, now); err != nil {
		return err
	}
	if err := s.repos.Tags.Delete(ctx, tagID); err != nil {
		return err
	}
	_ = s.bumpGeneration()
	return nil
}

// CreateList creates a list.
func (s *Service) CreateList(ctx context.Context, name string) (domain.List, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	trimmed := strings.TrimSpace(name)
	normalized := domain.NormalizeListName(trimmed)
	if normalized == "" {
		return domain.List{}, errors.New("workspace: a list needs a name")
	}
	if normalized == domain.NormalizeListName(domain.InboxListName) {
		return domain.List{}, &domain.ListError{Reason: `"Inbox" is reserved for the system list`}
	}

	if existing, found, err := s.repos.Lists.FindByNormalizedName(ctx, normalized); err != nil {
		return domain.List{}, err
	} else if found {
		return existing, nil
	}

	now := s.clock.Now()
	list := domain.List{
		ID:         s.newID("list"),
		Name:       trimmed,
		Normalized: normalized,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.repos.Lists.Create(ctx, list); err != nil {
		return domain.List{}, err
	}
	_ = s.bumpGeneration()
	return list, nil
}

// RenameList renames a list.
func (s *Service) RenameList(ctx context.Context, listID, newName string) (domain.List, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	list, found, err := s.repos.Lists.Get(ctx, listID)
	if err != nil {
		return domain.List{}, err
	}
	if !found {
		return domain.List{}, fmt.Errorf("workspace: no list %q", listID)
	}
	if err := domain.ValidateListRename(list, newName); err != nil {
		return domain.List{}, err
	}

	trimmed := strings.TrimSpace(newName)
	normalized := domain.NormalizeListName(trimmed)
	if existing, exists, err := s.repos.Lists.FindByNormalizedName(ctx, normalized); err != nil {
		return domain.List{}, err
	} else if exists && existing.ID != listID {
		return domain.List{}, fmt.Errorf("workspace: a list named %q already exists", trimmed)
	}

	list.Name = trimmed
	list.Normalized = normalized
	list.UpdatedAt = s.clock.Now()
	if err := s.repos.Lists.Update(ctx, list); err != nil {
		return domain.List{}, err
	}
	_ = s.bumpGeneration()
	return list, nil
}

// DeleteList removes a list, first moving its bookmarks to a destination.
//
// The destination is required rather than defaulted inside the store: the user chooses,
// and the store refuses an empty list precisely so that choice cannot be skipped.
func (s *Service) DeleteList(ctx context.Context, listID, moveTo string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	list, found, err := s.repos.Lists.Get(ctx, listID)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if err := domain.ValidateListDelete(list); err != nil {
		return err
	}

	if strings.TrimSpace(moveTo) == "" {
		return errors.New("workspace: a destination list is required")
	}
	if moveTo == listID {
		return errors.New("workspace: the destination must be a different list")
	}

	// Move the bookmarks out first: the store refuses to delete a list that still
	// holds any, which is what makes "silently lost their home" impossible.
	now := s.clock.Now()
	ids, err := s.bookmarksInList(ctx, listID)
	if err != nil {
		return err
	}
	if len(ids) > 0 {
		if err := s.repos.Bookmarks.AssignToList(ctx, ids, moveTo, now); err != nil {
			return err
		}
	}

	if err := s.repos.Lists.Delete(ctx, listID); err != nil {
		return err
	}
	_ = s.bumpGeneration()
	return nil
}

// bookmarksInList lists the bookmarks in a list.
func (s *Service) bookmarksInList(ctx context.Context, listID string) ([]string, error) {
	page, err := s.repos.Bookmarks.Query(ctx, domain.LibraryQuery{
		Filter: domain.LibraryFilter{ListID: listID},
		Sort:   domain.SortSavedDesc,
		Limit:  domain.MaxLibraryPageSize,
	})
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(page))
	for _, bookmark := range page {
		ids = append(ids, bookmark.TweetID)
	}
	return ids, nil
}

// LoadGraphInput reads everything the tag relation graph needs.
//
// It is a separate read rather than a field on View because the graph needs the whole
// library, not a page of it, and a caller that only wants the list should not pay for
// that. It lives here rather than in the UI so the UI keeps depending on usecases
// rather than on repositories.
func (s *Service) LoadGraphInput(ctx context.Context) (domain.TagRelationInput, error) {
	input := domain.TagRelationInput{Tags: []domain.Tag{}, Bookmarks: []domain.Bookmark{}, BookmarkTags: []domain.BookmarkTag{}}

	if s.repos.Tags != nil {
		tags, err := s.repos.Tags.List(ctx)
		if err != nil {
			return input, fmt.Errorf("workspace: read tags for the graph: %w", err)
		}
		input.Tags = tags

		relations, err := s.repos.Tags.AllRelations(ctx)
		if err != nil {
			return input, fmt.Errorf("workspace: read tag relations for the graph: %w", err)
		}
		input.BookmarkTags = relations
	}

	if s.repos.Bookmarks != nil {
		bookmarks, err := s.repos.Bookmarks.All(ctx)
		if err != nil {
			return input, fmt.Errorf("workspace: read bookmarks for the graph: %w", err)
		}
		input.Bookmarks = bookmarks
	}

	return input, nil
}

// MergeReport is what a merge changed.
type MergeReport struct {
	// Target is the surviving tag, with its absorbed aliases.
	Target domain.Tag
	// MergedTagIDs are the tags that no longer exist.
	MergedTagIDs []string
	// RemappedCount counts the relations that moved. A relation that already existed
	// on the target is not counted: nothing moved, it was already there.
	RemappedCount int
	// ReparentedCount counts the children that followed the survivor.
	ReparentedCount int
}

// MergeTags folds several tags into one.
//
// This is the other half of the vocabulary review: the model proposes merges and the
// user applies them, and without this the proposals could never take effect.
//
// The order matters for safety. Relations are added to the target before anything is
// removed, so an interruption leaves the library with too many tags rather than with
// bookmarks that lost a tag. The sources are only deleted once their relations have
// been moved.
func (s *Service) MergeTags(ctx context.Context, sourceTagIDs []string, targetTagID string) (MergeReport, error) {
	report := MergeReport{MergedTagIDs: []string{}}

	if targetTagID == "" {
		return report, errors.New("workspace: merge needs a target tag")
	}

	target, found, err := s.repos.Tags.Get(ctx, targetTagID)
	if err != nil {
		return report, err
	}
	if !found {
		return report, errors.New("workspace: the target tag does not exist")
	}

	// The target is never its own source.
	sources := make([]string, 0, len(sourceTagIDs))
	seen := make(map[string]bool, len(sourceTagIDs))
	for _, id := range sourceTagIDs {
		if id == "" || id == targetTagID || seen[id] {
			continue
		}
		seen[id] = true
		sources = append(sources, id)
	}
	if len(sources) == 0 {
		return report, nil
	}

	allTags, err := s.repos.Tags.List(ctx)
	if err != nil {
		return report, err
	}
	tagByID := make(map[string]domain.Tag, len(allTags))
	for _, tag := range allTags {
		tagByID[tag.ID] = tag
	}

	relations, err := s.repos.Tags.AllRelations(ctx)
	if err != nil {
		return report, err
	}

	// A bookmark that already carries the target must not gain a second identical
	// relation, so the existing pair is recorded first.
	alreadyOnTarget := make(map[string]bool, len(relations))
	for _, relation := range relations {
		if relation.TagID == targetTagID {
			alreadyOnTarget[relation.BookmarkID] = true
		}
	}

	aliases := append([]string{}, target.Aliases...)
	aliasKeys := make(map[string]bool, len(aliases)+1)
	aliasKeys[domain.CanonicalTagKey(target.Name)] = true
	for _, alias := range aliases {
		aliasKeys[domain.CanonicalTagKey(alias)] = true
	}

	merged := make(map[string]bool, len(sources))
	now := s.clock.Now()

	for _, sourceID := range sources {
		source, known := tagByID[sourceID]
		if !known {
			continue
		}

		for _, relation := range relations {
			if relation.TagID != sourceID {
				continue
			}

			if !alreadyOnTarget[relation.BookmarkID] {
				if err := s.repos.Tags.Attach(ctx, domain.BookmarkTag{
					BookmarkID: relation.BookmarkID,
					TagID:      targetTagID,
					CreatedAt:  relation.CreatedAt,
					Source:     relation.Source,
				}); err != nil {
					return report, fmt.Errorf("workspace: move a relation during merge: %w", err)
				}
				alreadyOnTarget[relation.BookmarkID] = true
				report.RemappedCount++
			}

			if err := s.repos.Tags.Detach(ctx, relation.BookmarkID, sourceID); err != nil {
				return report, fmt.Errorf("workspace: drop a merged relation: %w", err)
			}
		}

		// The absorbed names become aliases, so the tag can still be found by the name
		// it used to have and a later suggestion of that name resolves here.
		for _, name := range append([]string{source.Name}, source.Aliases...) {
			key := domain.CanonicalTagKey(name)
			if key == "" || aliasKeys[key] {
				continue
			}
			aliasKeys[key] = true
			aliases = append(aliases, name)
		}

		if err := s.repos.Tags.Delete(ctx, sourceID); err != nil {
			return report, fmt.Errorf("workspace: delete a merged tag: %w", err)
		}
		merged[sourceID] = true
		report.MergedTagIDs = append(report.MergedTagIDs, sourceID)
	}

	// The survivor keeps the absorbed names.
	target.Aliases = aliases
	// Its own parent may be one of the tags just deleted, in which case it becomes a
	// root. Writing the stale parentId would fail the foreign key, and writing it
	// successfully would leave a tag pointing at nothing.
	survivorWasChild := target.ParentID != ""
	if survivorWasChild && merged[target.ParentID] {
		target.ParentID = ""
	}
	target.UpdatedAt = domain.FormatTimestamp(now)
	if err := s.repos.Tags.Update(ctx, target); err != nil {
		return report, fmt.Errorf("workspace: save the surviving tag: %w", err)
	}

	// Children of a merged tag follow the survivor, unless the survivor is itself a
	// child -- the taxonomy stays two levels deep, so in that case they become roots.
	if len(report.MergedTagIDs) > 0 {
		// The survivor's parent may itself be one of the merged tags, in which case the
		// survivor is about to become a root. Deciding from the stale parentId would
		// send its children to the root level even though the survivor is a root they
		// could have followed.
		survivorEndsUpRoot := target.ParentID == ""

		for _, child := range allTags {
			if child.ParentID == "" || !merged[child.ParentID] {
				continue
			}
			// A merged source was just deleted; updating it would resurrect it.
			if child.ID == targetTagID || merged[child.ID] {
				continue
			}

			child.ParentID = ""
			if survivorEndsUpRoot {
				child.ParentID = targetTagID
			}
			child.UpdatedAt = domain.FormatTimestamp(now)
			if err := s.repos.Tags.Update(ctx, child); err != nil {
				return report, fmt.Errorf("workspace: reparent a merged tag's child: %w", err)
			}
			report.ReparentedCount++
		}
	}

	if err := s.bumpGeneration(); err != nil {
		return report, err
	}

	updated, _, err := s.repos.Tags.Get(ctx, targetTagID)
	if err != nil {
		return report, err
	}
	report.Target = updated

	return report, nil
}

// UpdateTagAliases replaces a tag's aliases.
//
// Blank aliases and an alias that duplicates the tag's own name are dropped: either
// would make the tag findable under a name it already answers to, which is noise
// rather than information.
//
// An alias that another tag already claims is dropped too, and "claims" means its name
// as well as its own aliases. FindByName matches a canonical key or any alias, so an
// alias equal to another tag's name makes one lookup answer with two tags. Blocking only
// names is what let two tags advertise the same alias in the reference, which is why it
// collects both there.
//
// Nothing is reported when an alias is dropped: a paste-heavy edit should still save the
// part that is usable.
func (s *Service) UpdateTagAliases(ctx context.Context, tagID string, aliases []string) (domain.Tag, error) {
	var empty domain.Tag

	tag, found, err := s.repos.Tags.Get(ctx, tagID)
	if err != nil {
		return empty, err
	}
	if !found {
		return empty, errors.New("workspace: the tag does not exist")
	}

	// Every canonical identity another tag already answers to.
	seen := map[string]bool{domain.CanonicalTagKey(tag.Name): true}
	all, err := s.repos.Tags.List(ctx)
	if err != nil {
		return empty, err
	}
	for _, other := range all {
		if other.ID == tag.ID {
			continue
		}
		seen[domain.CanonicalTagKey(other.Name)] = true
		for _, alias := range other.Aliases {
			seen[domain.CanonicalTagKey(alias)] = true
		}
	}

	cleaned := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		trimmed := strings.TrimSpace(alias)
		key := domain.CanonicalTagKey(trimmed)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		cleaned = append(cleaned, trimmed)
	}

	tag.Aliases = cleaned
	tag.UpdatedAt = domain.FormatTimestamp(s.clock.Now())
	if err := s.repos.Tags.Update(ctx, tag); err != nil {
		return empty, err
	}
	return tag, nil
}

// LoadTagLibrary reads the tag management view.
//
// It is a separate read from Load because it needs every tag and every suggestion,
// not a page of bookmarks, and the workspace list does not carry suggestions at all.
func (s *Service) LoadTagLibrary(ctx context.Context) ([]domain.TagLibraryItem, error) {
	tags, err := s.repos.Tags.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("workspace: read tags for the library: %w", err)
	}
	relations, err := s.repos.Tags.AllRelations(ctx)
	if err != nil {
		return nil, fmt.Errorf("workspace: read tag relations for the library: %w", err)
	}

	var suggestions []domain.TagSuggestion
	if s.repos.Suggestions != nil {
		suggestions, err = s.repos.Suggestions.All(ctx)
		if err != nil {
			return nil, fmt.Errorf("workspace: read suggestions for the library: %w", err)
		}
	}

	return domain.BuildTagLibraryItems(tags, relations, suggestions), nil
}

// CleanupOrphanTags deletes the tags safe to sweep.
//
// It goes through the domain rule rather than filtering here, so the
// pending-suggestion guard cannot be forgotten at the call site: without it a sweep
// deletes a tag the user is still reviewing, rejects those suggestions, and blocks the
// name from ever being suggested again.
func (s *Service) CleanupOrphanTags(ctx context.Context) (int, error) {
	items, err := s.LoadTagLibrary(ctx)
	if err != nil {
		return 0, err
	}

	ids := domain.GetRemovableOrphanTagIDs(items)
	if len(ids) == 0 {
		return 0, nil
	}

	removed, err := s.repos.Tags.DeleteMany(ctx, ids)
	if err != nil {
		return 0, fmt.Errorf("workspace: clean up empty tags: %w", err)
	}
	if err := s.bumpGeneration(); err != nil {
		return removed, err
	}
	return removed, nil
}
