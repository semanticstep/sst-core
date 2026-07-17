// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package interactive

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

type aliasHandler func(alias string, args []string)

type TopLevelCommandDef struct {
	Name    string
	Handler func(args []string)
	Usage   string
	Help    string
}

type AliasCommandDef struct {
	Name    string
	Kinds   []aliasKind
	Handler aliasHandler
	ByKind  map[aliasKind]aliasHandler
	Usage   string
	Help    string
}

var (
	topLevelCommands []TopLevelCommandDef
	aliasCommands    []AliasCommandDef
	topLevelByName   map[string]TopLevelCommandDef
	aliasByKindCmd   map[aliasKind]map[string]AliasCommandDef
	aliasCmdKinds    map[string][]aliasKind
	commandsByKind   map[aliasKind]map[string]struct{}
)

func init() {
	registerTopLevelCommands()
	registerAliasCommands()
	buildRegistryIndexes()
}

func registerTopLevelCommands() {
	topLevelCommands = []TopLevelCommandDef{
		{Name: "q", Handler: func(_ []string) { fmt.Println("Exiting...") }, Usage: "q", Help: "Exit interactive mode"},
		{Name: "help", Handler: func(_ []string) { displayHelp(os.Stdout) }, Usage: "help", Help: "Show this help message"},
		{Name: "openlocalrepository", Handler: func(args []string) { handleOpen(append([]string{"local"}, args...)) }, Usage: "openlocalrepository <path>", Help: "Open a local repository"},
		{Name: "createlocalrepository", Handler: func(args []string) { handleCreateLocalRepository(args) }, Usage: "createlocalrepository <path> [--basic]", Help: "Create a new local repository at the given path"},
		{Name: "openlocalflatrepository", Handler: func(args []string) { handleOpenLocalFlatRepository(args) }, Usage: "openlocalflatrepository <path>", Help: "Open a local flat repository (directory of .sst files)"},
		{Name: "openremoterepository", Handler: func(args []string) { handleOpen(append([]string{"remote"}, args...)) }, Usage: "openremoterepository <URL>", Help: "Open a remote repository"},
		{Name: "openlocalsuperrepository", Handler: func(args []string) { handleOpenLocalSuperRepository(args) }, Usage: "openlocalsuperrepository <path>", Help: "Open a local SuperRepository"},
		{Name: "openremotesuperrepository", Handler: func(args []string) { handleOpenRemoteSuperRepository(args) }, Usage: "openremotesuperrepository <URL>", Help: "Open a remote SuperRepository"},
		{Name: "status", Handler: func(_ []string) { handleStatus() }, Usage: "status", Help: "Show currently opened repo, dataset ..."},
		{Name: "rdfread", Handler: func(args []string) { handleRDFRead(args) }, Usage: "rdfread <file>", Help: "Read an RDF file in Turtle or TriG format into a new stage"},
		{Name: "sstread", Handler: func(args []string) { handleSstRead(args) }, Usage: "sstread <file>", Help: "Read an SST binary file into a new stage"},
		{Name: "importap242xml", Handler: func(args []string) { handleImportAP242XML(args) }, Usage: "importap242xml <file>", Help: "Import AP242 XML file into a new stage"},
		{Name: "importp21", Handler: func(args []string) { handleImportP21(args) }, Usage: "importp21 <file> [-o <raw-ttl>]", Help: "Import P21/STEP file into a new stage; use -o to write pre-conversion raw P21 Turtle for debugging"},
		{Name: "importsvg", Handler: func(args []string) { handleImportSVG(args) }, Usage: "importsvg <file.svg>", Help: "Convert SVG into a new SST geometry stage"},
	}
}

func registerAliasCommands() {
	aliasCommands = []AliasCommandDef{
		// shared across multiple kinds
		{Name: "info", Kinds: []aliasKind{kindSuperRepository, kindRepository, kindStage, kindNamedGraph}, Handler: func(alias string, _ []string) { handleInfo(alias) }, Usage: "info", Help: "Show info"},
		{Name: "close", Kinds: []aliasKind{kindSuperRepository, kindRepository}, Handler: func(alias string, _ []string) { handleClose(alias) }, Usage: "close", Help: "Close a repository or SuperRepository"},

		// repository
		{Name: "superrepository", Kinds: []aliasKind{kindRepository}, Handler: func(alias string, _ []string) { handleRepositorySuperRepository(alias) }, Usage: "superrepository", Help: "Show SuperRepository information for this repository"},
		{Name: "datasets", Kinds: []aliasKind{kindRepository}, Handler: func(alias string, _ []string) { handleDatasets(alias) }, Usage: "datasets", Help: "List datasets"},
		{Name: "dataset", Kinds: []aliasKind{kindRepository}, Handler: handleDataset, Usage: "dataset <iri>", Help: "Get dataset by IRI"},
		{Name: "query", Kinds: []aliasKind{kindRepository}, Handler: handleQuery, Usage: "query <bleve-query>", Help: "Run a Bleve text query in the repository index"},
		{Name: "listfield", Kinds: []aliasKind{kindRepository}, Handler: func(alias string, _ []string) { handleListField(alias) }, Usage: "listfield", Help: "List indexed fields in the repository"},
		{Name: "log", Kinds: []aliasKind{kindRepository}, Handler: handleLog, Usage: "log [-v|--verbose]", Help: "List commit history; use -v to show detailed info"},
		{Name: "commitinfo", Kinds: []aliasKind{kindRepository}, Handler: handleCommitInfo, Usage: "commitInfo <commit-hash>", Help: "Show commit details by commit hash"},
		{Name: "commitdiff", Kinds: []aliasKind{kindRepository}, Handler: handleCommitDiff, Usage: "commitdiff <commit-hash>", Help: "Show all changes (diff) in the given commit (added/modified/deleted NamedGraphs)"},
		{Name: "checkoutcommit", Kinds: []aliasKind{kindRepository}, Handler: handleRepoCheckoutCommit, Usage: "checkoutcommit <hash>", Help: "Materialize a stage at a repository commit (all NamedGraphs affected by the commit)"},
		{Name: "dump", Kinds: []aliasKind{kindRepository}, Handler: handleDump, Usage: "dump <bucket-key>[/<sub-key>]", Help: "Dump internal BoltDB data. Use with caution. See below for key meanings."},
		{Name: "openstage", Kinds: []aliasKind{kindRepository}, Handler: handleOpenStage, Usage: "openstage", Help: "Create an empty stage"},
		{Name: "documentset", Kinds: []aliasKind{kindRepository}, Handler: handleDocumentSet, Usage: "documentset <file>", Help: "Upload a document file"},
		{Name: "documentget", Kinds: []aliasKind{kindRepository}, Handler: handleDocument, Usage: "documentget <hash> <output>", Help: "Download a document by hash to a local file"},
		{Name: "documentinfo", Kinds: []aliasKind{kindRepository}, Handler: handleDocumentInfo, Usage: "documentinfo <hash>", Help: "Show document metadata in the repository"},
		{Name: "documents", Kinds: []aliasKind{kindRepository}, Handler: func(alias string, _ []string) { handleDocuments(alias) }, Usage: "documents", Help: "List all documents in the repository"},
		{Name: "documentdelete", Kinds: []aliasKind{kindRepository}, Handler: handleDocumentDelete, Usage: "documentdelete <hash>", Help: "Delete a document by its hash"},
		{Name: "extractsstfile", Kinds: []aliasKind{kindRepository}, Handler: handleExtractSstFile, Usage: "extractsstfile <hash>", Help: "Extract the raw SST file of a NamedGraphRevision by its hash"},
		{Name: "filterextract", Kinds: []aliasKind{kindRepository}, Handler: handleFilterExtract, Usage: "filterextract <input-directory>", Help: "Split large Turtle graphs into imported subgraphs and commit results to the repository"},
		{Name: "syncfrom", Kinds: []aliasKind{kindRepository}, Handler: handleSyncFrom, Usage: "syncfrom <source-repo-alias> [branch] [dataset1] [dataset2] ...", Help: "Sync data from another repository to this repository."},
		{Name: "clone", Kinds: []aliasKind{kindRepository}, Handler: handleClone, Usage: "clone <target-directory>", Help: "Clone this repository to a local directory"},

		// superrepository
		{Name: "list", Kinds: []aliasKind{kindSuperRepository}, Handler: handleSuperRepositoryList, Usage: "list", Help: "List all repositories in the SuperRepository"},
		{Name: "get", Kinds: []aliasKind{kindSuperRepository}, Handler: handleSuperRepositoryGet, Usage: "get <repo-name>", Help: "Get a repository from the SuperRepository"},
		{Name: "create", Kinds: []aliasKind{kindSuperRepository}, Handler: handleSuperRepositoryCreate, Usage: "create <repo-name>", Help: "Create a new repository in the SuperRepository"},
		{Name: "delete", Kinds: []aliasKind{kindSuperRepository}, Handler: handleSuperRepositoryDelete, Usage: "delete <repo-name>", Help: "Delete a repository from the SuperRepository"},

		// dataset
		{Name: "listcommits", Kinds: []aliasKind{kindDataset}, Handler: commitsHandler("listcommits"), Usage: "listcommits [--details]", Help: "Walk commit history from leaf commits and each branch tip (deduplicated)"},
		{Name: "commitdetailsbyhash", Kinds: []aliasKind{kindDataset}, Handler: commitsHandler("commitdetailsbyhash"), Usage: "commitdetailsbyhash <hash>", Help: "Show commit details by hash"},
		{Name: "commitdetailsbybranch", Kinds: []aliasKind{kindDataset}, Handler: commitsHandler("commitdetailsbybranch"), Usage: "commitdetailsbybranch <branch>", Help: "Show commit details by branch"},
		{Name: "branches", Kinds: []aliasKind{kindDataset}, Handler: handleBranches, Usage: "branches", Help: "List all branches and their commit hashes"},
		{Name: "leafcommits", Kinds: []aliasKind{kindDataset}, Handler: handleLeafCommits, Usage: "leafcommits", Help: "List all leaf commits (commits not identified by a branch)"},
		{Name: "checkoutcommit", Kinds: []aliasKind{kindDataset}, Handler: handleCheckoutCommit, Usage: "checkoutcommit <hash>", Help: "Checkout commit"},
		{Name: "checkoutrevision", Kinds: []aliasKind{kindDataset}, Handler: handleCheckoutRevision, Usage: "checkoutrevision <hash>", Help: "Checkout dataset revision"},
		{Name: "checkoutbranch", Kinds: []aliasKind{kindDataset}, Handler: handleCheckoutBranch, Usage: "checkoutbranch <name>", Help: "Checkout branch"},
		{Name: "setbranchcommit", Kinds: []aliasKind{kindDataset}, Handler: handleSetBranchCommit, Usage: "setbranchcommit <commit-hash> <branch-name>", Help: "Set a branch to point to a commit"},
		{Name: "setbranchrevision", Kinds: []aliasKind{kindDataset}, Handler: handleSetBranchRevision, Usage: "setbranchrevision <dataset-revision-hash> <branch-name>", Help: "Set a branch to point to a dataset revision"},
		{Name: "removebranch", Kinds: []aliasKind{kindDataset}, Handler: handleRemoveBranch, Usage: "removebranch <branch-name>", Help: "Remove a branch from the dataset"},
		{Name: "diff", Kinds: []aliasKind{kindDataset}, Handler: handleDiff, Usage: "diff <NGR-hash1> <NGR-hash2>", Help: "Compare two NamedGraphRevision hashes and show their differences"},
		{Name: "history", Kinds: []aliasKind{kindDataset}, Handler: handleHistory, Usage: "history", Help: "Show commit history graph for the dataset"},

		// stage
		{Name: "namedgraphs", Kinds: []aliasKind{kindStage}, Handler: func(alias string, _ []string) { handleListNamedGraphs(alias) }, Usage: "namedgraphs", Help: "List named graphs in a stage"},
		{Name: "referencednamedgraphs", Kinds: []aliasKind{kindStage}, Handler: func(alias string, _ []string) { handleListReferencedNamedGraphs(alias) }, Usage: "referencednamedgraphs", Help: "List referenced named graphs in a stage"},
		{Name: "namedgraph", Kinds: []aliasKind{kindStage}, Handler: handleNamedGraph, Usage: "namedgraph <iri>", Help: "Get named graph by IRI"},
		{Name: "moveandmerge", Kinds: []aliasKind{kindStage}, Handler: handleMoveAndMerge, Usage: "moveandmerge", Help: "Move and merge named graphs"},
		{Name: "alignhistory", Kinds: []aliasKind{kindStage}, Handler: handleAlignHistory, Usage: "alignhistory <from-stage>", Help: "Copy repository pointer and checkout metadata from from-stage onto to-stage (e.g. after rdfread) so commit preserves original history"},
		{Name: "commit", Kinds: []aliasKind{kindStage}, Handler: handleStageCommit, Usage: "commit <message> [branch]", Help: "Commit current changes in the stage, with a message and optional branch name"},
		{Name: "validate", Kinds: []aliasKind{kindStage}, Handler: handleStageValidate, Usage: "validate [-o <file>]", Help: "Validate stage (rdf/domain-range); print report to console, or write to file with -o (default extension .txt)"},
		{Name: "trig", Kinds: []aliasKind{kindStage}, Handler: handleStageTriG, Usage: "trig", Help: "Print RDF of the Stage to the console (TriG format)"},
		{Name: "writesstfilesdirectory", Kinds: []aliasKind{kindStage}, Handler: handleWriteSstFilesDirectory, Usage: "writesstfilesdirectory <directory>", Help: "Write modified NamedGraphs as SST files into a directory (IRI-base64 filenames)"},
		{
			Name:  "rdfwrite",
			Kinds: []aliasKind{kindStage, kindNamedGraph},
			ByKind: map[aliasKind]aliasHandler{
				kindStage:      handleStageRdfWrite,
				kindNamedGraph: handleRdfWrite,
			},
			Usage: "rdfwrite <file>",
			Help:  "Write RDF to a file (TriG for stage, Turtle for named graph)",
		},

		// namedgraph
		{Name: "foririnodes", Kinds: []aliasKind{kindNamedGraph}, Handler: func(alias string, _ []string) { handleListForIRINode(alias) }, Usage: "foririnodes", Help: "List all IRI nodes in the named graph"},
		{Name: "forallibnodes", Kinds: []aliasKind{kindNamedGraph}, Handler: func(alias string, _ []string) { handleListForAllIBNodes(alias) }, Usage: "forallibnodes", Help: "List all IBNodes (IRI nodes and blank nodes) in the named graph"},
		{Name: "forblanknodes", Kinds: []aliasKind{kindNamedGraph}, Handler: func(alias string, _ []string) { handleListForBlankNode(alias) }, Usage: "forblanknodes", Help: "List all blank nodes in the named graph"},
		{Name: "getirinodebyfragment", Kinds: []aliasKind{kindNamedGraph}, Handler: handleGetIRINodeByFragment, Usage: "getirinodebyfragment", Help: "Get IRINode by fragment (fragment ID)"},
		{Name: "getblanknodebyfragment", Kinds: []aliasKind{kindNamedGraph}, Handler: handleGetBlankNodeByFragment, Usage: "getblanknodebyfragment", Help: "Get blank node by fragment (fragment ID)"},
		{Name: "exportap242xml", Kinds: []aliasKind{kindNamedGraph}, Handler: handleNamedGraphExportAP242XML, Usage: "exportap242xml <file>", Help: "Export a NamedGraph to an AP242 XML .xml file"},
		{Name: "ttl", Kinds: []aliasKind{kindNamedGraph}, Handler: func(alias string, _ []string) { handleTtl(alias) }, Usage: "ttl", Help: "Print RDF of the NamedGraph to the console (Turtle format)"},
		{Name: "sstwrite", Kinds: []aliasKind{kindNamedGraph}, Handler: handleSstWrite, Usage: "sstwrite <file>", Help: "Write NamedGraph to an SST binary file"},
		{Name: "exportsvg", Kinds: []aliasKind{kindNamedGraph}, Handler: handleNamedGraphExportSVG, Usage: "exportsvg <file>", Help: "Export the named graph to SVG (default extension .svg)"},

		// ibnode
		{Name: "forall", Kinds: []aliasKind{kindIBNode}, Handler: func(alias string, _ []string) { handleForAllTriples(alias) }, Usage: "forall", Help: "List triples in an IBNode"},
	}
}

func commitsHandler(command string) aliasHandler {
	return func(alias string, args []string) {
		handleCommits(alias, command, args)
	}
}

func buildRegistryIndexes() {
	topLevelByName = make(map[string]TopLevelCommandDef, len(topLevelCommands))
	for _, def := range topLevelCommands {
		topLevelByName[strings.ToLower(def.Name)] = def
	}

	aliasByKindCmd = make(map[aliasKind]map[string]AliasCommandDef)
	aliasCmdKinds = make(map[string][]aliasKind)
	commandsByKind = make(map[aliasKind]map[string]struct{})

	for _, def := range aliasCommands {
		name := strings.ToLower(def.Name)
		for _, kind := range def.Kinds {
			if aliasByKindCmd[kind] == nil {
				aliasByKindCmd[kind] = make(map[string]AliasCommandDef)
			}
			aliasByKindCmd[kind][name] = def

			if commandsByKind[kind] == nil {
				commandsByKind[kind] = make(map[string]struct{})
			}
			commandsByKind[kind][name] = struct{}{}

			if !containsKind(aliasCmdKinds[name], kind) {
				aliasCmdKinds[name] = append(aliasCmdKinds[name], kind)
			}
		}
	}
}

func containsKind(kinds []aliasKind, kind aliasKind) bool {
	for _, k := range kinds {
		if k == kind {
			return true
		}
	}
	return false
}

func lookupAliasHandler(kind aliasKind, command string) (aliasHandler, bool) {
	def, ok := aliasByKindCmd[kind][strings.ToLower(command)]
	if !ok {
		return nil, false
	}
	if def.ByKind != nil {
		if h, ok := def.ByKind[kind]; ok {
			return h, true
		}
	}
	if def.Handler != nil {
		return def.Handler, true
	}
	return nil, false
}

func topLevelCommandNames() []string {
	names := make([]string, 0, len(topLevelCommands))
	for _, def := range topLevelCommands {
		names = append(names, def.Name)
	}
	return names
}

func kindsForCommand(command string) []aliasKind {
	kinds := aliasCmdKinds[strings.ToLower(command)]
	out := make([]aliasKind, len(kinds))
	copy(out, kinds)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func kindLabel(kind aliasKind) string {
	switch kind {
	case kindSuperRepository:
		return "superrepo"
	case kindRepository:
		return "repo"
	case kindDataset:
		return "dataset"
	case kindStage:
		return "stage"
	case kindNamedGraph:
		return "namedgraph"
	case kindIBNode:
		return "ibnode"
	default:
		return string(kind)
	}
}

func sortedCommandList(cmds []string) []string {
	out := make([]string, len(cmds))
	copy(out, cmds)
	sort.Strings(out)
	return out
}
