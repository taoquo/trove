package importexport

import (
	"context"
	"fmt"
	"github.com/taoquo/trove/internal/bundle"
	"github.com/taoquo/trove/internal/domain"
)

// Import merges a bundle into the library.
//
// Merging is idempotent: every write is an upsert or an insert-if-absent, so
// importing the same bundle twice leaves the library exactly as one import did.
// Existing data is never removed, which is what distinguishes Import from Restore.
func (s *Service) Import(ctx context.Context, raw []byte) (ImportReport, error) {
	result, err := s.parse(raw)
	if err != nil {
		return ImportReport{}, err
	}
	return s.apply(ctx, result.Bundle, false, result.Warnings...)
}

// Restore replaces the library with a bundle.
//
// This is the legacy restore semantics: the bundle becomes the library. It is
// destructive, so the caller is expected to have confirmed it — and unlike Import
// it does not merge, because merging into data the user is trying to get rid of
// would defeat the point.
func (s *Service) Restore(ctx context.Context, raw []byte) (ImportReport, error) {
	result, err := s.parse(raw)
	if err != nil {
		return ImportReport{}, err
	}

	// Snapshot before deleting anything. The apply path is not transactional --
	// it writes table by table through the repositories -- so a failure part way
	// through used to leave the library half replaced, with the original already
	// gone. The snapshot is in memory and is only kept while this call runs; the
	// guarantee is "a failed restore puts the library back", not "a crash is
	// survivable", which is what the transactional rewrite would add.
	// The snapshot includes raw payloads and tombstones, which the user-facing default
	// leaves out: a rollback must put back everything the failed restore replaced, not
	// just the part a backup keeps.
	snapshot, snapshotErr := s.Export(ctx, SnapshotExportOptions())
	if snapshotErr != nil {
		// Without a snapshot there is no way back, so the restore does not start. This
		// used to wipe first and only discover the missing snapshot after the apply had
		// failed, which is the worst of both.
		return ImportReport{}, fmt.Errorf("importexport: the library could not be snapshotted, so it was not touched: %w", snapshotErr)
	}

	// Validation happens above, before anything is deleted: a rejected bundle must
	// leave the existing library untouched.
	if err := s.wipe(ctx); err != nil {
		return ImportReport{}, err
	}

	report, applyErr := s.apply(ctx, result.Bundle, true, result.Warnings...)
	if applyErr == nil {
		return report, nil
	}

	// Put the old library back. The rollback itself can fail (the disk is the
	// disk), and when it does the honest report is both failures rather than a
	// single error that hides which data is still there.
	parsedSnapshot, parseErr := s.parse(snapshot)
	if parseErr != nil {
		return report, fmt.Errorf("importexport: restore failed (%v) and its snapshot could not be read: %w", applyErr, parseErr)
	}
	if wipeErr := s.wipe(ctx); wipeErr != nil {
		return report, fmt.Errorf("importexport: restore failed (%v); rollback could not clear the partial data: %w", applyErr, wipeErr)
	}
	if _, rollbackErr := s.apply(ctx, parsedSnapshot.Bundle, true); rollbackErr != nil {
		return report, fmt.Errorf("importexport: restore failed (%v) and rollback failed: %w", applyErr, rollbackErr)
	}
	if verifyErr := s.verifyRestored(ctx, parsedSnapshot.Bundle); verifyErr != nil {
		return report, fmt.Errorf("importexport: restore failed (%v) and its rollback is incomplete: %w", applyErr, verifyErr)
	}
	return report, fmt.Errorf("importexport: restore failed; the previous library was put back: %w", applyErr)
}

// verifyRestored checks that a rollback actually put the snapshot back.
//
// The apply path writes table by table, so "the rollback returned no error" is not the
// same as "the library is what it was": a table that silently skipped its rows leaves no
// error behind. The check is a fresh export compared with the snapshot, which is the
// same bundle machinery that produced the snapshot -- a difference is a row that did not
// come back, not a formatting artefact.
func (s *Service) verifyRestored(ctx context.Context, snapshot *bundle.Bundle) error {
	fresh, err := s.Export(ctx, SnapshotExportOptions())
	if err != nil {
		return fmt.Errorf("the library could not be re-read: %w", err)
	}
	parsed, err := s.parse(fresh)
	if err != nil {
		return fmt.Errorf("the re-read library could not be parsed: %w", err)
	}
	before, after := snapshot.Manifest.Counts, parsed.Bundle.Manifest.Counts

	// One difference a rollback may legitimately introduce: apply gives every bookmark
	// without a list the Inbox, so a library that was missing that row comes back with
	// it. Losing memberships is still a failure; gaining this one is a repair.
	repaired := after
	if repaired.BookmarkLists < before.BookmarkLists {
		return fmt.Errorf("the library holds %d list memberships, the snapshot held %d",
			repaired.BookmarkLists, before.BookmarkLists)
	}
	repaired.BookmarkLists = before.BookmarkLists

	if before != repaired {
		return fmt.Errorf("the library holds %+v, the snapshot held %+v", after, before)
	}
	return nil
}

// clearSecrets is installed by the composition root, which holds both this service and
// the credential store. It is nil in tests that do not care, and the reset still runs.
//
// UseSecretClearer installs it.
//
// This action belongs beside "Import restore", behind a confirmation. It was missing here
// in a way nothing caught: the cascade existed, but only as the first half of a restore,
// and the checker that finds unreached methods walks internal/usecase -- this is
// internal/importexport, so it was outside that sweep.
func (s *Service) ResetLocalData(ctx context.Context) error {
	if err := s.require(); err != nil {
		return err
	}

	// The stored secrets go first, and a failure stops the reset before anything is
	// wiped. The confirmation promises that local settings are cleared, and it would be
	// false if the Keychain accounts survived: the API key lives under its own account,
	// so emptying the tables would never reach it. The secrets store has to be cleared as
	// well for the promise to hold.
	//
	// The clearer is injected, not reached for: the accounts belong to the credentials
	// package, and importing it here would tie this service to the Keychain for one
	// call. A nil clearer means there is nothing to clear, as in the tests that build
	// this service without one.
	if s.clearSecrets != nil {
		if err := s.clearSecrets(ctx); err != nil {
			return err
		}
	}

	// The confirmation names bookmarks, tags and settings, so all three go.
	if err := s.wipe(ctx); err != nil {
		return err
	}
	if _, err := s.repos.Tags.DeleteAll(ctx); err != nil {
		return err
	}
	if _, err := s.repos.Settings.DeleteAll(ctx); err != nil {
		return err
	}
	return nil
}

// wipe removes the library's data before a restore.
//
// The order matters: relations and memberships cascade from bookmarks, and a tag
// still referenced by a relation cannot be removed first.
func (s *Service) wipe(ctx context.Context) error {
	if _, err := s.repos.Bookmarks.DeleteAll(ctx); err != nil {
		return err
	}
	if _, err := s.repos.Suggestions.DeleteAll(ctx); err != nil {
		return err
	}
	if _, err := s.repos.Analysis.DeleteAll(ctx); err != nil {
		return err
	}

	tags, err := s.repos.Tags.List(ctx)
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(tags))
	for _, tag := range tags {
		ids = append(ids, tag.ID)
	}
	if _, err := s.repos.Tags.DeleteMany(ctx, ids); err != nil {
		return err
	}

	// Lists are removed except the Inbox, which is a system row the schema and the
	// UI both assume exists.
	lists, err := s.repos.Lists.List(ctx)
	if err != nil {
		return err
	}
	for _, list := range lists {
		if list.ID == domain.InboxListID {
			continue
		}
		if err := s.repos.Lists.Delete(ctx, list.ID); err != nil {
			return err
		}
	}

	// Saved views belong to the library, not to the app: the bundle is the new
	// library, and a view the bundle does not contain must not survive it. The
	// repository has no DeleteAll, and the count is small.
	if s.repos.SavedViews != nil {
		views, err := s.repos.SavedViews.List(ctx)
		if err != nil {
			return err
		}
		for _, view := range views {
			if err := s.repos.SavedViews.Delete(ctx, view.ID); err != nil {
				return err
			}
		}
	}
	return nil
}
