package ui

import (
	"context"
	"strings"

	"github.com/egoist/mygo/ui"
	engine "github.com/taoquo/trove/apps/macos/internal/autotag"
	"github.com/taoquo/trove/apps/macos/internal/i18n"
)

// ReviewSelectionCount is how many proposals Apply would run.
func (s *Shell) ReviewSelectionCount() int {
	count := 0
	for _, merge := range s.state.ReviewMerges {
		if s.state.ReviewSelected[engine.MergePlanKey(merge)] {
			count++
		}
	}
	for _, rename := range s.state.ReviewRenames {
		if s.state.ReviewSelected[engine.RenamePlanKey(rename)] {
			count++
		}
	}
	return count
}

// vocabularyReviewPanel draws the review.
//
// The source's review is an inline panel under the tag page's toolbar, so its error, its
// list and its Apply all stay in one place while it runs (TagResultsPane.tsx:820-905). This
// one is a modal, and the same three things live inside it: the error above the list rather
// than instead of it, a busy Apply rather than a panel that vanishes on the press, and a
// retry when the request itself failed (round 29).
func (s *Shell) vocabularyReviewPanel(c *ui.Context, theme *ui.Theme, entrance float32) {
	rail := Rail(s.state.Appearance, s.state.SystemDark, s.translator.Locale() == i18n.ZHCN)
	empty := len(s.state.ReviewMerges) == 0 && len(s.state.ReviewRenames) == 0

	// The panel fades in and scales up from the stylesheet's 0.96, at its own duration.
	ui.Column(c).Width(620).GapY(SpaceLoose).Padding(PaddingPanel).Radius(RadiusDialog).
		Opacity(EntranceDetailCard.OpacityAt(entrance)).
		Scale(EntranceDetailCard.ScaleAt(entrance), EntranceDetailCard.ScaleAt(entrance)).Children(func() {
		ui.Text(c, s.t().T(i18n.ReviewTitle)).
			Font(s.displayFace()).FontSize(FontModalTitle).FontWeight(600)
		ui.Text(c, s.t().T(i18n.ReviewDescription)).FontSize(FontSmall).TextColor(theme.TextMuted).MaxLines(3)

		if s.state.ReviewRunning {
			ui.Text(c, s.t().T(i18n.ReviewRunning)).FontSize(FontBody).TextColor(theme.TextMuted)
			return
		}

		if s.state.ReviewError != "" {
			ui.Text(c, s.t().T(i18n.ReviewFailed)+": "+s.state.ReviewError).
				FontSize(FontBody).TextColor(theme.Danger).MaxLines(4)
			// A request that never produced proposals has nothing else to show, so the one
			// useful control is the one that asks again. The source gets this for free: its
			// panel is inline, so the toolbar's "Tidy vocabulary" stays visible above it.
			// This panel covers the page, so the retry has to be inside it.
			if empty {
				ui.Row(c).GapX(SpaceBase).Children(func() {
					textAction(c, theme, rail, textActionSpec{
						label: s.t().T(i18n.ReviewRetry), primary: true,
						onClick: func() { s.OpenVocabularyReview() },
					})
					settingsAction(c, theme, rail, s.t().T(i18n.ReviewDismiss), false, false, func() {
						s.state.ReviewOpen = false
					})
				})
				return
			}
			// A partly failed apply keeps its plan below: what did not land is exactly what
			// the reader has to see and can press again.
		}

		if empty {
			// Two different empty answers. "The list looks healthy" is about a list that
			// exists; telling that to a reader whose list is empty reads as nonsense --
			// there was nothing to compare, so no proposal could have been made.
			message := i18n.ReviewEmpty
			if len(s.state.TagItems) == 0 {
				message = i18n.ReviewNoTags
			}
			ui.Text(c, s.t().T(message)).FontSize(FontBody).TextColor(theme.TextMuted).MaxLines(3)
			settingsAction(c, theme, rail, s.t().T(i18n.ReviewDismiss), false, false, func() {
				s.state.ReviewOpen = false
			})
			return
		}

		if s.state.ReviewApplying {
			ui.Text(c, s.t().T(i18n.ReviewApplying)).FontSize(FontSmall).TextColor(theme.TextMuted)
		}

		ui.Scroll(c).FillWidth().MaxHeight(380).Children(func() {
			ui.Column(c).FillWidth().GapY(SpaceSmall).Children(func() {
				for _, merge := range s.state.ReviewMerges {
					merge := merge
					key := engine.MergePlanKey(merge)
					checked := s.state.ReviewSelected[key]
					label := s.t().T(i18n.ReviewMerge) + ": " +
						strings.Join(merge.SourceNames, ", ") + " → " + merge.TargetName
					ui.Row(c).FillWidth().GapX(SpaceBase).Children(func() {
						ui.Checkbox(c, &checked, label).OnChange(func() {
							s.state.ReviewSelected[key] = checked
						})
					})
				}
				for _, rename := range s.state.ReviewRenames {
					rename := rename
					key := engine.RenamePlanKey(rename)
					checked := s.state.ReviewSelected[key]
					label := s.t().T(i18n.ReviewRename) + ": " + rename.From + " → " + rename.To
					ui.Row(c).FillWidth().GapX(SpaceBase).Children(func() {
						ui.Checkbox(c, &checked, label).OnChange(func() {
							s.state.ReviewSelected[key] = checked
						})
					})
				}
			})
		})

		ui.Row(c).FillWidth().GapX(SpaceBase).Children(func() {
			// The source disables Apply while nothing is ticked and while the write runs
			// (TagResultsPane.tsx:891-899); a press that would close the panel and apply
			// nothing reads as a successful no-op.
			textAction(c, theme, rail, textActionSpec{
				label:    s.t().T(i18n.ReviewApply),
				primary:  true,
				disabled: s.state.ReviewApplying || s.ReviewSelectionCount() == 0,
				busy:     s.state.ReviewApplying,
				onClick:  func() { s.ApplyVocabularyReview(context.Background()) },
			})
			settingsAction(c, theme, rail, s.t().T(i18n.ReviewDismiss), false, false, func() {
				s.state.ReviewOpen = false
			})
		})
	})
}
