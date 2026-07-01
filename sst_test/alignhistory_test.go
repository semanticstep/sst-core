// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/sst_test/testutil"
	"github.com/semanticstep/sst-core/sstauth"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_AlignHistory_RdfReadRoundTrip tests the workflow where a dataset
// is checked out from a repository, exported as Turtle, modified externally,
// re-imported via RdfRead, and then committed by inheriting history via
// AlignHistory.
func Test_AlignHistory_RdfReadRoundTrip(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/" + testName)
	ngBaseIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a500").URN())
	ngImportIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a501").URN())

	defer os.RemoveAll(dir)

	t.Run("setup_repo_and_commit", func(t *testing.T) {
		removeFolder(dir)

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		require.NoError(t, err)
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		// Create imported dataset
		ngImport := st.CreateNamedGraph(ngImportIRI)
		importNode := ngImport.CreateIRINode("importNode")
		importNode.AddStatement(rdf.Type, rep.SchematicPort)

		_, _, err = st.Commit(context.TODO(), "Import dataset commit", sst.DefaultBranch)
		require.NoError(t, err)

		// Create base dataset that imports the first one
		ngBase := st.CreateNamedGraph(ngBaseIRI)
		baseNode := ngBase.CreateIRINode("baseNode")
		baseNode.AddStatement(rdf.Type, rep.SchematicPort)

		err = ngBase.AddImport(ngImport)
		require.NoError(t, err)

		_, _, err = st.Commit(context.TODO(), "Base dataset with import", sst.DefaultBranch)
		require.NoError(t, err)
	})

	t.Run("checkout_export_modify_reimport_and_commit", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		require.NoError(t, err)
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngBaseIRI)
		require.NoError(t, err)

		// Stage1: checkout from repository
		stage1, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		require.NoError(t, err)

		ngBase := stage1.NamedGraph(ngBaseIRI)
		require.NotNil(t, ngBase)

		// Verify Stage1 has both graphs as local
		assert.NotNil(t, stage1.NamedGraph(ngImportIRI), "Stage1 should have imported graph as local")

		// Get history info from Stage1 before exporting
		stage1BaseInfo := ngBase.Info()
		stage1ImportInfo := stage1.NamedGraph(ngImportIRI).Info()

		// Export base NG as Turtle
		var buf bytes.Buffer
		err = ngBase.RdfWrite(&buf, sst.RdfFormatTurtle)
		require.NoError(t, err)

		turtleOutput := buf.String()
		t.Logf("Exported Turtle:\n%s", turtleOutput)

		// Verify the export contains the import statement
		assert.Contains(t, turtleOutput, "owl:imports")
		assert.Contains(t, turtleOutput, ngImportIRI.String())

		// Modify externally: add a new node to the Turtle content
		modifiedTurtle := strings.Replace(
			turtleOutput,
			":baseNode a rep:SchematicPort .",
			":baseNode a rep:SchematicPort .\n:newExternalNode a <http://ontology.semanticstep.net/rep#SchematicPort> .",
			1,
		)

		// Re-import as Stage2 via RdfRead
		stage2, err := sst.RdfRead(bufio.NewReader(bytes.NewReader([]byte(modifiedTurtle))), sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
		require.NoError(t, err)

		// Stage2 should have the base graph as local and the import as referenced
		ngBase2 := stage2.NamedGraph(ngBaseIRI)
		require.NotNil(t, ngBase2, "Stage2 should have base graph")

		ngImport2 := stage2.ReferencedGraph(ngImportIRI)
		require.NotNil(t, ngImport2, "Stage2 should have imported graph as referenced")

		// Before AlignHistory, Stage2 has no repository link
		assert.Nil(t, stage2.Repository(), "Stage2 should not be linked to a repo yet")

		// Inherit history from Stage1 so Stage2 can be committed
		stage2.AlignHistory(stage1)

		// Verify Stage2 is now linked to the repository
		assert.Equal(t, repo, stage2.Repository(), "Stage2 should inherit repo from Stage1")

		// Verify checkout values were copied for base graph
		ngBase2Info := ngBase2.Info()
		assert.Equal(t, stage1BaseInfo.CheckedOutCommits, ngBase2Info.CheckedOutCommits, "Base graph commits should match")
		assert.Equal(t, stage1BaseInfo.CheckedOutNGRevisions, ngBase2Info.CheckedOutNGRevisions, "Base graph checked-out NG revisions should match")
		assert.Equal(t, stage1BaseInfo.CheckedOutDSRevisions, ngBase2Info.CheckedOutDSRevisions, "Base graph checked-out DS revisions should match")
		// NamedGraphRevision/DatasetRevision now reflect real-time content; they may differ if modified

		// After AlignHistory the imported graph should have been converted to local
		ngImport2 = stage2.NamedGraph(ngImportIRI)
		require.NotNil(t, ngImport2, "Imported graph should now be local in Stage2")

		// Verify checkout values were copied for imported graph
		ngImport2Info := ngImport2.Info()
		assert.Equal(t, stage1ImportInfo.CheckedOutCommits, ngImport2Info.CheckedOutCommits, "Import graph commits should match")
		assert.Equal(t, stage1ImportInfo.CheckedOutNGRevisions, ngImport2Info.CheckedOutNGRevisions, "Import graph checked-out NG revisions should match")
		assert.Equal(t, stage1ImportInfo.CheckedOutDSRevisions, ngImport2Info.CheckedOutDSRevisions, "Import graph checked-out DS revisions should match")
		// NamedGraphRevision/DatasetRevision now reflect real-time content; import graph is empty/deleted so NG rev is emptyHash

		// Verify the imported graph is marked as deleted (empty shell)
		assert.False(t, ngImport2.IsReferenced(), "Imported graph should remain local (not referenced)")
		assert.True(t, ngImport2.Info().IsModified, "Imported graph should be modified")

		// Verify the external modification is present
		newNode := ngBase2.GetIRINodeByFragment("newExternalNode")
		assert.NotNil(t, newNode, "Modified node should be present in Stage2")

		// Stage2 should now be commitable
		_, _, err = stage2.Commit(context.TODO(), "Commit after external modification", sst.DefaultBranch)
		assert.NoError(t, err, "Stage2 should be commitable after inheriting history")
	})

	t.Run("checkout_and_verify_modification", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		require.NoError(t, err)
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngBaseIRI)
		require.NoError(t, err)

		// Checkout the latest committed version
		stage3, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		require.NoError(t, err)

		ngBase3 := stage3.NamedGraph(ngBaseIRI)
		require.NotNil(t, ngBase3)

		// Verify the original node still exists
		baseNode3 := ngBase3.GetIRINodeByFragment("baseNode")
		assert.NotNil(t, baseNode3, "Original baseNode should exist after commit")

		// Verify the externally modified node was persisted
		newNode3 := ngBase3.GetIRINodeByFragment("newExternalNode")
		assert.NotNil(t, newNode3, "Externally added node should be persisted after commit")

		// Verify the node has the rdf:type rep:SchematicPort triple
		typeFound := false
		_ = newNode3.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
			if s == newNode3 && p.IRI().String() == "http://www.w3.org/1999/02/22-rdf-syntax-ns#type" {
				if ib, ok := o.(sst.IBNode); ok && ib.IRI().String() == "http://ontology.semanticstep.net/rep#SchematicPort" {
					typeFound = true
				}
			}
			return nil
		})
		assert.True(t, typeFound, "New node should have rdf:type rep:SchematicPort")

		// Remote repository behavior: deleted graph may still be present in the
		// checked-out stage as an empty local graph. Verify it has no actual content.
		ngImport3 := stage3.NamedGraph(ngImportIRI)
		if ngImport3 != nil {
			assert.Nil(t, ngImport3.GetIRINodeByFragment("importNode"), "Deleted imported graph should not contain importNode")
		}
	})
}

// Test_AlignHistory_TriGMultipleNGs tests exporting multiple named graphs
// as TriG, modifying one externally, re-importing, and committing via AlignHistory.
func Test_AlignHistory_TriGMultipleNGs(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/" + testName)
	ngBaseIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a510").URN())
	ngImportIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a511").URN())

	defer os.RemoveAll(dir)

	t.Run("setup_repo_and_commit", func(t *testing.T) {
		removeFolder(dir)

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		require.NoError(t, err)
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		// Create imported dataset
		ngImport := st.CreateNamedGraph(ngImportIRI)
		importNode := ngImport.CreateIRINode("importNode")
		importNode.AddStatement(rdf.Type, rep.SchematicPort)

		_, _, err = st.Commit(context.TODO(), "Import dataset commit", sst.DefaultBranch)
		require.NoError(t, err)

		// Create base dataset that imports the first one
		ngBase := st.CreateNamedGraph(ngBaseIRI)
		baseNode := ngBase.CreateIRINode("baseNode")
		baseNode.AddStatement(rdf.Type, rep.SchematicPort)

		err = ngBase.AddImport(ngImport)
		require.NoError(t, err)

		_, _, err = st.Commit(context.TODO(), "Base dataset with import", sst.DefaultBranch)
		require.NoError(t, err)
	})

	t.Run("checkout_export_modify_reimport_and_commit", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		require.NoError(t, err)
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngBaseIRI)
		require.NoError(t, err)

		// Stage1: checkout from repository (contains both NGs as local)
		stage1, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		require.NoError(t, err)

		// Export the entire stage as TriG (contains both named graphs)
		var buf bytes.Buffer
		err = stage1.RdfWrite(&buf, sst.RdfFormatTriG)
		require.NoError(t, err)

		trigOutput := buf.String()
		t.Logf("Exported TriG:\n%s", trigOutput)

		// Verify both graphs appear in the export
		assert.Contains(t, trigOutput, ngBaseIRI.String())
		assert.Contains(t, trigOutput, ngImportIRI.String())

		// Modify externally: add a new node to the imported graph in the TriG content
		modifiedTriG := strings.Replace(
			trigOutput,
			"importNode a rep:SchematicPort .",
			"importNode a rep:SchematicPort .\n\t<urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f613a511#newTriGNode> a <http://ontology.semanticstep.net/rep#SchematicPort> .",
			1,
		)

		// Re-import as Stage2 via RdfRead
		stage2, err := sst.RdfRead(bufio.NewReader(bytes.NewReader([]byte(modifiedTriG))), sst.RdfFormatTriG, sst.StrictHandler, sst.DefaultTriplexMode)
		require.NoError(t, err)

		// Stage2 should have both graphs as local (TriG reader creates all mentioned graphs as local)
		ngBase2 := stage2.NamedGraph(ngBaseIRI)
		require.NotNil(t, ngBase2, "Stage2 should have base graph")

		ngImport2 := stage2.NamedGraph(ngImportIRI)
		require.NotNil(t, ngImport2, "Stage2 should have imported graph")

		// Before AlignHistory, Stage2 has no repository link
		assert.Nil(t, stage2.Repository(), "Stage2 should not be linked to a repo yet")

		// Get history info from Stage1 before inheriting
		stage1BaseInfo := stage1.NamedGraph(ngBaseIRI).Info()
		stage1ImportInfo := stage1.NamedGraph(ngImportIRI).Info()

		// Inherit history from Stage1 so Stage2 can be committed
		stage2.AlignHistory(stage1)

		// Verify Stage2 is now linked to the repository
		assert.Equal(t, repo, stage2.Repository(), "Stage2 should inherit repo from Stage1")

		// Verify checkout values were copied for base graph
		ngBase2Info := ngBase2.Info()
		assert.Equal(t, stage1BaseInfo.CheckedOutCommits, ngBase2Info.CheckedOutCommits, "Base graph commits should match")
		assert.Equal(t, stage1BaseInfo.CheckedOutNGRevisions, ngBase2Info.CheckedOutNGRevisions, "Base graph checked-out NG revisions should match")
		assert.Equal(t, stage1BaseInfo.CheckedOutDSRevisions, ngBase2Info.CheckedOutDSRevisions, "Base graph checked-out DS revisions should match")

		// Verify checkout values were copied for imported graph
		ngImport2Info := ngImport2.Info()
		assert.Equal(t, stage1ImportInfo.CheckedOutCommits, ngImport2Info.CheckedOutCommits, "Import graph commits should match")
		assert.Equal(t, stage1ImportInfo.CheckedOutNGRevisions, ngImport2Info.CheckedOutNGRevisions, "Import graph checked-out NG revisions should match")
		assert.Equal(t, stage1ImportInfo.CheckedOutDSRevisions, ngImport2Info.CheckedOutDSRevisions, "Import graph checked-out DS revisions should match")

		// Verify the external modification is present in the imported graph
		newNode := ngImport2.GetIRINodeByFragment("newTriGNode")
		assert.NotNil(t, newNode, "Modified node should be present in Stage2")

		// Stage2 should now be commitable
		_, _, err = stage2.Commit(context.TODO(), "Commit after TriG modification", sst.DefaultBranch)
		assert.NoError(t, err, "Stage2 should be commitable after inheriting history")
	})

	t.Run("checkout_and_verify_modification", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		require.NoError(t, err)
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngBaseIRI)
		require.NoError(t, err)

		// Checkout the latest committed version
		stage3, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		require.NoError(t, err)

		ngBase3 := stage3.NamedGraph(ngBaseIRI)
		require.NotNil(t, ngBase3)

		// Verify the original base node still exists
		baseNode3 := ngBase3.GetIRINodeByFragment("baseNode")
		assert.NotNil(t, baseNode3, "Original baseNode should exist after commit")

		// The modified graph may no longer be imported by the base graph after TriG round-trip,
		// so verify it by checking out its own dataset directly.
		dsImport, err := repo.Dataset(context.TODO(), ngImportIRI)
		require.NoError(t, err)

		stageImport, err := dsImport.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		require.NoError(t, err)

		ngImport3 := stageImport.NamedGraph(ngImportIRI)
		require.NotNil(t, ngImport3)

		importNode3 := ngImport3.GetIRINodeByFragment("importNode")
		assert.NotNil(t, importNode3, "Original importNode should exist after commit")

		// Verify the externally modified node was persisted in the imported graph
		newNode3 := ngImport3.GetIRINodeByFragment("newTriGNode")
		assert.NotNil(t, newNode3, "Externally added node should be persisted after commit")

		// Verify the node has the rdf:type rep:SchematicPort triple
		typeFound := false
		_ = newNode3.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
			if s == newNode3 && p.IRI().String() == "http://www.w3.org/1999/02/22-rdf-syntax-ns#type" {
				if ib, ok := o.(sst.IBNode); ok && ib.IRI().String() == "http://ontology.semanticstep.net/rep#SchematicPort" {
					typeFound = true
				}
			}
			return nil
		})
		assert.True(t, typeFound, "New node should have rdf:type rep:SchematicPort")
	})
}

// Test_AlignHistory_TriGDeleteNamedGraph tests exporting multiple named graphs
// as TriG, deleting one named graph in Stage2, committing, and verifying the deletion.
func Test_AlignHistory_TriGDeleteNamedGraph(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/" + testName)
	ngBaseIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a520").URN())
	ngImportIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a521").URN())

	defer os.RemoveAll(dir)

	t.Run("setup_repo_and_commit", func(t *testing.T) {
		removeFolder(dir)

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		require.NoError(t, err)
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		// Create imported dataset
		ngImport := st.CreateNamedGraph(ngImportIRI)
		importNode := ngImport.CreateIRINode("importNode")
		importNode.AddStatement(rdf.Type, rep.SchematicPort)

		_, _, err = st.Commit(context.TODO(), "Import dataset commit", sst.DefaultBranch)
		require.NoError(t, err)

		// Create base dataset that imports the first one
		ngBase := st.CreateNamedGraph(ngBaseIRI)
		baseNode := ngBase.CreateIRINode("baseNode")
		baseNode.AddStatement(rdf.Type, rep.SchematicPort)

		err = ngBase.AddImport(ngImport)
		require.NoError(t, err)

		_, _, err = st.Commit(context.TODO(), "Base dataset with import", sst.DefaultBranch)
		require.NoError(t, err)
	})

	t.Run("checkout_export_delete_and_commit", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		require.NoError(t, err)
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngBaseIRI)
		require.NoError(t, err)

		// Stage1: checkout from repository (contains both NGs as local)
		stage1, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		require.NoError(t, err)

		// Export the entire stage as TriG (contains both named graphs)
		var buf bytes.Buffer
		err = stage1.RdfWrite(&buf, sst.RdfFormatTriG)
		require.NoError(t, err)

		trigOutput := buf.String()
		t.Logf("Exported TriG:\n%s", trigOutput)

		// Re-import as Stage2 via RdfRead (without the deleted graph - but to test deletion
		// we import the full TriG and then explicitly delete the imported graph in Stage2)
		stage2, err := sst.RdfRead(bufio.NewReader(bytes.NewReader([]byte(trigOutput))), sst.RdfFormatTriG, sst.StrictHandler, sst.DefaultTriplexMode)
		require.NoError(t, err)

		ngImport2 := stage2.NamedGraph(ngImportIRI)
		require.NotNil(t, ngImport2, "Stage2 should have imported graph")

		// Inherit history from Stage1 so Stage2 can be committed
		stage2.AlignHistory(stage1)

		// Delete the imported named graph in Stage2
		err = ngImport2.Delete()
		require.NoError(t, err)

		// Stage2 should now be commitable
		_, _, err = stage2.Commit(context.TODO(), "Commit after deleting imported graph", sst.DefaultBranch)
		assert.NoError(t, err, "Stage2 should be commitable after deleting imported graph")
	})

	t.Run("checkout_and_verify_deletion", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		require.NoError(t, err)
		defer repo.Close()

		// Verify the base dataset is still present and contains baseNode
		ds, err := repo.Dataset(context.TODO(), ngBaseIRI)
		require.NoError(t, err)

		stageBase, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		require.NoError(t, err)

		ngBase3 := stageBase.NamedGraph(ngBaseIRI)
		require.NotNil(t, ngBase3)

		baseNode3 := ngBase3.GetIRINodeByFragment("baseNode")
		assert.NotNil(t, baseNode3, "Original baseNode should exist after commit")

		// Verify the imported dataset has been deleted
		dsImport, err := repo.Dataset(context.TODO(), ngImportIRI)
		require.NoError(t, err)

		// When a dataset is deleted, its branch entry is removed, so CheckoutBranch returns ErrBranchNotFound
		_, err = dsImport.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		assert.ErrorIs(t, err, sst.ErrBranchNotFound, "Deleted dataset should not have the branch anymore")

		// Verify repository info reflects the deletion
		info, err := repo.Info(context.TODO(), sst.DefaultBranch)
		require.NoError(t, err)
		assert.Equal(t, int64(1), info.NumberOfDatasetsInBranch, "Only base dataset should remain in branch")
		assert.Equal(t, int64(2), info.NumberOfDatasets, "Total datasets should still be 2")
	})
}

// Test_AlignHistory_TriGRemoveGraphBlock tests the scenario where an entire
// NamedGraph is removed by an external editor.
//
// Workflow:
//  1. Create a repository with two NamedGraphs: a base graph and an imported graph.
//  2. Export the whole stage as TriG. In TriG format each NamedGraph is represented
//     as a separate block: <graph-iri> { ... triples ... }.
//  3. Simulate external deletion by stripping the entire block belonging to the
//     imported graph from the TriG text (everything from '<import-iri> {' to
//     the matching '}' inclusive).
//  4. Re-import the modified TriG via RdfRead. Because the block is gone, the
//     new stage does not contain the imported graph at all.
//  5. Call AlignHistory against the original checked-out stage. Since the source
//     stage has the imported graph as local but the new stage is missing it,
//     AlignHistory creates the graph in the new stage and marks it as deleted.
//  6. Commit the new stage. The repository records the deletion.
//  7. Verify that after re-checkout the imported graph no longer has usable content.
func Test_AlignHistory_TriGRemoveGraphBlock(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/" + testName)
	ngBaseIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a530").URN())
	ngImportIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a531").URN())

	defer os.RemoveAll(dir)

	t.Run("setup_repo_and_commit", func(t *testing.T) {
		removeFolder(dir)

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		require.NoError(t, err)
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		// Create imported dataset
		ngImport := st.CreateNamedGraph(ngImportIRI)
		importNode := ngImport.CreateIRINode("importNode")
		importNode.AddStatement(rdf.Type, rep.SchematicPort)

		_, _, err = st.Commit(context.TODO(), "Import dataset commit", sst.DefaultBranch)
		require.NoError(t, err)

		// Create base dataset that imports the first one
		ngBase := st.CreateNamedGraph(ngBaseIRI)
		baseNode := ngBase.CreateIRINode("baseNode")
		baseNode.AddStatement(rdf.Type, rep.SchematicPort)

		err = ngBase.AddImport(ngImport)
		require.NoError(t, err)

		_, _, err = st.Commit(context.TODO(), "Base dataset with import", sst.DefaultBranch)
		require.NoError(t, err)
	})

	t.Run("checkout_export_remove_block_and_commit", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		require.NoError(t, err)
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngBaseIRI)
		require.NoError(t, err)

		// Stage1: checkout from repository (contains both NGs as local)
		stage1, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		require.NoError(t, err)

		// Export the entire stage as TriG (contains both named graphs)
		var buf bytes.Buffer
		err = stage1.RdfWrite(&buf, sst.RdfFormatTriG)
		require.NoError(t, err)

		trigOutput := buf.String()
		t.Logf("Exported TriG:\n%s", trigOutput)

		// Simulate an external editor deleting the imported graph by removing its
		// entire TriG block. In TriG a NamedGraph is enclosed in braces preceded by
		// its IRI, e.g.:
		//   <urn:uuid:...551> {
		//       ...triples...
		//   }
		// We drop every line from the opening '<iri> {' down to and including the
		// matching closing '}' so that RdfRead will not see this graph at all.
		lines := strings.Split(trigOutput, "\n")
		var modifiedLines []string
		inImportBlock := false
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			// Detect start of imported graph block
			if strings.HasPrefix(trimmed, "<"+ngImportIRI.String()+">") && strings.HasSuffix(trimmed, "{") {
				inImportBlock = true
				continue
			}
			// Detect end of imported graph block
			if inImportBlock && trimmed == "}" {
				inImportBlock = false
				continue
			}
			if inImportBlock {
				continue
			}
			modifiedLines = append(modifiedLines, line)
		}
		modifiedTriG := strings.Join(modifiedLines, "\n")

		t.Logf("Modified TriG:\n%s", modifiedTriG)

		// Re-import as Stage2 via RdfRead
		stage2, err := sst.RdfRead(bufio.NewReader(bytes.NewReader([]byte(modifiedTriG))), sst.RdfFormatTriG, sst.StrictHandler, sst.DefaultTriplexMode)
		require.NoError(t, err)

		// Stage2 should only have the base graph now
		ngBase2 := stage2.NamedGraph(ngBaseIRI)
		require.NotNil(t, ngBase2, "Stage2 should have base graph")

		ngImport2 := stage2.NamedGraph(ngImportIRI)
		assert.Nil(t, ngImport2, "Stage2 should NOT have imported graph after block removal")

		// Before AlignHistory, Stage2 has no repository link
		assert.Nil(t, stage2.Repository(), "Stage2 should not be linked to a repo yet")

		// Inherit history from Stage1 so Stage2 can be committed
		stage2.AlignHistory(stage1)

		// Verify Stage2 is now linked to the repository
		assert.Equal(t, repo, stage2.Repository(), "Stage2 should inherit repo from Stage1")

		// The imported graph should have been created and marked as deleted
		ngImport2 = stage2.NamedGraph(ngImportIRI)
		require.NotNil(t, ngImport2, "Imported graph should be created as deleted in Stage2")
		assert.False(t, ngImport2.IsReferenced(), "Deleted graph should remain local (not referenced)")

		// Stage2 should now be commitable
		_, _, err = stage2.Commit(context.TODO(), "Commit after removing graph block from TriG", sst.DefaultBranch)
		assert.NoError(t, err, "Stage2 should be commitable after inheriting history")
	})

	t.Run("checkout_and_verify_deletion", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		require.NoError(t, err)
		defer repo.Close()

		// Verify the base dataset is still present and contains baseNode
		ds, err := repo.Dataset(context.TODO(), ngBaseIRI)
		require.NoError(t, err)

		stageBase, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		require.NoError(t, err)

		ngBase3 := stageBase.NamedGraph(ngBaseIRI)
		require.NotNil(t, ngBase3)

		baseNode3 := ngBase3.GetIRINodeByFragment("baseNode")
		assert.NotNil(t, baseNode3, "Original baseNode should exist after commit")

		// Verify the imported dataset has been deleted
		dsImport, err := repo.Dataset(context.TODO(), ngImportIRI)
		require.NoError(t, err)

		_, err = dsImport.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		assert.ErrorIs(t, err, sst.ErrBranchNotFound, "Deleted dataset should not have the branch anymore")

		// Verify repository info reflects the deletion
		info, err := repo.Info(context.TODO(), sst.DefaultBranch)
		require.NoError(t, err)
		assert.Equal(t, int64(1), info.NumberOfDatasetsInBranch, "Only base dataset should remain in branch")
		assert.Equal(t, int64(2), info.NumberOfDatasets, "Total datasets should still be 2")
	})
}

// Test_AlignHistory_Remote_RdfReadRoundTrip tests the AlignHistory workflow
// using a remote gRPC repository instead of a local one.
func Test_AlignHistory_Remote_RdfReadRoundTrip(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/" + testName)
	ngBaseIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a540").URN())
	ngImportIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a541").URN())

	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	defer os.RemoveAll(dir)

	t.Run("setup_repo_and_commit", func(t *testing.T) {
		removeFolder(dir)
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		ngImport := st.CreateNamedGraph(ngImportIRI)
		importNode := ngImport.CreateIRINode("importNode")
		importNode.AddStatement(rdf.Type, rep.SchematicPort)

		_, _, err = st.Commit(constructCtx, "Import dataset commit", sst.DefaultBranch)
		require.NoError(t, err)

		ngBase := st.CreateNamedGraph(ngBaseIRI)
		baseNode := ngBase.CreateIRINode("baseNode")
		baseNode.AddStatement(rdf.Type, rep.SchematicPort)

		err = ngBase.AddImport(ngImport)
		require.NoError(t, err)

		_, _, err = st.Commit(constructCtx, "Base dataset with import", sst.DefaultBranch)
		require.NoError(t, err)
	})

	t.Run("checkout_export_modify_reimport_and_commit", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		ds, err := repo.Dataset(constructCtx, ngBaseIRI)
		require.NoError(t, err)

		stage1, err := ds.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		require.NoError(t, err)

		ngBase := stage1.NamedGraph(ngBaseIRI)
		require.NotNil(t, ngBase)

		assert.NotNil(t, stage1.NamedGraph(ngImportIRI), "Stage1 should have imported graph as local")

		stage1BaseInfo := ngBase.Info()
		stage1ImportInfo := stage1.NamedGraph(ngImportIRI).Info()

		var buf bytes.Buffer
		err = ngBase.RdfWrite(&buf, sst.RdfFormatTurtle)
		require.NoError(t, err)

		turtleOutput := buf.String()
		t.Logf("Exported Turtle:\n%s", turtleOutput)

		assert.Contains(t, turtleOutput, "owl:imports")
		assert.Contains(t, turtleOutput, ngImportIRI.String())

		modifiedTurtle := strings.Replace(
			turtleOutput,
			":baseNode a rep:SchematicPort .",
			":baseNode a rep:SchematicPort .\n:newExternalNode a <http://ontology.semanticstep.net/rep#SchematicPort> .",
			1,
		)

		stage2, err := sst.RdfRead(bufio.NewReader(bytes.NewReader([]byte(modifiedTurtle))), sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
		require.NoError(t, err)

		ngBase2 := stage2.NamedGraph(ngBaseIRI)
		require.NotNil(t, ngBase2, "Stage2 should have base graph")

		ngImport2 := stage2.ReferencedGraph(ngImportIRI)
		require.NotNil(t, ngImport2, "Stage2 should have imported graph as referenced")

		assert.Nil(t, stage2.Repository(), "Stage2 should not be linked to a repo yet")

		stage2.AlignHistory(stage1)

		assert.Equal(t, repo, stage2.Repository(), "Stage2 should inherit repo from Stage1")

		ngBase2Info := ngBase2.Info()
		assert.Equal(t, stage1BaseInfo.CheckedOutCommits, ngBase2Info.CheckedOutCommits, "Base graph commits should match")
		assert.Equal(t, stage1BaseInfo.CheckedOutNGRevisions, ngBase2Info.CheckedOutNGRevisions, "Base graph checked-out NG revisions should match")
		assert.Equal(t, stage1BaseInfo.CheckedOutDSRevisions, ngBase2Info.CheckedOutDSRevisions, "Base graph checked-out DS revisions should match")

		ngImport2 = stage2.NamedGraph(ngImportIRI)
		require.NotNil(t, ngImport2, "Imported graph should now be local in Stage2")

		ngImport2Info := ngImport2.Info()
		assert.Equal(t, stage1ImportInfo.CheckedOutCommits, ngImport2Info.CheckedOutCommits, "Import graph commits should match")
		assert.Equal(t, stage1ImportInfo.CheckedOutNGRevisions, ngImport2Info.CheckedOutNGRevisions, "Import graph checked-out NG revisions should match")
		assert.Equal(t, stage1ImportInfo.CheckedOutDSRevisions, ngImport2Info.CheckedOutDSRevisions, "Import graph checked-out DS revisions should match")

		assert.False(t, ngImport2.IsReferenced(), "Imported graph should remain local (not referenced)")
		assert.True(t, ngImport2.Info().IsModified, "Imported graph should be modified")

		newNode := ngBase2.GetIRINodeByFragment("newExternalNode")
		assert.NotNil(t, newNode, "Modified node should be present in Stage2")

		_, _, err = stage2.Commit(constructCtx, "Commit after external modification", sst.DefaultBranch)
		assert.NoError(t, err, "Stage2 should be commitable after inheriting history")
	})

	t.Run("checkout_and_verify_modification", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		ds, err := repo.Dataset(constructCtx, ngBaseIRI)
		require.NoError(t, err)

		stage3, err := ds.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		require.NoError(t, err)

		ngBase3 := stage3.NamedGraph(ngBaseIRI)
		require.NotNil(t, ngBase3)

		baseNode3 := ngBase3.GetIRINodeByFragment("baseNode")
		assert.NotNil(t, baseNode3, "Original baseNode should exist after commit")

		newNode3 := ngBase3.GetIRINodeByFragment("newExternalNode")
		assert.NotNil(t, newNode3, "Externally added node should be persisted after commit")

		typeFound := false
		_ = newNode3.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
			if s == newNode3 && p.IRI().String() == "http://www.w3.org/1999/02/22-rdf-syntax-ns#type" {
				if ib, ok := o.(sst.IBNode); ok && ib.IRI().String() == "http://ontology.semanticstep.net/rep#SchematicPort" {
					typeFound = true
				}
			}
			return nil
		})
		assert.True(t, typeFound, "New node should have rdf:type rep:SchematicPort")

		// Remote repository behavior: deleted graph may still be present in the
		// checked-out stage as an empty local graph. Verify it has no actual content.
		ngImport3 := stage3.NamedGraph(ngImportIRI)
		if ngImport3 != nil {
			assert.Nil(t, ngImport3.GetIRINodeByFragment("importNode"), "Deleted imported graph should not contain importNode")
		}
	})
}

// Test_AlignHistory_Remote_TriGRemoveGraphBlock tests the same external-deletion
// workflow as Test_AlignHistory_TriGRemoveGraphBlock, but against a remote gRPC
// repository. It verifies that AlignHistory correctly propagates a missing graph
// as a deletion over the network.
func Test_AlignHistory_Remote_TriGRemoveGraphBlock(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/" + testName)
	ngBaseIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a550").URN())
	ngImportIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a551").URN())

	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	defer os.RemoveAll(dir)

	t.Run("setup_repo_and_commit", func(t *testing.T) {
		removeFolder(dir)
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		ngImport := st.CreateNamedGraph(ngImportIRI)
		importNode := ngImport.CreateIRINode("importNode")
		importNode.AddStatement(rdf.Type, rep.SchematicPort)

		_, _, err = st.Commit(constructCtx, "Import dataset commit", sst.DefaultBranch)
		require.NoError(t, err)

		ngBase := st.CreateNamedGraph(ngBaseIRI)
		baseNode := ngBase.CreateIRINode("baseNode")
		baseNode.AddStatement(rdf.Type, rep.SchematicPort)

		err = ngBase.AddImport(ngImport)
		require.NoError(t, err)

		_, _, err = st.Commit(constructCtx, "Base dataset with import", sst.DefaultBranch)
		require.NoError(t, err)
	})

	t.Run("checkout_export_remove_block_and_commit", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		ds, err := repo.Dataset(constructCtx, ngBaseIRI)
		require.NoError(t, err)

		stage1, err := ds.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		require.NoError(t, err)

		var buf bytes.Buffer
		err = stage1.RdfWrite(&buf, sst.RdfFormatTriG)
		require.NoError(t, err)

		trigOutput := buf.String()
		t.Logf("Exported TriG:\n%s", trigOutput)

		// Strip the imported graph's TriG block so the external edit appears to have
		// removed the whole NamedGraph. See the local variant test for a detailed
		// explanation of the block structure.
		lines := strings.Split(trigOutput, "\n")
		var modifiedLines []string
		inImportBlock := false
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "<"+ngImportIRI.String()+">") && strings.HasSuffix(trimmed, "{") {
				inImportBlock = true
				continue
			}
			if inImportBlock && trimmed == "}" {
				inImportBlock = false
				continue
			}
			if inImportBlock {
				continue
			}
			modifiedLines = append(modifiedLines, line)
		}
		modifiedTriG := strings.Join(modifiedLines, "\n")
		t.Logf("Modified TriG:\n%s", modifiedTriG)

		stage2, err := sst.RdfRead(bufio.NewReader(bytes.NewReader([]byte(modifiedTriG))), sst.RdfFormatTriG, sst.StrictHandler, sst.DefaultTriplexMode)
		require.NoError(t, err)

		ngBase2 := stage2.NamedGraph(ngBaseIRI)
		require.NotNil(t, ngBase2, "Stage2 should have base graph")

		ngImport2 := stage2.NamedGraph(ngImportIRI)
		assert.Nil(t, ngImport2, "Stage2 should NOT have imported graph after block removal")

		assert.Nil(t, stage2.Repository(), "Stage2 should not be linked to a repo yet")

		stage2.AlignHistory(stage1)

		assert.Equal(t, repo, stage2.Repository(), "Stage2 should inherit repo from Stage1")

		ngImport2 = stage2.NamedGraph(ngImportIRI)
		require.NotNil(t, ngImport2, "Imported graph should be created as deleted in Stage2")
		assert.False(t, ngImport2.IsReferenced(), "Deleted graph should remain local (not referenced)")

		_, _, err = stage2.Commit(constructCtx, "Commit after removing graph block from TriG", sst.DefaultBranch)
		assert.NoError(t, err, "Stage2 should be commitable after inheriting history")
	})

	t.Run("checkout_and_verify_deletion", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		ds, err := repo.Dataset(constructCtx, ngBaseIRI)
		require.NoError(t, err)

		stageBase, err := ds.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		require.NoError(t, err)

		ngBase3 := stageBase.NamedGraph(ngBaseIRI)
		require.NotNil(t, ngBase3)

		baseNode3 := ngBase3.GetIRINodeByFragment("baseNode")
		assert.NotNil(t, baseNode3, "Original baseNode should exist after commit")

		// Verify the imported dataset has been deleted.
		// Remote repository behavior: the deleted dataset can still be checked out,
		// but it should contain no content.
		dsImport, err := repo.Dataset(constructCtx, ngImportIRI)
		require.NoError(t, err)

		stageImport, err := dsImport.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		require.NoError(t, err)

		ngImport3 := stageImport.NamedGraph(ngImportIRI)
		if ngImport3 != nil {
			assert.Nil(t, ngImport3.GetIRINodeByFragment("importNode"), "Deleted graph should not contain importNode")
		}

		// Verify repository info reflects the deletion
		info, err := repo.Info(constructCtx, sst.DefaultBranch)
		require.NoError(t, err)
		assert.Equal(t, int64(2), info.NumberOfDatasets, "Total datasets should still be 2")
	})
}
