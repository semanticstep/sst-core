// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package interactive

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	fs "github.com/relab/wrfs"
	"github.com/semanticstep/sst-core/cli/cmd/utils"
	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/tools/validate"
	"go.uber.org/zap"
)

func handleListNamedGraphs(stageAlias string) {
	stage, exists := interactiveConfig.Stages[stageAlias]
	if !exists {
		fmt.Printf("Error: Stage alias '%s' not found.\n", stageAlias)
		return
	}

	namedGraphs := stage.NamedGraphs()
	if len(namedGraphs) == 0 {
		fmt.Printf("No named graphs found in Stage '%s'.\n", stageAlias)
		return
	}

	for _, graph := range namedGraphs {
		fmt.Printf("- IRI: %s\n", graph.IRI())
	}
}

func handleListReferencedNamedGraphs(stageAlias string) {
	stage, exists := interactiveConfig.Stages[stageAlias]
	if !exists {
		fmt.Printf("Error: Stage alias '%s' not found.\n", stageAlias)
		return
	}

	referencedGraphs := stage.ReferencedGraphs()
	if len(referencedGraphs) == 0 {
		fmt.Printf("No referenced named graphs found in Stage '%s'.\n", stageAlias)
		return
	}

	for _, graph := range referencedGraphs {
		fmt.Printf("- IRI: %s\n", graph.IRI())
	}
}

// handleNamedGraph resolves a NamedGraph by its graph IRI.
func handleNamedGraph(stageAlias string, args []string) {
	aliasResult, err := utils.GetAlias(args, "namedgraph")
	if err != nil {
		fmt.Println(err)
		return
	}
	graphAlias := aliasResult.Alias

	success := false
	defer func() {
		if success {
			aliasResult.Confirm()
		}
	}()

	iriArgs := utils.RemoveAliasFlag(args)
	if len(iriArgs) < 1 {
		fmt.Println("Error: Missing IRI. Use '<stage>.namedgraph <iri> [-a <alias>]'.")
		return
	}

	iri := sst.IRI(iriArgs[0])

	stage, exists := interactiveConfig.Stages[stageAlias]
	if !exists {
		fmt.Printf("Error: Stage '%s' not found.\n", stageAlias)
		return
	}

	namedGraph := stage.NamedGraph(iri)
	if namedGraph == nil {
		fmt.Printf("Error: NamedGraph with IRI '%s' not found.\n", iri)
		return
	}

	success = true

	interactiveConfig.NamedGraphs[graphAlias] = namedGraph
	interactiveConfig.NamedGraphAliases = append(interactiveConfig.NamedGraphAliases, graphAlias)

	utils.PrintNamedGraphDetails(graphAlias, namedGraph)
}

// <target-stage-alias>.moveandmerge <source-stage-alias>
func handleMoveAndMerge(targetAlias string, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: <target-stage-alias>.moveandmerge <source-stage-alias>")
		return
	}
	sourceAlias := args[0]

	// 1) Resolve target stage
	targetStage, ok := interactiveConfig.Stages[targetAlias]
	if !ok {
		fmt.Printf("Stage alias '%s' not found.\n", targetAlias)
		return
	}
	if targetStage == nil || !targetStage.IsValid() {
		fmt.Printf("Stage '%s' is not valid.\n", targetAlias)
		return
	}
	if targetStage.Repository() == nil {
		fmt.Printf("Stage '%s' is not linked to a repository.\n", targetAlias)
		return
	}

	// 2) Resolve source stage
	sourceStage, ok := interactiveConfig.Stages[sourceAlias]
	if !ok {
		fmt.Printf("Source stage alias '%s' not found.\n", sourceAlias)
		return
	}
	if sourceStage == nil || !sourceStage.IsValid() {
		fmt.Printf("Source stage '%s' is not valid.\n", sourceAlias)
		return
	}

	// 3) Auth context comes from the target's repository
	ctx := utils.GetAuthContext(targetStage.Repository(), interactiveConfig.AuthContexts)

	// 4) MoveAndMerge
	if _, err := targetStage.MoveAndMerge(ctx, sourceStage); err != nil {
		utils.PrintCLIProblem("move and merge", err)
		return
	}

	// 5) Cleanup source stage
	cleanupStageFromConfig(sourceAlias, sourceStage)

	fmt.Printf("Merged stage '%s' into '%s'.\n", sourceAlias, targetAlias)
}

// <to-stage-alias>.alignhistory <from-stage-alias>
// Copies repository pointer and checkout metadata from from into to (see sst.Stage.AlignHistory).
func handleAlignHistory(toAlias string, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: <to-stage-alias>.alignhistory <from-stage-alias>")
		return
	}
	fromAlias := args[0]

	toStage, ok := interactiveConfig.Stages[toAlias]
	if !ok {
		fmt.Printf("Stage alias '%s' not found.\n", toAlias)
		return
	}
	if toStage == nil || !toStage.IsValid() {
		fmt.Printf("Stage '%s' is not valid.\n", toAlias)
		return
	}

	fromStage, ok := interactiveConfig.Stages[fromAlias]
	if !ok {
		fmt.Printf("Source stage alias '%s' not found.\n", fromAlias)
		return
	}
	if fromStage == nil || !fromStage.IsValid() {
		fmt.Printf("Source stage '%s' is not valid.\n", fromAlias)
		return
	}
	if fromStage.Repository() == nil {
		fmt.Printf("Source stage '%s' is not linked to a repository.\n", fromAlias)
		return
	}

	toStage.AlignHistory(fromStage)

	fmt.Printf("Aligned history from stage '%s' onto '%s'.\n", fromAlias, toAlias)
}

// <stage-alias>.rdfwrite <output-file>
func handleStageRdfWrite(stageAlias string, args []string) {
	stage, exists := interactiveConfig.Stages[stageAlias]
	if !exists {
		fmt.Printf("Error: Stage alias '%s' not found.\n", stageAlias)
		return
	}
	if stage == nil {
		fmt.Printf("Error: Stage '%s' is nil.\n", stageAlias)
		return
	}
	if !stage.IsValid() {
		fmt.Printf("Error: Stage '%s' is not valid.\n", stageAlias)
		return
	}

	if len(args) == 0 {
		fmt.Println("No file path provided. Usage: <stage>.rdfwrite <filename>")
		return
	}

	fileName := args[0]
	fileName = utils.EnsureOutputExt(fileName, ".trig")

	// Check if file exists
	if _, err := os.Stat(fileName); err == nil {
		fmt.Printf("File '%s' already exists. Overwrite? (y/N): ", fileName)
		var input string
		fmt.Scanln(&input)
		if strings.ToLower(strings.TrimSpace(input)) != "y" {
			fmt.Println("Aborted.")
			return
		}
	} else if !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error checking file '%s': %v\n", fileName, err)
		return
	}

	f, err := os.Create(fileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create file '%s': %v\n", fileName, err)
		return
	}
	defer f.Close()

	var writeErr error
	utils.ShowLoadingIndicator(fmt.Sprintf("Writing TriG for '%s'", stageAlias), func() {
		writeErr = stage.RdfWrite(f, sst.RdfFormatTriG)
	})
	if writeErr != nil {
		fmt.Fprintf(os.Stderr, "\nError writing TriG: %v\n", writeErr)
		return
	}

	fmt.Printf("TriG successfully written to %s\n", fileName)
}

// <stage-alias>.writesstfilesdirectory <directory>
func handleWriteSstFilesDirectory(stageAlias string, args []string) {
	stage, exists := interactiveConfig.Stages[stageAlias]
	if !exists {
		fmt.Printf("Error: Stage alias '%s' not found.\n", stageAlias)
		return
	}
	if stage == nil {
		fmt.Printf("Error: Stage '%s' is nil.\n", stageAlias)
		return
	}
	if !stage.IsValid() {
		fmt.Printf("Error: Stage '%s' is not valid.\n", stageAlias)
		return
	}

	if len(args) == 0 {
		fmt.Println("Usage: <stage>.writesstfilesdirectory <directory>")
		return
	}

	dir := args[0]
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			utils.PrintCLIProblem("create directory", err)
			return
		}
	} else if err != nil {
		utils.PrintCLIProblem("access directory", err)
		return
	} else if !info.IsDir() {
		fmt.Printf("Error: path is not a directory: %s\n", dir)
		return
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		absDir = dir
	}

	var writeErr error
	utils.ShowLoadingIndicator(fmt.Sprintf("Writing SST files for '%s'", stageAlias), func() {
		writeErr = stage.WriteSstFilesDirectory(fs.DirFS(dir))
	})
	if writeErr != nil {
		if errors.Is(writeErr, sst.ErrDirectoryExpectedAsBasePath) {
			fmt.Fprintf(os.Stderr, "Error: path must be an existing directory: %s\n", dir)
			return
		}
		utils.PrintCLIProblem("write SST files", writeErr)
		return
	}

	fileCount := 0
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				fileCount++
			}
		}
	}
	fmt.Printf("SST files written to %s (%d file(s))\n", absDir, fileCount)
}

// <stage-alias>.trig
func handleStageTriG(stageAlias string, args []string) {
	stage, exists := interactiveConfig.Stages[stageAlias]
	if !exists {
		fmt.Printf("Error: Stage alias '%s' not found.\n", stageAlias)
		return
	}
	if stage == nil {
		fmt.Printf("Error: Stage '%s' is nil.\n", stageAlias)
		return
	}
	if !stage.IsValid() {
		fmt.Printf("Error: Stage '%s' is not valid.\n", stageAlias)
		return
	}

	if len(args) != 0 {
		fmt.Println("Usage: <stage>.trig")
		return
	}

	fmt.Printf("--- RDF Output for Stage '%s' ---\n", stageAlias)
	if err := stage.RdfWrite(os.Stdout, sst.RdfFormatTriG); err != nil {
		fmt.Fprintf(os.Stderr, "\nError writing TriG to console: %v\n", err)
		return
	}
	fmt.Println("\n--- End of RDF Output ---")
}

// cleanupStageFromConfig removes a stage and all its related metadata from the interactive config.
// This includes its branch/commit/source entries, as well as any NamedGraph aliases that belong to it.
func cleanupStageFromConfig(stageAlias string, st sst.Stage) {
	// 1) Basic cleanup
	delete(interactiveConfig.Stages, stageAlias)
	interactiveConfig.StageAliases = utils.RemoveAlias(interactiveConfig.StageAliases, stageAlias)
	delete(interactiveConfig.StageBranches, stageAlias)
	delete(interactiveConfig.StageCommits, stageAlias)
	delete(interactiveConfig.StageRevisions, stageAlias)
	delete(interactiveConfig.StageSources, stageAlias)

	// // 2) Clean up NamedGraph aliases belonging to this stage (safe by matching NG.ID)
	// ngIDs := make(map[uuid.UUID]struct{})
	// for _, ng := range st.NamedGraphs() {
	// 	ngIDs[ng.ID()] = struct{}{}
	// }

	// if len(ngIDs) > 0 {
	// 	newAliases := make([]string, 0, len(interactiveConfig.NamedGraphAliases))
	// 	for _, a := range interactiveConfig.NamedGraphAliases {
	// 		ng := interactiveConfig.NamedGraphs[a]
	// 		if ng != nil {
	// 			if _, inStage := ngIDs[ng.ID()]; inStage {
	// 				// Remove aliases that point to NamedGraphs contained in the deleted stage.
	// 				delete(interactiveConfig.NamedGraphs, a)
	// 				continue
	// 			}
	// 		}
	// 		newAliases = append(newAliases, a)
	// 	}
	// 	interactiveConfig.NamedGraphAliases = newAliases
	// }

	// // 3) IBNode aliases are not removed for now,
	// // since there is no node→graph mapping to ensure safe cleanup.
}

func handleStageCommit(stageAlias string, args []string) {

	if sst.GlobalLogger != nil {
		cfg := zap.NewProductionConfig()
		cfg.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
		logger, _ := cfg.Build()
		sst.GlobalLogger = logger
	}

	if len(args) < 1 {
		fmt.Println(`Usage: <stage-alias>.commit <message> [branch]  (quote message if it contains spaces)`)
		return
	}
	if len(args) > 2 {
		fmt.Println(`Usage: <stage-alias>.commit <message> [branch]  (quote message if it contains spaces)`)
		fmt.Println("Error: too many arguments.")
		return
	}

	stage, exists := interactiveConfig.Stages[stageAlias]
	if !exists {
		fmt.Printf("Error: stage '%s' not found.\n", stageAlias)
		return
	}
	if stage == nil {
		fmt.Printf("Error: stage '%s' is nil.\n", stageAlias)
		return
	}
	if stage.Repository() == nil {
		fmt.Printf("Error: stage '%s' is not linked to a repository.\n", stageAlias)
		return
	}

	message := strings.TrimSpace(args[0])
	if message == "" {
		fmt.Println("Error: commit message cannot be empty.")
		return
	}

	branchName := ""
	if len(args) >= 2 {
		branchName = strings.TrimSpace(args[1])
	}

	ctx := utils.GetAuthContext(stage.Repository(), interactiveConfig.AuthContexts)

	commitHash, _, err := stage.Commit(ctx, message, branchName)
	if err != nil {
		utils.PrintCLIProblem("commit", err)
		return
	}

	fmt.Printf("Commit: %s\n", commitHash.String())
}

func handleStageValidate(stageAlias string, args []string) {
	stage, ok := interactiveConfig.Stages[stageAlias]
	if !ok {
		fmt.Printf("Stage alias '%s' not found.\n", stageAlias)
		return
	}

	if len(args) != 0 {
		fmt.Printf("Usage: %s.validate\n", stageAlias)
		return
	}

	// Suppress validation process output (ERROR, starting, finished messages)
	utils.MuteStdout()

	report, err := validate.Validate(stage, validate.KindRdfType, validate.KindDomainRange)
	utils.RestoreStdout()

	if err != nil {
		utils.PrintCLIProblem("validate stage", err)
		return
	}
	fmt.Print(report.FormatHumanReadable())
}
