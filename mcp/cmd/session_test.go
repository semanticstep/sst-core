// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package mcp

import (
	"path/filepath"
	"testing"

	"github.com/semanticstep/sst-core/sst"
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
