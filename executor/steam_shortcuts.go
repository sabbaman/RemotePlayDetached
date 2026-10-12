package executor

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DiscoverSteamShortcuts reads non-Steam entries from all local Steam profiles.
func DiscoverSteamShortcuts() ([]App, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return discoverSteamShortcuts(steamRoots(home))
}

func discoverSteamShortcuts(roots []string) ([]App, error) {
	seenFiles := make(map[string]bool)
	seenApps := make(map[App]bool)
	var apps []App
	for _, root := range roots {
		files, err := filepath.Glob(filepath.Join(root, "userdata", "*", "config", "shortcuts.vdf"))
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			canonical, err := filepath.EvalSymlinks(file)
			if err != nil {
				return nil, fmt.Errorf("cannot access Steam shortcuts %q: %w", file, err)
			}
			if seenFiles[canonical] {
				continue
			}
			seenFiles[canonical] = true
			data, err := ioutil.ReadFile(canonical)
			if err != nil {
				return nil, fmt.Errorf("cannot read Steam shortcuts %q: %w", file, err)
			}
			shortcuts, err := parseSteamShortcuts(data)
			if err != nil {
				return nil, fmt.Errorf("cannot parse Steam shortcuts %q: %w", file, err)
			}
			for _, app := range shortcuts {
				if !seenApps[app] {
					seenApps[app] = true
					apps = append(apps, app)
				}
			}
		}
	}
	sort.Slice(apps, func(i, j int) bool {
		return strings.ToLower(apps[i].Name) < strings.ToLower(apps[j].Name)
	})
	if len(apps) == 0 {
		return nil, fmt.Errorf("no non-Steam shortcuts found; add a game to Steam and close Steam to save it, or set STEAM_DIR for a custom installation")
	}
	return apps, nil
}

type shortcutValue struct {
	text   string
	object map[string]shortcutValue
}

// shortcuts.vdf uses binary KeyValues, not the text format used by manifests.
func parseSteamShortcuts(data []byte) ([]App, error) {
	offset := 0
	readString := func() (string, error) {
		end := bytes.IndexByte(data[offset:], 0)
		if end < 0 {
			return "", fmt.Errorf("unterminated string at byte %d", offset)
		}
		value := string(data[offset : offset+end])
		offset += end + 1
		return value, nil
	}
	var readObject func(int) (map[string]shortcutValue, error)
	readObject = func(depth int) (map[string]shortcutValue, error) {
		if depth > 64 {
			return nil, fmt.Errorf("excessive object nesting")
		}
		values := make(map[string]shortcutValue)
		for offset < len(data) {
			kind := data[offset]
			offset++
			if kind == 8 {
				return values, nil
			}
			key, err := readString()
			if err != nil {
				return nil, err
			}
			var value shortcutValue
			switch kind {
			case 0:
				value.object, err = readObject(depth + 1)
			case 1:
				value.text, err = readString()
			case 2, 3, 4, 6, 7, 10:
				// Integer, float, pointer, color, uint64 and int64 metadata.
				size := 4
				if kind == 7 || kind == 10 {
					size = 8
				}
				if len(data)-offset < size {
					return nil, fmt.Errorf("truncated numeric value at byte %d", offset)
				}
				offset += size
			default:
				return nil, fmt.Errorf("unsupported value type %d at byte %d", kind, offset-1)
			}
			if err != nil {
				return nil, err
			}
			values[strings.ToLower(key)] = value
		}
		return nil, fmt.Errorf("unterminated object at byte %d", offset)
	}
	root, err := readObject(0)
	if err != nil {
		return nil, err
	}
	if offset != len(data) || root["shortcuts"].object == nil {
		return nil, fmt.Errorf("invalid shortcuts root or trailing data")
	}
	var apps []App
	for _, entry := range root["shortcuts"].object {
		values := entry.object
		app := App{
			Name:             values["appname"].text,
			Path:             unquoteShortcutPath(values["exe"].text),
			Args:             values["launchoptions"].text,
			WorkingDirectory: unquoteShortcutPath(values["startdir"].text),
		}
		if app.Name != "" && app.Path != "" {
			apps = append(apps, app)
		}
	}
	return apps, nil
}

func unquoteShortcutPath(path string) string {
	path = strings.TrimSpace(path)
	if len(path) >= 2 && path[0] == '"' && path[len(path)-1] == '"' {
		return path[1 : len(path)-1]
	}
	return path
}
