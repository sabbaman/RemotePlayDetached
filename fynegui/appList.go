package fynegui

import (
	"fmt"
	"strings"

	"fyne.io/fyne"
	"fyne.io/fyne/layout"
	"fyne.io/fyne/widget"
	"github.com/smaTc/RemotePlayDetached/executor"
)

var apps *[]executor.App

var appList *widget.Form

// var appList widget.NewVBox
var appListContainer *fyne.Container
var appListGroup *widget.Group

func importApp() {
	importWindow := rpd.NewWindow("Import App")
	importWindow.Resize(fyne.NewSize(760, 520))

	nameEntry := NewButtonEntry()

	pathEntry := NewWrappedButtonEntry()

	argsEntry := NewWrappedButtonEntry()
	workingDirectoryEntry := NewWrappedButtonEntry()
	workingDirectoryEntry.SetPlaceHolder("Empty = executable directory")

	/*
		nameEntry := widget.NewEntry()
		pathEntry := widget.NewEntry()
		argsEntry := widget.NewEntry()
	*/

	name := widget.NewFormItem("Name", nameEntry)

	path := widget.NewFormItem("Path", pathEntry)

	args := widget.NewFormItem("Args", argsEntry)
	workingDirectory := widget.NewFormItem("Working Directory", workingDirectoryEntry)
	form := widget.NewForm(name, path, args, workingDirectory)
	cancelButton := widget.NewButton("Cancel", func() {
		importWindow.Close()
		if explorerWindow != nil {
			explorerWindow.Close()
		}
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
		importWindow.Close()
		executor.ImportApp(newApp)
		refreshMainWindow()
	})

	fileExlporerButton := widget.NewButton("File Explorer", func() {
		FileExplorer(pathEntry)
	})
	showLibrary := func(title string, entries []executor.App) {
		window := rpd.NewWindow(title)
		window.Resize(fyne.NewSize(500, 400))
		search := widget.NewEntry()
		search.SetPlaceHolder("Search games...")
		list := widget.NewVBox()
		update := func(query string) {
			list.Children = nil
			for _, entry := range entries {
				entry := entry
				if !strings.Contains(strings.ToLower(entry.Name), strings.ToLower(query)) {
					continue
				}
				list.Append(widget.NewButton(entry.Name, func() {
					// Saved entry names are also filenames.
					nameEntry.SetText(strings.NewReplacer("/", "_", "\\", "_", "\n", " ", "\r", " ").Replace(entry.Name))
					pathEntry.SetText(entry.Path)
					argsEntry.SetText(entry.Args)
					workingDirectoryEntry.SetText(entry.WorkingDirectory)
					window.Close()
				}))
			}
			if len(list.Children) == 0 {
				list.Append(widget.NewLabel("No matching games"))
			}
			list.Refresh()
		}
		search.OnChanged = update
		update("")
		scroll := widget.NewScrollContainer(list)
		header := widget.NewVBox(search, widget.NewLabel("Select a target game, not the donor that launches RPD."))
		window.SetContent(fyne.NewContainerWithLayout(layout.NewBorderLayout(header, nil, nil, nil), header, scroll))
		window.Show()
	}
	steamButton := widget.NewButton("Steam Library", func() {
		games, client, err := executor.DiscoverSteamGames()
		if err != nil {
			TextPopup(err.Error(), "Steam Library")
			return
		}
		entries := make([]executor.App, 0, len(games))
		for _, game := range games {
			entries = append(entries, executor.App{
				Name: game.Name, Path: client.Path,
				Args:             strings.TrimSpace(client.Args + " -applaunch " + game.AppID),
				WorkingDirectory: game.InstallDir,
			})
		}
		showLibrary("Steam Library", entries)
	})
	nonSteamButton := widget.NewButton("Non-Steam", func() {
		entries, err := executor.DiscoverSteamShortcuts()
		if err != nil {
			TextPopup(err.Error(), "Non-Steam Shortcuts")
			return
		}
		showLibrary("Non-Steam Shortcuts", entries)
	})
	heroicButton := widget.NewButton("Heroic", func() {
		entries, err := executor.DiscoverHeroicGames()
		if err != nil {
			TextPopup(err.Error(), "Heroic Games")
			return
		}
		showLibrary("Heroic Games", entries)
	})

	nameEntry.SetConfirmButton(okButton)
	pathEntry.SetConfirmButton(okButton)
	argsEntry.SetConfirmButton(okButton)
	workingDirectoryEntry.SetConfirmButton(okButton)

	buttons := fyne.NewContainerWithLayout(layout.NewHBoxLayout(), okButton, layout.NewSpacer(), steamButton, nonSteamButton, heroicButton, fileExlporerButton, layout.NewSpacer(), cancelButton)

	scroll := widget.NewScrollContainer(form)
	importWindow.SetContent(fyne.NewContainerWithLayout(layout.NewBorderLayout(nil, buttons, nil, nil), scroll, buttons))

	importWindow.Show()
}

func buildAppListContainer() *widget.Group {
	appListGroup = loadApps()
	return appListGroup
}

func addToList(app executor.App) {
	appItem := NewAppItem(app)
	formItem := widget.NewFormItem(app.Name, appItem)
	appList.AppendItem(formItem)
}

func loadApps() *widget.Group {
	fmt.Println("(re)loading apps")
	apps = executor.GetApps()

	itemList := make([]*widget.FormItem, 0)

	for _, app := range *apps {
		appItem := NewAppItem(app)
		formItem := widget.NewFormItem(app.Name, appItem)
		itemList = append(itemList, formItem)
	}

	appList = widget.NewForm(itemList...)
	appGroup := widget.NewGroupWithScroller("Apps", appList)
	return appGroup
}
