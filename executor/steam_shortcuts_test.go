package executor

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func shortcutsFixture(apps ...App) []byte {
	var data bytes.Buffer
	key := func(kind byte, name string) {
		data.WriteByte(kind)
		data.WriteString(name)
		data.WriteByte(0)
	}
	text := func(name, value string) {
		key(1, name)
		data.WriteString(value)
		data.WriteByte(0)
	}
	key(0, "shortcuts")
	for i, app := range apps {
		key(0, string(rune('0'+i)))
		key(2, "appid")
		data.Write([]byte{1, 2, 3, 128})
		text("AppName", app.Name)
		text("Exe", `"`+app.Path+`"`)
		text("StartDir", `"`+app.WorkingDirectory+`"`)
		text("LaunchOptions", app.Args)
		key(2, "AllowOverlay")
		data.Write([]byte{1, 0, 0, 0})
		key(0, "tags")
		text("0", "Favorites")
		data.WriteByte(8)
		data.WriteByte(8)
	}
	data.Write([]byte{8, 8})
	return data.Bytes()
}

func TestParseSteamShortcuts(t *testing.T) {
	want := App{
		Name: "Astral Ascent", Path: "/usr/bin/heroic", WorkingDirectory: "/usr/bin",
		Args: `--no-gui --no-sandbox "heroic://launch?appName=77f83ebadd0543ec938020fb68dc49bb&runner=legendary"`,
	}
	apps, err := parseSteamShortcuts(shortcutsFixture(want, App{Name: "Missing executable"}))
	if err != nil || !reflect.DeepEqual(apps, []App{want}) {
		t.Fatalf("parsed shortcuts = %+v, %v; want %+v", apps, err, want)
	}
	windows := App{Name: "Windows Game", Path: `C:\Program Files\Game\game.exe`, WorkingDirectory: `C:\Program Files\Game`, Args: `--save "C:\My Saves"`}
	apps, err = parseSteamShortcuts(shortcutsFixture(windows))
	if err != nil || !reflect.DeepEqual(apps, []App{windows}) {
		t.Fatalf("Windows shortcuts = %+v, %v; want %+v", apps, err, windows)
	}
}

func TestParseSteamShortcutsMalformed(t *testing.T) {
	data := shortcutsFixture(App{Name: "Game", Path: "/bin/game"})
	for length := 0; length < len(data); length++ {
		if _, err := parseSteamShortcuts(data[:length]); err == nil {
			t.Fatalf("accepted truncated data of length %d", length)
		}
	}
	for _, invalid := range [][]byte{
		append(append([]byte(nil), data...), 0),
		{99, 'x', 0, 8},
		{0, 'x', 0, 8, 8},
	} {
		if _, err := parseSteamShortcuts(invalid); err == nil {
			t.Fatalf("accepted invalid data: %v", invalid)
		}
	}
	apps, err := parseSteamShortcuts(shortcutsFixture())
	if err != nil || len(apps) != 0 {
		t.Fatalf("empty shortcuts = %+v, %v", apps, err)
	}
}

func TestDiscoverSteamShortcutsProfiles(t *testing.T) {
	dir, cleanup := testDirectory(t)
	defer cleanup()
	want := []App{
		{Name: "Alpha", Path: "/bin/alpha"},
		{Name: "Zulu", Path: "/bin/zulu", Args: "--fullscreen"},
	}
	for _, profile := range []string{"123", "456"} {
		config := filepath.Join(dir, "userdata", profile, "config")
		if err := os.MkdirAll(config, 0700); err != nil {
			t.Fatal(err)
		}
		if err := ioutil.WriteFile(filepath.Join(config, "shortcuts.vdf"), shortcutsFixture(want[1], want[0]), 0600); err != nil {
			t.Fatal(err)
		}
	}
	apps, err := discoverSteamShortcuts([]string{dir, dir})
	if err != nil || !reflect.DeepEqual(apps, want) {
		t.Fatalf("discovered shortcuts = %+v, %v; want %+v", apps, err, want)
	}
	bad := filepath.Join(dir, "userdata", "456", "config", "shortcuts.vdf")
	if err := ioutil.WriteFile(bad, []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := discoverSteamShortcuts([]string{dir}); err == nil || !strings.Contains(err.Error(), bad) {
		t.Fatalf("expected contextual parse error, got %v", err)
	}
	if _, err := discoverSteamShortcuts([]string{filepath.Join(dir, "missing")}); err == nil || !strings.Contains(err.Error(), "no non-Steam shortcuts") {
		t.Fatalf("expected empty discovery error, got %v", err)
	}
}

func TestExecuteImportedSteamShortcut(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX executable")
	}
	dir, cleanup := testDirectory(t)
	defer cleanup()
	work := filepath.Join(dir, "Game Directory")
	if err := os.Mkdir(work, 0700); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(dir, "Heroic Launcher")
	if err := ioutil.WriteFile(executable, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > arguments\n"), 0700); err != nil {
		t.Fatal(err)
	}
	app := App{Name: "Game", Path: executable, WorkingDirectory: work, Args: `--no-gui "heroic://launch?appName=id&runner=legendary" "argument with spaces"`}
	apps, err := parseSteamShortcuts(shortcutsFixture(app))
	if err != nil {
		t.Fatal(err)
	}
	if err := executeApp(apps[0]); err != nil {
		t.Fatal(err)
	}
	want := "--no-gui\nheroic://launch?appName=id&runner=legendary\nargument with spaces\n"
	deadline := time.Now().Add(3 * time.Second)
	for {
		data, err := ioutil.ReadFile(filepath.Join(work, "arguments"))
		if err == nil && string(data) == want {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("unexpected child arguments: %q, %v; want %q", data, err, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
	assertRPDDirectory(t, dir)
}
