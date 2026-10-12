package executor

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestParseArguments(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		want        []string
	}{
		{"empty", "", nil},
		{"whitespace only", " \t\r\n", nil},
		{"whitespace separators", "  one\ttwo\nthree\r\nfour  ", []string{"one", "two", "three", "four"}},
		{"unicode whitespace", "one\u2003two", []string{"one", "two"}},
		{"heroic", `--no-gui --no-sandbox "heroic://launch?appName=id&runner=legendary"`, []string{"--no-gui", "--no-sandbox", "heroic://launch?appName=id&runner=legendary"}},
		{"quoted spaces", `"double quoted" 'single quoted'`, []string{"double quoted", "single quoted"}},
		{"empty quotes", `"" '' a ""`, []string{"", "", "a", ""}},
		{"adjacent segments", `prefix"two words"'suffix' a""b`, []string{"prefixtwo wordssuffix", "ab"}},
		{"other quote literal", `"it's fine" 'say "hello"'`, []string{"it's fine", `say "hello"`}},
		{"escaped quotes", `say\"hello\" it\'s "say \"hello\""`, []string{`say"hello"`, "it's", `say "hello"`}},
		{"escaped whitespace", "two\\ words tab\\\tvalue", []string{"two words", "tab\tvalue"}},
		{"single quoted backslashes", `'C:\Games\game' 'literal\"quote'`, []string{`C:\Games\game`, `literal\"quote`}},
		{"windows paths", `C:\Games\game.exe "C:\Program Files\game.exe"`, []string{`C:\Games\game.exe`, `C:\Program Files\game.exe`}},
		{"UNC paths", `\\server\share\game "\\server\shared games\game"`, []string{`\\server\share\game`, `\\server\shared games\game`}},
		{"trailing backslash", `C:\Games\`, []string{`C:\Games\`}},
		{"ordinary escapes literal", `\n \t "a\ b"`, []string{`\n`, `\t`, `a\ b`}},
		{"no shell expansion", `$HOME ${USER} %PATH% ; & | > < $(whoami) *.exe ~`, []string{"$HOME", "${USER}", "%PATH%", ";", "&", "|", ">", "<", "$(whoami)", "*.exe", "~"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseArguments(tc.input)
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseArguments(%q) = %#v, %v; want %#v", tc.input, got, err, tc.want)
			}
		})
	}
}

func TestParseArgumentsUnmatchedQuotes(t *testing.T) {
	for _, tc := range []struct {
		input, want string
	}{
		{`one "two`, `unmatched " quote at character 5`},
		{`'two`, `unmatched ' quote at character 1`},
		{`"escaped\"`, `unmatched " quote at character 1`},
		{`ok "" 'bad`, `unmatched ' quote at character 7`},
	} {
		args, err := parseArguments(tc.input)
		if args != nil || err == nil || err.Error() != tc.want {
			t.Errorf("parseArguments(%q) = %#v, %v; want %q", tc.input, args, err, tc.want)
		}
	}
}

func TestExecuteAppRejectsUnmatchedQuotes(t *testing.T) {
	dir, cleanup := testDirectory(t)
	defer cleanup()
	err := executeApp(App{Path: os.Args[0], WorkingDirectory: dir, Args: `"unterminated`})
	if err == nil || !strings.Contains(err.Error(), "cannot parse launch arguments: unmatched") {
		t.Fatalf("expected argument parsing error, got %v", err)
	}
	assertRPDDirectory(t, dir)
}
