// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package mcp

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/google/uuid"
)

func testRepoDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "repo")
}

func TestSessionOpenAndListDatasets(t *testing.T) {
	dir := testRepoDir(t)
	repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
	if err != nil {
		t.Fatalf("CreateLocalRepository: %v", err)
	}
	if err := repo.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	sess := NewSession()
	t.Cleanup(func() {
		if err := sess.CloseAll(); err != nil {
			t.Errorf("CloseAll: %v", err)
		}
	})
	id, err := sess.OpenLocalRepository(dir, "")
	if err != nil {
		t.Fatalf("OpenLocalRepository: %v", err)
	}
	if id != "r1" {
		t.Fatalf("repo alias = %q, want r1", id)
	}

	st := sess.Status()
	if len(st.Repositories) != 1 || st.Repositories[0].RepoAlias != "r1" || st.Repositories[0].Type != "local" {
		t.Fatalf("Status() = %+v, want one local repo r1", st)
	}

	datasets, err := sess.ListDatasets(id)
	if err != nil {
		t.Fatalf("ListDatasets: %v", err)
	}
	if len(datasets) != 0 {
		t.Fatalf("datasets = %v, want empty", datasets)
	}
}

func TestSessionDuplicateAlias(t *testing.T) {
	dir := testRepoDir(t)
	repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
	if err != nil {
		t.Fatalf("CreateLocalRepository: %v", err)
	}
	if err := repo.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	sess := NewSession()
	t.Cleanup(func() {
		if err := sess.CloseAll(); err != nil {
			t.Errorf("CloseAll: %v", err)
		}
	})
	if _, err := sess.OpenLocalRepository(dir, "r1"); err != nil {
		t.Fatalf("first open: %v", err)
	}
	if _, err := sess.OpenLocalRepository(dir, "r1"); err == nil {
		t.Fatal("expected error for duplicate alias")
	}
}

func TestSessionCloseRepository(t *testing.T) {
	dir := testRepoDir(t)
	repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
	if err != nil {
		t.Fatalf("CreateLocalRepository: %v", err)
	}
	if err := repo.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	sess := NewSession()
	id, err := sess.OpenLocalRepository(dir, "r1")
	if err != nil {
		t.Fatalf("OpenLocalRepository: %v", err)
	}
	if id != "r1" {
		t.Fatalf("repo alias = %q, want r1", id)
	}

	if err := sess.CloseRepository("r1"); err != nil {
		t.Fatalf("CloseRepository: %v", err)
	}
	if st := sess.Status(); len(st.Repositories) != 0 {
		t.Fatalf("Status() after close = %+v, want empty", st)
	}
	if err := sess.CloseRepository("r1"); err == nil {
		t.Fatal("expected error closing unknown repo")
	}
}

func createRepoWithDataset(t *testing.T, dir string) string {
	t.Helper()

	repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
	if err != nil {
		t.Fatalf("CreateLocalRepository: %v", err)
	}
	defer repo.Close()

	ngID := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363")
	stage := repo.OpenStage(sst.DefaultTriplexMode)
	ng := stage.CreateNamedGraph(sst.IRI(ngID.URN()))
	main := ng.CreateIRINode("main")
	main.AddStatement(rdf.Type, rep.SchematicPort)

	if _, _, err := stage.Commit(context.Background(), "init", sst.DefaultBranch); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	return ngID.URN()
}

func TestSessionOpenDataset(t *testing.T) {
	dir := testRepoDir(t)
	iri := createRepoWithDataset(t, dir)

	sess := NewSession()
	t.Cleanup(func() {
		if err := sess.CloseAll(); err != nil {
			t.Errorf("CloseAll: %v", err)
		}
	})

	repoAlias, err := sess.OpenLocalRepository(dir, "r1")
	if err != nil {
		t.Fatalf("OpenLocalRepository: %v", err)
	}
	if repoAlias != "r1" {
		t.Fatalf("repo alias = %q, want r1", repoAlias)
	}

	datasetAlias, err := sess.OpenDataset("r1", iri, "")
	if err != nil {
		t.Fatalf("OpenDataset: %v", err)
	}
	if datasetAlias != "d1" {
		t.Fatalf("dataset alias = %q, want d1", datasetAlias)
	}

	st := sess.Status()
	if len(st.Datasets) != 1 {
		t.Fatalf("Status().Datasets = %+v, want one dataset", st.Datasets)
	}
	if st.Datasets[0].DatasetAlias != "d1" || st.Datasets[0].RepoAlias != "r1" || st.Datasets[0].IRI != iri {
		t.Fatalf("Status().Datasets[0] = %+v, want d1/r1/%s", st.Datasets[0], iri)
	}

	ds, err := sess.Dataset("d1")
	if err != nil {
		t.Fatalf("Dataset: %v", err)
	}
	if ds.IRI().String() != iri {
		t.Fatalf("dataset IRI = %q, want %q", ds.IRI(), iri)
	}

	if _, err := sess.OpenDataset("r1", iri, "d1"); err == nil {
		t.Fatal("expected error for duplicate dataset alias")
	}

	_, err = sess.OpenDataset("r1", "urn:uuid:00000000-0000-0000-0000-000000000000", "")
	if err == nil {
		t.Fatal("expected error for unknown dataset IRI")
	}
	if !errors.Is(err, sst.ErrDatasetNotFound) {
		t.Fatalf("unknown IRI error = %v, want ErrDatasetNotFound", err)
	}

	if _, err := sess.OpenDataset("missing", iri, ""); err == nil {
		t.Fatal("expected error for unknown repo")
	}

	if _, err := sess.OpenDataset("r1", "", ""); err == nil {
		t.Fatal("expected error for empty IRI")
	}

	datasetAlias2, err := sess.OpenDataset("r1", iri, "myds")
	if err != nil {
		t.Fatalf("OpenDataset with explicit alias: %v", err)
	}
	if datasetAlias2 != "myds" {
		t.Fatalf("dataset alias = %q, want myds", datasetAlias2)
	}

	if err := sess.CloseRepository("r1"); err != nil {
		t.Fatalf("CloseRepository: %v", err)
	}
	if len(sess.Status().Datasets) != 0 {
		t.Fatalf("Status().Datasets after repo close = %+v, want empty", sess.Status().Datasets)
	}
}

func TestSessionListDocuments(t *testing.T) {
	dir := testRepoDir(t)
	repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
	if err != nil {
		t.Fatalf("CreateLocalRepository: %v", err)
	}
	if err := repo.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	sess := NewSession()
	t.Cleanup(func() {
		if err := sess.CloseAll(); err != nil {
			t.Errorf("CloseAll: %v", err)
		}
	})

	repoAlias, err := sess.OpenLocalRepository(dir, "r1")
	if err != nil {
		t.Fatalf("OpenLocalRepository: %v", err)
	}

	docs, err := sess.ListDocuments(repoAlias)
	if err != nil {
		t.Fatalf("ListDocuments empty repo: %v", err)
	}
	if len(docs) != 0 {
		t.Fatalf("documents = %v, want empty", docs)
	}

	uploadPath := filepath.Join(t.TempDir(), "list.txt")
	if err := os.WriteFile(uploadPath, []byte("hello document list"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	uploadResult, err := sess.SetDocument(repoAlias, uploadPath)
	if err != nil {
		t.Fatalf("SetDocument: %v", err)
	}
	hash, _ := uploadResult["hash"].(string)
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}

	docs, err = sess.ListDocuments(repoAlias)
	if err != nil {
		t.Fatalf("ListDocuments after upload: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("documents = %v, want one", docs)
	}
	if docs[0]["hash"] != hash {
		t.Fatalf("document hash = %v, want %s", docs[0]["hash"], hash)
	}
	if docs[0]["mime_type"] != "text/plain" {
		t.Fatalf("document mime_type = %v, want text/plain", docs[0]["mime_type"])
	}

	if _, err := sess.ListDocuments("missing"); err == nil {
		t.Fatal("expected error for unknown repo")
	}
}

func TestSessionSetDocument(t *testing.T) {
	dir := testRepoDir(t)
	repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
	if err != nil {
		t.Fatalf("CreateLocalRepository: %v", err)
	}
	if err := repo.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	sess := NewSession()
	t.Cleanup(func() {
		if err := sess.CloseAll(); err != nil {
			t.Errorf("CloseAll: %v", err)
		}
	})

	repoAlias, err := sess.OpenLocalRepository(dir, "r1")
	if err != nil {
		t.Fatalf("OpenLocalRepository: %v", err)
	}

	filePath := filepath.Join(t.TempDir(), "upload.txt")
	if err := os.WriteFile(filePath, []byte("upload test content"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	result, err := sess.SetDocument(repoAlias, filePath)
	if err != nil {
		t.Fatalf("SetDocument: %v", err)
	}
	if result["deduplicated"] != false {
		t.Fatalf("deduplicated = %v, want false", result["deduplicated"])
	}
	if result["mime_type"] != "text/plain" {
		t.Fatalf("mime_type = %v, want text/plain", result["mime_type"])
	}
	hash, _ := result["hash"].(string)
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}

	result2, err := sess.SetDocument(repoAlias, filePath)
	if err != nil {
		t.Fatalf("SetDocument second time: %v", err)
	}
	if result2["deduplicated"] != true {
		t.Fatalf("deduplicated = %v, want true", result2["deduplicated"])
	}
	if result2["hash"] != hash {
		t.Fatalf("hash = %v, want %s", result2["hash"], hash)
	}

	docs, err := sess.ListDocuments(repoAlias)
	if err != nil {
		t.Fatalf("ListDocuments: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("documents = %v, want one", docs)
	}

	if _, err := sess.SetDocument("missing", filePath); err == nil {
		t.Fatal("expected error for unknown repo")
	}
	if _, err := sess.SetDocument(repoAlias, ""); err == nil {
		t.Fatal("expected error for empty file_path")
	}
}

func TestSessionGetDocument(t *testing.T) {
	dir := testRepoDir(t)
	repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
	if err != nil {
		t.Fatalf("CreateLocalRepository: %v", err)
	}
	if err := repo.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	sess := NewSession()
	t.Cleanup(func() {
		if err := sess.CloseAll(); err != nil {
			t.Errorf("CloseAll: %v", err)
		}
	})

	repoAlias, err := sess.OpenLocalRepository(dir, "r1")
	if err != nil {
		t.Fatalf("OpenLocalRepository: %v", err)
	}

	uploadPath := filepath.Join(t.TempDir(), "source.txt")
	content := []byte("download test content")
	if err := os.WriteFile(uploadPath, content, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	uploadResult, err := sess.SetDocument(repoAlias, uploadPath)
	if err != nil {
		t.Fatalf("SetDocument: %v", err)
	}
	hash, _ := uploadResult["hash"].(string)

	outDir := t.TempDir()
	result, err := sess.GetDocument(repoAlias, hash, outDir)
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}

	outputPath, _ := result["output_path"].(string)
	if outputPath == "" {
		t.Fatal("expected non-empty output_path")
	}
	got, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("content = %q, want %q", got, content)
	}
	if result["mime_type"] != "text/plain" {
		t.Fatalf("mime_type = %v, want text/plain", result["mime_type"])
	}

	if _, err := sess.GetDocument(repoAlias, "invalid-hash", outDir); err == nil {
		t.Fatal("expected error for invalid hash")
	}
	if _, err := sess.GetDocument("missing", hash, outDir); err == nil {
		t.Fatal("expected error for unknown repo")
	}
}

func TestSessionDocumentInfo(t *testing.T) {
	dir := testRepoDir(t)
	repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
	if err != nil {
		t.Fatalf("CreateLocalRepository: %v", err)
	}
	if err := repo.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	sess := NewSession()
	t.Cleanup(func() {
		if err := sess.CloseAll(); err != nil {
			t.Errorf("CloseAll: %v", err)
		}
	})

	repoAlias, err := sess.OpenLocalRepository(dir, "r1")
	if err != nil {
		t.Fatalf("OpenLocalRepository: %v", err)
	}

	uploadPath := filepath.Join(t.TempDir(), "info.txt")
	if err := os.WriteFile(uploadPath, []byte("document info test"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	uploadResult, err := sess.SetDocument(repoAlias, uploadPath)
	if err != nil {
		t.Fatalf("SetDocument: %v", err)
	}
	hash, _ := uploadResult["hash"].(string)

	info, err := sess.DocumentInfo(repoAlias, hash)
	if err != nil {
		t.Fatalf("DocumentInfo: %v", err)
	}
	if info["hash"] != hash {
		t.Fatalf("hash = %v, want %s", info["hash"], hash)
	}
	if info["mime_type"] != "text/plain" {
		t.Fatalf("mime_type = %v, want text/plain", info["mime_type"])
	}
	if info["size"] == nil || info["size"].(int64) <= 0 {
		t.Fatalf("size = %v, want positive", info["size"])
	}

	if _, err := sess.DocumentInfo(repoAlias, "invalid-hash"); err == nil {
		t.Fatal("expected error for invalid hash")
	}
	if _, err := sess.DocumentInfo("missing", hash); err == nil {
		t.Fatal("expected error for unknown repo")
	}
}

func TestSessionDeleteDocument(t *testing.T) {
	dir := testRepoDir(t)
	repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
	if err != nil {
		t.Fatalf("CreateLocalRepository: %v", err)
	}
	if err := repo.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	sess := NewSession()
	t.Cleanup(func() {
		if err := sess.CloseAll(); err != nil {
			t.Errorf("CloseAll: %v", err)
		}
	})

	repoAlias, err := sess.OpenLocalRepository(dir, "r1")
	if err != nil {
		t.Fatalf("OpenLocalRepository: %v", err)
	}

	uploadPath := filepath.Join(t.TempDir(), "delete.txt")
	if err := os.WriteFile(uploadPath, []byte("delete test content"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	uploadResult, err := sess.SetDocument(repoAlias, uploadPath)
	if err != nil {
		t.Fatalf("SetDocument: %v", err)
	}
	hash, _ := uploadResult["hash"].(string)

	docs, err := sess.ListDocuments(repoAlias)
	if err != nil {
		t.Fatalf("ListDocuments before delete: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("documents before delete = %v, want one", docs)
	}

	if err := sess.DeleteDocument(repoAlias, hash); err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}

	docs, err = sess.ListDocuments(repoAlias)
	if err != nil {
		t.Fatalf("ListDocuments after delete: %v", err)
	}
	if len(docs) != 0 {
		t.Fatalf("documents after delete = %v, want empty", docs)
	}

	if _, err := sess.DocumentInfo(repoAlias, hash); err == nil {
		t.Fatal("expected error getting info for deleted document")
	}

	if err := sess.DeleteDocument(repoAlias, "invalid-hash"); err == nil {
		t.Fatal("expected error for invalid hash")
	}
	if err := sess.DeleteDocument("missing", hash); err == nil {
		t.Fatal("expected error for unknown repo")
	}
}

func TestSessionBranches(t *testing.T) {
	dir := testRepoDir(t)
	iri := createRepoWithDataset(t, dir)

	sess := NewSession()
	t.Cleanup(func() {
		if err := sess.CloseAll(); err != nil {
			t.Errorf("CloseAll: %v", err)
		}
	})

	if _, err := sess.OpenLocalRepository(dir, "r1"); err != nil {
		t.Fatalf("OpenLocalRepository: %v", err)
	}
	if _, err := sess.OpenDataset("r1", iri, "d1"); err != nil {
		t.Fatalf("OpenDataset: %v", err)
	}

	branches, err := sess.Branches("d1")
	if err != nil {
		t.Fatalf("Branches: %v", err)
	}
	hash, ok := branches[sst.DefaultBranch]
	if !ok {
		t.Fatalf("branches = %v, want %q", branches, sst.DefaultBranch)
	}
	if hash == "" {
		t.Fatal("expected non-empty commit hash for default branch")
	}

	if _, err := sess.Branches("missing"); err == nil {
		t.Fatal("expected error for unknown dataset")
	}
}

func TestSessionListCommits(t *testing.T) {
	dir := testRepoDir(t)
	iri := createRepoWithDataset(t, dir)

	sess := NewSession()
	t.Cleanup(func() {
		if err := sess.CloseAll(); err != nil {
			t.Errorf("CloseAll: %v", err)
		}
	})

	if _, err := sess.OpenLocalRepository(dir, "r1"); err != nil {
		t.Fatalf("OpenLocalRepository: %v", err)
	}
	if _, err := sess.OpenDataset("r1", iri, "d1"); err != nil {
		t.Fatalf("OpenDataset: %v", err)
	}

	commits, err := sess.ListCommits("d1", false)
	if err != nil {
		t.Fatalf("ListCommits: %v", err)
	}
	if len(commits) == 0 {
		t.Fatal("expected at least one commit")
	}
	hash, ok := commits[0].(string)
	if !ok || hash == "" {
		t.Fatalf("commits[0] = %#v, want non-empty hash string", commits[0])
	}

	detailed, err := sess.ListCommits("d1", true)
	if err != nil {
		t.Fatalf("ListCommits details: %v", err)
	}
	if len(detailed) == 0 {
		t.Fatal("expected at least one detailed commit")
	}
	detail, ok := detailed[0].(map[string]any)
	if !ok {
		t.Fatalf("detailed[0] = %#v, want map", detailed[0])
	}
	if detail["commit"] != hash {
		t.Fatalf("detail commit = %v, want %s", detail["commit"], hash)
	}
	if detail["message"] != "init" {
		t.Fatalf("detail message = %v, want init", detail["message"])
	}

	if _, err := sess.ListCommits("missing", false); err == nil {
		t.Fatal("expected error for unknown dataset")
	}
}

func TestSessionCommitDetailsByHash(t *testing.T) {
	dir := testRepoDir(t)
	iri := createRepoWithDataset(t, dir)

	sess := NewSession()
	t.Cleanup(func() {
		if err := sess.CloseAll(); err != nil {
			t.Errorf("CloseAll: %v", err)
		}
	})

	if _, err := sess.OpenLocalRepository(dir, "r1"); err != nil {
		t.Fatalf("OpenLocalRepository: %v", err)
	}
	if _, err := sess.OpenDataset("r1", iri, "d1"); err != nil {
		t.Fatalf("OpenDataset: %v", err)
	}

	commits, err := sess.ListCommits("d1", false)
	if err != nil {
		t.Fatalf("ListCommits: %v", err)
	}
	if len(commits) == 0 {
		t.Fatal("expected at least one commit")
	}
	hash, ok := commits[0].(string)
	if !ok || hash == "" {
		t.Fatalf("commits[0] = %#v, want non-empty hash string", commits[0])
	}

	details, err := sess.CommitDetailsByHash("d1", hash)
	if err != nil {
		t.Fatalf("CommitDetailsByHash: %v", err)
	}
	if details["commit"] != hash {
		t.Fatalf("commit = %v, want %s", details["commit"], hash)
	}
	if details["message"] != "init" {
		t.Fatalf("message = %v, want init", details["message"])
	}
	if details["author"] == "" {
		t.Fatal("expected non-empty author")
	}

	if _, err := sess.CommitDetailsByHash("d1", "not-a-valid-hash"); err == nil {
		t.Fatal("expected error for invalid hash")
	}
	if _, err := sess.CommitDetailsByHash("d1", ""); err == nil {
		t.Fatal("expected error for empty hash")
	}
	if _, err := sess.CommitDetailsByHash("missing", hash); err == nil {
		t.Fatal("expected error for unknown dataset")
	}
}

func TestSessionCommitDetailsByBranch(t *testing.T) {
	dir := testRepoDir(t)
	iri := createRepoWithDataset(t, dir)

	sess := NewSession()
	t.Cleanup(func() {
		if err := sess.CloseAll(); err != nil {
			t.Errorf("CloseAll: %v", err)
		}
	})

	if _, err := sess.OpenLocalRepository(dir, "r1"); err != nil {
		t.Fatalf("OpenLocalRepository: %v", err)
	}
	if _, err := sess.OpenDataset("r1", iri, "d1"); err != nil {
		t.Fatalf("OpenDataset: %v", err)
	}

	details, err := sess.CommitDetailsByBranch("d1", sst.DefaultBranch)
	if err != nil {
		t.Fatalf("CommitDetailsByBranch: %v", err)
	}
	if details["commit"] == "" || details["commit"] == nil {
		t.Fatalf("commit = %#v, want non-empty hash", details["commit"])
	}
	if details["message"] != "init" {
		t.Fatalf("message = %v, want init", details["message"])
	}
	if details["author"] == "" {
		t.Fatal("expected non-empty author")
	}

	if _, err := sess.CommitDetailsByBranch("d1", ""); err == nil {
		t.Fatal("expected error for empty branch")
	}
	if _, err := sess.CommitDetailsByBranch("missing", sst.DefaultBranch); err == nil {
		t.Fatal("expected error for unknown dataset")
	}
}

func TestSessionSyncFrom(t *testing.T) {
	sourceDir := filepath.Join(t.TempDir(), "source")
	targetDir := filepath.Join(t.TempDir(), "target")
	iri := createRepoWithDataset(t, sourceDir)

	targetRepo, err := sst.CreateLocalRepository(targetDir, "default@semanticstep.net", "default", true)
	if err != nil {
		t.Fatalf("CreateLocalRepository target: %v", err)
	}
	if err := targetRepo.Close(); err != nil {
		t.Fatalf("Close target: %v", err)
	}

	sess := NewSession()
	t.Cleanup(func() {
		if err := sess.CloseAll(); err != nil {
			t.Errorf("CloseAll: %v", err)
		}
	})

	if _, err := sess.OpenLocalRepository(targetDir, "r1"); err != nil {
		t.Fatalf("OpenLocalRepository target: %v", err)
	}
	if _, err := sess.OpenLocalRepository(sourceDir, "r2"); err != nil {
		t.Fatalf("OpenLocalRepository source: %v", err)
	}

	result, err := sess.SyncFrom("r1", "r2", "", nil)
	if err != nil {
		t.Fatalf("SyncFrom: %v", err)
	}
	if result["message"] != "sync completed" {
		t.Fatalf("message = %v, want sync completed", result["message"])
	}

	datasets, err := sess.ListDatasets("r1")
	if err != nil {
		t.Fatalf("ListDatasets: %v", err)
	}
	if len(datasets) != 1 || datasets[0] != iri {
		t.Fatalf("datasets = %v, want [%s]", datasets, iri)
	}

	if _, err := sess.SyncFrom("r1", "r1", "", nil); err == nil {
		t.Fatal("expected error syncing repo to itself")
	}
	if _, err := sess.SyncFrom("missing", "r2", "", nil); err == nil {
		t.Fatal("expected error for unknown target repo")
	}
	if _, err := sess.SyncFrom("r1", "missing", "", nil); err == nil {
		t.Fatal("expected error for unknown source repo")
	}
}

func TestSessionHistory(t *testing.T) {
	dir := testRepoDir(t)
	iri := createRepoWithDataset(t, dir)

	sess := NewSession()
	t.Cleanup(func() {
		if err := sess.CloseAll(); err != nil {
			t.Errorf("CloseAll: %v", err)
		}
	})

	if _, err := sess.OpenLocalRepository(dir, "r1"); err != nil {
		t.Fatalf("OpenLocalRepository: %v", err)
	}
	if _, err := sess.OpenDataset("r1", iri, "d1"); err != nil {
		t.Fatalf("OpenDataset: %v", err)
	}

	commits, err := sess.History("d1")
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(commits) == 0 {
		t.Fatal("expected at least one commit in history")
	}
	first := commits[0]
	if first["commit"] == "" || first["commit"] == nil {
		t.Fatalf("history[0] = %#v, want commit hash", first)
	}
	if first["message"] != "init" {
		t.Fatalf("message = %v, want init", first["message"])
	}
	branches, ok := first["branches"].([]string)
	if !ok || len(branches) == 0 {
		t.Fatalf("branches = %#v, want default branch tip", first["branches"])
	}
	foundDefault := false
	for _, b := range branches {
		if b == sst.DefaultBranch {
			foundDefault = true
			break
		}
	}
	if !foundDefault {
		t.Fatalf("branches = %v, want %q", branches, sst.DefaultBranch)
	}

	if _, err := sess.History("missing"); err == nil {
		t.Fatal("expected error for unknown dataset")
	}
}

func TestSessionStagesLifecycle(t *testing.T) {
	dir := testRepoDir(t)
	repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
	if err != nil {
		t.Fatalf("CreateLocalRepository: %v", err)
	}
	if err := repo.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	sess := NewSession()
	t.Cleanup(func() {
		if err := sess.CloseAll(); err != nil {
			t.Errorf("CloseAll: %v", err)
		}
	})

	if _, err := sess.OpenLocalRepository(dir, "r1"); err != nil {
		t.Fatalf("OpenLocalRepository: %v", err)
	}
	opened, err := sess.Repository("r1")
	if err != nil {
		t.Fatalf("Repository: %v", err)
	}

	id, autoID, err := sess.reserveStageAlias("")
	if err != nil {
		t.Fatalf("reserveStageAlias: %v", err)
	}
	if id != "s1" || !autoID {
		t.Fatalf("reserveStageAlias = %q autoID=%v, want s1 true", id, autoID)
	}
	stage := opened.OpenStage(sst.DefaultTriplexMode)
	sess.commitStage(id, autoID, "r1", stage, stageMeta{Branch: sst.DefaultBranch, Commit: "abc"})

	st := sess.Status()
	if len(st.Stages) != 1 {
		t.Fatalf("Status().Stages = %+v, want one stage", st.Stages)
	}
	if st.Stages[0].StageAlias != "s1" || st.Stages[0].RepoAlias != "r1" {
		t.Fatalf("Status().Stages[0] = %+v, want s1/r1", st.Stages[0])
	}
	if st.Stages[0].Branch != sst.DefaultBranch || st.Stages[0].Commit != "abc" {
		t.Fatalf("Status().Stages[0] meta = %+v, want branch/commit", st.Stages[0])
	}

	got, err := sess.Stage("s1")
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil stage")
	}

	if _, _, err := sess.reserveStageAlias("s1"); err == nil {
		t.Fatal("expected error for duplicate stage alias")
	}
	if _, err := sess.Stage("missing"); err == nil {
		t.Fatal("expected error for unknown stage")
	}

	if err := sess.CloseRepository("r1"); err != nil {
		t.Fatalf("CloseRepository: %v", err)
	}
	if len(sess.Status().Stages) != 0 {
		t.Fatalf("Status().Stages after repo close = %+v, want empty", sess.Status().Stages)
	}
}

func TestSessionCheckoutBranch(t *testing.T) {
	dir := testRepoDir(t)
	iri := createRepoWithDataset(t, dir)

	sess := NewSession()
	t.Cleanup(func() {
		if err := sess.CloseAll(); err != nil {
			t.Errorf("CloseAll: %v", err)
		}
	})

	if _, err := sess.OpenLocalRepository(dir, "r1"); err != nil {
		t.Fatalf("OpenLocalRepository: %v", err)
	}
	if _, err := sess.OpenDataset("r1", iri, "d1"); err != nil {
		t.Fatalf("OpenDataset: %v", err)
	}

	stageAlias, err := sess.CheckoutBranch("d1", sst.DefaultBranch, "")
	if err != nil {
		t.Fatalf("CheckoutBranch: %v", err)
	}
	if stageAlias != "s1" {
		t.Fatalf("stage alias = %q, want s1", stageAlias)
	}

	st := sess.Status()
	if len(st.Stages) != 1 {
		t.Fatalf("Status().Stages = %+v, want one stage", st.Stages)
	}
	if st.Stages[0].StageAlias != "s1" || st.Stages[0].RepoAlias != "r1" {
		t.Fatalf("Status().Stages[0] = %+v, want s1/r1", st.Stages[0])
	}
	if st.Stages[0].Branch != sst.DefaultBranch {
		t.Fatalf("branch = %q, want %q", st.Stages[0].Branch, sst.DefaultBranch)
	}

	stage, err := sess.Stage("s1")
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if stage == nil {
		t.Fatal("expected non-nil stage")
	}

	if _, err := sess.CheckoutBranch("d1", sst.DefaultBranch, "s1"); err == nil {
		t.Fatal("expected error for duplicate stage alias")
	}
	if _, err := sess.CheckoutBranch("d1", "", ""); err == nil {
		t.Fatal("expected error for empty branch")
	}
	if _, err := sess.CheckoutBranch("missing", sst.DefaultBranch, ""); err == nil {
		t.Fatal("expected error for unknown dataset")
	}

	stageAlias2, err := sess.CheckoutBranch("d1", sst.DefaultBranch, "mystage")
	if err != nil {
		t.Fatalf("CheckoutBranch explicit alias: %v", err)
	}
	if stageAlias2 != "mystage" {
		t.Fatalf("stage alias = %q, want mystage", stageAlias2)
	}
}

func TestSessionCheckoutCommit(t *testing.T) {
	dir := testRepoDir(t)
	iri := createRepoWithDataset(t, dir)

	sess := NewSession()
	t.Cleanup(func() {
		if err := sess.CloseAll(); err != nil {
			t.Errorf("CloseAll: %v", err)
		}
	})

	if _, err := sess.OpenLocalRepository(dir, "r1"); err != nil {
		t.Fatalf("OpenLocalRepository: %v", err)
	}
	if _, err := sess.OpenDataset("r1", iri, "d1"); err != nil {
		t.Fatalf("OpenDataset: %v", err)
	}

	commits, err := sess.ListCommits("d1", false)
	if err != nil {
		t.Fatalf("ListCommits: %v", err)
	}
	if len(commits) == 0 {
		t.Fatal("expected at least one commit")
	}
	hash, ok := commits[0].(string)
	if !ok || hash == "" {
		t.Fatalf("commits[0] = %#v, want non-empty hash string", commits[0])
	}

	stageAlias, err := sess.CheckoutCommit("d1", hash, "")
	if err != nil {
		t.Fatalf("CheckoutCommit: %v", err)
	}
	if stageAlias != "s1" {
		t.Fatalf("stage alias = %q, want s1", stageAlias)
	}

	st := sess.Status()
	if len(st.Stages) != 1 {
		t.Fatalf("Status().Stages = %+v, want one stage", st.Stages)
	}
	if st.Stages[0].StageAlias != "s1" || st.Stages[0].RepoAlias != "r1" {
		t.Fatalf("Status().Stages[0] = %+v, want s1/r1", st.Stages[0])
	}
	if st.Stages[0].Commit != hash {
		t.Fatalf("commit = %q, want %q", st.Stages[0].Commit, hash)
	}

	if _, err := sess.Stage("s1"); err != nil {
		t.Fatalf("Stage: %v", err)
	}

	if _, err := sess.CheckoutCommit("d1", hash, "s1"); err == nil {
		t.Fatal("expected error for duplicate stage alias")
	}
	if _, err := sess.CheckoutCommit("d1", "", ""); err == nil {
		t.Fatal("expected error for empty hash")
	}
	if _, err := sess.CheckoutCommit("d1", "not-a-valid-hash", ""); err == nil {
		t.Fatal("expected error for invalid hash")
	}
	if _, err := sess.CheckoutCommit("missing", hash, ""); err == nil {
		t.Fatal("expected error for unknown dataset")
	}
}

func TestSessionOpenStage(t *testing.T) {
	dir := testRepoDir(t)
	repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
	if err != nil {
		t.Fatalf("CreateLocalRepository: %v", err)
	}
	if err := repo.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	sess := NewSession()
	t.Cleanup(func() {
		if err := sess.CloseAll(); err != nil {
			t.Errorf("CloseAll: %v", err)
		}
	})

	if _, err := sess.OpenLocalRepository(dir, "r1"); err != nil {
		t.Fatalf("OpenLocalRepository: %v", err)
	}

	stageAlias, err := sess.OpenStage("r1", "")
	if err != nil {
		t.Fatalf("OpenStage: %v", err)
	}
	if stageAlias != "s1" {
		t.Fatalf("stage alias = %q, want s1", stageAlias)
	}

	st := sess.Status()
	if len(st.Stages) != 1 {
		t.Fatalf("Status().Stages = %+v, want one stage", st.Stages)
	}
	if st.Stages[0].StageAlias != "s1" || st.Stages[0].RepoAlias != "r1" {
		t.Fatalf("Status().Stages[0] = %+v, want s1/r1", st.Stages[0])
	}
	if st.Stages[0].Branch != "" || st.Stages[0].Commit != "" {
		t.Fatalf("empty stage should have no branch/commit meta, got %+v", st.Stages[0])
	}

	if _, err := sess.Stage("s1"); err != nil {
		t.Fatalf("Stage: %v", err)
	}

	if _, err := sess.OpenStage("r1", "s1"); err == nil {
		t.Fatal("expected error for duplicate stage alias")
	}
	if _, err := sess.OpenStage("missing", ""); err == nil {
		t.Fatal("expected error for unknown repo")
	}

	stageAlias2, err := sess.OpenStage("r1", "empty")
	if err != nil {
		t.Fatalf("OpenStage explicit alias: %v", err)
	}
	if stageAlias2 != "empty" {
		t.Fatalf("stage alias = %q, want empty", stageAlias2)
	}
}

func TestSessionInfo(t *testing.T) {
	dir := testRepoDir(t)
	iri := createRepoWithDataset(t, dir)

	sess := NewSession()
	t.Cleanup(func() {
		if err := sess.CloseAll(); err != nil {
			t.Errorf("CloseAll: %v", err)
		}
	})

	if _, err := sess.OpenLocalRepository(dir, "r1"); err != nil {
		t.Fatalf("OpenLocalRepository: %v", err)
	}
	if _, err := sess.OpenDataset("r1", iri, "d1"); err != nil {
		t.Fatalf("OpenDataset: %v", err)
	}

	stageAlias, err := sess.CheckoutBranch("d1", sst.DefaultBranch, "")
	if err != nil {
		t.Fatalf("CheckoutBranch: %v", err)
	}

	info, err := sess.Info(stageAlias)
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info["stage_alias"] != stageAlias {
		t.Fatalf("stage_alias = %v, want %s", info["stage_alias"], stageAlias)
	}
	if info["repo_alias"] != "r1" {
		t.Fatalf("repo_alias = %v, want r1", info["repo_alias"])
	}
	if info["branch"] != sst.DefaultBranch {
		t.Fatalf("branch = %v, want %q", info["branch"], sst.DefaultBranch)
	}
	localCount, ok := info["number_of_local_graphs"].(int)
	if !ok || localCount < 1 {
		t.Fatalf("number_of_local_graphs = %#v, want >= 1", info["number_of_local_graphs"])
	}
	localGraphs, ok := info["local_graphs"].([]map[string]any)
	if !ok || len(localGraphs) < 1 {
		t.Fatalf("local_graphs = %#v, want non-empty", info["local_graphs"])
	}
	if localGraphs[0]["iri"] != iri {
		t.Fatalf("local_graphs[0].iri = %v, want %s", localGraphs[0]["iri"], iri)
	}

	emptyAlias, err := sess.OpenStage("r1", "empty")
	if err != nil {
		t.Fatalf("OpenStage: %v", err)
	}
	emptyInfo, err := sess.Info(emptyAlias)
	if err != nil {
		t.Fatalf("Info empty stage: %v", err)
	}
	if emptyInfo["number_of_local_graphs"] != 0 {
		t.Fatalf("empty stage local graphs = %v, want 0", emptyInfo["number_of_local_graphs"])
	}

	if _, err := sess.Info("missing"); err == nil {
		t.Fatal("expected error for unknown stage")
	}
}

func TestSessionNamedGraphs(t *testing.T) {
	dir := testRepoDir(t)
	iri := createRepoWithDataset(t, dir)

	sess := NewSession()
	t.Cleanup(func() {
		if err := sess.CloseAll(); err != nil {
			t.Errorf("CloseAll: %v", err)
		}
	})

	if _, err := sess.OpenLocalRepository(dir, "r1"); err != nil {
		t.Fatalf("OpenLocalRepository: %v", err)
	}
	if _, err := sess.OpenDataset("r1", iri, "d1"); err != nil {
		t.Fatalf("OpenDataset: %v", err)
	}

	stageAlias, err := sess.CheckoutBranch("d1", sst.DefaultBranch, "")
	if err != nil {
		t.Fatalf("CheckoutBranch: %v", err)
	}

	graphs, err := sess.NamedGraphs(stageAlias)
	if err != nil {
		t.Fatalf("NamedGraphs: %v", err)
	}
	if len(graphs) != 1 {
		t.Fatalf("named graphs = %v, want one", graphs)
	}
	if graphs[0] != iri {
		t.Fatalf("named graph IRI = %q, want %q", graphs[0], iri)
	}

	emptyAlias, err := sess.OpenStage("r1", "empty")
	if err != nil {
		t.Fatalf("OpenStage: %v", err)
	}
	emptyGraphs, err := sess.NamedGraphs(emptyAlias)
	if err != nil {
		t.Fatalf("NamedGraphs empty: %v", err)
	}
	if len(emptyGraphs) != 0 {
		t.Fatalf("empty stage graphs = %v, want empty", emptyGraphs)
	}

	if _, err := sess.NamedGraphs("missing"); err == nil {
		t.Fatal("expected error for unknown stage")
	}
}

func TestSessionValidate(t *testing.T) {
	dir := testRepoDir(t)
	iri := createRepoWithDataset(t, dir)

	sess := NewSession()
	t.Cleanup(func() {
		if err := sess.CloseAll(); err != nil {
			t.Errorf("CloseAll: %v", err)
		}
	})

	if _, err := sess.OpenLocalRepository(dir, "r1"); err != nil {
		t.Fatalf("OpenLocalRepository: %v", err)
	}
	if _, err := sess.OpenDataset("r1", iri, "d1"); err != nil {
		t.Fatalf("OpenDataset: %v", err)
	}

	stageAlias, err := sess.CheckoutBranch("d1", sst.DefaultBranch, "")
	if err != nil {
		t.Fatalf("CheckoutBranch: %v", err)
	}

	result, err := sess.Validate(stageAlias, "")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if result["stage_alias"] != stageAlias {
		t.Fatalf("stage_alias = %v, want %s", result["stage_alias"], stageAlias)
	}
	report, ok := result["report"].(string)
	if !ok || report == "" {
		t.Fatalf("report = %#v, want non-empty string", result["report"])
	}

	outPath := filepath.Join(t.TempDir(), "report")
	result, err = sess.Validate(stageAlias, outPath)
	if err != nil {
		t.Fatalf("Validate with output: %v", err)
	}
	written, ok := result["output_path"].(string)
	if !ok || written == "" {
		t.Fatalf("output_path = %#v", result["output_path"])
	}
	data, err := os.ReadFile(written)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != report {
		t.Fatalf("written report mismatch")
	}

	if _, err := sess.Validate("missing", ""); err == nil {
		t.Fatal("expected error for unknown stage")
	}
}

func TestSessionTriG(t *testing.T) {
	dir := testRepoDir(t)
	iri := createRepoWithDataset(t, dir)

	sess := NewSession()
	t.Cleanup(func() {
		if err := sess.CloseAll(); err != nil {
			t.Errorf("CloseAll: %v", err)
		}
	})

	if _, err := sess.OpenLocalRepository(dir, "r1"); err != nil {
		t.Fatalf("OpenLocalRepository: %v", err)
	}
	if _, err := sess.OpenDataset("r1", iri, "d1"); err != nil {
		t.Fatalf("OpenDataset: %v", err)
	}

	stageAlias, err := sess.CheckoutBranch("d1", sst.DefaultBranch, "")
	if err != nil {
		t.Fatalf("CheckoutBranch: %v", err)
	}

	result, err := sess.TriG(stageAlias, "")
	if err != nil {
		t.Fatalf("TriG: %v", err)
	}
	trig, ok := result["trig"].(string)
	if !ok || trig == "" {
		t.Fatalf("trig = %#v, want non-empty string", result["trig"])
	}
	if !bytes.Contains([]byte(trig), []byte(iri)) {
		t.Fatalf("trig does not contain dataset IRI %q", iri)
	}

	outPath := filepath.Join(t.TempDir(), "stage")
	result, err = sess.TriG(stageAlias, outPath)
	if err != nil {
		t.Fatalf("TriG with output: %v", err)
	}
	written, ok := result["output_path"].(string)
	if !ok || written == "" {
		t.Fatalf("output_path = %#v", result["output_path"])
	}
	data, err := os.ReadFile(written)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != trig {
		t.Fatalf("written trig mismatch")
	}

	if _, err := sess.TriG("missing", ""); err == nil {
		t.Fatal("expected error for unknown stage")
	}
}

func TestSessionCommit(t *testing.T) {
	dir := testRepoDir(t)
	repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
	if err != nil {
		t.Fatalf("CreateLocalRepository: %v", err)
	}
	if err := repo.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	sess := NewSession()
	t.Cleanup(func() {
		if err := sess.CloseAll(); err != nil {
			t.Errorf("CloseAll: %v", err)
		}
	})

	if _, err := sess.OpenLocalRepository(dir, "r1"); err != nil {
		t.Fatalf("OpenLocalRepository: %v", err)
	}

	stageAlias, err := sess.OpenStage("r1", "")
	if err != nil {
		t.Fatalf("OpenStage: %v", err)
	}
	stage, err := sess.Stage(stageAlias)
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}

	ngID := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363")
	ng := stage.CreateNamedGraph(sst.IRI(ngID.URN()))
	main := ng.CreateIRINode("main")
	main.AddStatement(rdf.Type, rep.SchematicPort)

	result, err := sess.Commit(stageAlias, "mcp commit", sst.DefaultBranch)
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	hash, ok := result["commit"].(string)
	if !ok || hash == "" {
		t.Fatalf("commit = %#v, want non-empty hash", result["commit"])
	}
	if result["message"] != "mcp commit" {
		t.Fatalf("message = %v, want mcp commit", result["message"])
	}
	if result["branch"] != sst.DefaultBranch {
		t.Fatalf("branch = %v, want %q", result["branch"], sst.DefaultBranch)
	}

	if _, err := sess.OpenDataset("r1", ngID.URN(), "d1"); err != nil {
		t.Fatalf("OpenDataset after commit: %v", err)
	}
	branches, err := sess.Branches("d1")
	if err != nil {
		t.Fatalf("Branches: %v", err)
	}
	if branches[sst.DefaultBranch] != hash {
		t.Fatalf("branch tip = %q, want %q", branches[sst.DefaultBranch], hash)
	}

	if _, err := sess.Commit(stageAlias, "", ""); err == nil {
		t.Fatal("expected error for empty message")
	}
	if _, err := sess.Commit("missing", "x", ""); err == nil {
		t.Fatal("expected error for unknown stage")
	}
}
