package executor

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// SteamGame is an installed game discovered from Steam's local manifests.
type SteamGame struct {
	Name       string
	AppID      string
	InstallDir string
}

var steamPairs = regexp.MustCompile(`"((?:\\.|[^"\\])*)"\s*"((?:\\.|[^"\\])*)"`)

func steamValues(data []byte) map[string]string {
	values := make(map[string]string)
	for _, pair := range steamPairs.FindAllSubmatch(data, -1) {
		value := strings.ReplaceAll(string(pair[2]), `\\`, `\`)
		values[string(pair[1])] = strings.ReplaceAll(value, `\"`, `"`)
	}
	return values
}

func steamRoots(home string) []string {
	flatpakRoot := filepath.Join(home, ".var", "app", "com.valvesoftware.Steam")
	roots := []string{
		filepath.Join(home, ".steam", "steam"),
		filepath.Join(home, ".steam", "root"),
		filepath.Join(home, ".local", "share", "Steam"),
		filepath.Join(flatpakRoot, ".local", "share", "Steam"),
	}
	if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
		roots = append(roots, filepath.Join(dataHome, "Steam"))
	}
	if runtime.GOOS == "darwin" {
		roots = append(roots, filepath.Join(home, "Library", "Application Support", "Steam"))
	}
	if runtime.GOOS == "windows" {
		for _, key := range []string{"ProgramFiles(x86)", "ProgramFiles"} {
			if base := os.Getenv(key); base != "" {
				roots = append(roots, filepath.Join(base, "Steam"))
			}
		}
	}
	if root := os.Getenv("STEAM_DIR"); root != "" {
		roots = append([]string{root}, roots...)
	}
	return roots
}

// DiscoverSteamGames includes libraries listed in libraryfolders.vdf.
func DiscoverSteamGames() ([]SteamGame, App, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, App{}, err
	}
	roots := steamRoots(home)
	flatpakRoot := filepath.Join(home, ".var", "app", "com.valvesoftware.Steam")
	client, _ := exec.LookPath("steam")
	if client == "" {
		for _, root := range roots {
			if root == filepath.Join(flatpakRoot, ".local", "share", "Steam") {
				continue
			}
			for _, name := range []string{"steam.exe", "steam.sh"} {
				candidate := filepath.Join(root, name)
				if executable, err := exec.LookPath(candidate); err == nil {
					client = executable
					break
				}
			}
			if client != "" {
				break
			}
		}
	}
	launcher := App{Path: client}
	if client == "" {
		if info, err := os.Stat(flatpakRoot); err == nil && info.IsDir() {
			if flatpak, err := exec.LookPath("flatpak"); err == nil {
				launcher = App{Path: flatpak, Args: "run com.valvesoftware.Steam"}
			}
		}
	}
	if launcher.Path == "" {
		return nil, App{}, fmt.Errorf("cannot find the Steam executable; install Steam or add it to PATH")
	}
	games, err := discoverSteamLibraries(roots)
	return games, launcher, err
}

func discoverSteamLibraries(roots []string) ([]SteamGame, error) {
	libraries := append([]string(nil), roots...)
	for _, root := range roots {
		for _, relative := range []string{"steamapps/libraryfolders.vdf", "config/libraryfolders.vdf"} {
			data, err := ioutil.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
			if err != nil {
				continue
			}
			for _, pair := range steamPairs.FindAllSubmatch(data, -1) {
				key := string(pair[1])
				value := strings.ReplaceAll(string(pair[2]), `\\`, `\`)
				_, numeric := strconv.Atoi(key)
				if key == "path" || (numeric == nil && filepath.IsAbs(value)) {
					libraries = append(libraries, value)
				}
			}
		}
	}
	seenLibraries := make(map[string]bool)
	seenGames := make(map[string]bool)
	var games []SteamGame
	for _, library := range libraries {
		canonical, err := filepath.EvalSymlinks(library)
		if err != nil || seenLibraries[canonical] {
			continue
		}
		seenLibraries[canonical] = true
		manifests, err := filepath.Glob(filepath.Join(canonical, "steamapps", "appmanifest_*.acf"))
		if err != nil {
			return nil, err
		}
		for _, manifest := range manifests {
			data, err := ioutil.ReadFile(manifest)
			if err != nil {
				return nil, fmt.Errorf("cannot read Steam manifest %q: %w", manifest, err)
			}
			values := steamValues(data)
			id := values["appid"]
			if _, err := strconv.ParseUint(id, 10, 32); err != nil || id == "0" || values["name"] == "" || values["installdir"] == "" || seenGames[id] {
				continue
			}
			installDir := filepath.Join(canonical, "steamapps", "common", values["installdir"])
			if info, err := os.Stat(installDir); err != nil || !info.IsDir() {
				continue
			}
			seenGames[id] = true
			games = append(games, SteamGame{Name: values["name"], AppID: id, InstallDir: installDir})
		}
	}
	sort.Slice(games, func(i, j int) bool {
		return strings.ToLower(games[i].Name) < strings.ToLower(games[j].Name)
	})
	if len(games) == 0 {
		return nil, fmt.Errorf("no installed Steam games found; set STEAM_DIR to your Steam installation if it is in a custom location")
	}
	return games, nil
}
