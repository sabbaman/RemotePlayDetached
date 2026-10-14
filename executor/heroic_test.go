package executor

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

type heroicFixture struct {
	root, home, install, runner, wine, bin string
	settings                               map[string]interface{}
}

func writeHeroicFixture(t *testing.T, path string, value interface{}) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func heroicExecutableFixture(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	// A backend records argv without ever invoking Heroic, Wine, or a game.
	if err := ioutil.WriteFile(path, []byte("#!/bin/sh\nprintf '%s\\000' \"$@\" > heroic-argv\nprintf '%s\\000' \"$WINEPREFIX\" \"$LEGENDARY_CONFIG_PATH\" \"$TEST_VALUE\" \"$PWD\" > heroic-env\n"), 0700); err != nil {
		t.Fatal(err)
	}
}

func newHeroicFixture(t *testing.T) heroicFixture {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("direct Heroic import supports Linux")
	}
	dir := t.TempDir()
	f := heroicFixture{
		root:    filepath.Join(dir, `Heroic's config\directory`),
		home:    filepath.Join(dir, `User's home\directory`),
		install: filepath.Join(dir, `Game's install\directory`),
		runner:  filepath.Join(dir, `Runner's binaries\directory`, "custom backend"),
		wine:    filepath.Join(dir, `Wine's binaries\directory`, "wine"),
		bin:     filepath.Join(dir, "path"),
	}
	for _, path := range []string{f.root, f.home, f.install, f.bin} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	heroicExecutableFixture(t, f.runner)
	heroicExecutableFixture(t, f.wine)
	env, err := exec.LookPath("env")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(env, filepath.Join(f.bin, "env")); err != nil {
		t.Fatal(err)
	}
	// Isolate runner and UMU lookup from the developer's installed tools.
	t.Setenv("PATH", f.bin)
	f.settings = map[string]interface{}{
		"altLegendaryBin": f.runner, "altGogdlBin": f.runner, "altNileBin": f.runner,
		"wineVersion": map[string]string{"bin": f.wine, "type": "wine"},
	}
	return f
}

func (f heroicFixture) saveSettings(t *testing.T) {
	t.Helper()
	writeHeroicFixture(t, filepath.Join(f.root, "config.json"), map[string]interface{}{"defaultSettings": f.settings})
}

func (f heroicFixture) epic(t *testing.T, games map[string]interface{}) {
	t.Helper()
	writeHeroicFixture(t, filepath.Join(f.root, "legendaryConfig", "legendary", "installed.json"), games)
}

func heroicCommandFixture(t *testing.T, app App, backend string) (map[string]string, []string) {
	t.Helper()
	args, err := parseArguments(app.Args)
	if err != nil {
		t.Fatal(err)
	}
	if len(args) == 0 || args[0] != "--" {
		t.Fatalf("missing env option terminator: %q", args)
	}
	environment := make(map[string]string)
	for i := 1; i < len(args); i++ {
		if args[i] == backend {
			return environment, args[i+1:]
		}
		entry := strings.SplitN(args[i], "=", 2)
		if len(entry) != 2 {
			t.Fatalf("invalid environment assignment: %q", args[i])
		}
		environment[entry[0]] = entry[1]
	}
	t.Fatalf("configured backend %q missing from %q", backend, args)
	return nil, nil
}

func TestDiscoverHeroicEpicCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name, kind                  string
		umu, disableUMU, bundledUMU bool
	}{
		{name: "wine", kind: "wine"},
		{name: "proton without umu", kind: "proton"},
		{name: "proton with umu", kind: "proton", umu: true},
		{name: "proton with bundled umu", kind: "proton", bundledUMU: true},
		{name: "proton disabled umu", kind: "proton", umu: true, disableUMU: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newHeroicFixture(t)
			f.settings["wineVersion"] = map[string]string{"bin": "'" + f.wine + "'", "type": tc.kind}
			f.settings["disableUMU"] = tc.disableUMU
			umu := filepath.Join(f.bin, "umu-run")
			if tc.bundledUMU {
				umu = filepath.Join(f.root, "tools", "runtimes", "umu", "umu_run.py")
			}
			if tc.umu || tc.bundledUMU {
				heroicExecutableFixture(t, umu)
			}
			f.saveSettings(t)
			f.epic(t, map[string]interface{}{"epic-id": map[string]interface{}{"title": "Epic Game", "install_path": f.install, "platform": "Windows"}})
			apps, err := discoverHeroicGames([]string{f.root}, f.home)
			if err != nil || len(apps) != 1 {
				t.Fatalf("discovery = %+v, %v", apps, err)
			}
			app := apps[0]
			if app.Name != "Epic Game [Heroic legendary]" || app.WorkingDirectory != f.install || app.Path != filepath.Join(f.bin, "env") {
				t.Fatalf("unexpected app: %+v", app)
			}
			environment, args := heroicCommandFixture(t, app, f.runner)
			prefix := filepath.Join(f.home, "Games", "Heroic", "Prefixes", "shared")
			wantEnv := map[string]string{"LEGENDARY_CONFIG_PATH": filepath.Join(f.root, "legendaryConfig", "legendary"), "WINEPREFIX": prefix}
			wantArgs := []string{"launch", "epic-id", "--wine", f.wine}
			if tc.kind == "proton" {
				wantEnv["STEAM_COMPAT_DATA_PATH"] = prefix
				wantEnv["STEAM_COMPAT_CLIENT_INSTALL_PATH"] = filepath.Join(f.home, ".steam", "steam")
				wantEnv["STEAM_COMPAT_INSTALL_PATH"] = f.install
				wantEnv["PROTONPATH"] = filepath.Dir(f.wine)
				wantArgs = []string{"launch", "epic-id", "--no-wine", "--wrapper"}
				wantWrapper := []string{f.wine, "waitforexitandrun"}
				if len(args) != 5 {
					t.Fatalf("Proton args: %q", args)
				}
				wrapper, err := parseArguments(args[4])
				if err != nil || !reflect.DeepEqual(wrapper, wantWrapper) {
					t.Fatalf("wrapper = %q, %v; want %q", wrapper, err, wantWrapper)
				}
				wantArgs = append(wantArgs, args[4])
			}
			if !reflect.DeepEqual(environment, wantEnv) || !reflect.DeepEqual(args, wantArgs) {
				t.Fatalf("launch = %v, %q; want %v, %q", environment, args, wantEnv, wantArgs)
			}
		})
	}
}

func TestHeroicGameLanguage(t *testing.T) {
	for _, tc := range []struct{ name, storeLanguage, gameLanguage, want string }{
		{"UI language is not inherited", "", "", ""},
		{"store fallback", "it", "", "it"},
		{"game override", "it", "de", "de"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newHeroicFixture(t)
			f.settings["language"] = "fr"
			f.saveSettings(t)
			if tc.storeLanguage != "" {
				writeHeroicFixture(t, filepath.Join(f.root, "store", "config.json"), map[string]string{"language": tc.storeLanguage})
			}
			if tc.gameLanguage != "" {
				writeHeroicFixture(t, filepath.Join(f.root, "GamesConfig", "game.json"), map[string]interface{}{"game": map[string]string{"language": tc.gameLanguage}})
			}
			f.epic(t, map[string]interface{}{"game": map[string]string{"install_path": f.install}})
			apps, err := discoverHeroicGames([]string{f.root}, f.home)
			if err != nil || len(apps) != 1 {
				t.Fatalf("discovery = %v, %v", apps, err)
			}
			_, args := heroicCommandFixture(t, apps[0], f.runner)
			got := ""
			for i, arg := range args {
				if arg == "--language" && i+1 < len(args) {
					got = args[i+1]
				}
			}
			if got != tc.want {
				t.Fatalf("language = %q; want %q", got, tc.want)
			}
		})
	}
}

func TestHeroicErrorSeparatesDiagnostics(t *testing.T) {
	f := newHeroicFixture(t)
	f.saveSettings(t)
	f.epic(t, map[string]interface{}{
		"missing-one": map[string]string{"install_path": filepath.Join(f.home, "missing-one")},
		"missing-two": map[string]string{"install_path": filepath.Join(f.home, "missing-two")},
	})
	_, err := discoverHeroicGames([]string{f.root}, f.home)
	if err == nil {
		t.Fatal("expected discovery error")
	}
	parts := strings.Split(err.Error(), "\n\n")
	if len(parts) != 3 || !strings.HasPrefix(parts[1], "Details:\nlegendary/") || !strings.HasPrefix(parts[2], "legendary/") {
		t.Fatalf("diagnostics not separated into paragraphs: %q", err.Error())
	}
}

func TestHeroicAnticheatWithDirectProton(t *testing.T) {
	for _, disableUMU := range []bool{false, true} {
		f := newHeroicFixture(t)
		f.settings["wineVersion"] = map[string]string{"bin": f.wine, "type": "proton"}
		f.settings["disableUMU"] = disableUMU
		f.settings["eacRuntime"] = true
		f.settings["battlEyeRuntime"] = true
		f.saveSettings(t)
		heroicExecutableFixture(t, filepath.Join(f.bin, "umu-run"))
		writeHeroicFixture(t, filepath.Join(f.root, "store_cache", "umu.json"), map[string]interface{}{"legendary_game": "umu-known-game", "__timestamp": map[string]string{"legendary_game": "2020-01-01"}})
		f.epic(t, map[string]interface{}{"game": map[string]string{"install_path": f.install}})
		apps, err := discoverHeroicGames([]string{f.root}, f.home)
		if err != nil || len(apps) != 1 {
			t.Fatalf("discovery = %v, %v", apps, err)
		}
		environment, _ := heroicCommandFixture(t, apps[0], f.runner)
		for key, want := range map[string]string{
			"PROTON_EAC_RUNTIME":      filepath.Join(f.root, "tools", "runtimes", "eac_runtime"),
			"PROTON_BATTLEYE_RUNTIME": filepath.Join(f.root, "tools", "runtimes", "battleye_runtime"),
		} {
			if environment[key] != want {
				t.Fatalf("%s = %q; want %q", key, environment[key], want)
			}
		}
		if environment["GAMEID"] != "" || environment["STORE"] != "" {
			t.Fatalf("direct Proton must not add UMU environment: %v", environment)
		}
	}
}

func TestHeroicProtonVersionSnapshot(t *testing.T) {
	for _, perGame := range []bool{false, true} {
		name := "global default"
		if perGame {
			name = "per-game override"
		}
		t.Run(name, func(t *testing.T) {
			f := newHeroicFixture(t)
			f.settings["wineVersion"] = map[string]string{"bin": f.wine, "type": "proton"}
			f.saveSettings(t)
			f.epic(t, map[string]interface{}{"game": map[string]string{"install_path": f.install}})
			configured := f.wine
			if perGame {
				configured = filepath.Join(f.home, "GE-Proton game version", "proton")
				heroicExecutableFixture(t, configured)
				writeHeroicFixture(t, filepath.Join(f.root, "GamesConfig", "game.json"), map[string]interface{}{
					"game": map[string]interface{}{"wineVersion": map[string]string{"bin": configured, "type": "proton"}},
				})
			}
			apps, err := discoverHeroicGames([]string{f.root}, f.home)
			if err != nil || len(apps) != 1 {
				t.Fatalf("discovery = %v, %v", apps, err)
			}
			saved, err := appFromConfig([]string{apps[0].Name, apps[0].Path, apps[0].Args, apps[0].WorkingDirectory})
			if err != nil {
				t.Fatal(err)
			}
			assertProton := func(app App, want string) {
				t.Helper()
				environment, args := heroicCommandFixture(t, app, f.runner)
				for i, arg := range args {
					if arg == "--wrapper" && i+1 < len(args) {
						wrapper, err := parseHeroicArguments(args[i+1])
						if err != nil || !reflect.DeepEqual(wrapper, []string{want, "waitforexitandrun"}) || environment["PROTONPATH"] != filepath.Dir(want) {
							t.Fatalf("configured Proton mismatch: wrapper = %q, env = %v, error = %v", wrapper, environment, err)
						}
						return
					}
				}
				t.Fatal("missing Proton wrapper")
			}
			assertProton(saved, configured)
			next := filepath.Join(f.home, "GE-Proton new version", "proton")
			heroicExecutableFixture(t, next)
			if perGame {
				writeHeroicFixture(t, filepath.Join(f.root, "GamesConfig", "game.json"), map[string]interface{}{
					"game": map[string]interface{}{"wineVersion": map[string]string{"bin": next, "type": "proton"}},
				})
			} else {
				f.settings["wineVersion"] = map[string]string{"bin": next, "type": "proton"}
				f.saveSettings(t)
			}
			assertProton(saved, configured)
			reimported, err := discoverHeroicGames([]string{f.root}, f.home)
			if err != nil || len(reimported) != 1 {
				t.Fatalf("reimport = %v, %v", reimported, err)
			}
			assertProton(reimported[0], next)
		})
	}
}

func TestDiscoverHeroicGOGNativeAndNile(t *testing.T) {
	f := newHeroicFixture(t)
	f.saveSettings(t)
	// GOG's installed entries use appName, not id.
	writeHeroicFixture(t, filepath.Join(f.root, "gog_store", "installed.json"), map[string]interface{}{"installed": []interface{}{
		map[string]interface{}{"appName": "12345", "install_path": f.install, "platform": "linux"},
	}})
	writeHeroicFixture(t, filepath.Join(f.root, "store_cache", "gog_library.json"), map[string]interface{}{"games": []interface{}{
		map[string]string{"app_name": "12345", "title": "GOG Native"},
	}})
	writeHeroicFixture(t, filepath.Join(f.root, "nile_config", "nile", "installed.json"), []interface{}{
		map[string]interface{}{"id": "nile-id", "path": f.install},
	})
	writeHeroicFixture(t, filepath.Join(f.root, "nile_config", "nile", "library.json"), []interface{}{
		map[string]interface{}{"product": map[string]string{"id": "nile-id", "title": "Nile Game"}},
	})
	apps, err := discoverHeroicGames([]string{f.root}, f.home)
	if err != nil || len(apps) != 2 {
		t.Fatalf("discovery = %+v, %v", apps, err)
	}
	environment, args := heroicCommandFixture(t, apps[0], f.runner)
	wantArgs := []string{"--auth-config-path", filepath.Join(f.root, "gog_store", "auth.json"), "launch", f.install, "12345", "--platform", "linux"}
	if apps[0].Name != "GOG Native [Heroic gog]" || !reflect.DeepEqual(args, wantArgs) || !reflect.DeepEqual(environment, map[string]string{"GOGDL_CONFIG_PATH": filepath.Join(f.root, "gogdlConfig")}) {
		t.Fatalf("GOG launch = %+v, %v, %q", apps[0], environment, args)
	}
	environment, args = heroicCommandFixture(t, apps[1], f.runner)
	prefix := filepath.Join(f.home, "Games", "Heroic", "Prefixes", "shared")
	wantArgs = []string{"launch", "nile-id", "--wine", f.wine, "--wine-prefix", prefix}
	if apps[1].Name != "Nile Game [Heroic nile]" || !reflect.DeepEqual(args, wantArgs) || !reflect.DeepEqual(environment, map[string]string{"NILE_CONFIG_PATH": filepath.Join(f.root, "nile_config"), "WINEPREFIX": prefix}) {
		t.Fatalf("Nile launch = %+v, %v, %q", apps[1], environment, args)
	}
}

func TestDiscoverHeroicConfigurationRoots(t *testing.T) {
	for _, location := range []string{"override", "xdg", "home", "flatpak"} {
		t.Run(location, func(t *testing.T) {
			f := newHeroicFixture(t)
			t.Setenv("HOME", f.home)
			t.Setenv("HEROIC_DIR", "")
			t.Setenv("XDG_CONFIG_HOME", "")
			switch location {
			case "override":
				t.Setenv("HEROIC_DIR", f.root)
			case "xdg":
				config := filepath.Join(f.home, "xdg configuration")
				t.Setenv("XDG_CONFIG_HOME", config)
				f.root = filepath.Join(config, "heroic")
			case "home":
				f.root = filepath.Join(f.home, ".config", "heroic")
			case "flatpak":
				f.root = filepath.Join(f.home, ".var", "app", "com.heroicgameslauncher.hgl", "config", "heroic")
			}
			f.saveSettings(t)
			writeHeroicFixture(t, filepath.Join(f.root, "nile_config", "nile", "installed.json"), []interface{}{
				map[string]interface{}{"id": "fallback-id", "path": f.install},
			})
			apps, err := DiscoverHeroicGames()
			if err != nil || len(apps) != 1 || apps[0].Name != "fallback-id [Heroic nile]" {
				t.Fatalf("%s discovery = %+v, %v", location, apps, err)
			}
			environment, _ := heroicCommandFixture(t, apps[0], f.runner)
			if environment["NILE_CONFIG_PATH"] != filepath.Join(f.root, "nile_config") {
				t.Fatalf("wrong root: %v", environment)
			}
		})
	}
}

func TestDiscoverHeroicMalformedStoreDoesNotHideOtherStores(t *testing.T) {
	f := newHeroicFixture(t)
	f.saveSettings(t)
	f.epic(t, map[string]interface{}{"epic-id": map[string]interface{}{"title": "Healthy", "install_path": f.install}})
	path := filepath.Join(f.root, "gog_store", "installed.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(path, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	apps, err := discoverHeroicGames([]string{f.root}, f.home)
	if err != nil || len(apps) != 1 || apps[0].Name != "Healthy [Heroic legendary]" {
		t.Fatalf("partial discovery = %+v, %v", apps, err)
	}
}

func TestHeroicOverridesQuotingAndPersistence(t *testing.T) {
	f := newHeroicFixture(t)
	f.settings["winePrefix"] = filepath.Join(f.home, "global prefix")
	f.settings["language"] = "en"
	f.settings["launcherArgs"] = "--global"
	f.saveSettings(t)
	f.epic(t, map[string]interface{}{"override-id": map[string]interface{}{"title": "Override Game", "install_path": f.install}})
	value := `spaces, apostrophe's, and C:\Game\save files`
	wrapper := filepath.Join(f.home, `Wrapper's directory\bin`, "wrapper")
	target := `C:\Game's directory\game.exe`
	writeHeroicFixture(t, filepath.Join(f.root, "GamesConfig", "override-id.json"), map[string]interface{}{"override-id": map[string]interface{}{
		"winePrefix": `~/Prefix's directory\wine`, "language": "fr", "offlineMode": true,
		"targetExe": target, "launcherArgs": `--save "C:\My Saves\slot" "argument's value" "" "a\\b"`,
		"enviromentOptions": []interface{}{map[string]string{"key": "TEST_VALUE", "value": value}, map[string]string{"key": "QUOTED_VALUE", "value": `"quoted value"`}},
		"wrapperOptions":    []interface{}{map[string]string{"exe": wrapper, "args": `--label "wrapper's value" "C:\path with spaces" "wrapper\\path"`}},
	}})
	appsFound, err := discoverHeroicGames([]string{f.root}, f.home)
	if err != nil || len(appsFound) != 1 {
		t.Fatalf("discovery = %+v, %v", appsFound, err)
	}
	app := appsFound[0]
	environment, args := heroicCommandFixture(t, app, f.runner)
	prefix := filepath.Join(f.home, `Prefix's directory\wine`)
	if !reflect.DeepEqual(environment, map[string]string{"LEGENDARY_CONFIG_PATH": filepath.Join(f.root, "legendaryConfig", "legendary"), "WINEPREFIX": prefix, "TEST_VALUE": value, "QUOTED_VALUE": "quoted value"}) {
		t.Fatalf("environment = %v", environment)
	}
	if len(args) != 16 {
		t.Fatalf("unexpected arguments: %q", args)
	}
	wantArgs := []string{"launch", "override-id", "--offline", "--language", "fr", "--wine", f.wine, "--wrapper", args[8], "--override-exe", target, "--save", `C:\My Saves\slot`, "argument's value", "", `a\b`}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %q; want %q", args, wantArgs)
	}
	wrapped, err := parseArguments(args[8])
	if err != nil || !reflect.DeepEqual(wrapped, []string{wrapper, "--label", "wrapper's value", `C:\path with spaces`, `wrapper\path`}) {
		t.Fatalf("nested wrapper = %q, %v", wrapped, err)
	}
	// Execute only the fake standalone backend to check env and argv end to end.
	commandArgs, err := parseArguments(app.Args)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(app.Path, commandArgs...)
	command.Dir = app.WorkingDirectory
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("fake backend failed: %v, %s", err, output)
	}
	for _, tc := range []struct {
		file string
		want []string
	}{
		{"heroic-argv", wantArgs},
		{"heroic-env", []string{prefix, environment["LEGENDARY_CONFIG_PATH"], value, f.install}},
	} {
		data, err := ioutil.ReadFile(filepath.Join(f.install, tc.file))
		if err != nil || string(data) != strings.Join(tc.want, "\x00")+"\x00" {
			t.Fatalf("%s = %q, %v; want %q", tc.file, data, err, tc.want)
		}
	}
	persisted, err := appFromConfig(strings.Split(app.Name+"\n"+app.Path+"\n"+app.Args+"\n"+app.WorkingDirectory, "\n"))
	if err != nil || persisted != app {
		t.Fatalf("four-field round trip = %+v, %v; want %+v", persisted, err, app)
	}
}

func TestDiscoverHeroicSkipsAndDeduplicates(t *testing.T) {
	f := newHeroicFixture(t)
	f.saveSettings(t)
	f.epic(t, map[string]interface{}{
		"zulu":     map[string]interface{}{"title": "Zulu", "install_path": f.install},
		"alpha":    map[string]interface{}{"title": "alpha", "install_path": f.install},
		"dlc":      map[string]interface{}{"title": "DLC", "install_path": f.install, "is_dlc": true},
		"missing":  map[string]interface{}{"title": "Missing", "install_path": filepath.Join(f.install, "missing")},
		"relative": map[string]interface{}{"title": "Relative", "install_path": "relative"},
	})
	apps, err := discoverHeroicGames([]string{filepath.Join(f.root, "missing root"), f.root, f.root}, f.home)
	if err != nil || len(apps) != 2 || apps[0].Name != "alpha [Heroic legendary]" || apps[1].Name != "Zulu [Heroic legendary]" {
		t.Fatalf("filtered, sorted and deduplicated apps = %+v, %v", apps, err)
	}
}

func TestDiscoverHeroicMalformedAndMissingRunner(t *testing.T) {
	for _, tc := range []struct{ name, path, data, diagnostic string }{
		{"global JSON", "config.json", "{", "cannot parse"},
		{"default settings", "config.json", `{"defaultSettings": []}`, "invalid defaultSettings"},
		{"installed JSON", "legendaryConfig/legendary/installed.json", "{", "cannot parse"},
		{"game JSON", "GamesConfig/epic-id.json", "{", "cannot parse"},
		{"game settings", "GamesConfig/epic-id.json", `{"epic-id":{"winePrefix":42}}`, "winePrefix"},
		{"missing backend", "", "", "missing backend"},
		{"missing wine", "", "", "Wine/Proton executable unavailable"},
		{"missing installs", "", "", "installation directory unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newHeroicFixture(t)
			if tc.name == "missing backend" {
				f.settings["altLegendaryBin"] = filepath.Join(f.bin, "missing backend")
			}
			if tc.name == "missing wine" {
				f.settings["wineVersion"] = map[string]string{"bin": filepath.Join(f.bin, "missing wine"), "type": "wine"}
			}
			install := f.install
			if tc.name == "missing installs" {
				install = filepath.Join(f.install, "missing")
			}
			f.saveSettings(t)
			f.epic(t, map[string]interface{}{"epic-id": map[string]interface{}{"install_path": install}})
			if tc.path != "" {
				path := filepath.Join(f.root, filepath.FromSlash(tc.path))
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := ioutil.WriteFile(path, []byte(tc.data), 0600); err != nil {
					t.Fatal(err)
				}
			}
			apps, err := discoverHeroicGames([]string{f.root}, f.home)
			if err == nil || len(apps) != 0 || !strings.Contains(err.Error(), tc.diagnostic) {
				t.Fatalf("discovery = %+v, %v; expected %q diagnostic", apps, err, tc.diagnostic)
			}
		})
	}
}

func TestQuoteHeroicArguments(t *testing.T) {
	want := []string{"", "plain", "two words", "apostrophe's", `C:\path with spaces\file`, `both 'single' and "double" quotes`, `trailing\`, "$HOME; $(false)"}
	got, err := parseArguments(quoteHeroicArguments(want))
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("quoted argument round trip = %q, %v; want %q", got, err, want)
	}
}

func TestHeroicPackagedBackendLookup(t *testing.T) {
	f := newHeroicFixture(t)
	for _, layout := range []string{
		"usr/lib/heroic-games-launcher/app.asar.unpacked",
		"opt/Heroic/resources/app.asar.unpacked",
	} {
		t.Run(layout, func(t *testing.T) {
			root := filepath.Join(f.home, layout)
			for _, name := range []string{"legendary", "gogdl", "nile"} {
				want := filepath.Join(root, "build", "bin", "x64", "linux", name)
				heroicExecutableFixture(t, want)
				got, err := findHeroicBinary(name, "", []string{root})
				if err != nil || got != want {
					t.Fatalf("%s lookup = %q, %v; want %q", name, got, err, want)
				}
			}
		})
	}
}
