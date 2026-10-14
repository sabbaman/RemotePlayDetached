package executor

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// DiscoverHeroicGames imports installed games through Heroic's standalone backends.
func DiscoverHeroicGames() ([]App, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("direct Heroic import currently supports Linux only")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	config := os.Getenv("XDG_CONFIG_HOME")
	if config == "" {
		config = filepath.Join(home, ".config")
	}
	roots := []string{filepath.Join(config, "heroic"), filepath.Join(home, ".var", "app", "com.heroicgameslauncher.hgl", "config", "heroic")}
	if root := os.Getenv("HEROIC_DIR"); root != "" {
		roots = []string{root}
	}
	return discoverHeroicGames(roots, home)
}

type heroicGame struct {
	ID          string `json:"id"`
	AppName     string `json:"appName"`
	Title       string `json:"title"`
	InstallPath string `json:"install_path"`
	Path        string `json:"path"`
	Platform    string `json:"platform"`
	IsDLC       bool   `json:"is_dlc"`
}

type heroicSettings struct {
	WinePrefix  string `json:"winePrefix"`
	WineVersion struct {
		Bin  string `json:"bin"`
		Type string `json:"type"`
	} `json:"wineVersion"`
	LauncherArgs    string `json:"launcherArgs"`
	TargetExe       string `json:"targetExe"`
	Language        string `json:"language"`
	OfflineMode     bool   `json:"offlineMode"`
	EACRuntime      bool   `json:"eacRuntime"`
	BattlEyeRuntime bool   `json:"battlEyeRuntime"`
	Environment     []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"enviromentOptions"`
	Wrappers []struct {
		Exe  string `json:"exe"`
		Args string `json:"args"`
	} `json:"wrapperOptions"`
}

func readHeroicJSON(path string, target interface{}) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("cannot parse %q: %w", path, err)
	}
	return nil
}

func discoverHeroicGames(roots []string, home string) ([]App, error) {
	var apps []App
	var diagnostics []string
	seen := make(map[App]bool)
	for _, root := range roots {
		global := make(map[string]json.RawMessage)
		config := make(map[string]json.RawMessage)
		if err := readHeroicJSON(filepath.Join(root, "config.json"), &config); err != nil && !os.IsNotExist(err) {
			diagnostics = append(diagnostics, err.Error())
			continue
		}
		if raw := config["defaultSettings"]; raw != nil {
			if err := json.Unmarshal(raw, &global); err != nil {
				diagnostics = append(diagnostics, fmt.Sprintf("invalid defaultSettings in %s: %v", root, err))
				continue
			}
		}
		for _, store := range []string{"legendary", "gog", "nile"} {
			var games []heroicGame
			var err error
			switch store {
			case "legendary":
				installed := make(map[string]heroicGame)
				err = readHeroicJSON(filepath.Join(root, "legendaryConfig", "legendary", "installed.json"), &installed)
				for id, game := range installed {
					game.ID = id
					games = append(games, game)
				}
			case "gog":
				var installed struct {
					Installed []heroicGame `json:"installed"`
				}
				err = readHeroicJSON(filepath.Join(root, "gog_store", "installed.json"), &installed)
				games = installed.Installed
			case "nile":
				err = readHeroicJSON(filepath.Join(root, "nile_config", "nile", "installed.json"), &games)
			}
			if err != nil {
				if !os.IsNotExist(err) {
					diagnostics = append(diagnostics, err.Error())
				}
				continue
			}
			titles := make(map[string]string)
			if store == "gog" {
				var library struct {
					Games []struct {
						ID    string `json:"app_name"`
						Title string `json:"title"`
					} `json:"games"`
				}
				if err := readHeroicJSON(filepath.Join(root, "store_cache", "gog_library.json"), &library); err == nil {
					for _, entry := range library.Games {
						titles[entry.ID] = entry.Title
					}
				}
			}
			if store == "nile" {
				var library []struct {
					Product struct {
						ID    string `json:"id"`
						Title string `json:"title"`
					} `json:"product"`
				}
				if err := readHeroicJSON(filepath.Join(root, "nile_config", "nile", "library.json"), &library); err == nil {
					for _, entry := range library {
						titles[entry.Product.ID] = entry.Product.Title
					}
				}
			}
			for _, game := range games {
				if game.ID == "" {
					game.ID = game.AppName
				}
				if game.Title == "" {
					game.Title = titles[game.ID]
				}
				if game.InstallPath == "" {
					game.InstallPath = game.Path
				}
				if game.IsDLC {
					continue
				}
				app, err := heroicLaunchApp(root, home, store, game, global)
				if err != nil {
					diagnostics = append(diagnostics, fmt.Sprintf("%s/%s: %v", store, game.ID, err))
					continue
				}
				if !seen[app] {
					seen[app] = true
					apps = append(apps, app)
				}
			}
		}
	}
	for _, diagnostic := range diagnostics {
		fmt.Println("Heroic import:", diagnostic)
	}
	if len(apps) == 0 {
		message := "No directly launchable installed Heroic games found.\nCheck runner binaries and Wine/Proton settings,\nor set HEROIC_DIR to the Heroic configuration directory."
		if len(diagnostics) > 0 {
			message += "\n\nDetails:\n" + strings.Join(diagnostics, "\n\n")
		}
		return nil, fmt.Errorf("%s", message)
	}
	sort.Slice(apps, func(i, j int) bool { return strings.ToLower(apps[i].Name) < strings.ToLower(apps[j].Name) })
	return apps, nil
}

func heroicLaunchApp(root, home, store string, game heroicGame, global map[string]json.RawMessage) (App, error) {
	if game.ID == "" || filepath.Base(game.ID) != game.ID || game.ID == "." || game.ID == ".." {
		return App{}, fmt.Errorf("invalid game ID")
	}
	if !filepath.IsAbs(game.InstallPath) {
		return App{}, fmt.Errorf("missing absolute installation path")
	}
	if info, err := os.Stat(game.InstallPath); err != nil || !info.IsDir() {
		return App{}, fmt.Errorf("installation directory unavailable: %s", game.InstallPath)
	}
	if game.Platform != "" && !strings.EqualFold(game.Platform, "windows") && !strings.EqualFold(game.Platform, "linux") {
		return App{}, fmt.Errorf("unsupported installation platform: %s", game.Platform)
	}
	merged := make(map[string]json.RawMessage)
	for key, value := range global {
		merged[key] = value
	}
	// Heroic's UI language is not a default game language.
	delete(merged, "language")
	local := make(map[string]json.RawMessage)
	if err := readHeroicJSON(filepath.Join(root, "GamesConfig", game.ID+".json"), &local); err != nil && !os.IsNotExist(err) {
		return App{}, err
	}
	if raw := local[game.ID]; raw != nil {
		var overrides map[string]json.RawMessage
		if err := json.Unmarshal(raw, &overrides); err != nil {
			return App{}, err
		}
		for key, value := range overrides {
			merged[key] = value
		}
	}
	data, err := json.Marshal(merged)
	if err != nil {
		return App{}, err
	}
	var settings heroicSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return App{}, err
	}
	if store == "legendary" && settings.Language == "" {
		var config struct {
			Language string `json:"language"`
		}
		if err := readHeroicJSON(filepath.Join(root, "store", "config.json"), &config); err != nil && !os.IsNotExist(err) {
			return App{}, err
		}
		settings.Language = config.Language
	}
	binName, altKey := store, "altLegendaryBin"
	if store == "gog" {
		binName, altKey = "gogdl", "altGogdlBin"
	}
	if store == "nile" {
		altKey = "altNileBin"
	}
	var configured string
	if raw := global[altKey]; raw != nil {
		if err := json.Unmarshal(raw, &configured); err != nil {
			return App{}, err
		}
	}
	backend, err := heroicBinary(binName, configured)
	if err != nil {
		return App{}, err
	}
	environment := make(map[string]string)
	args := []string{"launch", game.ID}
	switch store {
	case "legendary":
		environment["LEGENDARY_CONFIG_PATH"] = filepath.Join(root, "legendaryConfig", "legendary")
		if settings.OfflineMode {
			args = append(args, "--offline")
		}
		if settings.Language != "" {
			args = append(args, "--language", settings.Language)
		}
	case "gog":
		environment["GOGDL_CONFIG_PATH"] = filepath.Join(root, "gogdlConfig")
		if game.Platform == "" {
			return App{}, fmt.Errorf("missing GOG installation platform")
		}
		args = []string{"--auth-config-path", filepath.Join(root, "gog_store", "auth.json"), "launch", game.InstallPath, game.ID, "--platform", strings.ToLower(game.Platform)}
	case "nile":
		environment["NILE_CONFIG_PATH"] = filepath.Join(root, "nile_config")
	}
	var wrappers []string
	for _, wrapper := range settings.Wrappers {
		if wrapper.Exe == "" {
			return App{}, fmt.Errorf("empty wrapper executable")
		}
		parsed, err := parseHeroicArguments(wrapper.Args)
		if err != nil {
			return App{}, err
		}
		wrappers = append(wrappers, wrapper.Exe)
		wrappers = append(wrappers, parsed...)
	}
	native := strings.EqualFold(game.Platform, "linux")
	if !native {
		prefix := settings.WinePrefix
		if prefix == "~" {
			prefix = home
		} else if strings.HasPrefix(prefix, "~/") {
			prefix = filepath.Join(home, strings.TrimPrefix(prefix, "~/"))
		}
		if prefix == "" {
			prefix = filepath.Join(home, "Games", "Heroic", "Prefixes", "shared")
		}
		if !filepath.IsAbs(prefix) {
			return App{}, fmt.Errorf("Wine prefix must be absolute")
		}
		wine := strings.Trim(settings.WineVersion.Bin, "'")
		if _, err := exec.LookPath(wine); err != nil {
			return App{}, fmt.Errorf("configured Wine/Proton executable unavailable: %s", wine)
		}
		environment["WINEPREFIX"] = prefix
		switch settings.WineVersion.Type {
		case "wine":
			args = append(args, "--wine", wine)
		case "proton":
			environment["STEAM_COMPAT_DATA_PATH"] = prefix
			environment["STEAM_COMPAT_CLIENT_INSTALL_PATH"] = filepath.Join(home, ".steam", "steam")
			for _, steamRoot := range steamRoots(home) {
				if info, err := os.Stat(steamRoot); err == nil && info.IsDir() {
					environment["STEAM_COMPAT_CLIENT_INSTALL_PATH"] = steamRoot
					break
				}
			}
			environment["STEAM_COMPAT_INSTALL_PATH"] = game.InstallPath
			environment["PROTONPATH"] = filepath.Dir(wine)
			args = append(args, "--no-wine")
			// Use the configured Proton directly; UMU can replace the donor's Steam identity.
			wrappers = append(wrappers, wine, "waitforexitandrun")
		default:
			return App{}, fmt.Errorf("unsupported compatibility layer: %s", settings.WineVersion.Type)
		}
		if store == "nile" {
			args = append(args, "--wine-prefix", prefix)
		}
		if settings.EACRuntime {
			environment["PROTON_EAC_RUNTIME"] = filepath.Join(root, "tools", "runtimes", "eac_runtime")
		}
		if settings.BattlEyeRuntime {
			environment["PROTON_BATTLEYE_RUNTIME"] = filepath.Join(root, "tools", "runtimes", "battleye_runtime")
		}
		for _, option := range []struct{ Key, Wine, Proton string }{
			{"enableEsync", "WINEESYNC", "PROTON_NO_ESYNC"},
			{"enableFsync", "WINEFSYNC", "PROTON_NO_FSYNC"},
		} {
			if raw := merged[option.Key]; raw != nil {
				var enabled bool
				if err := json.Unmarshal(raw, &enabled); err != nil {
					return App{}, err
				}
				if settings.WineVersion.Type == "proton" {
					if !enabled {
						environment[option.Proton] = "1"
					}
				} else {
					environment[option.Wine] = "0"
					if enabled {
						environment[option.Wine] = "1"
					}
				}
			}
		}
	}
	if len(wrappers) > 0 {
		args = append(args, "--wrapper", quoteHeroicArguments(wrappers))
	}
	if settings.TargetExe != "" {
		args = append(args, "--override-exe", settings.TargetExe)
	}
	extra, err := parseHeroicArguments(settings.LauncherArgs)
	if err != nil {
		return App{}, err
	}
	args = append(args, extra...)
	for _, entry := range settings.Environment {
		if entry.Key == "" || strings.ContainsAny(entry.Key, "=\x00") {
			return App{}, fmt.Errorf("invalid environment variable name")
		}
		value := entry.Value
		if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
			value = strings.Trim(value, "\"")
		}
		environment[entry.Key] = value
	}
	keys := make([]string, 0, len(environment))
	for key := range environment {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	command := []string{"--"}
	for _, key := range keys {
		command = append(command, key+"="+environment[key])
	}
	command = append(command, backend)
	command = append(command, args...)
	// env keeps the existing four-field persistence format and inherits Steam's environment.
	env, err := exec.LookPath("env")
	if err != nil {
		return App{}, err
	}
	if game.Title == "" {
		game.Title = game.ID
	}
	app := App{Name: game.Title + " [Heroic " + store + "]", Path: env, Args: quoteHeroicArguments(command), WorkingDirectory: game.InstallPath}
	if strings.ContainsAny(app.Name+app.Args+app.WorkingDirectory, "\r\n\x00") {
		return App{}, fmt.Errorf("launch configuration contains unsupported control characters")
	}
	return app, nil
}

func heroicBinary(name, configured string) (string, error) {
	return findHeroicBinary(name, configured, []string{
		"/opt/Heroic/resources/app.asar.unpacked",
		"/opt/heroic/resources/app.asar.unpacked",
		"/usr/lib/heroic/resources/app.asar.unpacked",
		"/usr/lib64/heroic/resources/app.asar.unpacked",
		"/usr/share/heroic/resources/app.asar.unpacked",
		"/usr/lib/heroic-games-launcher/app.asar.unpacked",
	})
}

func findHeroicBinary(name, configured string, roots []string) (string, error) {
	if configured != "" {
		return exec.LookPath(unquoteShortcutPath(configured))
	}
	if bin, err := exec.LookPath(name); err == nil {
		return bin, nil
	}
	for _, root := range roots {
		for _, arch := range []string{runtime.GOARCH, "x64"} {
			for _, directory := range []string{"build", "public"} {
				candidate := filepath.Join(root, directory, "bin", arch, "linux", name)
				if bin, err := exec.LookPath(candidate); err == nil {
					return bin, nil
				}
			}
		}
	}
	return "", fmt.Errorf("%s executable not found; install the standalone backend or configure its alternate binary in Heroic", name)
}

// Single quotes preserve backslashes for both RPD's tokenizer and backend shlex.
func quoteHeroicArguments(args []string) string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
	}
	return strings.Join(quoted, " ")
}
