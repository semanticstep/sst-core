// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package utils

import (
	"fmt"
	"strconv"
	"unicode"
	"unicode/utf8"
)

// SplitCommandArgs splits s into tokens like strings.Fields, but treats
// "..." and '...' as single tokens (quotes stripped from the result).
func SplitCommandArgs(s string) ([]string, error) {
	var tokens []string
	i := 0
	for i < len(s) {
		r, size := utf8.DecodeRuneInString(s[i:])
		if unicode.IsSpace(r) {
			i += size
			continue
		}
		if r == '"' || r == '\'' {
			token, next, err := readQuotedToken(s, i, byte(r))
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, token)
			i = next
			continue
		}
		start := i
		for i < len(s) {
			r, size := utf8.DecodeRuneInString(s[i:])
			if unicode.IsSpace(r) {
				break
			}
			if r == '"' || r == '\'' {
				return nil, fmt.Errorf("mismatched or unclosed quote in command (check your \" or ')")
			}
			i += size
		}
		tokens = append(tokens, s[start:i])
	}
	return tokens, nil
}

func readQuotedToken(s string, start int, quote byte) (string, int, error) {
	if quote == '\'' {
		return readSingleQuotedToken(s, start)
	}
	end := start + 1
	escaped := false
	for end < len(s) {
		if escaped {
			escaped = false
			end++
			continue
		}
		if s[end] == '\\' {
			escaped = true
			end++
			continue
		}
		if s[end] == quote {
			quoted := s[start : end+1]
			value, err := strconv.Unquote(quoted)
			if err != nil {
				return "", start, fmt.Errorf("mismatched or unclosed quote in command (check your \" or ')")
			}
			return value, end + 1, nil
		}
		end++
	}
	return "", start, fmt.Errorf("mismatched or unclosed quote in command (check your \" or ')")
}

func readSingleQuotedToken(s string, start int) (string, int, error) {
	end := start + 1
	for end < len(s) {
		if s[end] == '\'' {
			return s[start+1 : end], end + 1, nil
		}
		end++
	}
	return "", start, fmt.Errorf("mismatched or unclosed quote in command (check your \" or ')")
}
