// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package interactive

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/blevesearch/bleve/v2"
	"github.com/google/uuid"
	"github.com/semanticstep/sst-core/cli/cmd/utils"
	"github.com/semanticstep/sst-core/defaultderive"
	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/sstauth"
	"go.etcd.io/bbolt"
)

// handleClose handles closing a repository or SuperRepository based on its alias
func handleClose(alias string) {
	// Check if alias is a SuperRepository
	if _, exists := interactiveConfig.SuperRepositories[alias]; exists {
		handleCloseSuperRepository(alias)
		return
	}

	// close Repository
	if _, exists := interactiveConfig.Repositories[alias]; exists {
		// Check if the repository exists
		repository, exists := interactiveConfig.Repositories[alias]
		if !exists {
			fmt.Printf("Error: No repository found with alias '%s'.\n", alias)
			return
		}

		// Close the repository
		if err := repository.Close(); err != nil {
			utils.PrintCLIProblem("close repository", err)
			return
		}
		delete(interactiveConfig.Repositories, alias)
		fmt.Printf("Repository '%s' closed.\n", alias)

		// remove alias from RepositoryAliases
		newRepoAliases := []string{}
		for _, existingAlias := range interactiveConfig.RepositoryAliases {
			if existingAlias != alias {
				newRepoAliases = append(newRepoAliases, existingAlias)
			}
		}
		interactiveConfig.RepositoryAliases = newRepoAliases

		return
	}

	fmt.Printf("Error: No repository or SuperRepository found with alias '%s'.\n", alias)
}

func handleInfo(alias string) {
	// Check if alias is a SuperRepository
	if _, exists := interactiveConfig.SuperRepositories[alias]; exists {
		handleSuperRepositoryInfo(alias)
		return
	}

	// Check if alias is a Repository
	if repository, exists := interactiveConfig.Repositories[alias]; exists {
		handleRepositoryInfo(repository)
		return
	}

	// Check if alias is a Stage
	if stage, exists := interactiveConfig.Stages[alias]; exists {
		handleStageInfo(stage)
		return
	}

	// // Check if alias is a Dataset
	// if dataset, exists := interactiveConfig.Datasets[alias]; exists {
	// 	handleDatasetInfo(dataset)
	// 	return
	// }

	// Check if alias is a NamedGraph
	if namedGraph, exists := interactiveConfig.NamedGraphs[alias]; exists {
		handleNamedGraphInfo(namedGraph)
		return
	}

	// If alias does not match any known type
	fmt.Printf("Error: Alias '%s' not found in superrepositories, repositories, stages, datasets, or named graphs.\n", alias)
}

// Handle repository info
func handleRepositoryInfo(repository sst.Repository) {
	authCtx := utils.GetAuthContext(repository, interactiveConfig.AuthContexts)
	info, err := repository.Info(authCtx, "")
	if err != nil {
		utils.PrintCLIProblem("get repository info", err)
		return
	}
	utils.PrintRepositoryInfo(info)
}

// Handle repository superrepository info
func handleRepositorySuperRepository(repoAlias string) {
	// Check if repository exists
	repository, exists := interactiveConfig.Repositories[repoAlias]
	if !exists {
		fmt.Printf("Error: Repository alias '%s' not found.\n", repoAlias)
		return
	}

	// Get SuperRepository from the repository
	superRepo := repository.SuperRepository()
	if superRepo == nil {
		fmt.Printf("Repository '%s' does not belong to any SuperRepository.\n", repoAlias)
		return
	}

	// Find the SuperRepository alias in our config
	var superRepoAlias string
	for alias, sr := range interactiveConfig.SuperRepositories {
		if sr == superRepo {
			superRepoAlias = alias
			break
		}
	}

	if superRepoAlias == "" {
		fmt.Printf("Repository '%s' belongs to a SuperRepository, but it's not currently opened in the CLI.\n", repoAlias)
		fmt.Println("- SuperRepository information:")
		fmt.Println("  - Type: unknown (not opened in CLI)")
		return
	}

	// Display SuperRepository information
	repoType := interactiveConfig.SuperRepositoryTypes[superRepoAlias]
	location := interactiveConfig.SuperRepositoryLocations[superRepoAlias]

	fmt.Printf("- Repository '%s' belongs to SuperRepository '%s':\n", repoAlias, superRepoAlias)
	fmt.Printf("  - SuperRepository Type: %s\n", repoType)
	fmt.Printf("  - SuperRepository Location: %s\n", location)

	// List all repositories in the SuperRepository
	ctx := context.TODO()
	if repoType == "remote" {
		realProvider := utils.GetRealProvider()
		constructCtx := sstauth.ContextWithAuthProvider(ctx, realProvider)
		ctx = constructCtx
	}

	repoNames, err := superRepo.List(ctx)
	if err != nil {
		fmt.Printf("  - %s\n", utils.ExplainCLIError("list repositories", err))
	} else {
		fmt.Printf("  - Total Repositories: %d\n", len(repoNames))
		if len(repoNames) > 0 {
			fmt.Print("  - Repository Names: ")
			for i, name := range repoNames {
				if i > 0 {
					fmt.Print(", ")
				}
				fmt.Print(name)
			}
			fmt.Println()
		}
	}
}

// Handle stage info
func handleStageInfo(stage sst.Stage) {
	utils.PrintStageInfo(stage)
}

// Handle named graph info
func handleNamedGraphInfo(namedGraph sst.NamedGraph) {
	info := namedGraph.Info()
	utils.PrintNamedGraphInfo(info)
}

func handleDatasets(repoAlias string) {
	// check if repository exists
	repository, exists := interactiveConfig.Repositories[repoAlias]
	if !exists {
		fmt.Printf("Error: Repository alias '%s' not found.\n", repoAlias)
		return
	}

	var datasets []sst.IRI
	var err error

	authCtx := utils.GetAuthContext(repository, interactiveConfig.AuthContexts)

	// get all dataset IRIs
	// Show loading indicator while fetching dataset IRIs
	utils.ShowLoadingIndicator("Fetching dataset IRIs...", func() {
		datasets, err = repository.Datasets(authCtx)
	})

	// Handle errors gracefully
	if err != nil {
		errMsg := err.Error()
		// Check if the error indicates an empty repository (no datasets bucket)
		if errMsg == "datasets bucket not found" || errMsg == "repository bucket not found" {
			fmt.Printf("No datasets found in repository '%s'. The repository is empty.\n", repoAlias)
			return
		}
		utils.PrintCLIProblem("list datasets", err)
		return
	}

	if len(datasets) == 0 {
		fmt.Printf("No datasets found in repository '%s'.\n", repoAlias)
		return
	}

	// Store dataset IRIs in a slice for pagination
	var lines []string
	for _, dataset := range datasets {
		lines = append(lines, fmt.Sprintf("- %s", dataset))
	}

	// Display paginated output
	fmt.Printf("- Datasets in repository '%s':\n", repoAlias)
	utils.PaginateOutput(lines, 20)
}

func handleDataset(repoAlias string, args []string) {
	// get current repository
	repository, exists := interactiveConfig.Repositories[repoAlias]
	if !exists {
		fmt.Printf("Error: Repository '%s' not found.\n", repoAlias)
		return
	}

	var iri string

	// Get alias using GetAlias function
	aliasResult, err := utils.GetAlias(args, "dataset")
	if err != nil {
		fmt.Println(err) // If there's an error, print and return
		return
	}
	datasetAlias := aliasResult.Alias

	// Use defer to confirm alias generation only on success
	success := false
	defer func() {
		if success {
			aliasResult.Confirm()
		}
	}()

	// Extract dataset IRI
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-a":
			// Ignore this case as alias is handled by GetAlias
			i++ // Skip the next argument as it's the value for -a
		default:
			if iri == "" { // only accept the first non -a argument as IRI
				iri = args[i]
			}
		}
	}

	if iri == "" {
		fmt.Println("Error: Missing dataset IRI.")
		return
	}

	authCtx := utils.GetAuthContext(repository, interactiveConfig.AuthContexts)

	dataset, err := repository.Dataset(authCtx, sst.IRI(iri))
	if err != nil {
		utils.PrintCLIProblem("open dataset", err)
		return
	}

	// Success! Set flag so defer will confirm
	success = true

	interactiveConfig.Datasets[datasetAlias] = dataset
	interactiveConfig.DatasetAliases = append(interactiveConfig.DatasetAliases, datasetAlias)

	fmt.Printf("Dataset '%s' opened successfully.\n", datasetAlias)
}

var registerIndexOnce sync.Once

func handleQuery(alias string, args []string) {
	repo, exists := interactiveConfig.Repositories[alias]
	if !exists {
		fmt.Printf("Error: Repository alias '%s' not found.\n", alias)
		return
	}

	authCtx := utils.GetAuthContext(repo, interactiveConfig.AuthContexts)

	info, err := repo.Info(authCtx, "")
	if err != nil {
		utils.PrintCLIProblem("get repository info", err)
		return
	}

	if !info.IsRemote {
		registerIndexOnce.Do(func() {
			repo.RegisterIndexHandler(defaultderive.DeriveInfo())
		})
	}

	bleveIndex := repo.Bleve()
	if bleveIndex == nil {
		fmt.Printf("Error: Repository '%s' does not have an index.\n", alias)
		return
	}

	if len(args) == 0 {
		fmt.Println("Error: No search query provided.")
		return
	}

	limit := 10
	var queryParts []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--limit" && i+1 < len(args) {
			n, err := strconv.Atoi(args[i+1])
			if err == nil {
				limit = n
			}
			i++
		} else {
			queryParts = append(queryParts, args[i])
		}
	}

	queryString := strings.Join(queryParts, " ")
	q := bleve.NewQueryStringQuery(queryString)

	req := bleve.NewSearchRequest(q)
	req.Size = limit
	req.Fields = []string{"*"}

	sr, err := bleveIndex.SearchInContext(authCtx, req)
	if err != nil {
		utils.PrintCLIProblem("search", err)
		return
	}

	// Print all fields for query command (no whitelist filtering)
	utils.PrintSearchResultAllFields(sr)
}

func handleLog(alias string, args []string) {
	repo, ok := interactiveConfig.Repositories[alias]
	if !ok {
		fmt.Printf("Error: Repository alias '%s' not found.\n", alias)
		return
	}

	verbose := false
	for _, arg := range args {
		if arg == "-v" || arg == "--verbose" {
			verbose = true
			break
		}
	}

	authCtx := utils.GetAuthContext(repo, interactiveConfig.AuthContexts)

	logs, err := repo.Log(authCtx, nil, nil)

	if err != nil {
		utils.PrintCLIProblem("get repository log", err)
		return
	}

	if len(logs) == 0 {
		fmt.Println("No log entries found in repository.")
		return
	}

	var entryGroups [][]string
	if verbose {
		entryGroups = buildVerboseEntryGroups(repo, authCtx, logs)
	} else {
		entryGroups = buildSimpleEntryGroups(logs)
	}

	utils.PaginateLogEntries(entryGroups, 10)
}

func handleCommitInfo(alias string, args []string) {
	repo, ok := interactiveConfig.Repositories[alias]
	if !ok {
		fmt.Printf("Error: Repository alias '%s' not found.\n", alias)
		return
	}

	authCtx := utils.GetAuthContext(repo, interactiveConfig.AuthContexts)

	if len(args) < 1 {
		fmt.Println("Usage:")
		fmt.Println("  r1.commitInfo <commit-hash>   # show details for a specific commit")
		return
	}

	commitHash := args[0]
	hash, err := sst.StringToHash(commitHash)
	if err != nil {
		fmt.Println(utils.FormatCLIProblem("parse commit hash", "invalid commit hash"))
		return
	}

	detailsList, err := repo.CommitDetails(authCtx, []sst.Hash{hash})
	if err != nil {
		utils.PrintCLIProblem("get commit details", err)
		return
	}
	if len(detailsList) == 0 || detailsList[0] == nil {
		fmt.Printf("Commit %s not found\n", hash)
		return
	}
	detail := detailsList[0]

	utils.PrintCommitDetails(detail)
}

func handleRepoCheckoutCommit(alias string, args []string) {
	repo, ok := interactiveConfig.Repositories[alias]
	if !ok {
		fmt.Printf("Error: Repository alias '%s' not found.\n", alias)
		return
	}

	if len(args) < 1 {
		fmt.Println("Error: Missing arguments. Use '<repo-alias>.checkoutcommit <commit-hash>'.")
		return
	}

	commitID := args[0]
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

	hash, err := sst.StringToHash(commitID)
	if err != nil {
		fmt.Println(utils.FormatCLIProblem("parse commit hash", "invalid commit hash"))
		return
	}

	if _, exists := interactiveConfig.Stages[stageAlias]; exists {
		fmt.Printf("Error: Stage alias '%s' already exists.\n", stageAlias)
		return
	}

	authCtx := utils.GetAuthContext(repo, interactiveConfig.AuthContexts)

	stage, err := repo.CheckoutCommit(authCtx, hash, sst.DefaultTriplexMode)
	if err != nil {
		utils.PrintCLIProblem("checkout commit", err)
		return
	}

	success = true

	interactiveConfig.Stages[stageAlias] = stage
	interactiveConfig.StageAliases = append(interactiveConfig.StageAliases, stageAlias)
	interactiveConfig.StageCommits[stageAlias] = hash

	fmt.Printf("Stage '%s' (commit %s) opened successfully.\n", stageAlias, commitID)
}

type RepoWithDB interface {
	sst.Repository
	DB() *bbolt.DB
}

func handleDump(alias string, args []string) {
	repo, ok := interactiveConfig.Repositories[alias]
	if !ok {
		fmt.Printf("Error: Repository alias '%s' not found.\n", alias)
		return
	}

	repoWithDB, ok := repo.(RepoWithDB)
	if !ok {
		fmt.Println("Error: 'dump' only supported for local full repositories")
		return
	}

	if len(args) < 1 {
		fmt.Println("Usage:")
		fmt.Println(`  r1.dump "log"`)
		fmt.Println(`  r1.dump "log/...0005"`)
		return
	}

	bucketPath := utils.CleanQuotes(strings.Join(args, " "))

	db := repoWithDB.DB()
	if db == nil {
		fmt.Println("Error: repository DB is nil.")
		return
	}

	err := utils.DumpBboltFromDB(db, bucketPath)
	if err != nil {
		utils.PrintCLIProblem("dump repository", err)
	}
}

func handleCommitDiff(alias string, args []string) {
	if len(args) != 1 {
		fmt.Println("Usage: <repo>.commitdiff <commit hash>")
		return
	}

	repo, ok := interactiveConfig.Repositories[alias]
	if !ok {
		fmt.Printf("Error: Repository alias '%s' not found.\n", alias)
		return
	}
	ctx := utils.GetAuthContext(repo, interactiveConfig.AuthContexts)

	// Parse commit hash
	commitHashStr := args[0]
	commitHash, err := sst.StringToHash(commitHashStr)
	if err != nil {
		fmt.Println(utils.FormatCLIProblem("parse commit hash", "invalid commit hash"))
		return
	}

	// Load current commit details
	curList, err := repo.CommitDetails(ctx, []sst.Hash{commitHash})
	if err != nil || len(curList) == 0 || curList[0] == nil {
		fmt.Printf("Commit %s not found\n", commitHashStr)
		return
	}
	cur := curList[0]

	// Load all distinct parent commit details (merge-safe)
	parentCache := map[sst.Hash]*sst.CommitDetails{}
	seenParent := map[sst.Hash]struct{}{}
	for _, parents := range cur.ParentCommits {
		for _, h := range parents {
			if _, seen := seenParent[h]; seen {
				continue
			}
			seenParent[h] = struct{}{}
			lst, err := repo.CommitDetails(ctx, []sst.Hash{h})
			if err == nil && len(lst) > 0 {
				parentCache[h] = lst[0]
			} else {
				parentCache[h] = nil
			}
		}
	}

	// -------- Pass 1: NGs present in current commit --------
	for ngID, newNGR := range cur.NamedGraphRevisions {
		// Find a parent that contains this NG (if any)
		var parentDet *sst.CommitDetails
		for _, det := range parentCache {
			if det == nil {
				continue
			}
			if _, ok := det.NamedGraphRevisions[ngID]; ok {
				parentDet = det
				break
			}
		}

		// Naming rule (simplified): always "ng:<UUID>"
		name := "ng:" + string(ngID)

		// If no parent has this NG → announce addition
		if parentDet == nil {
			fmt.Printf("Added NamedGraph %s\n", name)
			continue
		}

		// Both sides exist → diff; skip if identical
		oldNGR := parentDet.NamedGraphRevisions[ngID]
		if oldNGR == newNGR {
			continue // unchanged
		}

		fmt.Printf("=== %s (modified)\n", name)
		tris, err := utils.SstDiffTriples(ctx, repo, oldNGR, newNGR, true)
		if err != nil {
			fmt.Printf("  (failed to diff: %v)\n", err)
			continue
		}
		utils.PrintDiffTriples(tris)
	}

	// -------- Pass 2: NGs deleted in current commit --------
	// Union of NG IDs across all parents
	parentNGs := map[sst.IRI]struct{}{}
	for _, det := range parentCache {
		if det == nil {
			continue
		}
		for ngID := range det.NamedGraphRevisions {
			parentNGs[ngID] = struct{}{}
		}
	}

	// For each parent NG not present now → announce deletion
	for ngID := range parentNGs {
		if _, still := cur.NamedGraphRevisions[ngID]; still {
			continue
		}
		name := "ng:" + ngID.String()
		fmt.Printf("Deleted NamedGraph %s\n", name)
	}
}

func handleDocumentSet(alias string, args []string) {
	if len(args) != 1 {
		fmt.Println("Usage: <repo>.upload <file_path>")
		return
	}

	inputPath := args[0]

	repo, exists := interactiveConfig.Repositories[alias]
	if !exists {
		fmt.Printf("Error: Repository alias '%s' not found.\n", alias)
		return
	}

	// Open the file
	file, err := os.Open(inputPath)
	if err != nil {
		utils.PrintCLIProblem("open file", err)
		return
	}
	defer file.Close()

	// Get file metadata
	stat, err := file.Stat()
	if err != nil {
		utils.PrintCLIProblem("stat file", err)
		return
	}

	// Step 1: Calculate hash first to check if document already exists
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		utils.PrintCLIProblem("read file", err)
		return
	}

	var hash sst.Hash
	copy(hash[:], hasher.Sum(nil))

	// Step 2: Check if document already exists
	authCtx := utils.GetAuthContext(repo, interactiveConfig.AuthContexts)
	docInfo, err := repo.Document(authCtx, hash, nil) // nil writer means we only check existence
	documentExists := err == nil

	// Step 3: Handle existing document
	if documentExists {
		fmt.Printf("Document already exists in repository '%s'.\n", alias)
		fmt.Printf("Hash: %s\n", hash.String())
		if docInfo != nil {
			fmt.Printf("MIME Type: %s\n", docInfo.MIMEType)
			fmt.Printf("Author: %s\n", docInfo.Author)
			fmt.Printf("Timestamp: %s\n", docInfo.Timestamp.Format("2006-01-02T15:04:05Z"))
		}
		fmt.Println("(No new upload log entry created due to content deduplication)")
		return
	}

	// Step 4: Reset file pointer and prepare for upload
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		utils.PrintCLIProblem("read file", err)
		return
	}

	// Guess MIME type based on file extension or content
	reader := bufio.NewReader(file)
	mimeType := guessMimeType(stat.Name(), reader)

	// Step 5: Upload new document
	fmt.Printf("Uploading document '%s' to repository '%s'...\n", inputPath, alias)
	uploadedHash, err := repo.DocumentSet(authCtx, mimeType, reader)
	if err != nil {
		utils.PrintCLIProblem("upload document", err)
		return
	}

	if uploadedHash != hash {
		fmt.Printf("Warning: Uploaded hash (%s) differs from calculated hash (%s)\n", uploadedHash.String(), hash.String())
	}

	fmt.Println("Upload successful! Hash:", uploadedHash.String())
}

func handleDocument(alias string, args []string) {
	if len(args) < 1 || len(args) > 2 {
		fmt.Println("Usage: <repo>.download <hashBase58> [<outPathOrDir>]")
		return
	}

	hashStr := args[0]
	var outPath string
	if len(args) == 2 {
		outPath = args[1]
	}

	repo, ok := interactiveConfig.Repositories[alias]
	if !ok {
		fmt.Printf("Repository alias '%s' not found.\n", alias)
		return
	}

	hash, err := sst.StringToHash(hashStr)
	if err != nil {
		fmt.Println("Invalid hash: must be a valid 44-character Base58 string")
		return
	}

	authCtx := utils.GetAuthContext(repo, interactiveConfig.AuthContexts)

	// Step 1: Download into buffer
	var buf bytes.Buffer
	info, err := repo.Document(authCtx, hash, &buf)
	if err != nil {
		utils.PrintCLIProblem("download document", err)
		return
	}

	// Step 2: Generate filename using hash + mime-based extension
	ext := extFromMime(info.MIMEType)
	filename := hash.String() + ext
	savePath := outPath
	if savePath == "" {
		savePath = filename
	} else if stat, err := os.Stat(savePath); err == nil && stat.IsDir() {
		savePath = filepath.Join(savePath, filename)
	}

	// Step 3: Write to file
	outFile, err := os.Create(savePath)
	if err != nil {
		utils.PrintCLIProblem("write file", err)
		return
	}
	defer outFile.Close()

	if _, err := buf.WriteTo(outFile); err != nil {
		utils.PrintCLIProblem("write file", err)
		return
	}

	fmt.Printf("Download successful. File saved as '%s'\n", savePath)
	fmt.Printf("- Hash: %s\n", hash.String())
}

func handleDocumentInfo(alias string, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: <repo>.documentinfo <hashBase58>")
		return
	}

	hashStr := args[0]
	hash, err := sst.StringToHash(hashStr)
	if err != nil {
		fmt.Println(utils.FormatCLIProblem("parse document hash", "invalid hash"))
		return
	}

	repo, ok := interactiveConfig.Repositories[alias]
	if !ok {
		fmt.Printf("Error: Repository alias '%s' not found.\n", alias)
		return
	}

	ctx := utils.GetAuthContext(repo, interactiveConfig.AuthContexts)

	doc, err := repo.Document(ctx, hash, nil)
	if err != nil {
		utils.PrintCLIProblem("get document info", err)
		return
	}

	utils.PrintDocumentInfo(doc)
}

func handleDocuments(alias string) {
	repo, ok := interactiveConfig.Repositories[alias]
	if !ok {
		fmt.Printf("Error: Repository alias '%s' not found.\n", alias)
		return
	}

	ctx := utils.GetAuthContext(repo, interactiveConfig.AuthContexts)

	docs, err := repo.Documents(ctx)

	if err != nil {
		utils.PrintCLIProblem("list documents", err)
		return
	}

	utils.PrintDocumentList(docs)
}

func handleDocumentDelete(alias string, args []string) {
	repo, ok := interactiveConfig.Repositories[alias]
	if !ok {
		fmt.Printf("Error: Repository alias '%s' not found.\n", alias)
		return
	}

	if len(args) < 1 {
		fmt.Println("Usage: <alias>.docdelete <hash>")
		return
	}

	ctx := utils.GetAuthContext(repo, interactiveConfig.AuthContexts)

	hashStr := args[0]
	hash, err := sst.StringToHash(hashStr)
	if err != nil {
		fmt.Println("Invalid hash: must be a valid 44-character Base58 string")
		return
	}

	err = repo.DocumentDelete(ctx, hash)
	if err != nil {
		utils.PrintCLIProblem("delete document", err)
		return
	}

	fmt.Println("Document deleted successfully.")
}

func handleExtractSstFile(alias string, args []string) {
	if len(args) < 1 || len(args) > 2 {
		fmt.Println("Usage: <alias>.extractsst <revisionHash> [savePath]")
		return
	}

	repo, ok := interactiveConfig.Repositories[alias]
	if !ok {
		fmt.Printf("Error: Repository alias '%s' not found.\n", alias)
		return
	}

	ctx := utils.GetAuthContext(repo, interactiveConfig.AuthContexts)

	// Parse revisionHash
	revisionHashStr := args[0]
	revisionHash, err := sst.StringToHash(revisionHashStr)
	if err != nil {
		fmt.Println("Invalid hash: must be a valid 44-character Base58 string")
		return
	}

	// Determine save path
	var savePath string
	filename := revisionHash.String() + ".sst"
	if len(args) == 2 {
		savePath = args[1]
		// If savePath is a directory, join with default filename
		if stat, err := os.Stat(savePath); err == nil && stat.IsDir() {
			savePath = filepath.Join(savePath, filename)
		}
	} else {
		savePath = filename
	}

	// Extract SST file into buffer
	var buf bytes.Buffer
	err = repo.ExtractSstFile(ctx, revisionHash, &buf)
	if err != nil {
		utils.PrintCLIProblem("extract SST file", err)
		return
	}

	// Write to file
	outFile, err := os.Create(savePath)
	if err != nil {
		utils.PrintCLIProblem("write file", err)
		return
	}
	defer outFile.Close()

	if n, err := buf.WriteTo(outFile); err != nil {
		utils.PrintCLIProblem("write file", err)
	} else {
		fmt.Printf("Successfully extracted %d bytes to '%s'\n", n, savePath)
	}
}

func handleOpenStage(alias string, args []string) {
	repo, ok := interactiveConfig.Repositories[alias]
	if !ok {
		fmt.Printf("Error: Repository alias '%s' not found.\n", alias)
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

	stage := repo.OpenStage(sst.DefaultTriplexMode)

	// Success! Set flag so defer will confirm
	success = true

	interactiveConfig.Stages[stageAlias] = stage
	interactiveConfig.StageAliases = append(interactiveConfig.StageAliases, stageAlias)

	fmt.Printf("Stage '%s' opened successfully.\n", stageAlias)
}

func handleSyncFrom(alias string, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: <target-repo-alias>.syncfrom <source-repo-alias> [branch] [dataset1] [dataset2] ...")
		fmt.Println("Example: r1.syncfrom r2")                                      // copy whole repo, all branches
		fmt.Println("Example: r1.syncfrom r2 master")                               // copy whole repo, master branch only
		fmt.Println("Example: r1.syncfrom r2 master dataset-uuid-1 dataset-uuid-2") // copy only specific datasets in the master branch
		fmt.Println("Example: r1.syncfrom r2 * dataset-uuid-1 dataset-uuid-2")      // copy only specific datasets, all branches
		fmt.Println("Note: Branch name is optional. If not specified, all branches will be synced.")
		fmt.Println("      Use '*' to explicitly sync all branches when specifying datasets.")
		fmt.Println("      Datasets can be specified as UUIDs or dataset aliases.")
		fmt.Println("      If datasets are specified, only those datasets (including their imported dependencies) will be synced.")
		fmt.Println("      If no datasets are specified, all datasets will be synced.")
		return
	}

	// Get target repository (the one that will receive the sync)
	targetRepo, ok := interactiveConfig.Repositories[alias]
	if !ok {
		fmt.Printf("Error: Target repository alias '%s' not found.\n", alias)
		return
	}

	// Get source repository (the one to sync from)
	sourceAlias := args[0]
	sourceRepo, ok := interactiveConfig.Repositories[sourceAlias]
	if !ok {
		fmt.Printf("Error: Source repository alias '%s' not found.\n", sourceAlias)
		return
	}

	if alias == sourceAlias {
		fmt.Println("Error: Cannot sync from a repository to itself.")
		return
	}

	// Parse arguments: [branch] [dataset1] [dataset2] ...
	// Branch name is optional. If the first arg after source-repo is not a dataset ref (UUID / IRI / opened dataset alias) and not "*",
	// it's treated as branch name.
	var branchName string
	var datasetIRIs []sst.IRI
	var datasetStartIdx int = 1

	looksLikeDatasetRef := func(s string) bool {
		if s == "" {
			return false
		}
		// already opened dataset alias
		if _, exists := interactiveConfig.Datasets[s]; exists {
			return true
		}
		// UUID
		if _, err := uuid.Parse(s); err == nil {
			return true
		}
		// URN IRI form used by SST datasets
		if strings.HasPrefix(s, "urn:uuid:") {
			return true
		}
		return false
	}

	if len(args) > 1 {
		firstArg := args[1]

		// Check if first arg is a branch name
		if firstArg == "*" {
			// "*" means all branches, explicitly set branchName to ""
			branchName = ""
			datasetStartIdx = 2
		} else if !looksLikeDatasetRef(firstArg) {
			// Not a dataset ref, treat as branch name
			branchName = firstArg
			datasetStartIdx = 2
		}

		// Parse dataset IRIs or aliases
		if len(args) > datasetStartIdx {
			datasetIRIs = make([]sst.IRI, 0, len(args)-datasetStartIdx)
			for i := datasetStartIdx; i < len(args); i++ {
				arg := args[i]
				// If arg is an opened dataset alias, use its IRI.
				if dataset, exists := interactiveConfig.Datasets[arg]; exists {
					datasetIRIs = append(datasetIRIs, dataset.IRI())
					continue
				}

				// If arg is a plain UUID, normalize to URN IRI.
				if id, err := uuid.Parse(arg); err == nil {
					datasetIRIs = append(datasetIRIs, sst.IRI(id.URN()))
					continue
				}

				// Otherwise treat as IRI string (e.g. "urn:uuid:...").
				datasetIRIs = append(datasetIRIs, sst.IRI(arg))
			}
		}
	}

	// Build sync options
	var syncOptions []sst.SyncOption
	if branchName != "" {
		syncOptions = append(syncOptions, sst.WithBranch(branchName))
	}
	if len(datasetIRIs) > 0 {
		syncOptions = append(syncOptions, sst.WithDatasetIRIs(datasetIRIs...))
	}

	// Print sync information
	if branchName != "" {
		fmt.Printf("Syncing branch: %s\n", branchName)
	} else {
		fmt.Println("Syncing all branches")
	}
	if len(datasetIRIs) > 0 {
		fmt.Printf("Syncing specified datasets (including import dependencies): %d dataset(s)\n", len(datasetIRIs))
	} else {
		fmt.Println("Syncing all datasets")
	}

	// Get auth context based on repository types
	targetType := interactiveConfig.RepositoryTypes[alias]
	sourceType := interactiveConfig.RepositoryTypes[sourceAlias]

	repo := targetRepo
	if targetType != "remote" && sourceType == "remote" {
		repo = sourceRepo
	}
	ctx := utils.GetAuthContext(repo, interactiveConfig.AuthContexts)

	// Perform the sync
	fmt.Printf("Syncing from repository '%s' to repository '%s'...\n", sourceAlias, alias)
	err := targetRepo.SyncFrom(ctx, sourceRepo, syncOptions...)

	if err != nil {
		utils.PrintCLIProblem("sync repository", err)
		return
	}

	fmt.Printf("Successfully synced from repository '%s' to repository '%s'.\n", sourceAlias, alias)
}
