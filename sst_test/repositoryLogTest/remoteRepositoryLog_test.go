// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/sst_test/testutil"
	"github.com/semanticstep/sst-core/sstauth"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/stretchr/testify/require"
)

func Test_remoteRepositoryLog_SingleCommit(t *testing.T) {
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	// Helper: Create int pointer
	pointerTo := func(x int) *int { return &x }

	t.Run("SingleCommit_LogEntryCreated_Remote", func(t *testing.T) {

		url := testutil.ServerServe(t, filepath.Join(t.TempDir(), t.Name()))
		ctx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(ctx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		stage := repo.OpenStage(sst.DefaultTriplexMode)
		ng := stage.CreateNamedGraph("")
		node := ng.CreateIRINode("logNode")
		node.AddStatement(rdf.Type, rep.SchematicPort)

		_, modifiedDSIDs, err := stage.Commit(ctx, "Initial commit for remote log", sst.DefaultBranch)
		require.NoError(t, err)
		require.Len(t, modifiedDSIDs, 1)

		// Use default log retrieval (should return recent 100 logs)
		logEntries, err := repo.Log(ctx, nil, nil)
		require.NoError(t, err)

		// Filter for entries with commit_id
		var commits []sst.RepositoryLogEntry
		for _, entry := range logEntries {
			if cid, ok := entry.Fields["commit_id"]; ok && cid != "" {
				commits = append(commits, entry)
			}
		}

		require.Len(t, commits, 1, "Expected 1 commit log entry (excluding repo creation)")
		require.NotEmpty(t, commits[0].Fields["commit_id"])
	})

	t.Run("LogWithStartAndEnd_Remote", func(t *testing.T) {

		url := testutil.ServerServe(t, filepath.Join(t.TempDir(), t.Name()))
		ctx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(ctx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		stage := repo.OpenStage(sst.DefaultTriplexMode)
		ng := stage.CreateNamedGraph("")
		node := ng.CreateIRINode("logNode")

		// Commit 10 times with unique values
		for i := 0; i < 10; i++ {
			node.AddStatementAlt(rdf.Type, rep.SchematicPort)
			_, _, err := stage.Commit(ctx, fmt.Sprintf("Remote commit #%d", i), sst.DefaultBranch)
			require.NoError(t, err)
		}

		// Get all recent 20 logs and extract logKeys
		allLogs, err := repo.Log(ctx, nil, pointerTo(-20))
		require.NoError(t, err)

		var commitLogs []sst.RepositoryLogEntry
		for _, entry := range allLogs {
			if _, ok := entry.Fields["commit_id"]; ok {
				commitLogs = append(commitLogs, entry)
			}
		}
		require.Len(t, commitLogs, 10)

		// 1) From latest, get max 3 logs
		startKey := int(commitLogs[0].LogKey)
		end := -3
		logs, err := repo.Log(ctx, pointerTo(startKey), &end)
		require.NoError(t, err)
		require.Len(t, logs, 3)

		// 2) From middle range: log[4] to log[2] (exclusive)
		startRange := int(commitLogs[2].LogKey)
		endRange := int(commitLogs[4].LogKey)
		rangeLogs, err := repo.Log(ctx, pointerTo(startRange), pointerTo(endRange))
		require.NoError(t, err)

		// Should include 4, 3 (not 2)
		require.Len(t, rangeLogs, 2)
		require.Greater(t, rangeLogs[0].LogKey, rangeLogs[1].LogKey)
	})

	t.Run("StartEqualsEnd_ShouldReturnEmpty", func(t *testing.T) {

		url := testutil.ServerServe(t, filepath.Join(t.TempDir(), t.Name()))
		ctx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(ctx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		stage := repo.OpenStage(sst.DefaultTriplexMode)
		ng := stage.CreateNamedGraph("")
		node := ng.CreateIRINode("main")
		node.AddStatement(rdf.Type, rep.SchematicPort)

		// Commit twice
		stage.Commit(ctx, "A", sst.DefaultBranch)
		node.AddStatement(rdf.Bag, rep.Angle)
		stage.Commit(ctx, "B", sst.DefaultBranch)

		logs, _ := repo.Log(ctx, nil, pointerTo(-10))
		var commitLogKeys []int
		for _, entry := range logs {
			if _, ok := entry.Fields["commit_id"]; ok {
				commitLogKeys = append(commitLogKeys, int(entry.LogKey))
			}
		}

		require.GreaterOrEqual(t, len(commitLogKeys), 2)

		// Test start==end
		start := commitLogKeys[0]
		end := start
		subset, err := repo.Log(ctx, &start, &end)
		require.NoError(t, err)
		require.Len(t, subset, 0)
	})

}
