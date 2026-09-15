package editor

// THE EDITOR'S DIALOGS.
//
// This file used to be a modal WIDGET: a card, a scrim, button rows, focus
// handling and dismissal, 558 lines of it. golib/tui ships all of that now, so
// what is left is the two dialogs this editor actually shows and the rule for
// what each button does.
//
// TWO BEHAVIOURAL NOTES, both consequences of the widget change rather than
// choices made here:
//
//  1. A BUTTON DOES NOT CLOSE THE DIALOG BY ITSELF. Upstream, only Escape
//     self-dismisses; a button's role decides where Escape routes and which
//     control takes initial focus, not whether the dialog closes. Every button
//     below therefore calls Dismiss with the reason it means. The old widget
//     closed on any activation, which quietly made "Yes" and "No" the same
//     event as far as an observer was concerned.
//
//  2. THE y / n / o MNEMONICS ARE GONE. The old buttons carried a mnemonic
//     rune; golib's Button has no equivalent, and the alternatives all fight
//     the widget's focus model rather than extending it. The role semantics
//     cover the same ground for dialogs this shape: the default-role button
//     takes initial focus so Enter confirms, and Escape resolves to the
//     cancel-role button. Tab still moves between them.

import (
	"github.com/yongjohnlee80/golib/tui/widget"
)

// OpenModal puts a dialog up, replacing whatever was already there.
//
// One at a time: this editor has no flow that stacks dialogs, and replacing is
// a more predictable answer than silently refusing the second one.
func (tm *TopMenu) OpenModal(m *widget.Modal) error {
	if m == nil || tm.host == nil {
		return nil
	}
	tm.CloseModal()
	if err := m.Open(tm.host); err != nil {
		return err
	}
	tm.modal = m
	return nil
}

// CloseModal dismisses the current dialog, if there is one.
//
// Dismiss is idempotent upstream, so calling this after a button has already
// dismissed costs nothing — which is what lets the button handlers below say
// what they mean without coordinating with this method.
func (tm *TopMenu) CloseModal() {
	if tm.modal == nil {
		return
	}
	tm.modal.Dismiss(widget.DismissProgrammatic)
	tm.modal = nil
}

// finish is what every dialog does on its way out: forget it, close the menu
// behind it, and put focus back in the editor.
//
// Shared because getting one of the three wrong is invisible until someone
// types into a dead editor.
func (tm *TopMenu) finish() {
	tm.modal = nil
	tm.Deactivate()
}

// newExitModal builds the quit confirmation.
//
// Yes is the default role and No the cancel role, so Enter quits and Escape
// does not — which is the safe way round for a dialog whose confirm branch
// discards the session.
func (tm *TopMenu) newExitModal() *widget.Modal {
	var m *widget.Modal

	yes := widget.NewButton("Yes",
		widget.WithRole(widget.ButtonRoleDefault),
		widget.WithOnActivate(func() {
			m.Dismiss(widget.DismissAccept)
			tm.finish()
			if tm.cb.OnQuit != nil {
				tm.cb.OnQuit()
			}
		}))
	no := widget.NewButton("No",
		widget.WithRole(widget.ButtonRoleCancel),
		widget.WithOnActivate(func() {
			m.Dismiss(widget.DismissCancel)
			tm.finish()
		}))

	m = widget.NewModal(widget.NewText("Are you sure to quit?", widget.WithTextStyle(bodyStyle)),
		widget.WithModalTitle("Exit Confirmation"),
		widget.WithModalStyle(defaultModalStyle),
		widget.WithButtons(yes, no),
		// Escape reaches here too, having routed through the cancel button, so
		// the teardown lives in one place rather than in each button.
		widget.WithOnDismiss(func(widget.DismissReason) { tm.finish() }))
	return m
}

// newNoticeModal builds a one-button acknowledgement.
//
// The single button is the cancel role as well as the default one in effect:
// with no second control, Escape and Enter must both resolve, and giving it the
// cancel role is what makes Escape route to it rather than closing around it.
func (tm *TopMenu) newNoticeModal(title, body string) *widget.Modal {
	var m *widget.Modal

	ok := widget.NewButton("OK",
		widget.WithRole(widget.ButtonRoleCancel),
		widget.WithOnActivate(func() {
			m.Dismiss(widget.DismissAccept)
			tm.finish()
		}))

	m = widget.NewModal(widget.NewText(body, widget.WithTextStyle(bodyStyle)),
		widget.WithModalTitle(title),
		widget.WithModalStyle(defaultModalStyle),
		widget.WithButtons(ok),
		widget.WithOnDismiss(func(widget.DismissReason) { tm.finish() }))
	return m
}

// OpenExitModal shows the quit confirmation.
func (tm *TopMenu) OpenExitModal() { _ = tm.OpenModal(tm.newExitModal()) }

// OpenNotImplemented reports that a menu command has no implementation yet.
//
// A dialog rather than a status message because it is reached by choosing a
// menu item, and a menu that appears to do nothing reads as broken.
func (tm *TopMenu) OpenNotImplemented(what string) {
	_ = tm.OpenModal(tm.newNoticeModal("Not Implemented", what+" is not implemented yet."))
}
