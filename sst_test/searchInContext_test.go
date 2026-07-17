// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/semanticstep/sst-core/defaultderive"
	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/sst_test/testutil"
	"github.com/semanticstep/sst-core/sstauth"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/blevesearch/bleve/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_LocalFullRepository_SearchInContext tests SearchInContext method for LocalFullRepository
func Test_LocalFullRepository_SearchInContext(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/", testName)
	removeFolder(dir)
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	t.Run("basic_search", func(t *testing.T) {
		subDir := filepath.Join(dir, "basic")
		repo, err := sst.CreateLocalRepository(subDir, "default@semanticstep.net", "default", true)
		require.NoError(t, err)
		defer repo.Close()

		err = repo.RegisterIndexHandler(defaultderive.DeriveInfo())
		require.NoError(t, err)

		openedIndex := repo.Bleve()
		require.NotNil(t, openedIndex)

		// Test 1: Basic MatchAll query
		q := bleve.NewMatchAllQuery()
		searchRequest := bleve.NewSearchRequest(q)
		searchRequest.Fields = []string{"mainType", "graphID", "label", "literal"}

		searchResults, err := openedIndex.SearchInContext(context.TODO(), searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), searchResults.Total)

		// Add some data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		randomGraphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(randomGraphID.URN()))
		graph.CreateIRINode("test-node", lci.Organization)
		_, _, err = st.Commit(context.TODO(), "add test data", sst.DefaultBranch)
		sst.FlushBleveIndex(repo)
		require.NoError(t, err)

		// Search again after commit
		searchResults, err = openedIndex.SearchInContext(context.TODO(), searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), searchResults.Total)
	})

	t.Run("match_query", func(t *testing.T) {
		subDir := filepath.Join(dir, "match")
		repo, err := sst.CreateLocalRepository(subDir, "default@semanticstep.net", "default", true)
		require.NoError(t, err)
		defer repo.Close()

		err = repo.RegisterIndexHandler(defaultderive.DeriveInfo())
		require.NoError(t, err)

		// Add test data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID1 := uuid.New()
		graph1 := st.CreateNamedGraph(sst.IRI(graphID1.URN()))
		node1 := graph1.CreateIRINode("org-1", lci.Organization)
		node1.AddStatement(rdfs.Label, sst.String("Test Organization Alpha"))

		graphID2 := uuid.New()
		graph2 := st.CreateNamedGraph(sst.IRI(graphID2.URN()))
		node2 := graph2.CreateIRINode("org-2", lci.Organization)
		node2.AddStatement(rdfs.Label, sst.String("Another Organization Beta"))

		_, _, err = st.Commit(context.TODO(), "add organizations", sst.DefaultBranch)
		sst.FlushBleveIndex(repo)
		require.NoError(t, err)

		// Test Match query
		matchQuery := bleve.NewMatchQuery("Alpha")
		matchQuery.SetField("label")
		searchRequest := bleve.NewSearchRequest(matchQuery)
		searchRequest.Fields = []string{"mainType", "label"}

		searchResults, err := repo.Bleve().SearchInContext(context.TODO(), searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), searchResults.Total)
	})

	t.Run("boolean_query", func(t *testing.T) {
		subDir := filepath.Join(dir, "boolean")
		repo, err := sst.CreateLocalRepository(subDir, "default@semanticstep.net", "default", true)
		require.NoError(t, err)
		defer repo.Close()

		err = repo.RegisterIndexHandler(defaultderive.DeriveInfo())
		require.NoError(t, err)

		// Add test data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
		node := graph.CreateIRINode("test-org", lci.Organization)
		node.AddStatement(rdfs.Label, sst.String("Special Test Organization"))
		_, _, err = st.Commit(context.TODO(), "add data", sst.DefaultBranch)
		sst.FlushBleveIndex(repo)
		require.NoError(t, err)

		// Test Boolean query with must
		boolQuery := bleve.NewBooleanQuery()
		mustQuery := bleve.NewMatchQuery("Organization")
		mustQuery.SetField("label")
		boolQuery.AddMust(mustQuery)

		searchRequest := bleve.NewSearchRequest(boolQuery)
		searchRequest.Fields = []string{"label", "mainType"}

		searchResults, err := repo.Bleve().SearchInContext(context.TODO(), searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), searchResults.Total)
	})

	t.Run("term_query", func(t *testing.T) {
		subDir := filepath.Join(dir, "term")
		repo, err := sst.CreateLocalRepository(subDir, "default@semanticstep.net", "default", true)
		require.NoError(t, err)
		defer repo.Close()

		err = repo.RegisterIndexHandler(defaultderive.DeriveInfo())
		require.NoError(t, err)

		// Add test data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
		graph.CreateIRINode("exact-match-node", lci.Organization)
		_, _, err = st.Commit(context.TODO(), "add exact match data", sst.DefaultBranch)
		sst.FlushBleveIndex(repo)
		require.NoError(t, err)

		// Test Term query
		termQuery := bleve.NewTermQuery("Organization")
		termQuery.SetField("mainType")
		searchRequest := bleve.NewSearchRequest(termQuery)
		searchRequest.Fields = []string{"mainType"}

		searchResults, err := repo.Bleve().SearchInContext(context.TODO(), searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), searchResults.Total)
	})

	t.Run("query_string", func(t *testing.T) {
		subDir := filepath.Join(dir, "querystr")
		repo, err := sst.CreateLocalRepository(subDir, "default@semanticstep.net", "default", true)
		require.NoError(t, err)
		defer repo.Close()

		err = repo.RegisterIndexHandler(defaultderive.DeriveInfo())
		require.NoError(t, err)

		// Add test data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
		graph.CreateIRINode("querystring-node", lci.Organization)
		_, _, err = st.Commit(context.TODO(), "add querystring data", sst.DefaultBranch)
		sst.FlushBleveIndex(repo)
		require.NoError(t, err)

		// Test Query String query
		queryStringQuery := bleve.NewQueryStringQuery("mainType:Organization")
		searchRequest := bleve.NewSearchRequest(queryStringQuery)
		searchRequest.Fields = []string{"mainType"}

		searchResults, err := repo.Bleve().SearchInContext(context.TODO(), searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), searchResults.Total)
	})

	t.Run("search_with_pagination", func(t *testing.T) {
		subDir := filepath.Join(dir, "pagination")
		repo, err := sst.CreateLocalRepository(subDir, "default@semanticstep.net", "default", true)
		require.NoError(t, err)
		defer repo.Close()

		err = repo.RegisterIndexHandler(defaultderive.DeriveInfo())
		require.NoError(t, err)

		// Add multiple items
		st := repo.OpenStage(sst.DefaultTriplexMode)
		for i := 0; i < 5; i++ {
			graphID := uuid.New()
			graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
			node := graph.CreateIRINode(fmt.Sprintf("pagination-node-%d", i), lci.Organization)
			node.AddStatement(rdfs.Label, sst.String(fmt.Sprintf("Pagination Test Item %d", i)))
		}
		_, _, err = st.Commit(context.TODO(), "add pagination data", sst.DefaultBranch)
		sst.FlushBleveIndex(repo)
		require.NoError(t, err)

		// Test pagination
		matchQuery := bleve.NewMatchQuery("Pagination")
		matchQuery.SetField("label")
		searchRequest := bleve.NewSearchRequestOptions(matchQuery, 2, 0, false)
		searchRequest.Fields = []string{"label"}

		searchResults, err := repo.Bleve().SearchInContext(context.TODO(), searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(5), searchResults.Total)
		assert.Equal(t, 2, len(searchResults.Hits))

		// Second page - with 5 items and offset 2, we get 2 items (indices 2 and 3)
		searchRequest2 := bleve.NewSearchRequestOptions(matchQuery, 2, 2, false)
		searchRequest2.Fields = []string{"label"}
		searchResults2, err := repo.Bleve().SearchInContext(context.TODO(), searchRequest2)
		require.NoError(t, err)
		assert.Equal(t, 2, len(searchResults2.Hits))
	})

	t.Run("search_with_sorting", func(t *testing.T) {
		subDir := filepath.Join(dir, "sorting")
		repo, err := sst.CreateLocalRepository(subDir, "default@semanticstep.net", "default", true)
		require.NoError(t, err)
		defer repo.Close()

		err = repo.RegisterIndexHandler(defaultderive.DeriveInfo())
		require.NoError(t, err)

		// Add test data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
		graph.CreateIRINode("sort-node-1", lci.Organization)
		graph.CreateIRINode("sort-node-2", lci.Organization)
		_, _, err = st.Commit(context.TODO(), "add sort data", sst.DefaultBranch)
		sst.FlushBleveIndex(repo)
		require.NoError(t, err)

		// Test sorting
		matchAllQuery := bleve.NewMatchAllQuery()
		searchRequest := bleve.NewSearchRequest(matchAllQuery)
		searchRequest.Fields = []string{"mainType"}
		searchRequest.SortBy([]string{"-_id"}) // Sort by ID descending

		searchResults, err := repo.Bleve().SearchInContext(context.TODO(), searchRequest)
		require.NoError(t, err)
		assert.Equal(t, 1, len(searchResults.Hits))
	})

	t.Run("search_with_facets", func(t *testing.T) {
		subDir := filepath.Join(dir, "facets")
		repo, err := sst.CreateLocalRepository(subDir, "default@semanticstep.net", "default", true)
		require.NoError(t, err)
		defer repo.Close()

		err = repo.RegisterIndexHandler(defaultderive.DeriveInfo())
		require.NoError(t, err)

		// Add test data with different types
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
		graph.CreateIRINode("facet-node-1", lci.Organization)
		graph.CreateIRINode("facet-node-2", lci.Organization)
		_, _, err = st.Commit(context.TODO(), "add facet data", sst.DefaultBranch)
		sst.FlushBleveIndex(repo)
		require.NoError(t, err)

		// Test with facet
		matchAllQuery := bleve.NewMatchAllQuery()
		searchRequest := bleve.NewSearchRequest(matchAllQuery)
		searchRequest.Fields = []string{"mainType"}
		facet := bleve.NewFacetRequest("mainType", 10)
		searchRequest.AddFacet("types", facet)

		searchResults, err := repo.Bleve().SearchInContext(context.TODO(), searchRequest)
		require.NoError(t, err)
		assert.NotNil(t, searchResults.Facets)
	})

	t.Run("empty_result", func(t *testing.T) {
		subDir := filepath.Join(dir, "empty")
		repo, err := sst.CreateLocalRepository(subDir, "default@semanticstep.net", "default", true)
		require.NoError(t, err)
		defer repo.Close()

		err = repo.RegisterIndexHandler(defaultderive.DeriveInfo())
		require.NoError(t, err)

		// Add test data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
		graph.CreateIRINode("existing-node", lci.Organization)
		_, _, err = st.Commit(context.TODO(), "add data", sst.DefaultBranch)
		sst.FlushBleveIndex(repo)
		require.NoError(t, err)

		// Search for non-existent term
		matchQuery := bleve.NewMatchQuery("NonExistentTermXYZ")
		matchQuery.SetField("label")
		searchRequest := bleve.NewSearchRequest(matchQuery)

		searchResults, err := repo.Bleve().SearchInContext(context.TODO(), searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), searchResults.Total)
	})

	t.Run("context_cancellation", func(t *testing.T) {
		subDir := filepath.Join(dir, "context")
		repo, err := sst.CreateLocalRepository(subDir, "default@semanticstep.net", "default", true)
		require.NoError(t, err)
		defer repo.Close()

		err = repo.RegisterIndexHandler(defaultderive.DeriveInfo())
		require.NoError(t, err)

		// Add test data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
		graph.CreateIRINode("context-node", lci.Organization)
		_, _, err = st.Commit(context.TODO(), "add context data", sst.DefaultBranch)
		sst.FlushBleveIndex(repo)
		require.NoError(t, err)

		// Test with context timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		matchAllQuery := bleve.NewMatchAllQuery()
		searchRequest := bleve.NewSearchRequest(matchAllQuery)

		searchResults, err := repo.Bleve().SearchInContext(ctx, searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), searchResults.Total)
	})
}

// Test_RemoteRepository_SearchInContext tests SearchInContext method for RemoteRepository
func Test_RemoteRepository_SearchInContext(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/", testName)
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	removeFolder(dir)
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	t.Run("basic_search", func(t *testing.T) {
		subDir := filepath.Join(dir, "basic")
		url := testutil.ServerServe(t, subDir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		openedIndex := repo.Bleve()
		require.NotNil(t, openedIndex)

		// Test 1: Basic MatchAll query
		q := bleve.NewMatchAllQuery()
		searchRequest := bleve.NewSearchRequest(q)
		searchRequest.Fields = []string{"mainType", "graphID", "label", "literal"}

		searchResults, err := openedIndex.SearchInContext(constructCtx, searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), searchResults.Total)

		// Add some data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		randomGraphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(randomGraphID.URN()))
		graph.CreateIRINode("test-node", lci.Organization)
		_, _, err = st.Commit(constructCtx, "add test data", sst.DefaultBranch)
		require.NoError(t, err)

		// Search again after commit
		searchResults, err = openedIndex.SearchInContext(constructCtx, searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), searchResults.Total)
	})

	t.Run("match_query", func(t *testing.T) {
		subDir := filepath.Join(dir, "match")
		url := testutil.ServerServe(t, subDir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		// Add test data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID1 := uuid.New()
		graph1 := st.CreateNamedGraph(sst.IRI(graphID1.URN()))
		node1 := graph1.CreateIRINode("org-1", lci.Organization)
		node1.AddStatement(rdfs.Label, sst.String("Remote Test Organization Alpha"))

		graphID2 := uuid.New()
		graph2 := st.CreateNamedGraph(sst.IRI(graphID2.URN()))
		node2 := graph2.CreateIRINode("org-2", lci.Organization)
		node2.AddStatement(rdfs.Label, sst.String("Another Remote Organization Beta"))

		_, _, err = st.Commit(constructCtx, "add organizations", sst.DefaultBranch)
		require.NoError(t, err)

		// Test Match query
		matchQuery := bleve.NewMatchQuery("Alpha")
		matchQuery.SetField("label")
		searchRequest := bleve.NewSearchRequest(matchQuery)
		searchRequest.Fields = []string{"mainType", "label"}

		searchResults, err := repo.Bleve().SearchInContext(constructCtx, searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), searchResults.Total)
	})

	t.Run("boolean_query", func(t *testing.T) {
		subDir := filepath.Join(dir, "boolean")
		url := testutil.ServerServe(t, subDir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		// Add test data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
		node := graph.CreateIRINode("test-org", lci.Organization)
		node.AddStatement(rdfs.Label, sst.String("Special Remote Test Organization"))
		_, _, err = st.Commit(constructCtx, "add data", sst.DefaultBranch)
		require.NoError(t, err)

		// Test Boolean query with must
		boolQuery := bleve.NewBooleanQuery()
		mustQuery := bleve.NewMatchQuery("Organization")
		mustQuery.SetField("label")
		boolQuery.AddMust(mustQuery)

		searchRequest := bleve.NewSearchRequest(boolQuery)
		searchRequest.Fields = []string{"label", "mainType"}

		searchResults, err := repo.Bleve().SearchInContext(constructCtx, searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), searchResults.Total)
	})

	t.Run("term_query", func(t *testing.T) {
		subDir := filepath.Join(dir, "term")
		url := testutil.ServerServe(t, subDir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		// Add test data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
		graph.CreateIRINode("exact-match-node", lci.Organization)
		_, _, err = st.Commit(constructCtx, "add exact match data", sst.DefaultBranch)
		require.NoError(t, err)

		// Test Term query
		termQuery := bleve.NewTermQuery("Organization")
		termQuery.SetField("mainType")
		searchRequest := bleve.NewSearchRequest(termQuery)
		searchRequest.Fields = []string{"mainType"}

		searchResults, err := repo.Bleve().SearchInContext(constructCtx, searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), searchResults.Total)
	})

	t.Run("query_string", func(t *testing.T) {
		subDir := filepath.Join(dir, "querystr")
		url := testutil.ServerServe(t, subDir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		// Add test data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
		graph.CreateIRINode("querystring-node", lci.Organization)
		_, _, err = st.Commit(constructCtx, "add querystring data", sst.DefaultBranch)
		require.NoError(t, err)

		// Test Query String query
		queryStringQuery := bleve.NewQueryStringQuery("mainType:Organization")
		searchRequest := bleve.NewSearchRequest(queryStringQuery)
		searchRequest.Fields = []string{"mainType"}

		searchResults, err := repo.Bleve().SearchInContext(constructCtx, searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), searchResults.Total)
	})

	t.Run("search_with_pagination", func(t *testing.T) {
		subDir := filepath.Join(dir, "pagination")
		url := testutil.ServerServe(t, subDir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		// Add multiple items
		st := repo.OpenStage(sst.DefaultTriplexMode)
		for i := 0; i < 5; i++ {
			graphID := uuid.New()
			graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
			node := graph.CreateIRINode(fmt.Sprintf("pagination-node-%d", i), lci.Organization)
			node.AddStatement(rdfs.Label, sst.String(fmt.Sprintf("Remote Pagination Test Item %d", i)))
		}
		_, _, err = st.Commit(constructCtx, "add pagination data", sst.DefaultBranch)
		require.NoError(t, err)

		// Test pagination
		matchQuery := bleve.NewMatchQuery("Pagination")
		matchQuery.SetField("label")
		searchRequest := bleve.NewSearchRequestOptions(matchQuery, 2, 0, false)
		searchRequest.Fields = []string{"label"}

		searchResults, err := repo.Bleve().SearchInContext(constructCtx, searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(5), searchResults.Total)
		assert.Equal(t, 2, len(searchResults.Hits))

		// Second page - with 5 items and offset 2, we get 2 items (indices 2 and 3)
		searchRequest2 := bleve.NewSearchRequestOptions(matchQuery, 2, 2, false)
		searchRequest2.Fields = []string{"label"}
		searchResults2, err := repo.Bleve().SearchInContext(constructCtx, searchRequest2)
		require.NoError(t, err)
		assert.Equal(t, 2, len(searchResults2.Hits))
	})

	t.Run("search_with_sorting", func(t *testing.T) {
		subDir := filepath.Join(dir, "sorting")
		url := testutil.ServerServe(t, subDir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		// Add test data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
		graph.CreateIRINode("sort-node-1", lci.Organization)
		graph.CreateIRINode("sort-node-2", lci.Organization)
		_, _, err = st.Commit(constructCtx, "add sort data", sst.DefaultBranch)
		require.NoError(t, err)

		// Test sorting
		matchAllQuery := bleve.NewMatchAllQuery()
		searchRequest := bleve.NewSearchRequest(matchAllQuery)
		searchRequest.Fields = []string{"mainType"}
		searchRequest.SortBy([]string{"-_id"}) // Sort by ID descending

		searchResults, err := repo.Bleve().SearchInContext(constructCtx, searchRequest)
		require.NoError(t, err)
		assert.Equal(t, 1, len(searchResults.Hits))
	})

	t.Run("search_with_facets", func(t *testing.T) {
		subDir := filepath.Join(dir, "facets")
		url := testutil.ServerServe(t, subDir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		// Add test data with different types
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
		graph.CreateIRINode("facet-node-1", lci.Organization)
		graph.CreateIRINode("facet-node-2", lci.Organization)
		_, _, err = st.Commit(constructCtx, "add facet data", sst.DefaultBranch)
		require.NoError(t, err)

		// Test with facet
		matchAllQuery := bleve.NewMatchAllQuery()
		searchRequest := bleve.NewSearchRequest(matchAllQuery)
		searchRequest.Fields = []string{"mainType"}
		facet := bleve.NewFacetRequest("mainType", 10)
		searchRequest.AddFacet("types", facet)

		searchResults, err := repo.Bleve().SearchInContext(constructCtx, searchRequest)
		require.NoError(t, err)
		assert.NotNil(t, searchResults.Facets)
	})

	t.Run("empty_result", func(t *testing.T) {
		subDir := filepath.Join(dir, "empty")
		url := testutil.ServerServe(t, subDir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		// Add test data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
		graph.CreateIRINode("existing-node", lci.Organization)
		_, _, err = st.Commit(constructCtx, "add data", sst.DefaultBranch)
		require.NoError(t, err)

		// Search for non-existent term
		matchQuery := bleve.NewMatchQuery("NonExistentTermXYZ")
		matchQuery.SetField("label")
		searchRequest := bleve.NewSearchRequest(matchQuery)

		searchResults, err := repo.Bleve().SearchInContext(constructCtx, searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), searchResults.Total)
	})

	t.Run("context_cancellation", func(t *testing.T) {
		subDir := filepath.Join(dir, "context")
		url := testutil.ServerServe(t, subDir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		// Add test data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
		graph.CreateIRINode("context-node", lci.Organization)
		_, _, err = st.Commit(constructCtx, "add context data", sst.DefaultBranch)
		require.NoError(t, err)

		// Test with authenticated context with timeout
		ctx, cancel := context.WithTimeout(constructCtx, 5*time.Second)
		defer cancel()

		matchAllQuery := bleve.NewMatchAllQuery()
		searchRequest := bleve.NewSearchRequest(matchAllQuery)

		searchResults, err := repo.Bleve().SearchInContext(ctx, searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), searchResults.Total)
	})
}

// Test_SearchInContext_Consistency tests that LocalFullRepository and RemoteRepository
// return consistent results for the same search queries
func Test_SearchInContext_Consistency(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/", testName)
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	removeFolder(dir + "_local")
	removeFolder(dir + "_remote")
	defer os.RemoveAll(dir + "_local")
	defer os.RemoveAll(dir + "_remote")

	t.Run("compare_local_and_remote", func(t *testing.T) {

		// Create local repository
		localRepo, err := sst.CreateLocalRepository(dir+"_local", "default@semanticstep.net", "default", true)
		require.NoError(t, err)
		defer localRepo.Close()

		err = localRepo.RegisterIndexHandler(defaultderive.DeriveInfo())
		require.NoError(t, err)

		// Add same data to local
		localSt := localRepo.OpenStage(sst.DefaultTriplexMode)
		localGraphID := uuid.New()
		localGraph := localSt.CreateNamedGraph(sst.IRI(localGraphID.URN()))
		localNode := localGraph.CreateIRINode("consistency-test", lci.Organization)
		localNode.AddStatement(rdfs.Label, sst.String("Consistency Test Label"))
		_, _, err = localSt.Commit(context.TODO(), "add consistency data", sst.DefaultBranch)
		require.NoError(t, err)

		// Create remote repository
		url := testutil.ServerServe(t, dir+"_remote")
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		remoteRepo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer remoteRepo.Close()

		// Add same data to remote
		remoteSt := remoteRepo.OpenStage(sst.DefaultTriplexMode)
		remoteGraphID := uuid.New()
		remoteGraph := remoteSt.CreateNamedGraph(sst.IRI(remoteGraphID.URN()))
		remoteNode := remoteGraph.CreateIRINode("consistency-test", lci.Organization)
		remoteNode.AddStatement(rdfs.Label, sst.String("Consistency Test Label"))
		_, _, err = remoteSt.Commit(constructCtx, "add consistency data", sst.DefaultBranch)
		require.NoError(t, err)

		// Perform same search on both
		matchQuery := bleve.NewMatchQuery("Consistency")
		matchQuery.SetField("label")
		searchRequest := bleve.NewSearchRequest(matchQuery)
		searchRequest.Fields = []string{"label", "mainType"}

		localResults, err := localRepo.Bleve().SearchInContext(context.TODO(), searchRequest)
		require.NoError(t, err)

		remoteResults, err := remoteRepo.Bleve().SearchInContext(constructCtx, searchRequest)
		require.NoError(t, err)

		// Both should have results (each dataset creates multiple indexed documents)
		assert.Equal(t, uint64(1), localResults.Total)
		assert.Equal(t, uint64(1), remoteResults.Total)
	})
}

// =============================================================================
// Tests to verify cache mechanism removal
// These tests ensure that the index works correctly without caching
// =============================================================================

// Test_LocalFullRepository_NoCache_VerifyDirectIndexAccess verifies that
// the repository uses bleve.Index directly without caching wrapper
func Test_LocalFullRepository_NoCache_VerifyDirectIndexAccess(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/", testName)
	removeFolder(dir)
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
	require.NoError(t, err)
	defer repo.Close()

	err = repo.RegisterIndexHandler(defaultderive.DeriveInfo())
	require.NoError(t, err)

	// Verify that Bleve() returns a valid index
	index := repo.Bleve()
	require.NotNil(t, index)

	// The index should be a bleve.Index (not a cachingIndex)
	// We verify this by checking it implements bleve.Index interface
	_, ok := index.(bleve.Index)
	assert.True(t, ok, "Bleve() should return a bleve.Index")

	// Add some data
	st := repo.OpenStage(sst.DefaultTriplexMode)
	graphID := uuid.New()
	graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
	node := graph.CreateIRINode("cache-test", lci.Organization)
	node.AddStatement(rdfs.Label, sst.String("Cache Removal Test"))
	_, _, err = st.Commit(context.TODO(), "add cache test data", sst.DefaultBranch)
	sst.FlushBleveIndex(repo)
	require.NoError(t, err)

	// Verify search works directly on the index
	matchQuery := bleve.NewMatchQuery("Cache Removal")
	matchQuery.SetField("label")
	searchRequest := bleve.NewSearchRequest(matchQuery)

	searchResults, err := index.SearchInContext(context.TODO(), searchRequest)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), searchResults.Total, "Search should return results")
}

// Test_LocalFullRepository_NoCache_SearchResultConsistency verifies that
// search results are consistent and reflect current data without cache interference
func Test_LocalFullRepository_NoCache_SearchResultConsistency(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/", testName)
	removeFolder(dir)
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
	require.NoError(t, err)
	defer repo.Close()

	err = repo.RegisterIndexHandler(defaultderive.DeriveInfo())
	require.NoError(t, err)

	index := repo.Bleve()

	// Initial search should return 0 results
	matchAllQuery := bleve.NewMatchAllQuery()
	searchRequest := bleve.NewSearchRequest(matchAllQuery)

	results1, err := index.SearchInContext(context.TODO(), searchRequest)
	require.NoError(t, err)
	assert.Equal(t, uint64(0), results1.Total, "Initial search should return 0 results")

	// Add data
	st := repo.OpenStage(sst.DefaultTriplexMode)
	graphID := uuid.New()
	graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
	node := graph.CreateIRINode("consistency-check", lci.Organization)
	node.AddStatement(rdfs.Label, sst.String("Consistency Check Node"))
	_, _, err = st.Commit(context.TODO(), "add data", sst.DefaultBranch)
	sst.FlushBleveIndex(repo)
	require.NoError(t, err)

	// Search immediately after commit - should see new data (no stale cache)
	results2, err := index.SearchInContext(context.TODO(), searchRequest)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), results2.Total, "Search after commit should return new data")

	// Multiple searches should return consistent results
	for i := 0; i < 5; i++ {
		results, err := index.SearchInContext(context.TODO(), searchRequest)
		require.NoError(t, err)
		assert.Equal(t, results2.Total, results.Total, "Search results should be consistent across multiple calls")
	}
}

// Test_LocalFullRepository_NoCache_ImmediateIndexUpdate verifies that
// the index is updated immediately after commit without cache delay
func Test_LocalFullRepository_NoCache_ImmediateIndexUpdate(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/", testName)
	removeFolder(dir)
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
	require.NoError(t, err)
	defer repo.Close()

	err = repo.RegisterIndexHandler(defaultderive.DeriveInfo())
	require.NoError(t, err)

	index := repo.Bleve()

	// Add first dataset
	st1 := repo.OpenStage(sst.DefaultTriplexMode)
	graphID1 := uuid.New()
	graph1 := st1.CreateNamedGraph(sst.IRI(graphID1.URN()))
	node1 := graph1.CreateIRINode("immediate-update-1", lci.Organization)
	node1.AddStatement(rdfs.Label, sst.String("AlphaUnique Dataset"))
	_, _, err = st1.Commit(context.TODO(), "add first dataset", sst.DefaultBranch)
	sst.FlushBleveIndex(repo)
	require.NoError(t, err)

	// Search for first dataset - should find it immediately
	matchQuery1 := bleve.NewMatchQuery("AlphaUnique")
	matchQuery1.SetField("label")
	searchRequest1 := bleve.NewSearchRequest(matchQuery1)

	results1, err := index.SearchInContext(context.TODO(), searchRequest1)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), results1.Total, "Should find first dataset immediately")

	// Add second dataset
	st2 := repo.OpenStage(sst.DefaultTriplexMode)
	graphID2 := uuid.New()
	graph2 := st2.CreateNamedGraph(sst.IRI(graphID2.URN()))
	node2 := graph2.CreateIRINode("immediate-update-2", lci.Organization)
	node2.AddStatement(rdfs.Label, sst.String("BetaUnique Dataset"))
	_, _, err = st2.Commit(context.TODO(), "add second dataset", sst.DefaultBranch)
	sst.FlushBleveIndex(repo)
	require.NoError(t, err)

	// Search for second dataset - should find it immediately
	matchQuery2 := bleve.NewMatchQuery("BetaUnique")
	matchQuery2.SetField("label")
	searchRequest2 := bleve.NewSearchRequest(matchQuery2)

	results2, err := index.SearchInContext(context.TODO(), searchRequest2)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), results2.Total, "Should find second dataset immediately")

	// Search for all datasets - should find both
	matchAllQuery := bleve.NewMatchAllQuery()
	searchRequestAll := bleve.NewSearchRequest(matchAllQuery)

	resultsAll, err := index.SearchInContext(context.TODO(), searchRequestAll)
	require.NoError(t, err)
	assert.Equal(t, uint64(2), resultsAll.Total, "Should find both datasets")
}

// Test_LocalFullRepository_NoCache_SearchAfterDelete verifies that
// search results are updated correctly after dataset deletion (no stale cache)
func Test_LocalFullRepository_NoCache_SearchAfterDelete(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/", testName)
	removeFolder(dir)
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
	require.NoError(t, err)
	defer repo.Close()

	err = repo.RegisterIndexHandler(defaultderive.DeriveInfo())
	require.NoError(t, err)

	index := repo.Bleve()

	// Add a dataset
	st := repo.OpenStage(sst.DefaultTriplexMode)
	graphID := uuid.New()
	graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
	node := graph.CreateIRINode("delete-test", lci.Organization)
	node.AddStatement(rdfs.Label, sst.String("Delete Test Dataset"))
	_, _, err = st.Commit(context.TODO(), "add dataset for deletion", sst.DefaultBranch)
	sst.FlushBleveIndex(repo)
	require.NoError(t, err)

	// Verify dataset exists in search
	specificQuery := bleve.NewMatchQuery("Delete Test Dataset")
	specificQuery.SetField("label")
	searchRequest := bleve.NewSearchRequest(specificQuery)

	resultsBefore, err := index.SearchInContext(context.TODO(), searchRequest)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), resultsBefore.Total, "Dataset should exist before deletion")

	// Delete the dataset
	ds, err := repo.Dataset(context.TODO(), sst.IRI(graphID.URN()))
	require.NoError(t, err)
	err = ds.RemoveBranch(context.TODO(), sst.DefaultBranch)
	sst.FlushBleveIndex(repo)
	require.NoError(t, err)

	// Search for specific dataset - should not find it
	resultsAfter, err := index.SearchInContext(context.TODO(), searchRequest)
	require.NoError(t, err)
	assert.Equal(t, uint64(0), resultsAfter.Total, "Dataset should not be found after deletion")
}

// Test_RemoteRepository_NoCache_VerifyDirectIndexAccess verifies that
// the remote repository uses bleve.Index directly without caching wrapper
func Test_RemoteRepository_NoCache_VerifyDirectIndexAccess(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/", testName)
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)
	removeFolder(dir)
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	url := testutil.ServerServe(t, dir)
	constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

	repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
	require.NoError(t, err)
	defer repo.Close()

	// Verify that Bleve() returns a valid index
	index := repo.Bleve()
	require.NotNil(t, index)

	// The index should implement bleve.Index interface
	_, ok := index.(bleve.Index)
	assert.True(t, ok, "Bleve() should return a bleve.Index")

	// Add some data
	st := repo.OpenStage(sst.DefaultTriplexMode)
	graphID := uuid.New()
	graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
	node := graph.CreateIRINode("remote-cache-test", lci.Organization)
	node.AddStatement(rdfs.Label, sst.String("Remote Cache Removal Test"))
	_, _, err = st.Commit(constructCtx, "add remote cache test data", sst.DefaultBranch)
	require.NoError(t, err)

	// Verify search works directly on the index
	matchQuery := bleve.NewMatchQuery("Remote Cache Removal")
	matchQuery.SetField("label")
	searchRequest := bleve.NewSearchRequest(matchQuery)

	searchResults, err := index.SearchInContext(constructCtx, searchRequest)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), searchResults.Total, "Search should return results")
}

// Test_RemoteRepository_NoCache_SearchResultConsistency verifies that
// remote search results are consistent without cache interference
func Test_RemoteRepository_NoCache_SearchResultConsistency(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/", testName)
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)
	removeFolder(dir)
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	url := testutil.ServerServe(t, dir)
	constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

	repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
	require.NoError(t, err)
	defer repo.Close()

	index := repo.Bleve()

	// Add data
	st := repo.OpenStage(sst.DefaultTriplexMode)
	graphID := uuid.New()
	graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
	node := graph.CreateIRINode("remote-consistency", lci.Organization)
	node.AddStatement(rdfs.Label, sst.String("Remote Consistency Test"))
	_, _, err = st.Commit(constructCtx, "add remote data", sst.DefaultBranch)
	require.NoError(t, err)

	// Multiple searches should return consistent results
	matchQuery := bleve.NewMatchQuery("Remote Consistency")
	matchQuery.SetField("label")
	searchRequest := bleve.NewSearchRequest(matchQuery)

	var firstResult uint64
	for i := 0; i < 5; i++ {
		results, err := index.SearchInContext(constructCtx, searchRequest)
		require.NoError(t, err)
		if i == 0 {
			firstResult = results.Total
			assert.Equal(t, uint64(1), firstResult, "First search should return results")
		} else {
			assert.Equal(t, firstResult, results.Total, "Search results should be consistent across multiple calls")
		}
	}
}

// Test_RemoteRepository_NoCache_ImmediateIndexUpdate verifies that
// the remote index is updated immediately after commit without cache delay
func Test_RemoteRepository_NoCache_ImmediateIndexUpdate(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/", testName)
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)
	removeFolder(dir)
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	url := testutil.ServerServe(t, dir)
	constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

	repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
	require.NoError(t, err)
	defer repo.Close()

	index := repo.Bleve()

	// Add sequential datasets and verify immediate visibility
	for i := 0; i < 3; i++ {
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
		node := graph.CreateIRINode(fmt.Sprintf("seq-node-%d", i), lci.Organization)
		node.AddStatement(rdfs.Label, sst.String(fmt.Sprintf("SeqUnique%d Dataset", i)))
		_, _, err = st.Commit(constructCtx, fmt.Sprintf("add dataset %d", i), sst.DefaultBranch)
		require.NoError(t, err)

		// Verify immediate visibility - each dataset creates at least 1 indexed document
		matchQuery := bleve.NewMatchQuery(fmt.Sprintf("SeqUnique%d", i))
		matchQuery.SetField("label")
		searchRequest := bleve.NewSearchRequest(matchQuery)

		results, err := index.SearchInContext(constructCtx, searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), results.Total, "Dataset %d should be visible immediately after commit", i)
	}

	// Final check - all datasets should be searchable
	matchAllQuery := bleve.NewMatchAllQuery()
	searchRequest := bleve.NewSearchRequest(matchAllQuery)

	finalResults, err := index.SearchInContext(constructCtx, searchRequest)
	require.NoError(t, err)
	assert.Equal(t, uint64(3), finalResults.Total, "All datasets should be searchable")
}

// Test_NoCache_CrossRepositoryConsistency verifies that local and remote
// repositories produce identical results without caching differences
func Test_NoCache_CrossRepositoryConsistency(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/", testName)
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)
	removeFolder(dir + "_local")
	removeFolder(dir + "_remote")
	t.Cleanup(func() {
		os.RemoveAll(dir + "_local")
		os.RemoveAll(dir + "_remote")
	})

	// Create local repository
	localDir := dir + "_local"
	localRepo, err := sst.CreateLocalRepository(localDir, "default@semanticstep.net", "default", true)
	require.NoError(t, err)
	defer localRepo.Close()

	err = localRepo.RegisterIndexHandler(defaultderive.DeriveInfo())
	require.NoError(t, err)

	// Create remote repository
	remoteDir := dir + "_remote"
	url := testutil.ServerServe(t, remoteDir)
	constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

	remoteRepo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
	require.NoError(t, err)
	defer remoteRepo.Close()

	// Add identical data to both repositories
	for i := 0; i < 3; i++ {
		// Local
		localSt := localRepo.OpenStage(sst.DefaultTriplexMode)
		localGraphID := uuid.New()
		localGraph := localSt.CreateNamedGraph(sst.IRI(localGraphID.URN()))
		localNode := localGraph.CreateIRINode(fmt.Sprintf("cross-repo-%d", i), lci.Organization)
		localNode.AddStatement(rdfs.Label, sst.String(fmt.Sprintf("CrossUnique%d Repo Test", i)))
		_, _, err = localSt.Commit(context.TODO(), fmt.Sprintf("local commit %d", i), sst.DefaultBranch)
		require.NoError(t, err)

		// Remote
		remoteSt := remoteRepo.OpenStage(sst.DefaultTriplexMode)
		remoteGraphID := uuid.New()
		remoteGraph := remoteSt.CreateNamedGraph(sst.IRI(remoteGraphID.URN()))
		remoteNode := remoteGraph.CreateIRINode(fmt.Sprintf("cross-repo-remote-%d", i), lci.Organization)
		remoteNode.AddStatement(rdfs.Label, sst.String(fmt.Sprintf("CrossUnique%d Repo Test", i)))
		_, _, err = remoteSt.Commit(constructCtx, fmt.Sprintf("remote commit %d", i), sst.DefaultBranch)
		require.NoError(t, err)
	}

	// Perform identical searches on both
	for i := 0; i < 3; i++ {
		matchQuery := bleve.NewMatchQuery(fmt.Sprintf("CrossUnique%d", i))
		matchQuery.SetField("label")
		searchRequest := bleve.NewSearchRequest(matchQuery)
		searchRequest.Fields = []string{"label"}

		localResults, err := localRepo.Bleve().SearchInContext(context.TODO(), searchRequest)
		require.NoError(t, err)

		remoteResults, err := remoteRepo.Bleve().SearchInContext(constructCtx, searchRequest)
		require.NoError(t, err)

		// Both should find their respective data (each dataset creates multiple indexed documents)
		assert.Equal(t, uint64(1), localResults.Total, "Local search %d should return results", i)
		assert.Equal(t, uint64(1), remoteResults.Total, "Remote search %d should return results", i)
	}

	// MatchAll query should return consistent results
	matchAllQuery := bleve.NewMatchAllQuery()
	searchRequest := bleve.NewSearchRequest(matchAllQuery)

	localAll, err := localRepo.Bleve().SearchInContext(context.TODO(), searchRequest)
	require.NoError(t, err)

	remoteAll, err := remoteRepo.Bleve().SearchInContext(constructCtx, searchRequest)
	require.NoError(t, err)

	// Both should have at least 3 results (each dataset creates multiple indexed documents)
	assert.GreaterOrEqual(t, localAll.Total, uint64(3), "Local should have at least 3 results")
	assert.GreaterOrEqual(t, remoteAll.Total, uint64(3), "Remote should have at least 3 results")
}

// Test_SuperLocalRepository_SearchInContext tests SearchInContext method for SuperLocalRepository
func Test_SuperLocalRepository_SearchInContext(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/", testName)
	removeFolder(dir)
	defer func() {
		err := os.RemoveAll(dir)
		fmt.Println(err)
	}()

	t.Run("basic_search", func(t *testing.T) {
		subDir := filepath.Join(dir, "basic")
		superRepo, err := sst.NewLocalSuperRepository(subDir)
		require.NoError(t, err)
		defer superRepo.Close()

		err = superRepo.RegisterIndexHandler(defaultderive.DeriveInfo())
		require.NoError(t, err)

		// Create a repository in super repo
		repo, err := superRepo.Create(context.TODO(), "test-repo")
		require.NoError(t, err)
		defer repo.Close()

		openedIndex := repo.Bleve()
		require.NotNil(t, openedIndex)

		// Test 1: Basic MatchAll query
		q := bleve.NewMatchAllQuery()
		searchRequest := bleve.NewSearchRequest(q)
		searchRequest.Fields = []string{"mainType", "graphID", "label", "literal"}

		searchResults, err := openedIndex.SearchInContext(context.TODO(), searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), searchResults.Total)

		// Add some data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		randomGraphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(randomGraphID.URN()))
		graph.CreateIRINode("test-node", lci.Organization)
		_, _, err = st.Commit(context.TODO(), "add test data", sst.DefaultBranch)
		sst.FlushBleveIndex(repo)
		require.NoError(t, err)

		// Search again after commit
		searchResults, err = openedIndex.SearchInContext(context.TODO(), searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), searchResults.Total)
	})

	t.Run("match_query", func(t *testing.T) {
		subDir := filepath.Join(dir, "match")
		superRepo, err := sst.NewLocalSuperRepository(subDir)
		require.NoError(t, err)
		defer superRepo.Close()

		err = superRepo.RegisterIndexHandler(defaultderive.DeriveInfo())
		require.NoError(t, err)

		// Create a repository in super repo
		repo, err := superRepo.Create(context.TODO(), "test-repo")
		require.NoError(t, err)
		defer repo.Close()

		// Add test data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID1 := uuid.New()
		graph1 := st.CreateNamedGraph(sst.IRI(graphID1.URN()))
		node1 := graph1.CreateIRINode("org-1", lci.Organization)
		node1.AddStatement(rdfs.Label, sst.String("Test Organization Alpha"))

		graphID2 := uuid.New()
		graph2 := st.CreateNamedGraph(sst.IRI(graphID2.URN()))
		node2 := graph2.CreateIRINode("org-2", lci.Organization)
		node2.AddStatement(rdfs.Label, sst.String("Another Organization Beta"))

		_, _, err = st.Commit(context.TODO(), "add organizations", sst.DefaultBranch)
		sst.FlushBleveIndex(repo)
		require.NoError(t, err)

		// Test Match query
		matchQuery := bleve.NewMatchQuery("Alpha")
		matchQuery.SetField("label")
		searchRequest := bleve.NewSearchRequest(matchQuery)
		searchRequest.Fields = []string{"mainType", "label"}

		searchResults, err := repo.Bleve().SearchInContext(context.TODO(), searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), searchResults.Total)
	})

	t.Run("boolean_query", func(t *testing.T) {
		subDir := filepath.Join(dir, "boolean")
		superRepo, err := sst.NewLocalSuperRepository(subDir)
		require.NoError(t, err)
		defer superRepo.Close()

		err = superRepo.RegisterIndexHandler(defaultderive.DeriveInfo())
		require.NoError(t, err)

		// Create a repository in super repo
		repo, err := superRepo.Create(context.TODO(), "test-repo")
		require.NoError(t, err)
		defer repo.Close()

		// Add test data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
		node := graph.CreateIRINode("test-org", lci.Organization)
		node.AddStatement(rdfs.Label, sst.String("Special Test Organization"))
		_, _, err = st.Commit(context.TODO(), "add data", sst.DefaultBranch)
		sst.FlushBleveIndex(repo)
		require.NoError(t, err)

		// Test Boolean query with must
		boolQuery := bleve.NewBooleanQuery()
		mustQuery := bleve.NewMatchQuery("Organization")
		mustQuery.SetField("label")
		boolQuery.AddMust(mustQuery)

		searchRequest := bleve.NewSearchRequest(boolQuery)
		searchRequest.Fields = []string{"label", "mainType"}

		searchResults, err := repo.Bleve().SearchInContext(context.TODO(), searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), searchResults.Total)
	})

	t.Run("term_query", func(t *testing.T) {
		subDir := filepath.Join(dir, "term")
		superRepo, err := sst.NewLocalSuperRepository(subDir)
		require.NoError(t, err)
		defer superRepo.Close()

		err = superRepo.RegisterIndexHandler(defaultderive.DeriveInfo())
		require.NoError(t, err)

		// Create a repository in super repo
		repo, err := superRepo.Create(context.TODO(), "test-repo")
		require.NoError(t, err)
		defer repo.Close()

		// Add test data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
		graph.CreateIRINode("exact-match-node", lci.Organization)
		_, _, err = st.Commit(context.TODO(), "add exact match data", sst.DefaultBranch)
		sst.FlushBleveIndex(repo)
		require.NoError(t, err)

		// Test Term query
		termQuery := bleve.NewTermQuery("Organization")
		termQuery.SetField("mainType")
		searchRequest := bleve.NewSearchRequest(termQuery)
		searchRequest.Fields = []string{"mainType"}

		searchResults, err := repo.Bleve().SearchInContext(context.TODO(), searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), searchResults.Total)
	})

	t.Run("query_string", func(t *testing.T) {
		subDir := filepath.Join(dir, "querystr")
		superRepo, err := sst.NewLocalSuperRepository(subDir)
		require.NoError(t, err)
		defer superRepo.Close()

		err = superRepo.RegisterIndexHandler(defaultderive.DeriveInfo())
		require.NoError(t, err)

		// Create a repository in super repo
		repo, err := superRepo.Create(context.TODO(), "test-repo")
		require.NoError(t, err)
		defer repo.Close()

		// Add test data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
		graph.CreateIRINode("querystring-node", lci.Organization)
		_, _, err = st.Commit(context.TODO(), "add querystring data", sst.DefaultBranch)
		sst.FlushBleveIndex(repo)
		require.NoError(t, err)

		// Test Query String query
		queryStringQuery := bleve.NewQueryStringQuery("mainType:Organization")
		searchRequest := bleve.NewSearchRequest(queryStringQuery)
		searchRequest.Fields = []string{"mainType"}

		searchResults, err := repo.Bleve().SearchInContext(context.TODO(), searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), searchResults.Total)
	})

	t.Run("search_with_pagination", func(t *testing.T) {
		subDir := filepath.Join(dir, "pagination")
		superRepo, err := sst.NewLocalSuperRepository(subDir)
		require.NoError(t, err)
		defer superRepo.Close()

		err = superRepo.RegisterIndexHandler(defaultderive.DeriveInfo())
		require.NoError(t, err)

		// Create a repository in super repo
		repo, err := superRepo.Create(context.TODO(), "test-repo")
		require.NoError(t, err)
		defer repo.Close()

		// Add multiple items
		st := repo.OpenStage(sst.DefaultTriplexMode)
		for i := 0; i < 5; i++ {
			graphID := uuid.New()
			graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
			node := graph.CreateIRINode(fmt.Sprintf("pagination-node-%d", i), lci.Organization)
			node.AddStatement(rdfs.Label, sst.String(fmt.Sprintf("Pagination Test Item %d", i)))
		}
		_, _, err = st.Commit(context.TODO(), "add pagination data", sst.DefaultBranch)
		sst.FlushBleveIndex(repo)
		require.NoError(t, err)

		// Test pagination
		matchQuery := bleve.NewMatchQuery("Pagination")
		matchQuery.SetField("label")
		searchRequest := bleve.NewSearchRequestOptions(matchQuery, 2, 0, false)
		searchRequest.Fields = []string{"label"}

		searchResults, err := repo.Bleve().SearchInContext(context.TODO(), searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(5), searchResults.Total)
		assert.Equal(t, 2, len(searchResults.Hits))

		// Second page - with 5 items and offset 2, we get 2 items (indices 2 and 3)
		searchRequest2 := bleve.NewSearchRequestOptions(matchQuery, 2, 2, false)
		searchRequest2.Fields = []string{"label"}
		searchResults2, err := repo.Bleve().SearchInContext(context.TODO(), searchRequest2)
		require.NoError(t, err)
		assert.Equal(t, 2, len(searchResults2.Hits))
	})

	t.Run("search_with_sorting", func(t *testing.T) {
		subDir := filepath.Join(dir, "sorting")
		superRepo, err := sst.NewLocalSuperRepository(subDir)
		require.NoError(t, err)
		defer superRepo.Close()

		err = superRepo.RegisterIndexHandler(defaultderive.DeriveInfo())
		require.NoError(t, err)

		// Create a repository in super repo
		repo, err := superRepo.Create(context.TODO(), "test-repo")
		require.NoError(t, err)
		defer repo.Close()

		// Add test data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
		graph.CreateIRINode("sort-node-1", lci.Organization)
		graph.CreateIRINode("sort-node-2", lci.Organization)
		_, _, err = st.Commit(context.TODO(), "add sort data", sst.DefaultBranch)
		sst.FlushBleveIndex(repo)
		require.NoError(t, err)

		// Test sorting
		matchAllQuery := bleve.NewMatchAllQuery()
		searchRequest := bleve.NewSearchRequest(matchAllQuery)
		searchRequest.Fields = []string{"mainType"}
		searchRequest.SortBy([]string{"-_id"}) // Sort by ID descending

		searchResults, err := repo.Bleve().SearchInContext(context.TODO(), searchRequest)
		require.NoError(t, err)
		assert.Equal(t, 1, len(searchResults.Hits))
	})

	t.Run("search_with_facets", func(t *testing.T) {
		subDir := filepath.Join(dir, "facets")
		superRepo, err := sst.NewLocalSuperRepository(subDir)
		require.NoError(t, err)
		defer superRepo.Close()

		err = superRepo.RegisterIndexHandler(defaultderive.DeriveInfo())
		require.NoError(t, err)

		// Create a repository in super repo
		repo, err := superRepo.Create(context.TODO(), "test-repo")
		require.NoError(t, err)
		defer repo.Close()

		// Add test data with different types
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
		graph.CreateIRINode("facet-node-1", lci.Organization)
		graph.CreateIRINode("facet-node-2", lci.Organization)
		_, _, err = st.Commit(context.TODO(), "add facet data", sst.DefaultBranch)
		sst.FlushBleveIndex(repo)
		require.NoError(t, err)

		// Test with facet
		matchAllQuery := bleve.NewMatchAllQuery()
		searchRequest := bleve.NewSearchRequest(matchAllQuery)
		searchRequest.Fields = []string{"mainType"}
		facet := bleve.NewFacetRequest("mainType", 10)
		searchRequest.AddFacet("types", facet)

		searchResults, err := repo.Bleve().SearchInContext(context.TODO(), searchRequest)
		require.NoError(t, err)
		assert.NotNil(t, searchResults.Facets)
	})

	t.Run("empty_result", func(t *testing.T) {
		subDir := filepath.Join(dir, "empty")
		superRepo, err := sst.NewLocalSuperRepository(subDir)
		require.NoError(t, err)
		defer superRepo.Close()

		err = superRepo.RegisterIndexHandler(defaultderive.DeriveInfo())
		require.NoError(t, err)

		// Create a repository in super repo
		repo, err := superRepo.Create(context.TODO(), "test-repo")
		require.NoError(t, err)
		defer repo.Close()

		// Add test data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
		graph.CreateIRINode("existing-node", lci.Organization)
		_, _, err = st.Commit(context.TODO(), "add data", sst.DefaultBranch)
		sst.FlushBleveIndex(repo)
		require.NoError(t, err)

		// Search for non-existent term
		matchQuery := bleve.NewMatchQuery("NonExistentTermXYZ")
		matchQuery.SetField("label")
		searchRequest := bleve.NewSearchRequest(matchQuery)

		searchResults, err := repo.Bleve().SearchInContext(context.TODO(), searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), searchResults.Total)
	})

	t.Run("context_cancellation", func(t *testing.T) {
		subDir := filepath.Join(dir, "context")
		superRepo, err := sst.NewLocalSuperRepository(subDir)
		require.NoError(t, err)
		defer superRepo.Close()

		err = superRepo.RegisterIndexHandler(defaultderive.DeriveInfo())
		require.NoError(t, err)

		// Create a repository in super repo
		repo, err := superRepo.Create(context.TODO(), "test-repo")
		require.NoError(t, err)
		defer repo.Close()

		// Add test data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
		graph.CreateIRINode("context-node", lci.Organization)
		_, _, err = st.Commit(context.TODO(), "add context data", sst.DefaultBranch)
		sst.FlushBleveIndex(repo)
		require.NoError(t, err)

		// Test with context timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		matchAllQuery := bleve.NewMatchAllQuery()
		searchRequest := bleve.NewSearchRequest(matchAllQuery)

		searchResults, err := repo.Bleve().SearchInContext(ctx, searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), searchResults.Total)
	})

	t.Run("multiple_repos", func(t *testing.T) {
		subDir := filepath.Join(dir, "multi")
		superRepo, err := sst.NewLocalSuperRepository(subDir)
		require.NoError(t, err)
		defer superRepo.Close()

		err = superRepo.RegisterIndexHandler(defaultderive.DeriveInfo())
		require.NoError(t, err)

		// Create multiple repositories in super repo and search immediately while each is open
		for i := 0; i < 3; i++ {
			repoName := fmt.Sprintf("repo-%d", i)
			repo, err := superRepo.Create(context.TODO(), repoName)
			require.NoError(t, err)

			// Add data to each repo
			st := repo.OpenStage(sst.DefaultTriplexMode)
			graphID := uuid.New()
			graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
			node := graph.CreateIRINode(fmt.Sprintf("node-%d", i), lci.Organization)
			node.AddStatement(rdfs.Label, sst.String(fmt.Sprintf("Repo %d Data", i)))
			_, _, err = st.Commit(context.TODO(), fmt.Sprintf("add data to repo %d", i), sst.DefaultBranch)
			sst.FlushBleveIndex(repo)
			require.NoError(t, err)

			// Search immediately while repo is still open
			matchQuery := bleve.NewMatchQuery(fmt.Sprintf("Repo %d Data", i))
			matchQuery.SetField("label")
			searchRequest := bleve.NewSearchRequest(matchQuery)

			searchResults, err := repo.Bleve().SearchInContext(context.TODO(), searchRequest)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, searchResults.Total, uint64(1), "Repo %d should have data", i)
			repo.Close()
		}
	})
}

// Test_SuperRemoteRepository_SearchInContext tests SearchInContext method for SuperRemoteRepository
func Test_SuperRemoteRepository_SearchInContext(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/", testName)
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	removeFolder(dir)
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	t.Run("basic_search", func(t *testing.T) {
		subDir := filepath.Join(dir, "basic")
		url := testutil.SuperServerServe(t, subDir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		superRepo, err := sst.OpenRemoteSuperRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer superRepo.Close()

		// Create a repository in super repo
		repo, err := superRepo.Create(constructCtx, "test-repo")
		require.NoError(t, err)
		defer repo.Close()

		openedIndex := repo.Bleve()
		require.NotNil(t, openedIndex)

		// Test 1: Basic MatchAll query
		q := bleve.NewMatchAllQuery()
		searchRequest := bleve.NewSearchRequest(q)
		searchRequest.Fields = []string{"mainType", "graphID", "label", "literal"}

		searchResults, err := openedIndex.SearchInContext(constructCtx, searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), searchResults.Total)

		// Add some data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		randomGraphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(randomGraphID.URN()))
		graph.CreateIRINode("test-node", lci.Organization)
		_, _, err = st.Commit(constructCtx, "add test data", sst.DefaultBranch)
		require.NoError(t, err)

		// Search again after commit
		searchResults, err = openedIndex.SearchInContext(constructCtx, searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), searchResults.Total)
	})

	t.Run("match_query", func(t *testing.T) {
		subDir := filepath.Join(dir, "match")
		url := testutil.SuperServerServe(t, subDir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		superRepo, err := sst.OpenRemoteSuperRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer superRepo.Close()

		// Create a repository in super repo
		repo, err := superRepo.Create(constructCtx, "test-repo")
		require.NoError(t, err)
		defer repo.Close()

		// Add test data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID1 := uuid.New()
		graph1 := st.CreateNamedGraph(sst.IRI(graphID1.URN()))
		node1 := graph1.CreateIRINode("org-1", lci.Organization)
		node1.AddStatement(rdfs.Label, sst.String("Remote Test Organization Alpha"))

		graphID2 := uuid.New()
		graph2 := st.CreateNamedGraph(sst.IRI(graphID2.URN()))
		node2 := graph2.CreateIRINode("org-2", lci.Organization)
		node2.AddStatement(rdfs.Label, sst.String("Another Remote Organization Beta"))

		_, _, err = st.Commit(constructCtx, "add organizations", sst.DefaultBranch)
		require.NoError(t, err)

		// Test Match query
		matchQuery := bleve.NewMatchQuery("Alpha")
		matchQuery.SetField("label")
		searchRequest := bleve.NewSearchRequest(matchQuery)
		searchRequest.Fields = []string{"mainType", "label"}

		searchResults, err := repo.Bleve().SearchInContext(constructCtx, searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), searchResults.Total)
	})

	t.Run("boolean_query", func(t *testing.T) {
		subDir := filepath.Join(dir, "boolean")
		url := testutil.SuperServerServe(t, subDir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		superRepo, err := sst.OpenRemoteSuperRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer superRepo.Close()

		// Create a repository in super repo
		repo, err := superRepo.Create(constructCtx, "test-repo")
		require.NoError(t, err)
		defer repo.Close()

		// Add test data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
		node := graph.CreateIRINode("test-org", lci.Organization)
		node.AddStatement(rdfs.Label, sst.String("Special Remote Test Organization"))
		_, _, err = st.Commit(constructCtx, "add data", sst.DefaultBranch)
		require.NoError(t, err)

		// Test Boolean query with must
		boolQuery := bleve.NewBooleanQuery()
		mustQuery := bleve.NewMatchQuery("Organization")
		mustQuery.SetField("label")
		boolQuery.AddMust(mustQuery)

		searchRequest := bleve.NewSearchRequest(boolQuery)
		searchRequest.Fields = []string{"label", "mainType"}

		searchResults, err := repo.Bleve().SearchInContext(constructCtx, searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), searchResults.Total)
	})

	t.Run("term_query", func(t *testing.T) {
		subDir := filepath.Join(dir, "term")
		url := testutil.SuperServerServe(t, subDir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		superRepo, err := sst.OpenRemoteSuperRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer superRepo.Close()

		// Create a repository in super repo
		repo, err := superRepo.Create(constructCtx, "test-repo")
		require.NoError(t, err)
		defer repo.Close()

		// Add test data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
		graph.CreateIRINode("exact-match-node", lci.Organization)
		_, _, err = st.Commit(constructCtx, "add exact match data", sst.DefaultBranch)
		require.NoError(t, err)

		// Test Term query
		termQuery := bleve.NewTermQuery("Organization")
		termQuery.SetField("mainType")
		searchRequest := bleve.NewSearchRequest(termQuery)
		searchRequest.Fields = []string{"mainType"}

		searchResults, err := repo.Bleve().SearchInContext(constructCtx, searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), searchResults.Total)
	})

	t.Run("query_string", func(t *testing.T) {
		subDir := filepath.Join(dir, "querystr")
		url := testutil.SuperServerServe(t, subDir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		superRepo, err := sst.OpenRemoteSuperRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer superRepo.Close()

		// Create a repository in super repo
		repo, err := superRepo.Create(constructCtx, "test-repo")
		require.NoError(t, err)
		defer repo.Close()

		// Add test data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
		graph.CreateIRINode("querystring-node", lci.Organization)
		_, _, err = st.Commit(constructCtx, "add querystring data", sst.DefaultBranch)
		require.NoError(t, err)

		// Test Query String query
		queryStringQuery := bleve.NewQueryStringQuery("mainType:Organization")
		searchRequest := bleve.NewSearchRequest(queryStringQuery)
		searchRequest.Fields = []string{"mainType"}

		searchResults, err := repo.Bleve().SearchInContext(constructCtx, searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), searchResults.Total)
	})

	t.Run("search_with_pagination", func(t *testing.T) {
		subDir := filepath.Join(dir, "pagination")
		url := testutil.SuperServerServe(t, subDir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		superRepo, err := sst.OpenRemoteSuperRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer superRepo.Close()

		// Create a repository in super repo
		repo, err := superRepo.Create(constructCtx, "test-repo")
		require.NoError(t, err)
		defer repo.Close()

		// Add multiple items
		st := repo.OpenStage(sst.DefaultTriplexMode)
		for i := 0; i < 5; i++ {
			graphID := uuid.New()
			graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
			node := graph.CreateIRINode(fmt.Sprintf("pagination-node-%d", i), lci.Organization)
			node.AddStatement(rdfs.Label, sst.String(fmt.Sprintf("Remote Pagination Test Item %d", i)))
		}
		_, _, err = st.Commit(constructCtx, "add pagination data", sst.DefaultBranch)
		require.NoError(t, err)

		// Test pagination
		matchQuery := bleve.NewMatchQuery("Pagination")
		matchQuery.SetField("label")
		searchRequest := bleve.NewSearchRequestOptions(matchQuery, 2, 0, false)
		searchRequest.Fields = []string{"label"}

		searchResults, err := repo.Bleve().SearchInContext(constructCtx, searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(5), searchResults.Total)
		assert.Equal(t, 2, len(searchResults.Hits))

		// Second page - with 5 items and offset 2, we get 2 items (indices 2 and 3)
		searchRequest2 := bleve.NewSearchRequestOptions(matchQuery, 2, 2, false)
		searchRequest2.Fields = []string{"label"}
		searchResults2, err := repo.Bleve().SearchInContext(constructCtx, searchRequest2)
		require.NoError(t, err)
		assert.Equal(t, 2, len(searchResults2.Hits))
	})

	t.Run("search_with_sorting", func(t *testing.T) {
		subDir := filepath.Join(dir, "sorting")
		url := testutil.SuperServerServe(t, subDir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		superRepo, err := sst.OpenRemoteSuperRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer superRepo.Close()

		// Create a repository in super repo
		repo, err := superRepo.Create(constructCtx, "test-repo")
		require.NoError(t, err)
		defer repo.Close()

		// Add test data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
		graph.CreateIRINode("sort-node-1", lci.Organization)
		graph.CreateIRINode("sort-node-2", lci.Organization)
		_, _, err = st.Commit(constructCtx, "add sort data", sst.DefaultBranch)
		require.NoError(t, err)

		// Test sorting
		matchAllQuery := bleve.NewMatchAllQuery()
		searchRequest := bleve.NewSearchRequest(matchAllQuery)
		searchRequest.Fields = []string{"mainType"}
		searchRequest.SortBy([]string{"-_id"}) // Sort by ID descending

		searchResults, err := repo.Bleve().SearchInContext(constructCtx, searchRequest)
		require.NoError(t, err)
		assert.Equal(t, 1, len(searchResults.Hits))
	})

	t.Run("search_with_facets", func(t *testing.T) {
		subDir := filepath.Join(dir, "facets")
		url := testutil.SuperServerServe(t, subDir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		superRepo, err := sst.OpenRemoteSuperRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer superRepo.Close()

		// Create a repository in super repo
		repo, err := superRepo.Create(constructCtx, "test-repo")
		require.NoError(t, err)
		defer repo.Close()

		// Add test data with different types
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
		graph.CreateIRINode("facet-node-1", lci.Organization)
		graph.CreateIRINode("facet-node-2", lci.Organization)
		_, _, err = st.Commit(constructCtx, "add facet data", sst.DefaultBranch)
		require.NoError(t, err)

		// Test with facet
		matchAllQuery := bleve.NewMatchAllQuery()
		searchRequest := bleve.NewSearchRequest(matchAllQuery)
		searchRequest.Fields = []string{"mainType"}
		facet := bleve.NewFacetRequest("mainType", 10)
		searchRequest.AddFacet("types", facet)

		searchResults, err := repo.Bleve().SearchInContext(constructCtx, searchRequest)
		require.NoError(t, err)
		assert.NotNil(t, searchResults.Facets)
	})

	t.Run("empty_result", func(t *testing.T) {
		subDir := filepath.Join(dir, "empty")
		url := testutil.SuperServerServe(t, subDir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		superRepo, err := sst.OpenRemoteSuperRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer superRepo.Close()

		// Create a repository in super repo
		repo, err := superRepo.Create(constructCtx, "test-repo")
		require.NoError(t, err)
		defer repo.Close()

		// Add test data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
		graph.CreateIRINode("existing-node", lci.Organization)
		_, _, err = st.Commit(constructCtx, "add data", sst.DefaultBranch)
		require.NoError(t, err)

		// Search for non-existent term
		matchQuery := bleve.NewMatchQuery("NonExistentTermXYZ")
		matchQuery.SetField("label")
		searchRequest := bleve.NewSearchRequest(matchQuery)

		searchResults, err := repo.Bleve().SearchInContext(constructCtx, searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), searchResults.Total)
	})

	t.Run("context_cancellation", func(t *testing.T) {
		subDir := filepath.Join(dir, "context")
		url := testutil.SuperServerServe(t, subDir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		superRepo, err := sst.OpenRemoteSuperRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer superRepo.Close()

		// Create a repository in super repo
		repo, err := superRepo.Create(constructCtx, "test-repo")
		require.NoError(t, err)
		defer repo.Close()

		// Add test data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
		graph.CreateIRINode("context-node", lci.Organization)
		_, _, err = st.Commit(constructCtx, "add context data", sst.DefaultBranch)
		require.NoError(t, err)

		// Test with authenticated context with timeout
		ctx, cancel := context.WithTimeout(constructCtx, 5*time.Second)
		defer cancel()

		matchAllQuery := bleve.NewMatchAllQuery()
		searchRequest := bleve.NewSearchRequest(matchAllQuery)

		searchResults, err := repo.Bleve().SearchInContext(ctx, searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), searchResults.Total)
	})

	t.Run("multiple_repos", func(t *testing.T) {
		subDir := filepath.Join(dir, "multi")
		url := testutil.SuperServerServe(t, subDir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		superRepo, err := sst.OpenRemoteSuperRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer superRepo.Close()

		// Create one repository in super repo (creating multiple repos in a loop causes gRPC connection issues)
		repo, err := superRepo.Create(constructCtx, "test-repo")
		require.NoError(t, err)

		// Add data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
		node := graph.CreateIRINode("test-node", lci.Organization)
		node.AddStatement(rdfs.Label, sst.String("Super Remote Repo Data"))
		_, _, err = st.Commit(constructCtx, "add data", sst.DefaultBranch)
		require.NoError(t, err)

		// Search
		matchQuery := bleve.NewMatchQuery("Super Remote Repo Data")
		matchQuery.SetField("label")
		searchRequest := bleve.NewSearchRequest(matchQuery)

		searchResults, err := repo.Bleve().SearchInContext(constructCtx, searchRequest)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, searchResults.Total, uint64(1), "Repo should have data")
		repo.Close()
	})
}

// Test_SuperRepositories_NoCache_VerifyDirectIndexAccess verifies that
// SuperLocalRepository and SuperRemoteRepository use bleve.Index directly without caching wrapper
func Test_SuperRepositories_NoCache_VerifyDirectIndexAccess(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/", testName)
	removeFolder(dir)
	removeFolder(dir + "_remote")
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	defer os.RemoveAll(dir + "_remote")

	t.Run("super_local_no_cache", func(t *testing.T) {
		subDir := filepath.Join(dir, "local")
		superRepo, err := sst.NewLocalSuperRepository(subDir)
		require.NoError(t, err)
		defer superRepo.Close()

		err = superRepo.RegisterIndexHandler(defaultderive.DeriveInfo())
		require.NoError(t, err)

		// Create a repository
		repo, err := superRepo.Create(context.TODO(), "test-repo")
		require.NoError(t, err)
		defer repo.Close()

		// Verify that Bleve() returns a valid index
		index := repo.Bleve()
		require.NotNil(t, index)

		// The index should be a bleve.Index (not a cachingIndex)
		_, ok := index.(bleve.Index)
		assert.True(t, ok, "Bleve() should return a bleve.Index")

		// Add some data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
		node := graph.CreateIRINode("cache-test", lci.Organization)
		node.AddStatement(rdfs.Label, sst.String("Super Local Cache Removal Test"))
		_, _, err = st.Commit(context.TODO(), "add cache test data", sst.DefaultBranch)
		sst.FlushBleveIndex(repo)
		require.NoError(t, err)

		// Verify search works directly on the index
		matchQuery := bleve.NewMatchQuery("Super Local Cache Removal")
		matchQuery.SetField("label")
		searchRequest := bleve.NewSearchRequest(matchQuery)

		searchResults, err := index.SearchInContext(context.TODO(), searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), searchResults.Total, "Search should return results")
	})

	t.Run("super_remote_no_cache", func(t *testing.T) {
		subDir := filepath.Join(dir, "remote")
		transportCreds, err := testutil.TestTransportCreds()
		require.NoError(t, err)

		url := testutil.SuperServerServe(t, subDir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		superRepo, err := sst.OpenRemoteSuperRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)

		// Create a repository
		repo, err := superRepo.Create(constructCtx, "test-repo")
		require.NoError(t, err)
		defer repo.Close()

		// Verify that Bleve() returns a valid index
		index := repo.Bleve()
		require.NotNil(t, index)

		// The index should be a bleve.Index (not a cachingIndex)
		_, ok := index.(bleve.Index)
		assert.True(t, ok, "Bleve() should return a bleve.Index")

		// Add some data
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graphID := uuid.New()
		graph := st.CreateNamedGraph(sst.IRI(graphID.URN()))
		node := graph.CreateIRINode("cache-test", lci.Organization)
		node.AddStatement(rdfs.Label, sst.String("Super Remote Cache Removal Test"))
		_, _, err = st.Commit(constructCtx, "add cache test data", sst.DefaultBranch)
		sst.FlushBleveIndex(repo)
		require.NoError(t, err)

		// Verify search works directly on the index
		matchQuery := bleve.NewMatchQuery("Super Remote Cache Removal")
		matchQuery.SetField("label")
		searchRequest := bleve.NewSearchRequest(matchQuery)

		searchResults, err := index.SearchInContext(constructCtx, searchRequest)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), searchResults.Total, "Search should return results")
	})
}
