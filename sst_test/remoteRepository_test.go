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

	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/sst_test/testutil"
	"github.com/semanticstep/sst-core/sstauth"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/semanticstep/sst-core/vocabularies/sso"
	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// go run cmd/repository/main.go --verbose --dir=./sst_test_realvoc/testdata/Test_RemoteRepository_CheckoutRevision_BasicServer/ --issuer test://issuer

func Test_StartServerManually(t *testing.T) {
	t.Skip("start server for below test case")
	t.Run("Step1", func(t *testing.T) {
		// args := []string{
		// 	"--verbose",
		// 	"--dir=../sst_test_realvoc/testdata/Test_RemoteRepository_CheckoutRevision_BasicServer/",
		// 	"--issuer",
		// 	"test://issuer",
		// }
		// err := sst.RepoRemoteRun(defaultderive.DeriveInfo(), args[:])
		// if err != nil {
		// 	panic(err)
		// }
	})
}

func Test_ExistRemoteRepositoryManually(t *testing.T) {
	t.Skip("need run above case firstly")
	// dir := "./testdata/Test_RemoteRepository_CheckoutRevision_BasicServer"
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)
	constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

	t.Run("Step1", func(t *testing.T) {
		repo, err := sst.OpenRemoteRepository(constructCtx, "localhost:5581", transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		openedIndex := repo.Bleve()
		query := bleve.NewMatchAllQuery()
		searchRequest := bleve.NewSearchRequestOptions(query, 5, 0, false)
		searchRequest.Fields = append(searchRequest.Fields, "mainType", "graphID", "label",
			"literal", "id", "mainNode", "additionalType", "directImport", "nodeCount", "idOwner", "comment")

		searchResults, err := openedIndex.SearchInContext(constructCtx, searchRequest)
		if err != nil {
			log.Fatal(err)
		}

		assert.Equal(t, len(searchResults.Hits), 5)

		assert.NoError(t, err)
	})

	t.Run("Step2", func(t *testing.T) {
		repo, err := sst.OpenRemoteRepository(constructCtx, "localhost:5581", transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		formattedStr := ".*"
		fc := filterConditions{
			MainType: []string{"Organization"},
			Start:    0,
			End:      15,
			State:    2,
		}
		mixQuery := bleve.NewBooleanQuery()
		if !isEmpty(fc.RE) {
			formattedStr = ".*" + fc.RE + ".*"
		}

		q := bleve.NewRegexpQuery(formattedStr)
		q.FieldVal = "literal"
		mixQuery.AddMust(q)
		createSearchRequest(mixQuery, fc)

		search := bleve.NewSearchRequestOptions(mixQuery, fc.End-fc.Start, fc.Start, false)
		search.Fields = append(search.Fields, "mainType", "graphID", "partCategory",
			"literal", "id", "mainNode", "additionalType", "directImport", "nodeCount", "id.idOwner.label", "comment")
		if len(fc.SortModel) > 0 {
			field := fc.SortModel[0].ColId
			direct := fc.SortModel[0].Sort
			if direct == "desc" {
				field = "-" + field
			}
			search.SortBy([]string{field})
		}
		searchResults, err := repo.Bleve().SearchInContext(constructCtx, search)
		fmt.Printf("*****     %st,%+v     *****", "searchResults", searchResults)
		assert.NoError(t, err)
	})

	t.Run("Step3", func(t *testing.T) {
		repo, err := sst.OpenRemoteRepository(constructCtx, "localhost:5581", transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		repoInfo, err := repo.Info(constructCtx, "")
		assert.NoError(t, err)
		assert.Equal(t, "defaultderive", repoInfo.BleveName)

		ngID := uuid.MustParse("8c159d72-253e-4a02-8d9d-e5042f76607f")
		dataset, err := repo.Dataset(constructCtx, sst.IRI(ngID.URN()))
		if err != nil {
			panic(err)
		}

		st, err := dataset.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ngID1 := uuid.MustParse("fbe2b5ad-2cc1-4549-a4d4-eb16972ce619")
		dataset1, err := repo.Dataset(constructCtx, sst.IRI(ngID1.URN()))
		if err != nil {
			panic(err)
		}

		s1, err := dataset1.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ngID2 := uuid.MustParse("7da86e18-732d-4eb9-b0b7-22358e82bf1d")
		dataset2, err := repo.Dataset(constructCtx, sst.IRI(ngID2.URN()))
		if err != nil {
			panic(err)
		}

		s2, err := dataset2.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ngID3 := uuid.MustParse("3c7d3691-c082-41e4-bd6a-2a7b36a50374")
		dataset3, err := repo.Dataset(constructCtx, sst.IRI(ngID3.URN()))
		if err != nil {
			panic(err)
		}
		s3, err := dataset3.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ngID4 := uuid.MustParse("154a8f8e-68e4-4b8a-a9ac-84cf6d4da7a2")
		dataset4, err := repo.Dataset(constructCtx, sst.IRI(ngID4.URN()))
		if err != nil {
			panic(err)
		}
		s4, err := dataset4.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ngID5 := uuid.MustParse("3153c042-8483-4660-9c18-e3fa23608477")
		dataset5, err := repo.Dataset(constructCtx, sst.IRI(ngID5.URN()))
		if err != nil {
			panic(err)
		}
		s5, err := dataset5.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		_, err = st.MoveAndMerge(context.TODO(), s1)
		if err != nil {
			panic(err)
		}

		_, err = st.MoveAndMerge(context.TODO(), s2)
		if err != nil {
			panic(err)
		}

		_, err = st.MoveAndMerge(context.TODO(), s3)
		if err != nil {
			panic(err)
		}
		_, err = st.MoveAndMerge(context.TODO(), s4)
		if err != nil {
			panic(err)
		}
		_, err = st.MoveAndMerge(context.TODO(), s5)
		if err != nil {
			panic(err)
		}

		ng := st.NamedGraph(sst.IRI(ngID.URN()))
		ibNode := ng.CreateIRINode("", sso.BreakdownOccurrence)
		ibNode.AddStatement(sso.ID, sst.String("New added"))

		commithash, influencedDatasetIDs, err := st.Commit(constructCtx, "first commit: add main code", sst.DefaultBranch)
		if err != nil {
			panic(err)
		}
		fmt.Println(commithash)
		fmt.Println(influencedDatasetIDs)

		ds, err := repo.Dataset(constructCtx, sst.IRI(ngID.URN()))
		if err != nil {
			panic(err)
		}

		st2, err := ds.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		fmt.Println()
		for _, ng := range st2.NamedGraphs() {
			// fmt.Println(ng.ID())
			ng.Dump()
		}
	})
}

func Test_remoteRepository_MoveAndMergeBetweenTwoRepos(t *testing.T) {
	dir1 := filepath.Join(t.TempDir(), t.Name()+"1")
	dir2 := filepath.Join(t.TempDir(), t.Name()+"2")
	ngCIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363").URN())

	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	var commitHash sst.Hash
	var modifiedDSIDs []uuid.UUID

	t.Run("write_dir_1", func(t *testing.T) {

		url := testutil.ServerServe(t, dir1)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		stageC := repo.OpenStage(sst.DefaultTriplexMode)

		ng := stageC.CreateNamedGraph(ngCIRI)

		assert.Equal(t, sst.IRI("urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f613a363"), ng.IRI())

		mainC := ng.CreateIRINode("mainC")
		assert.True(t, mainC.IsIRINode())
		assert.False(t, mainC.IsBlankNode())
		assert.False(t, mainC.IsTermCollection())

		mainC.AddStatement(rdf.Type, rep.SchematicPort)
		mainC.AddStatement(rdf.Bag, rep.Angle)

		commitHash, modifiedDSIDs, err = stageC.Commit(constructCtx, "First commit of C", sst.DefaultBranch)
		assert.NoError(t, err)
		assert.Equal(t, len(modifiedDSIDs), 1)
		assert.Equal(t, sst.IRI(modifiedDSIDs[0].URN()), ngCIRI)
	})

	t.Run("read_dir_1", func(t *testing.T) {
		url := testutil.ServerServe(t, dir1)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(constructCtx, ngCIRI)
		if err != nil {
			panic(err)
		}

		st, err := ds.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		url2 := testutil.ServerServe(t, dir2)
		// constructCtx := auth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo2, err := sst.OpenRemoteRepository(constructCtx, url2, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo2.Close()

		st2 := repo2.OpenStage(sst.DefaultTriplexMode)
		st2.MoveAndMerge(constructCtx, st)

		commitHash, modifiedDSIDs, err = st2.Commit(constructCtx, "First commit of C", sst.DefaultBranch)
		assert.NoError(t, err)
		assert.Equal(t, len(modifiedDSIDs), 1)
		assert.Equal(t, sst.IRI(modifiedDSIDs[0].URN()), ngCIRI)
		_ = commitHash
	})

	t.Run("read_dir_2", func(t *testing.T) {
		url2 := testutil.ServerServe(t, dir2)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo2, err := sst.OpenRemoteRepository(constructCtx, url2, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo2.Close()

		ds, err := repo2.Dataset(constructCtx, ngCIRI)
		if err != nil {
			panic(err)
		}

		st2, err := ds.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		assert.NoError(t, err)

		st2.Dump()

		assert.Equal(t, len(st2.NamedGraphs()), 1)
		assert.Equal(t, st2.NamedGraphs()[0].IRI(), ngCIRI)
	})
}

func Test_RemoteRepository_BleveCount(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)
	query := bleve.NewMatchAllQuery()
	searchRequest := bleve.NewSearchRequest(query)
	var randomGraphID uuid.UUID

	t.Run("write", func(t *testing.T) {

		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()
		openedIndex := repo.Bleve()

		searchResults1, err := openedIndex.SearchInContext(constructCtx, searchRequest)
		if err != nil {
			panic(err)
		}

		st := repo.OpenStage(sst.DefaultTriplexMode)

		randomGraphID = uuid.New()
		fmt.Println("graphID", randomGraphID)
		graph := st.CreateNamedGraph(sst.IRI(randomGraphID.URN()))

		graph.CreateIRINode("main", lci.Organization)
		_, _, err = st.Commit(constructCtx, "first commit: add main code", sst.DefaultBranch)
		assert.NoError(t, err)

		searchResults2, err := repo.Bleve().SearchInContext(constructCtx, searchRequest)
		if err != nil {
			panic(err)
		}
		fmt.Println("r1 :          ", searchResults1)

		fmt.Println()
		fmt.Println("r2 :          ", searchResults2)

		// there should be one more dataset in the repository
		assert.Equal(t, uint64(1), searchResults2.Total-searchResults1.Total)
	})

	t.Run("remove branch", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(constructCtx, sst.IRI(randomGraphID.URN()))
		assert.NoError(t, err)

		err = ds.RemoveBranch(constructCtx, sst.DefaultBranch)
		assert.NoError(t, err)
		sr2, err := repo.Bleve().SearchInContext(constructCtx, searchRequest)
		if err != nil {
			panic(err)
		}
		assert.Equal(t, uint64(0), sr2.Total)
	})
}

func Test_RemoteRepositoryWithName_UUIDNamedGraph_Bleve(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngCIRI := sst.IRI(uuid.New().URN())
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	t.Run("write", func(t *testing.T) {

		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		repoInfo, err := repo.Info(constructCtx, "")
		assert.NoError(t, err)
		assert.Equal(t, "defaultderive", repoInfo.BleveName)

		st := repo.OpenStage(sst.DefaultTriplexMode)

		graph := st.CreateNamedGraph(ngCIRI)

		main := graph.CreateIRINode("main")
		main.AddStatement(rdf.Type, lci.Organization)
		main.AddStatement(rdfs.Label, sst.String("ABC Ltd."))
		_, _, err = st.Commit(constructCtx, "First commit of C", sst.DefaultBranch)
		assert.NoError(t, err)
	})

	t.Run("read", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(constructCtx, ngCIRI)
		if err != nil {
			panic(err)
		}

		st, err := ds.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ng := st.NamedGraph(ngCIRI)
		if ng == nil {
			panic("got nil NamedGraph")
		}

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
		if err != nil {
			panic(err)
		}
		assert.Equal(t, sr.Hits.Len(), 1)
		// fmt.Printf("%+v", sr)
	})
}

func Test_RemoteRepository_UUIDNamedGraph_Bleve(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngCIRI := sst.IRI(uuid.New().URN())
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	t.Run("write", func(t *testing.T) {

		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		repoInfo, err := repo.Info(constructCtx, "")
		assert.NoError(t, err)
		assert.Equal(t, "defaultderive", repoInfo.BleveName)

		st := repo.OpenStage(sst.DefaultTriplexMode)

		graph := st.CreateNamedGraph(ngCIRI)

		main := graph.CreateIRINode("main")
		main.AddStatement(rdf.Type, lci.Organization)
		main.AddStatement(rdfs.Label, sst.String("ABC Ltd."))
		_, _, err = st.Commit(constructCtx, "First commit of C", sst.DefaultBranch)
		assert.NoError(t, err)
	})

	t.Run("read", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(constructCtx, ngCIRI)
		if err != nil {
			panic(err)
		}

		st, err := ds.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ng := st.NamedGraph(ngCIRI)
		if ng == nil {
			panic("got nil NamedGraph")
		}

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
		if err != nil {
			panic(err)
		}
		assert.Equal(t, sr.Hits.Len(), 1)
		// fmt.Printf("%+v", sr)
	})
}

func Test_RemoteRepository_CommitOnClosedRepo(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngCIRI := sst.IRI(uuid.New().URN())
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	var modifiedDSIDs []uuid.UUID
	t.Run("write", func(t *testing.T) {

		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		graph := st.CreateNamedGraph(ngCIRI)

		main := graph.CreateIRINode("main")
		main.AddStatement(rdf.Type, lci.Organization)
		main.AddStatement(rdfs.Label, sst.String("ABC Ltd."))

		err = repo.Close()
		assert.NoError(t, err)

		_, modifiedDSIDs, err = st.Commit(constructCtx, "First commit of C", sst.DefaultBranch)
		assert.Error(t, err)
		assert.Equal(t, len(modifiedDSIDs), 0)
	})

	t.Run("leafCommits", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(constructCtx, ngCIRI)
		assert.Nil(t, ds)
		assert.Error(t, err)
	})

}

func Test_RemoteRepository_UUIDNamedGraph(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngCIRI := sst.IRI(uuid.New().URN())
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	var returnedCommitHash sst.Hash
	var modifiedDSIDs []uuid.UUID
	t.Run("write", func(t *testing.T) {

		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		graph := st.CreateNamedGraph(ngCIRI)

		main := graph.CreateIRINode("main")
		main.AddStatement(rdf.Type, lci.Organization)
		main.AddStatement(rdfs.Label, sst.String("ABC Ltd."))

		returnedCommitHash, modifiedDSIDs, err = st.Commit(constructCtx, "First commit of C", sst.DefaultBranch)
		if err != nil {
			panic(err)
		}
		fmt.Println(returnedCommitHash)
		assert.Equal(t, len(modifiedDSIDs), 1)
		assert.Equal(t, sst.IRI(modifiedDSIDs[0].URN()), ngCIRI)
	})

	t.Run("leafCommits", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(constructCtx, ngCIRI)
		if err != nil {
			panic(err)
		}
		commitHashes, err := ds.LeafCommits(constructCtx)
		assert.Nil(t, err)
		assert.Equal(t, len(commitHashes), 0)

		branchMap, err := ds.Branches(constructCtx)
		assert.NoError(t, err)
		assert.Equal(t, len(branchMap), 1)
		assert.Equal(t, branchMap["master"], returnedCommitHash)
	})

	t.Run("CommitDetailsByHash", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(constructCtx, ngCIRI)
		if err != nil {
			panic(err)
		}
		cd, _ := ds.CommitDetailsByHash(constructCtx, returnedCommitHash)
		assert.Equal(t, "test1@semanticstep.net", cd.Author)
		assert.Equal(t, "First commit of C", cd.Message)
		assert.NotNil(t, cd.DatasetRevisions[ngCIRI])
	})

	t.Run("CommitDetailsByBranch", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(constructCtx, ngCIRI)
		if err != nil {
			panic(err)
		}
		cd, _ := ds.CommitDetailsByBranch(constructCtx, sst.DefaultBranch)
		assert.Equal(t, "First commit of C", cd.Message)
		assert.NotNil(t, cd.DatasetRevisions[ngCIRI])
	})

	t.Run("CommitForRevision", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(constructCtx, ngCIRI)
		if err != nil {
			panic(err)
		}

		cd, err := ds.CommitDetailsByHash(constructCtx, returnedCommitHash)
		assert.NoError(t, err)
		datasetRevisionHash, ok := cd.DatasetRevisions[ngCIRI]
		assert.True(t, ok, "dataset revision should exist in commit details")

		foundCommitHash, err := ds.CommitForRevision(constructCtx, datasetRevisionHash)
		assert.NoError(t, err)
		assert.Equal(t, returnedCommitHash, foundCommitHash, "CommitForRevision should return the original commit hash")

		_, err = ds.CommitForRevision(constructCtx, sst.BytesToHash([]byte("non-existent-revision")))
		assert.ErrorIs(t, err, sst.ErrDatasetRevisionNotFound)
	})

	t.Run("SetBranchRevision", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(constructCtx, ngCIRI)
		if err != nil {
			panic(err)
		}

		cd, err := ds.CommitDetailsByHash(constructCtx, returnedCommitHash)
		assert.NoError(t, err)
		datasetRevisionHash, ok := cd.DatasetRevisions[ngCIRI]
		assert.True(t, ok, "dataset revision should exist in commit details")

		err = ds.SetBranchRevision(constructCtx, datasetRevisionHash, "revisionBranch")
		assert.NoError(t, err)

		branchCommitHashMap, err := ds.Branches(constructCtx)
		assert.NoError(t, err)
		assert.Equal(t, returnedCommitHash, branchCommitHashMap["revisionBranch"], "branch should point to the commit that created the revision")

		err = ds.SetBranchRevision(constructCtx, sst.BytesToHash([]byte("non-existent-revision")), "revisionBranch2")
		assert.ErrorIs(t, err, sst.ErrDatasetRevisionNotFound)

		// Clean up so that subsequent subtests see the expected branch state.
		err = ds.RemoveBranch(constructCtx, "revisionBranch")
		assert.NoError(t, err)
	})

	t.Run("setBranchAndBranches", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(constructCtx, ngCIRI)
		if err != nil {
			panic(err)
		}

		err = ds.SetBranchCommit(constructCtx, returnedCommitHash, "testBranch")
		if err != nil {
			panic(err)
		}

		branchCommitHashMap, err := ds.Branches(constructCtx)
		if err != nil {
			panic(err)
		}
		assert.Equal(t, 2, len(branchCommitHashMap))
	})

	t.Run("CheckoutTestBranch", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(constructCtx, ngCIRI)
		if err != nil {
			panic(err)
		}

		st, err := ds.CheckoutBranch(constructCtx, "testBranch", sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ng := st.NamedGraph(ngCIRI)
		if ng == nil {
			panic("got nil NamedGraph")
		}

		ng.Dump()
	})

	t.Run("RemoveTestBranchAndCheckoutTestBranch", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(constructCtx, ngCIRI)
		if err != nil {
			panic(err)
		}

		err = ds.RemoveBranch(constructCtx, "testBranch")
		if err != nil {
			panic(err)
		}

		branchCommitHashMap, err := ds.Branches(constructCtx)
		if err != nil {
			panic(err)
		}
		assert.Equal(t, 1, len(branchCommitHashMap))

		_, err = ds.CheckoutBranch(constructCtx, "testBranch", sst.DefaultTriplexMode)
		assert.Contains(t, err.Error(), "branch not found")
	})

	t.Run("checkoutMaster", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(constructCtx, ngCIRI)
		if err != nil {
			panic(err)
		}

		st, err := ds.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ng := st.NamedGraph(ngCIRI)
		if ng == nil {
			panic("got nil NamedGraph")
		}

		ng.Dump()
	})
}

func Test_RemoteRepository_IRINamedGraph(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)
	testIri := "http://ontology.semanticstep.net/abc#"

	t.Run("write", func(t *testing.T) {

		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		ng := st.CreateNamedGraph(sst.IRI(testIri))

		mainB := ng.CreateIRINode("mainB")

		mainB.AddStatement(rdf.Type, rep.SchematicPort)

		st.Commit(constructCtx, "First commit of B", sst.DefaultBranch)
	})

	t.Run("read", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(constructCtx, sst.IRI(testIri))
		if err != nil {
			panic(err)
		}

		st, err := ds.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ng := st.NamedGraph(sst.IRI(testIri))
		if ng == nil {
			panic("got nil NamedGraph")
		}
		mainB := ng.GetIRINodeByFragment("mainB")
		assert.NotNil(t, mainB)
		fmt.Println(mainB.IRI())

		ng.Dump()
	})
}

func Test_RemoteRepository_NGBImportNGC(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngBIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362").URN())
	ngCIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363").URN())
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)
	var returnedCommitHash sst.Hash

	t.Run("write", func(t *testing.T) {

		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance2)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		ngB := st.CreateNamedGraph(ngBIRI)

		ngC := st.CreateNamedGraph(ngCIRI)
		ngB.AddImport(ngC)

		mainC := ngC.CreateIRINode("mainC")
		mainC.AddStatement(rdf.Type, rep.SchematicPort)

		mainB := ngB.CreateIRINode("mainB")
		mainB.AddStatement(rep.MappingSource, mainC)

		returnedCommitHash, _, err = st.Commit(constructCtx, "Commit NGA imports NGB", sst.DefaultBranch)
		assert.NoError(t, err)
	})
	t.Run("read", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(constructCtx, ngBIRI)
		if err != nil {
			panic(err)
		}

		st, err := ds.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ngB := st.NamedGraph(ngBIRI)
		if ngB == nil {
			panic("got nil NamedGraph")
		}

		ngB.Dump()

		ngC := st.NamedGraph(ngCIRI)
		if ngC == nil {
			panic("got nil NamedGraph")
		}

		ngC.Dump()
		fmt.Println()
		traversalByAPI(ngB)
	})
	t.Run("CommitDetailsByHash", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(constructCtx, ngBIRI)
		if err != nil {
			panic(err)
		}
		cd, _ := ds.CommitDetailsByHash(constructCtx, returnedCommitHash)
		assert.Equal(t, "test2@semanticstep.net", cd.Author)
		assert.Equal(t, "Commit NGA imports NGB", cd.Message)
		assert.NotNil(t, cd.DatasetRevisions[ngBIRI])
	})
}

func Test_RemoteRepository_NGAImportsNGBImportsNGC(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngAIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361").URN())
	ngBIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362").URN())
	ngCIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363").URN())
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	t.Run("write", func(t *testing.T) {

		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		ngA := st.CreateNamedGraph(ngAIRI)

		ngB := st.CreateNamedGraph(ngBIRI)

		ngC := st.CreateNamedGraph(ngCIRI)

		// Create mainC node
		mainC := ngC.CreateIRINode("mainC")
		mainC.AddStatement(rdf.Type, rep.SchematicPort)

		// NG-A imports NG-B
		// Create mainA node references mainB
		ngB.AddImport(ngC)
		mainB := ngB.CreateIRINode("mainB")
		mainB.AddStatement(rep.MappingSource, mainC)

		// NG-D imports NG-A
		// creates mainD node references mainA
		ngA.AddImport(ngB)
		mainA := ngA.CreateIRINode("mainA")
		mainA.AddStatement(rep.MappingSource, mainB)

		st.Commit(constructCtx, "D imports A imports B", sst.DefaultBranch)
	})
	t.Run("read", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		dsD, err := repo.Dataset(constructCtx, ngAIRI)
		if err != nil {
			panic(err)
		}

		st, err := dsD.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ngA := st.NamedGraph(ngAIRI)
		if ngA == nil {
			panic("got nil NamedGraph")
		}

		ngA.Dump()

		ngB := st.NamedGraph(ngBIRI)
		if ngB == nil {
			panic("got nil NamedGraph")
		}

		ngB.Dump()

		ngC := st.NamedGraph(ngCIRI)
		if ngC == nil {
			panic("got nil NamedGraph")
		}

		ngC.Dump()

		traversalByAPI(ngA)
	})
}

func Test_RemoteRepository_DiamondCase(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngAIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361").URN())
	ngBIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362").URN())
	ngCIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363").URN())
	ngIDD := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a364")
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	t.Run("write", func(t *testing.T) {

		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		ngA := st.CreateNamedGraph(ngAIRI)
		ngB := st.CreateNamedGraph(ngBIRI)
		ngC := st.CreateNamedGraph(ngCIRI)
		ngD := st.CreateNamedGraph(sst.IRI(ngIDD.URN()))

		// Create mainC node
		mainC := ngC.CreateIRINode("mainC")
		mainC.AddStatement(rdf.Type, rep.SchematicPort)

		// NG-B imports NG-C
		// Create mainB node references mainC
		ngB.AddImport(ngC)
		mainB := ngB.CreateIRINode("mainB")
		mainB.AddStatement(rep.MappingSource, mainC)

		// NG-D imports NG-C
		// Create mainD node references mainC
		ngD.AddImport(ngC)

		mainD := ngD.CreateIRINode("mainD")
		mainD.AddStatement(rep.MappingSource, mainC)

		// NG-A imports NG-B
		// creates mainA node references mainB
		ngA.AddImport(ngB)
		mainA := ngA.CreateIRINode("mainA")
		mainA.AddStatement(rep.MappingSource, mainB)

		// NG-A imports NG-D
		// creates mainA node references mainD
		ngA.AddImport(ngD)

		mainA.AddStatement(rep.MappingSource, mainD)

		st.Commit(constructCtx, "Diamond Case", sst.DefaultBranch)
	})
	t.Run("read", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		dsA, err := repo.Dataset(constructCtx, ngAIRI)
		if err != nil {
			panic(err)
		}

		st, err := dsA.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ngA := st.NamedGraph(ngAIRI)
		if ngA == nil {
			panic("got nil NamedGraph")
		}

		ngA.Dump()

		ngB := st.NamedGraph(ngBIRI)
		if ngB == nil {
			panic("got nil NamedGraph")
		}

		ngB.Dump()

		ngC := st.NamedGraph(ngCIRI)
		if ngC == nil {
			panic("got nil NamedGraph")
		}

		ngC.Dump()

		traversalByAPI(ngA)
	})
}

func Test_RemoteRepository_DiamondCaseContext(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngAIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361").URN())
	ngBIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362").URN())
	ngCIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363").URN())
	ngIDD := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a364")
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	t.Run("write", func(t *testing.T) {

		// start test gRPC server
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		ngA := st.CreateNamedGraph(ngAIRI)
		ngB := st.CreateNamedGraph(ngBIRI)
		ngC := st.CreateNamedGraph(ngCIRI)
		ngD := st.CreateNamedGraph(sst.IRI(ngIDD.URN()))

		// Create mainC node
		mainC := ngC.CreateIRINode("mainC")
		mainC.AddStatement(rdf.Type, rep.SchematicPort)

		// NG-B imports NG-C
		// Create mainB node references mainC
		ngB.AddImport(ngC)
		mainB := ngB.CreateIRINode("mainB")
		mainB.AddStatement(rep.MappingSource, mainC)

		// NG-D imports NG-C
		// Create mainD node references mainC
		ngD.AddImport(ngC)

		mainD := ngD.CreateIRINode("mainD")
		mainD.AddStatement(rep.MappingSource, mainC)

		// NG-A imports NG-B
		// creates mainA node references mainB
		ngA.AddImport(ngB)
		mainA := ngA.CreateIRINode("mainA")
		mainA.AddStatement(rep.MappingSource, mainB)

		// NG-A imports NG-D
		// creates mainA node references mainD
		ngA.AddImport(ngD)

		mainA.AddStatement(rep.MappingSource, mainD)

		st.Commit(constructCtx, "Diamond Case", sst.DefaultBranch)
	})
	t.Run("read", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		dsA, err := repo.Dataset(constructCtx, ngAIRI)
		if err != nil {
			panic(err)
		}

		st, err := dsA.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ngA := st.NamedGraph(ngAIRI)
		if ngA == nil {
			panic("got nil NamedGraph")
		}

		ngA.Dump()

		ngB := st.NamedGraph(ngBIRI)
		if ngB == nil {
			panic("got nil NamedGraph")
		}

		ngB.Dump()

		ngC := st.NamedGraph(ngCIRI)
		if ngC == nil {
			panic("got nil NamedGraph")
		}

		ngC.Dump()

		traversalByAPI(ngA)
	})
}
