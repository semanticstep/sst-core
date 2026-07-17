// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/vocabularies/countrycodes"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to write Stage to TriG file
func writeStageToTriGFile(stage sst.Stage, fileName string) error {
	f, err := os.Create(fileName + ".trig")
	if err != nil {
		return err
	}
	defer f.Close()

	return stage.RdfWrite(f, sst.RdfFormatTriG)
}

// Helper function to read TriG file and return Stage
func readTriGFile(fileName string) (sst.Stage, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return sst.RdfRead(bufio.NewReader(file), sst.RdfFormatTriG, sst.StrictHandler, sst.DefaultTriplexMode)
}

// Test_StageRdfWriteTriG_SingleGraph tests writing a single named graph to TriG format
func Test_StageRdfWriteTriG_SingleGraph(t *testing.T) {
	testName := filepath.Join(t.TempDir(), t.Name())

	t.Run("write_single_graph", func(t *testing.T) {
		stage := sst.OpenStage(sst.DefaultTriplexMode)

		ngID := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361")
		ng := stage.CreateNamedGraph(sst.IRI(ngID.URN()))

		// Create some data
		jane := ng.CreateIRINode("Jane", lci.Person)
		organization := ng.CreateIRINode("ECT", lci.Organization)
		organization.AddStatement(rdfs.Label, sst.String("ECT"))

		workFor := ng.CreateIRINode("workFor", rdf.Property)
		workFor.AddStatement(rdfs.Domain, lci.Person)
		workFor.AddStatement(rdfs.Range, lci.Organization)

		jane.AddStatement(workFor, organization)

		// Write stage to TriG
		err := writeStageToTriGFile(stage, testName)
		require.NoError(t, err)

		// Verify file was created
		_, err = os.Stat(testName + ".trig")
		assert.NoError(t, err)
	})
}

// Test_StageRdfWriteTriG_MultipleGraphs tests writing multiple named graphs to TriG format
func Test_StageRdfWriteTriG_MultipleGraphs(t *testing.T) {
	testName := filepath.Join(t.TempDir(), t.Name())

	t.Run("write_multiple_graphs", func(t *testing.T) {
		stage := sst.OpenStage(sst.DefaultTriplexMode)

		// Create first named graph - People
		ngID1 := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1")
		ng1 := stage.CreateNamedGraph(sst.IRI(ngID1.URN()))

		jane := ng1.CreateIRINode("Jane", lci.Person)
		john := ng1.CreateIRINode("John", lci.Person)
		jane.AddStatement(rdfs.Label, sst.String("Jane Doe"))
		john.AddStatement(rdfs.Label, sst.String("John Doe"))

		// Create second named graph - Organizations
		ngID2 := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb2")
		ng2 := stage.CreateNamedGraph(sst.IRI(ngID2.URN()))

		org1 := ng2.CreateIRINode("CompanyA", lci.Organization)
		org2 := ng2.CreateIRINode("CompanyB", lci.Organization)
		org1.AddStatement(rdfs.Label, sst.String("Company A"))
		org2.AddStatement(rdfs.Label, sst.String("Company B"))

		// Create third named graph - Relationships
		ngID3 := uuid.MustParse("cccccccc-cccc-cccc-cccc-ccccccccccc3")
		ng3 := stage.CreateNamedGraph(sst.IRI(ngID3.URN()))

		workFor := ng3.CreateIRINode("workFor", rdf.Property)
		workFor.AddStatement(rdfs.Domain, lci.Person)
		workFor.AddStatement(rdfs.Range, lci.Organization)

		// Write stage to TriG
		err := writeStageToTriGFile(stage, testName)
		require.NoError(t, err)

		// Verify file was created and read content
		content, err := os.ReadFile(testName + ".trig")
		require.NoError(t, err)

		trigStr := string(content)
		// Verify all three graph IRIs appear in output
		assert.Contains(t, trigStr, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1")
		assert.Contains(t, trigStr, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb2")
		assert.Contains(t, trigStr, "cccccccc-cccc-cccc-cccc-ccccccccccc3")

		// Verify graph block structure
		assert.Contains(t, trigStr, "{")
		assert.Contains(t, trigStr, "}")

		// Verify prefixes
		assert.Contains(t, trigStr, "@prefix")
		assert.Contains(t, trigStr, "rdf:")
		assert.Contains(t, trigStr, "owl:")
	})
}

// Test_StageRdfWriteTriG_WithBlankNodes tests TriG output with blank nodes
func Test_StageRdfWriteTriG_WithBlankNodes(t *testing.T) {
	testName := filepath.Join(t.TempDir(), t.Name())

	t.Run("write_with_blank_nodes", func(t *testing.T) {
		stage := sst.OpenStage(sst.DefaultTriplexMode)

		ngID := uuid.MustParse("dddddddd-dddd-dddd-dddd-ddddddddddd4")
		ng := stage.CreateNamedGraph(sst.IRI(ngID.URN()))

		jane := ng.CreateIRINode("Jane", lci.Person)

		// Create blank node for organization
		blankOrganization := ng.CreateBlankNode(lci.Organization)
		blankOrganization.AddStatement(rdfs.Label, sst.String("Anonymous Org"))

		workFor := ng.CreateIRINode("workFor", rdf.Property)
		workFor.AddStatement(rdfs.Domain, lci.Person)
		workFor.AddStatement(rdfs.Range, lci.Organization)

		jane.AddStatement(workFor, blankOrganization)

		// Write stage to TriG
		err := writeStageToTriGFile(stage, testName)
		require.NoError(t, err)

		// Read and verify content
		content, err := os.ReadFile(testName + ".trig")
		require.NoError(t, err)

		trigStr := string(content)
		// Verify blank node notation (inline blank nodes use [ ] syntax in Turtle/TriG)
		assert.Contains(t, trigStr, "[")
		assert.Contains(t, trigStr, "]")
		assert.Contains(t, trigStr, "Anonymous Org")
	})
}

// Test_StageRdfWriteTriG_WithCollections tests TriG output with RDF collections
func Test_StageRdfWriteTriG_WithCollections(t *testing.T) {
	testName := filepath.Join(t.TempDir(), t.Name())

	t.Run("write_with_collections", func(t *testing.T) {
		stage := sst.OpenStage(sst.DefaultTriplexMode)

		ngID := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeee5")
		ng := stage.CreateNamedGraph(sst.IRI(ngID.URN()))

		john := ng.CreateIRINode("John", lci.Person)
		jane := ng.CreateIRINode("Jane", lci.Person)
		adam := ng.CreateIRINode("Adam", lci.Person)

		// Create collection of friends
		friends := ng.CreateCollection(jane, adam)
		hasFriend := ng.CreateIRINode("hasFriend", rdf.Property)
		john.AddStatement(hasFriend, friends)

		// Write stage to TriG
		err := writeStageToTriGFile(stage, testName)
		require.NoError(t, err)

		// Read and verify content
		content, err := os.ReadFile(testName + ".trig")
		require.NoError(t, err)

		trigStr := string(content)
		// Verify collection notation (parentheses)
		assert.Contains(t, trigStr, "(")
		assert.Contains(t, trigStr, ")")
	})
}

// Test_StageRdfWriteTriG_WithLiteralCollections tests TriG output with literal collections
func Test_StageRdfWriteTriG_WithLiteralCollections(t *testing.T) {
	testName := filepath.Join(t.TempDir(), t.Name())

	t.Run("write_with_literal_collections", func(t *testing.T) {
		stage := sst.OpenStage(sst.DefaultTriplexMode)

		ngID := uuid.MustParse("ffffffff-ffff-ffff-ffff-fffffffffff6")
		ng := stage.CreateNamedGraph(sst.IRI(ngID.URN()))

		white := ng.CreateIRINode("white", rep.ColourRGB)
		lc1 := sst.NewLiteralCollection(sst.Integer(255), sst.Integer(255), sst.Integer(255))
		white.AddStatement(rep.Rgb, lc1)

		// Write stage to TriG
		err := writeStageToTriGFile(stage, testName)
		require.NoError(t, err)

		// Read and verify content
		content, err := os.ReadFile(testName + ".trig")
		require.NoError(t, err)

		trigStr := string(content)
		// Verify literal values
		assert.Contains(t, trigStr, "255")
	})
}

// Test_StageRdfWriteTriG_GraphWithImports tests TriG output with imported graphs
func Test_StageRdfWriteTriG_GraphWithImports(t *testing.T) {
	testName := filepath.Join(t.TempDir(), t.Name())

	t.Run("write_with_imports", func(t *testing.T) {
		stage := sst.OpenStage(sst.DefaultTriplexMode)

		// Create base vocabulary graph
		ngID1 := uuid.MustParse("11111111-1111-1111-1111-111111111111")
		ng1 := stage.CreateNamedGraph(sst.IRI(ngID1.URN()))

		workFor := ng1.CreateIRINode("workFor", rdf.Property)
		workFor.AddStatement(rdfs.Domain, lci.Person)
		workFor.AddStatement(rdfs.Range, lci.Organization)

		// Create data graph that imports vocabulary
		ngID2 := uuid.MustParse("22222222-2222-2222-2222-222222222222")
		ng2 := stage.CreateNamedGraph(sst.IRI(ngID2.URN()))
		ng2.AddImport(ng1)

		jane := ng2.CreateIRINode("Jane", lci.Person)
		org := ng2.CreateIRINode("ECT", lci.Organization)
		jane.AddStatement(workFor, org)

		// Write stage to TriG
		err := writeStageToTriGFile(stage, testName)
		require.NoError(t, err)

		// Read and verify content
		content, err := os.ReadFile(testName + ".trig")
		require.NoError(t, err)

		trigStr := string(content)
		// Verify both graphs are present
		assert.Contains(t, trigStr, "11111111-1111-1111-1111-111111111111")
		assert.Contains(t, trigStr, "22222222-2222-2222-2222-222222222222")
	})
}

// Test_StageRdfWriteTriG_ComplexScenario tests a complex scenario with multiple features
func Test_StageRdfWriteTriG_ComplexScenario(t *testing.T) {
	testName := filepath.Join(t.TempDir(), t.Name())

	t.Run("write_complex_scenario", func(t *testing.T) {
		stage := sst.OpenStage(sst.DefaultTriplexMode)

		// Graph 1: People and their basic info
		ngID1 := uuid.MustParse("aaaaaaaa-1111-1111-1111-111111111111")
		ng1 := stage.CreateNamedGraph(sst.IRI(ngID1.URN()))

		john := ng1.CreateIRINode("John", lci.Person)
		jane := ng1.CreateIRINode("Jane", lci.Person)
		adam := ng1.CreateIRINode("Adam", lci.Person)

		john.AddStatement(rdfs.Label, sst.String("John Doe"))
		jane.AddStatement(rdfs.Label, sst.String("Jane Doe"))
		adam.AddStatement(rdfs.Label, sst.String("Adam Smith"))

		// Graph 2: Organizations
		ngID2 := uuid.MustParse("bbbbbbbb-2222-2222-2222-222222222222")
		ng2 := stage.CreateNamedGraph(sst.IRI(ngID2.URN()))

		org1 := ng2.CreateIRINode("CompanyA", lci.Organization)
		org2 := ng2.CreateIRINode("CompanyB", lci.Organization)
		blankOrg := ng2.CreateBlankNode(lci.Organization)

		org1.AddStatement(rdfs.Label, sst.String("Company A"))
		org2.AddStatement(rdfs.Label, sst.String("Company B"))
		blankOrg.AddStatement(rdfs.Label, sst.String("Stealth Startup"))

		// Graph 3: Relationships using collections
		ngID3 := uuid.MustParse("cccccccc-3333-3333-3333-333333333333")
		ng3 := stage.CreateNamedGraph(sst.IRI(ngID3.URN()))

		worksAt := ng3.CreateIRINode("worksAt", rdf.Property)
		hasFriends := ng3.CreateIRINode("hasFriends", rdf.Property)
		hasSkill := ng3.CreateIRINode("hasSkill", rdf.Property)

		// John works at CompanyA and has friends Jane and Adam
		johnRef := ng3.CreateIRINode("John")
		companyARef := ng3.CreateIRINode("CompanyA")
		friends := ng3.CreateCollection(jane, adam)

		johnRef.AddStatement(worksAt, companyARef)
		johnRef.AddStatement(hasFriends, friends)

		// Skills as literal collection
		skills := sst.NewLiteralCollection(sst.String("Go"), sst.String("Python"), sst.String("RDF"))
		johnRef.AddStatement(hasSkill, skills)

		// Graph 4: Geographic data with referenced vocab
		ngID4 := uuid.MustParse("dddddddd-4444-4444-4444-444444444444")
		ng4 := stage.CreateNamedGraph(sst.IRI(ngID4.URN()))

		johnGeo := ng4.CreateIRINode("John")
		janeGeo := ng4.CreateIRINode("Jane")

		johnGeo.AddStatement(lci.PartOf, countrycodes.Cn)
		janeGeo.AddStatement(lci.PartOf, countrycodes.De)

		// Write stage to TriG
		err := writeStageToTriGFile(stage, testName)
		require.NoError(t, err)

		// Read and verify content
		content, err := os.ReadFile(testName + ".trig")
		require.NoError(t, err)

		trigStr := string(content)

		// Verify all 4 graphs are present
		assert.Contains(t, trigStr, "aaaaaaaa-1111-1111-1111-111111111111")
		assert.Contains(t, trigStr, "bbbbbbbb-2222-2222-2222-222222222222")
		assert.Contains(t, trigStr, "cccccccc-3333-3333-3333-333333333333")
		assert.Contains(t, trigStr, "dddddddd-4444-4444-4444-444444444444")

		// Verify graph blocks
		assert.Contains(t, trigStr, "{")
		assert.Contains(t, trigStr, "}")

		// Verify data
		assert.Contains(t, trigStr, "John Doe")
		assert.Contains(t, trigStr, "Jane Doe")
		assert.Contains(t, trigStr, "Adam Smith")
		assert.Contains(t, trigStr, "Company A")
		assert.Contains(t, trigStr, "Stealth Startup")
	})
}

// Test_StageRdfWriteTriG_EmptyStage tests writing an empty stage
func Test_StageRdfWriteTriG_EmptyStage(t *testing.T) {
	testName := filepath.Join(t.TempDir(), t.Name())

	t.Run("write_empty_stage", func(t *testing.T) {
		stage := sst.OpenStage(sst.DefaultTriplexMode)

		// Write empty stage to TriG - should handle gracefully
		err := writeStageToTriGFile(stage, testName)
		require.NoError(t, err)

		// Read content
		content, err := os.ReadFile(testName + ".trig")
		require.NoError(t, err)

		// Empty stage produces empty output (no graphs to write)
		trigStr := string(content)
		// Empty output is acceptable for empty stage
		_ = trigStr
	})
}

// Test_StageRdfWriteTriG_EmptyNamedGraph tests writing a stage with empty named graphs
func Test_StageRdfWriteTriG_EmptyNamedGraph(t *testing.T) {
	testName := filepath.Join(t.TempDir(), t.Name())

	t.Run("write_empty_named_graph", func(t *testing.T) {
		stage := sst.OpenStage(sst.DefaultTriplexMode)

		// Create a named graph but don't add any nodes
		ngID := uuid.MustParse("99999999-9999-9999-9999-999999999999")
		_ = stage.CreateNamedGraph(sst.IRI(ngID.URN()))

		// Write stage to TriG
		err := writeStageToTriGFile(stage, testName)
		require.NoError(t, err)

		// Read content
		content, err := os.ReadFile(testName + ".trig")
		require.NoError(t, err)

		trigStr := string(content)
		// Should contain the graph IRI and empty block
		assert.Contains(t, trigStr, "99999999-9999-9999-9999-999999999999")
		assert.Contains(t, trigStr, "{")
		assert.Contains(t, trigStr, "}")
	})
}

// Test_NamedGraphRdfWriteTurtleVsStageRdfWriteTriG compares Turtle vs TriG output
func Test_NamedGraphRdfWriteTurtleVsStageRdfWriteTriG(t *testing.T) {
	testName := filepath.Join(t.TempDir(), t.Name())

	t.Run("compare_turtle_vs_trig", func(t *testing.T) {
		stage := sst.OpenStage(sst.DefaultTriplexMode)

		ngID := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
		ng := stage.CreateNamedGraph(sst.IRI(ngID.URN()))

		jane := ng.CreateIRINode("Jane", lci.Person)
		jane.AddStatement(rdfs.Label, sst.String("Jane Doe"))

		// Write single graph using NamedGraph.RdfWrite (Turtle)
		turtleFile := testName + "_single.ttl"
		f, err := os.Create(turtleFile)
		require.NoError(t, err)
		err = ng.RdfWrite(f, sst.RdfFormatTurtle)
		f.Close()
		require.NoError(t, err)

		// Write using Stage.RdfWrite (TriG)
		trigFile := testName + "_single.trig"
		f2, err := os.Create(trigFile)
		require.NoError(t, err)
		err = stage.RdfWrite(f2, sst.RdfFormatTriG)
		f2.Close()
		require.NoError(t, err)

		// Read both files
		turtleContent, err := os.ReadFile(turtleFile)
		require.NoError(t, err)

		trigContent, err := os.ReadFile(trigFile)
		require.NoError(t, err)

		turtleStr := string(turtleContent)
		trigStr := string(trigContent)

		// Both should contain the data
		assert.Contains(t, turtleStr, "Jane Doe")
		assert.Contains(t, trigStr, "Jane Doe")

		// TriG should have graph blocks, Turtle should not
		assert.Contains(t, trigStr, "{")
		assert.Contains(t, trigStr, "}")
		assert.NotContains(t, turtleStr, "{")
		assert.NotContains(t, turtleStr, "}")
	})
}

// Test_StageRdfWriteTriG_MultipleGraphsBlankNodeLabels tests how blank node labels
// are generated when multiple NamedGraphs each contain blank nodes.
// Per RDF 1.1 TriG, a blank node label represents the same blank node throughout
// the document, so SST must ensure labels are unique across graph blocks.
func Test_StageRdfWriteTriG_MultipleGraphsBlankNodeLabels(t *testing.T) {
	t.Run("independent_blank_nodes_in_different_graphs", func(t *testing.T) {
		testName := filepath.Join(t.TempDir(), t.Name())
		err := os.MkdirAll(filepath.Dir(testName), 0755)
		require.NoError(t, err)
		stage := sst.OpenStage(sst.DefaultTriplexMode)

		// Graph 1: a blank node that is not referenced by any other node
		// (useCnt == 0, therefore cannot be inlined and will be emitted as _:b0)
		ngID1 := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1")
		ng1 := stage.CreateNamedGraph(sst.IRI(ngID1.URN()))
		b1 := ng1.CreateBlankNode()
		b1.AddStatement(rdfs.Label, sst.String("Graph1 Blank"))

		// Graph 2: another independent blank node
		ngID2 := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb2")
		ng2 := stage.CreateNamedGraph(sst.IRI(ngID2.URN()))
		b2 := ng2.CreateBlankNode()
		b2.AddStatement(rdfs.Label, sst.String("Graph2 Blank"))

		err = writeStageToTriGFile(stage, testName)
		require.NoError(t, err)

		content, err := os.ReadFile(testName + ".trig")
		require.NoError(t, err)
		trigOutput := string(content)
		t.Logf("TriG output:\n%s", trigOutput)

		// Both graphs should be present
		assert.Contains(t, trigOutput, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1")
		assert.Contains(t, trigOutput, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb2")
		assert.Contains(t, trigOutput, "Graph1 Blank")
		assert.Contains(t, trigOutput, "Graph2 Blank")
	})

	t.Run("blank_nodes_referenced_multiple_times", func(t *testing.T) {
		testName := filepath.Join(t.TempDir(), t.Name())
		err := os.MkdirAll(filepath.Dir(testName), 0755)
		require.NoError(t, err)
		stage := sst.OpenStage(sst.DefaultTriplexMode)

		// Graph 1: blank node referenced by TWO IRI nodes (useCnt > 1 => not inlined)
		ngID1 := uuid.MustParse("cccccccc-cccc-cccc-cccc-ccccccccccc3")
		ng1 := stage.CreateNamedGraph(sst.IRI(ngID1.URN()))
		b1 := ng1.CreateBlankNode()
		b1.AddStatement(rdfs.Label, sst.String("Shared Blank 1"))
		n1a := ng1.CreateIRINode("Node1A")
		n1b := ng1.CreateIRINode("Node1B")
		n1a.AddStatement(rdf.Value, b1)
		n1b.AddStatement(rdf.Value, b1)

		// Graph 2: another blank node referenced by TWO IRI nodes
		ngID2 := uuid.MustParse("dddddddd-dddd-dddd-dddd-ddddddddddd4")
		ng2 := stage.CreateNamedGraph(sst.IRI(ngID2.URN()))
		b2 := ng2.CreateBlankNode()
		b2.AddStatement(rdfs.Label, sst.String("Shared Blank 2"))
		n2a := ng2.CreateIRINode("Node2A")
		n2b := ng2.CreateIRINode("Node2B")
		n2a.AddStatement(rdf.Value, b2)
		n2b.AddStatement(rdf.Value, b2)

		err = writeStageToTriGFile(stage, testName)
		require.NoError(t, err)

		content, err := os.ReadFile(testName + ".trig")
		require.NoError(t, err)
		trigOutput := string(content)
		t.Logf("TriG output:\n%s", trigOutput)

		// Both graphs should be present
		assert.Contains(t, trigOutput, "cccccccc-cccc-cccc-cccc-ccccccccccc3")
		assert.Contains(t, trigOutput, "dddddddd-dddd-dddd-dddd-ddddddddddd4")
		assert.Contains(t, trigOutput, "Shared Blank 1")
		assert.Contains(t, trigOutput, "Shared Blank 2")
	})

	t.Run("blank_node_label_collision_demo", func(t *testing.T) {
		testName := filepath.Join(t.TempDir(), t.Name())
		err := os.MkdirAll(filepath.Dir(testName), 0755)
		require.NoError(t, err)
		// This test explicitly demonstrates the current collision issue.
		// We create two graphs, each with a blank node that will receive
		// the label _:b0 in its own writerContext. Under TriG semantics
		// these would be interpreted as the SAME blank node, which is wrong
		// because they are distinct SST nodes.
		stage := sst.OpenStage(sst.DefaultTriplexMode)

		ngID1 := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeee5")
		ng1 := stage.CreateNamedGraph(sst.IRI(ngID1.URN()))
		b1 := ng1.CreateBlankNode()
		b1.AddStatement(rdfs.Label, sst.String("Demo Blank 1"))

		ngID2 := uuid.MustParse("ffffffff-ffff-ffff-ffff-fffffffffff6")
		ng2 := stage.CreateNamedGraph(sst.IRI(ngID2.URN()))
		b2 := ng2.CreateBlankNode()
		b2.AddStatement(rdfs.Label, sst.String("Demo Blank 2"))

		err = writeStageToTriGFile(stage, testName)
		require.NoError(t, err)

		content, err := os.ReadFile(testName + ".trig")
		require.NoError(t, err)
		trigOutput := string(content)
		t.Logf("TriG output:\n%s", trigOutput)

		// Count how many times _:b0 appears in the whole document.
		// Currently there will be two (one per graph), which is a collision.
		count := strings.Count(trigOutput, "_:b0")
		t.Logf("Occurrences of _:b0 in output: %d", count)
	})
}

// Test_StageRdfWriteTriG_MemoryBuffer tests writing to memory buffer
func Test_StageRdfWriteTriG_MemoryBuffer(t *testing.T) {
	t.Run("write_to_memory", func(t *testing.T) {
		stage := sst.OpenStage(sst.DefaultTriplexMode)

		ngID1 := uuid.MustParse("11111111-1111-1111-1111-111111111111")
		ng1 := stage.CreateNamedGraph(sst.IRI(ngID1.URN()))
		ng1.CreateIRINode("Node1", lci.Person)

		ngID2 := uuid.MustParse("22222222-2222-2222-2222-222222222222")
		ng2 := stage.CreateNamedGraph(sst.IRI(ngID2.URN()))
		ng2.CreateIRINode("Node2", lci.Organization)

		// Write to memory buffer
		var buf bytes.Buffer
		err := stage.RdfWrite(&buf, sst.RdfFormatTriG)
		require.NoError(t, err)

		trigStr := buf.String()

		// Verify content
		assert.Contains(t, trigStr, "11111111-1111-1111-1111-111111111111")
		assert.Contains(t, trigStr, "22222222-2222-2222-2222-222222222222")
		assert.Contains(t, trigStr, "{")
		assert.Contains(t, trigStr, "}")
		assert.Contains(t, trigStr, "@prefix")
	})
}

// Test_TriGFormatRoundTrip tests that TriG can be written and read back
func Test_TriGFormatRoundTrip(t *testing.T) {
	testName := filepath.Join(t.TempDir(), t.Name())

	t.Run("round_trip_single_graph", func(t *testing.T) {
		// Create initial stage
		stage1 := sst.OpenStage(sst.DefaultTriplexMode)

		ngID := uuid.MustParse("aaaaaaaa-1111-1111-1111-111111111111")
		ng := stage1.CreateNamedGraph(sst.IRI(ngID.URN()))

		jane := ng.CreateIRINode("Jane", lci.Person)
		jane.AddStatement(rdfs.Label, sst.String("Jane Doe"))

		// Write to TriG file
		err := writeStageToTriGFile(stage1, testName+"_single")
		require.NoError(t, err)

		// Read back from TriG file
		stage2, err := readTriGFile(testName + "_single.trig")
		require.NoError(t, err)
		defer stage2.Close()

		// Verify data was preserved
		graphs := stage2.NamedGraphs()
		require.Len(t, graphs, 1)

		// Verify the graph IRI matches
		assert.Equal(t, "urn:uuid:aaaaaaaa-1111-1111-1111-111111111111", string(graphs[0].IRI()))
	})

	t.Run("round_trip_multiple_graphs", func(t *testing.T) {
		// Create initial stage with multiple graphs
		stage1 := sst.OpenStage(sst.DefaultTriplexMode)

		ngID1 := uuid.MustParse("bbbbbbbb-1111-1111-1111-111111111111")
		ng1 := stage1.CreateNamedGraph(sst.IRI(ngID1.URN()))
		jane := ng1.CreateIRINode("Jane", lci.Person)
		jane.AddStatement(rdfs.Label, sst.String("Jane Doe"))

		ngID2 := uuid.MustParse("bbbbbbbb-2222-2222-2222-222222222222")
		ng2 := stage1.CreateNamedGraph(sst.IRI(ngID2.URN()))
		org := ng2.CreateIRINode("CompanyA", lci.Organization)
		org.AddStatement(rdfs.Label, sst.String("Company A"))

		// Write to TriG file
		err := writeStageToTriGFile(stage1, testName+"_multi")
		require.NoError(t, err)

		// Read back from TriG file
		stage2, err := readTriGFile(testName + "_multi.trig")
		require.NoError(t, err)
		defer stage2.Close()

		// Verify data was preserved
		graphs := stage2.NamedGraphs()
		require.Len(t, graphs, 2)

		// Verify graph IRIs
		graphIRIs := make([]string, len(graphs))
		for i, g := range graphs {
			graphIRIs[i] = string(g.IRI())
		}
		assert.Contains(t, graphIRIs, "urn:uuid:bbbbbbbb-1111-1111-1111-111111111111")
		assert.Contains(t, graphIRIs, "urn:uuid:bbbbbbbb-2222-2222-2222-222222222222")
	})

	t.Run("round_trip_complex", func(t *testing.T) {
		// Create initial stage with complex data
		stage1 := sst.OpenStage(sst.DefaultTriplexMode)

		ngID1 := uuid.MustParse("cccccccc-1111-1111-1111-111111111111")
		ng1 := stage1.CreateNamedGraph(sst.IRI(ngID1.URN()))
		john := ng1.CreateIRINode("John", lci.Person)
		jane := ng1.CreateIRINode("Jane", lci.Person)
		john.AddStatement(rdfs.Label, sst.String("John Doe"))
		jane.AddStatement(rdfs.Label, sst.String("Jane Doe"))

		ngID2 := uuid.MustParse("cccccccc-2222-2222-2222-222222222222")
		ng2 := stage1.CreateNamedGraph(sst.IRI(ngID2.URN()))
		org := ng2.CreateIRINode("CompanyA", lci.Organization)
		org.AddStatement(rdfs.Label, sst.String("Company A"))

		// Write to TriG file
		err := writeStageToTriGFile(stage1, testName+"_complex")
		require.NoError(t, err)

		// Read back from TriG file
		stage2, err := readTriGFile(testName + "_complex.trig")
		require.NoError(t, err)
		defer stage2.Close()

		// Verify data was preserved
		graphs := stage2.NamedGraphs()
		require.Len(t, graphs, 2)
	})

	t.Run("round_trip_with_single_import", func(t *testing.T) {
		stage1 := sst.OpenStage(sst.DefaultTriplexMode)

		ngID1 := uuid.MustParse("dddddddd-1111-1111-1111-111111111111")
		ng1 := stage1.CreateNamedGraph(sst.IRI(ngID1.URN()))
		org := ng1.CreateIRINode("Org1", lci.Organization)
		org.AddStatement(rdfs.Label, sst.String("Org One"))

		ngID2 := uuid.MustParse("dddddddd-2222-2222-2222-222222222222")
		ng2 := stage1.CreateNamedGraph(sst.IRI(ngID2.URN()))
		err := ng2.AddImport(ng1)
		require.NoError(t, err)
		person := ng2.CreateIRINode("Person1", lci.Person)
		person.AddStatement(rdfs.Label, sst.String("Person One"))

		// Write to TriG file
		err = writeStageToTriGFile(stage1, testName+"_single_import")
		require.NoError(t, err)

		// Read back from TriG file
		stage2, err := readTriGFile(testName + "_single_import.trig")
		require.NoError(t, err)
		defer stage2.Close()

		// Verify both graphs are present as local graphs
		graphs := stage2.NamedGraphs()
		require.Len(t, graphs, 2)

		// Verify import relationship preserved
		var importerGraph sst.NamedGraph
		for _, g := range graphs {
			if g.IRI().String() == "urn:uuid:dddddddd-2222-2222-2222-222222222222" {
				importerGraph = g
				break
			}
		}
		require.NotNil(t, importerGraph, "Importer graph should be found")
		assert.Len(t, importerGraph.DirectImports(), 1, "Importer should have 1 direct import")
		assert.Equal(t, "urn:uuid:dddddddd-1111-1111-1111-111111111111", string(importerGraph.DirectImports()[0].IRI()))

		// Verify data preserved in both graphs
		for _, g := range graphs {
			switch g.IRI().String() {
			case "urn:uuid:dddddddd-1111-1111-1111-111111111111":
				assert.NotNil(t, g.GetIRINodeByFragment("Org1"))
			case "urn:uuid:dddddddd-2222-2222-2222-222222222222":
				assert.NotNil(t, g.GetIRINodeByFragment("Person1"))
			}
		}
	})

	t.Run("round_trip_with_multiple_imports", func(t *testing.T) {
		stage1 := sst.OpenStage(sst.DefaultTriplexMode)

		ngID1 := uuid.MustParse("eeeeeeee-1111-1111-1111-111111111111")
		ng1 := stage1.CreateNamedGraph(sst.IRI(ngID1.URN()))
		ng1.CreateIRINode("Base1", lci.Organization)

		ngID2 := uuid.MustParse("eeeeeeee-2222-2222-2222-222222222222")
		ng2 := stage1.CreateNamedGraph(sst.IRI(ngID2.URN()))
		ng2.CreateIRINode("Base2", lci.Organization)

		ngID3 := uuid.MustParse("eeeeeeee-3333-3333-3333-333333333333")
		ng3 := stage1.CreateNamedGraph(sst.IRI(ngID3.URN()))
		err := ng3.AddImport(ng1)
		require.NoError(t, err)
		err = ng3.AddImport(ng2)
		require.NoError(t, err)
		ng3.CreateIRINode("Data", lci.Person)

		// Write to TriG file
		err = writeStageToTriGFile(stage1, testName+"_multiple_imports")
		require.NoError(t, err)

		// Read back from TriG file
		stage2, err := readTriGFile(testName + "_multiple_imports.trig")
		require.NoError(t, err)
		defer stage2.Close()

		// Verify all three graphs are present as local graphs
		graphs := stage2.NamedGraphs()
		require.Len(t, graphs, 3)

		// Verify import relationship preserved
		var importerGraph sst.NamedGraph
		for _, g := range graphs {
			if g.IRI().String() == "urn:uuid:eeeeeeee-3333-3333-3333-333333333333" {
				importerGraph = g
				break
			}
		}
		require.NotNil(t, importerGraph, "Importer graph should be found")
		assert.Len(t, importerGraph.DirectImports(), 2, "Importer should have 2 direct imports")

		importIRIs := make([]string, len(importerGraph.DirectImports()))
		for i, imp := range importerGraph.DirectImports() {
			importIRIs[i] = string(imp.IRI())
		}
		assert.Contains(t, importIRIs, "urn:uuid:eeeeeeee-1111-1111-1111-111111111111")
		assert.Contains(t, importIRIs, "urn:uuid:eeeeeeee-2222-2222-2222-222222222222")

		// Verify data preserved in all graphs
		for _, g := range graphs {
			switch g.IRI().String() {
			case "urn:uuid:eeeeeeee-1111-1111-1111-111111111111":
				assert.NotNil(t, g.GetIRINodeByFragment("Base1"))
			case "urn:uuid:eeeeeeee-2222-2222-2222-222222222222":
				assert.NotNil(t, g.GetIRINodeByFragment("Base2"))
			case "urn:uuid:eeeeeeee-3333-3333-3333-333333333333":
				assert.NotNil(t, g.GetIRINodeByFragment("Data"))
			}
		}
	})
}

// Test_StageRdfWriteTriG_ImportedNamedGraph tests TriG output with imported named graphs
func Test_StageRdfWriteTriG_ImportedNamedGraph(t *testing.T) {
	t.Run("single_import_owl_imports_declared", func(t *testing.T) {
		stage := sst.OpenStage(sst.DefaultTriplexMode)

		ngID1 := uuid.MustParse("11111111-1111-1111-1111-111111111111")
		ng1 := stage.CreateNamedGraph(sst.IRI(ngID1.URN()))
		ng1.CreateIRINode("Entity1", lci.Organization)

		ngID2 := uuid.MustParse("22222222-2222-2222-2222-222222222222")
		ng2 := stage.CreateNamedGraph(sst.IRI(ngID2.URN()))
		err := ng2.AddImport(ng1)
		require.NoError(t, err)
		ng2.CreateIRINode("Entity2", lci.Person)

		var buf bytes.Buffer
		err = stage.RdfWrite(&buf, sst.RdfFormatTriG)
		require.NoError(t, err)

		trigStr := buf.String()

		// Both graphs should be present
		assert.Contains(t, trigStr, "11111111-1111-1111-1111-111111111111")
		assert.Contains(t, trigStr, "22222222-2222-2222-2222-222222222222")

		// Graph2 should declare owl:imports for Graph1
		assert.Contains(t, trigStr, "owl:imports")
		assert.Contains(t, trigStr, "urn:uuid:11111111-1111-1111-1111-111111111111")
	})

	t.Run("multiple_imports_all_declared", func(t *testing.T) {
		stage := sst.OpenStage(sst.DefaultTriplexMode)

		ngID1 := uuid.MustParse("11111111-1111-1111-1111-111111111111")
		ng1 := stage.CreateNamedGraph(sst.IRI(ngID1.URN()))

		ngID2 := uuid.MustParse("22222222-2222-2222-2222-222222222222")
		ng2 := stage.CreateNamedGraph(sst.IRI(ngID2.URN()))

		ngID3 := uuid.MustParse("33333333-3333-3333-3333-333333333333")
		ng3 := stage.CreateNamedGraph(sst.IRI(ngID3.URN()))
		err := ng3.AddImport(ng1)
		require.NoError(t, err)
		err = ng3.AddImport(ng2)
		require.NoError(t, err)
		ng3.CreateIRINode("Entity3", lci.Person)

		var buf bytes.Buffer
		err = stage.RdfWrite(&buf, sst.RdfFormatTriG)
		require.NoError(t, err)

		trigStr := buf.String()

		// All three graphs should be present
		assert.Contains(t, trigStr, "11111111-1111-1111-1111-111111111111")
		assert.Contains(t, trigStr, "22222222-2222-2222-2222-222222222222")
		assert.Contains(t, trigStr, "33333333-3333-3333-3333-333333333333")

		// Graph3 should declare owl:imports for both Graph1 and Graph2
		assert.Contains(t, trigStr, "owl:imports")
		assert.Contains(t, trigStr, "urn:uuid:11111111-1111-1111-1111-111111111111")
		assert.Contains(t, trigStr, "urn:uuid:22222222-2222-2222-2222-222222222222")
	})

	t.Run("nested_imports_declared", func(t *testing.T) {
		stage := sst.OpenStage(sst.DefaultTriplexMode)

		ngID1 := uuid.MustParse("11111111-1111-1111-1111-111111111111")
		ng1 := stage.CreateNamedGraph(sst.IRI(ngID1.URN()))

		ngID2 := uuid.MustParse("22222222-2222-2222-2222-222222222222")
		ng2 := stage.CreateNamedGraph(sst.IRI(ngID2.URN()))
		err := ng2.AddImport(ng1)
		require.NoError(t, err)

		ngID3 := uuid.MustParse("33333333-3333-3333-3333-333333333333")
		ng3 := stage.CreateNamedGraph(sst.IRI(ngID3.URN()))
		err = ng3.AddImport(ng2)
		require.NoError(t, err)

		var buf bytes.Buffer
		err = stage.RdfWrite(&buf, sst.RdfFormatTriG)
		require.NoError(t, err)

		trigStr := buf.String()

		// All graphs should be present
		assert.Contains(t, trigStr, "11111111-1111-1111-1111-111111111111")
		assert.Contains(t, trigStr, "22222222-2222-2222-2222-222222222222")
		assert.Contains(t, trigStr, "33333333-3333-3333-3333-333333333333")

		// owl:imports should be declared
		assert.Contains(t, trigStr, "owl:imports")
	})

	t.Run("imported_graph_data_preserved", func(t *testing.T) {
		stage := sst.OpenStage(sst.DefaultTriplexMode)

		ngID1 := uuid.MustParse("11111111-1111-1111-1111-111111111111")
		ng1 := stage.CreateNamedGraph(sst.IRI(ngID1.URN()))
		org := ng1.CreateIRINode("ECT", lci.Organization)
		org.AddStatement(rdfs.Label, sst.String("ECT International"))

		ngID2 := uuid.MustParse("22222222-2222-2222-2222-222222222222")
		ng2 := stage.CreateNamedGraph(sst.IRI(ngID2.URN()))
		err := ng2.AddImport(ng1)
		require.NoError(t, err)
		ng2.CreateIRINode("Jane", lci.Person)

		var buf bytes.Buffer
		err = stage.RdfWrite(&buf, sst.RdfFormatTriG)
		require.NoError(t, err)

		trigStr := buf.String()

		// Imported graph data should be present
		assert.Contains(t, trigStr, "ECT International")

		// The label should appear within the imported graph block
		idxGraph1 := strings.Index(trigStr, "11111111-1111-1111-1111-111111111111")
		idxLabel := strings.Index(trigStr, "ECT International")
		assert.Greater(t, idxLabel, idxGraph1, "Label should appear in imported graph block")
	})

	t.Run("imported_graph_has_no_owl_imports_if_not_importing", func(t *testing.T) {
		stage := sst.OpenStage(sst.DefaultTriplexMode)

		ngID1 := uuid.MustParse("11111111-1111-1111-1111-111111111111")
		ng1 := stage.CreateNamedGraph(sst.IRI(ngID1.URN()))
		ng1.CreateIRINode("Entity1", lci.Organization)

		ngID2 := uuid.MustParse("22222222-2222-2222-2222-222222222222")
		ng2 := stage.CreateNamedGraph(sst.IRI(ngID2.URN()))
		err := ng2.AddImport(ng1)
		require.NoError(t, err)
		ng2.CreateIRINode("Entity2", lci.Person)

		var buf bytes.Buffer
		err = stage.RdfWrite(&buf, sst.RdfFormatTriG)
		require.NoError(t, err)

		trigStr := buf.String()

		// Graph1 does not import anything, so its block should not contain owl:imports
		// Find the graph block for ng1 and verify no owl:imports inside
		graph1Start := strings.Index(trigStr, "11111111-1111-1111-1111-111111111111")
		graph1BlockStart := strings.Index(trigStr[graph1Start:], "{") + graph1Start
		graph1BlockEnd := strings.Index(trigStr[graph1BlockStart:], "}") + graph1BlockStart
		graph1Block := trigStr[graph1BlockStart:graph1BlockEnd]

		assert.NotContains(t, graph1Block, "owl:imports", "Graph1 should not have owl:imports since it imports nothing")
	})
}

// Test_TriGFormatRoundTrip_SingleGraphWithBlankNode tests round-trip of a single
// named graph containing a non-inlined blank node (useCnt == 0, emitted as _:b0).
func Test_TriGFormatRoundTrip_SingleGraphWithBlankNode(t *testing.T) {
	stage1 := sst.OpenStage(sst.DefaultTriplexMode)

	ngID := uuid.MustParse("aaaaaaaa-1111-1111-1111-111111111111")
	ng := stage1.CreateNamedGraph(sst.IRI(ngID.URN()))

	// Blank node with only subject triples (useCnt == 0 => not inlined)
	b := ng.CreateBlankNode()
	b.AddStatement(rdfs.Label, sst.String("Anonymous Org"))

	n := ng.CreateIRINode("Jane", lci.Person)
	n.AddStatement(rdfs.Label, sst.String("Jane Doe"))

	// Write to TriG
	var buf1 bytes.Buffer
	err := stage1.RdfWrite(&buf1, sst.RdfFormatTriG)
	require.NoError(t, err)
	trig1 := buf1.String()
	t.Logf("First write:\n%s", trig1)

	// Verify blank node appears with a label
	assert.Contains(t, trig1, "_:b0")
	assert.Contains(t, trig1, "Anonymous Org")

	// Read back
	stage2, err := sst.RdfRead(bufio.NewReader(bytes.NewReader(buf1.Bytes())), sst.RdfFormatTriG, sst.StrictHandler, sst.DefaultTriplexMode)
	require.NoError(t, err)
	defer stage2.Close()

	graphs := stage2.NamedGraphs()
	require.Len(t, graphs, 1)
	assert.Equal(t, "urn:uuid:aaaaaaaa-1111-1111-1111-111111111111", string(graphs[0].IRI()))

	// --- API validation of in-memory structure ---
	ngRead := graphs[0]

	// Should have 1 blank node (the "Anonymous Org" node)
	assert.Equal(t, 1, ngRead.BlankNodeCount(), "Expected 1 blank node in graph")

	// Should have at least 2 IRI nodes (Jane + NG node with empty fragment)
	assert.GreaterOrEqual(t, ngRead.IRINodeCount(), 2, "Expected at least 2 IRI nodes")

	// Verify Jane node exists
	janeRead := ngRead.GetIRINodeByFragment("Jane")
	require.NotNil(t, janeRead, "Jane node should exist after roundtrip")

	// Verify blank node content via ForBlankNodes API
	var blankLabelFound bool
	err = ngRead.ForBlankNodes(func(b sst.IBNode) error {
		return b.ForAll(func(_ int, sub, pred sst.IBNode, obj sst.Term) error {
			if sub == b && pred.Is(rdfs.Label) && obj.TermKind() == sst.TermKindLiteral {
				if str, ok := obj.(sst.String); ok && str == sst.String("Anonymous Org") {
					blankLabelFound = true
				}
			}
			return nil
		})
	})
	require.NoError(t, err)
	assert.True(t, blankLabelFound, "Blank node should have rdfs:label 'Anonymous Org'")

	// Write again and compare structure
	var buf2 bytes.Buffer
	err = stage2.RdfWrite(&buf2, sst.RdfFormatTriG)
	require.NoError(t, err)
	trig2 := buf2.String()
	t.Logf("Second write after roundtrip:\n%s", trig2)

	assert.Contains(t, trig2, "_:b0")
	assert.Contains(t, trig2, "Anonymous Org")
	assert.Contains(t, trig2, "Jane Doe")
}

// Test_TriGFormatRoundTrip_MultipleGraphsWithBlankNodes tests round-trip of
// multiple named graphs each containing independent blank nodes.
func Test_TriGFormatRoundTrip_MultipleGraphsWithBlankNodes(t *testing.T) {
	stage1 := sst.OpenStage(sst.DefaultTriplexMode)

	// Graph 1: blank node with subject triples only (useCnt == 0)
	ngID1 := uuid.MustParse("bbbbbbbb-1111-1111-1111-111111111111")
	ng1 := stage1.CreateNamedGraph(sst.IRI(ngID1.URN()))
	b1 := ng1.CreateBlankNode()
	b1.AddStatement(rdfs.Label, sst.String("Blank One"))

	// Graph 2: another blank node with subject triples only
	ngID2 := uuid.MustParse("bbbbbbbb-2222-2222-2222-222222222222")
	ng2 := stage1.CreateNamedGraph(sst.IRI(ngID2.URN()))
	b2 := ng2.CreateBlankNode()
	b2.AddStatement(rdfs.Label, sst.String("Blank Two"))

	// Write to TriG
	var buf1 bytes.Buffer
	err := stage1.RdfWrite(&buf1, sst.RdfFormatTriG)
	require.NoError(t, err)
	trig1 := buf1.String()
	t.Logf("First write:\n%s", trig1)

	// Labels should be globally unique: b0 and b1
	assert.Contains(t, trig1, "_:b0")
	assert.Contains(t, trig1, "_:b1")
	assert.Contains(t, trig1, "Blank One")
	assert.Contains(t, trig1, "Blank Two")

	// Read back
	stage2, err := sst.RdfRead(bufio.NewReader(bytes.NewReader(buf1.Bytes())), sst.RdfFormatTriG, sst.StrictHandler, sst.DefaultTriplexMode)
	require.NoError(t, err)
	defer stage2.Close()

	graphs := stage2.NamedGraphs()
	require.Len(t, graphs, 2)

	// --- API validation: each graph should have exactly 1 blank node ---
	for _, g := range graphs {
		assert.Equal(t, 1, g.BlankNodeCount(), "Graph %s should have 1 blank node", g.IRI())
	}

	// Build a map from graph IRI to its blank-node label text
	graphToBlankLabel := make(map[string]string)
	for _, g := range graphs {
		err := g.ForBlankNodes(func(b sst.IBNode) error {
			return b.ForAll(func(_ int, sub, pred sst.IBNode, obj sst.Term) error {
				if sub == b && pred.Is(rdfs.Label) && obj.TermKind() == sst.TermKindLiteral {
					if str, ok := obj.(sst.String); ok {
						graphToBlankLabel[g.IRI().String()] = string(str)
					}
				}
				return nil
			})
		})
		require.NoError(t, err)
	}

	// Verify each graph has the correct blank-node label
	assert.Equal(t, "Blank One", graphToBlankLabel["urn:uuid:bbbbbbbb-1111-1111-1111-111111111111"])
	assert.Equal(t, "Blank Two", graphToBlankLabel["urn:uuid:bbbbbbbb-2222-2222-2222-222222222222"])

	// Write again
	var buf2 bytes.Buffer
	err = stage2.RdfWrite(&buf2, sst.RdfFormatTriG)
	require.NoError(t, err)
	trig2 := buf2.String()
	t.Logf("Second write after roundtrip:\n%s", trig2)

	// After roundtrip, labels must still be unique across the document
	assert.Contains(t, trig2, "_:b0")
	assert.Contains(t, trig2, "_:b1")
	assert.Contains(t, trig2, "Blank One")
	assert.Contains(t, trig2, "Blank Two")
}

// Test_TriGFormatRoundTrip_SharedBlankNodesInMultipleGraphs tests round-trip
// where blank nodes are referenced multiple times (useCnt > 1, not inlined)
// across different named graphs.
func Test_TriGFormatRoundTrip_SharedBlankNodesInMultipleGraphs(t *testing.T) {
	stage1 := sst.OpenStage(sst.DefaultTriplexMode)

	// Graph 1: blank node referenced by two IRI nodes
	ngID1 := uuid.MustParse("cccccccc-1111-1111-1111-111111111111")
	ng1 := stage1.CreateNamedGraph(sst.IRI(ngID1.URN()))
	b1 := ng1.CreateBlankNode()
	b1.AddStatement(rdfs.Label, sst.String("Shared Blank 1"))
	n1a := ng1.CreateIRINode("Node1A")
	n1b := ng1.CreateIRINode("Node1B")
	n1a.AddStatement(rdf.Value, b1)
	n1b.AddStatement(rdf.Value, b1)

	// Graph 2: another blank node referenced by two IRI nodes
	ngID2 := uuid.MustParse("cccccccc-2222-2222-2222-222222222222")
	ng2 := stage1.CreateNamedGraph(sst.IRI(ngID2.URN()))
	b2 := ng2.CreateBlankNode()
	b2.AddStatement(rdfs.Label, sst.String("Shared Blank 2"))
	n2a := ng2.CreateIRINode("Node2A")
	n2b := ng2.CreateIRINode("Node2B")
	n2a.AddStatement(rdf.Value, b2)
	n2b.AddStatement(rdf.Value, b2)

	// Write to TriG
	var buf1 bytes.Buffer
	err := stage1.RdfWrite(&buf1, sst.RdfFormatTriG)
	require.NoError(t, err)
	trig1 := buf1.String()
	t.Logf("First write:\n%s", trig1)

	// Two distinct blank nodes => b0 and b1 should be present
	assert.Contains(t, trig1, "_:b0")
	assert.Contains(t, trig1, "_:b1")
	assert.Contains(t, trig1, "Shared Blank 1")
	assert.Contains(t, trig1, "Shared Blank 2")

	// Read back
	stage2, err := sst.RdfRead(bufio.NewReader(bytes.NewReader(buf1.Bytes())), sst.RdfFormatTriG, sst.StrictHandler, sst.DefaultTriplexMode)
	require.NoError(t, err)
	defer stage2.Close()

	graphs := stage2.NamedGraphs()
	require.Len(t, graphs, 2)

	// --- API validation: each graph should have exactly 1 blank node and 2 IRI nodes ---
	for _, g := range graphs {
		assert.Equal(t, 1, g.BlankNodeCount(), "Graph %s should have 1 blank node", g.IRI())
		assert.GreaterOrEqual(t, g.IRINodeCount(), 3, "Graph %s should have at least 3 IRI nodes (NG + NodeA + NodeB)", g.IRI())
	}

	// Verify graph contents via API
	for _, g := range graphs {
		var blankLabel string
		err := g.ForBlankNodes(func(b sst.IBNode) error {
			return b.ForAll(func(_ int, sub, pred sst.IBNode, obj sst.Term) error {
				if sub == b && pred.Is(rdfs.Label) && obj.TermKind() == sst.TermKindLiteral {
					if str, ok := obj.(sst.String); ok {
						blankLabel = string(str)
					}
				}
				return nil
			})
		})
		require.NoError(t, err)

		switch g.IRI().String() {
		case "urn:uuid:cccccccc-1111-1111-1111-111111111111":
			assert.Equal(t, "Shared Blank 1", blankLabel)
			// Verify IRI nodes Node1A and Node1B exist
			require.NotNil(t, g.GetIRINodeByFragment("Node1A"))
			require.NotNil(t, g.GetIRINodeByFragment("Node1B"))
		case "urn:uuid:cccccccc-2222-2222-2222-222222222222":
			assert.Equal(t, "Shared Blank 2", blankLabel)
			require.NotNil(t, g.GetIRINodeByFragment("Node2A"))
			require.NotNil(t, g.GetIRINodeByFragment("Node2B"))
		}
	}

	// Write again and verify no label collision occurred
	var buf2 bytes.Buffer
	err = stage2.RdfWrite(&buf2, sst.RdfFormatTriG)
	require.NoError(t, err)
	trig2 := buf2.String()
	t.Logf("Second write after roundtrip:\n%s", trig2)

	assert.Contains(t, trig2, "_:b0")
	assert.Contains(t, trig2, "_:b1")
	assert.Contains(t, trig2, "Shared Blank 1")
	assert.Contains(t, trig2, "Shared Blank 2")
}
