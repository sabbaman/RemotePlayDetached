package executor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var threaded bool = false
var exitAfterExec = false

// Threaded func
func Threaded(b bool) {
	threaded = b
}

// SetExitAfterExec func
func SetExitAfterExec(b bool) {
	exitAfterExec = b
}

// IsThreaded func
func IsThreaded() bool {
	return threaded
}

func executeApp(app App) (resultErr error) {
	if err := os.Chdir(rpdPath); err != nil {
		return fmt.Errorf("cannot access RPD directory %q: %w", rpdPath, err)
	}
	defer func() {
		if err := os.Chdir(rpdPath); err != nil {
			if resultErr != nil {
				resultErr = fmt.Errorf("%v; cannot restore RPD directory %q: %w", resultErr, rpdPath, err)
			} else {
				resultErr = fmt.Errorf("process started, but cannot restore RPD directory %q: %w", rpdPath, err)
			}
		}
	}()

	executable, workingDirectory, err := executionPaths(app)
	if err != nil {
		return err
	}
	if err := os.Chdir(workingDirectory); err != nil {
		return fmt.Errorf("cannot access working directory %q: %w", workingDirectory, err)
	}
	var argsArray []string

	if app.Args != "" {
		argsArray = strings.Split(app.Args, " ")
	}

	fmt.Println("path for execution:", workingDirectory, "; executable to run:", executable)
	cmd := exec.Command(executable, argsArray...)
	err = cmd.Start()
	if err != nil {
		fmt.Println(err)
		return err
	}
	go cmd.Wait()
	return nil

}

func executionPaths(app App) (string, string, error) {
	if app.Path == "" {
		return "", "", fmt.Errorf("executable path cannot be empty")
	}
	path := app.Path
	if filepath.Base(path) == path {
		var err error
		path, err = exec.LookPath(path)
		if err != nil {
			return "", "", fmt.Errorf("cannot find executable %q: %w", app.Path, err)
		}
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(rpdPath, path)
	}
	path = filepath.Clean(path)
	directory := app.WorkingDirectory
	if directory == "" {
		directory = filepath.Dir(path)
	} else if !filepath.IsAbs(directory) {
		directory = filepath.Join(rpdPath, directory)
	}
	return path, filepath.Clean(directory), nil
}
