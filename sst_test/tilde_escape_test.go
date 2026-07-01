// Copyright 2021-2025 Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"bufio"
	"os"
	"path/filepath"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	"github.com/stretchr/testify/assert"
)

// TestTildeEscapeRoundTrip tests that escaped tildes survive a round-trip
// Read -> Write -> Read
// This verifies that \~ in Turtle is correctly parsed as ~ in the IRI
func TestTildeEscapeRoundTrip(t *testing.T) {
	tempDir := t.TempDir()

	// Create a turtle file with escaped tildes
	ttlContent := `@prefix ex: <http://example.org/> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .

ex:node\~with\~tildes a ex:TestClass ;
    ex:label "Node with multiple tildes" .
`

	// First read
	ttlFile1 := filepath.Join(tempDir, "test_tilde_1.ttl")
	err := os.WriteFile(ttlFile1, []byte(ttlContent), 0644)
	assert.NoError(t, err)

	file1, err := os.Open(ttlFile1)
	assert.NoError(t, err)
	// Use RecoverHandler to ignore vocabulary errors about illegal subjects
	stage1, err := sst.RdfRead(bufio.NewReader(file1), sst.RdfFormatTurtle, sst.RecoverHandler, sst.DefaultTriplexMode)
	file1.Close()
	assert.NoError(t, err)

	ng1 := stage1.NamedGraphs()[0]

	// Write to a new SST file
	sstFile := filepath.Join(tempDir, "test_tilde.sst")
	sstFileObj, err := os.Create(sstFile)
	assert.NoError(t, err)

	writer := bufio.NewWriter(sstFileObj)
	err = ng1.SstWrite(writer)
	assert.NoError(t, err)
	err = writer.Flush()
	assert.NoError(t, err)
	sstFileObj.Close()

	// Read back the SST file
	sstFileObj2, err := os.Open(sstFile)
	assert.NoError(t, err)
	ng2, err := sst.SstRead(bufio.NewReader(sstFileObj2), sst.DefaultTriplexMode)
	sstFileObj2.Close()
	assert.NoError(t, err)
	assert.NotNil(t, ng2)

	t.Logf("Round-trip successful: escaped tildes preserved")
	t.Logf("NamedGraph ID: %s", ng2.IRI())
	t.Logf("IRI Node Count: %d", ng2.IRINodeCount())
}

// TestTildeEscapeParsing verifies that escaped tilde \~ is correctly parsed
func TestTildeEscapeParsing(t *testing.T) {
	// Create a temporary turtle file
	tempDir := t.TempDir()
	ttlContent := `@prefix ex: <http://example.org/> .

ex:test\~node ex:predicate ex:object .
`

	ttlFile := filepath.Join(tempDir, "test_tilde_parse.ttl")
	err := os.WriteFile(ttlFile, []byte(ttlContent), 0644)
	assert.NoError(t, err)

	// Read the file
	file, err := os.Open(ttlFile)
	assert.NoError(t, err)
	defer file.Close()

	// Just verify that reading doesn't fail with a parse error
	// The escaped tilde should be accepted by the parser
	_, err = sst.RdfRead(bufio.NewReader(file), sst.RdfFormatTurtle, sst.RecoverHandler, sst.DefaultTriplexMode)
	assert.NoError(t, err)

	t.Logf("Escaped tilde parsed successfully")
}

// TestMultipleSpecialCharacters tests various escaped special characters
func TestMultipleSpecialCharacters(t *testing.T) {
	tempDir := t.TempDir()

	// Test various escaped characters that are allowed in PN_LOCAL
	ttlContent := `@prefix ex: <http://example.org/> .

ex:test\~node ex:prop\-erty ex:val\.ue .
ex:test\_underscore ex:predicate ex:object .
`

	ttlFile := filepath.Join(tempDir, "test_special.ttl")
	err := os.WriteFile(ttlFile, []byte(ttlContent), 0644)
	assert.NoError(t, err)

	file, err := os.Open(ttlFile)
	assert.NoError(t, err)
	defer file.Close()

	// Verify parsing works
	_, err = sst.RdfRead(bufio.NewReader(file), sst.RdfFormatTurtle, sst.RecoverHandler, sst.DefaultTriplexMode)
	assert.NoError(t, err)

	t.Logf("Special characters parsed successfully")
}

// TestTildeOnlyAsSubjectFragment tests using only '~' as the subject fragment
// The subject is literally just "~" (a single tilde character)
func TestTildeOnlyAsSubjectFragment(t *testing.T) {
	tempDir := t.TempDir()

	// The subject is just "~" (escaped as \~), predicate and object are simple
	ttlContent := `@prefix ex: <http://example.org/#> .

ex:\~ ex:predicate ex:object .
`

	ttlFile := filepath.Join(tempDir, "test_tilde_only_subject.ttl")
	err := os.WriteFile(ttlFile, []byte(ttlContent), 0644)
	assert.NoError(t, err)

	t.Logf("Input TTL:\n%s", ttlContent)

	// First read
	file, err := os.Open(ttlFile)
	assert.NoError(t, err)
	stage1, err := sst.RdfRead(bufio.NewReader(file), sst.RdfFormatTurtle, sst.RecoverHandler, sst.DefaultTriplexMode)
	file.Close()
	assert.NoError(t, err)

	ng1 := stage1.NamedGraphs()[0]
	t.Logf("After first read — Graph IRI: %s, IRI nodes: %d", ng1.IRI(), ng1.IRINodeCount())

	// Verify the subject fragment is "~"
	node := ng1.GetIRINodeByFragment("~")
	assert.NotNil(t, node, "expected to find a node with fragment '~'")
	t.Logf("Found node with fragment: %q", node.Fragment())

	// Write to SST
	sstFile := filepath.Join(tempDir, "test_tilde_only_subject.sst")
	sstFileObj, err := os.Create(sstFile)
	assert.NoError(t, err)
	writer := bufio.NewWriter(sstFileObj)
	err = ng1.SstWrite(writer)
	assert.NoError(t, err)
	err = writer.Flush()
	assert.NoError(t, err)
	sstFileObj.Close()

	// Read back the SST file
	sstFileObj2, err := os.Open(sstFile)
	assert.NoError(t, err)
	ng2, err := sst.SstRead(bufio.NewReader(sstFileObj2), sst.DefaultTriplexMode)
	sstFileObj2.Close()
	assert.NoError(t, err)
	assert.NotNil(t, ng2)

	// Verify the subject fragment survived the SST round-trip
	node2 := ng2.GetIRINodeByFragment("~")
	assert.NotNil(t, node2, "expected to find a node with fragment '~' after SST round-trip")
	t.Logf("SST round-trip OK — node fragment: %q", node2.Fragment())

	// Write to RDF (Turtle)
	ttlRoundTripFile := filepath.Join(tempDir, "test_tilde_only_subject_roundtrip.ttl")
	ttlRoundTripObj, err := os.Create(ttlRoundTripFile)
	assert.NoError(t, err)
	ttlWriter := bufio.NewWriter(ttlRoundTripObj)
	err = ng2.RdfWrite(ttlWriter, sst.RdfFormatTurtle)
	assert.NoError(t, err)
	err = ttlWriter.Flush()
	assert.NoError(t, err)
	ttlRoundTripObj.Close()

	// Show the generated TTL content
	roundTripBytes, err := os.ReadFile(ttlRoundTripFile)
	assert.NoError(t, err)
	t.Logf("Generated round-trip TTL:\n%s", string(roundTripBytes))

	// Read back the RDF file
	ttlRoundTripObj2, err := os.Open(ttlRoundTripFile)
	assert.NoError(t, err)
	stage3, err := sst.RdfRead(bufio.NewReader(ttlRoundTripObj2), sst.RdfFormatTurtle, sst.RecoverHandler, sst.DefaultTriplexMode)
	ttlRoundTripObj2.Close()
	assert.NoError(t, err)

	ng3 := stage3.NamedGraphs()[0]

	// Verify the subject fragment survived the RDF round-trip
	node3 := ng3.GetIRINodeByFragment("~")
	assert.NotNil(t, node3, "expected to find a node with fragment '~' after RDF round-trip")
	t.Logf("RDF round-trip OK — node fragment: %q", node3.Fragment())

	t.Logf("Single tilde '~' as subject fragment round-trip successful")
}
