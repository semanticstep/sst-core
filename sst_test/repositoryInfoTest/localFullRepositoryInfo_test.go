// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/semanticstep/sst-core/defaultderive"
	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestLocalFullRepositoryInfo1(t *testing.T) {
	path := filepath.Join(t.TempDir(), t.Name())

	repo, err := sst.CreateLocalRepository(path, "default@semanticstep.net", "default", true)
	assert.NoError(t, err)
	defer repo.Close()

	repo.RegisterIndexHandler(defaultderive.DeriveInfo())

	stats, err := repo.Info(context.TODO(), "")
	assert.NoError(t, err)

	assert.Greater(t, stats.MasterDBSize, int64(0), "BboltSize should be greater than 0")
	assert.GreaterOrEqual(t, stats.DerivedDBSize, int64(0), "BleveSize should be greater or equal to 0")
	assert.GreaterOrEqual(t, stats.NumberOfDatasets, int64(0), "NumberOfDatasets should be greater or equal to 0")
	assert.GreaterOrEqual(t, stats.NumberOfDatasetRevisions, int64(0), "NumberOfDatasetRevisions should be greater or equal to 0")
	assert.GreaterOrEqual(t, stats.NumberOfNamedGraphRevisions, int64(0), "NumberOfNamedGraphRevisions should be greater or equal to 0")
	assert.GreaterOrEqual(t, stats.NumberOfCommits, int64(0), "NumberOfCommits should be greater or equal to 0")
	assert.GreaterOrEqual(t, stats.NumberOfRepositoryLogs, int64(0), "NumberOfDatasetLog should be greater or equal to 0")

	t.Log("BboltSize:", stats.MasterDBSize)
	t.Log("BleveSize:", stats.DerivedDBSize)
	t.Log("NumberOfDatasets:", stats.NumberOfDatasets)
	t.Log("NumberOfDatasetRevisions:", stats.NumberOfDatasetRevisions)
	t.Log("NumberOfNamedGraphRevisions:", stats.NumberOfNamedGraphRevisions)
	t.Log("NumberOfCommits:", stats.NumberOfCommits)
	t.Log("NumberOfRepositoryLogs:", stats.NumberOfRepositoryLogs)
}

// TestRepositoryInfo2 tests the RepositoryInfo function under various scenarios.
// The test ensures that the statistics returned by RepositoryInfo (such as
// NumberOfDatasets, NumberOfDatasetRevisions, NumberOfNamedGraphRevisions, and NumberOfCommits)
// accurately reflect the operations performed on the repository.
//
// Scenarios tested:
// 1. Create 1 NamedGraph and commit once.
//   - Expected results:
//     NumberOfDatasets = 1, NumberOfDatasetRevisions = 1,
//     NumberOfNamedGraphRevisions = 1, NumberOfCommits = 1.
//
// 2. Create 1 NamedGraph, commit once, modify it, and commit again.
//   - Expected results:
//     NumberOfDatasets = 1, NumberOfDatasetRevisions = 2,
//     NumberOfNamedGraphRevisions = 2, NumberOfCommits = 2.
//
// 3. Create 2 NamedGraphs and commit once.
//   - Expected results:
//     NumberOfDatasets = 2, NumberOfDatasetRevisions = 2,
//     NumberOfNamedGraphRevisions = 2, NumberOfCommits = 1.
//
// 4. Create 2 NamedGraphs, commit once, modify one NamedGraph, and commit again.
//   - Expected results:
//     NumberOfDatasets = 2, NumberOfDatasetRevisions = 3,
//     NumberOfNamedGraphRevisions = 3, NumberOfCommits = 2.
//
// 5. Create 2 NamedGraphs, commit once, modify both NamedGraphs, and commit again.
//   - Expected results:
//     NumberOfDatasets = 2, NumberOfDatasetRevisions = 4,
//     NumberOfNamedGraphRevisions = 4, NumberOfCommits = 2.
//
// Each test resets the repository environment to ensure no data from previous tests affects the results.
func TestLocalFullRepositoryInfo2(t *testing.T) {
	// Helper function to reset test environment
	setupTestEnvironment := func(t *testing.T) sst.Repository {
		repo, err := sst.CreateLocalRepository(filepath.Join(t.TempDir(), t.Name()), "default@semanticstep.net", "default", true)
		assert.NoError(t, err)
		repo.RegisterIndexHandler(defaultderive.DeriveInfo()) // Bleve
		return repo
	}

	// Create 1 NamedGraph, commit 1 time
	t.Run("TestCase1", func(t *testing.T) {
		repository := setupTestEnvironment(t) // Initialize a new repository
		defer repository.Close()

		ngIDC := uuid.New()

		// Create NamedGraph and commit
		stageC := repository.OpenStage(sst.DefaultTriplexMode)
		ng := stageC.CreateNamedGraph(sst.IRI(ngIDC.URN()))

		mainC := ng.CreateIRINode("mainC")
		mainC.AddStatement(rdf.Type, rep.SchematicPort)

		stageC.Commit(context.TODO(), "First commit of NamedGraph", sst.DefaultBranch)

		// Get RepositoryInfo
		stats, err := repository.Info(context.TODO(), "")
		assert.NoError(t, err)

		t.Logf("RepositoryInfo: %+v", stats)

		// Verify statistics
		assert.Equal(t, stats.NumberOfDatasets, int64(1))
		assert.Equal(t, stats.NumberOfDatasetRevisions, int64(1))
		assert.Equal(t, stats.NumberOfNamedGraphRevisions, int64(1))
		assert.Equal(t, stats.NumberOfCommits, int64(1))
	})

	// Create 1 NamedGraph, commit 1 time, modify it, then commit again
	t.Run("TestCase2", func(t *testing.T) {
		repository := setupTestEnvironment(t) // Initialize a new repository
		defer repository.Close()

		ngIDC := uuid.New()

		// Create NamedGraph and commit
		st := repository.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(sst.IRI(ngIDC.URN()))

		mainC := ng.CreateIRINode("mainC")
		mainC.AddStatement(rdf.Type, rep.SchematicPort)
		st.Commit(context.TODO(), "First commit of NamedGraph", sst.DefaultBranch)

		// Modify NamedGraph
		mainC.AddStatement(rdf.Bag, rep.Angle)
		st.Commit(context.TODO(), "Second commit of NamedGraph", sst.DefaultBranch)

		// Get RepositoryInfo
		stats, err := repository.Info(context.TODO(), "")
		assert.NoError(t, err)

		t.Logf("RepositoryInfo: %+v", stats)

		assert.Equal(t, stats.NumberOfDatasets, int64(1))
		assert.Equal(t, stats.NumberOfDatasetRevisions, int64(2))
		assert.Equal(t, stats.NumberOfNamedGraphRevisions, int64(2))
		assert.Equal(t, stats.NumberOfCommits, int64(2))
	})

	// Create 2 NamedGraphs, commit 1 time
	t.Run("TestCase3", func(t *testing.T) {
		repository := setupTestEnvironment(t) // Initialize a new repository
		defer repository.Close()

		ngIDC1 := uuid.New()
		ngIDC2 := uuid.New()

		// Create two NamedGraphs and commit
		st := repository.OpenStage(sst.DefaultTriplexMode)

		ng1 := st.CreateNamedGraph(sst.IRI(ngIDC1.URN()))
		mainC1 := ng1.CreateIRINode("mainC1")
		mainC1.AddStatement(rdf.Type, rep.SchematicPort)

		ng2 := st.CreateNamedGraph(sst.IRI(ngIDC2.URN()))
		mainC2 := ng2.CreateIRINode("mainC2")
		mainC2.AddStatement(rdf.Type, rep.SchematicPort)

		st.Commit(context.TODO(), "Commit of 2 NamedGraphs", sst.DefaultBranch)

		// Get RepositoryInfo
		stats, err := repository.Info(context.TODO(), "")
		assert.NoError(t, err)

		t.Logf("RepositoryInfo: %+v", stats)

		// Verify statistics
		assert.Equal(t, stats.NumberOfDatasets, int64(2))
		assert.Equal(t, stats.NumberOfDatasetRevisions, int64(2))
		assert.Equal(t, stats.NumberOfNamedGraphRevisions, int64(2))
		assert.Equal(t, stats.NumberOfCommits, int64(1))
	})

	// Create 2 NamedGraphs, commit 1 time, modify one NamedGraph, then commit again
	t.Run("TestCase4", func(t *testing.T) {
		repository := setupTestEnvironment(t) // Initialize a new repository
		defer repository.Close()

		ngIDC1 := uuid.New()
		ngIDC2 := uuid.New()

		// Create two NamedGraphs and commit
		stageC := repository.OpenStage(sst.DefaultTriplexMode)

		ng1 := stageC.CreateNamedGraph(sst.IRI(ngIDC1.URN()))
		mainC1 := ng1.CreateIRINode("mainC1")
		mainC1.AddStatement(rdf.Type, rep.SchematicPort)

		ng2 := stageC.CreateNamedGraph(sst.IRI(ngIDC2.URN()))
		mainC2 := ng2.CreateIRINode("mainC2")
		mainC2.AddStatement(rdf.Type, rep.SchematicPort)

		stageC.Commit(context.TODO(), "Commit of 2 NamedGraphs", sst.DefaultBranch)

		// Modify one NamedGraph
		mainC1.AddStatement(rdf.Bag, rep.Angle)
		stageC.Commit(context.TODO(), "Commit after modifying NamedGraph 1", sst.DefaultBranch)

		// get RepositoryInfo
		stats, err := repository.Info(context.TODO(), "")
		assert.NoError(t, err)

		t.Logf("RepositoryInfo: %+v", stats)

		// Verify statistics
		assert.Equal(t, stats.NumberOfDatasets, int64(2))
		assert.Equal(t, stats.NumberOfDatasetRevisions, int64(3))
		assert.Equal(t, stats.NumberOfNamedGraphRevisions, int64(3))
		assert.Equal(t, stats.NumberOfCommits, int64(2))
	})

	// Create 2 NamedGraphs, commit 1 time, modify both NamedGraphs, then commit again
	t.Run("TestCase5", func(t *testing.T) {
		repository := setupTestEnvironment(t) // Initialize a new repository
		defer repository.Close()

		ngIDC1 := uuid.New()
		ngIDC2 := uuid.New()

		// Create two NamedGraphs and commit
		stageC := repository.OpenStage(sst.DefaultTriplexMode)

		ng1 := stageC.CreateNamedGraph(sst.IRI(ngIDC1.URN()))
		mainC1 := ng1.CreateIRINode("mainC1")
		mainC1.AddStatement(rdf.Type, rep.SchematicPort)

		ng2 := stageC.CreateNamedGraph(sst.IRI(ngIDC2.URN()))
		mainC2 := ng2.CreateIRINode("mainC2")
		mainC2.AddStatement(rdf.Type, rep.SchematicPort)

		stageC.Commit(context.TODO(), "Commit of 2 NamedGraphs", sst.DefaultBranch)

		// Modify both NamedGraphs
		mainC1.AddStatement(rdf.Bag, rep.Angle)
		mainC2.AddStatement(rdf.Bag, rep.Angle)
		stageC.Commit(context.TODO(), "Commit after modifying both NamedGraphs", sst.DefaultBranch)

		// Get RepositoryInfo
		stats, err := repository.Info(context.TODO(), "")
		assert.NoError(t, err)

		t.Logf("RepositoryInfo: %+v", stats)

		// Verify statistics
		assert.Equal(t, stats.NumberOfDatasets, int64(2))
		assert.Equal(t, stats.NumberOfDatasetRevisions, int64(4))
		assert.Equal(t, stats.NumberOfNamedGraphRevisions, int64(4))
		assert.Equal(t, stats.NumberOfCommits, int64(2))
	})
}
