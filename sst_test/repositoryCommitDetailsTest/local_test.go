// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package repositorycommitdetails

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/semanticstep/sst-core/defaultderive"
	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_localRepository_CommitDetails(t *testing.T) {
	path := filepath.Join(t.TempDir(), t.Name())

	t.Run("returnsMultipleCommitDetailsCorrectly", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(path, "test@semanticstep.net", "default", true)
		require.NoError(t, err)
		defer repo.Close()

		err = repo.RegisterIndexHandler(defaultderive.DeriveInfo())
		require.NoError(t, err)

		ctx := context.TODO()
		st := repo.OpenStage(sst.DefaultTriplexMode)

		// First commit
		ng1 := st.CreateNamedGraph("")
		require.NotNil(t, ng1)
		ng1.CreateIRINode("node1").AddStatement(rdf.Type, rep.SchematicPort)

		commit1, modifiedDSs1, err := st.Commit(ctx, "first commit", sst.DefaultBranch)
		require.NoError(t, err)
		require.NotEmpty(t, modifiedDSs1)

		// Second commit (different NG)
		ng2 := st.CreateNamedGraph("")
		require.NotNil(t, ng2)
		ng2.CreateIRINode("node2").AddStatement(rdf.Type, rep.SchematicPort)

		commit2, modifiedDSs2, err := st.Commit(ctx, "second commit", sst.DefaultBranch)
		require.NoError(t, err)
		require.NotEmpty(t, modifiedDSs2)

		// Third commit (modify NG1 again to test parent)
		ng1Again := st.NamedGraph(ng1.IRI())
		require.NotNil(t, ng1Again)
		ng1Again.CreateIRINode("node1b").AddStatement(rdf.Type, rep.SchematicPort)

		commit3, modifiedDSs3, err := st.Commit(ctx, "third commit - modify ng1 again", sst.DefaultBranch)
		require.NoError(t, err)
		require.NotEmpty(t, modifiedDSs3)

		// Add a fake hash to test robustness
		var fakeHash sst.Hash
		copy(fakeHash[:], bytes.Repeat([]byte{0xAB}, 32)) // some invalid hash

		// Call new CommitDetails method (note: include commit3 now)
		details, err := repo.CommitDetails(ctx, []sst.Hash{commit1, fakeHash, commit2, commit3})
		require.NoError(t, err)

		// We expect only 3 valid commits in the result
		require.Len(t, details, 3)

		// ---- First Commit Assertions ----
		assert.Equal(t, commit1, details[0].Commit)
		assert.Equal(t, "first commit", details[0].Message)
		assert.NotEmpty(t, details[0].Author)
		assert.Contains(t, details[0].NamedGraphRevisions, ng1.IRI(), "first commit should contain ng1's revision")

		// ---- Second Commit Assertions ----
		assert.Equal(t, commit2, details[1].Commit)
		assert.Equal(t, "second commit", details[1].Message)
		assert.NotEmpty(t, details[1].Author)
		assert.Contains(t, details[1].NamedGraphRevisions, ng2.IRI(), "second commit should contain ng2's revision")

		// ---- Third Commit Assertions (parent of NG1 should be commit1) ----
		assert.Equal(t, commit3, details[2].Commit)
		assert.Equal(t, "third commit - modify ng1 again", details[2].Message)
		assert.NotEmpty(t, details[2].Author)
		assert.Contains(t, details[2].NamedGraphRevisions, ng1.IRI(), "third commit should contain ng1's revision")

		parentsForNG1, ok := details[2].ParentCommits[ng1.IRI()]
		require.True(t, ok, "third commit should record parent commits for ng1")
		require.NotNil(t, parentsForNG1)
		require.Contains(t, parentsForNG1, commit1, "parent of ng1 in third commit should be the first commit")

		// Optionally print for debug
		for _, d := range details {
			t.Logf("Commit %s", d.Commit.String())
			t.Logf("  NG revisions:")
			for ngid, h := range d.NamedGraphRevisions {
				t.Logf("    %s => %s", ngid.String(), h.String())
			}
			t.Logf("  ParentCommits (by dataset/ng uuid):")
			for dsid, ps := range d.ParentCommits {
				var arr []string
				for _, p := range ps {
					arr = append(arr, p.String())
				}
				t.Logf("    %s => [%s]", dsid.String(), strings.Join(arr, ", "))
			}
		}
	})
}

func Test_LocalFullRepositoryMultipleCommits_UUIDNamedGraph(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngIDC := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363")

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

		ng := stageC.CreateNamedGraph(sst.IRI(ngIDC.URN()))

		mainC := ng.CreateIRINode("mainC")

		mainC.AddStatement(rdf.Type, rep.SchematicPort)
		commitHash1, modifiedDS, _ = stageC.Commit(context.TODO(), "First commit of C", sst.DefaultBranch)
		assert.Equal(t, 1, len(modifiedDS))
		assert.Equal(t, ngIDC, modifiedDS[0])

		ds, err := repo.Dataset(context.TODO(), sst.IRI(ngIDC.URN()))
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
		assert.Equal(t, ngIDC, modifiedDS[0])

		mainC.AddStatement(rdf.Direction, rep.Blue)
		commitHash3, _, _ = stageC.Commit(context.TODO(), "Third commit of C", sst.DefaultBranch)

	})

	t.Run("CommitDetailsByHash", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		cds, err := repo.CommitDetails(context.TODO(), []sst.Hash{commitHash3})

		cds[0].Dump()
	})
}
