package executor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"
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
	argsArray, err := parseArguments(app.Args)
	if err != nil {
		return fmt.Errorf("cannot parse launch arguments: %w", err)
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

// parseArguments tokenizes direct-exec arguments without shell expansion.
func parseArguments(input string) ([]string, error) {
	var args []string
	var arg strings.Builder
	var quote rune
	started := false
	quotePosition := 0
	chars := []rune(input)
	for i := 0; i < len(chars); i++ {
		c := chars[i]
		// Only escape syntax characters; ordinary Windows path separators stay literal.
		if c == '\\' && quote != '\'' && i+1 < len(chars) {
			next := chars[i+1]
			if (quote == '"' && next == '"') || (quote == 0 && (next == '\'' || next == '"' || unicode.IsSpace(next))) {
				arg.WriteRune(next)
				started = true
				i++
				continue
			}
		}
		if quote != 0 {
			if c == quote {
				quote = 0
			} else {
				arg.WriteRune(c)
			}
		} else if c == '\'' || c == '"' {
			quote = c
			quotePosition = i + 1
			started = true
		} else if unicode.IsSpace(c) {
			if started {
				args = append(args, arg.String())
				arg.Reset()
				started = false
			}
		} else {
			arg.WriteRune(c)
			started = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unmatched %c quote at character %d", quote, quotePosition)
	}
	if started {
		args = append(args, arg.String())
	}
	return args, nil
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
