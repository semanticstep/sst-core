// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

/*
HelloWorldRepoLocalBasic is an example SST application that demonstrates how to
create, open, and interact with a LocalBasic repository. It includes tests for
creating a repository, adding data with typed nodes, committing changes, and
performing Bleve searches.
*/
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/blevesearch/bleve/v2"
	"github.com/google/uuid"
	"github.com/semanticstep/sst-core/defaultderive"
	"github.com/semanticstep/sst-core/sst"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/stretchr/testify/assert"
)

// TestOpen demonstrates creating a LocalBasic repository, adding data, committing,
// reopening it, and performing a Bleve search.
func TestOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repo")
	id := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a369")

	ttlFile := filepath.Join(t.TempDir(), "repolocalbasic.ttl")

	t.Run("Create Repository", func(t *testing.T) {
		id := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a369")

		// creates a new repository
		repo, err := sst.CreateLocalRepository(path, "", "", false)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		err = repo.RegisterIndexHandler(defaultderive.DeriveInfo())
		if err != nil {
			panic(err)
		}

		stage := repo.OpenStage(sst.DefaultTriplexMode)
		graph := stage.CreateNamedGraph(sst.IRI(id.URN()))

		main := graph.CreateIRINode("main")

		main.AddStatement(rdf.Type, lci.Organization)
		main.AddStatement(rdfs.Label, sst.String("ABC Ltd."))
		stage.Commit(context.TODO(), "first commit: add main code", sst.DefaultBranch)

		secondary := graph.CreateIRINode("secondary")

		secondary.AddStatement(rdf.Type, lci.Person)
		secondary.AddStatement(rdfs.Label, sst.String("Adam"))
		stage.Commit(context.TODO(), "second commit: add secondary code", sst.DefaultBranch)

		f, err := os.Create(ttlFile)
		if err != nil {
			panic(err)
		}
		defer f.Close()

		graph.RdfWrite(f, sst.RdfFormatTurtle)
	})

	t.Run("Step1", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(path, "", "")

		assert.NoError(t, err)
		defer repo.Close()

		dataset, err := repo.Dataset(context.TODO(), sst.IRI(id.URN()))
		assert.NoError(t, err)

		stage, err := dataset.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		assert.NoError(t, err)

		g := stage.NamedGraph(sst.IRI(id.URN()))
		assert.NoError(t, err)
		assert.Equal(t, 3, g.IRINodeCount())
	})
	t.Run("Step2", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(path, "", "")
		assert.NoError(t, err)
		defer repo.Close()

		dataset, err := repo.Dataset(context.TODO(), sst.IRI(id.URN()))
		assert.NoError(t, err)

		stage, err := dataset.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		assert.NoError(t, err)

		g := stage.NamedGraph(sst.IRI(id.URN()))
		assert.NoError(t, err)
		assert.Equal(t, 3, g.IRINodeCount())
	})

	t.Run("Step3", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(path, "", "")
		assert.NoError(t, err)
		defer repo.Close()

		err = repo.RegisterIndexHandler(defaultderive.DeriveInfo())
		if err != nil {
			panic(err)
		}

		openedIndex := repo.Bleve()
		query := bleve.NewMatchAllQuery()
		searchRequest := bleve.NewSearchRequest(query)

		searchResults, err := openedIndex.SearchInContext(context.TODO(), searchRequest)
		if err != nil {
			panic(err)
		}
		fmt.Println(searchResults)
	})
}
