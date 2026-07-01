// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"bufio"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/stretchr/testify/require"
)

func TestSuperRepositoryQuota(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	super, err := sst.NewLocalSuperRepository(tmpDir)
	require.NoError(t, err)
	defer super.Close()

	// By default no limits are enforced.
	repo, err := super.Create(ctx, "repo1")
	require.NoError(t, err)
	require.NotNil(t, repo)

	// Set a per-repository quota smaller than the empty repo size (impossible to use).
	const tinyQuota int64 = 1024
	err = super.SetQuota(ctx, "repo1", tinyQuota)
	require.NoError(t, err)

	q, err := super.GetQuota(ctx, "repo1")
	require.NoError(t, err)
	require.True(t, q.ActualSizeBytes > tinyQuota, "empty repo should already exceed tiny quota")
	require.Equal(t, tinyQuota, q.MaxSizeBytes)

	// DocumentSet should fail because the repo already exceeds its quota.
	data := strings.NewReader("hello world")
	_, err = repo.DocumentSet(ctx, "text/plain", bufio.NewReader(data))
	require.Error(t, err)
	require.True(t, errors.Is(err, sst.ErrQuotaExceeded))

	// Set a reasonable per-repository quota and verify DocumentSet succeeds.
	err = super.SetQuota(ctx, "repo1", 10*1024*1024)
	require.NoError(t, err)

	data = strings.NewReader("hello world")
	hash, err := repo.DocumentSet(ctx, "text/plain", bufio.NewReader(data))
	require.NoError(t, err)
	require.NotEqual(t, sst.Hash{}, hash)

	// DocumentDelete should succeed and reduce usage.
	err = repo.DocumentDelete(ctx, hash)
	require.NoError(t, err)
}

func TestSuperRepositoryMaxCount(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	super, err := sst.NewLocalSuperRepository(tmpDir)
	require.NoError(t, err)
	defer super.Close()

	// Default is unlimited.
	_, err = super.Create(ctx, "repo1")
	require.NoError(t, err)

	// Set max count to 2 (default + repo1 already == 2).
	err = super.SetMaxRepositoryCount(ctx, 2)
	require.NoError(t, err)

	_, err = super.Create(ctx, "repo2")
	require.Error(t, err)
	require.True(t, errors.Is(err, sst.ErrQuotaExceeded))

	// Increase limit and create should succeed.
	err = super.SetMaxRepositoryCount(ctx, 3)
	require.NoError(t, err)

	_, err = super.Create(ctx, "repo2")
	require.NoError(t, err)
}

func TestSuperRepositoryTotalQuota(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	super, err := sst.NewLocalSuperRepository(tmpDir)
	require.NoError(t, err)
	defer super.Close()

	// Set total quota very low so even one more repo would exceed.
	err = super.SetTotalQuota(ctx, 1024)
	require.NoError(t, err)

	// default repo already exists; creating another should fail.
	_, err = super.Create(ctx, "repo1")
	require.Error(t, err)
	require.True(t, errors.Is(err, sst.ErrQuotaExceeded))
}

func TestRepositoryInfoQuotaFields(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	super, err := sst.NewLocalSuperRepository(tmpDir)
	require.NoError(t, err)
	defer super.Close()

	repo, err := super.Create(ctx, "repo1")
	require.NoError(t, err)

	err = super.SetQuota(ctx, "repo1", 10*1024*1024)
	require.NoError(t, err)

	info, err := repo.Info(ctx, "")
	require.NoError(t, err)
	require.True(t, info.ActualRepositorySize > 0)
	require.Equal(t, int64(10*1024*1024), info.MaxRepositorySize)
}

func TestCommitQuota(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	super, err := sst.NewLocalSuperRepository(tmpDir)
	require.NoError(t, err)
	defer super.Close()

	repo, err := super.Create(ctx, "repo1")
	require.NoError(t, err)

	// Set a tiny per-repository quota so any commit will exceed it.
	err = super.SetQuota(ctx, "repo1", 1024)
	require.NoError(t, err)

	stage := repo.OpenStage(sst.DefaultTriplexMode)
	ng := stage.CreateNamedGraph(sst.IRI("urn:uuid:12345678-1234-1234-1234-123456789abc"))
	node := ng.CreateIRINode("test", lci.Individual)
	node.AddStatement(rdfs.Label, sst.String("hello"))

	_, _, err = stage.Commit(ctx, "test commit", sst.DefaultBranch)
	require.Error(t, err)
	require.True(t, errors.Is(err, sst.ErrQuotaExceeded))
}

func TestDocumentSizeCounterSurvivesReopen(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	super, err := sst.NewLocalSuperRepository(tmpDir)
	require.NoError(t, err)

	repo, err := super.Create(ctx, "repo1")
	require.NoError(t, err)

	// Upload a document while quota is generous.
	err = super.SetQuota(ctx, "repo1", 10*1024*1024)
	require.NoError(t, err)
	data := strings.NewReader("hello world")
	_, err = repo.DocumentSet(ctx, "text/plain", bufio.NewReader(data))
	require.NoError(t, err)

	infoBefore, err := repo.Info(ctx, "")
	require.NoError(t, err)
	require.True(t, infoBefore.ActualRepositorySize > 0)

	// Close and reopen the SuperRepository; the existing repo will be opened via Get.
	require.NoError(t, super.Close())
	super2, err := sst.NewLocalSuperRepository(tmpDir)
	require.NoError(t, err)
	defer super2.Close()

	repo2, err := super2.Get(ctx, "repo1")
	require.NoError(t, err)

	// The reopened repository must report the same actual size,
	// proving the document-size counter was re-initialized.
	infoAfter, err := repo2.Info(ctx, "")
	require.NoError(t, err)
	require.Equal(t, infoBefore.ActualRepositorySize, infoAfter.ActualRepositorySize)
}

func TestQuotaPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	super, err := sst.NewLocalSuperRepository(tmpDir)
	require.NoError(t, err)

	err = super.SetQuota(ctx, "repo1", 5*1024*1024)
	require.NoError(t, err)
	err = super.SetTotalQuota(ctx, 100*1024*1024)
	require.NoError(t, err)
	err = super.SetMaxRepositoryCount(ctx, 10)
	require.NoError(t, err)

	require.NoError(t, super.Close())

	// Re-open and verify persistence.
	super2, err := sst.NewLocalSuperRepository(tmpDir)
	require.NoError(t, err)
	defer super2.Close()

	q, err := super2.GetQuota(ctx, "repo1")
	require.NoError(t, err)
	require.Equal(t, int64(5*1024*1024), q.MaxSizeBytes)

	tq, err := super2.GetTotalQuota(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(100*1024*1024), tq.MaxSizeBytes)

	require.Equal(t, 10, super2.GetMaxRepositoryCount(ctx))

	// Verify quota file exists.
	_, err = os.Stat(filepath.Join(tmpDir, "super-repo-quotas.json"))
	require.NoError(t, err)
}
