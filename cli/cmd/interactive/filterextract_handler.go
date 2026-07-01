// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package interactive

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/semanticstep/sst-core/cli/cmd/utils"
	"github.com/semanticstep/sst-core/tools/filterextraction"
)

func handleFilterExtract(repoAlias string, args []string) {
	repo, ok := interactiveConfig.Repositories[repoAlias]
	if !ok || repo == nil {
		fmt.Printf("Error: Repository alias '%s' not found.\n", repoAlias)
		return
	}

	// For now: filterextraction uses context.TODO() internally, which won't work for remote repos.
	if strings.EqualFold(interactiveConfig.RepositoryTypes[repoAlias], "remote") {
		fmt.Println("Error: <repo>.filterextract is currently supported only for local repositories (including localflat).")
		fmt.Println("       Open a local repo with 'openlocalrepository' or 'openlocalflatrepository'.")
		return
	}

	if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
		fmt.Println("Usage: <repo>.filterextract <input-directory>")
		fmt.Println("Example: r1.filterextract cli/testfile")
		fmt.Println("Note: Processes all .ttl files under the directory (skips names starting with '_') and commits results to the repo.")
		return
	}

	inputDir := strings.TrimSpace(args[0])
	abs, err := filepath.Abs(inputDir)
	if err == nil {
		inputDir = abs
	}
	st, err := os.Stat(inputDir)
	if err != nil {
		utils.PrintCLIProblem("access input directory", err)
		return
	}
	if !st.IsDir() {
		fmt.Printf("Error: Input path '%s' is not a directory.\n", inputDir)
		return
	}

	fmt.Printf("Running filter extraction for TTL files in %s\n", inputDir)
	filterextraction.Run(repo, inputDir)
	fmt.Println("Done.")
}
