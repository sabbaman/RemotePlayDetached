package fynegui

import (
	"errors"
	"strings"

	"fyne.io/fyne"
	"fyne.io/fyne/widget"
)

// ButtonEntry Widget
type ButtonEntry struct {
	widget.Entry
	confirmButton *widget.Button
}

func (e *ButtonEntry) onKey() {
	e.confirmButton.OnTapped()
}

// TypedKey keeps Enter as form confirmation, even in visually wrapped entries.
func (e *ButtonEntry) TypedKey(key *fyne.KeyEvent) {
	switch key.Name {
	case fyne.KeyReturn, fyne.KeyEnter:
		if e.confirmButton != nil {
			e.onKey()
		}
	default:
		e.Entry.TypedKey(key)
	}
}

// SetConfirmButton func
func (e *ButtonEntry) SetConfirmButton(b *widget.Button) {
	e.confirmButton = b
}

// NewButtonEntry func
func NewButtonEntry() *ButtonEntry {
	entry := &ButtonEntry{}
	entry.ExtendBaseWidget(entry)
	return entry
}

func NewWrappedButtonEntry() *ButtonEntry {
	entry := NewButtonEntry()
	entry.MultiLine = true
	entry.Wrapping = fyne.TextWrapBreak
	return entry
}

func appFieldsError(fields ...string) error {
	for _, field := range fields {
		if strings.ContainsAny(field, "\r\n") {
			return errors.New("Fields must contain a single logical line.\nVisual wrapping does not add line breaks; remove any pasted line breaks before saving.")
		}
	}
	return nil
}

// ClickLabel struct
type ClickLabel struct {
	widget.Label
	tapped func()
}

// NewClickLabel func
func NewClickLabel(labelName string, t func()) *ClickLabel {
	cl := &ClickLabel{Label: widget.Label{Text: labelName}, tapped: t}
	cl.ExtendBaseWidget(cl)
	return cl
}

// Tapped func
func (cl *ClickLabel) Tapped(_ *fyne.PointEvent) {
	cl.tapped()
}

// TappedSecondary func
func (cl *ClickLabel) TappedSecondary(_ *fyne.PointEvent) {
}
