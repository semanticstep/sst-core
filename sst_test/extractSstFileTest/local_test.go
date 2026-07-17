// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package extractsstfiletest

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/semanticstep/sst-core/defaultderive"
	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalFullRepository_ExtractSstFile(t *testing.T) {
	path := "./testLocalFullRepo_extractsst"
	defer removeFolder(path)

	// Helper function to reset test environment
	setupTestRepository := func(t *testing.T, path string) sst.Repository {
		removeFolder(path)
		repo, err := sst.CreateLocalRepository(path, "default@semanticstep.net", "default", true)
		require.NoError(t, err)
		repo.RegisterIndexHandler(defaultderive.DeriveInfo())
		return repo
	}

	t.Run("basic extract", func(t *testing.T) {
		repo := setupTestRepository(t, path)
		defer repo.Close()

		// Step 1: Create a NamedGraph with known UUID and commit it
		ngIRI := uuid.New().URN()
		stage := repo.OpenStage(sst.DefaultTriplexMode)
		ng := stage.CreateNamedGraph(sst.IRI(ngIRI))
		node := ng.CreateIRINode("node1")
		node.AddStatement(rdf.Type, rep.SchematicPort)

		commitHash, datasetIDs, err := stage.Commit(context.TODO(), "commit for extract", sst.DefaultBranch)
		require.NoError(t, err)
		require.Len(t, datasetIDs, 1)

		// Step 2: Get dataset and retrieve NamedGraphRevision hash
		dataset, err := repo.Dataset(context.TODO(), sst.IRI(datasetIDs[0].URN()))
		require.NoError(t, err)

		commitDetails, err := dataset.CommitDetailsByHash(context.TODO(), commitHash)
		require.NoError(t, err)

		revisionHash, ok := commitDetails.NamedGraphRevisions[sst.IRI(ngIRI)]
		require.True(t, ok, "NamedGraphRevision hash not found for given UUID")

		// Step 3: Extract SST file content
		var buf bytes.Buffer
		err = repo.ExtractSstFile(context.TODO(), revisionHash, &buf)
		require.NoError(t, err)

		// Step 4: Assert content is non-empty
		assert.NotEmpty(t, buf.Bytes())
		t.Logf("Extracted SST content size: %d bytes", len(buf.Bytes()))
	})

	t.Run("nonexistent revision should fail", func(t *testing.T) {
		repo := setupTestRepository(t, path)
		defer repo.Close()

		var fakeHash sst.Hash
		copy(fakeHash[:], bytes.Repeat([]byte{0xEE}, 32))

		var buf bytes.Buffer
		err := repo.ExtractSstFile(context.TODO(), fakeHash, &buf)
		assert.Error(t, err)
	})

	t.Run("extract from multiple NamedGraphs", func(t *testing.T) {
		repo := setupTestRepository(t, path)
		defer repo.Close()

		stage := repo.OpenStage(sst.DefaultTriplexMode)
		ngIRI := sst.IRI(uuid.New().URN())
		ng1 := stage.CreateNamedGraph(ngIRI)
		ng1.CreateIRINode("n1").AddStatement(rdf.Type, rep.SchematicPort)

		ngID2 := uuid.New()
		ng2 := stage.CreateNamedGraph(sst.IRI(ngID2.URN()))
		ng2.CreateIRINode("n2").AddStatement(rdf.Type, rep.Angle)

		commitHash, datasetIDs, err := stage.Commit(context.TODO(), "commit multi graphs", sst.DefaultBranch)
		require.NoError(t, err)
		require.Len(t, datasetIDs, 2)

		// Pick ngID1's revision
		dataset1, err := repo.Dataset(context.TODO(), sst.IRI(datasetIDs[0].URN()))
		require.NoError(t, err)

		commitDetails, err := dataset1.CommitDetailsByHash(context.TODO(), commitHash)
		require.NoError(t, err)

		revisionHash, ok := commitDetails.NamedGraphRevisions[ngIRI]
		require.True(t, ok)

		var buf bytes.Buffer
		err = repo.ExtractSstFile(context.TODO(), revisionHash, &buf)
		require.NoError(t, err)
		assert.NotEmpty(t, buf.Bytes())
	})
}

func removeFolder(dir string) {
	// check and delete old dir
	if _, err := os.Stat(dir); err == nil {
		err := os.RemoveAll(dir)
		if err != nil {
			fmt.Printf("Failed to delete %s: %s\n", dir, err)
		} else {
			fmt.Printf("%s has been deleted successfully\n", dir)
		}
	} else if os.IsNotExist(err) {
		fmt.Println(dir + " - This file or directory does not exist.")
	} else {
		fmt.Printf("Error checking if file exists: %s\n", err)
	}
}
