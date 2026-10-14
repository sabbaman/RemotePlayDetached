package fynegui

import (
	"strings"
	"testing"

	"fyne.io/fyne"
	"fyne.io/fyne/test"
	"fyne.io/fyne/widget"
)

func TestWrappedButtonEntry(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	entry := NewWrappedButtonEntry()
	text := strings.Repeat(`'WINEPREFIX=/home/user/Games/Astral Ascent' `, 30)
	entry.SetText(text)
	entry.Resize(fyne.NewSize(420, 300))
	entry.Refresh()
	if entry.Text != text {
		t.Fatal("visual wrapping changed the input text")
	}
	if !entry.MultiLine || entry.Wrapping != fyne.TextWrapBreak {
		t.Fatal("entry does not support wrapping long paths")
	}
	if entry.MinSize().Width > 420 {
		t.Fatalf("long input forces excessive minimum width: %v", entry.MinSize())
	}
	confirmed := 0
	entry.SetConfirmButton(widget.NewButton("OK", func() { confirmed++ }))
	entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEnter})
	if confirmed != 2 || entry.Text != text {
		t.Fatal("Enter must confirm the form without inserting a line break")
	}
	clipboard := test.NewClipboard()
	entry.TypedShortcut(&fyne.ShortcutSelectAll{})
	entry.TypedShortcut(&fyne.ShortcutCopy{Clipboard: clipboard})
	if clipboard.Content() != text {
		t.Fatal("copying wrapped input changed the command")
	}
}

func TestAppFieldsRejectRealLineBreaks(t *testing.T) {
	for _, tc := range []struct {
		text    string
		invalid bool
	}{
		{`--wrapper '"/path with spaces/proton" waitforexitandrun'`, false},
		{strings.Repeat("argument ", 300), false},
		{"", false},
		{"one\ntwo", true},
		{"one\r\ntwo", true},
		{"one\rtwo", true},
	} {
		if invalid := appFieldsError("Name", "/path", tc.text, "/directory") != nil; invalid != tc.invalid {
			t.Errorf("input %q: invalid = %v; want %v", tc.text, invalid, tc.invalid)
		}
	}
}
