package executor

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func testDirectory(t *testing.T) (string, func()) {
	t.Helper()
	dir, err := ioutil.TempDir("", "rpd-test-")
	if err != nil {
		t.Fatal(err)
	}
	oldPath := rpdPath
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	rpdPath = dir
	return dir, func() {
		os.Chdir(oldWD)
		rpdPath = oldPath
		os.RemoveAll(dir)
	}
}

func TestExecutionPaths(t *testing.T) {
	dir, cleanup := testDirectory(t)
	defer cleanup()
	for _, tc := range []struct {
		name, executable, working, wantWorking string
	}{
		{"default", filepath.Join("heroic", "heroic"), "", filepath.Join(dir, "heroic")},
		{"relative working directory", filepath.Join("bin", "heroic"), "games with spaces", filepath.Join(dir, "games with spaces")},
		{"absolute working directory", filepath.Join("bin", "heroic"), dir, dir},
	} {
		t.Run(tc.name, func(t *testing.T) {
			executable, working, err := executionPaths(App{Path: tc.executable, WorkingDirectory: tc.working})
			if err != nil || executable != filepath.Join(dir, tc.executable) || working != tc.wantWorking {
				t.Fatalf("paths = %q, %q, %v", executable, working, err)
			}
		})
	}
}

func TestExecuteAppWorkingDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell script")
	}
	dir, cleanup := testDirectory(t)
	defer cleanup()
	bin := filepath.Join(dir, "heroic with spaces")
	work := filepath.Join(dir, "games")
	for _, path := range []string{bin, work} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	executable := filepath.Join(bin, "heroic")
	if err := ioutil.WriteFile(executable, []byte("#!/bin/sh\npwd > launched\n"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, working := range []string{"", work} {
		if err := executeApp(App{Path: executable, WorkingDirectory: working}); err != nil {
			t.Fatal(err)
		}
		want := working
		if want == "" {
			want = bin
		}
		deadline := time.Now().Add(3 * time.Second)
		for {
			data, err := ioutil.ReadFile(filepath.Join(want, "launched"))
			if err == nil && strings.TrimSpace(string(data)) == want {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("child did not run from %q: %q, %v", want, data, err)
			}
			time.Sleep(10 * time.Millisecond)
		}
		assertRPDDirectory(t, dir)
	}
	// No launch must occur when the requested directory is invalid.
	marker := filepath.Join(bin, "launched")
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	err := executeApp(App{Path: executable, WorkingDirectory: filepath.Join(dir, "missing")})
	if err == nil || !strings.Contains(err.Error(), "cannot access working directory") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("process was launched despite invalid directory")
	}
	assertRPDDirectory(t, dir)
	if err := executeApp(App{Path: filepath.Join(bin, "missing")}); err == nil {
		t.Fatal("expected executable error")
	}
	assertRPDDirectory(t, dir)
	if os.Geteuid() != 0 {
		blocked := filepath.Join(dir, "blocked")
		if err := os.Mkdir(blocked, 0000); err != nil {
			t.Fatal(err)
		}
		err := executeApp(App{Path: executable, WorkingDirectory: blocked})
		os.Chmod(blocked, 0700)
		if err == nil || !strings.Contains(err.Error(), "cannot access working directory") {
			t.Fatalf("expected permission error: %v", err)
		}
		assertRPDDirectory(t, dir)
	}
}

func TestExecutableFromPATH(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX executable")
	}
	dir, cleanup := testDirectory(t)
	defer cleanup()
	path := filepath.Join(dir, "heroic")
	if err := ioutil.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	oldPATH := os.Getenv("PATH")
	defer os.Setenv("PATH", oldPATH)
	os.Setenv("PATH", dir)
	executable, working, err := executionPaths(App{Path: "heroic"})
	if err != nil || executable != path || working != dir {
		t.Fatalf("paths = %q, %q, %v", executable, working, err)
	}
}

func assertRPDDirectory(t *testing.T, want string) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil || wd != want {
		t.Fatalf("working directory = %q, %v; want %q", wd, err, want)
	}
}

func TestAppConfigCompatibility(t *testing.T) {
	for _, config := range [][]string{
		{"game", "/bin/game"},
		{"game", "/bin/game", "--launch"},
		{"game", "/bin/game", "", "/games"},
		{"game", "/bin/game", "--launch", "/games"},
	} {
		app, err := appFromConfig(config)
		if err != nil || app.Name != config[0] || app.Path != config[1] {
			t.Fatalf("config %v: %v, %v", config, app, err)
		}
		if len(config) >= 3 && app.Args != config[2] {
			t.Fatal("arguments not preserved")
		}
		if len(config) >= 4 && app.WorkingDirectory != config[3] {
			t.Fatal("working directory not preserved")
		}
	}
	if _, err := appFromConfig([]string{"incomplete"}); err == nil {
		t.Fatal("expected invalid configuration error")
	}
}

func TestWorkingDirectoryPersistence(t *testing.T) {
	dir, cleanup := testDirectory(t)
	defer cleanup()
	oldApps := apps
	defer func() { apps = oldApps }()
	if err := os.Mkdir(filepath.Join(dir, "importedApps"), 0700); err != nil {
		t.Fatal(err)
	}
	want := App{Name: "game", Path: "/bin/game", WorkingDirectory: "/games with spaces"}
	ImportApp(want)
	loaded := loadImportedApps()
	if len(loaded) != 1 || !loaded[0].Equals(want) {
		t.Fatalf("loaded %v; want %v", loaded, want)
	}
}
