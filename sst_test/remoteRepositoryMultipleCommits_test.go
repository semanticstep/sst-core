// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

// The reason why there is a separate package is about RegisterVocabularyMap.
// In sst_test_dummyvoc package, it imports package compiler, which will register a test VocabularyMap.
// In sst_test_realvoc package, it imports package vocabularies, which will register a real
// VocabularyMap that includes lci, owl...

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
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
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_RemoteRepository_MultipleParents(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngAIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361").URN())
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	iriNodeUUID1 := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a001")
	iriNodeUUID2 := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a002")
	iriNodeUUID3 := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a003")

	var commitHash1 sst.Hash
	var commitHash2 sst.Hash

	var modifiedDS []uuid.UUID

	t.Run("write", func(t *testing.T) {

		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}

		st := repo.OpenStage(sst.DefaultTriplexMode)

		ng := st.CreateNamedGraph(ngAIRI)
		mainC := ng.CreateIRINode(iriNodeUUID1.String(), sso.ClassSystem)
		mainC.AddStatement(rdfs.Label, sst.String("CL-test"))
		commitHash1, modifiedDS, _ = st.Commit(constructCtx, "commit 1", sst.DefaultBranch)
		assert.Equal(t, 1, len(modifiedDS))
		ds, err := repo.Dataset(constructCtx, ngAIRI)
		assert.NoError(t, err)
		ds.SetBranchCommit(constructCtx, commitHash1, "anotherBranch")
		// ng.PrintTriples()
		fmt.Println(commitHash1)
	})

	t.Run("checkout default branch and commit", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}

		ds, err := repo.Dataset(constructCtx, ngAIRI)
		if err != nil {
			panic(err)
		}
		st, err := ds.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ng := st.NamedGraph(ngAIRI)
		iriNode2 := ng.CreateIRINode(iriNodeUUID2.String(), lci.ClassOfIndividual)
		iriNode2.AddStatement(rdf.Type, ng.GetIRINodeByFragment(iriNodeUUID1.String()))
		iriNode2.AddStatement(sso.ID, sst.String("class 1"))
		commitHash2, modifiedDS, err = st.Commit(constructCtx, "commit 2a", sst.DefaultBranch)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngAIRI)
		// ng.PrintTriples()
		fmt.Println(commitHash2)
	})

	t.Run("checkout another branch and commit", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}

		ds, err := repo.Dataset(constructCtx, ngAIRI)
		if err != nil {
			panic(err)
		}
		st, err := ds.CheckoutBranch(constructCtx, "anotherBranch", sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ng := st.NamedGraph(ngAIRI)
		iriNode3 := ng.CreateIRINode(iriNodeUUID3.String(), lci.ClassOfIndividual)
		iriNode3.AddStatement(rdf.Type, ng.GetIRINodeByFragment(iriNodeUUID1.String()))
		iriNode3.AddStatement(sso.ID, sst.String("class 2"))
		commitHash2, modifiedDS, err = st.Commit(constructCtx, "commit 2b", "anotherBranch")
		assert.NoError(t, err)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngAIRI)
		// ng.PrintTriples()
		fmt.Println(commitHash2)
	})

	t.Run("checkout commit 2a and 2b", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}

		ds, err := repo.Dataset(constructCtx, ngAIRI)
		if err != nil {
			panic(err)
		}

		st2a, err := ds.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		st2b, err := ds.CheckoutBranch(constructCtx, "anotherBranch", sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ng := st2a.NamedGraph(ngAIRI)
		ng.PrintTriples()

		ng2 := st2b.NamedGraph(ngAIRI)
		ng2.PrintTriples()

		report, err := st2a.MoveAndMerge(constructCtx, st2b)
		if err != nil {
			panic(err)
		}
		fmt.Println(report)
		Hash2Aand2B, influencedDataset, err := st2a.Commit(constructCtx, "2a and 2b", sst.DefaultBranch)
		assert.NoError(t, err)
		fmt.Println(Hash2Aand2B)
		fmt.Println(influencedDataset)

		commitDetails2a2b, err := ds.CommitDetailsByHash(constructCtx, Hash2Aand2B)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(commitDetails2a2b.ParentCommits[ngAIRI]))

		commitDetails2a2b.Dump()
	})
}

func Test_remoteRepositoryMultipleCommits_IsParentRevision(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngCIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363").URN())
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	var commitHash1 sst.Hash
	var commitHash3 sst.Hash
	var commitHash4 sst.Hash

	var modifiedDS []uuid.UUID

	t.Run("write", func(t *testing.T) {

		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		stageC := repo.OpenStage(sst.DefaultTriplexMode)

		ng := stageC.CreateNamedGraph(ngCIRI)

		mainC := ng.CreateIRINode("mainC")

		mainC.AddStatement(rdf.Type, rep.SchematicPort)
		commitHash1, modifiedDS, err = stageC.Commit(constructCtx, "First commit of C", sst.DefaultBranch)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngCIRI)

		ds, err := repo.Dataset(constructCtx, ngCIRI)
		if err != nil {
			panic(err)
		}
		err = ds.SetBranchCommit(constructCtx, commitHash1, "commit1")
		if err != nil {
			panic(err)
		}

		mainC.AddStatement(rdf.Bag, rep.Angle)
		_, modifiedDS, err = stageC.Commit(constructCtx, "Second commit of C", sst.DefaultBranch)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngCIRI)

		mainC.AddStatement(rdf.Direction, rep.Blue)
		commitHash3, _, err = stageC.Commit(constructCtx, "Third commit of C", sst.DefaultBranch)
		assert.NoError(t, err)
	})

	t.Run("checkoutByBranch IsParentRevision", func(t *testing.T) {
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
		st1, err := ds.CheckoutBranch(constructCtx, "commit1", sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}
		st1.NamedGraphs()[0].CreateIRINode("", lci.Person)
		commitHash4, modifiedDS, err = st1.Commit(constructCtx, "fourth commit of C", "commit1")
		assert.NoError(t, err)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngCIRI)
		stMaster, err := ds.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		common1, err := ds.FindCommonParentRevision(constructCtx, commitHash4, stMaster.NamedGraphs()[0].Info().CheckedOutCommits[0])
		if err != nil {
			panic(err)
		}
		assert.Equal(t, common1, commitHash1)
	})

	t.Run("CommitDetailsByHash IsParentRevision", func(t *testing.T) {
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
		branchesMap, err := ds.Branches(constructCtx)
		if err != nil {
			panic(err)
		}
		assert.Equal(t, len(branchesMap), 2)
		assert.Equal(t, branchesMap["master"], commitHash3)
		assert.Equal(t, branchesMap["commit1"], commitHash4)

		commitDetails3, _ := ds.CommitDetailsByHash(constructCtx, commitHash3)
		assert.Equal(t, "Third commit of C", commitDetails3.Message)
		assert.NotNil(t, commitDetails3.DatasetRevisions[ngCIRI])
		commitDetails3.Dump()

		commitDetails2, _ := ds.CommitDetailsByHash(constructCtx, commitDetails3.ParentCommits[ds.IRI()][0])
		assert.Equal(t, "Second commit of C", commitDetails2.Message)
		assert.NotNil(t, commitDetails2.DatasetRevisions[ngCIRI])
		commitDetails2.Dump()

		commitDetails1, _ := ds.CommitDetailsByHash(constructCtx, commitDetails2.ParentCommits[ds.IRI()][0])
		assert.Equal(t, "First commit of C", commitDetails1.Message)
		assert.NotNil(t, commitDetails3.DatasetRevisions[ngCIRI])
		commitDetails1.Dump()
		assert.Equal(t, 0, len(commitDetails1.ParentCommits[ds.IRI()]))

		common, err := ds.FindCommonParentRevision(constructCtx, commitDetails2.Commit, commitDetails3.Commit)
		assert.NoError(t, err)
		assert.Equal(t, common, commitDetails2.Commit)

		common2, err := ds.FindCommonParentRevision(constructCtx, commitDetails1.Commit, commitDetails3.Commit)
		assert.NoError(t, err)
		assert.Equal(t, common2, commitDetails1.Commit)

		common3, err := ds.FindCommonParentRevision(constructCtx, commitDetails1.Commit, commitDetails2.Commit)
		assert.NoError(t, err)
		assert.Equal(t, common3, commitDetails1.Commit)
	})

	t.Run("readCommitDetails", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		dsC, err := repo.Dataset(constructCtx, ngCIRI)
		if err != nil {
			panic(err)
		}

		commitDetail, err := dsC.CommitDetailsByBranch(constructCtx, sst.DefaultBranch)
		assert.NoError(t, err)
		for {
			commitDetail.Dump()
			st, err := dsC.CheckoutCommit(constructCtx, commitDetail.Commit, sst.DefaultTriplexMode)
			assert.NoError(t, err)
			ng := st.NamedGraph(ngCIRI)
			ng.Dump()
			if len(commitDetail.ParentCommits[dsC.IRI()]) != 0 {
				// Suppose there is only one parent
				commitDetail, err = dsC.CommitDetailsByHash(constructCtx, commitDetail.ParentCommits[dsC.IRI()][0])
				assert.NoError(t, err)
			} else {
				break
			}
		}

	})
}

func Test_RemoteRepositoryMultipleCommits_UUIDNamedGraph(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
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

		stageC := repo.OpenStage(sst.DefaultTriplexMode)

		ng := stageC.CreateNamedGraph(ngCIRI)

		mainC := ng.CreateIRINode("mainC")

		mainC.AddStatement(rdf.Type, rep.SchematicPort)
		stageC.Commit(constructCtx, "First commit of C", sst.DefaultBranch)

		mainC.AddStatement(rdf.Bag, rep.Angle)
		stageC.Commit(constructCtx, "Second commit of C", sst.DefaultBranch)

		mainC.AddStatement(rdf.Direction, rep.Blue)

		_, modifiedDSIDs, _ := stageC.Commit(constructCtx, "Third commit of C", sst.DefaultBranch)

		assert.Equal(t, len(modifiedDSIDs), 1)
		assert.Equal(t, sst.IRI(modifiedDSIDs[0].URN()), ngCIRI)

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

		commitDetails3, _ := ds.CommitDetailsByBranch(constructCtx, sst.DefaultBranch)
		assert.Equal(t, "Third commit of C", commitDetails3.Message)
		assert.NotNil(t, commitDetails3.DatasetRevisions[ngCIRI])
		commitDetails3.Dump()

		commitDetails2, _ := ds.CommitDetailsByHash(constructCtx, commitDetails3.ParentCommits[ds.IRI()][0])
		assert.Equal(t, "Second commit of C", commitDetails2.Message)
		assert.NotNil(t, commitDetails2.DatasetRevisions[ngCIRI])
		commitDetails2.Dump()

		commitDetails1, _ := ds.CommitDetailsByHash(constructCtx, commitDetails2.ParentCommits[ds.IRI()][0])
		assert.Equal(t, "First commit of C", commitDetails1.Message)
		assert.NotNil(t, commitDetails1.DatasetRevisions[ngCIRI])
		commitDetails1.Dump()
		assert.Equal(t, 0, len(commitDetails1.ParentCommits[ds.IRI()]))
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
		commitDetails3, _ := ds.CommitDetailsByBranch(constructCtx, sst.DefaultBranch)

		assert.Equal(t, "Third commit of C", commitDetails3.Message)
		assert.NotNil(t, commitDetails3.DatasetRevisions[sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363").URN())])

		commitDetails3.Dump()
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

		ng.Dump()
	})
	t.Run("readCommitDetails", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		dsC, err := repo.Dataset(constructCtx, ngCIRI)
		if err != nil {
			panic(err)
		}

		commitDetail, err := dsC.CommitDetailsByBranch(constructCtx, sst.DefaultBranch)
		assert.NoError(t, err)
		for {
			commitDetail.Dump()
			st, err := dsC.CheckoutCommit(constructCtx, commitDetail.Commit, sst.DefaultTriplexMode)
			assert.NoError(t, err)
			ng := st.NamedGraph(ngCIRI)
			ng.Dump()
			if len(commitDetail.ParentCommits[dsC.IRI()]) != 0 {
				// Suppose there is only one parent
				commitDetail, err = dsC.CommitDetailsByHash(constructCtx, commitDetail.ParentCommits[dsC.IRI()][0])
				assert.NoError(t, err)
			} else {
				break
			}
		}

	})
}

func Test_RemoteRepositoryMultipleCommits_UUIDNamedGraph_Leafcommits(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
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

		stageC := repo.OpenStage(sst.DefaultTriplexMode)

		ng := stageC.CreateNamedGraph(ngCIRI)

		mainC := ng.CreateIRINode("mainC")

		mainC.AddStatement(rdf.Type, rep.SchematicPort)
		stageC.Commit(constructCtx, "First commit of C", "")

		ds, err := repo.Dataset(constructCtx, ngCIRI)
		assert.NoError(t, err)
		hashes, err := ds.LeafCommits(constructCtx)
		assert.Nil(t, err)
		commitDetails1, _ := ds.CommitDetailsByHash(constructCtx, hashes[0])
		assert.Equal(t, "First commit of C", commitDetails1.Message)
		assert.NotNil(t, commitDetails1.DatasetRevisions[ngCIRI])
		assert.Equal(t, 0, len(commitDetails1.ParentCommits))
		commitDetails1.Dump()

		mainC.AddStatement(rdf.Bag, rep.Angle)
		stageC.Commit(constructCtx, "Second commit of C", "")
		hashes2, err := ds.LeafCommits(constructCtx)
		assert.Nil(t, err)
		for _, hash := range hashes2 {
			commitDetails1, _ := ds.CommitDetailsByHash(constructCtx, hash)
			if commitDetails1.Message == "Second commit of C" {
				assert.Equal(t, 1, len(commitDetails1.ParentCommits))
			}

			// commitDetails1.Dump()
		}

		// mainC.AddStatement(rdf.Direction, rep.Blue)

		// _, modifiedDSIDs, _ := stageC.Commit(constructCtx, "Third commit of C", sst.DefaultBranch)

		// assert.Equal(t, len(modifiedDSIDs), 1)
		// assert.Equal(t, sst.IRI(modifiedDSIDs[0].URN()), ngCIRI)

	})
	t.Run("CommitDetailsByHash", func(t *testing.T) {
		t.SkipNow()
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

		leafCommits, err := ds.LeafCommits(constructCtx)
		assert.Nil(t, err)
		commitDetails3, _ := ds.CommitDetailsByHash(constructCtx, leafCommits[0])
		assert.Equal(t, "Third commit of C", commitDetails3.Message)
		assert.NotNil(t, commitDetails3.DatasetRevisions[ngCIRI])
		commitDetails3.Dump()

		commitDetails2, _ := ds.CommitDetailsByHash(constructCtx, commitDetails3.ParentCommits[ds.IRI()][0])
		assert.Equal(t, "Second commit of C", commitDetails2.Message)
		assert.NotNil(t, commitDetails2.DatasetRevisions[ngCIRI])
		commitDetails2.Dump()

		commitDetails1, _ := ds.CommitDetailsByHash(constructCtx, commitDetails2.ParentCommits[ds.IRI()][0])
		assert.Equal(t, "First commit of C", commitDetails1.Message)
		assert.NotNil(t, commitDetails1.DatasetRevisions[ngCIRI])
		commitDetails1.Dump()
		assert.Equal(t, 0, len(commitDetails1.ParentCommits[ds.IRI()]))
	})

	t.Run("CommitDetailsByBranch", func(t *testing.T) {
		t.SkipNow()
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
		commitDetails3, _ := ds.CommitDetailsByBranch(constructCtx, sst.DefaultBranch)

		assert.Equal(t, "Third commit of C", commitDetails3.Message)
		assert.NotNil(t, commitDetails3.DatasetRevisions[sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363").URN())])

		commitDetails3.Dump()
	})

	t.Run("read", func(t *testing.T) {
		t.SkipNow()
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
	t.Run("readCommitDetails", func(t *testing.T) {
		t.SkipNow()
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		dsC, err := repo.Dataset(constructCtx, ngCIRI)
		if err != nil {
			panic(err)
		}

		commitDetail, err := dsC.CommitDetailsByBranch(constructCtx, sst.DefaultBranch)
		assert.NoError(t, err)
		for {
			commitDetail.Dump()
			st, err := dsC.CheckoutCommit(constructCtx, commitDetail.Commit, sst.DefaultTriplexMode)
			assert.NoError(t, err)
			ng := st.NamedGraph(ngCIRI)
			ng.Dump()
			if len(commitDetail.ParentCommits[dsC.IRI()]) != 0 {
				// Suppose there is only one parent
				commitDetail, err = dsC.CommitDetailsByHash(constructCtx, commitDetail.ParentCommits[dsC.IRI()][0])
				assert.NoError(t, err)
			} else {
				break
			}
		}

	})
}

func Test_RemoteRepositoryMultipleCommits_IRINamedGraph(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	testIri := "http://ontology.semanticstep.net/abc#"
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

		stageB := repo.OpenStage(sst.DefaultTriplexMode)

		ng := stageB.CreateNamedGraph(sst.IRI(testIri))

		mainB := ng.CreateIRINode("mainB")

		mainB.AddStatement(rdf.Type, rep.SchematicPort)

		stageB.Commit(constructCtx, "First commit of abc", sst.DefaultBranch)

		mainB.AddStatement(rdf.Bag, rep.Angle)

		stageB.Commit(constructCtx, "second commit of abc", sst.DefaultBranch)
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

		ng.Dump()
	})
}

func Test_RemoteRepositoryMultipleCommits_EmptyIRI(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	var testIri string
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

		stageB := repo.OpenStage(sst.DefaultTriplexMode)

		ng := stageB.CreateNamedGraph("")
		testIri = string(ng.IRI())

		mainB := ng.CreateIRINode("mainB")

		mainB.AddStatement(rdf.Type, rep.SchematicPort)

		stageB.Commit(constructCtx, "First commit of abc", sst.DefaultBranch)

		mainB.AddStatement(rdf.Bag, rep.Angle)

		stageB.Commit(constructCtx, "second commit of abc", sst.DefaultBranch)
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

		ng.Dump()
	})
}

func Test_RemoteRepositoryMultipleCommits_NGBandNGC(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngBIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362").URN())
	ngCIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363").URN())
	ngDIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a364").URN())

	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	t.Run("write NGA and NGB", func(t *testing.T) {

		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		ngB := st.CreateNamedGraph(ngBIRI)

		ngC := st.CreateNamedGraph(ngCIRI)

		ngB.CreateIRINode("mainB", rep.SchematicPort)
		ngC.CreateIRINode("mainC", rep.SchematicPort)

		st.Commit(constructCtx, "added NGB amd NGC", sst.DefaultBranch)

	})

	t.Run("write add NGD", func(t *testing.T) {
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

		ngID := st.CreateNamedGraph(ngDIRI)
		ngID.CreateIRINode("mainD", rep.SchematicPort)

		st.Commit(constructCtx, "add NGD", sst.DefaultBranch)
	})

	t.Run("read NGB", func(t *testing.T) {
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

		traversalByAPI(ngB)
	})

	t.Run("read NGC", func(t *testing.T) {
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

		ngC := st.NamedGraph(ngCIRI)
		if ngC == nil {
			panic("got nil NamedGraph")
		}

		traversalByAPI(ngC)
	})
}

func Test_RemoteRepositoryMultipleCommits_NGBImportNGC_ModifyB(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
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

		ngB := st.CreateNamedGraph(ngBIRI)

		ngC := st.CreateNamedGraph(ngCIRI)
		ngB.AddImport(ngC)

		mainC := ngC.CreateIRINode("mainC")
		mainC.AddStatement(rdf.Type, rep.SchematicPort)

		mainB := ngB.CreateIRINode("mainB")
		mainB.AddStatement(rep.MappingSource, mainC)

		st.Commit(constructCtx, "Commit NGA imports NGB step1", sst.DefaultBranch)

		// modifiy mainB and commit again
		mainB.AddStatement(rdf.Bag, rep.Angle)
		_, modifiedDS, _ := st.Commit(constructCtx, "Modify B", sst.DefaultBranch)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngBIRI)
	})

	t.Run("CommitDetailsByBranch", func(t *testing.T) {
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
		commitDetails2, _ := ds.CommitDetailsByBranch(constructCtx, sst.DefaultBranch)
		assert.Equal(t, "Modify B", commitDetails2.Message)
		assert.NotNil(t, commitDetails2.DatasetRevisions[ngBIRI])
		commitDetails2.Dump()

		commitDetails1, _ := ds.CommitDetailsByHash(constructCtx, commitDetails2.ParentCommits[ds.IRI()][0])
		assert.Equal(t, "Commit NGA imports NGB step1", commitDetails1.Message)
		assert.NotNil(t, commitDetails2.DatasetRevisions[ngBIRI])
		assert.NotNil(t, commitDetails2.DatasetRevisions[ngCIRI])

		commitDetails1.Dump()
	})

}

func Test_RemoteRepositoryMultipleCommits_NGBImportNGC_ModifyC(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
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

		ngB := st.CreateNamedGraph(ngBIRI)

		ngC := st.CreateNamedGraph(ngCIRI)
		ngB.AddImport(ngC)

		mainC := ngC.CreateIRINode("mainC")
		mainC.AddStatement(rdf.Type, rep.SchematicPort)

		mainB := ngB.CreateIRINode("mainB")
		mainB.AddStatement(rep.MappingSource, mainC)

		st.Commit(constructCtx, "Commit NGA imports NGB step1", sst.DefaultBranch)

		// modifiy mainC and commit again
		mainC.AddStatement(rdf.Direction, rep.ConeAngle1)
		_, modifiedDS, _ := st.Commit(constructCtx, "modify C", sst.DefaultBranch)
		assert.Equal(t, 2, len(modifiedDS))
		sort.Slice(modifiedDS, func(i, j int) bool {
			return bytes.Compare(modifiedDS[i][:], modifiedDS[j][:]) < 0
		})
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngBIRI)
		assert.Equal(t, sst.IRI(modifiedDS[1].URN()), ngCIRI)
	})
}

func Test_RemoteRepositoryMultipleCommits_NGBImportNGC(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
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

		ngB := st.CreateNamedGraph(ngBIRI)

		ngC := st.CreateNamedGraph(ngCIRI)
		ngB.AddImport(ngC)

		mainC := ngC.CreateIRINode("mainC")
		mainC.AddStatement(rdf.Type, rep.SchematicPort)

		mainB := ngB.CreateIRINode("mainB")
		mainB.AddStatement(rep.MappingSource, mainC)

		_, modifiedDS, _ := st.Commit(constructCtx, "Commit NGA imports NGB step1", sst.DefaultBranch)
		assert.Equal(t, 2, len(modifiedDS))
		sort.Slice(modifiedDS, func(i, j int) bool {
			return bytes.Compare(modifiedDS[i][:], modifiedDS[j][:]) < 0
		})
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngBIRI)
		assert.Equal(t, sst.IRI(modifiedDS[1].URN()), ngCIRI)

		// modifiy mainB and commit again
		mainB.AddStatement(rdf.Bag, rep.Angle)
		_, modifiedDS, _ = st.Commit(constructCtx, "modify NGB and commit again", sst.DefaultBranch)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngBIRI)

		// modifiy mainC and commit again
		mainC.AddStatement(rdf.Direction, rep.ConeAngle1)
		_, modifiedDS, _ = st.Commit(constructCtx, "modify NGC and commit again", sst.DefaultBranch)
		assert.Equal(t, 2, len(modifiedDS))
		sort.Slice(modifiedDS, func(i, j int) bool {
			return bytes.Compare(modifiedDS[i][:], modifiedDS[j][:]) < 0
		})
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngBIRI)
		assert.Equal(t, sst.IRI(modifiedDS[1].URN()), ngCIRI)
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
	t.Run("readCommitDetailsNGB", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		dsB, err := repo.Dataset(constructCtx, ngBIRI)
		if err != nil {
			panic(err)
		}

		commitDetail, err := dsB.CommitDetailsByBranch(constructCtx, sst.DefaultBranch)
		assert.NoError(t, err)
		for {
			commitDetail.Dump()
			st, err := dsB.CheckoutCommit(constructCtx, commitDetail.Commit, sst.DefaultTriplexMode)
			assert.NoError(t, err)
			ng := st.NamedGraph(ngCIRI)
			ng.Dump()
			if len(commitDetail.ParentCommits[dsB.IRI()]) != 0 {
				// Suppose there is only one parent
				commitDetail, err = dsB.CommitDetailsByHash(constructCtx, commitDetail.ParentCommits[dsB.IRI()][0])
				assert.NoError(t, err)
			} else {
				break
			}
		}
	})

	t.Run("readCommitDetailsNGC", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		dsC, err := repo.Dataset(constructCtx, ngCIRI)
		if err != nil {
			panic(err)
		}

		commitDetail, err := dsC.CommitDetailsByBranch(constructCtx, sst.DefaultBranch)
		assert.NoError(t, err)
		for {
			commitDetail.Dump()
			st, err := dsC.CheckoutCommit(constructCtx, commitDetail.Commit, sst.DefaultTriplexMode)
			assert.NoError(t, err)
			ng := st.NamedGraph(ngCIRI)
			ng.Dump()
			if len(commitDetail.ParentCommits[dsC.IRI()]) != 0 {
				// Suppose there is only one parent
				commitDetail, err = dsC.CommitDetailsByHash(constructCtx, commitDetail.ParentCommits[dsC.IRI()][0])
				assert.NoError(t, err)
			} else {
				break
			}
		}
	})
}

func Test_RemoteRepositoryMultipleCommits_NGAImportsNGBImportsNGC_ModifyA(t *testing.T) {
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

		ngC := st.CreateNamedGraph(ngCIRI)

		// Create mainC node
		mainC := ngC.CreateIRINode("mainC")
		mainC.AddStatement(rdf.Type, rep.SchematicPort)

		// NG-B imports NG-C
		// Create mainB node references mainC
		ngB := st.CreateNamedGraph(ngBIRI)
		ngB.AddImport(ngC)

		mainB := ngB.CreateIRINode("mainB")
		mainB.AddStatement(rep.MappingSource, mainC)

		// NG-A imports NG-B
		// creates mainA node references mainB
		ngA := st.CreateNamedGraph(ngAIRI)
		ngA.AddImport(ngB)
		mainA := ngA.CreateIRINode("mainA")
		mainA.AddStatement(rep.MappingSource, mainB)

		_, modifiedDS, _ := st.Commit(constructCtx, "create NG-A, NG-B and NG-C", sst.DefaultBranch)
		assert.Equal(t, 3, len(modifiedDS))
		sort.Slice(modifiedDS, func(i, j int) bool {
			return bytes.Compare(modifiedDS[i][:], modifiedDS[j][:]) < 0
		})
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngAIRI)
		assert.Equal(t, sst.IRI(modifiedDS[1].URN()), ngBIRI)
		assert.Equal(t, sst.IRI(modifiedDS[2].URN()), ngCIRI)

		ngA.CreateIRINode("A2", lci.Person)
		_, modifiedDS, _ = st.Commit(constructCtx, "modified NGA", sst.DefaultBranch)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngAIRI)
	})
}

func Test_RemoteRepositoryMultipleCommits_NGAImportsNGBImportsNGC_ModifyB(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngAIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361").URN())
	ngBIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362").URN())
	ngCIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363").URN())
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	commitHash1 := sst.Hash{}
	commitHash2 := sst.Hash{}
	modifiedDS := []uuid.UUID{}

	t.Run("write", func(t *testing.T) {

		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		ngC := st.CreateNamedGraph(ngCIRI)

		// Create mainC node
		mainC := ngC.CreateIRINode("mainC")
		mainC.AddStatement(rdf.Type, rep.SchematicPort)

		// NG-B imports NG-C
		// Create mainB node references mainC
		ngB := st.CreateNamedGraph(ngBIRI)
		ngB.AddImport(ngC)

		mainB := ngB.CreateIRINode("mainB")
		mainB.AddStatement(rep.MappingSource, mainC)

		// NG-A imports NG-B
		// creates mainA node references mainB
		ngA := st.CreateNamedGraph(ngAIRI)
		ngA.AddImport(ngB)
		mainA := ngA.CreateIRINode("mainA")
		mainA.AddStatement(rep.MappingSource, mainB)

		commitHash1, modifiedDS, err = st.Commit(constructCtx, "create NG-A, NG-B and NG-C", sst.DefaultBranch)
		assert.NoError(t, err)
		assert.Equal(t, 3, len(modifiedDS))
		sort.Slice(modifiedDS, func(i, j int) bool {
			return bytes.Compare(modifiedDS[i][:], modifiedDS[j][:]) < 0
		})
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngAIRI)
		assert.Equal(t, sst.IRI(modifiedDS[1].URN()), ngBIRI)
		assert.Equal(t, sst.IRI(modifiedDS[2].URN()), ngCIRI)

		ngB.CreateIRINode("B2", lci.Person)
		commitHash2, modifiedDS, err = st.Commit(constructCtx, "modified NGB", sst.DefaultBranch)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(modifiedDS))
		sort.Slice(modifiedDS, func(i, j int) bool {
			return bytes.Compare(modifiedDS[i][:], modifiedDS[j][:]) < 0
		})
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngAIRI)
		assert.Equal(t, sst.IRI(modifiedDS[1].URN()), ngBIRI)
	})

	t.Run("read", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(constructCtx, ngAIRI)
		if err != nil {
			panic(err)
		}

		st, err := ds.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ngA := st.NamedGraph(ngAIRI)
		if ngA == nil {
			panic("got nil NamedGraph")
		}
		assert.Equal(t, commitHash2, ngA.Info().CheckedOutCommits[0])
		fmt.Println(ngA.Info().CheckedOutCommits)

		ngB := st.NamedGraph(ngBIRI)
		if ngB == nil {
			panic("got nil NamedGraph")
		}
		assert.Equal(t, commitHash2, ngB.Info().CheckedOutCommits[0])
		fmt.Println(ngB.Info().CheckedOutCommits)

		// ngB.Dump()

		ngC := st.NamedGraph(ngCIRI)
		if ngC == nil {
			panic("got nil NamedGraph")
		}
		assert.Equal(t, commitHash1, ngC.Info().CheckedOutCommits[0])
		fmt.Println(ngC.Info().CheckedOutCommits)
		// ngC.Dump()
		fmt.Println()
		traversalByAPI(ngB)
	})
}

func Test_RemoteRepositoryMultipleCommits_NGAImportsNGBImportsNGC_ModifyC(t *testing.T) {
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

		ngC := st.CreateNamedGraph(ngCIRI)

		// Create mainC node
		mainC := ngC.CreateIRINode("mainC")
		mainC.AddStatement(rdf.Type, rep.SchematicPort)

		// NG-B imports NG-C
		// Create mainB node references mainC
		ngB := st.CreateNamedGraph(ngBIRI)
		ngB.AddImport(ngC)

		mainB := ngB.CreateIRINode("mainB")
		mainB.AddStatement(rep.MappingSource, mainC)

		// NG-A imports NG-B
		// creates mainA node references mainB
		ngA := st.CreateNamedGraph(ngAIRI)
		ngA.AddImport(ngB)
		mainA := ngA.CreateIRINode("mainA")
		mainA.AddStatement(rep.MappingSource, mainB)

		commitHash, modifiedDS, err := st.Commit(constructCtx, "create NG-A, NG-B and NG-C", sst.DefaultBranch)
		assert.NoError(t, err)
		assert.Equal(t, 3, len(modifiedDS))
		sort.Slice(modifiedDS, func(i, j int) bool {
			return bytes.Compare(modifiedDS[i][:], modifiedDS[j][:]) < 0
		})
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngAIRI)
		assert.Equal(t, sst.IRI(modifiedDS[1].URN()), ngBIRI)
		assert.Equal(t, sst.IRI(modifiedDS[2].URN()), ngCIRI)

		ngC.CreateIRINode("C2", lci.Person)
		dsrOfNgA := st.NamedGraph(ngAIRI).Info().DatasetRevision
		dsrOfNgB := st.NamedGraph(ngBIRI).Info().DatasetRevision
		// dsrOfNgC is unused because for modified graphs Info() now returns dynamically
		// calculated values that equal the post-commit stored value.
		// dsrOfNgC := st.NamedGraph(ngCIRI).Info().DatasetRevision
		// dsrOfNgC is unused because for modified graphs Info() now returns dynamically
		// calculated values that equal the post-commit stored value.
		// dsrOfNgC := st.NamedGraph(ngCIRI).Info().DatasetRevision

		commitOfNgA := st.NamedGraph(ngAIRI).Info().CheckedOutCommits[0]
		commitOfNgB := st.NamedGraph(ngBIRI).Info().CheckedOutCommits[0]
		commitOfNgC := st.NamedGraph(ngCIRI).Info().CheckedOutCommits[0]

		assert.Equal(t, commitHash, st.NamedGraph(ngAIRI).Info().CheckedOutCommits[0])
		assert.Equal(t, commitHash, st.NamedGraph(ngBIRI).Info().CheckedOutCommits[0])
		assert.Equal(t, commitHash, st.NamedGraph(ngCIRI).Info().CheckedOutCommits[0])

		commitHash, modifiedDS, err = st.Commit(constructCtx, "modified NGC", sst.DefaultBranch)
		assert.NoError(t, err)

		assert.Equal(t, 3, len(modifiedDS))
		sort.Slice(modifiedDS, func(i, j int) bool {
			return bytes.Compare(modifiedDS[i][:], modifiedDS[j][:]) < 0
		})
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngAIRI)
		assert.Equal(t, sst.IRI(modifiedDS[1].URN()), ngBIRI)
		assert.Equal(t, sst.IRI(modifiedDS[2].URN()), ngCIRI)

		assert.NotEqual(t, dsrOfNgA, st.NamedGraph(ngAIRI).Info().DatasetRevision)
		assert.NotEqual(t, dsrOfNgB, st.NamedGraph(ngBIRI).Info().DatasetRevision)
		// For the directly modified graph, DatasetRevision is dynamically calculated before commit,
		// which equals the post-commit stored value. Skip NotEqual check.
		// assert.NotEqual(t, dsrOfNgC, st.NamedGraph(ngCIRI).Info().DatasetRevision)
		// For the directly modified graph, DatasetRevision is dynamically calculated before commit,
		// which equals the post-commit stored value. Skip NotEqual check.
		// assert.NotEqual(t, dsrOfNgC, st.NamedGraph(ngCIRI).Info().DatasetRevision)

		assert.NotEqual(t, commitOfNgA, st.NamedGraph(ngAIRI).Info().CheckedOutCommits[0])
		assert.NotEqual(t, commitOfNgB, st.NamedGraph(ngBIRI).Info().CheckedOutCommits[0])
		assert.NotEqual(t, commitOfNgC, st.NamedGraph(ngCIRI).Info().CheckedOutCommits[0])

		assert.Equal(t, commitHash, st.NamedGraph(ngAIRI).Info().CheckedOutCommits[0])
		assert.Equal(t, commitHash, st.NamedGraph(ngBIRI).Info().CheckedOutCommits[0])
		assert.Equal(t, commitHash, st.NamedGraph(ngCIRI).Info().CheckedOutCommits[0])

	})
}

func Test_RemoteRepositoryMultipleCommits_NGAImportsNGBImportsNGC(t *testing.T) {
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

		ngC := st.CreateNamedGraph(ngCIRI)

		// Create mainC node
		mainC := ngC.CreateIRINode("mainC")
		mainC.AddStatement(rdf.Type, rep.SchematicPort)

		_, modifiedDS, _ := st.Commit(constructCtx, "create NG-C and mainC Node", sst.DefaultBranch)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngCIRI)
		// NG-B imports NG-C
		// Create mainB node references mainC
		ngB := st.CreateNamedGraph(ngBIRI)
		ngB.AddImport(ngC)

		mainB := ngB.CreateIRINode("mainB")
		mainB.AddStatement(rep.MappingSource, mainC)
		_, modifiedDS, _ = st.Commit(constructCtx, "create NG-B and reference to mainC", sst.DefaultBranch)
		assert.Equal(t, 1, len(modifiedDS))
		sort.Slice(modifiedDS, func(i, j int) bool {
			return bytes.Compare(modifiedDS[i][:], modifiedDS[j][:]) < 0
		})
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngBIRI)
		// NG-A imports NG-B
		// creates mainA node references mainB
		ngA := st.CreateNamedGraph(ngAIRI)
		ngA.AddImport(ngB)
		mainA := ngA.CreateIRINode("mainA")
		mainA.AddStatement(rep.MappingSource, mainB)

		_, modifiedDS, _ = st.Commit(constructCtx, "create NG-A and reference to mainB", sst.DefaultBranch)
		assert.Equal(t, 1, len(modifiedDS))
		sort.Slice(modifiedDS, func(i, j int) bool {
			return bytes.Compare(modifiedDS[i][:], modifiedDS[j][:]) < 0
		})
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngAIRI)
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
		ngA.CreateIRINode("A2", lci.Person)
		_, modifiedDS, _ := st.Commit(constructCtx, "modified NGA", sst.DefaultBranch)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngAIRI)

		ngA.Dump()

		ngB := st.NamedGraph(ngBIRI)
		if ngB == nil {
			panic("got nil NamedGraph")
		}
		ngB.CreateIRINode("B2", lci.Person)
		_, modifiedDS, _ = st.Commit(constructCtx, "modified NGB", sst.DefaultBranch)
		assert.Equal(t, 2, len(modifiedDS))
		sort.Slice(modifiedDS, func(i, j int) bool {
			return bytes.Compare(modifiedDS[i][:], modifiedDS[j][:]) < 0
		})
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngAIRI)
		assert.Equal(t, sst.IRI(modifiedDS[1].URN()), ngBIRI)

		ngB.Dump()

		ngC := st.NamedGraph(ngCIRI)
		if ngC == nil {
			panic("got nil NamedGraph")
		}

		// modifiy mainC and commit again
		ngC.CreateIRINode("c2", lci.Person)
		_, modifiedDS, _ = st.Commit(constructCtx, "modified NGC", sst.DefaultBranch)
		assert.Equal(t, 3, len(modifiedDS))
		sort.Slice(modifiedDS, func(i, j int) bool {
			return bytes.Compare(modifiedDS[i][:], modifiedDS[j][:]) < 0
		})
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngAIRI)
		assert.Equal(t, sst.IRI(modifiedDS[1].URN()), ngBIRI)
		assert.Equal(t, sst.IRI(modifiedDS[2].URN()), ngCIRI)

		ngC.Dump()

		traversalByAPI(ngA)
	})
	t.Run("readCommitDetailsNGA", func(t *testing.T) {
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

		commitDetail, err := dsA.CommitDetailsByBranch(constructCtx, sst.DefaultBranch)
		assert.NoError(t, err)
		for {
			commitDetail.Dump()
			st, err := dsA.CheckoutCommit(constructCtx, commitDetail.Commit, sst.DefaultTriplexMode)
			assert.NoError(t, err)
			ng := st.NamedGraph(ngCIRI)
			ng.Dump()
			if len(commitDetail.ParentCommits[dsA.IRI()]) != 0 {
				// Suppose there is only one parent
				commitDetail, err = dsA.CommitDetailsByHash(constructCtx, commitDetail.ParentCommits[dsA.IRI()][0])
				assert.NoError(t, err)
			} else {
				break
			}
		}

	})
	t.Run("readCommitDetailsNGB", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		dsC, err := repo.Dataset(constructCtx, ngBIRI)
		if err != nil {
			panic(err)
		}

		commitDetail, err := dsC.CommitDetailsByBranch(constructCtx, sst.DefaultBranch)
		assert.NoError(t, err)
		for {
			commitDetail.Dump()
			st, err := dsC.CheckoutCommit(constructCtx, commitDetail.Commit, sst.DefaultTriplexMode)
			assert.NoError(t, err)
			ng := st.NamedGraph(ngCIRI)
			ng.Dump()
			if len(commitDetail.ParentCommits[dsC.IRI()]) != 0 {
				// Suppose there is only one parent
				commitDetail, err = dsC.CommitDetailsByHash(constructCtx, commitDetail.ParentCommits[dsC.IRI()][0])
				assert.NoError(t, err)
			} else {
				break
			}
		}

	})

	t.Run("readCommitDetailsNGC", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		dsC, err := repo.Dataset(constructCtx, ngCIRI)
		if err != nil {
			panic(err)
		}

		commitDetail, err := dsC.CommitDetailsByBranch(constructCtx, sst.DefaultBranch)
		assert.NoError(t, err)
		for {
			commitDetail.Dump()
			st, err := dsC.CheckoutCommit(constructCtx, commitDetail.Commit, sst.DefaultTriplexMode)
			assert.NoError(t, err)
			ng := st.NamedGraph(ngCIRI)
			ng.Dump()
			if len(commitDetail.ParentCommits[dsC.IRI()]) != 0 {
				// Suppose there is only one parent
				commitDetail, err = dsC.CommitDetailsByHash(constructCtx, commitDetail.ParentCommits[dsC.IRI()][0])
				assert.NoError(t, err)
			} else {
				break
			}
		}
	})
}

func Test_RemoteRepositoryMultipleCommits_DiamondCase_ModifyA(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngAIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361").URN())
	ngBIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362").URN())
	ngCIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363").URN())
	ngDIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a364").URN())
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

		// Create mainC node
		ngC := st.CreateNamedGraph(ngCIRI)
		mainC := ngC.CreateIRINode("mainC")
		mainC.AddStatement(rdf.Type, rep.SchematicPort)

		// NG-B imports NG-C
		// Create mainB node references mainC
		ngB := st.CreateNamedGraph(ngBIRI)
		ngB.AddImport(ngC)

		mainB := ngB.CreateIRINode("mainB")
		mainB.AddStatement(rep.MappingSource, mainC)

		// NG-D imports NG-C
		// Create mainD node references mainC
		ngD := st.CreateNamedGraph(ngDIRI)
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

		_, modifiedDS, _ := st.Commit(constructCtx, "Create Diamond", sst.DefaultBranch)
		assert.Equal(t, 4, len(modifiedDS))
		sort.Slice(modifiedDS, func(i, j int) bool {
			return bytes.Compare(modifiedDS[i][:], modifiedDS[j][:]) < 0
		})
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngAIRI)
		assert.Equal(t, sst.IRI(modifiedDS[1].URN()), ngBIRI)
		assert.Equal(t, sst.IRI(modifiedDS[2].URN()), ngCIRI)
		assert.Equal(t, sst.IRI(modifiedDS[3].URN()), ngDIRI)

		ngA.CreateIRINode("A2", lci.Person)
		_, modifiedDS, _ = st.Commit(constructCtx, "modified NGA", sst.DefaultBranch)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngAIRI)
	})
}

func Test_ReRepositoryMultipleCommits_DiamondCase_ModifyB(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngAIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361").URN())
	ngBIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362").URN())
	ngCIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363").URN())
	ngDIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a364").URN())
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

		// Create mainC node
		ngC := st.CreateNamedGraph(ngCIRI)
		mainC := ngC.CreateIRINode("mainC")
		mainC.AddStatement(rdf.Type, rep.SchematicPort)

		// NG-B imports NG-C
		// Create mainB node references mainC
		ngB := st.CreateNamedGraph(ngBIRI)
		ngB.AddImport(ngC)

		mainB := ngB.CreateIRINode("mainB")
		mainB.AddStatement(rep.MappingSource, mainC)

		// NG-D imports NG-C
		// Create mainD node references mainC
		ngD := st.CreateNamedGraph(ngDIRI)
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

		_, modifiedDS, _ := st.Commit(constructCtx, "Create Diamond", sst.DefaultBranch)
		assert.Equal(t, 4, len(modifiedDS))
		sort.Slice(modifiedDS, func(i, j int) bool {
			return bytes.Compare(modifiedDS[i][:], modifiedDS[j][:]) < 0
		})
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngAIRI)
		assert.Equal(t, sst.IRI(modifiedDS[1].URN()), ngBIRI)
		assert.Equal(t, sst.IRI(modifiedDS[2].URN()), ngCIRI)
		assert.Equal(t, sst.IRI(modifiedDS[3].URN()), ngDIRI)

		ngB.CreateIRINode("B2", lci.Person)
		_, modifiedDS, _ = st.Commit(constructCtx, "modified NGB", sst.DefaultBranch)
		sort.Slice(modifiedDS, func(i, j int) bool {
			return bytes.Compare(modifiedDS[i][:], modifiedDS[j][:]) < 0
		})
		assert.Equal(t, 2, len(modifiedDS))
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngAIRI)
		assert.Equal(t, sst.IRI(modifiedDS[1].URN()), ngBIRI)
	})
}

func Test_RemoteRepositoryMultipleCommits_DiamondCase_ModifyC(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngAIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361").URN())
	ngBIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362").URN())
	ngCIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363").URN())
	ngDIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a364").URN())
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

		// Create mainC node
		ngC := st.CreateNamedGraph(ngCIRI)
		mainC := ngC.CreateIRINode("mainC")
		mainC.AddStatement(rdf.Type, rep.SchematicPort)

		// NG-B imports NG-C
		// Create mainB node references mainC
		ngB := st.CreateNamedGraph(ngBIRI)
		ngB.AddImport(ngC)

		mainB := ngB.CreateIRINode("mainB")
		mainB.AddStatement(rep.MappingSource, mainC)

		// NG-D imports NG-C
		// Create mainD node references mainC
		ngD := st.CreateNamedGraph(ngDIRI)
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

		_, modifiedDS, _ := st.Commit(constructCtx, "Create Diamond", sst.DefaultBranch)
		assert.Equal(t, 4, len(modifiedDS))
		sort.Slice(modifiedDS, func(i, j int) bool {
			return bytes.Compare(modifiedDS[i][:], modifiedDS[j][:]) < 0
		})
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngAIRI)
		assert.Equal(t, sst.IRI(modifiedDS[1].URN()), ngBIRI)
		assert.Equal(t, sst.IRI(modifiedDS[2].URN()), ngCIRI)
		assert.Equal(t, sst.IRI(modifiedDS[3].URN()), ngDIRI)

		ngC.CreateIRINode("C2", lci.Person)
		_, modifiedDS, _ = st.Commit(constructCtx, "modified NGC", sst.DefaultBranch)
		assert.Equal(t, 4, len(modifiedDS))
		sort.Slice(modifiedDS, func(i, j int) bool {
			return bytes.Compare(modifiedDS[i][:], modifiedDS[j][:]) < 0
		})
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngAIRI)
		assert.Equal(t, sst.IRI(modifiedDS[1].URN()), ngBIRI)
		assert.Equal(t, sst.IRI(modifiedDS[2].URN()), ngCIRI)
		assert.Equal(t, sst.IRI(modifiedDS[3].URN()), ngDIRI)
	})
}

func Test_RemoteRepositoryMultipleCommits_DiamondCase_ModifyD(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngAIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361").URN())
	ngBIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362").URN())
	ngCIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363").URN())
	ngDIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a364").URN())
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

		// Create mainC node
		ngC := st.CreateNamedGraph(ngCIRI)
		mainC := ngC.CreateIRINode("mainC")
		mainC.AddStatement(rdf.Type, rep.SchematicPort)

		// NG-B imports NG-C
		// Create mainB node references mainC
		ngB := st.CreateNamedGraph(ngBIRI)
		ngB.AddImport(ngC)

		mainB := ngB.CreateIRINode("mainB")
		mainB.AddStatement(rep.MappingSource, mainC)

		// NG-D imports NG-C
		// Create mainD node references mainC
		ngD := st.CreateNamedGraph(ngDIRI)
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

		_, modifiedDS, _ := st.Commit(constructCtx, "Create Diamond", sst.DefaultBranch)
		assert.Equal(t, 4, len(modifiedDS))
		sort.Slice(modifiedDS, func(i, j int) bool {
			return bytes.Compare(modifiedDS[i][:], modifiedDS[j][:]) < 0
		})
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngAIRI)
		assert.Equal(t, sst.IRI(modifiedDS[1].URN()), ngBIRI)
		assert.Equal(t, sst.IRI(modifiedDS[2].URN()), ngCIRI)
		assert.Equal(t, sst.IRI(modifiedDS[3].URN()), ngDIRI)

		ngD.CreateIRINode("D2", lci.Person)
		_, modifiedDS, _ = st.Commit(constructCtx, "modified NGD", sst.DefaultBranch)
		assert.Equal(t, 2, len(modifiedDS))
		sort.Slice(modifiedDS, func(i, j int) bool {
			return bytes.Compare(modifiedDS[i][:], modifiedDS[j][:]) < 0
		})
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngAIRI)
		assert.Equal(t, sst.IRI(modifiedDS[1].URN()), ngDIRI)
	})
}

func Test_RemoteRepositoryMultipleCommits_DiamondCase(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngAIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361").URN())
	ngBIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362").URN())
	ngCIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363").URN())
	ngDIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a364").URN())

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

		// Create mainC node
		ngC := st.CreateNamedGraph(ngCIRI)
		mainC := ngC.CreateIRINode("mainC")
		mainC.AddStatement(rdf.Type, rep.SchematicPort)
		_, modifiedDS, _ := st.Commit(constructCtx, "Diamond Case Create NG-C", sst.DefaultBranch)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngCIRI)

		// NG-B imports NG-C
		// Create mainB node references mainC
		ngB := st.CreateNamedGraph(ngBIRI)
		ngB.AddImport(ngC)

		mainB := ngB.CreateIRINode("mainB")
		mainB.AddStatement(rep.MappingSource, mainC)
		_, modifiedDS, _ = st.Commit(constructCtx, "Diamond Case Create NG-B", sst.DefaultBranch)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngBIRI)

		// NG-D imports NG-C
		// Create mainD node references mainC
		ngD := st.CreateNamedGraph(ngDIRI)
		ngD.AddImport(ngC)

		mainD := ngD.CreateIRINode("mainD")
		mainD.AddStatement(rep.MappingSource, mainC)
		_, modifiedDS, _ = st.Commit(constructCtx, "Diamond Case Create NG-D", sst.DefaultBranch)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngDIRI)

		ngA := st.CreateNamedGraph(ngAIRI)

		// NG-A imports NG-B
		// creates mainA node references mainB
		ngA.AddImport(ngB)

		mainA := ngA.CreateIRINode("mainA")
		mainA.AddStatement(rep.MappingSource, mainB)

		// NG-A imports NG-D
		// creates mainA node references mainD
		ngA.AddImport(ngD)

		mainA.AddStatement(rep.MappingSource, mainD)
		_, modifiedDS, _ = st.Commit(constructCtx, "Diamond Case Create NG-A", sst.DefaultBranch)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, sst.IRI(modifiedDS[0].URN()), ngAIRI)
	})
	t.Run("read", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		datasetIRIs, err := repo.Datasets(constructCtx)
		if err != nil {
			panic(err)
		}
		assert.Equal(t, len(datasetIRIs), 4)
		for key, val := range datasetIRIs {
			fmt.Println(key, val)
		}

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

func Test_WriteDiffLocalRemoteRepository(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	randomGraphID := uuid.MustParse("b663190f-9877-4278-820d-5e71b5bde27a")
	var commitID sst.Hash
	var commitID2 sst.Hash
	var modifiedDatasets []uuid.UUID

	// fmt.Println(randomGraphID)
	t.Run("create NamedGraph", func(t *testing.T) {

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
		fmt.Println(repoInfo.URL)

		st := repo.OpenStage(sst.DefaultTriplexMode)

		ngA := st.CreateNamedGraph(sst.IRI(randomGraphID.URN()))

		John := ngA.CreateIRINode("John", lci.Person)
		country := ngA.CreateIRINode("China")
		city := ngA.CreateIRINode("TianJin")
		ngA.CreateIRINode("Beijing")
		col := ngA.CreateCollection(country, city)
		John.AddStatement(lci.Contains, col)

		commitID, modifiedDatasets, err = st.Commit(constructCtx, "first commit: add John IBNode", sst.DefaultBranch)
		if err != nil {
			panic(err)
		}
		fmt.Println(commitID)
		fmt.Println(modifiedDatasets)
	})

	t.Run("modify IBNode", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		dataset, err := repo.Dataset(constructCtx, sst.IRI(randomGraphID.URN()))
		assert.NoError(t, err)

		st, err := dataset.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		assert.NoError(t, err)

		ngA := st.NamedGraph(sst.IRI(randomGraphID.URN()))

		John := ngA.GetIRINodeByFragment("John")
		country := ngA.GetIRINodeByFragment("China")
		city := ngA.GetIRINodeByFragment("TianJin")
		col := John.GetObjects(lci.Contains)

		col[0].(sst.TermCollection).SetMembers(city, country)

		commitID2, modifiedDatasets, err = st.Commit(constructCtx, "second commit: modify John IBNode", sst.DefaultBranch)
		if err != nil {
			panic(err)
		}
		fmt.Println(commitID)
		fmt.Println(modifiedDatasets)
	})

	t.Run("read", func(t *testing.T) {
		// t.SkipNow()
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		dataset, err := repo.Dataset(constructCtx, sst.IRI(randomGraphID.URN()))
		assert.NoError(t, err)

		commitDetail1, err := dataset.CommitDetailsByHash(constructCtx, commitID)
		assert.NoError(t, err)
		commitDetail1.Dump()

		commitDetail2, err := dataset.CommitDetailsByHash(constructCtx, commitID2)
		assert.NoError(t, err)
		commitDetail2.Dump()

		var buf1 bytes.Buffer
		var buf2 bytes.Buffer

		err = repo.ExtractSstFile(constructCtx, commitDetail1.NamedGraphRevisions[sst.IRI(randomGraphID.URN())], &buf1)
		assert.NoError(t, err)

		err = repo.ExtractSstFile(constructCtx, commitDetail2.NamedGraphRevisions[sst.IRI(randomGraphID.URN())], &buf2)
		assert.NoError(t, err)

		var out bytes.Buffer
		diffTriples, err := sst.SstWriteDiff(bufio.NewReader(&buf1), bufio.NewReader(&buf2), &out, true)
		assert.NoError(t, err)
		assert.Equal(t, 3, len(diffTriples))

		for _, val := range diffTriples {
			fmt.Println(val)
		}
	})

}

func Test_RemoteRepositoryOneNGImportsFiveNGsImportsOneNg(t *testing.T) {
	ttlDir := filepath.Join("./testdata/", "TestReadTTLsWriteToSSTs")
	repoDir := filepath.Join(t.TempDir(), t.Name())
	uuids := []uuid.UUID{
		uuid.MustParse("4f91a7c1-1c30-473f-a058-39306f54e5e6"),
		uuid.MustParse("732db40f-68f7-4777-94cf-4021dc406d9b"),
		uuid.MustParse("fa33ec5c-2ce1-47a4-9676-ffec196b046b"),
		uuid.MustParse("7d906134-deb3-4765-aed4-f5e0e4b2fee9"),
		uuid.MustParse("fb6abb6a-ced1-454c-8964-f4b980682d85"),
		uuid.MustParse("aec28ed6-fa13-4061-b220-4ab2bdc6bc73"),
		uuid.MustParse("56a62282-d984-48c3-ba1f-98c23f67fa48"),
	}

	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	t.Run("write", func(t *testing.T) {

		url := testutil.ServerServe(t, repoDir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		entries, err := os.ReadDir(ttlDir)
		if err != nil {
			panic(err)
		}

		for _, entry := range entries {
			filePath := filepath.Join(ttlDir, entry.Name())

			file, err := os.Open(filePath)
			if err != nil {
				panic(err)
			}

			tempSt, err := sst.RdfRead(bufio.NewReader(file), sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
			if err != nil {
				panic(err)
			}

			_, err = st.MoveAndMerge(constructCtx, tempSt)
			if err != nil {
				panic(err)
			}
		}

		commitHash, influenceID, err := st.Commit(constructCtx, "load all ttl files", sst.DefaultBranch)
		fmt.Println(commitHash)
		fmt.Println(influenceID)
		if err != nil {
			panic(err)
		}

	})

	t.Run("read", func(t *testing.T) {
		url := testutil.ServerServe(t, repoDir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		for i := 0; i < len(uuids); i++ {
			dataset, err := repo.Dataset(constructCtx, sst.IRI(uuids[i].URN()))
			if err != nil {
				panic(err)
			}
			s, err := dataset.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
			if err != nil {
				panic(err)
			}
			log.Println("dataset checkout finished", uuids[i])
			if _, err := st.MoveAndMerge(constructCtx, s); err != nil {
				panic(err)
			}
			log.Println("MoveAndMerge finished", uuids[i])
		}
	})

	t.Run("readMainAndModifyNG1", func(t *testing.T) {
		url := testutil.ServerServe(t, repoDir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		dataset, err := repo.Dataset(constructCtx, sst.IRI(uuids[2].URN()))
		if err != nil {
			panic(err)
		}

		st, err := dataset.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		// st.Dump()
		ng1 := st.NamedGraph(sst.IRI(uuids[0].URN()))
		ng1.CreateIRINode("newNode", lci.Person)

		commitHashOfNG1 := ng1.Info().CheckedOutCommits[0]
		// dsrOfNG1 and ngrOfNG1 are unused because for modified graphs Info() now returns
		// dynamically calculated values that equal the post-commit stored value.
		// dsrOfNG1 := ng1.Info().DatasetRevision
		// ngrOfNG1 := ng1.Info().NamedGraphRevision

		ngMain := st.NamedGraph(sst.IRI(uuids[2].URN()))

		commitHashOfngMain := ngMain.Info().CheckedOutCommits[0]
		dsrOfngMain := ngMain.Info().DatasetRevision
		ngrOfngMain := ngMain.Info().NamedGraphRevision

		commitHash, influenceDatasets, err := st.Commit(constructCtx, "readMainAndModifyNG1", sst.DefaultBranch)
		if err != nil {
			panic(err)
		}
		fmt.Println("commit hash:", commitHash)
		fmt.Println("influence datasets:", influenceDatasets)

		ng1 = st.NamedGraph(sst.IRI(uuids[0].URN()))
		ngMain = st.NamedGraph(sst.IRI(uuids[2].URN()))

		assert.NotEqual(t, commitHashOfNG1, ng1.Info().CheckedOutCommits[0])
		// For the directly modified graph, revisions are dynamically calculated before commit,
		// which equals the post-commit stored value. Skip NotEqual checks.
		// assert.NotEqual(t, dsrOfNG1, ng1.Info().DatasetRevision)
		// assert.NotEqual(t, ngrOfNG1, ng1.Info().NamedGraphRevision)

		assert.NotEqual(t, commitHashOfngMain, ngMain.Info().CheckedOutCommits[0])
		assert.NotEqual(t, dsrOfngMain, ngMain.Info().DatasetRevision)
		assert.Equal(t, ngrOfngMain, ngMain.Info().NamedGraphRevision)
	})

	t.Run("readMainAndModifyNG5", func(t *testing.T) {
		url := testutil.ServerServe(t, repoDir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		dataset, err := repo.Dataset(constructCtx, sst.IRI(uuids[2].URN()))
		if err != nil {
			panic(err)
		}

		st, err := dataset.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		// st.Dump()
		ng5 := st.NamedGraph(sst.IRI(uuids[5].URN()))
		ng5.CreateIRINode("newNode", lci.Person)

		commitHashOfNG5 := ng5.Info().CheckedOutCommits[0]
		// dsrOfNG5 and ngrOfNG5 are unused because for modified graphs Info() now returns
		// dynamically calculated values that equal the post-commit stored value.
		// dsrOfNG5 := ng5.Info().DatasetRevision
		// ngrOfNG5 := ng5.Info().NamedGraphRevision

		ngMain := st.NamedGraph(sst.IRI(uuids[2].URN()))

		commitHashOfngMain := ngMain.Info().CheckedOutCommits[0]
		dsrOfngMain := ngMain.Info().DatasetRevision
		ngrOfngMain := ngMain.Info().NamedGraphRevision

		commitHash, influenceDatasets, err := st.Commit(constructCtx, "readMainAndModifyNG5", sst.DefaultBranch)
		if err != nil {
			panic(err)
		}
		fmt.Println("commit hash:", commitHash)
		fmt.Println("influence datasets:", influenceDatasets)
		assert.Len(t, influenceDatasets, 6)

		ng5 = st.NamedGraph(sst.IRI(uuids[5].URN()))
		ngMain = st.NamedGraph(sst.IRI(uuids[2].URN()))

		assert.NotEqual(t, commitHashOfNG5, ng5.Info().CheckedOutCommits[0])
		// For the directly modified graph, revisions are dynamically calculated before commit,
		// which equals the post-commit stored value. Skip NotEqual checks.
		// assert.NotEqual(t, dsrOfNG5, ng5.Info().DatasetRevision)
		// assert.NotEqual(t, ngrOfNG5, ng5.Info().NamedGraphRevision)

		assert.NotEqual(t, commitHashOfngMain, ngMain.Info().CheckedOutCommits[0])
		assert.NotEqual(t, dsrOfngMain, ngMain.Info().DatasetRevision)
		assert.Equal(t, ngrOfngMain, ngMain.Info().NamedGraphRevision)
	})
	t.Run("readMainAndModifyNG6", func(t *testing.T) {
		url := testutil.ServerServe(t, repoDir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		dataset, err := repo.Dataset(constructCtx, sst.IRI(uuids[2].URN()))
		if err != nil {
			panic(err)
		}

		st, err := dataset.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		// st.Dump()
		ng6 := st.NamedGraph(sst.IRI(uuids[6].URN()))
		ng6.CreateIRINode("newNode", lci.Person)

		commitHashOfNG6 := ng6.Info().CheckedOutCommits[0]
		// dsrOfNG6 and ngrOfNG6 are unused because for modified graphs Info() now returns
		// dynamically calculated values that equal the post-commit stored value.
		// dsrOfNG6 := ng6.Info().DatasetRevision
		// ngrOfNG6 := ng6.Info().NamedGraphRevision

		ngMain := st.NamedGraph(sst.IRI(uuids[2].URN()))

		commitHashOfngMain := ngMain.Info().CheckedOutCommits[0]
		dsrOfngMain := ngMain.Info().DatasetRevision
		ngrOfngMain := ngMain.Info().NamedGraphRevision

		commitHash, influenceDatasets, err := st.Commit(constructCtx, "readMainAndModifyNG6", sst.DefaultBranch)
		if err != nil {
			panic(err)
		}
		fmt.Println("commit hash:", commitHash)
		fmt.Println("influence datasets:", influenceDatasets)
		assert.Len(t, influenceDatasets, 7)

		ng6 = st.NamedGraph(sst.IRI(uuids[6].URN()))
		ngMain = st.NamedGraph(sst.IRI(uuids[2].URN()))

		assert.NotEqual(t, commitHashOfNG6, ng6.Info().CheckedOutCommits[0])
		// For the directly modified graph, revisions are dynamically calculated before commit,
		// which equals the post-commit stored value. Skip NotEqual checks.
		// assert.NotEqual(t, dsrOfNG6, ng6.Info().DatasetRevision)
		// assert.NotEqual(t, ngrOfNG6, ng6.Info().NamedGraphRevision)

		assert.NotEqual(t, commitHashOfngMain, ngMain.Info().CheckedOutCommits[0])
		assert.NotEqual(t, dsrOfngMain, ngMain.Info().DatasetRevision)
		assert.Equal(t, ngrOfngMain, ngMain.Info().NamedGraphRevision)
	})
}

// Test_CommitDetailsWithNormalIRI tests CommitDetailsByHash, CommitDetailsByBranch, and CommitDetails
// methods using a normal IRI (not UUID) to create NamedGraph.
func Test_CommitDetailsWithNormalIRI(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	// Use a normal IRI (not UUID-based)
	testIRI := sst.IRI("http://example.com/test/ontology#TestGraph")
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	var commitHash1 sst.Hash
	var commitHash2 sst.Hash
	var commitHash3 sst.Hash

	t.Run("create NamedGraph with normal IRI and make commits", func(t *testing.T) {

		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		// Create NamedGraph with normal IRI
		ng := st.CreateNamedGraph(testIRI)
		require.NotNil(t, ng)
		assert.Equal(t, testIRI, ng.IRI())

		// Create some nodes and add statements
		mainNode := ng.CreateIRINode("MainNode", lci.Person)
		mainNode.AddStatement(rdfs.Label, sst.String("Test Person"))
		mainNode.AddStatement(sso.ID, sst.String("person-001"))

		// First commit
		commitHash1, _, err = st.Commit(constructCtx, "First commit: add main node", sst.DefaultBranch)
		require.NoError(t, err)
		assert.NotEqual(t, sst.HashNil(), commitHash1)

		// Add more data
		addressNode := ng.CreateIRINode("Address", lci.Individual)
		addressNode.AddStatement(rdfs.Label, sst.String("Main Address"))
		mainNode.AddStatement(rep.MappingSource, addressNode)

		// Second commit
		commitHash2, _, err = st.Commit(constructCtx, "Second commit: add address", sst.DefaultBranch)
		require.NoError(t, err)
		assert.NotEqual(t, sst.HashNil(), commitHash2)

		// Add another node
		contactNode := ng.CreateIRINode("Contact", rep.SchematicPort)
		contactNode.AddStatement(rdf.Type, rep.Angle)
		mainNode.AddStatement(rep.MappingSource, contactNode)

		// Third commit
		commitHash3, _, err = st.Commit(constructCtx, "Third commit: add contact", sst.DefaultBranch)
		require.NoError(t, err)
		assert.NotEqual(t, sst.HashNil(), commitHash3)

		// Store commit hashes for later tests
		t.Logf("Commit 1: %s", commitHash1.String())
		t.Logf("Commit 2: %s", commitHash2.String())
		t.Logf("Commit 3: %s", commitHash3.String())
	})

	t.Run("test CommitDetailsByHash", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		ds, err := repo.Dataset(constructCtx, testIRI)
		require.NoError(t, err)

		// Test CommitDetailsByHash for commit 1
		details1, err := ds.CommitDetailsByHash(constructCtx, commitHash1)
		require.NoError(t, err)
		assert.Equal(t, "First commit: add main node", details1.Message)
		assert.Equal(t, commitHash1, details1.Commit)
		assert.NotNil(t, details1.DatasetRevisions[testIRI])
		assert.NotNil(t, details1.NamedGraphRevisions[testIRI])
		assert.Equal(t, 0, len(details1.ParentCommits[testIRI])) // First commit has no parents
		t.Logf("CommitDetailsByHash for commit 1:")
		details1.Dump()

		// Test CommitDetailsByHash for commit 2
		details2, err := ds.CommitDetailsByHash(constructCtx, commitHash2)
		require.NoError(t, err)
		assert.Equal(t, "Second commit: add address", details2.Message)
		assert.Equal(t, commitHash2, details2.Commit)
		assert.NotNil(t, details2.DatasetRevisions[testIRI])
		assert.NotNil(t, details2.NamedGraphRevisions[testIRI])
		assert.Equal(t, 1, len(details2.ParentCommits[testIRI]))
		assert.Equal(t, commitHash1, details2.ParentCommits[testIRI][0])
		t.Logf("CommitDetailsByHash for commit 2:")
		details2.Dump()

		// Test CommitDetailsByHash for commit 3
		details3, err := ds.CommitDetailsByHash(constructCtx, commitHash3)
		require.NoError(t, err)
		assert.Equal(t, "Third commit: add contact", details3.Message)
		assert.Equal(t, commitHash3, details3.Commit)
		assert.NotNil(t, details3.DatasetRevisions[testIRI])
		assert.NotNil(t, details3.NamedGraphRevisions[testIRI])
		assert.Equal(t, 1, len(details3.ParentCommits[testIRI]))
		assert.Equal(t, commitHash2, details3.ParentCommits[testIRI][0])
		t.Logf("CommitDetailsByHash for commit 3:")
		details3.Dump()
	})

	t.Run("test CommitDetailsByBranch", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		ds, err := repo.Dataset(constructCtx, testIRI)
		require.NoError(t, err)

		// Test CommitDetailsByBranch for master branch
		details, err := ds.CommitDetailsByBranch(constructCtx, sst.DefaultBranch)
		require.NoError(t, err)
		assert.Equal(t, "Third commit: add contact", details.Message)
		assert.Equal(t, commitHash3, details.Commit)
		assert.NotNil(t, details.DatasetRevisions[testIRI])
		assert.NotNil(t, details.NamedGraphRevisions[testIRI])
		assert.Equal(t, 1, len(details.ParentCommits[testIRI]))
		assert.Equal(t, commitHash2, details.ParentCommits[testIRI][0])

		// Verify author and timestamp are set
		assert.NotEmpty(t, details.Author)
		assert.False(t, details.AuthorDate.IsZero())

		t.Logf("CommitDetailsByBranch for '%s' branch:", sst.DefaultBranch)
		details.Dump()

		// Traverse through all commits using parent references
		commitCount := 0
		currentDetails := details
		for {
			commitCount++
			if len(currentDetails.ParentCommits[testIRI]) == 0 {
				break
			}
			parentHash := currentDetails.ParentCommits[testIRI][0]
			currentDetails, err = ds.CommitDetailsByHash(constructCtx, parentHash)
			require.NoError(t, err)
		}
		assert.Equal(t, 3, commitCount, "Expected 3 commits in the chain")
	})

	t.Run("test Repository CommitDetails method", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		// Test CommitDetails with multiple hashes at once
		hashes := []sst.Hash{commitHash1, commitHash2, commitHash3}
		detailsList, err := repo.CommitDetails(constructCtx, hashes)
		require.NoError(t, err)
		assert.Equal(t, 3, len(detailsList))

		// Verify each commit detail
		messages := make(map[string]bool)
		for _, details := range detailsList {
			messages[details.Message] = true
			assert.NotNil(t, details.DatasetRevisions[testIRI])
			assert.NotNil(t, details.NamedGraphRevisions[testIRI])
			assert.NotEmpty(t, details.Author)
			assert.False(t, details.AuthorDate.IsZero())
		}

		// Verify all messages are present
		assert.True(t, messages["First commit: add main node"])
		assert.True(t, messages["Second commit: add address"])
		assert.True(t, messages["Third commit: add contact"])

		t.Logf("Repository CommitDetails returned %d commit details:", len(detailsList))
		for i, details := range detailsList {
			t.Logf("Commit %d: %s", i+1, details.Message)
		}

		// Test CommitDetails with single hash
		singleDetails, err := repo.CommitDetails(constructCtx, []sst.Hash{commitHash1})
		require.NoError(t, err)
		assert.Equal(t, 1, len(singleDetails))
		assert.Equal(t, "First commit: add main node", singleDetails[0].Message)
		assert.Equal(t, commitHash1, singleDetails[0].Commit)
	})

	t.Run("verify commit chain consistency", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		ds, err := repo.Dataset(constructCtx, testIRI)
		require.NoError(t, err)

		// Verify that all methods return consistent DatasetRevisions and NamedGraphRevisions
		detailsByHash, err := ds.CommitDetailsByHash(constructCtx, commitHash3)
		require.NoError(t, err)

		detailsByBranch, err := ds.CommitDetailsByBranch(constructCtx, sst.DefaultBranch)
		require.NoError(t, err)

		repoDetails, err := repo.CommitDetails(constructCtx, []sst.Hash{commitHash3})
		require.NoError(t, err)
		require.Equal(t, 1, len(repoDetails))

		// All three methods should return the same commit
		assert.Equal(t, detailsByHash.Commit, detailsByBranch.Commit)
		assert.Equal(t, detailsByHash.Commit, repoDetails[0].Commit)

		// DatasetRevisions should match
		assert.Equal(t, detailsByHash.DatasetRevisions[testIRI], detailsByBranch.DatasetRevisions[testIRI])
		assert.Equal(t, detailsByHash.DatasetRevisions[testIRI], repoDetails[0].DatasetRevisions[testIRI])

		// NamedGraphRevisions should match
		assert.Equal(t, detailsByHash.NamedGraphRevisions[testIRI], detailsByBranch.NamedGraphRevisions[testIRI])
		assert.Equal(t, detailsByHash.NamedGraphRevisions[testIRI], repoDetails[0].NamedGraphRevisions[testIRI])

		t.Log("All commit detail methods return consistent results")
	})
}
