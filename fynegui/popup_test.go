package fynegui

import (
	"strings"
	"testing"
)

func TestWrapPopupText(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"short", "Short error.", "Short error."},
		{"existing breaks", "Error\n\nDetails:\none\ntwo", "Error\n\nDetails:\none\ntwo"},
		{"word boundary", strings.Repeat("a", 75) + " long error", strings.Repeat("a", 75) + " long\nerror"},
		{"long path", strings.Repeat("a", 170), strings.Repeat("a", 80) + "\n" + strings.Repeat("a", 80) + "\n" + strings.Repeat("a", 10)},
		{"unicode", strings.Repeat("\u00e8", 81), strings.Repeat("\u00e8", 80) + "\n\u00e8"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := wrapPopupText(tc.input)
			if got != tc.want {
				t.Fatalf("wrapped text = %q; want %q", got, tc.want)
			}
			for _, line := range strings.Split(got, "\n") {
				if len([]rune(line)) > 80 {
					t.Fatalf("line exceeds 80 characters: %q", line)
				}
			}
		})
	}
}
