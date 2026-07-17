// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"context"
	"os"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/sstauth"
	"github.com/semanticstep/sst-core/sst_test/testutil"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testRepoPath is the path to the existing repository.
// Set this to your repository path containing the bbolt file.
// Can be overridden via SST_TEST_REPO_PATH environment variable.
const testRepoPath = "./testdata/existing_repo"

// getRepoPath returns the path to the existing repository.
// Checks SST_TEST_REPO_PATH environment variable first, then falls back to default.
func getRepoPath() string {
	if path := os.Getenv("SST_TEST_REPO_PATH"); path != "" {
		return path
	}
	return testRepoPath
}

// Test_LocalFullRepository_CheckoutAllDatasetsMasterBranch opens an existing
// LocalFullRepository at the specified path and checks out the "master" branch for each dataset.
//
// Usage:
//   - Place your bbolt file in the path specified by testRepoPath or set SST_TEST_REPO_PATH env var
//   - Run: go test -v -run Test_LocalFullRepository_CheckoutAllDatasetsMasterBranch ./sst_test_realvoc/...
func Test_LocalFullRepository_CheckoutAllDatasetsMasterBranch(t *testing.T) {
	repoPath := getRepoPath()

	// Check if path exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		t.Skipf("Repository path does not exist: %s. Place bbolt file there or set SST_TEST_REPO_PATH env var.", repoPath)
	}

	// Open existing repository
	repo, err := sst.OpenLocalRepository(repoPath, "default@semanticstep.net", "default")
	require.NoError(t, err, "Failed to open repository at %s", repoPath)
	defer repo.Close()

	// Get all datasets
	datasetIRIs, err := repo.Datasets(context.TODO())
	require.NoError(t, err)

	t.Logf("Found %d datasets in repository", len(datasetIRIs))

	if len(datasetIRIs) == 0 {
		t.Log("No datasets found in repository")
		return
	}

	// Checkout master branch for each dataset
	var successCount, failCount int
	for _, dsIRI := range datasetIRIs {
		ds, err := repo.Dataset(context.TODO(), dsIRI)
		if err != nil {
			t.Logf("Failed to get dataset for IRI %s: %v", dsIRI, err)
			failCount++
			continue
		}

		// Get branches to verify master exists
		branches, err := ds.Branches(context.TODO())
		if err != nil {
			t.Logf("Failed to get branches for dataset %s: %v", dsIRI, err)
			failCount++
			continue
		}

		// Check if master branch exists
		_, hasMaster := branches[sst.DefaultBranch]
		if !hasMaster {
			t.Logf("Dataset %s does not have master branch, available branches: %v", dsIRI, getBranchNames(branches))
			failCount++
			continue
		}

		// Checkout master branch
		stage, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			t.Logf("Failed to checkout master branch for dataset %s: %v", dsIRI, err)
			failCount++
			continue
		}

		// Verify the stage has the expected named graph
		ng := stage.NamedGraph(dsIRI)
		if ng == nil {
			t.Logf("NamedGraph is nil for dataset %s after checkout", dsIRI)
			failCount++
			continue
		}

		t.Logf("Successfully checked out master branch for dataset: %s", dsIRI)
		successCount++
	}

	t.Logf("Checkout summary: %d succeeded, %d failed out of %d datasets", successCount, failCount, len(datasetIRIs))
	assert.Equal(t, 0, failCount, "Some datasets failed to checkout")
}

// Test_LocalFullRepository_CheckoutAllDatasetsMasterBranch_ForDatasets opens an existing
// LocalFullRepository and uses ForDatasets to checkout each dataset's master branch.
func Test_LocalFullRepository_CheckoutAllDatasetsMasterBranch_ForDatasets(t *testing.T) {
	repoPath := getRepoPath()

	// Check if path exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		t.Skipf("Repository path does not exist: %s. Place bbolt file there or set SST_TEST_REPO_PATH env var.", repoPath)
	}

	// Open existing repository
	repo, err := sst.OpenLocalRepository(repoPath, "default@semanticstep.net", "default")
	require.NoError(t, err, "Failed to open repository at %s", repoPath)
	defer repo.Close()

	var successCount, failCount, skipCount int

	// Iterate through all datasets and checkout master branch
	err = repo.ForDatasets(context.TODO(), func(ds sst.Dataset) error {
		dsIRI := ds.IRI()

		// Get branches to verify master exists
		branches, err := ds.Branches(context.TODO())
		if err != nil {
			t.Logf("Failed to get branches for dataset %s: %v", dsIRI, err)
			failCount++
			return nil // Continue with next dataset
		}

		// Check if master branch exists
		_, hasMaster := branches[sst.DefaultBranch]
		if !hasMaster {
			t.Logf("Dataset %s does not have master branch, available branches: %v", dsIRI, getBranchNames(branches))
			skipCount++
			return nil // Continue with next dataset
		}

		// Checkout master branch
		stage, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			t.Logf("Failed to checkout master branch for dataset %s: %v", dsIRI, err)
			failCount++
			return nil // Continue with next dataset
		}

		// Verify the stage has the expected named graph
		ng := stage.NamedGraph(dsIRI)
		if ng == nil {
			t.Logf("NamedGraph is nil for dataset %s after checkout", dsIRI)
			failCount++
			return nil // Continue with next dataset
		}

		t.Logf("Successfully checked out master branch for dataset: %s", dsIRI)
		successCount++
		return nil
	})

	require.NoError(t, err, "ForDatasets iteration failed")
	t.Logf("Checkout summary: %d succeeded, %d skipped (no master), %d failed", successCount, skipCount, failCount)
	assert.Equal(t, 0, failCount, "Some datasets failed to checkout")
}

// Test_RemoteRepository_CheckoutAllDatasetsMasterBranch opens an existing repository
// at the specified path, starts a test server, and checks out the "master" branch for each dataset.
//
// Usage:
//   - Place your bbolt file in the path specified by testRepoPath or set SST_TEST_REPO_PATH env var
//   - Run: go test -v -run Test_RemoteRepository_CheckoutAllDatasetsMasterBranch ./sst_test_realvoc/...
func Test_RemoteRepository_CheckoutAllDatasetsMasterBranch(t *testing.T) {
	repoPath := getRepoPath()

	// Check if path exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		t.Skipf("Repository path does not exist: %s. Place bbolt file there or set SST_TEST_REPO_PATH env var.", repoPath)
	}

	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

	// Start server from the existing repository path
	url := testutil.ServerServe(t, repoPath)

	// Open remote repository
	repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
	require.NoError(t, err, "Failed to open remote repository")
	defer repo.Close()

	// Get all datasets
	datasetIRIs, err := repo.Datasets(constructCtx)
	require.NoError(t, err)

	t.Logf("Found %d datasets in remote repository", len(datasetIRIs))

	if len(datasetIRIs) == 0 {
		t.Log("No datasets found in remote repository")
		return
	}

	// Checkout master branch for each dataset
	var successCount, failCount int
	for _, dsIRI := range datasetIRIs {
		ds, err := repo.Dataset(constructCtx, dsIRI)
		if err != nil {
			t.Logf("Failed to get dataset for IRI %s: %v", dsIRI, err)
			failCount++
			continue
		}

		// Get branches to verify master exists
		branches, err := ds.Branches(constructCtx)
		if err != nil {
			t.Logf("Failed to get branches for dataset %s: %v", dsIRI, err)
			failCount++
			continue
		}

		// Check if master branch exists
		_, hasMaster := branches[sst.DefaultBranch]
		if !hasMaster {
			t.Logf("Dataset %s does not have master branch, available branches: %v", dsIRI, getBranchNames(branches))
			failCount++
			continue
		}

		// Checkout master branch
		stage, err := ds.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			t.Logf("Failed to checkout master branch for dataset %s: %v", dsIRI, err)
			failCount++
			continue
		}

		// Verify the stage has the expected named graph
		ng := stage.NamedGraph(dsIRI)
		if ng == nil {
			t.Logf("NamedGraph is nil for dataset %s after checkout", dsIRI)
			failCount++
			continue
		}

		t.Logf("Successfully checked out master branch for dataset: %s", dsIRI)
		successCount++
	}

	t.Logf("Checkout summary: %d succeeded, %d failed out of %d datasets", successCount, failCount, len(datasetIRIs))
	assert.Equal(t, 0, failCount, "Some datasets failed to checkout")
}

// Test_RemoteRepository_CheckoutAllDatasetsMasterBranch_ForDatasets opens an existing
// repository at the specified path, starts a test server, and uses ForDatasets to checkout
// each dataset's master branch.
func Test_RemoteRepository_CheckoutAllDatasetsMasterBranch_ForDatasets(t *testing.T) {
	repoPath := getRepoPath()

	// Check if path exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		t.Skipf("Repository path does not exist: %s. Place bbolt file there or set SST_TEST_REPO_PATH env var.", repoPath)
	}

	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

	// Start server from the existing repository path
	url := testutil.ServerServe(t, repoPath)

	// Open remote repository
	repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
	require.NoError(t, err, "Failed to open remote repository")
	defer repo.Close()

	var successCount, failCount, skipCount int

	// Iterate through all datasets and checkout master branch
	err = repo.ForDatasets(constructCtx, func(ds sst.Dataset) error {
		dsIRI := ds.IRI()

		// Get branches to verify master exists
		branches, err := ds.Branches(constructCtx)
		if err != nil {
			t.Logf("Failed to get branches for dataset %s: %v", dsIRI, err)
			failCount++
			return nil // Continue with next dataset
		}

		// Check if master branch exists
		_, hasMaster := branches[sst.DefaultBranch]
		if !hasMaster {
			t.Logf("Dataset %s does not have master branch, available branches: %v", dsIRI, getBranchNames(branches))
			skipCount++
			return nil // Continue with next dataset
		}

		// Checkout master branch
		stage, err := ds.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			t.Logf("Failed to checkout master branch for dataset %s: %v", dsIRI, err)
			failCount++
			return nil // Continue with next dataset
		}

		// Verify the stage has the expected named graph
		ng := stage.NamedGraph(dsIRI)
		if ng == nil {
			t.Logf("NamedGraph is nil for dataset %s after checkout", dsIRI)
			failCount++
			return nil // Continue with next dataset
		}

		t.Logf("Successfully checked out master branch for dataset: %s", dsIRI)
		successCount++
		return nil
	})

	require.NoError(t, err, "ForDatasets iteration failed")
	t.Logf("Checkout summary: %d succeeded, %d skipped (no master), %d failed", successCount, skipCount, failCount)
	assert.Equal(t, 0, failCount, "Some datasets failed to checkout")
}

// getBranchNames returns a slice of branch names from the branches map
func getBranchNames(branches map[string]sst.Hash) []string {
	names := make([]string, 0, len(branches))
	for name := range branches {
		names = append(names, name)
	}
	return names
}
