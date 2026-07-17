// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

// The reason why there is a separate package is about RegisterVocabularyMap.
// In sst_test_dummyvoc package, it imports package compiler, which will register a test VocabularyMap.
// In sst_test_realvoc package, it imports package vocabularies, which will register a real
// VocabularyMap that includes lci, owl...

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/semanticstep/sst-core/defaultderive"
	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/sst_test/testutil"
	"github.com/semanticstep/sst-core/sstauth"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func Test_LocalFullRepository_UUIDNamedGraph_RemoveBranch_CheckInfo(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/" + testName)
	ngIDC := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363")

	var commitHash sst.Hash
	var modifiedDS []uuid.UUID

	defer os.RemoveAll(dir)
	t.Run("write", func(t *testing.T) {
		removeFolder(dir)

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		repo.RegisterIndexHandler(defaultderive.DeriveInfo())

		stageC := repo.OpenStage(sst.DefaultTriplexMode)

		ng := stageC.CreateNamedGraph(sst.IRI(ngIDC.URN()))

		mainC := ng.CreateIRINode("mainC")

		mainC.AddStatement(rdf.Type, rep.SchematicPort)
		commitHash, modifiedDS, err = stageC.Commit(context.TODO(), "First commit of C", sst.DefaultBranch)
		fmt.Println(commitHash)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, ngIDC, modifiedDS[0])

		info, err := repo.Info(context.TODO(), sst.DefaultBranch)
		assert.Nil(t, err)
		fmt.Println(info)
		assert.Equal(t, int64(1), info.NumberOfDatasetsInBranch)

		ds, err := repo.Dataset(context.TODO(), sst.IRI(ngIDC.URN()))
		assert.Nil(t, err)

		// DS - 2 branches: master, testbranch
		err = ds.SetBranchCommit(context.TODO(), commitHash, "testbranch")
		assert.Nil(t, err)

		// DS - 1 branch: testbranch
		err = ds.RemoveBranch(context.TODO(), sst.DefaultBranch)
		assert.Nil(t, err)

		fmt.Println("---- info on master ----")
		info, err = repo.Info(context.TODO(), sst.DefaultBranch)
		assert.Nil(t, err)
		fmt.Println(info)
		assert.Equal(t, int64(0), info.NumberOfDatasetsInBranch)

		fmt.Println("---- info on testbranch ----")
		info, err = repo.Info(context.TODO(), "testbranch")
		assert.Nil(t, err)
		fmt.Println(info)
		assert.Equal(t, int64(1), info.NumberOfDatasetsInBranch)

		fmt.Println("---- info on all branches ----")
		info, err = repo.Info(context.TODO(), "")
		assert.Nil(t, err)
		fmt.Println(info)
		assert.Equal(t, int64(1), info.NumberOfDatasetsInBranch)
	})

	t.Run("CommitDetailsByHash", func(t *testing.T) {
		t.SkipNow()
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), sst.IRI(ngIDC.URN()))
		if err != nil {
			panic(err)
		}
		branchesMap, err := ds.Branches(context.TODO())
		assert.Nil(t, err)
		assert.Equal(t, 0, len(branchesMap))

		leafCommits, err := ds.LeafCommits(context.TODO())
		assert.Nil(t, err)
		assert.Equal(t, 1, len(leafCommits))

		cd, err := ds.CommitDetailsByHash(context.TODO(), leafCommits[0])
		assert.Nil(t, err)
		assert.Equal(t, "First commit of C", cd.Message)

		err = ds.SetBranchCommit(context.TODO(), leafCommits[0], "testBranch")
		assert.Nil(t, err)

		info, err := repo.Info(context.TODO(), "")
		assert.Nil(t, err)
		assert.Equal(t, int64(1), info.NumberOfDatasets)
		assert.Equal(t, int64(1), info.NumberOfDatasetsInBranch)

		info, err = repo.Info(context.TODO(), "master")
		assert.Nil(t, err)
		assert.Equal(t, int64(1), info.NumberOfDatasets)
		assert.Equal(t, int64(0), info.NumberOfDatasetsInBranch)

		info, err = repo.Info(context.TODO(), "testBranch")
		assert.Nil(t, err)
		assert.Equal(t, int64(1), info.NumberOfDatasets)
		assert.Equal(t, int64(1), info.NumberOfDatasetsInBranch)

		branchesMap, err = ds.Branches(context.TODO())
		assert.Nil(t, err)
		assert.Equal(t, 1, len(branchesMap))
		assert.Equal(t, leafCommits[0], branchesMap["testBranch"])

		cd, err = ds.CommitDetailsByBranch(context.TODO(), "testBranch")
		assert.Nil(t, err)
		assert.Equal(t, "First commit of C", cd.Message)
	})
}

func Test_LocalFullRepository_UUIDNamedGraph_CommitToEmptyBranch(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/" + testName)
	ngIDC := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363")

	var modifiedDS []uuid.UUID

	defer os.RemoveAll(dir)
	t.Run("write", func(t *testing.T) {
		removeFolder(dir)

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		repo.RegisterIndexHandler(defaultderive.DeriveInfo())

		stageC := repo.OpenStage(sst.DefaultTriplexMode)

		ng := stageC.CreateNamedGraph(sst.IRI(ngIDC.URN()))

		mainC := ng.CreateIRINode("mainC")

		mainC.AddStatement(rdf.Type, rep.SchematicPort)
		_, modifiedDS, _ = stageC.Commit(context.TODO(), "First commit of C", "")
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, ngIDC, modifiedDS[0])
		info, err := repo.Info(context.TODO(), "")
		assert.Nil(t, err)
		assert.Equal(t, int64(0), info.NumberOfDatasetsInBranch)
	})

	t.Run("CommitDetailsByHash", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), sst.IRI(ngIDC.URN()))
		if err != nil {
			panic(err)
		}
		branchesMap, err := ds.Branches(context.TODO())
		assert.Nil(t, err)
		assert.Equal(t, 0, len(branchesMap))

		leafCommits, err := ds.LeafCommits(context.TODO())
		assert.Nil(t, err)
		assert.Equal(t, 1, len(leafCommits))

		cd, err := ds.CommitDetailsByHash(context.TODO(), leafCommits[0])
		assert.Nil(t, err)
		assert.Equal(t, "First commit of C", cd.Message)

		err = ds.SetBranchCommit(context.TODO(), leafCommits[0], "testBranch")
		assert.Nil(t, err)

		info, err := repo.Info(context.TODO(), "")
		assert.Nil(t, err)
		assert.Equal(t, int64(1), info.NumberOfDatasets)
		assert.Equal(t, int64(1), info.NumberOfDatasetsInBranch)

		info, err = repo.Info(context.TODO(), "master")
		assert.Nil(t, err)
		assert.Equal(t, int64(1), info.NumberOfDatasets)
		assert.Equal(t, int64(0), info.NumberOfDatasetsInBranch)

		info, err = repo.Info(context.TODO(), "testBranch")
		assert.Nil(t, err)
		assert.Equal(t, int64(1), info.NumberOfDatasets)
		assert.Equal(t, int64(1), info.NumberOfDatasetsInBranch)

		branchesMap, err = ds.Branches(context.TODO())
		assert.Nil(t, err)
		assert.Equal(t, 1, len(branchesMap))
		assert.Equal(t, leafCommits[0], branchesMap["testBranch"])

		cd, err = ds.CommitDetailsByBranch(context.TODO(), "testBranch")
		assert.Nil(t, err)
		assert.Equal(t, "First commit of C", cd.Message)
	})
}

func Test_RemoteRepository_UUIDNamedGraph_CommitToEmptyBranch(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/" + testName)
	ngIDC := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363")
	transportCreds, err := testutil.TestTransportCreds()
	assert.NoError(t, err)

	var modifiedDS []uuid.UUID

	defer os.RemoveAll(dir)
	t.Run("write", func(t *testing.T) {
		removeFolder(dir)

		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		repo.RegisterIndexHandler(defaultderive.DeriveInfo())

		stageC := repo.OpenStage(sst.DefaultTriplexMode)

		ng := stageC.CreateNamedGraph(sst.IRI(ngIDC.URN()))

		mainC := ng.CreateIRINode("mainC")

		mainC.AddStatement(rdf.Type, rep.SchematicPort)
		_, modifiedDS, err = stageC.Commit(constructCtx, "First commit of C", "")
		assert.NoError(t, err)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, ngIDC, modifiedDS[0])
		info, err := repo.Info(constructCtx, "")
		assert.Nil(t, err)
		assert.Equal(t, int64(0), info.NumberOfDatasetsInBranch)
	})

	t.Run("CommitDetailsByHash", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(constructCtx, sst.IRI(ngIDC.URN()))
		if err != nil {
			panic(err)
		}
		branchesMap, err := ds.Branches(constructCtx)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(branchesMap))

		leafCommits, err := ds.LeafCommits(constructCtx)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(leafCommits))

		cd, err := ds.CommitDetailsByHash(constructCtx, leafCommits[0])
		assert.Nil(t, err)
		assert.Equal(t, "First commit of C", cd.Message)

		err = ds.SetBranchCommit(constructCtx, leafCommits[0], "testBranch")
		assert.Nil(t, err)

		info, err := repo.Info(constructCtx, "")
		assert.Nil(t, err)
		assert.Equal(t, int64(1), info.NumberOfDatasets)
		assert.Equal(t, int64(1), info.NumberOfDatasetsInBranch)

		info, err = repo.Info(constructCtx, "master")
		assert.Nil(t, err)
		assert.Equal(t, int64(1), info.NumberOfDatasets)
		assert.Equal(t, int64(0), info.NumberOfDatasetsInBranch)

		info, err = repo.Info(constructCtx, "testBranch")
		assert.Nil(t, err)
		assert.Equal(t, int64(1), info.NumberOfDatasets)
		assert.Equal(t, int64(1), info.NumberOfDatasetsInBranch)

		branchesMap, err = ds.Branches(constructCtx)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(branchesMap))
		assert.Equal(t, leafCommits[0], branchesMap["testBranch"])

		cd, err = ds.CommitDetailsByBranch(constructCtx, "testBranch")
		assert.Nil(t, err)
		assert.Equal(t, "First commit of C", cd.Message)
	})
}

func Test_LocalFullRepository_UUIDNamedGraph_CommitToBranch(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/" + testName)
	ngIDC := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363")

	var commitHash1 sst.Hash
	var modifiedDS []uuid.UUID

	defer os.RemoveAll(dir)
	t.Run("write", func(t *testing.T) {
		removeFolder(dir)

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		stageC := repo.OpenStage(sst.DefaultTriplexMode)

		ng := stageC.CreateNamedGraph(sst.IRI(ngIDC.URN()))

		mainC := ng.CreateIRINode("mainC")

		mainC.AddStatement(rdf.Type, rep.SchematicPort)
		commitHash1, modifiedDS, _ = stageC.Commit(context.TODO(), "First commit of C", "commit1")
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, ngIDC, modifiedDS[0])

		ds, err := repo.Dataset(context.TODO(), sst.IRI(ngIDC.URN()))
		assert.Nil(t, err)

		err = ds.SetBranchCommit(context.TODO(), commitHash1, "branch2")
		assert.Nil(t, err)
	})

	t.Run("CommitDetailsByHash", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), sst.IRI(ngIDC.URN()))
		if err != nil {
			panic(err)
		}
		branchesMap, err := ds.Branches(context.TODO())
		if err != nil {
			panic(err)
		}
		assert.Equal(t, 2, len(branchesMap))
		assert.Equal(t, branchesMap["commit1"], commitHash1)

		commitDetails1, err := ds.CommitDetailsByBranch(context.TODO(), "commit1")
		assert.Nil(t, err)

		assert.Equal(t, "First commit of C", commitDetails1.Message)
		assert.NotNil(t, commitDetails1.DatasetRevisions[sst.IRI(ngIDC.URN())])
		// commitDetails1.Dump()
		assert.Equal(t, 0, len(commitDetails1.ParentCommits))

		commitDetails2, err := ds.CommitDetailsByBranch(context.TODO(), "branch2")
		assert.Nil(t, err)
		assert.Equal(t, commitDetails1, commitDetails2)

		err = ds.RemoveBranch(context.TODO(), "branch2")
		assert.Nil(t, err)

		branchesMap, err = ds.Branches(context.TODO())
		assert.Nil(t, err)
		assert.Equal(t, 1, len(branchesMap))

		lfs, err := ds.LeafCommits(context.TODO())
		assert.Nil(t, err)
		assert.Equal(t, 0, len(lfs))

		err = ds.RemoveBranch(context.TODO(), "commit1")
		assert.Nil(t, err)

		branchesMap, err = ds.Branches(context.TODO())
		assert.Nil(t, err)
		assert.Equal(t, 0, len(branchesMap))

		lfs, err = ds.LeafCommits(context.TODO())
		assert.Nil(t, err)
		assert.Equal(t, 1, len(lfs))
	})
}

func Test_LocalFullRepository_SetBranch(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/" + testName)
	ngIDC := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363")

	var commitHash sst.Hash
	var modifiedDSIDs []uuid.UUID

	defer os.RemoveAll(dir)
	t.Run("write", func(t *testing.T) {
		removeFolder(dir)

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		stageC := repo.OpenStage(sst.DefaultTriplexMode)

		ng := stageC.CreateNamedGraph(sst.IRI(ngIDC.URN()))

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
		assert.Equal(t, modifiedDSIDs[0], ngIDC)
	})

	t.Run("leafCommits", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), sst.IRI(ngIDC.URN()))
		if err != nil {
			panic(err)
		}
		commitHashes, err := ds.LeafCommits(context.TODO())
		assert.Nil(t, err)
		assert.Equal(t, 0, len(commitHashes))
	})

	t.Run("CommitDetailsByHash", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), sst.IRI(ngIDC.URN()))
		if err != nil {
			panic(err)
		}
		cd, err := ds.CommitDetailsByHash(context.TODO(), commitHash)
		assert.Nil(t, err)

		assert.Equal(t, "First commit of C", cd.Message)
	})

	t.Run("CommitDetailsByBranch", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), sst.IRI(ngIDC.URN()))
		if err != nil {
			panic(err)
		}
		cd, err := ds.CommitDetailsByBranch(context.TODO(), sst.DefaultBranch)
		assert.Nil(t, err)

		assert.Equal(t, "First commit of C", cd.Message)
	})

	t.Run("setBranch", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), sst.IRI(ngIDC.URN()))
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

		ds, err := repo.Dataset(context.TODO(), sst.IRI(ngIDC.URN()))
		if err != nil {
			panic(err)
		}

		s, err := ds.CheckoutBranch(context.TODO(), "testBranch", sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ng := s.NamedGraph(sst.IRI(ngIDC.URN()))
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

		ds, err := repo.Dataset(context.TODO(), sst.IRI(ngIDC.URN()))
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

		ds, err := repo.Dataset(context.TODO(), sst.IRI(ngIDC.URN()))
		if err != nil {
			panic(err)
		}

		s, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ng := s.NamedGraph(sst.IRI(ngIDC.URN()))
		if ng == nil {
			panic("got nil NamedGraph")
		}

		ng.Dump()
	})
}
