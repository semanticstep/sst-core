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

	"github.com/google/uuid"
	"github.com/semanticstep/sst-core/sst"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/semanticstep/sst-core/vocabularies/sso"
	"github.com/stretchr/testify/assert"
)

func Test_LocalFullRepository_MultipleParents(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngAIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361").URN())

	iriNodeUUID1 := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a001")
	iriNodeUUID2 := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a002")
	iriNodeUUID3 := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a003")

	var commitHash1 sst.Hash
	var commitHash2 sst.Hash
	var commitHash3 sst.Hash
	var commitHash4 sst.Hash

	var modifiedDS []uuid.UUID

	t.Run("write", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		ng := st.CreateNamedGraph(ngAIRI)
		mainC := ng.CreateIRINode(iriNodeUUID1.String(), sso.ClassSystem)
		mainC.AddStatement(rdfs.Label, sst.String("CL-test"))
		commitHash1, modifiedDS, _ = st.Commit(context.TODO(), "commit 1", sst.DefaultBranch)
		assert.Equal(t, 1, len(modifiedDS))
		ds, err := repo.Dataset(context.TODO(), ngAIRI)
		assert.NoError(t, err)
		ds.SetBranchCommit(context.TODO(), commitHash1, "anotherBranch")
		// ng.PrintTriples()
		fmt.Println(commitHash1)
	})

	t.Run("checkout default branch and commit", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngAIRI)
		if err != nil {
			panic(err)
		}
		st, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ng := st.NamedGraph(ngAIRI)
		iriNode2 := ng.CreateIRINode(iriNodeUUID2.String(), lci.ClassOfIndividual)
		iriNode2.AddStatement(rdf.Type, ng.GetIRINodeByFragment(iriNodeUUID1.String()))
		iriNode2.AddStatement(sso.ID, sst.String("class 1"))
		commitHash2, modifiedDS, err = st.Commit(context.TODO(), "commit 2a", sst.DefaultBranch)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, ngAIRI, sst.IRI(modifiedDS[0].URN()))
		// ng.PrintTriples()
		fmt.Println(commitHash2)
	})

	t.Run("checkout another branch and commit", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngAIRI)
		if err != nil {
			panic(err)
		}
		st, err := ds.CheckoutBranch(context.TODO(), "anotherBranch", sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ng := st.NamedGraph(ngAIRI)
		iriNode3 := ng.CreateIRINode(iriNodeUUID3.String(), lci.ClassOfIndividual)
		iriNode3.AddStatement(rdf.Type, ng.GetIRINodeByFragment(iriNodeUUID1.String()))
		iriNode3.AddStatement(sso.ID, sst.String("class 2"))
		commitHash2, modifiedDS, err = st.Commit(context.TODO(), "commit 2b", "anotherBranch")
		assert.NoError(t, err)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, ngAIRI, sst.IRI(modifiedDS[0].URN()))
		// ng.PrintTriples()
		fmt.Println(commitHash2)
	})

	t.Run("checkout commit 2a and 2b", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngAIRI)
		if err != nil {
			panic(err)
		}

		st2a, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		st2b, err := ds.CheckoutBranch(context.TODO(), "anotherBranch", sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		// ng := st2a.NamedGraph(ngAIRI)
		// ng.PrintTriples()

		// ng2 := st2b.NamedGraph(ngAIRI)
		// ng2.PrintTriples()

		_, err = st2a.MoveAndMerge(context.TODO(), st2b)
		if err != nil {
			panic(err)
		}
		Hash2Aand2B, influencedDataset, err := st2a.Commit(context.TODO(), "2a and 2b", sst.DefaultBranch)
		assert.NoError(t, err)
		fmt.Println(Hash2Aand2B)
		fmt.Println(influencedDataset)

		commitDetails2a2b, err := ds.CommitDetailsByHash(context.TODO(), Hash2Aand2B)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(commitDetails2a2b.ParentCommits[ngAIRI]))
		commitDetails2a2b.Dump()

		ng := st2a.NamedGraph(ngAIRI)
		assert.Equal(t, 4, ng.IRINodeCount())

		iriNodeUUID1 := ng.GetIRINodeByFragment(iriNodeUUID1.String())
		fmt.Println("triple count:", iriNodeUUID1.TripleCount())
		fmt.Println("-------------------------------------")
		ng.PrintTriples()
		fmt.Println("-------------------------------------")
	})

	t.Run("CommitDetailsByHash IsParentRevision", func(t *testing.T) {
		t.SkipNow()
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngAIRI)
		if err != nil {
			panic(err)
		}
		branchesMap, err := ds.Branches(context.TODO())
		if err != nil {
			panic(err)
		}
		assert.Equal(t, len(branchesMap), 2)
		assert.Equal(t, branchesMap["master"], commitHash3)
		assert.Equal(t, branchesMap["commit1"], commitHash4)

		commitDetails3, _ := ds.CommitDetailsByHash(context.TODO(), commitHash3)
		assert.Equal(t, "Third commit of C", commitDetails3.Message)
		assert.NotNil(t, commitDetails3.DatasetRevisions[ngAIRI])
		commitDetails3.Dump()

		commitDetails2, _ := ds.CommitDetailsByHash(context.TODO(), commitDetails3.ParentCommits[ds.IRI()][0])
		assert.Equal(t, "Second commit of C", commitDetails2.Message)
		assert.NotNil(t, commitDetails2.DatasetRevisions[ngAIRI])
		commitDetails2.Dump()

		commitDetails1, _ := ds.CommitDetailsByHash(context.TODO(), commitDetails2.ParentCommits[ds.IRI()][0])
		assert.Equal(t, "First commit of C", commitDetails1.Message)
		assert.NotNil(t, commitDetails3.DatasetRevisions[ngAIRI])
		commitDetails1.Dump()
		assert.Equal(t, 0, len(commitDetails1.ParentCommits[ds.IRI()]))

		common, err := ds.FindCommonParentRevision(context.TODO(), commitDetails2.Commit, commitDetails3.Commit)
		assert.NoError(t, err)
		assert.Equal(t, common, commitDetails2.Commit)

		common2, err := ds.FindCommonParentRevision(context.TODO(), commitDetails1.Commit, commitDetails3.Commit)
		assert.NoError(t, err)
		assert.Equal(t, common2, commitDetails1.Commit)

		common3, err := ds.FindCommonParentRevision(context.TODO(), commitDetails1.Commit, commitDetails2.Commit)
		assert.NoError(t, err)
		assert.Equal(t, common3, commitDetails1.Commit)
	})

	t.Run("readCommitDetails", func(t *testing.T) {
		t.SkipNow()
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		dsC, err := repo.Dataset(context.TODO(), ngAIRI)
		if err != nil {
			panic(err)
		}

		commitDetail, err := dsC.CommitDetailsByBranch(context.TODO(), sst.DefaultBranch)
		assert.NoError(t, err)
		for {
			commitDetail.Dump()
			st, err := dsC.CheckoutCommit(context.TODO(), commitDetail.Commit, sst.DefaultTriplexMode)
			assert.NoError(t, err)
			ng := st.NamedGraph(ngAIRI)
			ng.Dump()
			if len(commitDetail.ParentCommits[dsC.IRI()]) != 0 {
				// Suppose there is only one parent
				commitDetail, err = dsC.CommitDetailsByHash(context.TODO(), commitDetail.ParentCommits[dsC.IRI()][0])
				assert.NoError(t, err)
			} else {
				break
			}
		}

	})
}

func Test_LocalFullRepositoryMultipleCommits_IsParentRevision(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngCIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363").URN())

	var commitHash1 sst.Hash
	var commitHash3 sst.Hash
	var commitHash4 sst.Hash

	var modifiedDS []uuid.UUID

	t.Run("write", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		stageC := repo.OpenStage(sst.DefaultTriplexMode)

		ng := stageC.CreateNamedGraph(ngCIRI)

		mainC := ng.CreateIRINode("mainC")

		mainC.AddStatement(rdf.Type, rep.SchematicPort)
		commitHash1, modifiedDS, _ = stageC.Commit(context.TODO(), "First commit of C", sst.DefaultBranch)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, ngCIRI, sst.IRI(modifiedDS[0].URN()))

		ds, err := repo.Dataset(context.TODO(), ngCIRI)
		if err != nil {
			panic(err)
		}
		err = ds.SetBranchCommit(context.TODO(), commitHash1, "commit1")
		if err != nil {
			panic(err)
		}

		mainC.AddStatement(rdf.Bag, rep.Angle)
		commitHash2, modifiedDS, err := stageC.Commit(context.TODO(), "Second commit of C", sst.DefaultBranch)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, ngCIRI, sst.IRI(modifiedDS[0].URN()))

		mainC.AddStatement(rdf.Direction, rep.Blue)
		commitHash3, modifiedDS, err = stageC.Commit(context.TODO(), "Third commit of C", sst.DefaultBranch)
		assert.Equal(t, 1, len(modifiedDS))
		assert.NoError(t, err)

		common, err := ds.FindCommonParentRevision(context.TODO(), commitHash1, commitHash3)
		if err != nil {
			panic(err)
		}
		assert.Equal(t, common, commitHash1)

		common2, err := ds.FindCommonParentRevision(context.TODO(), commitHash2, commitHash3)
		if err != nil {
			panic(err)
		}
		assert.Equal(t, common2, commitHash2)

		_, err = ds.FindCommonParentRevision(context.TODO(), commitHash3, commitHash3)
		assert.Error(t, err, sst.ErrSameRevisions)
	})

	t.Run("checkoutByBranch IsParentRevision", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngCIRI)
		if err != nil {
			panic(err)
		}
		st1, err := ds.CheckoutBranch(context.TODO(), "commit1", sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}
		st1.NamedGraphs()[0].CreateIRINode("", lci.Person)
		commitHash4, modifiedDS, err = st1.Commit(context.TODO(), "fourth commit of C", "commit1")
		assert.NoError(t, err)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, ngCIRI, sst.IRI(modifiedDS[0].URN()))

		stMaster, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		common1, err := ds.FindCommonParentRevision(context.TODO(), commitHash4, stMaster.NamedGraphs()[0].Info().CheckedOutCommits[0])
		if err != nil {
			panic(err)
		}
		assert.Equal(t, common1, commitHash1)
	})

	t.Run("CommitDetailsByHash IsParentRevision", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngCIRI)
		if err != nil {
			panic(err)
		}
		branchesMap, err := ds.Branches(context.TODO())
		if err != nil {
			panic(err)
		}
		assert.Equal(t, len(branchesMap), 2)
		assert.Equal(t, branchesMap["master"], commitHash3)
		assert.Equal(t, branchesMap["commit1"], commitHash4)

		commitDetails3, _ := ds.CommitDetailsByHash(context.TODO(), commitHash3)
		assert.Equal(t, "Third commit of C", commitDetails3.Message)
		assert.NotNil(t, commitDetails3.DatasetRevisions[ngCIRI])
		commitDetails3.Dump()

		commitDetails2, _ := ds.CommitDetailsByHash(context.TODO(), commitDetails3.ParentCommits[ds.IRI()][0])
		assert.Equal(t, "Second commit of C", commitDetails2.Message)
		assert.NotNil(t, commitDetails2.DatasetRevisions[ngCIRI])
		commitDetails2.Dump()

		commitDetails1, _ := ds.CommitDetailsByHash(context.TODO(), commitDetails2.ParentCommits[ds.IRI()][0])
		assert.Equal(t, "First commit of C", commitDetails1.Message)
		assert.NotNil(t, commitDetails3.DatasetRevisions[ngCIRI])
		commitDetails1.Dump()
		assert.Equal(t, 0, len(commitDetails1.ParentCommits[ds.IRI()]))

		common, err := ds.FindCommonParentRevision(context.TODO(), commitDetails2.Commit, commitDetails3.Commit)
		assert.NoError(t, err)
		assert.Equal(t, common, commitDetails2.Commit)

		common2, err := ds.FindCommonParentRevision(context.TODO(), commitDetails1.Commit, commitDetails3.Commit)
		assert.NoError(t, err)
		assert.Equal(t, common2, commitDetails1.Commit)

		common3, err := ds.FindCommonParentRevision(context.TODO(), commitDetails1.Commit, commitDetails2.Commit)
		assert.NoError(t, err)
		assert.Equal(t, common3, commitDetails1.Commit)
	})

	t.Run("readCommitDetails", func(t *testing.T) {
		t.SkipNow()
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		dsC, err := repo.Dataset(context.TODO(), ngCIRI)
		if err != nil {
			panic(err)
		}

		commitDetail, err := dsC.CommitDetailsByBranch(context.TODO(), sst.DefaultBranch)
		assert.NoError(t, err)
		for {
			commitDetail.Dump()
			st, err := dsC.CheckoutCommit(context.TODO(), commitDetail.Commit, sst.DefaultTriplexMode)
			assert.NoError(t, err)
			ng := st.NamedGraph(ngCIRI)
			ng.Dump()
			if len(commitDetail.ParentCommits[dsC.IRI()]) != 0 {
				// Suppose there is only one parent
				commitDetail, err = dsC.CommitDetailsByHash(context.TODO(), commitDetail.ParentCommits[dsC.IRI()][0])
				assert.NoError(t, err)
			} else {
				break
			}
		}

	})
}

func Test_LocalFullRepositoryMultipleCommits_UUIDNamedGraph(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngCIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363").URN())

	var commitHash1 sst.Hash
	var commitHash3 sst.Hash
	var modifiedDS []uuid.UUID

	t.Run("write", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		stageC := repo.OpenStage(sst.DefaultTriplexMode)

		ng := stageC.CreateNamedGraph(ngCIRI)

		mainC := ng.CreateIRINode("mainC")

		mainC.AddStatement(rdf.Type, rep.SchematicPort)
		commitHash1, modifiedDS, _ = stageC.Commit(context.TODO(), "First commit of C", sst.DefaultBranch)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, ngCIRI, sst.IRI(modifiedDS[0].URN()))

		ds, err := repo.Dataset(context.TODO(), ngCIRI)
		if err != nil {
			panic(err)
		}
		err = ds.SetBranchCommit(context.TODO(), commitHash1, "commit1")
		if err != nil {
			panic(err)
		}

		mainC.AddStatement(rdf.Bag, rep.Angle)
		_, modifiedDS, _ = stageC.Commit(context.TODO(), "Second commit of C", sst.DefaultBranch)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, ngCIRI, sst.IRI(modifiedDS[0].URN()))

		mainC.AddStatement(rdf.Direction, rep.Blue)
		commitHash3, _, _ = stageC.Commit(context.TODO(), "Third commit of C", sst.DefaultBranch)

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
		branchesMap, err := ds.Branches(context.TODO())
		if err != nil {
			panic(err)
		}
		assert.Equal(t, len(branchesMap), 2)
		assert.Equal(t, branchesMap["master"], commitHash3)
		assert.Equal(t, branchesMap["commit1"], commitHash1)

		commitDetails3, _ := ds.CommitDetailsByHash(context.TODO(), commitHash3)
		assert.Equal(t, "Third commit of C", commitDetails3.Message)
		assert.NotNil(t, commitDetails3.DatasetRevisions[ngCIRI])

		commitDetails3.Dump()

		commitDetails2, _ := ds.CommitDetailsByHash(context.TODO(), commitDetails3.ParentCommits[ds.IRI()][0])
		assert.Equal(t, "Second commit of C", commitDetails2.Message)
		assert.NotNil(t, commitDetails2.DatasetRevisions[ngCIRI])
		commitDetails2.Dump()

		commitDetails1, _ := ds.CommitDetailsByHash(context.TODO(), commitDetails2.ParentCommits[ds.IRI()][0])
		assert.Equal(t, "First commit of C", commitDetails1.Message)
		assert.NotNil(t, commitDetails3.DatasetRevisions[ngCIRI])
		commitDetails1.Dump()
		assert.Equal(t, 0, len(commitDetails1.ParentCommits[ds.IRI()]))
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
		commitDetails3, err := ds.CommitDetailsByBranch(context.TODO(), sst.DefaultBranch)
		assert.NoError(t, err)
		commitDetails3.Dump()

		assert.Equal(t, "Third commit of C", commitDetails3.Message)
		assert.NotNil(t, commitDetails3.DatasetRevisions[sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363").URN())])
	})

	t.Run("read", func(t *testing.T) {
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
	t.Run("readCommitDetails", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		dsC, err := repo.Dataset(context.TODO(), ngCIRI)
		if err != nil {
			panic(err)
		}

		commitDetail, err := dsC.CommitDetailsByBranch(context.TODO(), sst.DefaultBranch)
		assert.NoError(t, err)
		for {
			commitDetail.Dump()
			st, err := dsC.CheckoutCommit(context.TODO(), commitDetail.Commit, sst.DefaultTriplexMode)
			assert.NoError(t, err)
			ng := st.NamedGraph(ngCIRI)
			ng.Dump()
			if len(commitDetail.ParentCommits[dsC.IRI()]) != 0 {
				// Suppose there is only one parent
				commitDetail, err = dsC.CommitDetailsByHash(context.TODO(), commitDetail.ParentCommits[dsC.IRI()][0])
				assert.NoError(t, err)
			} else {
				break
			}
		}

	})
}

func Test_LocalFullRepositoryMultipleCommits_UUIDNamedGraph_CommitToUserDefinedBranch(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngCIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363").URN())

	var commitHash1 sst.Hash
	var commitHash2 sst.Hash
	var commitHash3 sst.Hash
	var modifiedDS []uuid.UUID

	t.Run("write", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		stageC := repo.OpenStage(sst.DefaultTriplexMode)

		ng := stageC.CreateNamedGraph(ngCIRI)

		mainC := ng.CreateIRINode("mainC")

		mainC.AddStatement(rdf.Type, rep.SchematicPort)
		commitHash1, modifiedDS, err = stageC.Commit(context.TODO(), "First commit of C", "commit1")
		assert.NoError(t, err)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, ngCIRI, sst.IRI(modifiedDS[0].URN()))

		mainC.AddStatement(rdf.Bag, rep.Angle)
		commitHash2, modifiedDS, err = stageC.Commit(context.TODO(), "Second commit of C", "commit1")
		assert.NoError(t, err)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, ngCIRI, sst.IRI(modifiedDS[0].URN()))

		mainC.AddStatement(rdf.Direction, rep.Blue)
		commitHash3, _, _ = stageC.Commit(context.TODO(), "Third commit of C", sst.DefaultBranch)

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
		branchesMap, err := ds.Branches(context.TODO())
		if err != nil {
			panic(err)
		}
		assert.Equal(t, len(branchesMap), 2)
		assert.Equal(t, branchesMap["master"], commitHash3)
		assert.Equal(t, branchesMap["commit1"], commitHash2)

		commitDetails3, _ := ds.CommitDetailsByHash(context.TODO(), commitHash3)
		assert.Equal(t, "Third commit of C", commitDetails3.Message)
		assert.NotNil(t, commitDetails3.DatasetRevisions[ngCIRI])
		// assert.Equal(t, 0, len(commitDetails3.ParentCommits[ds.IRI()]))
		commitDetails3.Dump()

		commitDetails2, _ := ds.CommitDetailsByBranch(context.TODO(), "commit1")
		assert.Equal(t, "Second commit of C", commitDetails2.Message)
		assert.NotNil(t, commitDetails2.DatasetRevisions[ngCIRI])
		commitDetails2.Dump()

		commitDetails1, _ := ds.CommitDetailsByHash(context.TODO(), commitDetails2.ParentCommits[ds.IRI()][0])
		assert.Equal(t, "First commit of C", commitDetails1.Message)
		assert.NotNil(t, commitDetails1.DatasetRevisions[ngCIRI])
		commitDetails1.Dump()
		assert.Equal(t, 0, len(commitDetails1.ParentCommits[ds.IRI()]))

		commitDetails1ByHash, _ := ds.CommitDetailsByHash(context.TODO(), commitHash1)
		assert.Equal(t, commitDetails1ByHash.Message, commitDetails1.Message)
		assert.NotNil(t, commitDetails1ByHash.DatasetRevisions[ngCIRI])
		assert.Equal(t, 0, len(commitDetails1ByHash.ParentCommits[ds.IRI()]))
		commitDetails1ByHash.Dump()

	})

	t.Run("readDefaultBranch", func(t *testing.T) {
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

		mainC := ng.GetIRINodeByFragment("mainC")
		assert.Equal(t, mainC.TripleCount(), 3)
	})

	t.Run("readUserDefinedBranch", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngCIRI)
		if err != nil {
			panic(err)
		}

		st, err := ds.CheckoutBranch(context.TODO(), "commit1", sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ng := st.NamedGraph(ngCIRI)
		if ng == nil {
			panic("got nil NamedGraph")
		}

		mainC := ng.GetIRINodeByFragment("mainC")
		assert.Equal(t, mainC.TripleCount(), 2)
	})

	t.Run("readCommitDetails", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		dsC, err := repo.Dataset(context.TODO(), ngCIRI)
		if err != nil {
			panic(err)
		}

		commitDetail, err := dsC.CommitDetailsByBranch(context.TODO(), sst.DefaultBranch)
		assert.NoError(t, err)
		for {
			commitDetail.Dump()
			st, err := dsC.CheckoutCommit(context.TODO(), commitDetail.Commit, sst.DefaultTriplexMode)
			assert.NoError(t, err)
			ng := st.NamedGraph(ngCIRI)
			ng.Dump()
			if len(commitDetail.ParentCommits[dsC.IRI()]) != 0 {
				// Suppose there is only one parent
				commitDetail, err = dsC.CommitDetailsByHash(context.TODO(), commitDetail.ParentCommits[dsC.IRI()][0])
				assert.NoError(t, err)
			} else {
				break
			}
		}

	})
}

func Test_LocalFullRepositoryMultipleCommits_IRINamedGraph(t *testing.T) {
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

		stageB.Commit(context.TODO(), "First commit of abc", sst.DefaultBranch)

		mainB.AddStatement(rdf.Bag, rep.Angle)

		stageB.Commit(context.TODO(), "second commit of abc", sst.DefaultBranch)
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

func Test_LocalFullRepositoryMultipleCommits_NGBandNGC(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngBIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362").URN())
	ngCIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363").URN())
	ngDIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a364").URN())

	t.Run("write NGA and NGB", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		ngB := st.CreateNamedGraph(ngBIRI)

		ngC := st.CreateNamedGraph(ngCIRI)

		ngB.CreateIRINode("mainB", rep.SchematicPort)
		ngC.CreateIRINode("mainC", rep.SchematicPort)

		st.Commit(context.TODO(), "added NGB amd NGC", sst.DefaultBranch)

	})

	t.Run("write add NGD", func(t *testing.T) {
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

		ngID := st.CreateNamedGraph(ngDIRI)
		ngID.CreateIRINode("mainD", rep.SchematicPort)

		st.Commit(context.TODO(), "add NGD", sst.DefaultBranch)
	})

	t.Run("read NGB", func(t *testing.T) {
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

		traversalByAPI(ngB)
	})

	t.Run("read NGC", func(t *testing.T) {
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

		ngC := st.NamedGraph(ngCIRI)
		if ngC == nil {
			panic("got nil NamedGraph")
		}

		traversalByAPI(ngC)
	})
}

func Test_LocalFullRepositoryMultipleCommits_NGBImportNGC_ModifyB(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngBIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362").URN())
	ngCIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363").URN())

	commitHash1 := sst.Hash{}
	commitHash2 := sst.Hash{}
	modifiedDS := []uuid.UUID{}

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

		commitHash1, _, err = st.Commit(context.TODO(), "Commit NGA imports NGB step1", sst.DefaultBranch)
		if err != nil {
			panic(err)
		}

		// modify mainB and commit again
		mainB.AddStatement(rdf.Bag, rep.Angle)
		commitHash2, modifiedDS, err = st.Commit(context.TODO(), "Modify B", sst.DefaultBranch)
		if err != nil {
			panic(err)
		}
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, ngBIRI, sst.IRI(modifiedDS[0].URN()))
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
		fmt.Println(ngB.Info().CheckedOutCommits)
		assert.Equal(t, commitHash2, ngB.Info().CheckedOutCommits[0])

		// ngB.Dump()

		ngC := st.NamedGraph(ngCIRI)
		if ngC == nil {
			panic("got nil NamedGraph")
		}

		fmt.Println(ngC.Info().CheckedOutCommits)
		assert.Equal(t, commitHash1, ngC.Info().CheckedOutCommits[0])

		// ngC.Dump()
		fmt.Println()
		traversalByAPI(ngB)
	})
}

func Test_LocalFullRepositoryMultipleCommits_NGBImportNGC_ModifyC(t *testing.T) {
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

		st.Commit(context.TODO(), "Commit NGA imports NGB step1", sst.DefaultBranch)

		// modifiy mainC and commit again
		mainC.AddStatement(rdf.Direction, rep.ConeAngle1)
		_, modifiedDS, _ := st.Commit(context.TODO(), "modify C", sst.DefaultBranch)
		assert.Equal(t, 2, len(modifiedDS))
		assert.Equal(t, ngBIRI, sst.IRI(modifiedDS[0].URN()))
		assert.Equal(t, ngCIRI, sst.IRI(modifiedDS[1].URN()))
	})
}

func Test_LocalFullRepositoryMultipleCommits_NGBImportNGC(t *testing.T) {
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

		_, modifiedDS, _ := st.Commit(context.TODO(), "Commit NGA imports NGB step1", sst.DefaultBranch)
		assert.Equal(t, 2, len(modifiedDS))
		assert.Equal(t, ngBIRI, sst.IRI(modifiedDS[0].URN()))
		assert.Equal(t, ngCIRI, sst.IRI(modifiedDS[1].URN()))

		// modifiy mainB and commit again
		mainB.AddStatement(rdf.Bag, rep.Angle)
		_, modifiedDS, _ = st.Commit(context.TODO(), "modify NGB and commit again", sst.DefaultBranch)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, ngBIRI, sst.IRI(modifiedDS[0].URN()))

		// modifiy mainC and commit again
		mainC.AddStatement(rdf.Direction, rep.ConeAngle1)
		_, modifiedDS, _ = st.Commit(context.TODO(), "modify NGC and commit again", sst.DefaultBranch)
		assert.Equal(t, 2, len(modifiedDS))
		assert.Equal(t, ngBIRI, sst.IRI(modifiedDS[0].URN()))
		assert.Equal(t, ngCIRI, sst.IRI(modifiedDS[1].URN()))

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
	t.Run("readCommitDetailsNGB", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		dsC, err := repo.Dataset(context.TODO(), ngBIRI)
		if err != nil {
			panic(err)
		}

		commitDetail, err := dsC.CommitDetailsByBranch(context.TODO(), sst.DefaultBranch)
		assert.NoError(t, err)
		for {
			commitDetail.Dump()
			st, err := dsC.CheckoutCommit(context.TODO(), commitDetail.Commit, sst.DefaultTriplexMode)
			assert.NoError(t, err)
			ng := st.NamedGraph(ngCIRI)
			ng.Dump()
			if len(commitDetail.ParentCommits[dsC.IRI()]) != 0 {
				// Suppose there is only one parent
				commitDetail, err = dsC.CommitDetailsByHash(context.TODO(), commitDetail.ParentCommits[dsC.IRI()][0])
				assert.NoError(t, err)
			} else {
				break
			}
		}
	})

	t.Run("readCommitDetailsNGC", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		dsC, err := repo.Dataset(context.TODO(), ngCIRI)
		if err != nil {
			panic(err)
		}

		commitDetail, err := dsC.CommitDetailsByBranch(context.TODO(), sst.DefaultBranch)
		assert.NoError(t, err)
		for {
			commitDetail.Dump()
			st, err := dsC.CheckoutCommit(context.TODO(), commitDetail.Commit, sst.DefaultTriplexMode)
			assert.NoError(t, err)
			ng := st.NamedGraph(ngCIRI)
			ng.Dump()
			if len(commitDetail.ParentCommits[dsC.IRI()]) != 0 {
				// Suppose there is only one parent
				commitDetail, err = dsC.CommitDetailsByHash(context.TODO(), commitDetail.ParentCommits[dsC.IRI()][0])
				assert.NoError(t, err)
			} else {
				break
			}
		}
	})
}

func Test_LocalFullRepositoryMultipleCommits_NGAImportsNGBImportsNGC_ModifyA(t *testing.T) {
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

		_, modifiedDS, _ := st.Commit(context.TODO(), "create NG-A, NG-B and NG-C", sst.DefaultBranch)
		assert.Equal(t, 3, len(modifiedDS))
		assert.Equal(t, ngAIRI, sst.IRI(modifiedDS[0].URN()))
		assert.Equal(t, ngBIRI, sst.IRI(modifiedDS[1].URN()))
		assert.Equal(t, ngCIRI, sst.IRI(modifiedDS[2].URN()))

		ngA.CreateIRINode("A2", lci.Person)
		_, modifiedDS, _ = st.Commit(context.TODO(), "modified NGA", sst.DefaultBranch)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, ngAIRI, sst.IRI(modifiedDS[0].URN()))
	})
}

func Test_LocalFullRepositoryMultipleCommits_NGAImportsNGBImportsNGC_ModifyB(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngAIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361").URN())
	ngBIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362").URN())
	ngCIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363").URN())

	commitHash1 := sst.Hash{}
	commitHash2 := sst.Hash{}
	modifiedDS := []uuid.UUID{}

	t.Run("write", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
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

		commitHash1, modifiedDS, err = st.Commit(context.TODO(), "create NG-A, NG-B and NG-C", sst.DefaultBranch)
		if err != nil {
			panic(err)
		}
		assert.Equal(t, 3, len(modifiedDS))
		assert.Equal(t, ngAIRI, sst.IRI(modifiedDS[0].URN()))
		assert.Equal(t, ngBIRI, sst.IRI(modifiedDS[1].URN()))
		assert.Equal(t, ngCIRI, sst.IRI(modifiedDS[2].URN()))

		ngB.CreateIRINode("B2", lci.Person)
		commitHash2, modifiedDS, err = st.Commit(context.TODO(), "modified NGB", sst.DefaultBranch)
		if err != nil {
			panic(err)
		}

		assert.Equal(t, 2, len(modifiedDS))
		assert.Equal(t, ngAIRI, sst.IRI(modifiedDS[0].URN()))
		assert.Equal(t, ngBIRI, sst.IRI(modifiedDS[1].URN()))
	})

	t.Run("read", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngAIRI)
		if err != nil {
			panic(err)
		}

		st, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
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

func Test_LocalFullRepositoryMultipleCommits_NGAImportsNGBImportsNGC_ModifyC(t *testing.T) {
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

		_, modifiedDS, _ := st.Commit(context.TODO(), "create NG-A, NG-B and NG-C", sst.DefaultBranch)
		assert.Equal(t, 3, len(modifiedDS))
		assert.Equal(t, ngAIRI, sst.IRI(modifiedDS[0].URN()))
		assert.Equal(t, ngBIRI, sst.IRI(modifiedDS[1].URN()))
		assert.Equal(t, ngCIRI, sst.IRI(modifiedDS[2].URN()))

		ngC.CreateIRINode("C2", lci.Person)
		_, modifiedDS, _ = st.Commit(context.TODO(), "modified NGC", sst.DefaultBranch)
		assert.Equal(t, 3, len(modifiedDS))
		assert.Equal(t, ngAIRI, sst.IRI(modifiedDS[0].URN()))
		assert.Equal(t, ngBIRI, sst.IRI(modifiedDS[1].URN()))
		assert.Equal(t, ngCIRI, sst.IRI(modifiedDS[2].URN()))
	})
}

func Test_LocalFullRepositoryMultipleCommits_NGAImportsNGBImportsNGC(t *testing.T) {
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

		ngC := st.CreateNamedGraph(ngCIRI)

		// Create mainC node
		mainC := ngC.CreateIRINode("mainC")
		mainC.AddStatement(rdf.Type, rep.SchematicPort)

		_, modifiedDS, err := st.Commit(context.TODO(), "create NG-C and mainC Node", sst.DefaultBranch)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, ngCIRI, sst.IRI(modifiedDS[0].URN()))

		// NG-B imports NG-C
		// Create mainB node references mainC
		ngB := st.CreateNamedGraph(ngBIRI)
		ngB.AddImport(ngC)
		mainB := ngB.CreateIRINode("mainB")
		mainB.AddStatement(rep.MappingSource, mainC)
		_, modifiedDS, err = st.Commit(context.TODO(), "create NG-B and reference to mainC", sst.DefaultBranch)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, ngBIRI, sst.IRI(modifiedDS[0].URN()))

		// NG-A imports NG-B
		// creates mainA node references mainB
		ngA := st.CreateNamedGraph(ngAIRI)
		ngA.AddImport(ngB)
		mainA := ngA.CreateIRINode("mainA")
		mainA.AddStatement(rep.MappingSource, mainB)

		_, modifiedDS, err = st.Commit(context.TODO(), "create NG-A and reference to mainB", sst.DefaultBranch)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, ngAIRI, sst.IRI(modifiedDS[0].URN()))
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
		ngA.CreateIRINode("A2", lci.Person)
		_, modifiedDS, err := st.Commit(context.TODO(), "modified NGA", sst.DefaultBranch)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, ngAIRI, sst.IRI(modifiedDS[0].URN()))

		ngA.Dump()

		ngB := st.NamedGraph(ngBIRI)
		if ngB == nil {
			panic("got nil NamedGraph")
		}
		ngB.CreateIRINode("B2", lci.Person)
		_, modifiedDS, _ = st.Commit(context.TODO(), "modified NGB", sst.DefaultBranch)
		assert.Equal(t, 2, len(modifiedDS))
		assert.Equal(t, ngAIRI, sst.IRI(modifiedDS[0].URN()))
		assert.Equal(t, ngBIRI, sst.IRI(modifiedDS[1].URN()))

		ngB.Dump()

		ngC := st.NamedGraph(ngCIRI)
		if ngC == nil {
			panic("got nil NamedGraph")
		}

		// modifiy mainC and commit again
		ngC.CreateIRINode("c2", lci.Person)
		_, modifiedDS, _ = st.Commit(context.TODO(), "modified NGC", sst.DefaultBranch)
		assert.Equal(t, 3, len(modifiedDS))
		assert.Equal(t, ngAIRI, sst.IRI(modifiedDS[0].URN()))
		assert.Equal(t, ngBIRI, sst.IRI(modifiedDS[1].URN()))
		assert.Equal(t, ngCIRI, sst.IRI(modifiedDS[2].URN()))

		ngC.Dump()

		traversalByAPI(ngA)
	})
	t.Run("readCommitDetailsNGA", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		dsC, err := repo.Dataset(context.TODO(), ngAIRI)
		if err != nil {
			panic(err)
		}

		commitDetail, err := dsC.CommitDetailsByBranch(context.TODO(), sst.DefaultBranch)
		assert.NoError(t, err)
		for {
			commitDetail.Dump()
			st, err := dsC.CheckoutCommit(context.TODO(), commitDetail.Commit, sst.DefaultTriplexMode)
			assert.NoError(t, err)
			ng := st.NamedGraph(ngCIRI)
			ng.Dump()
			if len(commitDetail.ParentCommits[dsC.IRI()]) != 0 {
				// Suppose there is only one parent
				commitDetail, err = dsC.CommitDetailsByHash(context.TODO(), commitDetail.ParentCommits[dsC.IRI()][0])
				assert.NoError(t, err)
			} else {
				break
			}
		}

	})
	t.Run("readCommitDetailsNGB", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		dsC, err := repo.Dataset(context.TODO(), ngBIRI)
		if err != nil {
			panic(err)
		}

		commitDetail, err := dsC.CommitDetailsByBranch(context.TODO(), sst.DefaultBranch)
		assert.NoError(t, err)
		for {
			commitDetail.Dump()
			st, err := dsC.CheckoutCommit(context.TODO(), commitDetail.Commit, sst.DefaultTriplexMode)
			assert.NoError(t, err)
			ng := st.NamedGraph(ngCIRI)
			ng.Dump()
			if len(commitDetail.ParentCommits[dsC.IRI()]) != 0 {
				// Suppose there is only one parent
				commitDetail, err = dsC.CommitDetailsByHash(context.TODO(), commitDetail.ParentCommits[dsC.IRI()][0])
				assert.NoError(t, err)
			} else {
				break
			}
		}
	})

	t.Run("readCommitDetailsNGC", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		dsC, err := repo.Dataset(context.TODO(), ngCIRI)
		if err != nil {
			panic(err)
		}

		commitDetail, err := dsC.CommitDetailsByBranch(context.TODO(), sst.DefaultBranch)
		assert.NoError(t, err)
		for {
			commitDetail.Dump()
			st, err := dsC.CheckoutCommit(context.TODO(), commitDetail.Commit, sst.DefaultTriplexMode)
			assert.NoError(t, err)
			ng := st.NamedGraph(ngCIRI)
			ng.Dump()
			if len(commitDetail.ParentCommits[dsC.IRI()]) != 0 {
				// Suppose there is only one parent
				commitDetail, err = dsC.CommitDetailsByHash(context.TODO(), commitDetail.ParentCommits[dsC.IRI()][0])
				assert.NoError(t, err)
			} else {
				break
			}
		}
	})
}

func Test_LocalFullRepositoryMultipleCommits_DiamondCase_ModifyA(t *testing.T) {
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

		_, modifiedDS, _ := st.Commit(context.TODO(), "Create Diamond", sst.DefaultBranch)
		assert.Equal(t, 4, len(modifiedDS))
		assert.Equal(t, ngAIRI, sst.IRI(modifiedDS[0].URN()))
		assert.Equal(t, ngBIRI, sst.IRI(modifiedDS[1].URN()))
		assert.Equal(t, ngCIRI, sst.IRI(modifiedDS[2].URN()))
		assert.Equal(t, ngDIRI, sst.IRI(modifiedDS[3].URN()))

		ngA.CreateIRINode("A2", lci.Person)
		_, modifiedDS, _ = st.Commit(context.TODO(), "modified NGA", sst.DefaultBranch)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, ngAIRI, sst.IRI(modifiedDS[0].URN()))
	})
}

func Test_LocalFullRepositoryMultipleCommits_DiamondCase_ModifyB(t *testing.T) {
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

		_, modifiedDS, _ := st.Commit(context.TODO(), "Create Diamond", sst.DefaultBranch)
		assert.Equal(t, 4, len(modifiedDS))
		assert.Equal(t, ngAIRI, sst.IRI(modifiedDS[0].URN()))
		assert.Equal(t, ngBIRI, sst.IRI(modifiedDS[1].URN()))
		assert.Equal(t, ngCIRI, sst.IRI(modifiedDS[2].URN()))
		assert.Equal(t, ngDIRI, sst.IRI(modifiedDS[3].URN()))
		ngB.CreateIRINode("B2", lci.Person)
		_, modifiedDS, _ = st.Commit(context.TODO(), "modified NGB", sst.DefaultBranch)
		assert.Equal(t, 2, len(modifiedDS))
		assert.Equal(t, ngAIRI, sst.IRI(modifiedDS[0].URN()))
		assert.Equal(t, ngBIRI, sst.IRI(modifiedDS[1].URN()))
	})
}

func Test_LocalFullRepositoryMultipleCommits_DiamondCase_ModifyC(t *testing.T) {
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

		_, modifiedDS, _ := st.Commit(context.TODO(), "Create Diamond", sst.DefaultBranch)
		assert.Equal(t, 4, len(modifiedDS))
		assert.Equal(t, ngAIRI, sst.IRI(modifiedDS[0].URN()))
		assert.Equal(t, ngBIRI, sst.IRI(modifiedDS[1].URN()))
		assert.Equal(t, ngCIRI, sst.IRI(modifiedDS[2].URN()))
		assert.Equal(t, ngDIRI, sst.IRI(modifiedDS[3].URN()))

		ngC.CreateIRINode("C2", lci.Person)
		_, modifiedDS, _ = st.Commit(context.TODO(), "modified NGC", sst.DefaultBranch)
		assert.Equal(t, 4, len(modifiedDS))
		assert.Equal(t, ngAIRI, sst.IRI(modifiedDS[0].URN()))
		assert.Equal(t, ngBIRI, sst.IRI(modifiedDS[1].URN()))
		assert.Equal(t, ngCIRI, sst.IRI(modifiedDS[2].URN()))
		assert.Equal(t, ngDIRI, sst.IRI(modifiedDS[3].URN()))
	})
}

func Test_LocalFullRepositoryMultipleCommits_DiamondCase_ModifyD(t *testing.T) {
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

		_, modifiedDS, _ := st.Commit(context.TODO(), "Create Diamond", sst.DefaultBranch)
		assert.Equal(t, 4, len(modifiedDS))
		assert.Equal(t, ngAIRI, sst.IRI(modifiedDS[0].URN()))
		assert.Equal(t, ngBIRI, sst.IRI(modifiedDS[1].URN()))
		assert.Equal(t, ngCIRI, sst.IRI(modifiedDS[2].URN()))
		assert.Equal(t, ngDIRI, sst.IRI(modifiedDS[3].URN()))

		ngD.CreateIRINode("D2", lci.Person)
		_, modifiedDS, _ = st.Commit(context.TODO(), "modified NGD", sst.DefaultBranch)
		assert.Equal(t, 2, len(modifiedDS))
		assert.Equal(t, ngAIRI, sst.IRI(modifiedDS[0].URN()))
		assert.Equal(t, ngDIRI, sst.IRI(modifiedDS[1].URN()))
	})
}

func Test_LocalFullRepositoryMultipleCommits_DiamondCase(t *testing.T) {
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

		// Create mainC node
		ngC := st.CreateNamedGraph(ngCIRI)
		mainC := ngC.CreateIRINode("mainC")
		mainC.AddStatement(rdf.Type, rep.SchematicPort)
		_, modifiedDS, _ := st.Commit(context.TODO(), "Diamond Case Create NG-C", sst.DefaultBranch)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, ngCIRI, sst.IRI(modifiedDS[0].URN()))

		// NG-B imports NG-C
		// Create mainB node references mainC
		ngB := st.CreateNamedGraph(ngBIRI)
		ngB.AddImport(ngC)
		mainB := ngB.CreateIRINode("mainB")
		mainB.AddStatement(rep.MappingSource, mainC)
		_, modifiedDS, _ = st.Commit(context.TODO(), "Diamond Case Create NG-B", sst.DefaultBranch)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, ngBIRI, sst.IRI(modifiedDS[0].URN()))

		// NG-D imports NG-C
		// Create mainD node references mainC
		ngD := st.CreateNamedGraph(ngDIRI)
		ngD.AddImport(ngC)

		mainD := ngD.CreateIRINode("mainD")
		mainD.AddStatement(rep.MappingSource, mainC)
		_, modifiedDS, _ = st.Commit(context.TODO(), "Diamond Case Create NG-D", sst.DefaultBranch)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, ngDIRI, sst.IRI(modifiedDS[0].URN()))

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
		_, modifiedDS, _ = st.Commit(context.TODO(), "Diamond Case Create NG-A", sst.DefaultBranch)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, ngAIRI, sst.IRI(modifiedDS[0].URN()))
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
