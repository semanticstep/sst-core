// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"text/tabwriter"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/vocabularies/qau"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/sso"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func rdfReadThenSstWrite(t *testing.T, version1FilePath string) {
	file, err := os.Open(version1FilePath + ".ttl")
	if err != nil {
		fmt.Printf("Error: Failed to open file '%s': %v\n", version1FilePath, err)
		return
	}
	defer file.Close()

	stage1, err := sst.RdfRead(bufio.NewReader(file), sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
	if err != nil {
		fmt.Printf("Error: Failed to read RDF file: %v\n", err)
		return
	}

	out, err := os.Create(version1FilePath + ".sst")
	if err != nil {
		panic(err)
	}
	defer out.Close()

	err = stage1.NamedGraphs()[0].SstWrite(out)
	if err != nil {
		log.Panic(err)
	}
}

// Helper function to read and write diffs
func readSSTFilesThenWriteDiff(t *testing.T, file1 string, file2 string) []sst.DiffTriple {
	bufioFrom, err := os.Open(file1 + ".sst")
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}
	defer bufioFrom.Close()

	bufioTo, err := os.Open(file2 + ".sst")
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}
	defer bufioTo.Close()

	out, err := os.Create(file1 + "diff.sst")
	if err != nil {
		panic(err)
	}
	defer out.Close()

	diffTriples, err := sst.SstWriteDiff(bufio.NewReader(bufioFrom), bufio.NewReader(bufioTo), out, true)
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, val := range diffTriples {
		flagStr := "="
		if val.Flag < 0 {
			flagStr = "-"
		}
		if val.Flag > 0 {
			flagStr = "+"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", flagStr, val.Sub, val.Pred, val.Obj)
	}
	tw.Flush()
	if err != nil {
		t.Fatalf("failed to write diff: %v", err)
	}
	return diffTriples
}

func readThenApplyDiff(t *testing.T, baseFile string) (ng sst.NamedGraph) {
	fileFrom, err := os.Open(baseFile + ".sst")
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}
	defer fileFrom.Close()

	fileDiff, err := os.Open(baseFile + "diff.sst")
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}
	defer fileDiff.Close()

	ng, err = sst.SstReadDiff(sst.DefaultTriplexMode, bufio.NewReader(fileFrom), bufio.NewReader(fileDiff))
	if err != nil {
		panic(err)
	}
	ng.PrintTriples()
	return ng
}

func TestWriteDiffRdfReadV1V2(t *testing.T) {
	testName := filepath.Join("./testdata/", "TestWriteDiffRdfRead")

	version1FilePath := testName + "version1"
	version2FilePath := testName + "version2"

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(version1FilePath + ".sst")
		os.Remove(version2FilePath + ".sst")
		os.Remove(version1FilePath + "diff.sst")
	})

	t.Run("read TTLs", func(t *testing.T) {
		rdfReadThenSstWrite(t, version1FilePath)
		rdfReadThenSstWrite(t, version2FilePath)
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		diffTriples := readSSTFilesThenWriteDiff(t, version1FilePath, version2FilePath)
		assert.Equal(t, sst.DiffTripleFlag(0), diffTriples[0].Flag)
		assert.Equal(t, ":uuid1", diffTriples[0].Sub)
		assert.Equal(t, "sso:occurrenceQuantity", diffTriples[0].Pred)
		assert.Equal(t, "_:b0", diffTriples[0].Obj)

		assert.Equal(t, sst.DiffTripleFlag(-1), diffTriples[1].Flag)
		assert.Equal(t, "_:b0", diffTriples[1].Sub)
		assert.Equal(t, "qau:metre", diffTriples[1].Pred)
		assert.Equal(t, "5.00", diffTriples[1].Obj)

		assert.Equal(t, sst.DiffTripleFlag(1), diffTriples[2].Flag)
		assert.Equal(t, "", diffTriples[2].Sub)
		assert.Equal(t, "", diffTriples[2].Pred)
		assert.Equal(t, "7.00", diffTriples[2].Obj)

		assert.Equal(t, 3, len(diffTriples))
	})

	t.Run("ReadAndApplyDiff", func(t *testing.T) {
		ng := readThenApplyDiff(t, version1FilePath)

		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("uuid1"), "Expected IRINode 'uuid1' not found")

		// Check that the uuid1 node contains a blank node of type qau:Length
		uuid1 := ng.GetIRINodeByFragment("uuid1")
		assert.NotNil(t, uuid1, "Expected IRINode 'uuid1' not found")
		objects := uuid1.GetObjects(sso.OccurrenceQuantity)
		found := false
		for _, obj := range objects {
			if obj.(sst.IBNode).IsBlankNode() {
				blankObjects := obj.(sst.IBNode).GetObjects(qau.Metre)
				assert.Equal(t, sst.Double(7.00), blankObjects[0])
				found = true
				break
			}
		}
		assert.True(t, found, "Expected blank node of type lci.Person contained in 'John' not found")
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(version1FilePath + ".sst")
		os.Remove(version2FilePath + ".sst")
		os.Remove(version1FilePath + "diff.sst")
	})
}

func TestWriteDiffRdfReadV1V3(t *testing.T) {
	testName := filepath.Join("./testdata/", "TestWriteDiffRdfRead")

	version1FilePath := testName + "version1"
	version2FilePath := testName + "version3"
	// sst.AtomicLevel.SetLevel(zap.DebugLevel)
	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(version1FilePath + ".sst")
		os.Remove(version2FilePath + ".sst")
		os.Remove(version1FilePath + "diff.sst")
	})

	t.Run("read TTLs", func(t *testing.T) {
		rdfReadThenSstWrite(t, version1FilePath)
		rdfReadThenSstWrite(t, version2FilePath)
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		diffTriples := readSSTFilesThenWriteDiff(t, version1FilePath, version2FilePath)
		// assert.Equal(t, sst.DiffTripleFlag(0), diffTriples[0].Flag)
		// assert.Equal(t, ":uuid1", diffTriples[0].Sub)
		// assert.Equal(t, "sso:occurrenceQuantity", diffTriples[0].Pred)
		// assert.Equal(t, "_:b0", diffTriples[0].Obj)

		// assert.Equal(t, sst.DiffTripleFlag(-1), diffTriples[1].Flag)
		// assert.Equal(t, "_:b0", diffTriples[1].Sub)
		// assert.Equal(t, "qau:metre", diffTriples[1].Pred)
		// assert.Equal(t, "5.00", diffTriples[1].Obj)

		// assert.Equal(t, sst.DiffTripleFlag(1), diffTriples[2].Flag)
		// assert.Equal(t, "_:b0", diffTriples[2].Sub)
		// assert.Equal(t, "qau:metre", diffTriples[2].Pred)
		// assert.Equal(t, "7.00", diffTriples[2].Obj)

		assert.Equal(t, 4, len(diffTriples))
	})

	t.Run("ReadAndApplyDiff", func(t *testing.T) {
		// t.Skip("still panic now")
		sst.AtomicLevel.SetLevel(zap.DebugLevel)
		ng := readThenApplyDiff(t, version1FilePath)
		// ng.PrintTriples()

		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("uuid1"), "Expected IRINode 'uuid1' not found")

		// Check that the uuid1 node contains a blank node of type lci.Thing
		uuid1 := ng.GetIRINodeByFragment("uuid1")
		assert.NotNil(t, uuid1, "Expected IRINode 'uuid1' not found")
		objects := uuid1.GetObjects(sso.OccurrenceQuantity)
		found := false
		for _, obj := range objects {
			if obj.(sst.IBNode).IsBlankNode() {
				blankObjects := obj.(sst.IBNode).GetObjects(qau.Metre)
				assert.Equal(t, sst.Double(5.00), blankObjects[0])
				found = true
				assert.Equal(t, qau.Length.IRI(), obj.(sst.IBNode).GetObjects(rdf.Type)[0].(sst.IBNode).IRI())

				break
			}
		}
		assert.True(t, found, "Expected blank node of type lci.Person contained in 'John' not found")
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(version1FilePath + ".sst")
		os.Remove(version2FilePath + ".sst")
		os.Remove(version1FilePath + "diff.sst")
	})
}

func TestWriteDiffRdfReadV1V4(t *testing.T) {
	testName := filepath.Join("./testdata/", "TestWriteDiffRdfRead")

	version1FilePath := testName + "version1"
	version2FilePath := testName + "version4"

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(version1FilePath + ".sst")
		os.Remove(version2FilePath + ".sst")
		os.Remove(version1FilePath + "diff.sst")
	})

	t.Run("read TTLs", func(t *testing.T) {
		rdfReadThenSstWrite(t, version1FilePath)
		rdfReadThenSstWrite(t, version2FilePath)
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		diffTriples := readSSTFilesThenWriteDiff(t, version1FilePath, version2FilePath)
		// assert.Equal(t, sst.DiffTripleFlag(0), diffTriples[0].Flag)
		// assert.Equal(t, ":uuid1", diffTriples[0].Sub)
		// assert.Equal(t, "sso:occurrenceQuantity", diffTriples[0].Pred)
		// assert.Equal(t, "_:b0", diffTriples[0].Obj)

		// assert.Equal(t, sst.DiffTripleFlag(-1), diffTriples[1].Flag)
		// assert.Equal(t, "_:b0", diffTriples[1].Sub)
		// assert.Equal(t, "qau:metre", diffTriples[1].Pred)
		// assert.Equal(t, "5.00", diffTriples[1].Obj)

		// assert.Equal(t, sst.DiffTripleFlag(1), diffTriples[2].Flag)
		// assert.Equal(t, "_:b0", diffTriples[2].Sub)
		// assert.Equal(t, "qau:metre", diffTriples[2].Pred)
		// assert.Equal(t, "7.00", diffTriples[2].Obj)

		assert.Equal(t, 7, len(diffTriples))
	})

	t.Run("ReadAndApplyDiff", func(t *testing.T) {
		ng := readThenApplyDiff(t, version1FilePath)

		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("uuid1"), "Expected IRINode 'uuid1' not found")

		// Check that the uuid1 node contains a blank node of type lci.Thing
		uuid1 := ng.GetIRINodeByFragment("uuid1")
		assert.NotNil(t, uuid1, "Expected IRINode 'uuid1' not found")
		objects := uuid1.GetObjects(sso.OccurrenceQuantity)
		found := false
		for _, obj := range objects {
			if obj.(sst.IBNode).IsBlankNode() {
				blankObjects := obj.(sst.IBNode).GetObjects(qau.Metre)
				assert.Equal(t, sst.Double(7.00), blankObjects[0])
				found = true
				break
			}
		}
		assert.True(t, found, "Expected blank node of type lci.Person contained in 'John' not found")
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(version1FilePath + ".sst")
		os.Remove(version2FilePath + ".sst")
		os.Remove(version1FilePath + "diff.sst")
	})
}
