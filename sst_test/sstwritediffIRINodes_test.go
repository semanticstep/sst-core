// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// Helper function to create and write NamedGraph states
func createAndWriteNamedGraph(testName string, setupFunc func(stage sst.Stage, ngA sst.NamedGraph)) {
	stage := sst.OpenStage(sst.DefaultTriplexMode)
	ngIDA := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361")
	ngA := stage.CreateNamedGraph(sst.IRI(ngIDA.URN()))
	setupFunc(stage, ngA)
	writeToFile(ngA, testName)
}

// Helper function to read and write diffs
func readAndWriteDiff(t *testing.T, testName string) []sst.DiffTriple {
	bufioFrom, err := os.Open(testName + "NGA_before.sst")
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}
	defer bufioFrom.Close()

	bufioTo, err := os.Open(testName + "NGA_after.sst")
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}
	defer bufioTo.Close()

	out, err := os.Create(testName + "diff" + ".sst")
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

// Helper function to read and apply diffs
func readAndApplyDiff(t *testing.T, testName string) (ng sst.NamedGraph) {
	fileFrom, err := os.Open(testName + "NGA_before.sst")
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}
	defer fileFrom.Close()

	fileDiff, err := os.Open(testName + "diff" + ".sst")
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

// example 1: add a new IRINode with its one triple
func TestWriteDiffAddANewIRINode(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())
	// sst.AtomicLevel.SetLevel(zapcore.DebugLevel)
	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			john := ngA.CreateIRINode("John")
			john.AddStatement(rdf.Type, lci.Person)
			ngA.CreateIRINode("Linda", lci.Person)
			// john.AddStatement(lci.Contains, linda)
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			ngA.CreateIRINode("John", lci.Person)
			ngA.CreateIRINode("Linda", lci.Person)
			ngA.CreateIRINode("Lisa", lci.Person)
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		diffTriples := readAndWriteDiff(t, testName)
		assert.Equal(t, sst.DiffTripleFlag(1), diffTriples[0].Flag)
		assert.Equal(t, ":Lisa", diffTriples[0].Sub)
		assert.Equal(t, "rdf:type", diffTriples[0].Pred)
		assert.Equal(t, "lci:Person", diffTriples[0].Obj)
		assert.Equal(t, 1, len(diffTriples))
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)

		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("John"), "Expected IRINode 'John' not found")
		assert.NotNil(t, ng.GetIRINodeByFragment("Linda"), "Expected IRINode 'Linda' not found")
		assert.NotNil(t, ng.GetIRINodeByFragment("Lisa"), "Unexpected IRINode 'Lisa' not found")
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")

	})
}

// example 2 about remove an IRINode with its one triple
func TestWriteDiffDeleteAnIRINode(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())

	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			ngA.CreateIRINode("John", lci.Person)
			ngA.CreateIRINode("Linda", lci.Person)
			ngA.CreateIRINode("Lisa", lci.Person)
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			ngA.CreateIRINode("John", lci.Person)
			ngA.CreateIRINode("Linda", lci.Person)
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		diffTriples := readAndWriteDiff(t, testName)
		assert.Equal(t, sst.DiffTripleFlag(-1), diffTriples[0].Flag)
		assert.Equal(t, ":Lisa", diffTriples[0].Sub)
		assert.Equal(t, "rdf:type", diffTriples[0].Pred)
		assert.Equal(t, "lci:Person", diffTriples[0].Obj)
		assert.Equal(t, 1, len(diffTriples))
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)
		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("John"), "Expected IRINode 'John' not found")
		assert.NotNil(t, ng.GetIRINodeByFragment("Linda"), "Expected IRINode 'Linda' not found")
		assert.Nil(t, ng.GetIRINodeByFragment("Lisa"), "Unexpected IRINode 'Lisa' found")
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")

	})
}

// example 3: add a new triple on existing IRINode
func TestWriteDiffAddNewTriple(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())

	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			ngA.CreateIRINode("John", lci.Person)
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			mainA := ngA.CreateIRINode("John", lci.Person)
			mainA.AddStatement(rep.MappingSource, lci.Organization)
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		diffTriples := readAndWriteDiff(t, testName)
		assert.Equal(t, sst.DiffTripleFlag(1), diffTriples[0].Flag)
		assert.Equal(t, ":John", diffTriples[0].Sub)
		assert.Equal(t, "rep:mappingSource", diffTriples[0].Pred)
		assert.Equal(t, "lci:Organization", diffTriples[0].Obj)
		assert.Equal(t, 1, len(diffTriples))
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)

		// Assert that the NamedGraph contains the expected triples using assert package
		mainA := ng.GetIRINodeByFragment("John")
		assert.NotNil(t, mainA, "Expected IRINode 'John' not found")

		// Check for the triple (John, rdf:Type, lci:Individual)
		objects := mainA.GetObjects(rdf.Type)
		found := false
		for _, obj := range objects {
			if obj.(sst.IBNode).Is(lci.Person) {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected triple (John, rdf:Type, lci:Individual) not found")

		// Check for the triple (John, rep:MappingSource, lci.Organization)
		objects = mainA.GetObjects(rep.MappingSource)
		found = false
		for _, obj := range objects {
			if obj.(sst.IBNode).Is(lci.Organization) {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected triple (John, rep:MappingSource, lci:Organization) not found")
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})
}

// example 4: remove a triple from an existing IRINode
func TestWriteDiffDeleteOneTriple(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())

	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			mainA := ngA.CreateIRINode("John", lci.Person)
			mainA.AddStatement(rdf.Type, lci.Individual)
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			ngA.CreateIRINode("John", lci.Person)
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		diffTriples := readAndWriteDiff(t, testName)
		assert.Equal(t, sst.DiffTripleFlag(-1), diffTriples[0].Flag)
		assert.Equal(t, ":John", diffTriples[0].Sub)
		assert.Equal(t, "rdf:type", diffTriples[0].Pred)
		assert.Equal(t, "lci:Individual", diffTriples[0].Obj)
		assert.Equal(t, 1, len(diffTriples))
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)

		// Assert that the NamedGraph contains the expected triples using assert package
		mainA := ng.GetIRINodeByFragment("John")
		assert.NotNil(t, mainA, "Expected IRINode 'John' not found")

		// Check that the triple (John, rdf:Type, lci:Individual) is deleted
		objects := mainA.GetObjects(rdf.Type)
		found := false
		for _, obj := range objects {
			if obj.(sst.IBNode).Is(lci.Individual) {
				found = true
				break
			}
		}
		assert.False(t, found, "Unexpected triple (John, rdf:Type, lci:Individual) found")
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")

	})
}

// example 5: add several IRINodes (each IRINode has one triple)
func TestWriteDiffAddMultipleIRINodes(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())

	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			ngA.CreateIRINode("John", lci.Person)
			ngA.CreateIRINode("Linda", lci.Person)
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			ngA.CreateIRINode("John", lci.Person)
			ngA.CreateIRINode("Linda", lci.Person)
			ngA.CreateIRINode("Lisa", lci.Person)
			ngA.CreateIRINode("Mark", lci.Person)
			ngA.CreateIRINode("Sophia", lci.Person)
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		diffTriples := readAndWriteDiff(t, testName)
		assert.Equal(t, sst.DiffTripleFlag(1), diffTriples[0].Flag)
		assert.Equal(t, ":Lisa", diffTriples[0].Sub)
		assert.Equal(t, "rdf:type", diffTriples[0].Pred)
		assert.Equal(t, "lci:Person", diffTriples[0].Obj)
		assert.Equal(t, sst.DiffTripleFlag(1), diffTriples[1].Flag)
		assert.Equal(t, ":Mark", diffTriples[1].Sub)
		assert.Equal(t, "rdf:type", diffTriples[1].Pred)
		assert.Equal(t, "lci:Person", diffTriples[1].Obj)
		assert.Equal(t, sst.DiffTripleFlag(1), diffTriples[2].Flag)
		assert.Equal(t, ":Sophia", diffTriples[2].Sub)
		assert.Equal(t, "rdf:type", diffTriples[2].Pred)
		assert.Equal(t, "lci:Person", diffTriples[2].Obj)
		assert.Equal(t, 3, len(diffTriples))
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)

		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("John"), "Expected IRINode 'John' not found")
		assert.NotNil(t, ng.GetIRINodeByFragment("Linda"), "Expected IRINode 'Linda' not found")
		assert.NotNil(t, ng.GetIRINodeByFragment("Lisa"), "Expected IRINode 'Lisa' not found")
		assert.NotNil(t, ng.GetIRINodeByFragment("Mark"), "Expected IRINode 'Mark' not found")
		assert.NotNil(t, ng.GetIRINodeByFragment("Sophia"), "Expected IRINode 'Sophia' not found")
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")

	})
}

// example 6: remove several IRINodes (each IRINode has one triple)
func TestWriteDiffRemoveMultipleIRINodes(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())

	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			ngA.CreateIRINode("John", lci.Person)
			ngA.CreateIRINode("Linda", lci.Person)
			ngA.CreateIRINode("Lisa", lci.Person)
			ngA.CreateIRINode("Mark", lci.Person)
			ngA.CreateIRINode("Sophia", lci.Person)
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			ngA.CreateIRINode("John", lci.Person)
			ngA.CreateIRINode("Linda", lci.Person)
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		diffTriples := readAndWriteDiff(t, testName)
		assert.Equal(t, sst.DiffTripleFlag(-1), diffTriples[0].Flag)
		assert.Equal(t, ":Lisa", diffTriples[0].Sub)
		assert.Equal(t, "rdf:type", diffTriples[0].Pred)
		assert.Equal(t, "lci:Person", diffTriples[0].Obj)
		assert.Equal(t, sst.DiffTripleFlag(-1), diffTriples[1].Flag)
		assert.Equal(t, ":Mark", diffTriples[1].Sub)
		assert.Equal(t, "rdf:type", diffTriples[1].Pred)
		assert.Equal(t, "lci:Person", diffTriples[1].Obj)
		assert.Equal(t, sst.DiffTripleFlag(-1), diffTriples[2].Flag)
		assert.Equal(t, ":Sophia", diffTriples[2].Sub)
		assert.Equal(t, "rdf:type", diffTriples[2].Pred)
		assert.Equal(t, "lci:Person", diffTriples[2].Obj)
		assert.Equal(t, 3, len(diffTriples))
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)

		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("John"), "Expected IRINode 'John' not found")
		assert.NotNil(t, ng.GetIRINodeByFragment("Linda"), "Expected IRINode 'Linda' not found")
		assert.Nil(t, ng.GetIRINodeByFragment("Lisa"), "Unexpected IRINode 'Lisa' found")
		assert.Nil(t, ng.GetIRINodeByFragment("Mark"), "Unexpected IRINode 'Mark' found")
		assert.Nil(t, ng.GetIRINodeByFragment("Sophia"), "Unexpected IRINode 'Sophia' found")
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")

	})
}

// example 7: add multiple triples to an existing IRINode
func TestWriteDiffAddMultipleTriples(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())

	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			mainA := ngA.CreateIRINode("John", lci.Person)
			mainA.AddStatement(rdf.Type, lci.Individual)
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			mainA := ngA.CreateIRINode("John", lci.Person)
			mainA.AddStatement(rdf.Type, lci.Individual)
			mainA.AddStatement(rep.MappingSource, lci.Organization)
			mainA.AddStatement(rep.MappingTarget, lci.Person)
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		diffTriples := readAndWriteDiff(t, testName)
		assert.Equal(t, sst.DiffTripleFlag(1), diffTriples[0].Flag)
		assert.Equal(t, ":John", diffTriples[0].Sub)
		assert.Equal(t, "rep:mappingSource", diffTriples[0].Pred)
		assert.Equal(t, "lci:Organization", diffTriples[0].Obj)

		assert.Equal(t, sst.DiffTripleFlag(1), diffTriples[1].Flag)
		assert.Equal(t, "", diffTriples[1].Sub)
		assert.Equal(t, "rep:mappingTarget", diffTriples[1].Pred)
		assert.Equal(t, "lci:Person", diffTriples[1].Obj)
		assert.Equal(t, 2, len(diffTriples))
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)

		// Assert that the NamedGraph contains the expected triples using assert package
		mainA := ng.GetIRINodeByFragment("John")
		assert.NotNil(t, mainA, "Expected IRINode 'John' not found")

		// Check for the triple (John, rdf:Type, lci:Individual)
		objects := mainA.GetObjects(rdf.Type)
		found := false
		for _, obj := range objects {
			if obj.(sst.IBNode).Is(lci.Individual) {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected triple (John, rdf:Type, lci:Individual) not found")

		// Check for the triple (John, rep:MappingSource, lci.Organization)
		objects = mainA.GetObjects(rep.MappingSource)
		found = false
		for _, obj := range objects {
			if obj.(sst.IBNode).Is(lci.Organization) {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected triple (John, rep:MappingSource, lci:Organization) not found")

		// Check for the triple (John, rep.MappingTarget, lci.Person)
		objects = mainA.GetObjects(rep.MappingTarget)
		found = false
		for _, obj := range objects {
			if obj.(sst.IBNode).Is(lci.Person) {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected triple (John, rep.MappingTarget, lci.Person) not found")
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")

	})
}

// example 8: remove multiple triples from an existing IRINode
func TestWriteDiffRemoveMultipleTriples(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())

	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			mainA := ngA.CreateIRINode("John", lci.Person)
			mainA.AddStatement(rdf.Type, lci.Individual)
			mainA.AddStatement(rep.MappingSource, lci.Organization)
			mainA.AddStatement(rep.MappingTarget, lci.Person)
			mainA.AddStatement(rep.SchematicPort, lci.Individual)
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			mainA := ngA.CreateIRINode("John", lci.Person)
			mainA.AddStatement(rdf.Type, lci.Individual)
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		diffTriples := readAndWriteDiff(t, testName)
		assert.Equal(t, sst.DiffTripleFlag(-1), diffTriples[0].Flag)
		assert.Equal(t, ":John", diffTriples[0].Sub)
		assert.Equal(t, "rep:SchematicPort", diffTriples[0].Pred)
		assert.Equal(t, "lci:Individual", diffTriples[0].Obj)

		assert.Equal(t, sst.DiffTripleFlag(-1), diffTriples[1].Flag)
		assert.Equal(t, "", diffTriples[1].Sub)
		assert.Equal(t, "rep:mappingSource", diffTriples[1].Pred)
		assert.Equal(t, "lci:Organization", diffTriples[1].Obj)

		assert.Equal(t, sst.DiffTripleFlag(-1), diffTriples[2].Flag)
		assert.Equal(t, "", diffTriples[2].Sub)
		assert.Equal(t, "rep:mappingTarget", diffTriples[2].Pred)
		assert.Equal(t, "lci:Person", diffTriples[2].Obj)

		assert.Equal(t, 3, len(diffTriples))
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)

		// Assert that the NamedGraph contains the expected triples using assert package
		mainA := ng.GetIRINodeByFragment("John")
		assert.NotNil(t, mainA, "Expected IRINode 'John' not found")

		// Check for the triple (John, rdf:Type, lci.Individual)
		objects := mainA.GetObjects(rdf.Type)
		found := false
		for _, obj := range objects {
			if obj.(sst.IBNode).Is(lci.Individual) {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected triple (John, rdf:Type, lci:Individual) not found")

		// Check that the triple (John, rep:MappingSource, lci.Organization) is removed
		objects = mainA.GetObjects(rep.MappingSource)
		found = false
		for _, obj := range objects {
			if obj.(sst.IBNode).Is(lci.Organization) {
				found = true
				break
			}
		}
		assert.False(t, found, "Unexpected triple (John, rep:MappingSource, lci:Organization) found")

		// Check that the triple (John, rep.MappingTarget, lci.Person) is removed
		objects = mainA.GetObjects(rep.MappingTarget)
		found = false
		for _, obj := range objects {
			if obj.(sst.IBNode).Is(lci.Person) {
				found = true
				break
			}
		}
		assert.False(t, found, "Unexpected triple (John, rep.MappingTarget, lci:Person) found")

		// Check that the triple (John, rep.SchematicPort, lci.Individual) is removed
		objects = mainA.GetObjects(rep.SchematicPort)
		found = false
		for _, obj := range objects {
			if obj.(sst.IBNode).Is(lci.Individual) {
				found = true
				break
			}
		}
		assert.False(t, found, "Unexpected triple (John, rep.SchematicPort, lci:Individual) found")
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")

	})
}

// example 9: add several IRINodes with each IRINode has several triple
func TestWriteDiffAddMultipleIRINodesWithTriples(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())

	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			ngA.CreateIRINode("John", lci.Person)
			ngA.CreateIRINode("Linda", lci.Person)
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			john := ngA.CreateIRINode("John", lci.Person)
			john.AddStatement(rdf.Type, lci.Individual)
			john.AddStatement(rep.MappingSource, lci.Organization)

			linda := ngA.CreateIRINode("Linda", lci.Person)
			linda.AddStatement(rdf.Type, lci.Individual)
			linda.AddStatement(rep.MappingSource, lci.Organization)

			mark := ngA.CreateIRINode("Mark", lci.Person)
			mark.AddStatement(rdf.Type, lci.Individual)
			mark.AddStatement(rep.MappingSource, lci.Organization)
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		diffTriples := readAndWriteDiff(t, testName)
		assert.Equal(t, sst.DiffTripleFlag(1), diffTriples[0].Flag)
		assert.Equal(t, ":John", diffTriples[0].Sub)
		assert.Equal(t, "rdf:type", diffTriples[0].Pred)
		assert.Equal(t, "lci:Individual", diffTriples[0].Obj)

		assert.Equal(t, sst.DiffTripleFlag(1), diffTriples[1].Flag)
		assert.Equal(t, "", diffTriples[1].Sub)
		assert.Equal(t, "rep:mappingSource", diffTriples[1].Pred)
		assert.Equal(t, "lci:Organization", diffTriples[1].Obj)

		assert.Equal(t, sst.DiffTripleFlag(1), diffTriples[2].Flag)
		assert.Equal(t, ":Linda", diffTriples[2].Sub)
		assert.Equal(t, "rdf:type", diffTriples[2].Pred)
		assert.Equal(t, "lci:Individual", diffTriples[2].Obj)

		assert.Equal(t, sst.DiffTripleFlag(1), diffTriples[3].Flag)
		assert.Equal(t, "", diffTriples[3].Sub)
		assert.Equal(t, "rep:mappingSource", diffTriples[3].Pred)
		assert.Equal(t, "lci:Organization", diffTriples[3].Obj)

		assert.Equal(t, sst.DiffTripleFlag(1), diffTriples[4].Flag)
		assert.Equal(t, ":Mark", diffTriples[4].Sub)
		assert.Equal(t, "rdf:type", diffTriples[4].Pred)
		assert.Equal(t, "lci:Individual", diffTriples[4].Obj)

		assert.Equal(t, sst.DiffTripleFlag(1), diffTriples[5].Flag)
		assert.Equal(t, "", diffTriples[5].Sub)
		assert.Equal(t, "", diffTriples[5].Pred)
		assert.Equal(t, "lci:Person", diffTriples[5].Obj)

		assert.Equal(t, sst.DiffTripleFlag(1), diffTriples[6].Flag)
		assert.Equal(t, "", diffTriples[6].Sub)
		assert.Equal(t, "rep:mappingSource", diffTriples[6].Pred)
		assert.Equal(t, "lci:Organization", diffTriples[6].Obj)

		assert.Equal(t, 7, len(diffTriples))
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)

		// Assert that the NamedGraph contains the expected nodes and triples using assert package
		john := ng.GetIRINodeByFragment("John")
		assert.NotNil(t, john, "Expected IRINode 'John' not found")
		assert.True(t, hasTriple(john, rdf.Type, lci.Individual), "Expected triple (John, rdf:Type, lci:Individual) not found")
		assert.True(t, hasTriple(john, rep.MappingSource, lci.Organization), "Expected triple (John, rep:MappingSource, lci:Organization) not found")

		linda := ng.GetIRINodeByFragment("Linda")
		assert.NotNil(t, linda, "Expected IRINode 'Linda' not found")
		assert.True(t, hasTriple(linda, rdf.Type, lci.Individual), "Expected triple (Linda, rdf:Type, lci:Individual) not found")
		assert.True(t, hasTriple(linda, rep.MappingSource, lci.Organization), "Expected triple (Linda, rep:MappingSource, lci:Organization) not found")

		mark := ng.GetIRINodeByFragment("Mark")
		assert.NotNil(t, mark, "Expected IRINode 'Mark' not found")
		assert.True(t, hasTriple(mark, rdf.Type, lci.Individual), "Expected triple (Mark, rdf:Type, lci:Individual) not found")
		assert.True(t, hasTriple(mark, rep.MappingSource, lci.Organization), "Expected triple (Mark, rep:MappingSource, lci:Organization) not found")
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")

	})
}

func hasTriple(node sst.IBNode, predicate sst.Node, object sst.ElementInformer) bool {
	objects := node.GetObjects(predicate)
	for _, obj := range objects {
		if obj.(sst.IBNode).Is(object) {
			return true
		}
	}
	return false
}

// example 10: remove several IRINodes with each IRINode has several triple
func TestWriteDiffRemoveMultipleIRINodesWithTriples(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())

	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			john := ngA.CreateIRINode("John", lci.Person)
			john.AddStatement(rdf.Type, lci.Individual)
			john.AddStatement(rep.MappingSource, lci.Organization)

			linda := ngA.CreateIRINode("Linda", lci.Person)
			linda.AddStatement(rdf.Type, lci.Individual)
			linda.AddStatement(rep.MappingSource, lci.Organization)

			mark := ngA.CreateIRINode("Mark", lci.Person)
			mark.AddStatement(rdf.Type, lci.Individual)
			mark.AddStatement(rep.MappingSource, lci.Organization)
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			ngA.CreateIRINode("John", lci.Person)
			ngA.CreateIRINode("Linda", lci.Person)
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		diffTriples := readAndWriteDiff(t, testName)

		assert.Equal(t, sst.DiffTripleFlag(-1), diffTriples[0].Flag)
		assert.Equal(t, ":John", diffTriples[0].Sub)
		assert.Equal(t, "rdf:type", diffTriples[0].Pred)
		assert.Equal(t, "lci:Individual", diffTriples[0].Obj)

		assert.Equal(t, sst.DiffTripleFlag(-1), diffTriples[1].Flag)
		assert.Equal(t, "", diffTriples[1].Sub)
		assert.Equal(t, "rep:mappingSource", diffTriples[1].Pred)
		assert.Equal(t, "lci:Organization", diffTriples[1].Obj)

		assert.Equal(t, sst.DiffTripleFlag(-1), diffTriples[2].Flag)
		assert.Equal(t, ":Linda", diffTriples[2].Sub)
		assert.Equal(t, "rdf:type", diffTriples[2].Pred)
		assert.Equal(t, "lci:Individual", diffTriples[2].Obj)

		assert.Equal(t, sst.DiffTripleFlag(-1), diffTriples[3].Flag)
		assert.Equal(t, "", diffTriples[3].Sub)
		assert.Equal(t, "rep:mappingSource", diffTriples[3].Pred)
		assert.Equal(t, "lci:Organization", diffTriples[3].Obj)

		assert.Equal(t, sst.DiffTripleFlag(-1), diffTriples[4].Flag)
		assert.Equal(t, ":Mark", diffTriples[4].Sub)
		assert.Equal(t, "rdf:type", diffTriples[4].Pred)
		assert.Equal(t, "lci:Individual", diffTriples[4].Obj)

		assert.Equal(t, sst.DiffTripleFlag(-1), diffTriples[5].Flag)
		assert.Equal(t, "", diffTriples[5].Sub)
		assert.Equal(t, "", diffTriples[5].Pred)
		assert.Equal(t, "lci:Person", diffTriples[5].Obj)

		assert.Equal(t, sst.DiffTripleFlag(-1), diffTriples[6].Flag)
		assert.Equal(t, "", diffTriples[6].Sub)
		assert.Equal(t, "rep:mappingSource", diffTriples[6].Pred)
		assert.Equal(t, "lci:Organization", diffTriples[6].Obj)

		assert.Equal(t, 7, len(diffTriples))
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)

		// Assert that the NamedGraph contains the expected nodes and triples using assert package
		john := ng.GetIRINodeByFragment("John")
		assert.NotNil(t, john, "Expected IRINode 'John' not found")
		assert.False(t, hasTriple(john, rdf.Type, lci.Individual), "Expected triple (John, rdf:Type, lci:Individual) not found")
		assert.False(t, hasTriple(john, rep.MappingSource, lci.Organization), "Expected triple (John, rep:MappingSource, lci:Organization) not found")

		linda := ng.GetIRINodeByFragment("Linda")
		assert.NotNil(t, linda, "Expected IRINode 'Linda' found")

		mark := ng.GetIRINodeByFragment("Mark")
		assert.Nil(t, mark, "Unexpected IRINode 'Mark' found")
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")

	})
}

// example 11 remove an IRINode with its one triple
// and add an IRINode with its one triple
func TestWriteDiffDeleteThenAddAnIRINode(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())

	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			ngA.CreateIRINode("John", lci.Person)
			ngA.CreateIRINode("Linda", lci.Person)
			lisa := ngA.CreateIRINode("Lisa", lci.Person)
			lisa.AddStatement(rdfs.Label, lci.Person)
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			ngA.CreateIRINode("John", lci.Person)
			ngA.CreateIRINode("Linda", lci.Person)
			ngA.CreateIRINode("Adam", lci.Person)
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		diffTriples := readAndWriteDiff(t, testName)
		assert.Equal(t, sst.DiffTripleFlag(1), diffTriples[0].Flag)
		assert.Equal(t, ":Adam", diffTriples[0].Sub)
		assert.Equal(t, "rdf:type", diffTriples[0].Pred)
		assert.Equal(t, "lci:Person", diffTriples[0].Obj)

		assert.Equal(t, sst.DiffTripleFlag(-1), diffTriples[1].Flag)
		assert.Equal(t, ":Lisa", diffTriples[1].Sub)
		assert.Equal(t, "rdf:type", diffTriples[1].Pred)
		assert.Equal(t, "lci:Person", diffTriples[1].Obj)

		assert.Equal(t, sst.DiffTripleFlag(-1), diffTriples[2].Flag)
		assert.Equal(t, "", diffTriples[2].Sub)
		assert.Equal(t, "rdfs:label", diffTriples[2].Pred)
		assert.Equal(t, "lci:Person", diffTriples[2].Obj)
		assert.Equal(t, 3, len(diffTriples))
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)
		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("John"), "Expected IRINode 'John' not found")
		assert.NotNil(t, ng.GetIRINodeByFragment("Linda"), "Expected IRINode 'Linda' not found")
		assert.Nil(t, ng.GetIRINodeByFragment("Lisa"), "Unexpected IRINode 'Lisa' found")
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})
}

// example 12 remove a triple and then add a triple on the same IBNode
func TestWriteDiffDeleteThenAddATriple(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())

	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			lisa := ngA.CreateIRINode("Lisa", lci.Person)
			lisa.AddStatement(rdfs.Label, lci.Person)
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			lisa := ngA.CreateIRINode("Lisa", lci.Person)
			lisa.AddStatement(rdfs.Label, lci.Activity)
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		diffTriples := readAndWriteDiff(t, testName)
		assert.Equal(t, sst.DiffTripleFlag(-1), diffTriples[0].Flag)
		assert.Equal(t, ":Lisa", diffTriples[0].Sub)
		assert.Equal(t, "rdfs:label", diffTriples[0].Pred)
		assert.Equal(t, "lci:Person", diffTriples[0].Obj)

		assert.Equal(t, sst.DiffTripleFlag(1), diffTriples[1].Flag)
		assert.Equal(t, "", diffTriples[1].Sub)
		assert.Equal(t, "", diffTriples[1].Pred)
		assert.Equal(t, "lci:Activity", diffTriples[1].Obj)
		assert.Equal(t, 2, len(diffTriples))
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)
		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("Lisa"), "Expected IRINode 'Lisa' found")
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})
}

// example 13 remove triples and then add triples on the same IBNode
func TestWriteDiffDeleteTriplesThenAddTriples(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())

	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			lisa := ngA.CreateIRINode("Lisa", lci.Person)
			lisa.AddStatement(rdfs.Label, lci.Person)
			lisa.AddStatement(rdfs.Comment, lci.Activity)
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			lisa := ngA.CreateIRINode("Lisa", lci.Person)
			lisa.AddStatement(rdfs.Label, lci.Activity)
			lisa.AddStatement(rdfs.Comment, lci.ActualIndividual)
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		diffTriples := readAndWriteDiff(t, testName)
		assert.Equal(t, sst.DiffTripleFlag(-1), diffTriples[0].Flag)
		assert.Equal(t, ":Lisa", diffTriples[0].Sub)
		assert.Equal(t, "rdfs:comment", diffTriples[0].Pred)
		assert.Equal(t, "lci:Activity", diffTriples[0].Obj)

		assert.Equal(t, sst.DiffTripleFlag(1), diffTriples[1].Flag)
		assert.Equal(t, "", diffTriples[1].Sub)
		assert.Equal(t, "", diffTriples[1].Pred)
		assert.Equal(t, "lci:ActualIndividual", diffTriples[1].Obj)

		assert.Equal(t, sst.DiffTripleFlag(-1), diffTriples[2].Flag)
		assert.Equal(t, "", diffTriples[2].Sub)
		assert.Equal(t, "rdfs:label", diffTriples[2].Pred)
		assert.Equal(t, "lci:Person", diffTriples[2].Obj)

		assert.Equal(t, sst.DiffTripleFlag(1), diffTriples[3].Flag)
		assert.Equal(t, "", diffTriples[3].Sub)
		assert.Equal(t, "", diffTriples[3].Pred)
		assert.Equal(t, "lci:Activity", diffTriples[3].Obj)
		assert.Equal(t, 4, len(diffTriples))
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)
		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("Lisa"), "Expected IRINode 'Lisa' found")
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})
}

// example 14 modify triples order on the same IBNode
func TestWriteDiffModifyTriplesOrder(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())

	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			lisa := ngA.CreateIRINode("Lisa", sst.String("Person"))
			lisa.AddStatement(rdfs.Comment, lci.ActualIndividual)
			lisa.AddStatement(rdfs.Label, lci.Activity)
			lisa.AddStatement(rdfs.Label, lci.ChemicalSubstance)
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			lisa := ngA.CreateIRINode("Lisa", lci.Person)
			lisa.AddStatement(rdfs.Label, lci.ChemicalSubstance)
			lisa.AddStatement(rdfs.Label, lci.Activity)
			lisa.AddStatement(rdfs.Comment, lci.ActualIndividual)
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		diffTriples := readAndWriteDiff(t, testName)
		_ = diffTriples
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)
		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("Lisa"), "Expected IRINode 'Lisa' found")
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})
}
