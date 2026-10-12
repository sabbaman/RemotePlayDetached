package executor

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDiscoverSteamLibraries(t *testing.T) {
	dir, cleanup := testDirectory(t)
	defer cleanup()
	root := filepath.Join(dir, "Steam")
	extra := filepath.Join(dir, "Other Library")
	for _, library := range []string{root, extra} {
		if err := os.MkdirAll(filepath.Join(library, "steamapps", "common"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path, data string) {
		t.Helper()
		if err := ioutil.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	manifest := func(library, id, name, install string, installed bool) {
		t.Helper()
		write(filepath.Join(library, "steamapps", "appmanifest_"+id+".acf"),
			`"AppState" { "appid" "`+id+`" "name" "`+name+`" "installdir" "`+install+`" }`)
		if installed {
			if err := os.MkdirAll(filepath.Join(library, "steamapps", "common", install), 0700); err != nil {
				t.Fatal(err)
			}
		}
	}
	manifest(root, "20", "Zulu", "Zulu", true)
	manifest(extra, "10", `Alpha \"Game\"`, "Alpha Game", true)
	manifest(extra, "20", "Duplicate", "Duplicate", true)
	manifest(root, "30", "Removed", "Removed", false)
	write(filepath.Join(root, "steamapps", "appmanifest_bad.acf"), `"AppState" { "appid" "invalid" "name" "Bad" }`)
	for _, folders := range []string{
		`"libraryfolders" { "0" { "path" "` + root + `" "apps" { "20" "100" } } "1" { "path" "` + extra + `" } }`,
		`"LibraryFolders" { "0" "` + root + `" "1" "` + extra + `" }`,
	} {
		write(filepath.Join(root, "steamapps", "libraryfolders.vdf"), folders)
		games, err := discoverSteamLibraries([]string{root, root})
		if err != nil {
			t.Fatal(err)
		}
		if len(games) != 2 || games[0].Name != `Alpha "Game"` || games[0].AppID != "10" || games[1].Name != "Zulu" {
			t.Fatalf("unexpected games: %+v", games)
		}
		if games[0].InstallDir != filepath.Join(extra, "steamapps", "common", "Alpha Game") {
			t.Fatalf("unexpected install directory: %q", games[0].InstallDir)
		}
	}
}

func TestSteamValuesEscapes(t *testing.T) {
	values := steamValues([]byte(`"path" "C:\\Games\\Steam" "name" "A \"quoted\" game"`))
	if values["path"] != `C:\Games\Steam` || values["name"] != `A "quoted" game` {
		t.Fatalf("unexpected values: %v", values)
	}
}

func TestDiscoverSteamLibrariesEmpty(t *testing.T) {
	_, err := discoverSteamLibraries([]string{filepath.Join(os.TempDir(), "rpd-missing-steam-library")})
	if err == nil || !strings.Contains(err.Error(), "no installed Steam games") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDiscoverSteamGamesLaunchers(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("uses Linux Steam locations and executable scripts")
	}
	dir, cleanup := testDirectory(t)
	defer cleanup()
	for _, key := range []string{"HOME", "PATH", "STEAM_DIR", "XDG_DATA_HOME"} {
		key := key
		value, present := os.LookupEnv(key)
		defer func() {
			if present {
				os.Setenv(key, value)
			} else {
				os.Unsetenv(key)
			}
		}()
		os.Unsetenv(key)
	}
	for _, flatpak := range []bool{false, true} {
		home := filepath.Join(dir, "native")
		root := filepath.Join(home, ".local", "share", "Steam")
		command := "steam"
		wantArgs := ""
		if flatpak {
			home = filepath.Join(dir, "flatpak")
			root = filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam")
			command = "flatpak"
			wantArgs = "run com.valvesoftware.Steam"
		}
		bin := filepath.Join(home, "bin")
		if err := os.MkdirAll(bin, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(root, "steamapps", "common", "Game"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := ioutil.WriteFile(filepath.Join(bin, command), []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := ioutil.WriteFile(filepath.Join(root, "steamapps", "appmanifest_10.acf"), []byte(`"AppState" { "appid" "10" "name" "Game" "installdir" "Game" }`), 0600); err != nil {
			t.Fatal(err)
		}
		os.Setenv("HOME", home)
		os.Setenv("PATH", bin)
		games, launcher, err := DiscoverSteamGames()
		if err != nil {
			t.Fatal(err)
		}
		if len(games) != 1 || games[0].AppID != "10" || launcher.Path != filepath.Join(bin, command) || launcher.Args != wantArgs {
			t.Fatalf("unexpected discovery: %+v, %+v", games, launcher)
		}
	}
}
