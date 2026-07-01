// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package interactive

import (
	"fmt"
	"os"
	"sort"

	"github.com/blevesearch/bleve/v2"
	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/cli/cmd/utils"
)

func handleCommits(datasetAlias, command string, args []string) {
	// check if dataset exists
	dataset, exists := interactiveConfig.Datasets[datasetAlias]
	if !exists {
		fmt.Printf("Error: Dataset alias '%s' not found.\n", datasetAlias)
		return
	}

	authCtx := utils.GetAuthContext(dataset.Repository(), interactiveConfig.AuthContexts)

	switch command {
	case "listcommits":
		var showDetails bool
		for _, arg := range args {
			if arg == "--details" {
				showDetails = true
				break
			}
		}

		commits, err := utils.ListCommitsEntryHashes(authCtx, dataset)
		if err != nil {
			utils.PrintCLIProblem("list commits", err)
			return
		}

		if len(commits) == 0 {
			fmt.Println("No commits found in this dataset.")
			return
		}

		visited := make(map[string]bool)

		for _, commit := range commits {
			if showDetails {
				utils.ListCommitHistoryDetailed(dataset, commit, visited, authCtx)
			} else {
				utils.ListCommitHistoryHashOnly(dataset, commit, visited, authCtx)
			}
		}
		return

	case "commitdetailsbyhash":
		if len(args) == 0 {
			fmt.Println("Error: Missing commit hash.")
			return
		}
		hashInput := args[0]

		hashBytes, err := sst.StringToHash(hashInput)
		if err != nil {
			fmt.Println(utils.FormatCLIProblem("parse commit hash", "invalid commit hash"))
			return
		}

		commitDetails, err := dataset.CommitDetailsByHash(authCtx, hashBytes)
		if err != nil {
			utils.PrintCLIProblem("get commit details", err)
			return
		}

		utils.PrintCommitDetails(commitDetails)
		return

	case "commitdetailsbybranch":
		if len(args) == 0 {
			fmt.Println("Error: Missing branch name.")
			return
		}
		branch := args[0]

		// Catch potential panic (e.g. gRPC returning panic instead of error)
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "Error retrieving branch '%s': %v\n", branch, r)
			}
		}()

		commitDetails, err := dataset.CommitDetailsByBranch(authCtx, branch)
		if err != nil {
			utils.PrintCLIProblem("get commit details by branch", err)
			return
		}

		utils.PrintCommitDetails(commitDetails)
		return

	default:
		fmt.Println("Unknown commit command. Available commands: listcommits, commitdetailsbyhash <hash>, commitdetailsbybranch <branch>")
	}
}

func handleCheckoutCommit(datasetAlias string, args []string) {
	dataset, exists := interactiveConfig.Datasets[datasetAlias]
	if !exists {
		fmt.Printf("Error: Dataset alias '%s' not found.\n", datasetAlias)
		return
	}

	if len(args) < 1 {
		fmt.Println("Error: Missing arguments. Use '<dataset-alias>.CheckoutCommit <commit-id>'.")
		return
	}

	commitID := args[0]
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

	hash, err := sst.StringToHash(commitID)
	if err != nil {
		fmt.Println(utils.FormatCLIProblem("parse commit hash", "invalid commit hash"))
		return
	}

	if _, exists := interactiveConfig.Stages[stageAlias]; exists {
		fmt.Printf("Error: Stage alias '%s' already exists.\n", stageAlias)
		return
	}

	authCtx := utils.GetAuthContext(dataset.Repository(), interactiveConfig.AuthContexts)

	stage, err := dataset.CheckoutCommit(authCtx, hash, sst.DefaultTriplexMode)
	if err != nil {
		utils.PrintCLIProblem("checkout commit", err)
		return
	}

	// Success! Set flag so defer will confirm
	success = true

	interactiveConfig.Stages[stageAlias] = stage
	interactiveConfig.StageAliases = append(interactiveConfig.StageAliases, stageAlias)

	interactiveConfig.StageCommits[stageAlias] = hash

	fmt.Printf("Stage '%s' (commit %s) opened successfully.\n", stageAlias, commitID)
}

func handleCheckoutRevision(datasetAlias string, args []string) {
	dataset, exists := interactiveConfig.Datasets[datasetAlias]
	if !exists {
		fmt.Printf("Error: Dataset alias '%s' not found.\n", datasetAlias)
		return
	}

	if len(args) < 1 {
		fmt.Println("Error: Missing arguments. Use '<dataset-alias>.checkoutrevision <dataset-revision-hash>'.")
		return
	}

	revisionID := args[0]
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

	hash, err := sst.StringToHash(revisionID)
	if err != nil {
		fmt.Println(utils.FormatCLIProblem("parse revision hash", "invalid revision hash"))
		return
	}

	if _, exists := interactiveConfig.Stages[stageAlias]; exists {
		fmt.Printf("Error: Stage alias '%s' already exists.\n", stageAlias)
		return
	}

	authCtx := utils.GetAuthContext(dataset.Repository(), interactiveConfig.AuthContexts)

	stage, err := dataset.CheckoutRevision(authCtx, hash, sst.DefaultTriplexMode)
	if err != nil {
		utils.PrintCLIProblem("checkout revision", err)
		return
	}

	success = true

	interactiveConfig.Stages[stageAlias] = stage
	interactiveConfig.StageAliases = append(interactiveConfig.StageAliases, stageAlias)
	interactiveConfig.StageRevisions[stageAlias] = hash

	fmt.Printf("Stage '%s' (revision %s) opened successfully.\n", stageAlias, revisionID)
}

func handleCheckoutBranch(datasetAlias string, args []string) {
	dataset, exists := interactiveConfig.Datasets[datasetAlias]
	if !exists {
		fmt.Printf("Error: Dataset alias '%s' not found.\n", datasetAlias)
		return
	}

	if len(args) < 1 {
		fmt.Println("Error: Missing arguments. Use '<dataset-alias>.CheckoutBranch <branchName>'.")
		return
	}

	branchName := args[0]
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

	authCtx := utils.GetAuthContext(dataset.Repository(), interactiveConfig.AuthContexts)

	stage, err := dataset.CheckoutBranch(authCtx, branchName, sst.DefaultTriplexMode)

	if err != nil {
		utils.PrintCLIProblem("checkout branch", err)
		return
	}

	// Success! Set flag so defer will confirm
	success = true

	interactiveConfig.Stages[stageAlias] = stage
	interactiveConfig.StageAliases = append(interactiveConfig.StageAliases, stageAlias)

	interactiveConfig.StageBranches[stageAlias] = branchName

	fmt.Printf("Stage '%s' opened successfully.\n", stageAlias)
}

func handleSetBranchCommit(datasetAlias string, args []string) {
	dataset, exists := interactiveConfig.Datasets[datasetAlias]
	if !exists {
		fmt.Printf("Error: Dataset alias '%s' not found.\n", datasetAlias)
		return
	}

	if len(args) < 2 {
		fmt.Println("Usage: <dataset-alias>.setbranchcommit <commit-hash> <branch-name>")
		return
	}

	commitInput := args[0]
	branchName := args[1]

	hash, err := sst.StringToHash(commitInput)
	if err != nil {
		fmt.Println(utils.FormatCLIProblem("parse commit hash", "invalid commit hash"))
		return
	}

	authCtx := utils.GetAuthContext(dataset.Repository(), interactiveConfig.AuthContexts)

	if err := dataset.SetBranchCommit(authCtx, hash, branchName); err != nil {
		utils.PrintCLIProblem("set branch commit", err)
		return
	}

	fmt.Printf("SetBranchCommit successful. Branch '%s' now points to commit %s.\n", branchName, hash)
}

func handleSetBranchRevision(datasetAlias string, args []string) {
	dataset, exists := interactiveConfig.Datasets[datasetAlias]
	if !exists {
		fmt.Printf("Error: Dataset alias '%s' not found.\n", datasetAlias)
		return
	}

	if len(args) < 2 {
		fmt.Println("Usage: <dataset-alias>.setbranchrevision <dataset-revision-hash> <branch-name>")
		return
	}

	revisionInput := args[0]
	branchName := args[1]

	hash, err := sst.StringToHash(revisionInput)
	if err != nil {
		fmt.Println(utils.FormatCLIProblem("parse revision hash", "invalid revision hash"))
		return
	}

	authCtx := utils.GetAuthContext(dataset.Repository(), interactiveConfig.AuthContexts)

	if err := dataset.SetBranchRevision(authCtx, hash, branchName); err != nil {
		utils.PrintCLIProblem("set branch revision", err)
		return
	}

	fmt.Printf("SetBranchRevision successful. Branch '%s' now points to revision %s.\n", branchName, hash)
}

func handleRemoveBranch(datasetAlias string, args []string) {
	dataset, exists := interactiveConfig.Datasets[datasetAlias]
	if !exists {
		fmt.Printf("Error: Dataset alias '%s' not found.\n", datasetAlias)
		return
	}

	if len(args) < 1 {
		fmt.Println("Usage: <dataset-alias>.removebranch <branch-name>")
		return
	}

	branchName := args[0]

	authCtx := utils.GetAuthContext(dataset.Repository(), interactiveConfig.AuthContexts)

	if err := dataset.RemoveBranch(authCtx, branchName); err != nil {
		utils.PrintCLIProblem("remove branch", err)
		return
	}

	fmt.Printf("RemoveBranch successful. Branch '%s' removed.\n", branchName)
}

func handleListField(alias string) {
	repo, exists := interactiveConfig.Repositories[alias]
	if !exists {
		fmt.Printf("Error: Repository alias '%s' not found.\n", alias)
		return
	}

	authCtx := utils.GetAuthContext(repo, interactiveConfig.AuthContexts)

	bleveIndex := repo.Bleve()
	if bleveIndex == nil {
		fmt.Printf("Error: Repository '%s' does not have an index.\n", alias)
		return
	}

	req := bleve.NewSearchRequest(bleve.NewMatchAllQuery())
	req.Size = 1000
	req.Fields = []string{"*"}

	result, err := bleveIndex.SearchInContext(authCtx, req)
	if err != nil {
		utils.PrintCLIProblem("list index fields", err)
		return
	}

	fieldSet := map[string]struct{}{}
	for _, hit := range result.Hits {
		for field := range hit.Fields {
			fieldSet[field] = struct{}{}
		}
	}

	if len(fieldSet) == 0 {
		fmt.Println("No indexed fields found.")
		return
	}

	fmt.Println("- Available searchable fields:")
	for field := range fieldSet {
		fmt.Printf("  - %s\n", field)
	}
}

func handleDiff(datasetAlias string, args []string) {
	dataset, exists := interactiveConfig.Datasets[datasetAlias]
	if !exists {
		fmt.Printf("Error: Dataset alias '%s' not found.\n", datasetAlias)
		return
	}
	if len(args) != 2 {
		fmt.Println("Usage: <dataset-alias>.diff <NG-Revision-Hash1> <NG-Revision-Hash2>")
		return
	}

	hash1, err := sst.StringToHash(args[0])
	if err != nil {
		fmt.Println(utils.FormatCLIProblem("compute diff", "invalid hash"))
		return
	}
	hash2, err := sst.StringToHash(args[1])
	if err != nil {
		fmt.Println(utils.FormatCLIProblem("compute diff", "invalid hash"))
		return
	}

	repo := dataset.Repository()
	ctx := utils.GetAuthContext(repo, interactiveConfig.AuthContexts)

	tris, err := utils.SstDiffTriples(ctx, repo, hash1, hash2, true)
	if err != nil {
		utils.PrintCLIProblem("compute diff", err)
		return
	}
	fmt.Println("- DiffTriples:")
	utils.PrintDiffTriples(tris)
}

// handleHistory shows the commit history graph for the given dataset alias.
// Usage: <dataset-alias>.history
func handleHistory(datasetAlias string, args []string) {
	dataset, exists := interactiveConfig.Datasets[datasetAlias]
	if !exists {
		fmt.Printf("Error: Dataset alias '%s' not found.\n", datasetAlias)
		return
	}
	if len(args) != 0 {
		fmt.Println("Usage: <dataset-alias>.history")
		return
	}

	repo := dataset.Repository()
	if repo == nil {
		fmt.Printf("Error: Dataset '%s' is not linked to a repository.\n", datasetAlias)
		return
	}

	ctx := utils.GetAuthContext(repo, interactiveConfig.AuthContexts)
	ngIRI := dataset.IRI()

	_ = queryHistoryBranches(ngIRI, repo, ctx)
}

// handleBranches shows all branches for the given dataset alias.
// Usage: <dataset-alias>.branches
func handleBranches(datasetAlias string, args []string) {
	dataset, exists := interactiveConfig.Datasets[datasetAlias]
	if !exists {
		fmt.Printf("Error: Dataset alias '%s' not found.\n", datasetAlias)
		return
	}
	if len(args) != 0 {
		fmt.Println("Usage: <dataset-alias>.branches")
		return
	}

	authCtx := utils.GetAuthContext(dataset.Repository(), interactiveConfig.AuthContexts)

	branches, err := dataset.Branches(authCtx)

	if err != nil {
		utils.PrintCLIProblem("list branches", err)
		return
	}

	if len(branches) == 0 {
		fmt.Println("No branches found in this dataset.")
		return
	}

	// Sort branches by name for consistent output
	var branchNames []string
	for branchName := range branches {
		branchNames = append(branchNames, branchName)
	}
	sort.Strings(branchNames)

	fmt.Println("- Branches:")
	for _, branchName := range branchNames {
		fmt.Printf("  - %s: %s\n", branchName, branches[branchName])
	}
}

// handleLeafCommits shows all leaf commits for the given dataset alias.
// Usage: <dataset-alias>.leafcommits
func handleLeafCommits(datasetAlias string, args []string) {
	dataset, exists := interactiveConfig.Datasets[datasetAlias]
	if !exists {
		fmt.Printf("Error: Dataset alias '%s' not found.\n", datasetAlias)
		return
	}
	if len(args) != 0 {
		fmt.Println("Usage: <dataset-alias>.leafcommits")
		return
	}

	authCtx := utils.GetAuthContext(dataset.Repository(), interactiveConfig.AuthContexts)

	leafCommits, err := dataset.LeafCommits(authCtx)
	if err != nil {
		utils.PrintCLIProblem("list leaf commits", err)
		return
	}

	if len(leafCommits) == 0 {
		fmt.Println("No leaf commits found in this dataset.")
		return
	}

	fmt.Println("- Leaf Commits:")
	for _, commit := range leafCommits {
		fmt.Printf("  - %s\n", commit)
	}
}
