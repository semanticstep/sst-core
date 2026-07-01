// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package interactive

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/semanticstep/sst-core/cli/cmd/utils"
)

type inputForm int

const (
	formEmpty inputForm = iota
	formTopLevel
	formAliasCommand
)

func parseInput(input string) (form inputForm, topLevel, alias, command string, args []string, err error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return formEmpty, "", "", "", nil, nil
	}

	first, remainder, ok := firstToken(input)
	if !ok {
		return formEmpty, "", "", "", nil, nil
	}

	if parts := strings.SplitN(first, ".", 2); len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		alias = parts[0]
		cmdPart := parts[1]
		if remainder != "" {
			cmdPart += " " + remainder
		}
		commandArgs, splitErr := utils.SplitCommandArgs(cmdPart)
		if splitErr != nil {
			return formEmpty, "", "", "", nil, splitErr
		}
		if len(commandArgs) == 0 {
			return formEmpty, "", "", "", nil, errMissingAliasCommand
		}
		return formAliasCommand, "", alias, strings.ToLower(commandArgs[0]), commandArgs[1:], nil
	}

	topLevel = strings.ToLower(first)
	if remainder != "" {
		args, err = utils.SplitCommandArgs(remainder)
		if err != nil {
			return formEmpty, "", "", "", nil, err
		}
	}
	return formTopLevel, topLevel, "", "", args, nil
}

var errMissingAliasCommand = &parseError{msg: "Missing command after alias. Usage: <alias>.<command> [args...]"}

type parseError struct {
	msg string
}

func (e *parseError) Error() string {
	return e.msg
}

func firstToken(input string) (token, remainder string, ok bool) {
	if input == "" {
		return "", "", false
	}
	i := 0
	for i < len(input) {
		r, size := utf8.DecodeRuneInString(input[i:])
		if !unicode.IsSpace(r) {
			break
		}
		i += size
	}
	if i >= len(input) {
		return "", "", false
	}
	start := i
	for i < len(input) {
		r, size := utf8.DecodeRuneInString(input[i:])
		if unicode.IsSpace(r) {
			break
		}
		i += size
	}
	return input[start:i], strings.TrimSpace(input[i:]), true
}
