// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package interactive

import (
	"fmt"
	"io"
	"sort"
)

// displayHelp shows available commands
func displayHelp(w io.Writer) {
	fmt.Fprintln(w, "Available commands in interactive mode:")

	for _, def := range topLevelCommands {
		fmt.Fprintf(w, "    %-40s %s\n", def.Usage, def.Help)
		if topLevelHelpBreakAfter(def.Name) {
			fmt.Fprintln(w)
		}
	}

	fmt.Fprintln(w)

	aliasHelpByKind := collectAliasHelpByKind()
	kindOrder := []aliasKind{
		kindRepository,
		kindSuperRepository,
		kindDataset,
		kindStage,
		kindNamedGraph,
		kindIBNode,
	}
	firstKind := true
	for _, kind := range kindOrder {
		entries := aliasHelpByKind[kind]
		if len(entries) == 0 {
			continue
		}
		if !firstKind {
			fmt.Fprintln(w)
		}
		firstKind = false
		label := kindLabel(kind)
		for _, entry := range entries {
			fmt.Fprintf(w, "    %-40s %s\n", "<"+label+">."+entry.usage, entry.help)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "<repo>.dump key reference:")
	fmt.Fprintln(w, "    ngr   - NamedGraphRevisions   (revisions of named graphs)")
	fmt.Fprintln(w, "    dsr   - DatasetRevisions      (dataset version history)")
	fmt.Fprintln(w, "    c     - Commits               (commit metadata: author, message, timestamp)")
	fmt.Fprintln(w, "    ds    - Datasets              (dataset metadata: IRI, UUID, etc.)")
	fmt.Fprintln(w, "    dl    - DatasetLog            (chronological commit log entries)")

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "    r1.dump ds                     # Dump all datasets")
	fmt.Fprintln(w, "    r1.dump \"c/<commit-hash>\"      # Dump specific commit metadata entry")
	fmt.Fprintln(w, "    r1.dump c                      # View all commit metadata")
}

func topLevelHelpBreakAfter(name string) bool {
	switch name {
	case "help", "openremotesuperrepository", "status", "importsvg":
		return true
	default:
		return false
	}
}

type aliasHelpEntry struct {
	usage string
	help  string
}

func collectAliasHelpByKind() map[aliasKind][]aliasHelpEntry {
	out := make(map[aliasKind][]aliasHelpEntry)
	seen := make(map[aliasKind]map[string]struct{})

	for _, def := range aliasCommands {
		for _, kind := range def.Kinds {
			if seen[kind] == nil {
				seen[kind] = make(map[string]struct{})
			}
			name := def.Name
			if _, ok := seen[kind][name]; ok {
				continue
			}
			seen[kind][name] = struct{}{}
			out[kind] = append(out[kind], aliasHelpEntry{usage: def.Usage, help: def.Help})
		}
	}

	for kind := range out {
		sort.Slice(out[kind], func(i, j int) bool {
			return out[kind][i].usage < out[kind][j].usage
		})
	}
	return out
}
