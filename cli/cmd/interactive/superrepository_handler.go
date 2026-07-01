// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package interactive

import (
	"context"
	"fmt"
	"strings"

	"github.com/semanticstep/sst-core/cli/cmd/utils"
	"github.com/semanticstep/sst-core/defaultderive"
	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/sstauth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// handleOpenLocalSuperRepository handles opening a local SuperRepository
func handleOpenLocalSuperRepository(args []string) {
	var superRepoPath, alias string

	// Parse arguments
	if len(args) == 0 {
		fmt.Println("Usage: openlocalsuperrepository <path> [-a <alias>]")
		return
	}

	superRepoPath = args[0]

	aliasResult, err := utils.GetAlias(args[1:], "superrepository")
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
	if _, exists := interactiveConfig.SuperRepositories[alias]; exists {
		fmt.Printf("Error: SuperRepository with alias '%s' already exists.\n", alias)
		return
	}

	// Validate path
	if err := utils.ValidatePath(superRepoPath); err != nil {
		utils.PrintCLIProblem("validate path", err)
		return
	}

	// Open local SuperRepository
	superRepository, err := sst.NewLocalSuperRepository(superRepoPath)
	if err != nil {
		fmt.Printf("Error: Unable to open local SuperRepository: %s\n", err)
		return
	}

	// Register index handler for the SuperRepository
	err = superRepository.RegisterIndexHandler(defaultderive.DeriveInfo())
	if err != nil {
		fmt.Printf("Error: Unable to register index handler: %s\n", err)
		return
	}

	// Success. Set flag so defer will confirm
	success = true

	interactiveConfig.SuperRepositories[alias] = superRepository
	interactiveConfig.SuperRepositoryLocations[alias] = superRepoPath
	interactiveConfig.SuperRepositoryTypes[alias] = "local"
	interactiveConfig.SuperRepositoryAliases = append(interactiveConfig.SuperRepositoryAliases, alias)

	// Confirm success
	fmt.Printf("SuperRepository '%s' (local) opened successfully.\n", alias)
}

// handleOpenRemoteSuperRepository handles opening a remote SuperRepository
func handleOpenRemoteSuperRepository(args []string) {
	var superRepoURL, alias string

	// Parse arguments
	if len(args) == 0 {
		fmt.Println("Usage: openremotesuperrepository <url> [-a <alias>]")
		return
	}

	superRepoURL = args[0]

	aliasResult, err := utils.GetAlias(args[1:], "superrepository")
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
	if _, exists := interactiveConfig.SuperRepositories[alias]; exists {
		fmt.Printf("Error: SuperRepository with alias '%s' already exists.\n", alias)
		return
	}

	// Open remote SuperRepository
	ctx := context.TODO()
	creds := credentials.NewTLS(nil)
	realProvider := utils.GetRealProvider()
	constructCtx := sstauth.ContextWithAuthProvider(ctx, realProvider)

	var panicErr any
	var superRepository sst.SuperRepository
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicErr = r
			}
		}()

		superRepository, err = sst.OpenRemoteSuperRepository(constructCtx, superRepoURL, grpc.WithTransportCredentials(creds))
	}()
	if panicErr != nil {
		fmt.Printf("Cannot connect to remote SuperRepository at '%s'.\n", superRepoURL)
		fmt.Println("Please check that the URL is correct and your network is available.")
		fmt.Printf("(Technical info: %v)\n", panicErr)
		return
	}
	if err != nil {
		fmt.Printf("Error: Unable to open remote SuperRepository: %s\n", err)
		return
	}

	// Success. Set flag so defer will confirm
	success = true

	interactiveConfig.SuperRepositories[alias] = superRepository
	interactiveConfig.SuperRepositoryLocations[alias] = superRepoURL
	interactiveConfig.SuperRepositoryTypes[alias] = "remote"
	interactiveConfig.SuperRepositoryAliases = append(interactiveConfig.SuperRepositoryAliases, alias)
	interactiveConfig.SuperAuthContexts[alias] = constructCtx

	// Confirm success
	fmt.Printf("SuperRepository '%s' (remote) opened successfully.\n", alias)
}

// handleCloseSuperRepository handles closing a SuperRepository based on its alias
func handleCloseSuperRepository(alias string) {
	// Check if the SuperRepository exists
	superRepository, exists := interactiveConfig.SuperRepositories[alias]
	if !exists {
		fmt.Printf("Error: No SuperRepository found with alias '%s'.\n", alias)
		return
	}

	// For now, just remove from the map
	delete(interactiveConfig.SuperRepositories, alias)
	delete(interactiveConfig.SuperRepositoryLocations, alias)
	delete(interactiveConfig.SuperRepositoryTypes, alias)
	delete(interactiveConfig.SuperAuthContexts, alias)

	// Remove alias from SuperRepositoryAliases
	newSuperRepoAliases := []string{}
	for _, existingAlias := range interactiveConfig.SuperRepositoryAliases {
		if existingAlias != alias {
			newSuperRepoAliases = append(newSuperRepoAliases, existingAlias)
		}
	}
	interactiveConfig.SuperRepositoryAliases = newSuperRepoAliases

	fmt.Printf("SuperRepository '%s' closed.\n", alias)

	// should ideally be tracked, but for now just note it
	_ = superRepository
}

// handleSuperRepositoryGet handles getting a repository from a SuperRepository
func handleSuperRepositoryGet(superRepoAlias string, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: <superrepo-alias>.get <repo-name> [-a <repo-alias>]")
		return
	}

	// Get SuperRepository
	superRepository, exists := interactiveConfig.SuperRepositories[superRepoAlias]
	if !exists {
		fmt.Printf("Error: SuperRepository alias '%s' not found.\n", superRepoAlias)
		return
	}

	repoName := args[0]

	// Get alias for the repository
	aliasResult, err := utils.GetAlias(args[1:], "repository")
	if err != nil {
		fmt.Println(err)
		return
	}
	repoAlias := aliasResult.Alias

	// Use defer to confirm alias generation only on success
	success := false
	defer func() {
		if success {
			aliasResult.Confirm()
		}
	}()

	// Check if repository alias already exists
	if _, exists := interactiveConfig.Repositories[repoAlias]; exists {
		fmt.Printf("Error: Repository with alias '%s' already exists.\n", repoAlias)
		return
	}

	// Get repository from SuperRepository
	repository, err := superRepository.Get(context.TODO(), repoName)
	if err != nil {
		utils.PrintCLIProblem("get repository", err)
		return
	}

	// Success! Set flag so defer will confirm
	success = true

	// Store the repository
	interactiveConfig.Repositories[repoAlias] = repository
	interactiveConfig.RepositoryAliases = append(interactiveConfig.RepositoryAliases, repoAlias)

	// Store location and type
	location := fmt.Sprintf("%s/%s", interactiveConfig.SuperRepositoryLocations[superRepoAlias], repoName)
	interactiveConfig.RepositoryLocations[repoAlias] = location
	interactiveConfig.RepositoryTypes[repoAlias] = interactiveConfig.SuperRepositoryTypes[superRepoAlias]

	// Get or create auth context for the repository using GetAuthContext
	authCtx := utils.GetAuthContext(repository, interactiveConfig.AuthContexts)
	if authCtx == context.TODO() && interactiveConfig.SuperRepositoryTypes[superRepoAlias] == "remote" {
		// Prefer reusing the remote SuperRepository's auth context.
		if superCtx, ok := interactiveConfig.SuperAuthContexts[superRepoAlias]; ok && superCtx != nil {
			authCtx = superCtx
		} else {
			realProvider := utils.GetRealProvider()
			authCtx = sstauth.ContextWithAuthProvider(context.TODO(), realProvider)
			interactiveConfig.SuperAuthContexts[superRepoAlias] = authCtx
		}
		interactiveConfig.AuthContexts[repository] = authCtx
	}

	fmt.Printf("Repository '%s' (from SuperRepository '%s') opened successfully.\n", repoAlias, superRepoAlias)
}

// handleSuperRepositoryCreate handles creating a repository in a SuperRepository
func handleSuperRepositoryCreate(superRepoAlias string, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: <superrepo-alias>.create <repo-name> [-a <repo-alias>]")
		return
	}

	// Get SuperRepository
	superRepository, exists := interactiveConfig.SuperRepositories[superRepoAlias]
	if !exists {
		fmt.Printf("Error: SuperRepository alias '%s' not found.\n", superRepoAlias)
		return
	}

	repoName := strings.TrimSpace(args[0])
	if repoName == "" {
		fmt.Println("Error: Repository name cannot be empty.")
		return
	}

	// Get alias for the repository
	aliasResult, err := utils.GetAlias(args[1:], "repository")
	if err != nil {
		fmt.Println(err)
		return
	}
	repoAlias := aliasResult.Alias

	// Use defer to confirm alias generation only on success
	success := false
	defer func() {
		if success {
			aliasResult.Confirm()
		}
	}()

	// Check if repository alias already exists
	if _, exists := interactiveConfig.Repositories[repoAlias]; exists {
		fmt.Printf("Error: Repository with alias '%s' already exists.\n", repoAlias)
		return
	}

	// Get auth context if needed
	ctx := context.TODO()
	if interactiveConfig.SuperRepositoryTypes[superRepoAlias] == "remote" {
		if superCtx, ok := interactiveConfig.SuperAuthContexts[superRepoAlias]; ok && superCtx != nil {
			ctx = superCtx
		} else {
			realProvider := utils.GetRealProvider()
			constructCtx := sstauth.ContextWithAuthProvider(ctx, realProvider)
			ctx = constructCtx
			interactiveConfig.SuperAuthContexts[superRepoAlias] = ctx
		}
	}

	// Create repository in SuperRepository
	repository, err := superRepository.Create(ctx, repoName)
	if err != nil {
		utils.PrintCLIProblem("create repository", err)
		return
	}

	// Success. Set flag so defer will confirm
	success = true

	// Store the repository
	interactiveConfig.Repositories[repoAlias] = repository
	interactiveConfig.RepositoryAliases = append(interactiveConfig.RepositoryAliases, repoAlias)

	// Store location and type
	location := fmt.Sprintf("%s/%s", interactiveConfig.SuperRepositoryLocations[superRepoAlias], repoName)
	interactiveConfig.RepositoryLocations[repoAlias] = location
	interactiveConfig.RepositoryTypes[repoAlias] = interactiveConfig.SuperRepositoryTypes[superRepoAlias]

	// If it's a remote repository, set up auth context
	if interactiveConfig.SuperRepositoryTypes[superRepoAlias] == "remote" {
		interactiveConfig.AuthContexts[repository] = ctx
	}

	fmt.Printf("Repository '%s' created successfully in SuperRepository '%s'.\n", repoAlias, superRepoAlias)
}

// handleSuperRepositoryDelete handles deleting a repository from a SuperRepository
func handleSuperRepositoryDelete(superRepoAlias string, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: <superrepo-alias>.delete <repo-name>")
		return
	}

	// Get SuperRepository
	superRepository, exists := interactiveConfig.SuperRepositories[superRepoAlias]
	if !exists {
		fmt.Printf("Error: SuperRepository alias '%s' not found.\n", superRepoAlias)
		return
	}

	repoName := strings.TrimSpace(args[0])
	if repoName == "" {
		fmt.Println("Error: Repository name cannot be empty.")
		return
	}

	// Get auth context if needed
	ctx := context.TODO()
	if interactiveConfig.SuperRepositoryTypes[superRepoAlias] == "remote" {
		if superCtx, ok := interactiveConfig.SuperAuthContexts[superRepoAlias]; ok && superCtx != nil {
			ctx = superCtx
		} else {
			realProvider := utils.GetRealProvider()
			constructCtx := sstauth.ContextWithAuthProvider(ctx, realProvider)
			ctx = constructCtx
			interactiveConfig.SuperAuthContexts[superRepoAlias] = ctx
		}
	}

	// Find the repository alias if it's open in CLI
	var repoAliasToRemove string
	var repoToRemove sst.Repository
	expectedLocation := fmt.Sprintf("%s/%s", interactiveConfig.SuperRepositoryLocations[superRepoAlias], repoName)
	for alias, repo := range interactiveConfig.Repositories {
		if repo.SuperRepository() == superRepository {
			if interactiveConfig.RepositoryLocations[alias] == expectedLocation {
				repoAliasToRemove = alias
				repoToRemove = repo
				break
			}
		}
	}

	// Delete repository from SuperRepository
	err := superRepository.Delete(ctx, repoName)
	if err != nil {
		utils.PrintCLIProblem("delete repository", err)
		return
	}

	// Remove from CLI config if it was open
	if repoAliasToRemove != "" {
		delete(interactiveConfig.Repositories, repoAliasToRemove)
		delete(interactiveConfig.RepositoryLocations, repoAliasToRemove)
		delete(interactiveConfig.RepositoryTypes, repoAliasToRemove)

		// Remove auth context if exists
		if repoToRemove != nil {
			delete(interactiveConfig.AuthContexts, repoToRemove)
		}

		// Remove from aliases
		newAliases := []string{}
		for _, a := range interactiveConfig.RepositoryAliases {
			if a != repoAliasToRemove {
				newAliases = append(newAliases, a)
			}
		}
		interactiveConfig.RepositoryAliases = newAliases
		fmt.Printf("Repository '%s' was open and has been closed.\n", repoAliasToRemove)
	}

	fmt.Printf("Repository '%s' deleted successfully from SuperRepository '%s'.\n", repoName, superRepoAlias)
}

// handleSuperRepositoryList handles listing repositories in a SuperRepository
func handleSuperRepositoryList(superRepoAlias string, args []string) {
	// Get SuperRepository
	superRepository, exists := interactiveConfig.SuperRepositories[superRepoAlias]
	if !exists {
		fmt.Printf("Error: SuperRepository alias '%s' not found.\n", superRepoAlias)
		return
	}

	// Get auth context if needed
	ctx := context.TODO()
	if interactiveConfig.SuperRepositoryTypes[superRepoAlias] == "remote" {
		if superCtx, ok := interactiveConfig.SuperAuthContexts[superRepoAlias]; ok && superCtx != nil {
			ctx = superCtx
		} else {
			realProvider := utils.GetRealProvider()
			constructCtx := sstauth.ContextWithAuthProvider(ctx, realProvider)
			ctx = constructCtx
			interactiveConfig.SuperAuthContexts[superRepoAlias] = ctx
		}
	}

	// List repositories
	repoNames, err := superRepository.List(ctx)
	if err != nil {
		utils.PrintCLIProblem("list repositories", err)
		return
	}

	if len(repoNames) == 0 {
		fmt.Printf("No repositories found in SuperRepository '%s'.\n", superRepoAlias)
		return
	}

	// Display paginated output
	var lines []string
	for _, name := range repoNames {
		lines = append(lines, fmt.Sprintf("- %s", name))
	}

	fmt.Printf("- Repositories in SuperRepository '%s':\n", superRepoAlias)
	utils.PaginateOutput(lines, 20)
}

// handleSuperRepositoryInfo handles showing information about a SuperRepository
func handleSuperRepositoryInfo(superRepoAlias string) {
	superRepository, exists := interactiveConfig.SuperRepositories[superRepoAlias]
	if !exists {
		fmt.Printf("Error: SuperRepository alias '%s' not found.\n", superRepoAlias)
		return
	}

	repoType := interactiveConfig.SuperRepositoryTypes[superRepoAlias]
	location := interactiveConfig.SuperRepositoryLocations[superRepoAlias]

	fmt.Printf("- SuperRepository: %s\n", superRepoAlias)
	fmt.Printf("- Type: %s\n", repoType)
	fmt.Printf("- Location: %s\n", location)

	// List repositories
	ctx := context.TODO()
	if repoType == "remote" {
		if superCtx, ok := interactiveConfig.SuperAuthContexts[superRepoAlias]; ok && superCtx != nil {
			ctx = superCtx
		} else {
			realProvider := utils.GetRealProvider()
			constructCtx := sstauth.ContextWithAuthProvider(ctx, realProvider)
			ctx = constructCtx
			interactiveConfig.SuperAuthContexts[superRepoAlias] = ctx
		}
	}

	repoNames, err := superRepository.List(ctx)
	if err != nil {
		fmt.Println("- " + utils.ExplainCLIError("list repositories", err))
	} else {
		fmt.Printf("- Repositories: %d\n", len(repoNames))
		if len(repoNames) > 0 {
			fmt.Print("  - ")
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
