package ui

import "testing"

// TestConfirmationFailsClosed is the safety property the gate calls out: a
// destructive action proceeds only for an explicit confirming click. Every other
// outcome denies, so a missing or mis-wired answer can never delete user data.
func TestConfirmationFailsClosed(t *testing.T) {
	t.Run("the zero value allows nothing", func(t *testing.T) {
		var confirmation Confirmation

		if confirmation.Allowed() {
			t.Error("an unasked confirmation must deny")
		}
		if confirmation.Asked() {
			t.Error("an unasked confirmation must not be on screen")
		}
		if confirmation.Denied() {
			t.Error("an unasked confirmation is not a denial yet")
		}
	})

	t.Run("asking does not by itself authorise", func(t *testing.T) {
		var confirmation Confirmation
		confirmation.Ask("Delete?", "This removes 3 bookmarks.", "Delete")

		if !confirmation.Asked() {
			t.Error("the dialog should be on screen")
		}
		if confirmation.Allowed() {
			t.Error("showing a dialog must not authorise the action")
		}
		if confirmation.Denied() {
			t.Error("an unanswered dialog is not a denial yet")
		}
	})

	t.Run("dismissal denies", func(t *testing.T) {
		var confirmation Confirmation
		confirmation.Ask("Delete?", "body", "Delete")
		confirmation.Answer(DismissedValue)

		if confirmation.Allowed() {
			t.Error("dismissing the dialog must deny: this is the fail-closed case")
		}
		if !confirmation.Denied() {
			t.Error("a dismissal is a denial")
		}
	})

	t.Run("cancel denies", func(t *testing.T) {
		var confirmation Confirmation
		confirmation.Ask("Delete?", "body", "Delete")
		// Buttons() puts cancel last, so index 1 is cancel.
		confirmation.Answer(1)

		if confirmation.Allowed() {
			t.Error("choosing cancel must deny")
		}
	})

	t.Run("confirming allows", func(t *testing.T) {
		var confirmation Confirmation
		confirmation.Ask("Delete?", "body", "Delete")
		confirmation.Answer(0)

		if !confirmation.Allowed() {
			t.Error("choosing the confirming button must allow the action")
		}
	})

	t.Run("an out-of-range index denies", func(t *testing.T) {
		var confirmation Confirmation
		confirmation.Ask("Delete?", "body", "Delete")

		// A value no button corresponds to must not accidentally match the
		// confirming index.
		for _, result := range []ConfirmResult{-99, -2, 2, 7} {
			confirmation.Answer(result)
			if confirmation.Allowed() {
				t.Errorf("result %d must not authorise the action", result)
			}
		}
	})
}

// TestAStaleAnswerCannotAuthoriseANewAction covers reuse: a "yes" from an earlier
// dialog must not carry over to the next one.
func TestAStaleAnswerCannotAuthoriseANewAction(t *testing.T) {
	var confirmation Confirmation

	confirmation.Ask("Delete?", "first", "Delete")
	confirmation.Answer(0)
	if !confirmation.Allowed() {
		t.Fatal("the first confirmation should be allowed")
	}

	// A new question must clear the previous answer.
	confirmation.Ask("Delete?", "second", "Delete")
	if confirmation.Allowed() {
		t.Error("a new question must not inherit the previous answer")
	}
}

func TestResetClearsTheAnswer(t *testing.T) {
	var confirmation Confirmation
	confirmation.Ask("Delete?", "body", "Delete")
	confirmation.Answer(0)
	confirmation.Reset()

	if confirmation.Allowed() {
		t.Error("reset must clear the answer")
	}
	if confirmation.Asked() {
		t.Error("reset must close the dialog")
	}
	if confirmation.Denied() {
		t.Error("reset is not a denial; nothing was answered")
	}
}

// TestPendingActionRequiresApproval pins that the payload an action runs on is
// only handed out after approval.
func TestPendingActionRequiresApproval(t *testing.T) {
	t.Run("the zero value hands out nothing", func(t *testing.T) {
		var action PendingAction[[]string]

		if payload, ok := action.Approved(); ok {
			t.Errorf("an unarmed action must not be approved, got %v", payload)
		}
	})

	t.Run("arming without an answer hands out nothing", func(t *testing.T) {
		var action PendingAction[[]string]
		action.Arm([]string{"a", "b"}, "Delete?", "body", "Delete", "Cancel")

		if _, ok := action.Approved(); ok {
			t.Error("arming must not approve the action")
		}
	})

	t.Run("a denial hands out nothing", func(t *testing.T) {
		var action PendingAction[[]string]
		action.Arm([]string{"a", "b"}, "Delete?", "body", "Delete", "Cancel")
		action.Confirmation.Answer(1)

		if payload, ok := action.Approved(); ok {
			t.Errorf("a denied action must not run, got %v", payload)
		}
	})

	t.Run("an approval hands out exactly what was shown", func(t *testing.T) {
		var action PendingAction[[]string]
		action.Arm([]string{"a", "b"}, "Delete?", "body", "Delete", "Cancel")
		action.Confirmation.Answer(0)

		payload, ok := action.Approved()
		if !ok {
			t.Fatal("the action should be approved")
		}
		if len(payload) != 2 || payload[0] != "a" {
			t.Errorf("payload = %v, want what was shown", payload)
		}
	})

	t.Run("clearing forgets the payload", func(t *testing.T) {
		var action PendingAction[[]string]
		action.Arm([]string{"a"}, "Delete?", "body", "Delete", "Cancel")
		action.Confirmation.Answer(0)
		action.Clear()

		if _, ok := action.Approved(); ok {
			t.Error("a cleared action must not be approved")
		}
		if action.Payload != nil {
			t.Error("a cleared action must forget its payload")
		}
	})
}

// TestPayloadIsNotSwappableMidDialog pins that the action cannot be retargeted
// after the user has been shown what it will do.
func TestPayloadIsNotSwappableMidDialog(t *testing.T) {
	var action PendingAction[[]string]
	action.Arm([]string{"first"}, "Delete?", "body", "Delete", "Cancel")

	// Re-arming replaces both at once, so the payload and the text can never
	// disagree.
	action.Arm([]string{"second"}, "Delete?", "body", "Delete", "Cancel")
	action.Confirmation.Answer(0)

	payload, ok := action.Approved()
	if !ok {
		t.Fatal("the re-armed action should be approved")
	}
	if len(payload) != 1 || payload[0] != "second" {
		t.Errorf("payload = %v, want the payload the user was shown", payload)
	}
}

func TestButtonsPutCancelLast(t *testing.T) {
	var confirmation Confirmation
	confirmation.Ask("Delete?", "body", "Delete")

	buttons := confirmation.Buttons()
	if len(buttons) != 2 {
		t.Fatalf("got %d buttons, want 2", len(buttons))
	}
	// MyGo focuses the last button, and Enter presses it, so Cancel must be last.
	if buttons[0] != "Delete" {
		t.Errorf("first button = %q, want the confirming label", buttons[0])
	}
	if buttons[1] == "Delete" {
		t.Error("cancel must come last")
	}

	// The confirming index must agree with the order.
	confirmation.Answer(0)
	if !confirmation.Allowed() {
		t.Error("the first button confirms, so it must allow")
	}
}
