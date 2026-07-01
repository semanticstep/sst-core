// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

// The reason why there is a separate package is about RegisterVocabularyMap.
// In sst_test_dummyvoc package, it imports package compiler, which will register a test VocabularyMap.
// In sst_test_realvoc package, it imports package vocabularies, which will register a real
// VocabularyMap that includes lci, owl...

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/blevesearch/bleve/v2"
	"github.com/google/uuid"
	"github.com/semanticstep/sst-core/defaultderive"
	"github.com/semanticstep/sst-core/sst"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_SuperLocalFullRepository_URL(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())

	superRepo, err := sst.NewLocalSuperRepository(dir)
	require.NoError(t, err)
	defer superRepo.Close()

	// Test that URL() returns the expected file:// URL with the directory path
	repoURL := superRepo.URL()
	assert.True(t, len(repoURL) > 0, "URL should not be empty")
	assert.Contains(t, repoURL, "file://", "URL should contain file:// scheme")
	t.Logf("Local SuperRepository URL: %s", repoURL)
}

func Test_SuperLocalFullRepository_UUIDNamedGraph(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngIDC := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363")

	var commitHash sst.Hash
	var modifiedDSIDs []uuid.UUID

	t.Run("write", func(t *testing.T) {

		superRepo, err := sst.NewLocalSuperRepository(dir)
		if err != nil {
			panic(err)
		}
		defer superRepo.Close()
		err = superRepo.RegisterIndexHandler(defaultderive.DeriveInfo())
		assert.Nil(t, err)

		// create repo A
		repoA, err := superRepo.Create(context.TODO(), "A")
		if err != nil {
			panic(err)
		}
		defer repoA.Close()

		stageA := repoA.OpenStage(sst.DefaultTriplexMode)

		ngAC := stageA.CreateNamedGraph(sst.IRI(ngIDC.URN()))

		assert.Equal(t, sst.IRI("urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f613a363"), ngAC.IRI())

		mainAC := ngAC.CreateIRINode("mainAC")
		assert.True(t, mainAC.IsIRINode())
		assert.False(t, mainAC.IsBlankNode())
		assert.False(t, mainAC.IsTermCollection())

		mainAC.AddStatement(rdf.Type, rep.SchematicPort)
		mainAC.AddStatement(rdf.Bag, rep.Angle)

		commitHash, modifiedDSIDs, err = stageA.Commit(context.TODO(), "First commit of AC", sst.DefaultBranch)
		assert.Nil(t, err)
		assert.Equal(t, len(modifiedDSIDs), 1)
		assert.Equal(t, modifiedDSIDs[0], ngIDC)
		fmt.Println("commitHash in repo A:", commitHash.String())

		openedIndex := repoA.Bleve()
		query := bleve.NewMatchAllQuery()
		searchRequest := bleve.NewSearchRequest(query)
		searchResults, err := openedIndex.SearchInContext(context.TODO(), searchRequest)
		if err != nil {
			panic(err)
		}
		fmt.Println("repoA", searchResults)

		repoDefault, err := superRepo.Get(context.TODO(), "default")
		if err != nil {
			panic(err)
		}

		openedIndexDefault := repoDefault.Bleve()
		queryDefault := bleve.NewMatchAllQuery()
		searchRequestDefault := bleve.NewSearchRequest(queryDefault)
		searchResultsDefault, err := openedIndexDefault.SearchInContext(context.TODO(), searchRequestDefault)
		if err != nil {
			panic(err)
		}
		fmt.Println("Default", searchResultsDefault)

		// create repo B
		repoB, err := superRepo.Create(context.TODO(), "B")
		if err != nil {
			panic(err)
		}
		defer repoB.Close()

		stageB := repoB.OpenStage(sst.DefaultTriplexMode)

		ngBC := stageB.CreateNamedGraph(sst.IRI(ngIDC.URN()))

		assert.Equal(t, sst.IRI("urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f613a363"), ngAC.IRI())

		mainBC := ngBC.CreateIRINode("mainBC")
		assert.True(t, mainBC.IsIRINode())
		assert.False(t, mainBC.IsBlankNode())
		assert.False(t, mainBC.IsTermCollection())

		mainBC.AddStatement(rdf.Type, rep.SchematicPort)
		mainBC.AddStatement(rdf.Bag, rep.Angle)

		commitHash, modifiedDSIDs, err = stageB.Commit(context.TODO(), "First commit of BC", sst.DefaultBranch)
		assert.Nil(t, err)
		assert.Equal(t, len(modifiedDSIDs), 1)
		assert.Equal(t, modifiedDSIDs[0], ngIDC)
		fmt.Println("commitHash in repoB:", commitHash.String())
	})

	t.Run("repo A", func(t *testing.T) {
		superRepo, err := sst.NewLocalSuperRepository(dir)
		if err != nil {
			panic(err)
		}
		err = superRepo.RegisterIndexHandler(defaultderive.DeriveInfo())
		assert.Nil(t, err)

		repo, err := superRepo.Get(context.TODO(), "A")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), sst.IRI(ngIDC.URN()))
		assert.Nil(t, err)

		cd, err := ds.CommitDetailsByBranch(context.TODO(), sst.DefaultBranch)
		assert.Nil(t, err)

		cd.Dump()
	})

	t.Run("repo B", func(t *testing.T) {
		superRepo, err := sst.NewLocalSuperRepository(dir)
		if err != nil {
			panic(err)
		}
		err = superRepo.RegisterIndexHandler(defaultderive.DeriveInfo())
		assert.Nil(t, err)

		repo, err := superRepo.Get(context.TODO(), "B")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), sst.IRI(ngIDC.URN()))
		assert.Nil(t, err)

		cd, err := ds.CommitDetailsByBranch(context.TODO(), sst.DefaultBranch)
		assert.Nil(t, err)

		cd.Dump()
	})
}
