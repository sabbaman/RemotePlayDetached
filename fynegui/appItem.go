package fynegui

import (
	"fmt"

	"fyne.io/fyne"
	"fyne.io/fyne/layout"
	"fyne.io/fyne/widget"
	"github.com/smaTc/RemotePlayDetached/executor"
)

// AppItem struct
type AppItem struct {
	App executor.App
	widget.Box
}

// NewAppItem func
func NewAppItem(app executor.App) fyne.Widget {
	item := &AppItem{App: app}
	item.ExtendBaseWidget(item)
	item.Box = *widget.NewHBox()
	item.Box.Append(layout.NewSpacer())

	runButton := widget.NewButton("Run", func() {
		fmt.Println("Run App")
		err := executor.RunApp(app)
		if err != nil {
			TextPopup(err.Error(), "Error:")
		}
	})
	item.Box.Append(runButton)

	editButton := widget.NewButton("Edit", func() {
		fmt.Println("Edit App")
		editApp(app)
	})
	item.Box.Append(editButton)

	deleteButton := widget.NewButton("Delete", func() {
		fmt.Println("Delete App")
		executor.DeleteApp(app)
		refreshMainWindow()
	})
	item.Box.Append(deleteButton)

	return item
}

func editApp(oldApp executor.App) {
	editWindow := rpd.NewWindow("Edit App")
	editWindow.Resize(fyne.NewSize(760, 520))

	nameEntry := NewButtonEntry()
	nameEntry.SetText(oldApp.Name)

	pathEntry := NewWrappedButtonEntry()
	pathEntry.SetText(oldApp.Path)

	argsEntry := NewWrappedButtonEntry()
	argsEntry.SetText(oldApp.Args)
	workingDirectoryEntry := NewWrappedButtonEntry()
	workingDirectoryEntry.SetPlaceHolder("Empty = executable directory")
	workingDirectoryEntry.SetText(oldApp.WorkingDirectory)

	name := widget.NewFormItem("Name", nameEntry)
	path := widget.NewFormItem("Path", pathEntry)
	args := widget.NewFormItem("Args", argsEntry)
	workingDirectory := widget.NewFormItem("Working Directory", workingDirectoryEntry)
	form := widget.NewForm(name, path, args, workingDirectory)

	cancelButton := widget.NewButton("Cancel", func() {
		editWindow.Close()
	})

	okButton := widget.NewButton("OK", func() {
		appName := nameEntry.Text
		appPath := pathEntry.Text
		argsString := argsEntry.Text

		if appName == "" || appPath == "" {
			return
		}
		if err := appFieldsError(appName, appPath, argsString, workingDirectoryEntry.Text); err != nil {
			TextPopup(err.Error(), "Invalid Input")
			return
		}

		newApp := executor.App{Name: appName, Path: appPath, Args: argsString, WorkingDirectory: workingDirectoryEntry.Text}
		editWindow.Close()
		executor.EditApp(oldApp, newApp)
		refreshMainWindow()
	})

	fileExlporerButton := widget.NewButton("File Explorer", func() {
		FileExplorer(pathEntry)
	})

	nameEntry.SetConfirmButton(okButton)
	pathEntry.SetConfirmButton(okButton)
	argsEntry.SetConfirmButton(okButton)
	workingDirectoryEntry.SetConfirmButton(okButton)

	buttons := fyne.NewContainerWithLayout(layout.NewHBoxLayout(), okButton, layout.NewSpacer(), fileExlporerButton, layout.NewSpacer(), cancelButton)

	scroll := widget.NewScrollContainer(form)
	editWindow.SetContent(fyne.NewContainerWithLayout(layout.NewBorderLayout(nil, buttons, nil, nil), scroll, buttons))

	editWindow.Show()
}
