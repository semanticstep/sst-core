// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package interactive

import (
	"errors"
	"fmt"
	"strings"
)

type aliasKind string

const (
	kindSuperRepository aliasKind = "superrepository"
	kindRepository      aliasKind = "repository"
	kindDataset         aliasKind = "dataset"
	kindStage           aliasKind = "stage"
	kindNamedGraph      aliasKind = "namedgraph"
	kindIBNode          aliasKind = "ibnode"
)

type aliasCmdErrKind int

const (
	errAliasNotFound aliasCmdErrKind = iota
	errCommandNotFound
	errCommandWrongKind
)

type aliasCmdError struct {
	Kind         aliasCmdErrKind
	Alias        string
	Command      string
	AliasKind    aliasKind
	AllowedKinds []aliasKind
}

func (e *aliasCmdError) Error() string {
	return formatAliasCmdError(e)
}

type topLevelCmdError struct {
	Command string
}

func (e *topLevelCmdError) Error() string {
	return formatTopLevelError(e)
}

func resolveAliasKind(alias string) (aliasKind, bool) {
	if _, ok := interactiveConfig.SuperRepositories[alias]; ok {
		return kindSuperRepository, true
	}
	if _, ok := interactiveConfig.Repositories[alias]; ok {
		return kindRepository, true
	}
	if _, ok := interactiveConfig.Datasets[alias]; ok {
		return kindDataset, true
	}
	if _, ok := interactiveConfig.Stages[alias]; ok {
		return kindStage, true
	}
	if _, ok := interactiveConfig.NamedGraphs[alias]; ok {
		return kindNamedGraph, true
	}
	if _, ok := interactiveConfig.IBNodes[alias]; ok {
		return kindIBNode, true
	}
	return "", false
}

func commandListForKind(kind aliasKind) []string {
	set, ok := commandsByKind[kind]
	if !ok {
		return nil
	}
	cmds := make([]string, 0, len(set))
	for cmd := range set {
		cmds = append(cmds, cmd)
	}
	return sortedCommandList(cmds)
}

func allKnownCommands() []string {
	seen := make(map[string]struct{})
	var cmds []string
	for _, set := range commandsByKind {
		for cmd := range set {
			if _, ok := seen[cmd]; ok {
				continue
			}
			seen[cmd] = struct{}{}
			cmds = append(cmds, cmd)
		}
	}
	return sortedCommandList(cmds)
}

func isCommandAllowed(kind aliasKind, command string) bool {
	set, ok := commandsByKind[kind]
	if !ok {
		return false
	}
	_, ok = set[strings.ToLower(command)]
	return ok
}

func isKnownCommand(command string) bool {
	_, ok := aliasCmdKinds[strings.ToLower(command)]
	return ok
}

func resolveAliasCommand(alias, command string) error {
	kind, aliasOK := resolveAliasKind(alias)
	if !aliasOK {
		return &aliasCmdError{Kind: errAliasNotFound, Alias: alias, Command: command}
	}
	if isCommandAllowed(kind, command) {
		return nil
	}
	if isKnownCommand(command) {
		return &aliasCmdError{
			Kind:         errCommandWrongKind,
			Alias:        alias,
			Command:      command,
			AliasKind:    kind,
			AllowedKinds: kindsForCommand(command),
		}
	}
	return &aliasCmdError{
		Kind:      errCommandNotFound,
		Alias:     alias,
		Command:   command,
		AliasKind: kind,
	}
}

func formatAliasCmdError(err error) string {
	var acErr *aliasCmdError
	if !errors.As(err, &acErr) {
		return err.Error()
	}

	switch acErr.Kind {
	case errAliasNotFound:
		return fmt.Sprintf("Alias '%s' not found. Use 'status' to see open aliases.", acErr.Alias)
	case errCommandNotFound:
		return fmt.Sprintf("Unknown command '%s' for %s '%s'.", acErr.Command, acErr.AliasKind, acErr.Alias)
	case errCommandWrongKind:
		kindNames := make([]string, 0, len(acErr.AllowedKinds))
		for _, k := range acErr.AllowedKinds {
			kindNames = append(kindNames, string(k))
		}
		return fmt.Sprintf("Command '%s' is not available for %s '%s' (available for: %s).", acErr.Command, acErr.AliasKind, acErr.Alias, strings.Join(kindNames, ", "))
	default:
		return err.Error()
	}
}

func formatTopLevelError(err error) string {
	var tlErr *topLevelCmdError
	if !errors.As(err, &tlErr) {
		return err.Error()
	}
	msg := fmt.Sprintf("Unknown command '%s'.", tlErr.Command)
	if suggestion := suggestClosest(tlErr.Command, topLevelCommandNames()); suggestion != "" {
		msg += fmt.Sprintf(" Did you mean: %s?", suggestion)
	}
	msg += " Type 'help' for available commands."
	return msg
}

func suggestClosest(name string, candidates []string) string {
	nameLower := strings.ToLower(name)
	var best string
	for _, c := range candidates {
		cLower := strings.ToLower(c)
		if strings.HasPrefix(cLower, nameLower) || strings.HasPrefix(nameLower, cLower) {
			if best == "" || len(c) < len(best) {
				best = c
			}
		}
	}
	return best
}

func dispatchAliasCommand(alias, command string, args []string) error {
	if err := resolveAliasCommand(alias, command); err != nil {
		return err
	}
	kind, _ := resolveAliasKind(alias)
	handler, ok := lookupAliasHandler(kind, command)
	if !ok {
		return &aliasCmdError{
			Kind:      errCommandNotFound,
			Alias:     alias,
			Command:   command,
			AliasKind: kind,
		}
	}
	handler(alias, args)
	return nil
}

func dispatchTopLevelCommand(name string, args []string) error {
	def, ok := topLevelByName[strings.ToLower(name)]
	if !ok {
		return &topLevelCmdError{Command: name}
	}
	def.Handler(args)
	return nil
}
