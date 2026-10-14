package fynegui

import (
	"strings"
	"unicode"

	"fyne.io/fyne"
	"fyne.io/fyne/layout"
	"fyne.io/fyne/widget"
)

// TextPopup func
func TextPopup(text, popupType string) {
	var window fyne.Window
	init := Init()
	window = rpd.NewWindow("RPD Notification")

	typeLabel := widget.NewLabel(popupType)
	textLabel := widget.NewLabel(wrapPopupText(text))
	okButton := widget.NewButton("OK", func() {
		window.Close()
		if exitAfterExec {
			rpd.Quit()
		}
	})

	headerContainer := fyne.NewContainerWithLayout(layout.NewHBoxLayout(), layout.NewSpacer(), typeLabel, layout.NewSpacer())
	buttonContainer := fyne.NewContainerWithLayout(layout.NewHBoxLayout(), layout.NewSpacer(), okButton, layout.NewSpacer())
	scroll := widget.NewScrollContainer(textLabel)
	windowContainer := fyne.NewContainerWithLayout(layout.NewBorderLayout(headerContainer, buttonContainer, nil, nil), headerContainer, scroll, buttonContainer)

	window.SetContent(windowContainer)
	window.Resize(fyne.NewSize(640, 360))
	if exitAfterExec {
		window.SetMaster()
		window.ShowAndRun()
	} else {
		if init {
			popupsAfterLoading = append(popupsAfterLoading, window)
		} else {
			window.Show()
		}

	}
}

// Bound diagnostic display lines, including unbroken paths.
func wrapPopupText(text string) string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		chars := []rune(line)
		for len(chars) > 80 {
			end := 80
			for i := end; i > 0; i-- {
				if unicode.IsSpace(chars[i]) {
					end = i
					break
				}
			}
			lines = append(lines, string(chars[:end]))
			chars = chars[end:]
			for len(chars) > 0 && unicode.IsSpace(chars[0]) {
				chars = chars[1:]
			}
		}
		lines = append(lines, string(chars))
	}
	return strings.Join(lines, "\n")
}
