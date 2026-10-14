package executor

import (
	"fmt"
	"strings"
)

// parseHeroicArguments implements shlex 2.1.2's POSIX quoting and escaping
// rules without invoking a shell or expanding arguments.
func parseHeroicArguments(input string) ([]string, error) {
	var args []string
	var token strings.Builder
	var quote byte
	started := false
	for i := 0; i < len(input); i++ {
		c := input[i]
		if c == '\\' && quote != '\'' {
			if i+1 == len(input) {
				if quote != 0 {
					return nil, fmt.Errorf("Got EOF while in a quoted string")
				}
				return nil, fmt.Errorf("Got EOF while in an escape sequence")
			}
			i++
			next := input[i]
			if next == '\n' {
				continue
			}
			if quote == '"' && next != '"' && next != '\\' {
				token.WriteByte('\\')
			}
			token.WriteByte(next)
			started = true
			continue
		}
		if quote != 0 {
			if c == quote {
				quote = 0
			} else {
				token.WriteByte(c)
			}
			continue
		}
		switch c {
		case '$':
			if i+1 < len(input) && (input[i+1] == '\'' || input[i+1] == '"') {
				return nil, fmt.Errorf("unsupported dollar-quoted Heroic argument")
			}
			token.WriteByte(c)
			started = true
		case '\'', '"':
			quote = c
			started = true
		case ' ', '\t', '\r', '\n':
			if started {
				args = append(args, token.String())
				token.Reset()
				started = false
			}
		default:
			token.WriteByte(c)
			started = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("Got EOF while in a quoted string")
	}
	if started {
		args = append(args, token.String())
	}
	return args, nil
}
