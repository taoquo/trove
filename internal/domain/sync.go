package domain

import "time"

// SyncStatus is the state of a sync run.
type SyncStatus = string

const (
	SyncIdle           SyncStatus = "idle"
	SyncRunning        SyncStatus = "running"
	SyncSuccess        SyncStatus = "success"
	SyncPartialSuccess SyncStatus = "partial_success"
	// SyncCancelled is the terminal state a cancelled run records. A cancel needs
	// a terminal state that is not an error.
	SyncCancelled SyncStatus = "cancelled"
	SyncError     SyncStatus = "error"
)

// ValidSyncStatus reports whether s is a known run state.
func ValidSyncStatus(s string) bool {
	switch s {
	case SyncIdle, SyncRunning, SyncSuccess, SyncPartialSuccess, SyncCancelled, SyncError:
		return true
	}
	return false
}

// SyncErrorKind classifies why a sync failed.
type SyncErrorKind = string

// Legacy error kinds. These appear in existing sync_runs rows and in backups, so
// they must stay readable.
const (
	ErrAuthExpired    SyncErrorKind = "auth_expired"
	ErrRateLimited    SyncErrorKind = "rate_limited"
	ErrNetworkError   SyncErrorKind = "network_error"
	ErrXSchemaChanged SyncErrorKind = "x_schema_changed"
	// ErrInterrupted means the run died without reporting. It has a real cause
	// (the worker being recycled) and a distinct remedy (just sync again).
	ErrInterrupted SyncErrorKind = "interrupted"
	ErrUnknown     SyncErrorKind = "unknown"
)

// Kinds that distinguish causes the legacy taxonomy lumped together.
const (
	ErrUnauthorized          SyncErrorKind = "unauthorized"
	ErrCSRFInvalid           SyncErrorKind = "csrf_invalid"
	ErrQueryIDStale          SyncErrorKind = "query_id_stale"
	ErrNetwork               SyncErrorKind = "network"
	ErrResponseSchemaChanged SyncErrorKind = "response_schema_changed"
	// ErrSyncInProgress is a run that was refused because another process holds
	// the library. It is not a fault: nothing was attempted, and the remedy is to
	// let the other run finish. It needs its own kind so the status line does not
	// say "sync failed" for a second window or an overlapping scheduled command.
	ErrSyncInProgress SyncErrorKind = "sync_in_progress"
	// ErrCredentialStore means the X session could not be read from the system
	// store. It is not an expired session: the cookies may be perfectly good, and
	// the fix is to unlock the store, not to sign in again.
	ErrCredentialStore SyncErrorKind = "credential_store"
)

// legacySyncErrorKinds and currentSyncErrorKinds are the two sets, kept separate
// so the mapping between them stays explicit and testable.
var (
	legacySyncErrorKinds = []SyncErrorKind{
		ErrAuthExpired, ErrRateLimited, ErrNetworkError,
		ErrXSchemaChanged, ErrInterrupted, ErrUnknown,
	}
	currentSyncErrorKinds = []SyncErrorKind{
		ErrUnauthorized, ErrCSRFInvalid, ErrQueryIDStale, ErrRateLimited,
		ErrNetwork, ErrResponseSchemaChanged, ErrSyncInProgress,
		ErrCredentialStore, ErrInterrupted, ErrUnknown,
	}
)

// AllSyncErrorKinds is the union that validation must accept. It deliberately
// accepts both vocabularies: narrowing it would make existing rows and old
// backups fail validation, and an unknown kind is rejected outright rather than
// defaulted.
var AllSyncErrorKinds = []SyncErrorKind{
	// The vocabulary already in users' databases.
	ErrAuthExpired, ErrRateLimited, ErrNetworkError,
	ErrXSchemaChanged, ErrInterrupted, ErrUnknown,
	// The finer-grained additions.
	ErrUnauthorized, ErrCSRFInvalid, ErrQueryIDStale,
	ErrNetwork, ErrResponseSchemaChanged, ErrSyncInProgress, ErrCredentialStore,
}

// ValidSyncErrorKind reports whether k is a known kind, from either vocabulary.
func ValidSyncErrorKind(k string) bool {
	if k == "" {
		// Absent is legal: a successful run has no error kind.
		return true
	}
	for _, candidate := range AllSyncErrorKinds {
		if k == candidate {
			return true
		}
	}
	return false
}

// SyncRun is one recorded attempt to sync bookmarks.
type SyncRun struct {
	// ID is a random identifier, so two runs starting in the same millisecond do
	// not collide.
	ID        string
	Status    SyncStatus
	StartedAt time.Time
	// FinishedAt is nil while the run is in flight.
	FinishedAt    *time.Time
	FetchedCount  int
	InsertedCount int
	UpdatedCount  int
	FailedCount   int
	// PagesCompleted is new, and lets the UI explain a partial run.
	PagesCompleted int
	// ErrorKind is empty for a run that did not fail.
	ErrorKind    SyncErrorKind
	ErrorSummary string
	// StrategyVersion identifies the pagination/early-stop policy that produced
	// this run, so a changed policy is distinguishable in history.
	StrategyVersion int
}

// SyncSummary is the headline result of the most recent sync.
//
// It is kept separately from the run history because a user may prune history
// but must never lose "when did I last sync, and did it work".
type SyncSummary struct {
	Status        SyncStatus
	FetchedCount  int
	InsertedCount int
	UpdatedCount  int
	FailedCount   int
	LastSyncedAt  *time.Time
	ErrorKind     SyncErrorKind
	ErrorSummary  string
}

// SyncStrategyVersion identifies the current pagination and early-stop policy.
const SyncStrategyVersion = 1

// Default page and limit constants.
const (
	// DefaultPageSize is both the request size and a hard ceiling: the X endpoint
	// clamps count to 20.
	DefaultPageSize = 20
	// DefaultSyncLimit caps a first full sync so it cannot run unbounded.
	DefaultSyncLimit = 10000
	// DefaultIncrementalStopBufferPages is how many consecutive fully-known pages
	// end an incremental sync early.
	DefaultIncrementalStopBufferPages = 3
)

// TerminationReason explains why pagination stopped.
//
// Without it, a run that hit the cap cannot be told from one that reached the
// end, which is exactly why a truncated first sync could be recorded as complete.
type TerminationReason = string

const (
	// TermEndOfPages means X reported no further cursor: the only reason a first
	// full sync may be recorded as complete.
	TermEndOfPages TerminationReason = "end-of-pages"
	// TermHitLimit means the configured cap stopped the run early.
	TermHitLimit TerminationReason = "hit-limit"
	// TermStoppedEarly means the incremental early-stop rule fired.
	TermStoppedEarly TerminationReason = "stopped-early"
	// TermCancelled means the user cancelled.
	TermCancelled TerminationReason = "cancelled"
	// TermFailed means a page failed and the run stopped.
	TermFailed TerminationReason = "failed"
)

// CompletesFullSync reports whether this termination reason may mark a first
// full sync as finished.
func CompletesFullSync(reason TerminationReason, failedCount, unrecognized int) bool {
	return reason == TermEndOfPages && failedCount == 0 && unrecognized == 0
}
