// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package setbranchremovebranchlogtest

import (
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

func Test_remoteRepository_SetRemoveBranch_LogEntry(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	t.Run("writeSetAndRemoveBranchLogs", func(t *testing.T) {

		url := testutil.ServerServe(t, dir)
		ctx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		repo, err := sst.OpenRemoteRepository(ctx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		stage := repo.OpenStage(sst.DefaultTriplexMode)

		ngID := uuid.New()
		ng := stage.CreateNamedGraph(sst.IRI(ngID.URN()))
		require.NotNil(t, ng)

		ng.CreateIRINode("testNode").AddStatement(rdf.Type, rep.SchematicPort)
		commitHash, _, err := stage.Commit(ctx, "initial remote commit", sst.DefaultBranch)
		require.NoError(t, err)

		ds, err := repo.Dataset(ctx, sst.IRI(ngID.URN()))
		require.NoError(t, err)

		err = ds.SetBranchCommit(ctx, commitHash, "dev")
		require.NoError(t, err)

		err = ds.RemoveBranch(ctx, "dev")
		require.NoError(t, err)

		logs, err := repo.Log(ctx, nil, nil)
		require.NoError(t, err)

		var hasSet, hasRemove bool
		for _, entry := range logs {
			if msg := entry.Fields["message"]; msg == "set branch" && entry.Fields["branch"] == "dev" {
				hasSet = true
			}
			if msg := entry.Fields["message"]; msg == "remove branch" && entry.Fields["branch"] == "dev" {
				hasRemove = true
			}
		}

		assert.True(t, hasSet, "log entry for set branch not found")
		assert.True(t, hasRemove, "log entry for remove branch not found")
	})
}
