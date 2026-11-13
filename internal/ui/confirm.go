package ui

// Dangerous actions are confirmed before they run, and the confirmation fails
// closed.
//
// "Fails closed" is the whole design: the action proceeds only when the user
// explicitly chose the confirming button. Every other outcome — the dialog was
// dismissed with Escape, the window closed, an index arrived that no button
// corresponds to, or nothing has been asked yet — denies. That is what makes it
// impossible for a missing or mis-wired answer to delete a user's data.

// ConfirmResult is what a dialog reported.
type ConfirmResult = int

// DismissedValue is what MyGo's AlertDialog returns when it closes without a
// button being chosen.
const DismissedValue ConfirmResult = -1

// Confirmation is a destructive action awaiting an answer.
//
// The zero value is safe: it has asked nothing, so it allows nothing.
type Confirmation struct {
	// Title and Body are shown to the user.
	Title string
	Body  string
	// ConfirmLabel and CancelLabel name the buttons.
	ConfirmLabel string
	CancelLabel  string

	// confirmIndex is the position of the confirming button. It is a field
	// rather than an assumption, because assuming which index means "yes" is
	// exactly how a dialog ends up confirming when the user cancelled.
	confirmIndex int

	// asked is true while a dialog is on screen.
	asked bool
	// answer is nil until the user answers.
	answer *ConfirmResult
}

// Buttons are the dialog's labels, in the order they are shown.
//
// Cancel comes last because MyGo gives the last button the focus and Enter: the
// destructive one must never be the button a reflexive Return presses.
func (c *Confirmation) Buttons() []string {
	cancel := c.CancelLabel
	if cancel == "" {
		cancel = "Cancel"
	}
	confirm := c.ConfirmLabel
	if confirm == "" {
		confirm = "Confirm"
	}
	return []string{confirm, cancel}
}

// Ask opens a confirmation with the given text.
//
// It resets any previous answer, so a stale "yes" from an earlier dialog cannot
// authorise a new action.
func (c *Confirmation) Ask(title, body, confirmLabel string) {
	c.Title = title
	c.Body = body
	c.ConfirmLabel = confirmLabel
	c.CancelLabel = ""
	// Buttons() puts cancel last, so the confirming button is index 0.
	c.confirmIndex = 0
	c.asked = true
	c.answer = nil
}

// Asked reports whether a dialog is on screen.
func (c *Confirmation) Asked() bool { return c.asked }

// Answer records what the dialog reported.
func (c *Confirmation) Answer(result ConfirmResult) {
	copied := result
	c.answer = &copied
	c.asked = false
}

// Allowed reports whether the action may proceed.
//
// This is the only way to authorise a destructive action, and it is true only for
// an explicit confirming click.
func (c *Confirmation) Allowed() bool {
	return c.answer != nil && *c.answer == c.confirmIndex
}

// Denied reports that the user was asked and did not confirm.
//
// It is distinct from "not asked" so a caller can tell "the user said no" from
// "nothing happened yet", which are different things to log.
func (c *Confirmation) Denied() bool {
	return c.answer != nil && *c.answer != c.confirmIndex
}

// Open reports whether the dialog should be drawn.
// It is the pointer MyGo's AlertDialog binds to, so it clears itself when the
// dialog closes.
func (c *Confirmation) Open() *bool { return &c.asked }

// Reset clears the confirmation without acting on it.
func (c *Confirmation) Reset() {
	c.asked = false
	c.answer = nil
}

// PendingAction carries the data an approved action needs.
//
// The payload is stored with the confirmation rather than passed separately, so
// the action cannot be performed against a different selection than the one the
// user was shown.
type PendingAction[T any] struct {
	Confirmation Confirmation
	// Payload is what the action will operate on.
	Payload T
	// armed is true once Ask has been given a payload.
	armed bool
}

// Arm opens a confirmation for a specific payload.
func (p *PendingAction[T]) Arm(payload T, title, body, confirmLabel string) {
	p.Payload = payload
	p.armed = true
	p.Confirmation.Ask(title, body, confirmLabel)
}

// Approved returns the payload, and whether the action may proceed.
//
// Both must be checked together: a caller that read Payload after Allowed would
// still be safe, but one that read it after a denial would act on stale data.
func (p *PendingAction[T]) Approved() (T, bool) {
	if !p.armed || !p.Confirmation.Allowed() {
		var zero T
		return zero, false
	}
	return p.Payload, true
}

// Clear forgets the action, whether it ran or was refused.
func (p *PendingAction[T]) Clear() {
	p.Confirmation.Reset()
	p.armed = false
	var zero T
	p.Payload = zero
}
