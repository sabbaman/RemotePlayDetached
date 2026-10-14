package executor

import (
	"reflect"
	"testing"
)

func TestParseHeroicArguments(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		want        []string
	}{
		{"empty", "", nil},
		{"whitespace", " \t\r\n", nil},
		{"separators", "one\ttwo\rthree\nfour", []string{"one", "two", "three", "four"}},
		{"unicode whitespace literal", "one\u2003two", []string{"one\u2003two"}},
		{"quoted spaces", `"two words" 'more words'`, []string{"two words", "more words"}},
		{"empty quotes", `"" '' a""b`, []string{"", "", "ab"}},
		{"adjacent segments", `pre"two words"'post'`, []string{"pretwo wordspost"}},
		{"ordinary escapes", `\n \t a\ b C:\Games\game \\`, []string{"n", "t", "a b", "C:Gamesgame", `\`}},
		{"escaped quotes", `a\"b it\'s "say \"hello\""`, []string{`a"b`, "it's", `say "hello"`}},
		{"double quoted escapes", `"a\\b\"c"`, []string{`a\b"c`}},
		{"double quoted ordinary backslashes", `"C:\Games\game \n \t \$HOME \` + "`" + ` a\ b"`, []string{`C:\Games\game \n \t \$HOME \` + "`" + ` a\ b`}},
		{"single quoted literals", `'a\b\"c' 'say "hello"'`, []string{`a\b\"c`, `say "hello"`}},
		{"line continuation", "one\\\ntwo \"three\\\nfour\"", []string{"onetwo", "threefour"}},
		{"only continuation", "\\\n", nil},
		{"single quoted newline", "'one\\\ntwo'", []string{"one\\\ntwo"}},
		{"no expansion", `$HOME ${USER} %PATH% ; & | > < $(whoami) *.exe ~ #comment`, []string{"$HOME", "${USER}", "%PATH%", ";", "&", "|", ">", "<", "$(whoami)", "*.exe", "~", "#comment"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseHeroicArguments(tc.input)
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseHeroicArguments(%q) = %#v, %v; want %#v", tc.input, got, err, tc.want)
			}
		})
	}
}

func TestParseHeroicArgumentsErrors(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{`one "two`, "Got EOF while in a quoted string"},
		{`'two`, "Got EOF while in a quoted string"},
		{`"escaped\"`, "Got EOF while in a quoted string"},
		{`"trailing\`, "Got EOF while in a quoted string"},
		{`one two\`, "Got EOF while in an escape sequence"},
		{`\`, "Got EOF while in an escape sequence"},
		{`$'line\n'`, "unsupported dollar-quoted Heroic argument"},
		{`$"localized"`, "unsupported dollar-quoted Heroic argument"},
	} {
		got, err := parseHeroicArguments(tc.input)
		if got != nil || err == nil || err.Error() != tc.want {
			t.Errorf("parseHeroicArguments(%q) = %#v, %v; want error %q", tc.input, got, err, tc.want)
		}
	}
}
