// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

// The reason why there is a separate package is about RegisterVocabularyMap.
// In sst_test_dummyvoc package, it imports package compiler, which will register a test VocabularyMap.
// In sst_test_realvoc package, it imports package vocabularies, which will register a real
// VocabularyMap that includes lci, owl...

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/semanticstep/sst-core/defaultderive"
	"github.com/semanticstep/sst-core/sst"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/owl"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/semanticstep/sst-core/vocabularies/sso"
	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func Test_NPE(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	//dir := filepath.Join(testName)
	repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
	if err != nil {
		panic(err)
	}
	defer repo.Close()

	repo.RegisterIndexHandler(defaultderive.DeriveInfo())
}

func Test_Time(t *testing.T) {
	fmt.Printf("%st %+v", "time : ", time.Now().UTC().Format(time.RFC3339))
}

func Test_Commit(t *testing.T) {
	t.Skip("no repo")
	dir := filepath.Join(t.TempDir(), t.Name())
	//dir := filepath.Join(testName)
	//defer os.RemoveAll(dir)

	repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
	if err != nil {
		panic(err)
	}
	defer repo.Close()

	repo.RegisterIndexHandler(defaultderive.DeriveInfo())

	openedIndex := repo.Bleve()
	query := bleve.NewMatchAllQuery()
	searchRequest := bleve.NewSearchRequest(query)
	searchRequest.Fields = append(searchRequest.Fields, "mainType", "graphID", "label",
		"literal", "id", "mainNode", "additionalType", "directImport", "nodeCount", "idOwner",
		"comment", "commitHash", "commitAuthor", "commitTime")
	//searchRequest.Size = math.MaxInt

	searchResults, err := openedIndex.SearchInContext(context.TODO(), searchRequest)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%st,%+v", "searchResults", searchResults)
}

func Test_Unique(t *testing.T) {
	t.Skip("no repo")

	dir := filepath.Join(t.TempDir(), t.Name())

	//defer os.RemoveAll(dir)

	/*
		Initialize will success for ture
		Initialize will insert an item 'urn:uuid:97f261fd-1a26-4e78-b4f5-5baadd1e193d#' into bbolt
		And this can be searched in bleve
	*/
	t.Run("initialize", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		repo.RegisterIndexHandler(defaultderive.DeriveInfo())

		st := repo.OpenStage(sst.DefaultTriplexMode)

		tempNg := readTTLFile("./testdata/id1.ttl")
		_, err = st.MoveAndMerge(context.TODO(), tempNg.Stage())
		if err != nil {
			panic(err)
		}

		st.Commit(context.TODO(), "initialize with urn:uuid:97f261fd-1a26-4e78-b4f5-5baadd1e193d#", sst.DefaultBranch)

		openedIndex := repo.Bleve()
		query := bleve.NewMatchAllQuery()
		searchRequest := bleve.NewSearchRequest(query)
		searchRequest.Fields = append(searchRequest.Fields, "mainType", "graphID", "label",
			"literal", "id", "mainNode", "additionalType", "directImport", "nodeCount", "idOwner",
			"comment", "commitHash", "commitAuthor", "commitTime")

		searchResults, err := openedIndex.SearchInContext(context.TODO(), searchRequest)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%st,%+v", "searchResults", searchResults)
	})

	/*
		First commit will success
		This commit will insert an item 'urn:uuid:87f261fd-1a26-4e78-b4f5-5baadd1e193d#' into bbolt
		And this can be searched in bleve
	*/
	t.Run("commit_success", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		repo.RegisterIndexHandler(defaultderive.DeriveInfo())

		st := repo.OpenStage(sst.DefaultTriplexMode)

		tempNg := readTTLFile("./testdata/id2.ttl")
		_, err = st.MoveAndMerge(context.TODO(), tempNg.Stage())
		if err != nil {
			panic(err)
		}

		_, _, err = st.Commit(context.TODO(), "commit with urn:uuid:87f261fd-1a26-4e78-b4f5-5baadd1e193d#", sst.DefaultBranch)
		assert.NoError(t, err)

		openedIndex := repo.Bleve()
		query := bleve.NewMatchAllQuery()
		searchRequest := bleve.NewSearchRequest(query)
		searchRequest.Fields = append(searchRequest.Fields, "mainType", "graphID", "label",
			"literal", "id", "mainNode", "additionalType", "directImport", "nodeCount", "idOwner",
			"comment", "commitHash", "commitAuthor", "commitTime")

		searchResults, err := openedIndex.SearchInContext(context.TODO(), searchRequest)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%st,%+v", "searchResults", searchResults)
	})

	/*
		Second commit will fail
		Because this commit try to insert an item 'urn:uuid:77f261fd-1a26-4e78-b4f5-5baadd1e193d#' into bbolt
		But the id of this new item is '1BITDSOURCE' which is same as the inserted item when initialize
		This violate the unique id rule and will be checked before insert into bbolt
		So the result of this operation is
		First, Return the error message :
		'unique constraint violated: cannot write: 77f261fd-1a26-4e78-b4f5-5baadd1e193d conflicts:[97f261fd-1a26-4e78-b4f5-5baadd1e193d]'
		Second, the new item will not be inserted into both bbolt and bleve
	*/
	t.Run("commit_fail", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		repo.RegisterIndexHandler(defaultderive.DeriveInfo())

		st := repo.OpenStage(sst.DefaultTriplexMode)

		tempNg := readTTLFile("./testdata/id3.ttl")
		_, err = st.MoveAndMerge(context.TODO(), tempNg.Stage())
		if err != nil {
			panic(err)
		}

		_, _, err = st.Commit(context.TODO(), "commit with urn:uuid:77f261fd-1a26-4e78-b4f5-5baadd1e193d#", sst.DefaultBranch)
		assert.Error(t, err)
		fmt.Println(err)

		openedIndex := repo.Bleve()
		query := bleve.NewMatchAllQuery()
		searchRequest := bleve.NewSearchRequest(query)
		searchRequest.Fields = append(searchRequest.Fields, "mainType", "graphID", "label",
			"literal", "id", "mainNode", "additionalType", "directImport", "nodeCount", "idOwner",
			"comment", "commitHash", "commitAuthor", "commitTime")

		searchResults, err := openedIndex.SearchInContext(context.TODO(), searchRequest)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%st,%+v", "searchResults", searchResults)
	})
}

func isEmpty(str string) bool {
	return len(strings.TrimSpace(str)) == 0
}

type filterConditions struct {
	State             int
	MainType          []string
	Library           []string
	PackageType       []string
	Manufacturer      []string
	Technology        []string
	EDWin_Type        []string
	RE                string
	SortModel         []SortModel
	ClassSystemTypeID string
	Start             int
	End               int
}

type SortModel struct {
	Sort  string
	ColId string
}

func createSearchRequest(mixQuery *query.BooleanQuery, fc filterConditions) {
	if len(fc.MainType) > 0 {
		q := bleve.NewMatchQuery(strings.Join(fc.MainType, ","))
		q.FieldVal = "mainType"
		mixQuery.AddMust(q)
	}

	var termList = []string{}
	if len(fc.Library) > 0 {
		termList = append(termList, fc.Library...)
	}
	if len(fc.PackageType) > 0 {
		termList = append(termList, fc.PackageType...)
	}
	if len(fc.Technology) > 0 {
		termList = append(termList, fc.Technology...)
	}
	if len(fc.EDWin_Type) > 0 {
		termList = append(termList, fc.EDWin_Type...)
	}
	if len(termList) > 0 {
		for _, val := range termList {
			q := bleve.NewMatchQuery(val)
			q.FieldVal = "additionalType"
			mixQuery.AddMust(q)
		}
	}
	if len(fc.Manufacturer) > 0 {
		q := bleve.NewMatchQuery(strings.Join(fc.Manufacturer, ","))
		q.FieldVal = "id.idOwner.label"
		mixQuery.AddMust(q)
	}
}

func Test_ReadTTLsIntoLocalFullRepositoryAndCheckout(t *testing.T) {
	dir := filepath.Join("./testdata/" + "TestReadTTLsWriteToSSTs")
	dirRepo := filepath.Join(t.TempDir(), t.Name())
	ngMain := uuid.MustParse("fa33ec5c-2ce1-47a4-9676-ffec196b046b")

	t.Run("write", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dirRepo, "default@semanticstep.net", "default", true)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		repo.RegisterIndexHandler(defaultderive.DeriveInfo())

		st := repo.OpenStage(sst.DefaultTriplexMode)

		entries, err := os.ReadDir(dir)
		if err != nil {
			panic(err)
		}

		for _, entry := range entries {
			filePath := filepath.Join(dir, entry.Name())

			file, err := os.Open(filePath)
			if err != nil {
				panic(err)
			}

			tempSt, err := sst.RdfRead(bufio.NewReader(file), sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
			if err != nil {
				panic(err)
			}

			_, err = st.MoveAndMerge(context.TODO(), tempSt)
			if err != nil {
				panic(err)
			}
		}

		commitHash, influenceID, err := st.Commit(context.TODO(), "load all ttl files", sst.DefaultBranch)
		fmt.Println(commitHash)
		fmt.Println(influenceID)
		if err != nil {
			panic(err)
		}

	})
	t.Run("read", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dirRepo, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), sst.IRI(ngMain.URN()))
		if err != nil {
			panic(err)
		}

		st, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ngMain := st.NamedGraph(sst.IRI(ngMain.URN()))
		if ngMain == nil {
			panic("got nil NamedGraph")
		}
		assert.Equal(t, len(st.NamedGraphs()), 7)
	})
}

func Test_ReadTTLsIntoLocalFullRepository(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())

	// ngAIRI := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361")
	ngBIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362").URN())
	ngCIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363").URN())
	// ngDIRI := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a364")
	t.Run("write", func(t *testing.T) {

		generatedFiles := []string{
			"./testdata/" + "Test_CreateEphemeralStage_DiamondCase" + "NGB",
			// "Test_CreateEphemeralStage_DiamondCase" + "NGA",
			"./testdata/" + "Test_CreateEphemeralStage_DiamondCase" + "NGC",
			// "Test_CreateEphemeralStage_DiamondCase" + "NGD",
		}
		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		repo.RegisterIndexHandler(defaultderive.DeriveInfo())

		repoInfo, err := repo.Info(context.TODO(), "")
		assert.NoError(t, err)
		assert.Equal(t, "defaultderive", repoInfo.BleveName)
		fmt.Println(repoInfo.URL)
		st := repo.OpenStage(sst.DefaultTriplexMode)

		for _, fileName := range generatedFiles {
			tempNg := readTTLFile(fileName + ".ttl")
			_, err = st.MoveAndMerge(context.TODO(), tempNg.Stage())
			if err != nil {
				panic(err)
			}
		}
		st.Commit(context.TODO(), "load all ttl files", sst.DefaultBranch)
	})
	t.Run("read", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngBIRI)
		if err != nil {
			panic(err)
		}

		st, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
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

		traversalByAPI(ngB)
	})
}

func Test_LocalFullRepository_MoveAndMergeBetweenTwoRepos(t *testing.T) {
	dir1 := filepath.Join(t.TempDir(), t.Name()+"1")
	dir2 := filepath.Join(t.TempDir(), t.Name()+"2")
	ngCIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363").URN())

	var commitHash sst.Hash
	var modifiedDSIDs []uuid.UUID

	t.Run("write_dir_1", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir1, "default@semanticstep.net", "default", true)
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

		blankC := ng.CreateBlankNode(lci.Person)
		assert.False(t, blankC.IsIRINode())
		assert.True(t, blankC.IsBlankNode())
		assert.False(t, blankC.IsTermCollection())

		mainC.AddStatement(rdf.Type, rep.SchematicPort)
		mainC.AddStatement(rdf.Bag, rep.Angle)

		commitHash, modifiedDSIDs, err = stageC.Commit(context.TODO(), "First commit of C", sst.DefaultBranch)
		assert.NoError(t, err)
		assert.Equal(t, len(modifiedDSIDs), 1)
		assert.Equal(t, sst.IRI(modifiedDSIDs[0].URN()), ngCIRI)
	})

	t.Run("read_dir_1", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir1, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngCIRI)
		if err != nil {
			panic(err)
		}

		st, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		repo2, err := sst.CreateLocalRepository(dir2, "default@semanticstep.net", "default", true)
		if err != nil {
			panic(err)
		}
		defer repo2.Close()

		st2 := repo2.OpenStage(sst.DefaultTriplexMode)
		report, err := st2.MoveAndMerge(context.TODO(), st)
		assert.NoError(t, err)
		fmt.Println(report)

		commitHash, modifiedDSIDs, err = st2.Commit(context.TODO(), "First commit of C", sst.DefaultBranch)
		assert.NoError(t, err)
		assert.Equal(t, len(modifiedDSIDs), 1)
		assert.Equal(t, sst.IRI(modifiedDSIDs[0].URN()), ngCIRI)
		_ = commitHash
	})

	t.Run("read_dir_2", func(t *testing.T) {
		repo2, err := sst.OpenLocalRepository(dir2, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo2.Close()

		ds, err := repo2.Dataset(context.TODO(), ngCIRI)
		if err != nil {
			panic(err)
		}

		st2, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		assert.NoError(t, err)

		st2.Dump()

		assert.Equal(t, len(st2.NamedGraphs()), 1)
		assert.Equal(t, st2.NamedGraphs()[0].IRI(), ngCIRI)
	})
}

func Test_LocalFullRepository_UUIDNamedGraph(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngCIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363").URN())

	var commitHash sst.Hash
	var modifiedDSIDs []uuid.UUID

	t.Run("write", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
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

		blankC := ng.CreateBlankNode(lci.Person)
		assert.False(t, blankC.IsIRINode())
		assert.True(t, blankC.IsBlankNode())
		assert.False(t, blankC.IsTermCollection())

		mainC.AddStatement(rdf.Type, rep.SchematicPort)
		mainC.AddStatement(rdf.Bag, rep.Angle)

		commitHash, modifiedDSIDs, _ = stageC.Commit(context.TODO(), "First commit of C", sst.DefaultBranch)

		assert.Equal(t, len(modifiedDSIDs), 1)
		assert.Equal(t, sst.IRI(modifiedDSIDs[0].URN()), ngCIRI)
	})

	t.Run("leafCommits", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngCIRI)
		if err != nil {
			panic(err)
		}
		commitHashs, err := ds.LeafCommits(context.TODO())
		assert.Nil(t, err)

		fmt.Println(commitHashs)
	})

	t.Run("CommitDetailsByHash", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngCIRI)
		if err != nil {
			panic(err)
		}
		cd, _ := ds.CommitDetailsByHash(context.TODO(), commitHash)

		cd.Dump()
	})

	t.Run("CommitDetailsByBranch", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngCIRI)
		if err != nil {
			panic(err)
		}
		cd, _ := ds.CommitDetailsByBranch(context.TODO(), sst.DefaultBranch)

		cd.Dump()
	})

	t.Run("CommitForRevision", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngCIRI)
		if err != nil {
			panic(err)
		}

		cd, err := ds.CommitDetailsByHash(context.TODO(), commitHash)
		assert.NoError(t, err)
		datasetRevisionHash, ok := cd.DatasetRevisions[ngCIRI]
		assert.True(t, ok, "dataset revision should exist in commit details")

		foundCommitHash, err := ds.CommitForRevision(context.TODO(), datasetRevisionHash)
		assert.NoError(t, err)
		assert.Equal(t, commitHash, foundCommitHash, "CommitForRevision should return the original commit hash")

		_, err = ds.CommitForRevision(context.TODO(), sst.BytesToHash([]byte("non-existent-revision")))
		assert.ErrorIs(t, err, sst.ErrDatasetRevisionNotFound)
	})

	t.Run("SetBranchRevision", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngCIRI)
		if err != nil {
			panic(err)
		}

		cd, err := ds.CommitDetailsByHash(context.TODO(), commitHash)
		assert.NoError(t, err)
		datasetRevisionHash, ok := cd.DatasetRevisions[ngCIRI]
		assert.True(t, ok, "dataset revision should exist in commit details")

		err = ds.SetBranchRevision(context.TODO(), datasetRevisionHash, "revisionBranch")
		assert.NoError(t, err)

		branchCommitHashMap, err := ds.Branches(context.TODO())
		assert.NoError(t, err)
		assert.Equal(t, commitHash, branchCommitHashMap["revisionBranch"], "branch should point to the commit that created the revision")

		err = ds.SetBranchRevision(context.TODO(), sst.BytesToHash([]byte("non-existent-revision")), "revisionBranch2")
		assert.ErrorIs(t, err, sst.ErrDatasetRevisionNotFound)

		// Clean up so that subsequent subtests see the expected branch state.
		err = ds.RemoveBranch(context.TODO(), "revisionBranch")
		assert.NoError(t, err)
	})

	t.Run("setBranch", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngCIRI)
		if err != nil {
			panic(err)
		}

		err = ds.SetBranchCommit(context.TODO(), commitHash, "testBranch")
		if err != nil {
			panic(err)
		}

		branchCommitHashMap, err := ds.Branches(context.TODO())
		if err != nil {
			panic(err)
		}
		assert.Equal(t, 2, len(branchCommitHashMap))
	})

	t.Run("CheckoutTestBranch", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngCIRI)
		if err != nil {
			panic(err)
		}

		st, err := ds.CheckoutBranch(context.TODO(), "testBranch", sst.DefaultTriplexMode)
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
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngCIRI)
		if err != nil {
			panic(err)
		}

		err = ds.RemoveBranch(context.TODO(), "testBranch")
		if err != nil {
			panic(err)
		}

		branchCommitHashMap, err := ds.Branches(context.TODO())
		if err != nil {
			panic(err)
		}
		assert.Equal(t, 1, len(branchCommitHashMap))

		_, err = ds.CheckoutBranch(context.TODO(), "testBranch", sst.DefaultTriplexMode)
		assert.Contains(t, err.Error(), "branch not found")
	})

	t.Run("CheckoutMasterBranch", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngCIRI)
		if err != nil {
			panic(err)
		}

		st, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
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

func Test_LocalFullRepository_IRINamedGraph(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	testIri := "http://ontology.semanticstep.net/abc#"

	t.Run("write", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		stageB := repo.OpenStage(sst.DefaultTriplexMode)

		ng := stageB.CreateNamedGraph(sst.IRI(testIri))

		mainB := ng.CreateIRINode("mainB")

		mainB.AddStatement(rdf.Type, rep.SchematicPort)

		stageB.Commit(context.TODO(), "First commit of B", sst.DefaultBranch)
	})
	t.Run("read", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), sst.IRI(testIri))
		if err != nil {
			panic(err)
		}

		st, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ng := st.NamedGraph(sst.IRI(testIri))
		if ng == nil {
			panic("got nil NamedGraph")
		}

		ng.Dump()
	})
}

func Test_LocalFullRepository_NGBImportNGC(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngBIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362").URN())
	ngCIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363").URN())

	t.Run("write", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
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

		st.Commit(context.TODO(), "Commit NGB imports NGC", sst.DefaultBranch)
	})
	t.Run("read", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngBIRI)
		if err != nil {
			panic(err)
		}

		st, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
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
}

func Test_LocalFullRepository_NGAImportsNGBImportsNGC(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngAIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361").URN())
	ngBIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362").URN())
	ngCIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363").URN())

	t.Run("write", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
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

		st.Commit(context.TODO(), "D imports A imports B", sst.DefaultBranch)
	})
	t.Run("read", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		dsD, err := repo.Dataset(context.TODO(), ngAIRI)
		if err != nil {
			panic(err)
		}

		st, err := dsD.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
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

func Test_LocalFullRepository_DiamondCase(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngAIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361").URN())
	ngBIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362").URN())
	ngCIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363").URN())
	ngDIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a364").URN())

	t.Run("write", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		ngA := st.CreateNamedGraph(ngAIRI)
		ngB := st.CreateNamedGraph(ngBIRI)
		ngC := st.CreateNamedGraph(ngCIRI)
		ngD := st.CreateNamedGraph(ngDIRI)

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

		st.Commit(context.TODO(), "Diamond Case", sst.DefaultBranch)
	})
	t.Run("read", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		datasetIRIs, err := repo.Datasets(context.TODO())
		if err != nil {
			panic(err)
		}
		assert.Equal(t, len(datasetIRIs), 4)

		for key, val := range datasetIRIs {
			fmt.Println(key, val)
		}

		dsA, err := repo.Dataset(context.TODO(), ngAIRI)
		if err != nil {
			panic(err)
		}

		st, err := dsA.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
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

func Test_LocalFullRepository_DiamondCaseCreateANDOpenLocalFullRepository(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngAIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361").URN())
	ngBIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362").URN())
	ngCIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363").URN())
	ngDIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a364").URN())

	t.Run("create", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		if err != nil {
			panic(err)
		}

		defer repo.Close()
	})

	t.Run("open and write", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		ngA := st.CreateNamedGraph(ngAIRI)
		ngB := st.CreateNamedGraph(ngBIRI)
		ngC := st.CreateNamedGraph(ngCIRI)
		ngD := st.CreateNamedGraph(ngDIRI)

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

		st.Commit(context.TODO(), "Diamond Case", sst.DefaultBranch)
	})
	t.Run("read", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		dsA, err := repo.Dataset(context.TODO(), ngAIRI)
		if err != nil {
			panic(err)
		}

		st, err := dsA.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
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

func Test_LocalFullRepository_NGAImportsNGBReferencedNGC(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngAIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa10").URN())
	ngBIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa20").URN())
	ngCIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa30").URN())

	// uuid10 := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613ad10")
	// uuid11 := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613ad11")

	// uuid20 := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613ad20")
	// uuid21 := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613ad21")

	// uuid30 := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613ad30")

	t.Run("write", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		ngA := st.CreateNamedGraph(ngAIRI)
		uuid10Node := ngA.CreateIRINode("c1efcf54-3e8e-4cc7-a7d1-82a9f613ad10", sso.ClassSystem)
		uuid11Node := ngA.CreateIRINode("c1efcf54-3e8e-4cc7-a7d1-82a9f613ad11", lci.ClassOfIndividual)
		uuid11Node.AddStatement(rdf.Type, uuid10Node)
		uuid11Node.AddStatement(rdfs.Label, sst.String("Commercial organizations"))

		ngB := st.CreateNamedGraph(ngBIRI)
		uuid20Node := ngB.CreateIRINode("c1efcf54-3e8e-4cc7-a7d1-82a9f613ad20", lci.Organization)
		uuid20Node.AddStatement(rdf.Type, uuid11Node)
		uuid20Node.AddStatement(rdfs.Label, sst.String("Test Company"))

		uuid21Node := ngB.CreateIRINode("c1efcf54-3e8e-4cc7-a7d1-82a9f613ad21", owl.ObjectProperty)
		uuid21Node.AddStatement(rdfs.SubPropertyOf, sso.ID)
		uuid21Node.AddStatement(sso.IDOwner, uuid20Node)

		ngC := st.CreateNamedGraph(ngCIRI)
		ngC.AddImport(ngB)
		uuid30Node := ngC.CreateIRINode("c1efcf54-3e8e-4cc7-a7d1-82a9f613ad30", sso.Part)
		uuid30Node.AddStatement(uuid21Node, sst.String("4711"))

		returnHash, influenceDS, err := st.Commit(context.TODO(), "aw30 is importing aw20; but aw20 is referencing aw10, but is not importing it", sst.DefaultBranch)
		if err != nil {
			panic(err)
		}
		fmt.Println("commit Hash:", returnHash)
		fmt.Println("influenceDS:", influenceDS)
		assert.Equal(t, 3, len(influenceDS))
	})
	t.Run("read_aw10", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		aw10DS, err := repo.Dataset(context.TODO(), ngAIRI)
		if err != nil {
			panic(err)
		}

		aw10Stage, err := aw10DS.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}
		assert.Equal(t, 1, len(aw10Stage.NamedGraphs()))
		assert.Equal(t, 4, len(aw10Stage.ReferencedGraphs()))

		aw10NG := aw10Stage.NamedGraph(ngAIRI)
		if aw10NG == nil {
			panic("got nil NamedGraph")
		}

		aw1File, err := os.Create(filepath.Join(t.TempDir(), "aw10.ttl"))
		if err != nil {
			panic(err)
		}
		defer aw1File.Close()
		aw10NG.RdfWrite(aw1File, sst.RdfFormatTurtle)

		aw10NG.PrintTriples()
	})

	t.Run("read_aw20", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		aw20DS, err := repo.Dataset(context.TODO(), ngBIRI)
		if err != nil {
			panic(err)
		}

		aw20Stage, err := aw20DS.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}
		assert.Equal(t, 1, len(aw20Stage.NamedGraphs()))
		assert.Equal(t, 6, len(aw20Stage.ReferencedGraphs()))

		aw20NG := aw20Stage.NamedGraph(ngBIRI)
		if aw20NG == nil {
			panic("got nil NamedGraph")
		}

		aw2File, err := os.Create(filepath.Join(t.TempDir(), "aw20.ttl"))
		if err != nil {
			panic(err)
		}
		defer aw2File.Close()
		aw20NG.RdfWrite(aw2File, sst.RdfFormatTurtle)

		aw20NG.PrintTriples()
	})

	t.Run("read_aw30", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		aw30DS, err := repo.Dataset(context.TODO(), ngCIRI)
		if err != nil {
			panic(err)
		}

		aw30Stage, err := aw30DS.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}
		assert.Equal(t, 2, len(aw30Stage.NamedGraphs()))
		assert.Equal(t, 6, len(aw30Stage.ReferencedGraphs()))

		aw30NG := aw30Stage.NamedGraph(ngCIRI)
		if aw30NG == nil {
			panic("got nil NamedGraph")
		}

		aw3File, err := os.Create(filepath.Join(t.TempDir(), "aw30.ttl"))
		if err != nil {
			panic(err)
		}
		defer aw3File.Close()
		aw30NG.RdfWrite(aw3File, sst.RdfFormatTurtle)

		aw30NG.PrintTriples()
	})
}

func Test_LocalRepository_BleveCount(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())

	query := bleve.NewMatchAllQuery()
	searchRequest := bleve.NewSearchRequest(query)

	t.Run("write", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		err = repo.RegisterIndexHandler(defaultderive.DeriveInfo())
		assert.NoError(t, err)
		openedIndex := repo.Bleve()

		searchResults1, err := openedIndex.SearchInContext(context.TODO(), searchRequest)
		if err != nil {
			panic(err)
		}

		st := repo.OpenStage(sst.DefaultTriplexMode)

		randomGraphID := uuid.New()
		fmt.Println("graphID", randomGraphID)
		graph := st.CreateNamedGraph(sst.IRI(randomGraphID.URN()))

		graph.CreateIRINode("main", lci.Organization)
		_, _, err = st.Commit(context.TODO(), "first commit: add main code", sst.DefaultBranch)
		assert.NoError(t, err)

		ds, err := repo.Dataset(context.TODO(), sst.IRI(randomGraphID.URN()))
		assert.NoError(t, err)

		err = ds.RemoveBranch(context.TODO(), sst.DefaultBranch)
		assert.NoError(t, err)
		sst.FlushBleveIndex(repo)
		// openedIndex.Delete(randomGraphID.String())

		searchResults2, err := repo.Bleve().SearchInContext(context.TODO(), searchRequest)
		if err != nil {
			panic(err)
		}
		fmt.Println("r1 :          ", searchResults1)
		for _, hit := range searchResults1.Hits {
			fmt.Println("docID =", hit.ID)
		}
		fmt.Println()
		fmt.Println("r2 :          ", searchResults2)
		for _, hit := range searchResults2.Hits {
			fmt.Println("docID =", hit.ID)
		}

		// there should be one more dataset in the repository
		assert.Equal(t, uint64(0), searchResults2.Total-searchResults1.Total)
	})
}
