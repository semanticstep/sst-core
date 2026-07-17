// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"path/filepath"
	"context"
	"testing"

	"github.com/semanticstep/sst-core/defaultderive"
	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalFullRepositoryLog(t *testing.T) {
	// Helper: Clean & init repo
	setupTestEnvironment := func(t *testing.T) sst.Repository {

		repo, err := sst.CreateLocalRepository(filepath.Join(t.TempDir(), t.Name()), "default@semanticstep.net", "default", true)
		assert.NoError(t, err)
		repo.RegisterIndexHandler(defaultderive.DeriveInfo())
		return repo
	}

	t.Run("SingleCommit_LogEntryCreated", func(t *testing.T) {
		repo := setupTestEnvironment(t)
		defer repo.Close()

		stage := repo.OpenStage(sst.DefaultTriplexMode)
		ng := stage.CreateNamedGraph("")
		main := ng.CreateIRINode("main")
		main.AddStatement(rdf.Type, rep.SchematicPort)

		_, _, err := stage.Commit(context.TODO(), "Initial commit for log test", sst.DefaultBranch)
		require.NoError(t, err)

		logs, err := repo.Log(context.TODO(), nil, nil)
		require.NoError(t, err)

		// Debug print all logs
		for _, entry := range logs {
			t.Logf("LogKey %d", entry.LogKey)
			for k, v := range entry.Fields {
				t.Logf("  %s = %s", k, v)
			}
		}

		// Only count logs with "commit_id"
		var commitEntries []sst.RepositoryLogEntry
		for _, entry := range logs {
			if _, ok := entry.Fields["commit_id"]; ok {
				commitEntries = append(commitEntries, entry)
			}
		}

		require.Len(t, commitEntries, 1, "Expected to find 1 commit log entry")
		require.NotEmpty(t, commitEntries[0].Fields["commit_id"])
	})

	t.Run("MultipleCommits_LogKeysIncrement", func(t *testing.T) {
		repo := setupTestEnvironment(t)
		defer repo.Close()

		stage := repo.OpenStage(sst.DefaultTriplexMode)
		ng := stage.CreateNamedGraph("")
		node := ng.CreateIRINode("main")
		node.AddStatement(rdf.Type, rep.SchematicPort)

		_, _, err := stage.Commit(context.TODO(), "First commit", sst.DefaultBranch)
		require.NoError(t, err)

		node.AddStatement(rdf.Bag, rep.Angle)
		_, _, err = stage.Commit(context.TODO(), "Second commit", sst.DefaultBranch)
		require.NoError(t, err)

		logs, err := repo.Log(context.TODO(), nil, nil)
		require.NoError(t, err)

		// Debug print all logs
		for _, entry := range logs {
			t.Logf("LogKey %d", entry.LogKey)
			for k, v := range entry.Fields {
				t.Logf("  %s = %s", k, v)
			}
		}

		var commits []sst.RepositoryLogEntry
		for _, entry := range logs {
			if _, ok := entry.Fields["commit_id"]; ok {
				commits = append(commits, entry)
			}
		}

		require.Len(t, commits, 2, "Expected two commit log entries")
		require.Greater(t, commits[0].LogKey, commits[1].LogKey)
		require.NotEqual(t, commits[0].Fields["commit_id"], commits[1].Fields["commit_id"])
	})

	t.Run("LimitedLogEntries", func(t *testing.T) {
		repo := setupTestEnvironment(t)
		defer repo.Close()

		stage := repo.OpenStage(sst.DefaultTriplexMode)
		ng := stage.CreateNamedGraph("")
		main := ng.CreateIRINode("main")
		main.AddStatement(rdf.Type, rep.SchematicPort)
		stage.Commit(context.TODO(), "Commit A", sst.DefaultBranch)
		main.AddStatement(rdf.Bag, rep.Angle)
		stage.Commit(context.TODO(), "Commit B", sst.DefaultBranch)

		end := -1
		logs, err := repo.Log(context.TODO(), nil, &end)
		require.NoError(t, err)

		var commits []sst.RepositoryLogEntry
		for _, entry := range logs {
			if _, ok := entry.Fields["commit_id"]; ok {
				commits = append(commits, entry)
			}
		}
		require.Len(t, commits, 1, "Should return 1 latest commit entry")
	})

	t.Run("RangeStartEnd_LogRange", func(t *testing.T) {
		repo := setupTestEnvironment(t)
		defer repo.Close()

		stage := repo.OpenStage(sst.DefaultTriplexMode)
		ng := stage.CreateNamedGraph("")
		main := ng.CreateIRINode("main")
		main.AddStatement(rdf.Type, rep.SchematicPort)

		// Commit 3 times to generate logKeys 2, 1, 0
		stage.Commit(context.TODO(), "Commit 1", sst.DefaultBranch)
		main.AddStatement(rdf.Bag, rep.Angle)
		stage.Commit(context.TODO(), "Commit 2", sst.DefaultBranch)
		main.AddStatement(rdf.Alt, rep.Angle)
		stage.Commit(context.TODO(), "Commit 3", sst.DefaultBranch)

		start := 2
		end := 0
		logs, err := repo.Log(context.TODO(), &start, &end)
		require.NoError(t, err)

		var commitLogKeys []int
		for _, entry := range logs {
			if _, ok := entry.Fields["commit_id"]; ok {
				commitLogKeys = append(commitLogKeys, int(entry.LogKey))
			}
		}

		require.Equal(t, []int{2, 1}, commitLogKeys, "Expected to return logKey 2 and 1 (exclusive of 0)")
	})

	t.Run("StartBeyondRange_ReturnsEmpty", func(t *testing.T) {
		repo := setupTestEnvironment(t)
		defer repo.Close()

		stage := repo.OpenStage(sst.DefaultTriplexMode)
		ng := stage.CreateNamedGraph("")
		ng.CreateIRINode("main").AddStatement(rdf.Type, rep.SchematicPort)
		stage.Commit(context.TODO(), "Only commit", sst.DefaultBranch)

		start := 100
		logs, err := repo.Log(context.TODO(), &start, nil)
		require.NoError(t, err)
		require.Len(t, logs, 0, "Start beyond available range should return empty result")
	})

	t.Run("StartEqualsEnd_EmptyResult", func(t *testing.T) {
		repo := setupTestEnvironment(t)
		defer repo.Close()

		stage := repo.OpenStage(sst.DefaultTriplexMode)
		ng := stage.CreateNamedGraph("")
		ng.CreateIRINode("main").AddStatement(rdf.Type, rep.SchematicPort)
		stage.Commit(context.TODO(), "Commit 1", sst.DefaultBranch)
		stage.Commit(context.TODO(), "Commit 2", sst.DefaultBranch)

		start := 1
		end := 1
		logs, err := repo.Log(context.TODO(), &start, &end)
		require.NoError(t, err)
		require.Len(t, logs, 0, "start == end should return no entries")
	})

}

