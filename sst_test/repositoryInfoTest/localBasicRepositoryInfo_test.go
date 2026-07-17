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

func TestLocalBasicCreateNamedGraph(t *testing.T) {
	// Tests creating a single NamedGraph, committing it, and verifying RepositoryInfo statistics.
	dir := filepath.Join(t.TempDir(), t.Name())

	// Clean up old test data

	// Open repo
	repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", false)
	assert.NoError(t, err)
	repo.RegisterIndexHandler(defaultderive.DeriveInfo()) // Bleve
	defer repo.Close()

	// Create a NamedGraph and commit
	ngID := uuid.New()
	st := repo.OpenStage(sst.DefaultTriplexMode)
	ng := st.CreateNamedGraph(sst.IRI(ngID.URN()))
	assert.NoError(t, err)

	mainNode := ng.CreateIRINode("mainNode")
	mainNode.AddStatement(rdf.Type, rep.SchematicPort)

	_, _, err = st.Commit(context.TODO(), "Initial commit", sst.DefaultBranch)
	assert.NoError(t, err)

	// Verify RepositoryInfo
	stats, err := repo.Info(context.TODO(), "")
	assert.NoError(t, err)

	assert.Equal(t, stats.NumberOfDatasets, int64(1))
	assert.Equal(t, stats.NumberOfDatasetRevisions, int64(1))
	assert.Equal(t, stats.NumberOfNamedGraphRevisions, int64(1))
	assert.Equal(t, stats.NumberOfCommits, int64(0))
}

func TestLocalBasicModifyNamedGraph(t *testing.T) {
	// Tests modifying an existing NamedGraph, committing the changes, and verifying RepositoryInfo statistics.
	dir := filepath.Join(t.TempDir(), t.Name())

	// Clean up old test data

	// Open repo
	repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", false)
	assert.NoError(t, err)
	repo.RegisterIndexHandler(defaultderive.DeriveInfo()) // Bleve
	defer repo.Close()

	// Create a NamedGraph and commit
	ngID := uuid.New()
	st := repo.OpenStage(sst.DefaultTriplexMode)
	ng := st.CreateNamedGraph(sst.IRI(ngID.URN()))
	assert.NoError(t, err)

	mainNode := ng.CreateIRINode("mainNode")
	mainNode.AddStatement(rdf.Type, rep.SchematicPort)

	_, _, err = st.Commit(context.TODO(), "Initial commit", sst.DefaultBranch)
	assert.NoError(t, err)

	// Modify the NamedGraph and commit again
	mainNode.AddStatement(rdf.Bag, rep.Angle)
	_, _, err = st.Commit(context.TODO(), "Modified commit", sst.DefaultBranch)
	assert.NoError(t, err)

	// Verify RepositoryInfo
	stats, err := repo.Info(context.TODO(), "")
	assert.NoError(t, err)

	assert.Equal(t, stats.NumberOfDatasets, int64(1))
	assert.Equal(t, stats.NumberOfDatasetRevisions, int64(1))
	assert.Equal(t, stats.NumberOfNamedGraphRevisions, int64(1))
	assert.Equal(t, stats.NumberOfCommits, int64(0))
}

func TestLocalBasicMultipleNamedGraphs(t *testing.T) {
	// Tests creating multiple NamedGraphs, committing them together, and verifying RepositoryInfo statistics.
	dir := filepath.Join(t.TempDir(), t.Name())

	// Clean up old test data

	// Open repo
	repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", false)
	assert.NoError(t, err)
	repo.RegisterIndexHandler(defaultderive.DeriveInfo()) // Bleve
	defer repo.Close()

	// Create two NamedGraphs and commit
	ngID1 := uuid.New()
	ngID2 := uuid.New()

	st := repo.OpenStage(sst.DefaultTriplexMode)

	ng1 := st.CreateNamedGraph(sst.IRI(ngID1.URN()))
	assert.NoError(t, err)
	ng1.CreateIRINode("mainNode1").AddStatement(rdf.Type, rep.SchematicPort)

	ng2 := st.CreateNamedGraph(sst.IRI(ngID2.URN()))
	assert.NoError(t, err)
	ng2.CreateIRINode("mainNode2").AddStatement(rdf.Type, rep.SchematicPort)

	_, _, err = st.Commit(context.TODO(), "Commit two NamedGraphs", sst.DefaultBranch)
	assert.NoError(t, err)

	// Verify RepositoryInfo
	stats, err := repo.Info(context.TODO(), "")
	assert.NoError(t, err)

	assert.Equal(t, stats.NumberOfDatasets, int64(2))
	assert.Equal(t, stats.NumberOfDatasetRevisions, int64(2))
	assert.Equal(t, stats.NumberOfNamedGraphRevisions, int64(2))
	assert.Equal(t, stats.NumberOfCommits, int64(0))
}
