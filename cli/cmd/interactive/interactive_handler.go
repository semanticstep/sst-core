// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package interactive

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/semanticstep/sst-core/cli/cmd/utils"
	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/sstauth"
	"github.com/semanticstep/sst-core/step/ap242xmlexport"
	"github.com/semanticstep/sst-core/step/ap242xmlimport"
	"github.com/semanticstep/sst-core/step/p21"
	svgtosst "github.com/semanticstep/sst-core/tools/svg_to_sst"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func handleOpen(args []string) {
	var repoType, repoPath, repoURL, alias string

	if len(args) == 0 || (args[0] != "local" && args[0] != "remote") {
		fmt.Println("Usage: openlocalrepository <path> [-a <alias>]")
		fmt.Println("   or: openremoterepository <URL> [-a <alias>]")
		return
	}
	repoType = args[0]
	args = args[1:]

	switch repoType {
	case "local":
		if len(args) == 0 {
			fmt.Println("Usage: openlocalrepository <path> [-a <alias>]")
			return
		}
		repoPath = args[0]
		args = args[1:]
	case "remote":
		if len(args) == 0 {
			fmt.Println("Usage: openremoterepository <URL> [-a <alias>]")
			return
		}
		repoURL = args[0]
		args = args[1:]
	default:
		fmt.Println("Invalid repository type. Use 'local' or 'remote'.")
		return
	}

	aliasResult, err := utils.GetAlias(args, "repository")
	if err != nil {
		fmt.Println(err)
		return
	}
	alias = aliasResult.Alias

	// Use defer to confirm alias generation only on success
	success := false
	defer func() {
		if success {
			aliasResult.Confirm()
		}
	}()

	// Check if alias already exists
	if _, exists := interactiveConfig.Repositories[alias]; exists {
		fmt.Printf("Error: Repository with alias '%s' already exists.\n", alias)
		return
	}

	// Open repository
	var repository sst.Repository

	switch repoType {
	case "local":
		if err := utils.ValidatePath(repoPath); err != nil {
			utils.PrintCLIProblem("validate path", err)
			return
		}
		repository, err = sst.OpenLocalRepository(repoPath, "default@semanticstep.net", "default")
		if err != nil {
			fmt.Printf("Error: Unable to open local repository: %s\n", err)
			return
		}

		interactiveConfig.RepositoryLocations[alias] = repoPath
		interactiveConfig.RepositoryTypes[alias] = "local"
	case "remote":
		// transportCreds, err := testutil.TestTransportCreds()
		// if err != nil {
		// 	fmt.Printf("Failed to load TLS credentials: %v\n", err)
		// 	return
		// }
		// constructCtx := auth.ContextWithAuthProvider(context.TODO(), utils.TestProvider)
		// repository, err = sst.OpenRemoteRepository(constructCtx, repoURL, transportCreds)
		// if err != nil {
		// 	fmt.Printf("Error: Unable to open %s repository: %s\n", repoType, err)
		// 	return
		// }
		creds := credentials.NewTLS(nil)
		realProvider := utils.GetRealProvider()
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), realProvider)

		var panicErr any
		func() {
			defer func() {
				if r := recover(); r != nil {
					panicErr = r
				}
			}()

			repository, err = sst.OpenRemoteRepository(constructCtx, repoURL, grpc.WithTransportCredentials(creds))
		}()
		if panicErr != nil {
			fmt.Printf("Cannot connect to remote repository at '%s'.\n", repoURL)
			fmt.Println("Please check that the URL is correct and your network is available.")
			fmt.Printf("(Technical info: %v)\n", panicErr)
			return
		}
		if err != nil {
			msg, _ := utils.ExplainRemoteRepositoryOpenError(repoURL, err)
			fmt.Println(msg)
			return
		}
		if repository == nil {
			fmt.Printf("Could not open remote repository '%s' (internal error: empty handle).\n", repoURL)
			return
		}

		interactiveConfig.AuthContexts[repository] = constructCtx
		interactiveConfig.RepositoryLocations[alias] = repoURL
		interactiveConfig.RepositoryTypes[alias] = "remote"
	default:
		fmt.Println("Invalid repository type. Use 'local' or 'remote'.")
		return
	}

	// Success! Set flag so defer will confirm
	success = true

	interactiveConfig.Repositories[alias] = repository
	interactiveConfig.RepositoryAliases = append(interactiveConfig.RepositoryAliases, alias)

	// Confirm success
	fmt.Printf("Repository '%s' (%s) opened successfully.\n", alias, repoType)
}

// handleOpenLocalFlatRepository opens a LocalFlat repository (directory of .sst files).
func handleOpenLocalFlatRepository(args []string) {
	var repoPath, alias string

	if len(args) == 0 {
		fmt.Println("Usage: openlocalflatrepository <path> [-a <alias>]")
		return
	}

	repoPath = args[0]

	aliasResult, err := utils.GetAlias(args[1:], "repository")
	if err != nil {
		fmt.Println(err)
		return
	}
	alias = aliasResult.Alias

	success := false
	defer func() {
		if success {
			aliasResult.Confirm()
		}
	}()

	if _, exists := interactiveConfig.Repositories[alias]; exists {
		fmt.Printf("Error: Repository with alias '%s' already exists.\n", alias)
		return
	}

	if err := utils.ValidatePath(repoPath); err != nil {
		utils.PrintCLIProblem("validate path", err)
		return
	}

	repository, err := sst.OpenLocalFlatRepository(repoPath)
	if err != nil {
		fmt.Printf("Error: Unable to open local flat repository: %s\n", err)
		return
	}

	success = true

	interactiveConfig.Repositories[alias] = repository
	interactiveConfig.RepositoryLocations[alias] = repoPath
	interactiveConfig.RepositoryTypes[alias] = "localflat"
	interactiveConfig.RepositoryAliases = append(interactiveConfig.RepositoryAliases, alias)

	fmt.Printf("Repository '%s' (local flat) opened successfully.\n", alias)
}

// handleCreateLocalRepository creates a new local repository at the given path and registers it in the session.
func handleCreateLocalRepository(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: createlocalrepository <path> [-a <alias>] [--basic]")
		fmt.Println("  --basic  Create a local basic repository without revision history (default: local full with history)")
		return
	}

	revisionHistory := true
	var pathArgs []string
	for _, arg := range args {
		if arg == "--basic" {
			revisionHistory = false
		} else {
			pathArgs = append(pathArgs, arg)
		}
	}

	if len(pathArgs) == 0 {
		fmt.Println("Usage: createlocalrepository <path> [-a <alias>] [--basic]")
		return
	}

	repoPath := pathArgs[0]

	aliasResult, err := utils.GetAlias(pathArgs[1:], "repository")
	if err != nil {
		fmt.Println(err)
		return
	}
	alias := aliasResult.Alias

	success := false
	defer func() {
		if success {
			aliasResult.Confirm()
		}
	}()

	if _, exists := interactiveConfig.Repositories[alias]; exists {
		fmt.Printf("Error: Repository with alias '%s' already exists.\n", alias)
		return
	}

	absRepoPath, err := filepath.Abs(repoPath)
	if err != nil {
		utils.PrintCLIProblem("resolve path", err)
		return
	}

	if _, err := os.Stat(absRepoPath); err == nil {
		fmt.Printf("Error: Directory '%s' already exists.\n", absRepoPath)
		return
	} else if !os.IsNotExist(err) {
		utils.PrintCLIProblem("access path", err)
		return
	}

	repository, err := sst.CreateLocalRepository(absRepoPath, "default@semanticstep.net", "default", revisionHistory)
	if err != nil {
		utils.PrintCLIProblem("create local repository", err)
		return
	}

	success = true

	interactiveConfig.Repositories[alias] = repository
	interactiveConfig.RepositoryLocations[alias] = repoPath
	interactiveConfig.RepositoryTypes[alias] = "local"
	interactiveConfig.RepositoryAliases = append(interactiveConfig.RepositoryAliases, alias)

	repoKind := "local full"
	if !revisionHistory {
		repoKind = "local basic"
	}
	fmt.Printf("Repository '%s' (%s) created successfully.\n", alias, repoKind)
}

func printStatusLine(indent int, alias, detail string) {
	fmt.Printf("%s- %s: %s\n", strings.Repeat(" ", indent), alias, detail)
}

func handleStatus() {
	// Track which resources have been printed to avoid duplicates
	printed := struct {
		superRepositories map[string]bool
		repositories      map[string]bool
		datasets          map[string]bool
		stages            map[string]bool
		namedGraphs       map[string]bool
		ibNodes           map[string]bool
	}{
		superRepositories: make(map[string]bool),
		repositories:      make(map[string]bool),
		datasets:          make(map[string]bool),
		stages:            make(map[string]bool),
		namedGraphs:       make(map[string]bool),
		ibNodes:           make(map[string]bool),
	}

	// Phase 0: Display SuperRepositories with their associated repositories (hierarchical)
	for _, superRepoAlias := range interactiveConfig.SuperRepositoryAliases {
		superRepo, exists := interactiveConfig.SuperRepositories[superRepoAlias]
		if !exists || superRepo == nil {
			printStatusLine(0, superRepoAlias, "(nil)")
			continue
		}

		// Display SuperRepository
		loc := "(unknown location)"
		if v, ok := interactiveConfig.SuperRepositoryLocations[superRepoAlias]; ok && v != "" {
			loc = v
		}
		printStatusLine(0, superRepoAlias, loc)
		printed.superRepositories[superRepoAlias] = true

		// Display repositories belonging to this SuperRepository
		for _, repoAlias := range interactiveConfig.RepositoryAliases {
			repo, exists := interactiveConfig.Repositories[repoAlias]
			if !exists || repo == nil {
				continue
			}
			if repo.SuperRepository() == superRepo {
				printed.repositories[repoAlias] = true
				repoLoc := "(unknown location)"
				if v, ok := interactiveConfig.RepositoryLocations[repoAlias]; ok && v != "" {
					repoLoc = v
				}
				printStatusLine(2, repoAlias, repoLoc)
			}
		}
	}

	// Phase 1: Display repositories with their associated resources (hierarchical)
	for _, repoAlias := range interactiveConfig.RepositoryAliases {
		if printed.repositories[repoAlias] {
			continue // Already displayed under a SuperRepository
		}
		repo, exists := interactiveConfig.Repositories[repoAlias]
		if !exists || repo == nil {
			printStatusLine(0, repoAlias, "(nil)")
			continue
		}

		// Display repository
		loc := "(unknown location)"
		if v, ok := interactiveConfig.RepositoryLocations[repoAlias]; ok && v != "" {
			loc = v
		}
		printStatusLine(0, repoAlias, loc)

		// Display datasets belonging to this repository
		for _, dsAlias := range interactiveConfig.DatasetAliases {
			ds, exists := interactiveConfig.Datasets[dsAlias]
			if exists && ds != nil && ds.Repository() == repo {
				printStatusLine(2, dsAlias, ds.IRI().String())
				printed.datasets[dsAlias] = true
			}
		}

		// Display stages belonging to this repository
		for _, stAlias := range interactiveConfig.StageAliases {
			st, exists := interactiveConfig.Stages[stAlias]
			if !exists || st == nil {
				continue
			}
			// Only show stages that are linked to this specific repository
			if st.Repository() == repo {
				printStatusLine(2, stAlias, stageRevisionSuffix(stAlias))
				printed.stages[stAlias] = true

				// Display namedgraphs in this stage
				for _, ngAlias := range interactiveConfig.NamedGraphAliases {
					ng, exists := interactiveConfig.NamedGraphs[ngAlias]
					if !exists || ng == nil {
						continue
					}
					if st.NamedGraph(ng.IRI()) == nil {
						continue
					}
					printed.namedGraphs[ngAlias] = true
					printStatusLine(4, ngAlias, ngRevisionSuffix(ng))

					// Display ibnodes in this namedgraph
					for _, nodeAlias := range interactiveConfig.IBNodeAliases {
						n, exists := interactiveConfig.IBNodes[nodeAlias]
						if !exists || n == nil {
							continue
						}
						if nodeBelongsToNamedGraph(n, ng) {
							printed.ibNodes[nodeAlias] = true
							printStatusLine(6, nodeAlias, getNodeDisplayString(n))
						}
					}
				}
			}
		}
	}

	// Phase 2: Display orphaned stages (without repository or not linked to any opened repository)
	for _, stAlias := range interactiveConfig.StageAliases {
		if printed.stages[stAlias] {
			continue
		}
		st, exists := interactiveConfig.Stages[stAlias]
		if !exists || st == nil {
			// Stage alias exists but stage is nil
			printStatusLine(0, stAlias, stageRevisionSuffix(stAlias)+" (nil stage)")
			continue
		}

		// Check if stage is linked to any opened repository
		stageRepo := st.Repository()
		hasOpenedRepo := false
		if stageRepo != nil {
			for _, repo := range interactiveConfig.Repositories {
				if repo == stageRepo {
					hasOpenedRepo = true
					break
				}
			}
		}

		// Display stage if it's not linked to any opened repository
		if stageRepo == nil || !hasOpenedRepo {
			suffix := stageRevisionSuffix(stAlias)
			if stageRepo == nil {
				suffix += " (no repository)"
			} else {
				suffix += " (repository not opened)"
			}
			printStatusLine(0, stAlias, suffix)

			// Display namedgraphs in this orphaned stage
			for _, ngAlias := range interactiveConfig.NamedGraphAliases {
				if printed.namedGraphs[ngAlias] {
					continue
				}
				ng, exists := interactiveConfig.NamedGraphs[ngAlias]
				if !exists || ng == nil {
					continue
				}
				if st.NamedGraph(ng.IRI()) == nil {
					continue
				}
				printed.namedGraphs[ngAlias] = true
				printStatusLine(2, ngAlias, ngRevisionSuffix(ng))

				// Display ibnodes in this namedgraph
				for _, nodeAlias := range interactiveConfig.IBNodeAliases {
					if printed.ibNodes[nodeAlias] {
						continue
					}
					n, exists := interactiveConfig.IBNodes[nodeAlias]
					if !exists || n == nil {
						continue
					}
					if nodeBelongsToNamedGraph(n, ng) {
						printed.ibNodes[nodeAlias] = true
						printStatusLine(4, nodeAlias, getNodeDisplayString(n))
					}
				}
			}
		}
	}

	// Phase 3: Display orphaned datasets (without repository)
	for _, dsAlias := range interactiveConfig.DatasetAliases {
		if printed.datasets[dsAlias] {
			continue
		}
		ds, exists := interactiveConfig.Datasets[dsAlias]
		if exists && ds != nil {
			printStatusLine(0, dsAlias, ds.IRI().String()+" (no repository)")
		}
	}

	// Phase 4: Display orphaned namedgraphs (without stage)
	for _, ngAlias := range interactiveConfig.NamedGraphAliases {
		if printed.namedGraphs[ngAlias] {
			continue
		}
		ng, exists := interactiveConfig.NamedGraphs[ngAlias]
		if exists && ng != nil {
			printStatusLine(0, ngAlias, ngRevisionSuffix(ng)+" (no stage)")

			// Display ibnodes in this orphaned namedgraph
			for _, nodeAlias := range interactiveConfig.IBNodeAliases {
				if printed.ibNodes[nodeAlias] {
					continue
				}
				n, exists := interactiveConfig.IBNodes[nodeAlias]
				if !exists || n == nil {
					continue
				}
				if nodeBelongsToNamedGraph(n, ng) {
					printed.ibNodes[nodeAlias] = true
					printStatusLine(2, nodeAlias, getNodeDisplayString(n))
				}
			}
		}
	}

	// Phase 5: Display orphaned ibnodes (without namedgraph)
	for _, nodeAlias := range interactiveConfig.IBNodeAliases {
		if printed.ibNodes[nodeAlias] {
			continue
		}
		n, exists := interactiveConfig.IBNodes[nodeAlias]
		if exists && n != nil {
			printStatusLine(0, nodeAlias, getNodeDisplayString(n)+" (no namedgraph)")
		}
	}
}

// nodeBelongsToNamedGraph checks if an IBNode belongs to a NamedGraph
func nodeBelongsToNamedGraph(n sst.IBNode, ng sst.NamedGraph) bool {
	if ng == nil {
		return false
	}
	var iriStr string
	func() {
		defer func() { recover() }()
		iriStr = n.IRI().String()
	}()
	if iriStr != "" {
		// IRI node - check by fragment
		return ng.GetIRINodeByFragment(n.Fragment()) != nil
	}
	// Blank node - check by ID
	return ng.GetBlankNodeByID(n.ID()) != nil
}

// getNodeDisplayString returns the display string for an IBNode
func getNodeDisplayString(n sst.IBNode) string {
	var iriStr string
	func() {
		defer func() { recover() }()
		iriStr = n.IRI().String()
	}()
	if iriStr != "" {
		return iriStr
	}
	return n.ID().String()
}

// stageRevisionSuffix returns a suffix string indicating the revision information for a stage.
// It returns "[branch: xxx]" if the stage has a branch, "[revision: ...]" or "[commit: ...]" for checkout metadata,
// "[source: filepath]" if it was created from a source file, or "empty stage" otherwise.
func stageRevisionSuffix(stageAlias string) string {
	if b, ok := interactiveConfig.StageBranches[stageAlias]; ok && b != "" {
		return "[branch: " + b + "]"
	}
	if h, ok := interactiveConfig.StageRevisions[stageAlias]; ok {
		return "[revision: " + h.String() + "]"
	}
	if h, ok := interactiveConfig.StageCommits[stageAlias]; ok {
		hs := h.String()
		if len(hs) > 8 {
			hs = hs[:8]
		}
		return "[commit: " + hs + "]"
	}
	if src, ok := interactiveConfig.StageSources[stageAlias]; ok && src != "" {
		return "[source: " + src + "]"
	}
	return "empty stage"
}

// ngRevisionSuffix returns a suffix string for status display: IRI if set, else empty string.
func ngRevisionSuffix(ng sst.NamedGraph) string {
	if s := ng.IRI().String(); s != "" {
		return "[iri: " + s + "]"
	} else {
		return ""
	}
}

func handleRDFRead(args []string) {
	if len(args) < 1 {
		fmt.Println("Error: Missing arguments. Usage: rdfread <file-path> [--format ttl|trig] [-a <stage-alias>]")
		return
	}

	// Determine input file path and optional explicit format.
	// Supported:
	//   rdfread file.ttl
	//   rdfread file.trig
	//   rdfread file --format ttl|trig
	// Alias flags (-a) are handled by utils.GetAlias below.
	var (
		filePath string
		format   = sst.RdfFormatTurtle
	)

	// Find first non-flag token as the file path
	for i := 0; i < len(args); i++ {
		a := strings.TrimSpace(args[i])
		if a == "" {
			continue
		}
		// Skip alias flag and its value
		if a == "-a" && i+1 < len(args) {
			i++
			continue
		}
		// Skip format flag and its value
		if (a == "--format" || a == "-f") && i+1 < len(args) {
			i++
			continue
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		filePath = a
		break
	}

	if filePath == "" {
		fmt.Println("Error: Missing file path. Usage: rdfread <file-path> [--format ttl|trig] [-a <stage-alias>]")
		return
	}

	// Optional explicit format flag
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--format", "-f":
			if i+1 >= len(args) {
				fmt.Println("Error: Missing value for --format. Use: --format ttl|trig")
				return
			}
			switch strings.ToLower(strings.TrimSpace(args[i+1])) {
			case "ttl", "turtle":
				format = sst.RdfFormatTurtle
			case "trig":
				format = sst.RdfFormatTriG
			default:
				fmt.Printf("Error: Unsupported format %q. Supported: ttl, trig\n", args[i+1])
				return
			}
		}
	}

	// If not explicitly set, infer from file extension
	if format == sst.RdfFormatTurtle {
		lower := strings.ToLower(filePath)
		if strings.HasSuffix(lower, ".trig") {
			format = sst.RdfFormatTriG
		}
	}

	aliasResult, err := utils.GetAlias(args, "stage")
	if err != nil {
		fmt.Println(err)
		return
	}
	stageAlias := aliasResult.Alias

	// Use defer to confirm alias generation only on success
	success := false
	defer func() {
		if success {
			aliasResult.Confirm()
		}
	}()

	if _, exists := interactiveConfig.Stages[stageAlias]; exists {
		fmt.Printf("Error: Stage alias '%s' already exists.\n", stageAlias)
		return
	}

	// Open RDF file (.ttl or .trig)
	file, err := os.Open(filePath)
	if err != nil {
		utils.PrintCLIProblem("open file", err)
		return
	}
	defer func() {
		if e := file.Close(); e != nil {
			log.Printf("Error closing file: %v", e)
		}
	}()

	// Convert RDF (Turtle/TriG) into a new Stage.
	stage, err := sst.RdfRead(bufio.NewReader(file), format, sst.StrictHandler, sst.DefaultTriplexMode)
	if err != nil {
		utils.PrintCLIProblem("read RDF file", err)
		return
	}

	// Success! Set flag so defer will confirm
	success = true

	interactiveConfig.Stages[stageAlias] = stage
	interactiveConfig.StageAliases = append(interactiveConfig.StageAliases, stageAlias)
	interactiveConfig.StageSources[stageAlias] = filePath

	fmt.Printf("Stage '%s' (file '%s') opened successfully.\n", stageAlias, filePath)
}

func handleSstRead(args []string) {
	if len(args) < 1 {
		fmt.Println("Error: Missing arguments. Usage: sstread <file-path> [-a <stage-alias>]")
		return
	}

	var filePath string
	for i := 0; i < len(args); i++ {
		a := strings.TrimSpace(args[i])
		if a == "" {
			continue
		}
		if a == "-a" && i+1 < len(args) {
			i++
			continue
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		filePath = a
		break
	}

	if filePath == "" {
		fmt.Println("Error: Missing file path. Usage: sstread <file-path> [-a <stage-alias>]")
		return
	}

	aliasResult, err := utils.GetAlias(args, "stage")
	if err != nil {
		fmt.Println(err)
		return
	}
	stageAlias := aliasResult.Alias

	success := false
	defer func() {
		if success {
			aliasResult.Confirm()
		}
	}()

	if _, exists := interactiveConfig.Stages[stageAlias]; exists {
		fmt.Printf("Error: Stage alias '%s' already exists.\n", stageAlias)
		return
	}

	file, err := os.Open(filePath)
	if err != nil {
		utils.PrintCLIProblem("open file", err)
		return
	}
	defer func() {
		if e := file.Close(); e != nil {
			log.Printf("Error closing file: %v", e)
		}
	}()

	var readErr error
	var stage sst.Stage
	utils.ShowLoadingIndicator(fmt.Sprintf("Reading SST file '%s'", filePath), func() {
		graph, err := sst.SstRead(bufio.NewReader(file), sst.DefaultTriplexMode)
		if err != nil {
			readErr = err
			return
		}
		stage = graph.Stage()
	})
	if readErr != nil {
		utils.PrintCLIProblem("read SST file", readErr)
		return
	}

	success = true

	interactiveConfig.Stages[stageAlias] = stage
	interactiveConfig.StageAliases = append(interactiveConfig.StageAliases, stageAlias)
	interactiveConfig.StageSources[stageAlias] = filePath

	fmt.Printf("Stage '%s' (file '%s') opened successfully.\n", stageAlias, filePath)
}

func handleImportAP242XML(args []string) {
	if len(args) < 1 {
		fmt.Println("Error: Missing arguments. Usage: importap242xml <file-path>")
		return
	}
	filePath := args[0]

	aliasResult, err := utils.GetAlias(args, "stage")
	if err != nil {
		fmt.Println(err)
		return
	}
	stageAlias := aliasResult.Alias

	// Use defer to confirm alias generation only on success
	success := false
	defer func() {
		if success {
			aliasResult.Confirm()
		}
	}()

	if _, exists := interactiveConfig.Stages[stageAlias]; exists {
		fmt.Printf("Error: Stage alias '%s' already exists.\n", stageAlias)
		return
	}

	// Open XML file
	file, err := os.Open(filePath)
	if err != nil {
		utils.PrintCLIProblem("open file", err)
		return
	}
	defer func() {
		if e := file.Close(); e != nil {
			log.Printf("Error closing file: %v", e)
		}
	}()

	reader := bufio.NewReader(file)

	// Mute both standard log and ap242xmlimport logger
	// Save original outputs
	originalLogOutput := log.Default().Writer()
	originalAp242LoggerOutput := ap242xmlimport.Logger().Writer()

	// Redirect both to null device
	nullDevice, _ := os.Open(os.DevNull)
	log.SetOutput(nullDevice)
	ap242xmlimport.Logger().SetOutput(nullDevice)

	defer func() {
		// Restore original outputs
		log.SetOutput(originalLogOutput)
		ap242xmlimport.Logger().SetOutput(originalAp242LoggerOutput)
		nullDevice.Close()
	}()

	// Import AP242 XML to SST graph
	graph, err := ap242xmlimport.FromXMLReader(reader)
	if err != nil {
		utils.PrintCLIProblem("import AP242 XML", err)
		return
	}

	// Get the stage from the graph
	stage := graph.Stage()

	// Success! Set flag so defer will confirm
	success = true

	interactiveConfig.Stages[stageAlias] = stage
	interactiveConfig.StageAliases = append(interactiveConfig.StageAliases, stageAlias)
	interactiveConfig.StageSources[stageAlias] = filePath

	// Create alias for the graph (remove stage alias flag from args first)
	remainingArgs := utils.RemoveAliasFlag(args)
	graphAliasResult, err := utils.GetAlias(remainingArgs, "namedgraph")
	var graphAlias string
	if err != nil {
		fmt.Printf("Warning: Failed to get graph alias: %v\n", err)
	} else {
		graphAlias = graphAliasResult.Alias
		graphSuccess := false
		defer func() {
			if graphSuccess {
				graphAliasResult.Confirm()
			}
		}()

		if _, exists := interactiveConfig.NamedGraphs[graphAlias]; exists {
			fmt.Printf("Warning: NamedGraph alias '%s' already exists, skipping graph alias creation.\n", graphAlias)
		} else {
			graphSuccess = true
			interactiveConfig.NamedGraphs[graphAlias] = graph
			interactiveConfig.NamedGraphAliases = append(interactiveConfig.NamedGraphAliases, graphAlias)
		}
	}

	fmt.Printf("Stage '%s' (file '%s') opened successfully.\n", stageAlias, filePath)

	if graphAlias != "" {
		if _, exists := interactiveConfig.NamedGraphs[graphAlias]; exists {
			utils.PrintNamedGraphDetails(graphAlias, graph)
		}
	}
}

func handleNamedGraphExportAP242XML(graphAlias string, args []string) {
	graph, exists := interactiveConfig.NamedGraphs[graphAlias]
	if !exists || graph == nil {
		fmt.Printf("Error: NamedGraph alias '%s' not found.\n", graphAlias)
		return
	}
	if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
		fmt.Println("Error: Missing output file. Usage: <namedgraph>.exportap242xml <file>")
		return
	}

	outputFile := strings.TrimSpace(args[0])
	outputFile = utils.EnsureOutputExt(outputFile, ".xml")

	// Check if file exists
	if _, err := os.Stat(outputFile); err == nil {
		fmt.Printf("File '%s' already exists. Overwrite? (y/N): ", outputFile)
		var input string
		fmt.Scanln(&input)
		if strings.ToLower(strings.TrimSpace(input)) != "y" {
			fmt.Println("Aborted.")
			return
		}
	} else if !os.IsNotExist(err) {
		utils.PrintCLIProblem("check file", err)
		return
	}

	f, err := os.Create(outputFile)
	if err != nil {
		utils.PrintCLIProblem("create file", err)
		return
	}
	defer func() {
		if e := f.Close(); e != nil {
			log.Printf("Error closing file: %v", e)
		}
	}()

	// Mute log output during export to suppress log output from ap242xmlexport
	// ap242xmlexport uses logger which outputs to os.Stderr, and also uses standard log.Panic
	// Save original stderr and log output
	originalStderr := os.Stderr
	originalLogOutput := log.Default().Writer()

	// Redirect both stderr and log to null device
	nullDevice, _ := os.Open(os.DevNull)
	os.Stderr = nullDevice
	log.SetOutput(nullDevice)

	defer func() {
		// Restore original outputs
		os.Stderr = originalStderr
		log.SetOutput(originalLogOutput)
		nullDevice.Close()
	}()

	// Export NamedGraph to AP242 XML
	if err := ap242xmlexport.AP242XmlExport(graph, f); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to export AP242 XML: %v\n", err)
		return
	}

	fmt.Printf("AP242 XML successfully written to %s\n", outputFile)
}

func handleImportP21(args []string) {
	if len(args) < 1 {
		fmt.Println("Error: Missing arguments. Usage: importp21 <file-path> [-o <raw-ttl-path>] [-a <stage-alias>]")
		return
	}
	filePath := args[0]

	rawOutputPath, err := utils.ExtractOutputFlag(args)
	if err != nil {
		fmt.Println(err)
		fmt.Println("Error: Missing arguments. Usage: importp21 <file-path> [-o <raw-ttl-path>] [-a <stage-alias>]")
		return
	}

	aliasResult, err := utils.GetAlias(args, "stage")
	if err != nil {
		fmt.Println(err)
		return
	}
	stageAlias := aliasResult.Alias

	// Use defer to confirm alias generation only on success
	success := false
	defer func() {
		if success {
			aliasResult.Confirm()
		}
	}()

	if _, exists := interactiveConfig.Stages[stageAlias]; exists {
		fmt.Printf("Error: Stage alias '%s' already exists.\n", stageAlias)
		return
	}

	// Open P21/STEP file
	file, err := os.Open(filePath)
	if err != nil {
		utils.PrintCLIProblem("open file", err)
		return
	}
	defer func() {
		if e := file.Close(); e != nil {
			log.Printf("Error closing file: %v", e)
		}
	}()

	var graph sst.NamedGraph
	if rawOutputPath != "" {
		graph, err = p21.ParseRaw(bufio.NewReader(file), log.Default())
		if err != nil {
			utils.PrintCLIProblem("import P21 file", err)
			return
		}
		if err := writeP21Turtle(rawOutputPath, graph); err != nil {
			utils.PrintCLIProblem("write raw P21 graph", err)
			return
		}
		if _, err := file.Seek(0, 0); err != nil {
			utils.PrintCLIProblem("rewind P21 file", err)
			return
		}
	}
	graph, err = p21.Parse(bufio.NewReader(file), log.Default())
	if err != nil {
		utils.PrintCLIProblem("convert P21 file", err)
		return
	}

	if graph == nil {
		fmt.Printf("Error: Failed to import P21/STEP: graph is nil\n")
		return
	}

	stage := graph.Stage()
	if stage == nil {
		fmt.Printf("Error: Failed to import P21/STEP: stage is nil\n")
		return
	}

	// Success! Set flag so defer will confirm
	success = true

	interactiveConfig.Stages[stageAlias] = stage
	interactiveConfig.StageAliases = append(interactiveConfig.StageAliases, stageAlias)
	interactiveConfig.StageSources[stageAlias] = filePath

	// Create alias for the graph (remove stage alias flag from args first)
	remainingArgs := utils.ArgsWithoutOutputFlag(utils.RemoveAliasFlag(args))
	graphAliasResult, err := utils.GetAlias(remainingArgs, "namedgraph")
	var graphAlias string
	if err != nil {
		fmt.Printf("Warning: Failed to get graph alias: %v\n", err)
	} else {
		graphAlias = graphAliasResult.Alias
		graphSuccess := false
		defer func() {
			if graphSuccess {
				graphAliasResult.Confirm()
			}
		}()

		if _, exists := interactiveConfig.NamedGraphs[graphAlias]; exists {
			fmt.Printf("Warning: NamedGraph alias '%s' already exists, skipping graph alias creation.\n", graphAlias)
		} else {
			graphSuccess = true
			interactiveConfig.NamedGraphs[graphAlias] = graph
			interactiveConfig.NamedGraphAliases = append(interactiveConfig.NamedGraphAliases, graphAlias)
		}
	}

	fmt.Printf("Stage '%s' (file '%s') opened successfully.\n", stageAlias, filePath)
	if rawOutputPath != "" {
		fmt.Printf("Raw P21 graph written to %s\n", rawOutputPath)
	}

	if graphAlias != "" {
		if _, exists := interactiveConfig.NamedGraphs[graphAlias]; exists {
			utils.PrintNamedGraphDetails(graphAlias, graph)
		}
	}
}

func writeP21Turtle(path string, graph sst.NamedGraph) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	return graph.RdfWrite(file, sst.RdfFormatTurtle)
}

func handleImportSVG(args []string) {
	if len(args) < 1 {
		fmt.Println("Error: Missing arguments. Usage: importsvg <file-path> [-a <stage-alias>]")
		return
	}
	filePath := args[0]

	aliasResult, err := utils.GetAlias(args, "stage")
	if err != nil {
		fmt.Println(err)
		return
	}
	stageAlias := aliasResult.Alias

	// Use defer to confirm alias generation only on success
	success := false
	defer func() {
		if success {
			aliasResult.Confirm()
		}
	}()

	if _, exists := interactiveConfig.Stages[stageAlias]; exists {
		fmt.Printf("Error: Stage alias '%s' already exists.\n", stageAlias)
		return
	}

	// Open SVG file
	file, err := os.Open(filePath)
	if err != nil {
		utils.PrintCLIProblem("open file", err)
		return
	}
	defer func() {
		if e := file.Close(); e != nil {
			log.Printf("Error closing file: %v", e)
		}
	}()

	reader := bufio.NewReader(file)

	// tools/svg_to_sst currently emits some debug output via both the log package and fmt.Printf.
	// Redirect both temporarily to keep interactive output readable.
	nullDevice, _ := os.Open(os.DevNull)
	originalStdout := os.Stdout
	originalLogOutput := log.Default().Writer()
	restoreDone := false
	defer func() {
		if !restoreDone {
			os.Stdout = originalStdout
			log.SetOutput(originalLogOutput)
			_ = nullDevice.Close()
		}
	}()

	os.Stdout = nullDevice
	log.SetOutput(nullDevice)
	graph, convertErr := svgtosst.ConvertSvgToGraph(reader, "")
	os.Stdout = originalStdout
	log.SetOutput(originalLogOutput)
	_ = nullDevice.Close()
	restoreDone = true

	if convertErr != nil {
		utils.PrintCLIProblem("convert SVG", convertErr)
		return
	}
	if graph == nil {
		fmt.Println("Error: Failed to convert SVG to SST: graph is nil")
		return
	}

	stage := graph.Stage()
	if stage == nil {
		fmt.Println("Error: Failed to convert SVG to SST: stage is nil")
		return
	}

	// Success! Set flag so defer will confirm
	success = true
	interactiveConfig.Stages[stageAlias] = stage
	interactiveConfig.StageAliases = append(interactiveConfig.StageAliases, stageAlias)
	interactiveConfig.StageSources[stageAlias] = filePath

	// Create alias for the graph (remove stage alias flag from args first)
	remainingArgs := utils.RemoveAliasFlag(args)
	graphAliasResult, err := utils.GetAlias(remainingArgs, "namedgraph")
	var graphAlias string
	if err != nil {
		fmt.Printf("Warning: Failed to get graph alias: %v\n", err)
	} else {
		graphAlias = graphAliasResult.Alias
		graphSuccess := false
		defer func() {
			if graphSuccess {
				graphAliasResult.Confirm()
			}
		}()

		if _, exists := interactiveConfig.NamedGraphs[graphAlias]; exists {
			fmt.Printf("Warning: NamedGraph alias '%s' already exists, skipping graph alias creation.\n", graphAlias)
		} else {
			graphSuccess = true
			interactiveConfig.NamedGraphs[graphAlias] = graph
			interactiveConfig.NamedGraphAliases = append(interactiveConfig.NamedGraphAliases, graphAlias)
		}
	}

	fmt.Printf("Stage '%s' (file '%s') opened successfully.\n", stageAlias, filePath)

	if graphAlias != "" {
		if _, exists := interactiveConfig.NamedGraphs[graphAlias]; exists {
			utils.PrintNamedGraphDetails(graphAlias, graph)
		}
	}
}

func handleClone(sourceAlias string, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: <source-repo-alias>.clone <target-directory> [target-alias]")
		fmt.Println("Example: r1.clone ./my-repo")
		fmt.Println("Example: r1.clone ./my-repo r2")
		return
	}

	// Get source repository
	sourceRepo, ok := interactiveConfig.Repositories[sourceAlias]
	if !ok {
		fmt.Printf("Error: Source repository alias '%s' not found.\n", sourceAlias)
		return
	}

	targetDir := args[0]

	// Get target alias if provided, otherwise generate automatically
	var targetAlias string
	var aliasResult utils.AliasResult
	if len(args) >= 2 {
		targetAlias = args[1]
	} else {
		// Generate alias automatically
		var err error
		aliasResult, err = utils.GetAlias([]string{}, "repository")
		if err != nil {
			fmt.Println(err)
			return
		}
		targetAlias = aliasResult.Alias
	}

	// Check if target alias already exists
	if _, exists := interactiveConfig.Repositories[targetAlias]; exists {
		fmt.Printf("Error: Repository alias '%s' already exists.\n", targetAlias)
		return
	}

	// Convert to absolute path
	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		utils.PrintCLIProblem("resolve path", err)
		return
	}

	// Check if target directory already exists
	if _, err := os.Stat(absTargetDir); err == nil {
		fmt.Printf("Error: Target directory '%s' already exists.\n", absTargetDir)
		return
	}

	// Get auth context for source repository
	sourceCtx := utils.GetAuthContext(sourceRepo, interactiveConfig.AuthContexts)

	// Create target local repository
	fmt.Printf("Creating local repository at %s...\n", absTargetDir)
	targetRepo, err := sst.CreateLocalRepository(absTargetDir, "default@semanticstep.net", "default", true)
	if err != nil {
		utils.PrintCLIProblem("create local repository", err)
		return
	}

	// Sync from source to target
	fmt.Printf("Syncing data from repository '%s' to '%s'...\n", sourceAlias, absTargetDir)
	err = targetRepo.SyncFrom(sourceCtx, sourceRepo)

	if err != nil {
		utils.PrintCLIProblem("sync repository", err)
		return
	}

	fmt.Printf("Successfully cloned repository to '%s'.\n", absTargetDir)

	targetRepo.Close()

	// Open the cloned repository in interactive mode
	fmt.Printf("Opening cloned repository as '%s'...\n", targetAlias)
	openedRepo, err := sst.OpenLocalRepository(absTargetDir, "default@semanticstep.net", "default")
	if err != nil {
		utils.PrintCLIProblem("open cloned repository", err)
		return
	}

	// Register the repository in interactive config
	interactiveConfig.Repositories[targetAlias] = openedRepo
	interactiveConfig.RepositoryLocations[targetAlias] = absTargetDir
	interactiveConfig.RepositoryTypes[targetAlias] = "local"
	interactiveConfig.RepositoryAliases = append(interactiveConfig.RepositoryAliases, targetAlias)

	fmt.Printf("Repository '%s' opened successfully.\n", targetAlias)

	// Confirm alias generation if it was auto-generated
	if len(args) < 2 {
		aliasResult.Confirm()
	}
}
