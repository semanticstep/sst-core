// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package repositorycommitdetails

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/sst_test/testutil"
	"github.com/semanticstep/sst-core/sstauth"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_remoteRepository_CommitDetails(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	t.Run("writeAndReadCommitDetails", func(t *testing.T) {

		url := testutil.ServerServe(t, dir)
		ctx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		repo, err := sst.OpenRemoteRepository(ctx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		// Commit 1
		ng1 := st.CreateNamedGraph("")
		require.NotNil(t, ng1)
		ng1.CreateIRINode("node1").AddStatement(rdf.Type, rep.SchematicPort)
		commit1, _, err := st.Commit(ctx, "First remote commit", sst.DefaultBranch)
		require.NoError(t, err)

		// Commit 2
		ng2 := st.CreateNamedGraph("")
		require.NotNil(t, ng2)
		ng2.CreateIRINode("node2").AddStatement(rdf.Type, rep.SchematicPort)
		commit2, _, err := st.Commit(ctx, "Second remote commit", sst.DefaultBranch)
		require.NoError(t, err)

		// Add fake hash to test behavior
		var fakeHash sst.Hash
		copy(fakeHash[:], bytes.Repeat([]byte{0xBA}, 32)) // some invalid hash

		// Call new unified CommitDetails method
		detailsList, err := repo.CommitDetails(ctx, []sst.Hash{commit1, fakeHash, commit2})
		require.NoError(t, err)

		// Should return only 2 valid entries
		require.Len(t, detailsList, 2)

		// Assert Commit 1
		assert.Equal(t, commit1, detailsList[0].Commit)
		assert.Equal(t, "First remote commit", detailsList[0].Message)
		assert.NotEmpty(t, detailsList[0].Author)
		assert.False(t, detailsList[0].AuthorDate.IsZero())

		// Assert Commit 2
		assert.Equal(t, commit2, detailsList[1].Commit)
		assert.Equal(t, "Second remote commit", detailsList[1].Message)
		assert.NotEmpty(t, detailsList[1].Author)
		assert.False(t, detailsList[1].AuthorDate.IsZero())
	})
}

func Test_RemoteRepositoryMultipleCommits_UUIDNamedGraph(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	ngIDC := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363")

	var commitHash1 sst.Hash
	var commitHash3 sst.Hash
	var modifiedDS []uuid.UUID

	t.Run("write", func(t *testing.T) {

		url := testutil.ServerServe(t, dir)
		ctx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		repo, err := sst.OpenRemoteRepository(ctx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		stageC := repo.OpenStage(sst.DefaultTriplexMode)
		ng := stageC.CreateNamedGraph(sst.IRI(ngIDC.URN()))
		mainC := ng.CreateIRINode("mainC")

		mainC.AddStatement(rdf.Type, rep.SchematicPort)
		commitHash1, modifiedDS, err = stageC.Commit(ctx, "First commit of C", sst.DefaultBranch)
		require.NoError(t, err)
		require.Len(t, modifiedDS, 1)
		require.Equal(t, ngIDC, modifiedDS[0])

		ds, err := repo.Dataset(ctx, sst.IRI(ngIDC.URN()))
		require.NoError(t, err)
		err = ds.SetBranchCommit(ctx, commitHash1, "commit1")
		require.NoError(t, err)

		mainC.AddStatement(rdf.Bag, rep.Angle)
		_, modifiedDS, err = stageC.Commit(ctx, "Second commit of C", sst.DefaultBranch)
		require.NoError(t, err)
		require.Len(t, modifiedDS, 1)
		require.Equal(t, ngIDC, modifiedDS[0])

		mainC.AddStatement(rdf.Direction, rep.Blue)
		commitHash3, _, err = stageC.Commit(ctx, "Third commit of C", sst.DefaultBranch)
		require.NoError(t, err)
	})

	t.Run("CommitDetailsByHash", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		ctx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		repo, err := sst.OpenRemoteRepository(ctx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		detailsList, err := repo.CommitDetails(ctx, []sst.Hash{commitHash3})
		require.NoError(t, err)
		require.Len(t, detailsList, 1)

		detailsList[0].Dump()

		require.Equal(t, commitHash3, detailsList[0].Commit)
		require.Contains(t, detailsList[0].Message, "Third")
		require.NotEmpty(t, detailsList[0].Author)
		require.False(t, detailsList[0].AuthorDate.IsZero())
	})
}
