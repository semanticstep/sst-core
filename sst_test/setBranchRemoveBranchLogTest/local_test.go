// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package setbranchremovebranchlogtest

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/semanticstep/sst-core/defaultderive"
	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalRepository_SetRemoveBranch_LogEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), t.Name())
	ngIDC := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363")

	t.Run("writesLogEntryOnSetAndRemoveBranch", func(t *testing.T) {
		repo, err := sst.CreateLocalRepository(path, "test@semanticstep.net", "default", true)
		require.NoError(t, err)
		defer repo.Close()

		repo.RegisterIndexHandler(defaultderive.DeriveInfo())

		ctx := context.TODO()
		branch := "commit1"

		// First commit
		st := repo.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(sst.IRI(ngIDC.URN()))
		ng.CreateIRINode("testNode").AddStatement(rdf.Type, rep.SchematicPort)

		commitHash, _, err := st.Commit(ctx, "test commit", "main")
		require.NoError(t, err)

		// Run SetBranch and RemoveBranch
		dataset, err := repo.Dataset(ctx, sst.IRI(ngIDC.URN()))
		require.NoError(t, err)
		datasetIRI := dataset.IRI()

		err = dataset.SetBranchCommit(ctx, commitHash, branch)
		require.NoError(t, err)

		err = dataset.RemoveBranch(ctx, branch)
		require.NoError(t, err)

		// Check log
		logs, err := repo.Log(ctx, nil, nil)
		require.NoError(t, err)

		var foundSet, foundRemove bool

		for _, entry := range logs {
			message := entry.Fields["message"]
			author := entry.Fields["author"]
			dsid := entry.Fields["dataset"]
			br := entry.Fields["branch"]
			ts := entry.Fields["timestamp"]

			if author != "default@semanticstep.net" {
				continue
			}
			if uuid.MustParse(dsid).URN() != datasetIRI.String() || br != branch || ts == "" {
				continue
			}

			switch message {
			case "set branch":
				foundSet = true
			case "remove branch":
				foundRemove = true
			}
		}

		assert.True(t, foundSet, "should find log entry for 'set branch'")
		assert.True(t, foundRemove, "should find log entry for 'remove branch'")
	})
}
