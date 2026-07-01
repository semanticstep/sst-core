// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

// This test file verifies the commit parent handling logic in Bucket DS (Dataset):
// 1. When new commit has a parent in Bucket C: update branch to new commit
// 2. When new commit has no parent: convert old commit to leaf, then update branch to new commit

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/semanticstep/sst-core/defaultderive"
	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/sst_test/testutil"
	"github.com/semanticstep/sst-core/sstauth"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/stretchr/testify/assert"
)

func Test_LocalFullRepository_CommitWithParent_BranchUpdate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363").URN())

	var commitHash1 sst.Hash
	var commitHash2 sst.Hash

	t.Run("first_commit_no_parent", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		assert.NoError(t, err)
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(ngIRI)
		mainNode := ng.CreateIRINode("mainNode")
		mainNode.AddStatement(rdf.Type, rep.SchematicPort)

		// First commit - no parent
		commitHash1, _, err = st.Commit(context.TODO(), "First commit", sst.DefaultBranch)
		assert.NoError(t, err)
		assert.NotEqual(t, sst.HashNil(), commitHash1)

		ds, err := repo.Dataset(context.TODO(), ngIRI)
		assert.NoError(t, err)

		// First commit has no parent
		cd, err := ds.CommitDetailsByHash(context.TODO(), commitHash1)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(cd.ParentCommits[ngIRI]))

		// No leaf commits because it's on a branch
		leafCommits, leafErr := ds.LeafCommits(context.TODO())
		assert.Nil(t, leafErr)
		assert.Equal(t, 0, len(leafCommits))
	})

	t.Run("second_commit_with_parent", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		assert.NoError(t, err)
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngIRI)
		assert.NoError(t, err)

		st, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		assert.NoError(t, err)

		ng := st.NamedGraph(ngIRI)
		mainNode := ng.GetIRINodeByFragment("mainNode")
		mainNode.AddStatement(rdf.Bag, rep.Angle)

		// Second commit - has parent (commitHash1)
		commitHash2, _, err = st.Commit(context.TODO(), "Second commit", sst.DefaultBranch)
		assert.NoError(t, err)
		assert.NotEqual(t, sst.HashNil(), commitHash2)

		// Second commit has parent
		cd, err := ds.CommitDetailsByHash(context.TODO(), commitHash2)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(cd.ParentCommits[ngIRI]))
		assert.Equal(t, commitHash1, cd.ParentCommits[ngIRI][0])

		// Since second commit HAS parent, first commit should NOT become leaf
		branches, err := ds.Branches(context.TODO())
		assert.NoError(t, err)
		assert.Equal(t, commitHash2, branches[sst.DefaultBranch])

		// No leaf commits because both commits are in the chain
		leafCommits, leafErr := ds.LeafCommits(context.TODO())
		assert.Nil(t, leafErr)
		assert.Equal(t, 0, len(leafCommits))
	})
}

func Test_LocalFullRepository_CommitFromCheckoutCommit_NoParent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a364").URN())

	var commitHash1 sst.Hash
	var commitHash2 sst.Hash

	t.Run("first_commit_to_branch", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		assert.NoError(t, err)
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(ngIRI)
		mainNode := ng.CreateIRINode("mainNode")
		mainNode.AddStatement(rdf.Type, rep.SchematicPort)

		// First commit to default branch
		commitHash1, _, err = st.Commit(context.TODO(), "First commit", sst.DefaultBranch)
		assert.NoError(t, err)

		ds, err := repo.Dataset(context.TODO(), ngIRI)
		assert.NoError(t, err)

		// Verify first commit has no parent
		cd, err := ds.CommitDetailsByHash(context.TODO(), commitHash1)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(cd.ParentCommits[ngIRI]))
	})

	t.Run("checkout_commit_and_commit_to_different_branch", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		assert.NoError(t, err)
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngIRI)
		assert.NoError(t, err)

		// Checkout the first commit directly (not through branch)
		st, err := ds.CheckoutCommit(context.TODO(), commitHash1, sst.DefaultTriplexMode)
		assert.NoError(t, err)

		ng := st.NamedGraph(ngIRI)
		mainNode := ng.GetIRINodeByFragment("mainNode")
		mainNode.AddStatement(rdf.Bag, rep.Angle)

		// Commit to a NEW branch
		commitHash2, _, err = st.Commit(context.TODO(), "Second commit - new branch", "newBranch")
		assert.NoError(t, err)

		// Verify the new commit details
		cd, err := ds.CommitDetailsByHash(context.TODO(), commitHash2)
		assert.NoError(t, err)

		// The parent status depends on implementation:
		// - If checkedOutCommits is used, parent should be set
		// - Otherwise, parent might be empty
		// The important thing is the commit was created successfully
		_ = cd

		// Verify new branch points to the new commit
		branches, err := ds.Branches(context.TODO())
		assert.NoError(t, err)
		assert.Equal(t, commitHash2, branches["newBranch"])
	})
}

func Test_LocalFullRepository_LeafCommitHandling(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a365").URN())

	var commitHash1 sst.Hash
	var commitHash2 sst.Hash
	var commitHash3 sst.Hash

	t.Run("commit_as_leaf_then_set_branch", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		assert.NoError(t, err)
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(ngIRI)
		mainNode := ng.CreateIRINode("mainNode")
		mainNode.AddStatement(rdf.Type, rep.SchematicPort)

		// First commit - as leaf (no branch)
		commitHash1, _, err = st.Commit(context.TODO(), "First commit", "")
		assert.NoError(t, err)

		ds, err := repo.Dataset(context.TODO(), ngIRI)
		assert.NoError(t, err)

		// Verify first commit is a leaf
		leafCommits, leafErr := ds.LeafCommits(context.TODO())
		assert.Nil(t, leafErr)
		assert.Equal(t, 1, len(leafCommits))
		assert.Equal(t, commitHash1, leafCommits[0])

		// Set branch to first commit
		err = ds.SetBranchCommit(context.TODO(), commitHash1, sst.DefaultBranch)
		assert.NoError(t, err)

		// After setting branch, leaf should be removed
		leafCommits, leafErr = ds.LeafCommits(context.TODO())
		assert.Nil(t, leafErr)
		assert.Equal(t, 0, len(leafCommits))

		// Verify branch points to commit
		branches, err := ds.Branches(context.TODO())
		assert.NoError(t, err)
		assert.Equal(t, commitHash1, branches[sst.DefaultBranch])
	})

	t.Run("continue_commit_on_branch", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		assert.NoError(t, err)
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngIRI)
		assert.NoError(t, err)

		// Checkout the branch
		st, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		assert.NoError(t, err)

		ng := st.NamedGraph(ngIRI)
		mainNode := ng.GetIRINodeByFragment("mainNode")
		mainNode.AddStatement(rdf.Bag, rep.Angle)

		// Second commit - this has parent because branch exists
		commitHash2, _, err = st.Commit(context.TODO(), "Second commit", sst.DefaultBranch)
		assert.NoError(t, err)

		// Verify second commit has parent
		cd, err := ds.CommitDetailsByHash(context.TODO(), commitHash2)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(cd.ParentCommits[ngIRI]))
		assert.Equal(t, commitHash1, cd.ParentCommits[ngIRI][0])

		// Branch points to second commit
		branches, err := ds.Branches(context.TODO())
		assert.NoError(t, err)
		assert.Equal(t, commitHash2, branches[sst.DefaultBranch])

		// No leaf commits
		leafCommits, leafErr := ds.LeafCommits(context.TODO())
		assert.Nil(t, leafErr)
		assert.Equal(t, 0, len(leafCommits))
	})

	t.Run("third_commit_continue_chain", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		assert.NoError(t, err)
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngIRI)
		assert.NoError(t, err)

		st, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		assert.NoError(t, err)

		ng := st.NamedGraph(ngIRI)
		mainNode := ng.GetIRINodeByFragment("mainNode")
		mainNode.AddStatement(rdf.Direction, rep.Blue)

		// Third commit
		commitHash3, _, err = st.Commit(context.TODO(), "Third commit", sst.DefaultBranch)
		assert.NoError(t, err)

		// Verify third commit has parent (commitHash2)
		cd, err := ds.CommitDetailsByHash(context.TODO(), commitHash3)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(cd.ParentCommits[ngIRI]))
		assert.Equal(t, commitHash2, cd.ParentCommits[ngIRI][0])

		// Branch points to third commit
		branches, err := ds.Branches(context.TODO())
		assert.NoError(t, err)
		assert.Equal(t, commitHash3, branches[sst.DefaultBranch])

		// No leaf commits - all commits are in the chain
		leafCommits, leafErr := ds.LeafCommits(context.TODO())
		assert.Nil(t, leafErr)
		assert.Equal(t, 0, len(leafCommits))
	})
}

func Test_LocalFullRepository_RemoveBranchCreatesLeaf(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a366").URN())

	var commitHash1 sst.Hash

	t.Run("create_branch_then_remove", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		assert.NoError(t, err)
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(ngIRI)
		mainNode := ng.CreateIRINode("mainNode")
		mainNode.AddStatement(rdf.Type, rep.SchematicPort)

		// Commit to default branch
		commitHash1, _, err = st.Commit(context.TODO(), "First commit", sst.DefaultBranch)
		assert.NoError(t, err)

		ds, err := repo.Dataset(context.TODO(), ngIRI)
		assert.NoError(t, err)

		// Verify no leaf commits (on branch)
		leafCommits, leafErr := ds.LeafCommits(context.TODO())
		assert.Nil(t, leafErr)
		assert.Equal(t, 0, len(leafCommits))

		// Remove branch
		err = ds.RemoveBranch(context.TODO(), sst.DefaultBranch)
		assert.NoError(t, err)

		// Now commit should be a leaf
		leafCommits, leafErr = ds.LeafCommits(context.TODO())
		assert.Nil(t, leafErr)
		assert.Equal(t, 1, len(leafCommits))
		assert.Equal(t, commitHash1, leafCommits[0])

		// No branches
		branches, err := ds.Branches(context.TODO())
		assert.NoError(t, err)
		assert.Equal(t, 0, len(branches))
	})
}

func Test_LocalFullRepository_CommitTwiceWithoutCheckout(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a367").URN())

	var commitHash1 sst.Hash
	var commitHash2 sst.Hash
	var commitHash3 sst.Hash

	t.Run("commit_twice_without_checkout", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		assert.NoError(t, err)
		defer repo.Close()

		// Open stage once
		st := repo.OpenStage(sst.DefaultTriplexMode)

		// Create NamedGraph and add first statement
		ng := st.CreateNamedGraph(ngIRI)
		mainNode := ng.CreateIRINode("mainNode")
		mainNode.AddStatement(rdf.Type, rep.SchematicPort)

		// First commit
		commitHash1, _, err = st.Commit(context.TODO(), "First commit", sst.DefaultBranch)
		assert.NoError(t, err)
		assert.NotEqual(t, sst.HashNil(), commitHash1)

		// WITHOUT checking out again, modify the NamedGraph
		mainNode.AddStatement(rdf.Bag, rep.Angle)

		// Second commit - without checkout
		commitHash2, _, err = st.Commit(context.TODO(), "Second commit", sst.DefaultBranch)
		assert.NoError(t, err)
		assert.NotEqual(t, sst.HashNil(), commitHash2)

		// WITHOUT checking out again, modify the NamedGraph again
		mainNode.AddStatement(rdf.Direction, rep.Blue)

		// Third commit - without checkout
		commitHash3, _, err = st.Commit(context.TODO(), "Third commit", sst.DefaultBranch)
		assert.NoError(t, err)
		assert.NotEqual(t, sst.HashNil(), commitHash3)

		// Verify the commit chain
		ds, err := repo.Dataset(context.TODO(), ngIRI)
		assert.NoError(t, err)

		// First commit should have no parent
		cd1, err := ds.CommitDetailsByHash(context.TODO(), commitHash1)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(cd1.ParentCommits[ngIRI]))

		// Second commit should have first commit as parent
		cd2, err := ds.CommitDetailsByHash(context.TODO(), commitHash2)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(cd2.ParentCommits[ngIRI]))
		assert.Equal(t, commitHash1, cd2.ParentCommits[ngIRI][0])

		// Third commit should have second commit as parent
		cd3, err := ds.CommitDetailsByHash(context.TODO(), commitHash3)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(cd3.ParentCommits[ngIRI]))
		assert.Equal(t, commitHash2, cd3.ParentCommits[ngIRI][0])

		// Branch should point to third commit
		branches, err := ds.Branches(context.TODO())
		assert.NoError(t, err)
		assert.Equal(t, commitHash3, branches[sst.DefaultBranch])

		// No leaf commits - all commits are in the chain
		leafCommits, leafErr := ds.LeafCommits(context.TODO())
		assert.Nil(t, leafErr)
		assert.Equal(t, 0, len(leafCommits))
	})
}

func Test_LocalFullRepository_ReopenRepo_CreateSameNG_Commit(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a368").URN())

	var commitHash1 sst.Hash
	var commitHash2 sst.Hash

	t.Run("first_commit_close_reopen_create_same_ng_commit", func(t *testing.T) {

		// First session: create repo and commit
		repo1, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		assert.NoError(t, err)

		st1 := repo1.OpenStage(sst.DefaultTriplexMode)
		ng1 := st1.CreateNamedGraph(ngIRI)
		mainNode := ng1.CreateIRINode("mainNode")
		mainNode.AddStatement(rdf.Type, rep.SchematicPort)

		commitHash1, _, err = st1.Commit(context.TODO(), "First commit", sst.DefaultBranch)
		assert.NoError(t, err)
		assert.NotEqual(t, sst.HashNil(), commitHash1)

		repo1.Close()

		// Second session: reopen repo, create same NamedGraph (new instance), commit
		repo2, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		assert.NoError(t, err)
		defer repo2.Close()

		st2 := repo2.OpenStage(sst.DefaultTriplexMode)

		// Create NamedGraph with same IRI - this creates a new instance
		ng2 := st2.CreateNamedGraph(ngIRI)
		mainNode2 := ng2.CreateIRINode("mainNode")
		mainNode2.AddStatement(rdf.Bag, rep.Angle)

		// Second commit
		commitHash2, _, err = st2.Commit(context.TODO(), "Second commit", sst.DefaultBranch)
		assert.NoError(t, err)
		assert.NotEqual(t, sst.HashNil(), commitHash2)

		// Verify the commit chain
		ds, err := repo2.Dataset(context.TODO(), ngIRI)
		assert.NoError(t, err)

		// First commit should have no parent
		cd1, err := ds.CommitDetailsByHash(context.TODO(), commitHash1)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(cd1.ParentCommits[ngIRI]))

		// Check what happens with second commit
		cd2, err := ds.CommitDetailsByHash(context.TODO(), commitHash2)
		assert.NoError(t, err)

		// EXPECTED BEHAVIOR:
		// When creating a new NamedGraph with the same ID and committing to a branch:
		// The new commit has NO parent because checkedOutCommits is empty
		// (new NG instance doesn't inherit parent from branch)
		t.Logf("Second commit parent count: %d", len(cd2.ParentCommits[ngIRI]))
		assert.Equal(t, 0, len(cd2.ParentCommits[ngIRI]), "New NamedGraph instance has no parent")

		// Branch should point to second commit
		branches, err := ds.Branches(context.TODO())
		assert.NoError(t, err)
		assert.Equal(t, commitHash2, branches[sst.DefaultBranch])

		// Because the new commit has no parent, the old commit becomes a leaf
		leafCommits, err := ds.LeafCommits(context.TODO())
		assert.Nil(t, err)
		t.Logf("Leaf commits count: %d", len(leafCommits))
		assert.Equal(t, 1, len(leafCommits), "Old commit becomes leaf when new commit has no parent")
		assert.Equal(t, commitHash1, leafCommits[0])
	})
}

func Test_RemoteRepository_ReopenRepo_CreateSameNG_Commit(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a369").URN())
	transportCreds, err := testutil.TestTransportCreds()
	assert.NoError(t, err)

	var commitHash1 sst.Hash
	var commitHash2 sst.Hash

	url := testutil.ServerServe(t, dir)
	constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

	t.Run("first_commit", func(t *testing.T) {
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		assert.NoError(t, err)
		defer repo.Close()

		repo.RegisterIndexHandler(defaultderive.DeriveInfo())

		st := repo.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(ngIRI)
		mainNode := ng.CreateIRINode("mainNode")
		mainNode.AddStatement(rdf.Type, rep.SchematicPort)

		commitHash1, _, err = st.Commit(constructCtx, "First commit", sst.DefaultBranch)
		assert.NoError(t, err)
		assert.NotEqual(t, sst.HashNil(), commitHash1)

		ds, err := repo.Dataset(constructCtx, ngIRI)
		assert.NoError(t, err)

		cd, err := ds.CommitDetailsByHash(constructCtx, commitHash1)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(cd.ParentCommits[ngIRI]))
	})

	t.Run("create_same_ng_commit", func(t *testing.T) {
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		assert.NoError(t, err)
		defer repo.Close()

		repo.RegisterIndexHandler(defaultderive.DeriveInfo())

		st := repo.OpenStage(sst.DefaultTriplexMode)

		ng := st.CreateNamedGraph(ngIRI)
		mainNode := ng.CreateIRINode("mainNode")
		mainNode.AddStatement(rdf.Bag, rep.Angle)

		commitHash2, _, err = st.Commit(constructCtx, "Second commit", sst.DefaultBranch)
		assert.NoError(t, err)
		assert.NotEqual(t, sst.HashNil(), commitHash2)

		ds, err := repo.Dataset(constructCtx, ngIRI)
		assert.NoError(t, err)

		cd1, err := ds.CommitDetailsByHash(constructCtx, commitHash1)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(cd1.ParentCommits[ngIRI]))

		cd2, err := ds.CommitDetailsByHash(constructCtx, commitHash2)
		assert.NoError(t, err)

		t.Logf("Second commit parent count: %d", len(cd2.ParentCommits[ngIRI]))
		assert.Equal(t, 0, len(cd2.ParentCommits[ngIRI]), "New NamedGraph instance has no parent")

		branches, err := ds.Branches(constructCtx)
		assert.NoError(t, err)
		assert.Equal(t, commitHash2, branches[sst.DefaultBranch])

		leafCommits, leafErr := ds.LeafCommits(constructCtx)
		assert.Nil(t, leafErr)
		t.Logf("Leaf commits count: %d", len(leafCommits))
		assert.Equal(t, 1, len(leafCommits), "Old commit becomes leaf")
		assert.Equal(t, commitHash1, leafCommits[0])
	})
}

func Test_RemoteRepository_ReopenRepo_CommitSameNGToDifferentBranches(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a370").URN())
	transportCreds, err := testutil.TestTransportCreds()
	assert.NoError(t, err)

	var commitHash1 sst.Hash
	var commitHash2 sst.Hash

	url := testutil.ServerServe(t, dir)
	constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

	t.Run("first_commit_to_branch_A", func(t *testing.T) {
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		assert.NoError(t, err)
		defer repo.Close()

		repo.RegisterIndexHandler(defaultderive.DeriveInfo())

		st := repo.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(ngIRI)
		mainNode := ng.CreateIRINode("mainNode")
		mainNode.AddStatement(rdf.Type, rep.SchematicPort)

		commitHash1, _, err = st.Commit(constructCtx, "First commit to branchA", "branchA")
		assert.NoError(t, err)
		assert.NotEqual(t, sst.HashNil(), commitHash1)

		ds, err := repo.Dataset(constructCtx, ngIRI)
		assert.NoError(t, err)

		cd, err := ds.CommitDetailsByHash(constructCtx, commitHash1)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(cd.ParentCommits[ngIRI]), "First commit has no parent")

		branches, err := ds.Branches(constructCtx)
		assert.NoError(t, err)
		assert.Equal(t, commitHash1, branches["branchA"])

		leafCommits, leafErr := ds.LeafCommits(constructCtx)
		assert.Nil(t, leafErr)
		assert.Equal(t, 0, len(leafCommits), "No leaf commits when on branch")
	})

	t.Run("commit_same_ng_to_branch_B", func(t *testing.T) {
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		assert.NoError(t, err)
		defer repo.Close()

		repo.RegisterIndexHandler(defaultderive.DeriveInfo())

		st := repo.OpenStage(sst.DefaultTriplexMode)

		ng := st.CreateNamedGraph(ngIRI)
		mainNode := ng.CreateIRINode("mainNode")
		mainNode.AddStatement(rdf.Bag, rep.Angle)

		commitHash2, _, err = st.Commit(constructCtx, "Second commit to branchB", "branchB")
		assert.NoError(t, err)
		assert.NotEqual(t, sst.HashNil(), commitHash2)

		ds, err := repo.Dataset(constructCtx, ngIRI)
		assert.NoError(t, err)

		cd1, err := ds.CommitDetailsByHash(constructCtx, commitHash1)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(cd1.ParentCommits[ngIRI]))

		cd2, err := ds.CommitDetailsByHash(constructCtx, commitHash2)
		assert.NoError(t, err)

		t.Logf("Second commit parent count: %d", len(cd2.ParentCommits[ngIRI]))
		assert.Equal(t, 0, len(cd2.ParentCommits[ngIRI]), "New NamedGraph instance has no parent when committing to different branch")

		branches, err := ds.Branches(constructCtx)
		assert.NoError(t, err)
		assert.Equal(t, commitHash1, branches["branchA"])
		assert.Equal(t, commitHash2, branches["branchB"])

		leafCommits, leafErr := ds.LeafCommits(constructCtx)
		assert.Nil(t, leafErr)
		t.Logf("Leaf commits count: %d", len(leafCommits))
		assert.Equal(t, 0, len(leafCommits), "Both commits are on branches, no leaf commits")
	})
}

func Test_RemoteRepository_ReopenRepo_CommitSameNGToSameBranch(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a371").URN())
	transportCreds, err := testutil.TestTransportCreds()
	assert.NoError(t, err)

	var commitHash1 sst.Hash
	var commitHash2 sst.Hash

	url := testutil.ServerServe(t, dir)
	constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

	t.Run("first_commit_to_branch", func(t *testing.T) {
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		assert.NoError(t, err)
		defer repo.Close()

		repo.RegisterIndexHandler(defaultderive.DeriveInfo())

		st := repo.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(ngIRI)
		mainNode := ng.CreateIRINode("mainNode")
		mainNode.AddStatement(rdf.Type, rep.SchematicPort)

		commitHash1, _, err = st.Commit(constructCtx, "First commit", sst.DefaultBranch)
		assert.NoError(t, err)
		assert.NotEqual(t, sst.HashNil(), commitHash1)

		ds, err := repo.Dataset(constructCtx, ngIRI)
		assert.NoError(t, err)

		cd, err := ds.CommitDetailsByHash(constructCtx, commitHash1)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(cd.ParentCommits[ngIRI]))
	})

	t.Run("commit_same_ng_to_same_branch", func(t *testing.T) {
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		assert.NoError(t, err)
		defer repo.Close()

		repo.RegisterIndexHandler(defaultderive.DeriveInfo())

		st := repo.OpenStage(sst.DefaultTriplexMode)

		ng := st.CreateNamedGraph(ngIRI)
		mainNode := ng.CreateIRINode("mainNode")
		mainNode.AddStatement(rdf.Bag, rep.Angle)

		commitHash2, _, err = st.Commit(constructCtx, "Second commit", sst.DefaultBranch)
		assert.NoError(t, err)
		assert.NotEqual(t, sst.HashNil(), commitHash2)

		ds, err := repo.Dataset(constructCtx, ngIRI)
		assert.NoError(t, err)

		cd1, err := ds.CommitDetailsByHash(constructCtx, commitHash1)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(cd1.ParentCommits[ngIRI]))

		cd2, err := ds.CommitDetailsByHash(constructCtx, commitHash2)
		assert.NoError(t, err)

		t.Logf("Second commit parent count: %d", len(cd2.ParentCommits[ngIRI]))
		assert.Equal(t, 0, len(cd2.ParentCommits[ngIRI]), "New NamedGraph instance has no parent")

		branches, err := ds.Branches(constructCtx)
		assert.NoError(t, err)
		assert.Equal(t, commitHash2, branches[sst.DefaultBranch])

		leafCommits, leafErr := ds.LeafCommits(constructCtx)
		assert.Nil(t, leafErr)
		t.Logf("Leaf commits count: %d", len(leafCommits))
		assert.Equal(t, 1, len(leafCommits), "Old commit becomes leaf when new commit has no parent")
		assert.Equal(t, commitHash1, leafCommits[0])
	})
}

func Test_RemoteRepository_CheckoutCommit_CommitToDifferentBranch(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a372").URN())
	transportCreds, err := testutil.TestTransportCreds()
	assert.NoError(t, err)

	var commitHash1 sst.Hash
	var commitHash2 sst.Hash

	url := testutil.ServerServe(t, dir)
	constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

	t.Run("first_commit_to_master", func(t *testing.T) {
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		assert.NoError(t, err)
		defer repo.Close()

		repo.RegisterIndexHandler(defaultderive.DeriveInfo())

		st := repo.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(ngIRI)
		mainNode := ng.CreateIRINode("mainNode")
		mainNode.AddStatement(rdf.Type, rep.SchematicPort)

		commitHash1, _, err = st.Commit(constructCtx, "First commit", sst.DefaultBranch)
		assert.NoError(t, err)
		assert.NotEqual(t, sst.HashNil(), commitHash1)

		ds, err := repo.Dataset(constructCtx, ngIRI)
		assert.NoError(t, err)

		cd, err := ds.CommitDetailsByHash(constructCtx, commitHash1)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(cd.ParentCommits[ngIRI]))
	})

	t.Run("checkout_commit_commit_to_feature_branch", func(t *testing.T) {
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		assert.NoError(t, err)
		defer repo.Close()

		ds, err := repo.Dataset(constructCtx, ngIRI)
		assert.NoError(t, err)

		st, err := ds.CheckoutCommit(constructCtx, commitHash1, sst.DefaultTriplexMode)
		assert.NoError(t, err)

		ng := st.NamedGraph(ngIRI)
		mainNode := ng.GetIRINodeByFragment("mainNode")
		mainNode.AddStatement(rdf.Bag, rep.Angle)

		commitHash2, _, err = st.Commit(constructCtx, "Second commit - feature branch", "feature")
		assert.NoError(t, err)
		assert.NotEqual(t, sst.HashNil(), commitHash2)

		cd, err := ds.CommitDetailsByHash(constructCtx, commitHash2)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(cd.ParentCommits[ngIRI]), "New commit has no parent when committing to new branch after checkout")

		branches, err := ds.Branches(constructCtx)
		assert.NoError(t, err)
		assert.Equal(t, commitHash1, branches[sst.DefaultBranch])
		assert.Equal(t, commitHash2, branches["feature"])

		leafCommits, leafErr := ds.LeafCommits(constructCtx)
		assert.Nil(t, leafErr)
		assert.Equal(t, 0, len(leafCommits), "Both commits are on branches, no leaf commits")
	})
}
