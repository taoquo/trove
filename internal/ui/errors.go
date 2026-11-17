package ui

import (
	"fmt"
	"github.com/taoquo/trove/internal/domain"
	"time"

	"github.com/taoquo/trove/internal/autotag"
	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/llm"
)

// The interface's vocabulary for a provider failure.
//
// The transport already classifies every failure into one of thirteen kinds, and those
// kinds existed before anything showed them: the panel printed the raw message, so a
// user saw "401" rather than "the API key is invalid or expired", and a dropped
// parameter (which the request survived) looked the same as a failure (which it did
// not). This is the missing translation layer.

// errorKindKeys maps a classified failure onto the sentence a user reads.
var errorKindKeys = map[llm.ErrorKind]i18n.Key{
	llm.KindUnauthorized:     i18n.LLMErrorUnauthorized,
	llm.KindForbidden:        i18n.LLMErrorForbidden,
	llm.KindRateLimit:        i18n.LLMErrorRateLimit,
	llm.KindQuota:            i18n.LLMErrorQuota,
	llm.KindBadRequest:       i18n.LLMErrorBadRequest,
	llm.KindUnsupportedParam: i18n.LLMErrorUnsupportedParam,
	llm.KindContextLength:    i18n.LLMErrorContextLength,
	llm.KindTimeout:          i18n.LLMErrorTimeout,
	llm.KindNetwork:          i18n.LLMErrorNetwork,
	llm.KindServer:           i18n.LLMErrorServer,
	llm.KindContentFilter:    i18n.LLMErrorContentFilter,
	llm.KindInvalidJSON:      i18n.LLMErrorInvalidJson,
	llm.KindUnknown:          i18n.LLMErrorUnknown,
}

// localizeProviderError renders a failure in the user's language.
//
// It returns the classified sentence and, when the transport reported a retry delay,
// the second sentence naming it. Those are separate because they answer different
// questions: what went wrong, and what to do about it.
func (s *Shell) localizeProviderError(kind string, retryAfterSeconds int) (string, string) {
	key, known := errorKindKeys[llm.ErrorKind(kind)]
	if !known {
		// An unclassified failure keeps its raw message: a wrong guess about what it
		// means is worse than the provider's own words.
		return "", ""
	}

	message := s.t().T(key)
	if retryAfterSeconds > 0 {
		return message, s.t().Tf(i18n.LLMErrorRateLimitLong, retryAfterSeconds)
	}
	return message, ""
}

// SummarizeRun renders a finished auto-tagging run in the user's language.
//
// The engine distinguishes thirteen outcomes and reports the counts behind each. The
// panel used to show one summary line, so a run that stopped because its budget ran out
// looked the same as one that stopped because the key was missing -- and those need
// different fixes. The second return value is the detail line, which carries the counts
// worth showing beside the headline.
func (s *Shell) SummarizeRun(outcome autotag.RunOutcome, summary autotag.RunSummary) (string, string) {
	// Each key takes its arguments in the order the sentence names them, and the two
	// locales agree on that order, which the generator checks.
	number := func(value int) any { return value }

	var headline string
	switch outcome {
	case autotag.OutcomeOK:
		headline = s.t().Tf(i18n.OutcomeOk, number(summary.SuggestedCount), number(summary.PendingCount))
	case autotag.OutcomeNoCandidates:
		headline = s.t().T(i18n.OutcomeNoCandidates)
	case autotag.OutcomeEmptyVocabulary:
		headline = s.t().T(i18n.OutcomeEmptyVocabulary)
	case autotag.OutcomeLLMDisabled:
		headline = s.t().T(i18n.OutcomeLlmDisabled)
	case autotag.OutcomeMissingAPIKey:
		headline = s.t().T(i18n.OutcomeMissingKey)
	case autotag.OutcomeAllBlocked:
		headline = s.t().T(i18n.OutcomeAllBlocked)
	case autotag.OutcomeBudgetExhausted:
		headline = s.t().T(i18n.OutcomeBudgetExhausted)
	case autotag.OutcomeTimedOut:
		headline = s.t().Tf(i18n.OutcomeTimedOut, number(summary.SuggestedCount), number(summary.UnprocessedCount))
	case autotag.OutcomeInterrupted:
		headline = s.t().Tf(i18n.OutcomeInterrupted, number(summary.UnprocessedCount))
	case autotag.OutcomeCancelled:
		headline = s.t().Tf(i18n.OutcomeCancelled, number(summary.SuggestedCount), number(summary.UnprocessedCount))
	case autotag.OutcomeErrorBudget:
		headline = s.t().Tf(i18n.OutcomeErrorBudget, number(summary.SuggestedCount))
	case autotag.OutcomeFailed:
		headline = s.t().T(i18n.OutcomeFailed)
		if summary.Error != "" {
			headline += ": " + summary.Error
		}
	case autotag.OutcomeDisabled:
		// The run button is drawn whether or not the feature's switch is on, so this
		// outcome has a reader: someone who pressed it. Reporting nothing left the
		// button looking broken; the sentence names the switch that stops the run.
		return s.t().T(i18n.OutcomeDisabled), ""
	default:
		return "", ""
	}

	detail := s.t().Tf(i18n.AutoTagDetail,
		number(summary.CandidateCount), number(summary.LLMProcessedCount),
		number(summary.LLMBatchCount), number(summary.LLMFailedBatchCount),
		number(summary.VocabularySize), number(summary.NewTagBudget))

	// Only mention what actually happened, so the line stays readable.
	if summary.BlockedDroppedCount > 0 {
		detail += " " + s.t().Tf(i18n.AutoTagBlockedDropped, number(summary.BlockedDroppedCount))
	}
	if summary.BudgetDroppedCount > 0 {
		detail += " " + s.t().Tf(i18n.AutoTagBudgetDropped, number(summary.BudgetDroppedCount))
	}
	if summary.BlockedTagKeyCount > 0 {
		detail += " " + s.t().Tf(i18n.AutoTagBlocked, number(summary.BlockedTagKeyCount))
	}
	// Only when the provider said so. "0 tokens" for an endpoint that reports
	// nothing would read as a free run rather than an unknown one.
	if summary.InputTokens > 0 || summary.OutputTokens > 0 {
		detail += " " + s.t().Tf(i18n.AutoTagTokens, number(summary.InputTokens), number(summary.OutputTokens))
	}

	return headline, detail
}

// LiveRunProgress renders a run that is happening now.
//
// The engine has reported a phase, a heartbeat and a progress count since it was
// written, and a stale run is detected by comparing the heartbeat against a liveness
// window. None of that reached the interface: the panel showed a summary only after the
// run finished, so a run that had silently died looked exactly like one still working.
//
// It returns the lines to draw in order, so the caller does not have to decide what is
// worth saying.
func (s *Shell) LiveRunProgress(state *autotag.RunState, now time.Time) []string {
	progress := autotag.BuildRunProgress(state, now)
	if !progress.Active {
		return nil
	}

	lines := make([]string, 0, 5)

	phase := s.t().T(i18n.RunPhaseHeuristics)
	if progress.Phase == autotag.PhaseLLM {
		phase = s.t().T(i18n.RunPhaseLlm)
	}
	headline := s.t().T(i18n.RunAnalyzing) + " · " + phase
	if progress.Total > 0 {
		headline += fmt.Sprintf(" %d%%", progress.Percent)
	}
	lines = append(lines, headline)

	// A total of zero means the snapshot has not been measured yet, and "0/0" would
	// read as finished.
	if progress.Total > 0 {
		lines = append(lines, fmt.Sprintf("%d / %d", progress.Processed, progress.Total))
	}

	if seconds := int(progress.Elapsed.Seconds()); seconds > 0 {
		lines = append(lines, s.t().Tf(i18n.RunElapsed, seconds))
	}

	switch {
	case progress.HasWaitingLeft:
		// Waiting for a provider's back-off is not a stall, and saying so would train
		// the user to ignore the message.
		lines = append(lines, s.t().T(i18n.RunWaiting))
	case progress.Stalled:
		lines = append(lines, s.t().Tf(i18n.RunStalled, progress.SecondsSinceHeartbeat))
	case progress.HasETA:
		// The estimate is only offered once something has finished, so it cannot be
		// shown as a guess made from nothing.
		lines = append(lines, s.t().Tf(i18n.RunRemaining, int(progress.ETA.Seconds())))
	}

	if state.CancelRequested {
		lines = append(lines, s.t().T(i18n.RunStopping))
	}

	return lines
}

// syncErrorKindKeys maps a stored sync failure kind to the sentence that explains it.
//
// The sidebar's chip says "Error"; this is what explains it where the reader needs to know
// what happened.
//
// A known kind gets a localized sentence, an unknown one keeps whatever summary came with it,
// and a run that carried neither shows nothing at all. This is one field and one lookup, the
// same shape as localizeProviderError above.
//
// The two vocabularies in internal/domain/sync.go both appear, because both are in users'
// databases: the finer-grained kinds fall back to the sentence for the older kind they refine.
var syncErrorKindKeys = map[domain.SyncErrorKind]i18n.Key{
	domain.ErrAuthExpired:  i18n.SyncErrorAuthExpired,
	domain.ErrUnauthorized: i18n.SyncErrorAuthExpired,
	domain.ErrRateLimited:  i18n.SyncErrorRateLimited,
	domain.ErrNetworkError: i18n.SyncErrorNetworkError,
	domain.ErrNetwork:      i18n.SyncErrorNetworkError,
	// A rejected CSRF token is a stale ct0, not a network fault: X answers 403 for
	// both, and 401 and 403 both mean auth_expired. Mapping it to the network sentence
	// would send the reader to check their connection for a cookie rotation.
	domain.ErrCSRFInvalid:           i18n.SyncErrorAuthExpired,
	domain.ErrQueryIDStale:          i18n.SyncErrorSchemaChanged,
	domain.ErrXSchemaChanged:        i18n.SyncErrorSchemaChanged,
	domain.ErrResponseSchemaChanged: i18n.SyncErrorSchemaChanged,
	domain.ErrInterrupted:           i18n.SyncErrorInterrupted,
	domain.ErrUnknown:               i18n.SyncErrorUnknown,
	domain.ErrSyncInProgress:        i18n.SyncErrorInProgress,
	domain.ErrCredentialStore:       i18n.SyncErrorCredentialStore,
}

// syncFailureMessage is what the status line shows for a failed run, or "" when the kind is not
// one this build knows -- an unclassified failure keeps its raw message rather than a guess.
func (s *Shell) syncFailureMessage(kind string) string {
	key, known := syncErrorKindKeys[domain.SyncErrorKind(kind)]
	if !known {
		return ""
	}
	return s.t().T(key)
}

// syncNeedsCredentials reports whether the Keychain is known to be missing the two X cookies.
//
// A test shell has no credential store, and "no store" must not read as "no credentials": that
// would put an instruction to paste cookies on a surface whose sync is a test double.
func (s *Shell) syncNeedsCredentials() bool {
	return s.credentials != nil && !s.state.XCredentialsStored
}

// syncCredentialFailure reports whether the failure is the missing-cookie one.
//
// It is what decides the sentence: the empty Keychain alone is not a failure (a fresh install has
// not tried yet), and a failure of another kind is not fixed by pasting cookies.
func (s *Shell) syncCredentialFailure() bool {
	if !s.syncNeedsCredentials() {
		return false
	}
	return s.isCredentialKind(s.state.LastSyncErrorKind)
}

// isCredentialKind reports whether a failure kind is fixed by new cookies.
//
// csrf_invalid belongs here: it is what a rotated ct0 looks like, and the fix is
// exactly the same -- take the browser's current pair.
func (s *Shell) isCredentialKind(kind string) bool {
	switch domain.SyncErrorKind(kind) {
	case domain.ErrUnauthorized, domain.ErrAuthExpired, domain.ErrCSRFInvalid:
		return true
	default:
		return false
	}
}

// syncNeedsNewCookies reports whether the failure is one that new cookies would fix, whatever the
// Keychain currently holds.
//
// It decides the next-step control under the sentence. The sentence itself is left alone: it is
// accurate when the session really has died, and softening it to suit a desktop app would be the
// same mistake in the other direction.
//
// What it lacks is a destination. This app has no sign-in screen to re-log-in at -- the two
// cookies are typed in, or read from a browser, under Settings > Sync -- so "sign in again" names
// an action that does not exist here. A stored unauthorized summary can leave the sidebar saying
// exactly that with no way to act on it. The control is what turns the instruction into something
// the user can do.
func (s *Shell) syncNeedsNewCookies() bool {
	return s.isCredentialKind(s.state.LastSyncErrorKind) && s.state.LastSyncStatus == domain.SyncError
}

// syncProblemLine is the sentence under the sync button, or "" when there is nothing to say.
//
// The line is drawn once, inside the sidebar's sync block and below the button, and it is
// `kind ? localized(kind) : summary`. The sidebar explains every failure: with an empty
// Keychain the stored kind is "unauthorized", whose sentence is "X login expired, sign in
// again" -- a fix that cannot work, because the app has no login to expire. The cookies are
// typed into Settings > Sync, and when the Keychain is empty that is what the line says.
func (s *Shell) syncProblemLine() string {
	if s.state.SyncRunning {
		return ""
	}

	kind := s.state.LastSyncErrorKind
	if s.syncCredentialFailure() {
		return s.t().T(i18n.SyncNeedsXCredentials)
	}

	// A partial run has something to say too: the pages that were lost are why the
	// library is incomplete, and the kind names what to do about it. The line is drawn
	// from the summary's kind whether the status is error or partial_success.
	if s.state.LastSyncStatus != domain.SyncError &&
		s.state.LastSyncStatus != domain.SyncPartialSuccess {
		return ""
	}
	if s.state.LastSyncStatus == domain.SyncPartialSuccess && kind == "" {
		// Nothing failed, so there is nothing to explain: a run stopped by the cap is
		// partial without a cause.
		return ""
	}
	if message := s.syncFailureMessage(kind); message != "" {
		return message
	}
	return s.state.LastSyncErrorSummary
}
