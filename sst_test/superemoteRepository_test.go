// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

// The reason why there is a separate package is about RegisterVocabularyMap.
// In sst_test_dummyvoc package, it imports package compiler, which will register a test VocabularyMap.
// In sst_test_realvoc package, it imports package vocabularies, which will register a real
// VocabularyMap that includes lci, owl...

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"testing"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/google/uuid"
	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/sst_test/testutil"
	"github.com/semanticstep/sst-core/sstauth"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_SuperRemoteRepository_URL(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	url := testutil.SuperServerServe(t, dir)
	constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
	super, err := sst.OpenRemoteSuperRepository(constructCtx, url, transportCreds)
	require.NoError(t, err)
	defer super.Close()

	// Test that URL() returns the expected grpc:// URL
	repoURL := super.URL()
	assert.True(t, len(repoURL) > 0, "URL should not be empty")
	assert.Contains(t, repoURL, url, "URL should contain the server address")
	t.Logf("Remote SuperRepository URL: %s", repoURL)
}

func Test_SuperRemoteRepository_CRUD(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngIDC := uuid.MustParse("123e4567-e89b-12d3-a456-42661417400c")
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	t.Run("write", func(t *testing.T) {
		// sst.AtomicLevel.SetLevel(zap.DebugLevel)
		url := testutil.SuperServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		super, err := sst.OpenRemoteSuperRepository(constructCtx, url, transportCreds)
		if err != nil {
			log.Fatal(err)
		}
		defer super.Close()

		repoA, err := super.Create(constructCtx, "repoA")
		assert.NoError(t, err)

		st := repoA.OpenStage(sst.DefaultTriplexMode)

		ng := st.CreateNamedGraph(sst.IRI(ngIDC.URN()))

		ib := ng.CreateIRINode("John", lci.Person)
		assert.NotNil(t, ib)

		commitHash, influenceDatasets, err := st.Commit(constructCtx, "Initial commit", sst.DefaultBranch)
		assert.NoError(t, err)
		fmt.Println("commitHash:", commitHash)
		fmt.Println("influenceDatasets:", influenceDatasets)

		info, err := repoA.Info(constructCtx, sst.DefaultBranch)
		assert.NoError(t, err)
		assert.NotNil(t, info)
		assert.NotEqual(t, "", info.BleveName)
		assert.NotEqual(t, "", info.BleveVersion)
		assert.True(t, info.IsRemote)
		assert.Equal(t, int64(1), info.NumberOfDatasets)
		assert.Equal(t, int64(1), info.NumberOfDatasetsInBranch)
		assert.Equal(t, int64(1), info.NumberOfCommits)
		assert.Equal(t, int64(1), info.NumberOfNamedGraphRevisions)
		assert.GreaterOrEqual(t, info.MasterDBSize, int64(0))
		assert.GreaterOrEqual(t, info.DerivedDBSize, int64(0))
		fmt.Println("repoA info:", info)

		repoDefault, err := super.Get(constructCtx, "default")
		assert.NoError(t, err)
		assert.NotNil(t, repoDefault)
		info, err = repoDefault.Info(constructCtx, sst.DefaultBranch)
		assert.NoError(t, err)
		assert.NotNil(t, info)
		assert.NotEqual(t, "", info.BleveName)
		assert.NotEqual(t, "", info.BleveVersion)
		assert.True(t, info.IsRemote)
		assert.Equal(t, info.NumberOfDatasets, int64(0))
		assert.Equal(t, info.NumberOfCommits, int64(0))
		fmt.Println("repoDefault info:", info)

		openedIndex := repoA.Bleve()
		query := bleve.NewMatchAllQuery()
		searchRequest := bleve.NewSearchRequest(query)
		searchResults, err := openedIndex.SearchInContext(constructCtx, searchRequest)
		assert.NoError(t, err)
		assert.NotNil(t, searchResults)
		assert.Equal(t, 1, int(searchResults.Total))
		assert.Equal(t, 1, len(searchResults.Hits))
		fmt.Println("repoA search results:", searchResults)

		// Test repoDefault search - should return 0 results as it's empty
		defaultIndex := repoDefault.Bleve()
		defaultQuery := bleve.NewMatchAllQuery()
		defaultSearchRequest := bleve.NewSearchRequest(defaultQuery)
		defaultSearchResults, err := defaultIndex.SearchInContext(constructCtx, defaultSearchRequest)
		assert.NoError(t, err)
		assert.NotNil(t, defaultSearchResults)
		assert.Equal(t, 0, int(defaultSearchResults.Total))
		assert.Equal(t, 0, len(defaultSearchResults.Hits))
		fmt.Println("repoDefault search results:", defaultSearchResults)

		_, err = super.Create(constructCtx, "repoB")
		assert.NoError(t, err)

		names, err := super.List(constructCtx)
		assert.NoError(t, err)
		assert.Equal(t, 3, len(names)) // default, repoA, repoB
		assert.Contains(t, names, "default")
		assert.Contains(t, names, "repoA")
		assert.Contains(t, names, "repoB")
		fmt.Println("repos:", names)

		err = super.Delete(constructCtx, "repoB")
		assert.NoError(t, err)

		names, err = super.List(constructCtx)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(names)) // default, repoA
		assert.Contains(t, names, "default")
		assert.Contains(t, names, "repoA")
		assert.NotContains(t, names, "repoB")
		fmt.Println("repos:", names)
	})

	t.Run("read", func(t *testing.T) {
		t.Skip("skip remote read test for now")
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		assert.NoError(t, err)
		assert.NotNil(t, repo)
		defer repo.Close()

		ds, err := repo.Dataset(constructCtx, sst.IRI(ngIDC.URN()))
		assert.NoError(t, err)
		assert.NotNil(t, ds)

		s, err := ds.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		assert.NoError(t, err)
		assert.NotNil(t, s)

		ng := s.NamedGraph(sst.IRI(ngIDC.URN()))
		assert.NotNil(t, ng, "got nil NamedGraph")

		var q query.Query
		searchRequestSize := 1024
		filterQuery := ""
		if filterQuery != "" {
			q = bleve.NewQueryStringQuery(filterQuery)
		} else {
			q = bleve.NewMatchAllQuery()
		}
		req := bleve.NewSearchRequestOptions(q, searchRequestSize, 0, false)
		req.Fields = []string{"graphID", "graphURI", "mainType", "nodeCount", "directImport", "partCategory"}
		sr, err := repo.Bleve().SearchInContext(constructCtx, req)
		assert.NoError(t, err)
		assert.NotNil(t, sr)
		assert.Equal(t, 1, sr.Hits.Len())
		assert.Equal(t, 1, int(sr.Total))
		// fmt.Printf("%+v", sr)
	})
}

func Test_SuperRemoteRepository_Branch(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngIDC := uuid.MustParse("123e4567-e89b-12d3-a456-42661417400c")
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)
	var commitHash2 sst.Hash
	var influenceDatasets2 []uuid.UUID

	t.Run("write", func(t *testing.T) {
		// sst.AtomicLevel.SetLevel(zap.DebugLevel)
		url := testutil.SuperServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		super, err := sst.OpenRemoteSuperRepository(constructCtx, url, transportCreds)
		if err != nil {
			log.Fatal(err)
		}

		repoA, err := super.Create(constructCtx, "repoA")
		assert.NoError(t, err)

		st := repoA.OpenStage(sst.DefaultTriplexMode)

		ng := st.CreateNamedGraph(sst.IRI(ngIDC.URN()))

		ib := ng.CreateIRINode("John", lci.Person)
		assert.NotNil(t, ib)

		commitHash, influenceDatasets, err := st.Commit(constructCtx, "Initial commit", sst.DefaultBranch)
		assert.NoError(t, err)
		fmt.Println("commitHash:", commitHash)
		fmt.Println("influenceDatasets:", influenceDatasets)

		ib = ng.CreateIRINode("Linda", lci.Person)
		assert.NotNil(t, ib)

		commitHash2, influenceDatasets2, err = st.Commit(constructCtx, "Second commit", sst.DefaultBranch)
		assert.NoError(t, err)
		fmt.Println("commitHash2:", commitHash2)
		fmt.Println("influenceDatasets2:", influenceDatasets2)

		ds, err := repoA.Dataset(constructCtx, sst.IRI(ngIDC.URN()))
		if err != nil {
			panic(err)
		}

		err = ds.SetBranchCommit(constructCtx, commitHash2, "testBranch")
		if err != nil {
			panic(err)
		}

		branchCommitHashMap, err := ds.Branches(constructCtx)
		if err != nil {
			panic(err)
		}
		fmt.Println(branchCommitHashMap)

		assert.Equal(t, 2, len(branchCommitHashMap))

		err = ds.RemoveBranch(constructCtx, "testBranch")
		if err != nil {
			panic(err)
		}

		branchCommitHashMap, err = ds.Branches(constructCtx)
		if err != nil {
			panic(err)
		}
		fmt.Println(branchCommitHashMap)
		assert.Equal(t, 1, len(branchCommitHashMap))

		err = ds.RemoveBranch(constructCtx, sst.DefaultBranch)
		if err != nil {
			panic(err)
		}

		lc, err := ds.LeafCommits(constructCtx)
		assert.Nil(t, err)
		fmt.Println(lc)
		assert.Equal(t, 1, len(lc))
	})

	t.Run("SetBranch", func(t *testing.T) {
		url := testutil.SuperServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		super, err := sst.OpenRemoteSuperRepository(constructCtx, url, transportCreds)
		if err != nil {
			log.Fatal(err)
		}

		repoA, err := super.Get(constructCtx, "repoA")
		assert.NoError(t, err)

		ds, err := repoA.Dataset(constructCtx, sst.IRI(ngIDC.URN()))
		if err != nil {
			panic(err)
		}

		lc, err := ds.LeafCommits(constructCtx)
		assert.Nil(t, err)
		fmt.Println(lc)
		assert.Equal(t, 1, len(lc))
	})

	t.Run("bleveInfo", func(t *testing.T) {
		url := testutil.SuperServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		super, err := sst.OpenRemoteSuperRepository(constructCtx, url, transportCreds)
		if err != nil {
			log.Fatal(err)
		}

		repoA, err := super.Get(constructCtx, "repoA")
		assert.NoError(t, err)

		bi, err := repoA.Info(constructCtx, sst.DefaultBranch)
		if err != nil {
			panic(err)
		}
		fmt.Println(bi)
		assert.NotEqual(t, "", bi.BleveName)
		assert.NotEqual(t, "", bi.BleveVersion)
		// fmt.Println(bi)
	})

	t.Run("DatasetIRIs", func(t *testing.T) {
		url := testutil.SuperServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		super, err := sst.OpenRemoteSuperRepository(constructCtx, url, transportCreds)
		if err != nil {
			log.Fatal(err)
		}

		repoA, err := super.Get(constructCtx, "repoA")
		assert.NoError(t, err)

		datasetIRIs, err := repoA.Datasets(constructCtx)
		fmt.Println(datasetIRIs)
		if err != nil {
			panic(err)
		}

		// fmt.Println(bi)
		assert.Equal(t, sst.IRI(ngIDC.URN()), datasetIRIs[0])
	})

	t.Run("CheckoutCommit", func(t *testing.T) {
		url := testutil.SuperServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		super, err := sst.OpenRemoteSuperRepository(constructCtx, url, transportCreds)
		if err != nil {
			log.Fatal(err)
		}

		repoA, err := super.Get(constructCtx, "repoA")
		assert.NoError(t, err)

		dsIRIs, err := repoA.Datasets(constructCtx)
		if err != nil {
			panic(err)
		}

		ds, err := repoA.Dataset(constructCtx, dsIRIs[0])
		if err != nil {
			panic(err)
		}
		fmt.Println(dsIRIs)

		st, err := ds.CheckoutCommit(constructCtx, commitHash2, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}
		ng := st.NamedGraph(sst.IRI(ngIDC.URN()))
		if ng == nil {
			panic("got nil NamedGraph")
		}

		// ng.Dump()

		assert.Equal(t, 3, ng.IRINodeCount())
	})

	t.Run("CheckoutBranch", func(t *testing.T) {
		url := testutil.SuperServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		super, err := sst.OpenRemoteSuperRepository(constructCtx, url, transportCreds)
		if err != nil {
			log.Fatal(err)
		}

		repoA, err := super.Get(constructCtx, "repoA")
		assert.NoError(t, err)

		dsIRIs, err := repoA.Datasets(constructCtx)
		if err != nil {
			panic(err)
		}

		ds, err := repoA.Dataset(constructCtx, dsIRIs[0])
		if err != nil {
			panic(err)
		}
		fmt.Println(dsIRIs)

		err = ds.SetBranchCommit(constructCtx, commitHash2, "testBranch")
		if err != nil {
			panic(err)
		}

		st, err := ds.CheckoutBranch(constructCtx, "testBranch", sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}
		ng := st.NamedGraph(sst.IRI(ngIDC.URN()))
		if ng == nil {
			panic("got nil NamedGraph")
		}
		assert.Equal(t, 3, ng.IRINodeCount())
	})
}
