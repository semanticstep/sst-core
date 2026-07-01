// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/qau"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/stretchr/testify/assert"
)

// example 0: modify a triple of blankNode
func TestWriteDiffModifyATripleOfBlankNode(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})

	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			john := ngA.CreateIRINode("John", lci.Person)
			blank := ngA.CreateBlankNode(lci.Thing)
			john.AddStatement(lci.Contains, blank)
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			john := ngA.CreateIRINode("John", lci.Person)
			blank := ngA.CreateBlankNode(lci.Individual)
			john.AddStatement(lci.Contains, blank)
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		diffTriples := readAndWriteDiff(t, testName)
		assert.Equal(t, sst.DiffTripleFlag(0), diffTriples[0].Flag)
		assert.Equal(t, ":John", diffTriples[0].Sub)
		assert.Equal(t, "lci:contains", diffTriples[0].Pred)
		assert.Equal(t, "_:b0", diffTriples[0].Obj)

		assert.Equal(t, sst.DiffTripleFlag(-1), diffTriples[1].Flag)
		assert.Equal(t, "_:b0", diffTriples[1].Sub)
		assert.Equal(t, "rdf:type", diffTriples[1].Pred)
		assert.Equal(t, "lci:Thing", diffTriples[1].Obj)

		assert.Equal(t, sst.DiffTripleFlag(1), diffTriples[2].Flag)
		assert.Equal(t, "", diffTriples[2].Sub)
		assert.Equal(t, "", diffTriples[2].Pred)
		assert.Equal(t, "lci:Individual", diffTriples[2].Obj)

		assert.Equal(t, 3, len(diffTriples))
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)

		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("John"), "Expected IRINode 'John' not found")

		// Check that the John node contains a blank node of type lci.Thing
		john := ng.GetIRINodeByFragment("John")
		assert.NotNil(t, john, "Expected IRINode 'John' not found")
		objects := john.GetObjects(lci.Contains)
		found := false
		for _, obj := range objects {
			if obj.(sst.IBNode).IsBlankNode() {
				blankObjects := obj.(sst.IBNode).GetObjects(rdf.Type)
				assert.True(t, blankObjects[0].(sst.IBNode).Is(lci.Individual), "Expected blank node object lci:Thing not found")
				found = true
				break
			}
		}
		assert.True(t, found, "Expected blank node of type lci.Person contained in 'John' not found")
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})
}

// example 01: add several blankNodes
func TestWriteDiffAddSeveralBlankNodes(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})

	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			john := ngA.CreateIRINode("John", lci.Person)
			blank := ngA.CreateBlankNode(lci.Thing)
			john.AddStatement(lci.Contains, blank)
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			john := ngA.CreateIRINode("John", lci.Person)
			blank := ngA.CreateBlankNode(lci.Individual)
			blank2 := ngA.CreateBlankNode(lci.Activity)
			blank3 := ngA.CreateBlankNode(lci.Class)
			john.AddStatement(lci.Contains, blank3)
			linda := ngA.CreateIRINode("Linda", lci.Person)
			linda.AddStatement(rdfs.Label, blank)
			linda.AddStatement(rdfs.Comment, blank2)
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		diffTriples := readAndWriteDiff(t, testName)
		assert.Equal(t, 8, len(diffTriples))
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)

		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("John"), "Expected IRINode 'John' not found")

		// Check that the John node contains a blank node of type lci.Thing
		john := ng.GetIRINodeByFragment("John")
		assert.NotNil(t, john, "Expected IRINode 'John' not found")
		objects := john.GetObjects(lci.Contains)
		found := false
		for _, obj := range objects {
			if obj.(sst.IBNode).IsBlankNode() {
				blankObjects := obj.(sst.IBNode).GetObjects(rdf.Type)
				assert.True(t, blankObjects[0].(sst.IBNode).Is(lci.Class), "Expected blank node object lci:Class not found")
				found = true
				break
			}
		}
		assert.True(t, found, "Expected blank node of type lci.Person contained in 'John' not found")
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})
}

// example 02: modify one of blankNodes
func TestWriteDiffModifyOneOfBlankNodes(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())

	defer os.Remove(testName + "NGA_before.sst")
	defer os.Remove(testName + "NGA_after.sst")
	defer os.Remove(testName + "diff.sst")
	defer os.Remove(testName + "NGA_before.ttl")
	defer os.Remove(testName + "NGA_after.ttl")

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})

	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			uuid1 := ngA.CreateIRINode("123e4567-e89b-12d3-a456-426614174001", lci.Individual)
			blank1 := ngA.CreateBlankNode(qau.Length)
			blank1.AddStatement(qau.Metre, sst.Double(5.0))
			uuid1.AddStatement(lci.Contains, blank1)

			uuid2 := ngA.CreateIRINode("123e4567-e89b-12d3-a456-426614174002", lci.Individual)
			blank2 := ngA.CreateBlankNode(qau.Length)
			blank2.AddStatement(qau.Metre, sst.Double(5.0))
			uuid2.AddStatement(lci.Contains, blank2)

		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			uuid1 := ngA.CreateIRINode("123e4567-e89b-12d3-a456-426614174001", lci.Individual)
			blank1 := ngA.CreateBlankNode(qau.Length)
			blank1.AddStatement(qau.Metre, sst.Double(7.0))
			uuid1.AddStatement(lci.Contains, blank1)

			uuid2 := ngA.CreateIRINode("123e4567-e89b-12d3-a456-426614174002", lci.Individual)
			blank2 := ngA.CreateBlankNode(qau.Length)
			blank2.AddStatement(qau.Metre, sst.Double(5.0))
			uuid2.AddStatement(lci.Contains, blank2)
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		diffTriples := readAndWriteDiff(t, testName)
		assert.Equal(t, 3, len(diffTriples))
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)

		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("123e4567-e89b-12d3-a456-426614174001"), "Expected IRINode 'UUID1' not found")

		// Check that the John node contains a blank node of type lci.Thing
		uuid1 := ng.GetIRINodeByFragment("123e4567-e89b-12d3-a456-426614174001")
		assert.NotNil(t, uuid1, "Expected IRINode 'UUID1' not found")
		objects := uuid1.GetObjects(lci.Contains)
		found := false
		for _, obj := range objects {
			if obj.(sst.IBNode).IsBlankNode() {
				blankObjects := obj.(sst.IBNode).GetObjects(qau.Metre)
				assert.Equal(t, sst.Double(7.0), blankObjects[0].(sst.Double), "Expected blank node object sst.Double(5.0) not found")
				found = true
				break
			}
		}
		assert.True(t, found, "Expected blank node of type lci.Person contained in 'John' not found")
	})
}

// example 03: modify to share a blankNode
func TestWriteDiffModifyToShareABlankNode(t *testing.T) {
	t.SkipNow()
	testName := filepath.Join("./testdata/" + t.Name())

	defer os.Remove(testName + "NGA_before.sst")
	defer os.Remove(testName + "NGA_after.sst")
	defer os.Remove(testName + "diff.sst")
	defer os.Remove(testName + "NGA_before.ttl")
	defer os.Remove(testName + "NGA_after.ttl")

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})

	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			uuid1 := ngA.CreateIRINode("123e4567-e89b-12d3-a456-426614174001", lci.Individual)
			blank1 := ngA.CreateBlankNode(qau.Length)
			blank1.AddStatement(qau.Metre, sst.Double(5.0))
			uuid1.AddStatement(lci.Contains, blank1)

			uuid2 := ngA.CreateIRINode("123e4567-e89b-12d3-a456-426614174002", lci.Individual)
			blank2 := ngA.CreateBlankNode(qau.Length)
			blank2.AddStatement(qau.Metre, sst.Double(5.0))
			uuid2.AddStatement(lci.Contains, blank2)

		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			uuid1 := ngA.CreateIRINode("123e4567-e89b-12d3-a456-426614174001", lci.Individual)
			blank1 := ngA.CreateBlankNode(qau.Length)
			blank1.AddStatement(qau.Metre, sst.Double(5.0))
			uuid1.AddStatement(lci.Contains, blank1)

			uuid2 := ngA.CreateIRINode("123e4567-e89b-12d3-a456-426614174002", lci.Individual)
			uuid2.AddStatement(lci.Contains, blank1)
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		diffTriples := readAndWriteDiff(t, testName)
		assert.Equal(t, 4, len(diffTriples))
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)

		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("123e4567-e89b-12d3-a456-426614174001"), "Expected IRINode 'UUID1' not found")

		// Check that the John node contains a blank node of type lci.Thing
		uuid1 := ng.GetIRINodeByFragment("123e4567-e89b-12d3-a456-426614174001")
		assert.NotNil(t, uuid1, "Expected IRINode 'UUID1' not found")
		objects := uuid1.GetObjects(lci.Contains)
		found := false
		for _, obj := range objects {
			if obj.(sst.IBNode).IsBlankNode() {
				blankObjects := obj.(sst.IBNode).GetObjects(qau.Metre)
				assert.Equal(t, sst.Double(7.0), blankObjects[0].(sst.Double), "Expected blank node object sst.Double(5.0) not found")
				found = true
				break
			}
		}
		assert.True(t, found, "Expected blank node of type lci.Person contained in 'John' not found")
	})
}

// example 04: modify one triple of blankNode that has several triples
func TestWriteDiffModifyATripleOfBlankNodeHasMultipleTriples(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})

	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			john := ngA.CreateIRINode("John", lci.Person)
			blank := ngA.CreateBlankNode(lci.Thing)
			blank.AddStatement(rdfs.Label, lci.Activity)
			john.AddStatement(lci.Contains, blank)
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			john := ngA.CreateIRINode("John", lci.Person)
			blank := ngA.CreateBlankNode(lci.Individual)
			blank.AddStatement(rdfs.Label, lci.Activity)
			john.AddStatement(lci.Contains, blank)
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		diffTriples := readAndWriteDiff(t, testName)
		assert.Equal(t, sst.DiffTripleFlag(0), diffTriples[0].Flag)
		assert.Equal(t, ":John", diffTriples[0].Sub)
		assert.Equal(t, "lci:contains", diffTriples[0].Pred)
		assert.Equal(t, "_:b0", diffTriples[0].Obj)

		assert.Equal(t, sst.DiffTripleFlag(-1), diffTriples[1].Flag)
		assert.Equal(t, "_:b0", diffTriples[1].Sub)
		assert.Equal(t, "rdf:type", diffTriples[1].Pred)
		assert.Equal(t, "lci:Thing", diffTriples[1].Obj)

		assert.Equal(t, sst.DiffTripleFlag(1), diffTriples[2].Flag)
		assert.Equal(t, "", diffTriples[2].Sub)
		assert.Equal(t, "", diffTriples[2].Pred)
		assert.Equal(t, "lci:Individual", diffTriples[2].Obj)

		assert.Equal(t, 3, len(diffTriples))
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)

		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("John"), "Expected IRINode 'John' not found")

		// Check that the John node contains a blank node of type lci.Thing
		john := ng.GetIRINodeByFragment("John")
		assert.NotNil(t, john, "Expected IRINode 'John' not found")
		objects := john.GetObjects(lci.Contains)
		found := false
		for _, obj := range objects {
			if obj.(sst.IBNode).IsBlankNode() {
				blankObjects := obj.(sst.IBNode).GetObjects(rdf.Type)
				assert.True(t, blankObjects[0].(sst.IBNode).Is(lci.Individual), "Expected blank node object lci:Thing not found")
				found = true
				break
			}
		}
		assert.True(t, found, "Expected blank node of type lci.Person contained in 'John' not found")
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})
}

// example 1: add a new Blank with its one triple
func TestWriteDiffAddANewBlankNode(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})

	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			ngA.CreateIRINode("John", lci.Person)
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			john := ngA.CreateIRINode("John", lci.Person)
			blank := ngA.CreateBlankNode(lci.Thing)
			john.AddStatement(lci.Contains, blank)
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		readAndWriteDiff(t, testName)
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)

		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("John"), "Expected IRINode 'John' not found")

		// Check that the John node contains a blank node of type lci.Thing
		john := ng.GetIRINodeByFragment("John")
		assert.NotNil(t, john, "Expected IRINode 'John' not found")
		objects := john.GetObjects(lci.Contains)
		found := false
		for _, obj := range objects {
			if obj.(sst.IBNode).IsBlankNode() {
				blankObjects := obj.(sst.IBNode).GetObjects(rdf.Type)
				assert.True(t, blankObjects[0].(sst.IBNode).Is(lci.Thing), "Expected blank node object lci:Thing not found")
				found = true
				break
			}
		}
		assert.True(t, found, "Expected blank node of type lci.Person contained in 'John' not found")
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})
}

// example 2: same term collection
func TestWriteDiffSameTermCollection(t *testing.T) {
	// t.Skip("skip for now due to term collection is not supported yet")
	testName := filepath.Join("./testdata/" + t.Name())

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})

	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			dct := ngA.CreateIRINode("John", lci.Organization)
			country := ngA.CreateIRINode("China")
			city := ngA.CreateIRINode("TianJin")
			col := ngA.CreateCollection(country, city)

			dct.AddStatement(lci.Contains, col)
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			dct := ngA.CreateIRINode("John", lci.Organization)
			country := ngA.CreateIRINode("China")
			city := ngA.CreateIRINode("TianJin")
			col := ngA.CreateCollection(country, city)

			dct.AddStatement(lci.Contains, col)
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		readAndWriteDiff(t, testName)
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)

		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("John"), "Expected IRINode 'John' not found")

		// Check that the John node contains a blank node of type lci.Person
		dct := ng.GetIRINodeByFragment("John")
		assert.NotNil(t, dct, "Expected IRINode 'John' not found")
		objects := dct.GetObjects(lci.Contains)
		for _, obj := range objects {
			assert.True(t, obj.(sst.TermCollection).Member(0).(sst.IBNode).Fragment() == "China")
			assert.True(t, obj.(sst.TermCollection).Member(1).(sst.IBNode).Fragment() == "TianJin")
		}
		ng.Dump()
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})
}

// example 3: add a new term collection
func TestWriteDiffAddTermCollection(t *testing.T) {
	// t.Skip("skip for now due to term collection is not supported yet")
	testName := filepath.Join("./testdata/" + t.Name())

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})

	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			ngA.CreateIRINode("John", lci.Organization)
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			dct := ngA.CreateIRINode("John", lci.Organization)
			country := ngA.CreateIRINode("China")
			city := ngA.CreateIRINode("TianJin")
			col := ngA.CreateCollection(country, city)

			dct.AddStatement(lci.Contains, col)
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		readAndWriteDiff(t, testName)
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)

		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("John"), "Expected IRINode 'John' not found")

		// Check that the John node contains a blank node of type lci.Person
		dct := ng.GetIRINodeByFragment("John")
		assert.NotNil(t, dct, "Expected IRINode 'John' not found")
		objects := dct.GetObjects(lci.Contains)
		for _, obj := range objects {
			assert.True(t, obj.(sst.TermCollection).Member(0).(sst.IBNode).Fragment() == "China")
			assert.True(t, obj.(sst.TermCollection).Member(1).(sst.IBNode).Fragment() == "TianJin")
		}
		ng.Dump()
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})
}

// example 3: add a new term collection
func TestWriteDiffAddATermCollection(t *testing.T) {
	// t.Skip("skip for now due to term collection is not supported yet")
	testName := filepath.Join("./testdata/" + t.Name())

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})

	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			ngA.CreateIRINode("John", lci.Organization)
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			dct := ngA.CreateIRINode("John", lci.Organization)
			country := ngA.CreateIRINode("China")
			city := ngA.CreateIRINode("TianJin")
			col := ngA.CreateCollection(country, city)

			dct.AddStatement(lci.Contains, col)
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		readAndWriteDiff(t, testName)
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)

		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("John"), "Expected IRINode 'John' not found")

		// Check that the John node contains a blank node of type lci.Person
		dct := ng.GetIRINodeByFragment("John")
		assert.NotNil(t, dct, "Expected IRINode 'John' not found")
		objects := dct.GetObjects(lci.Contains)
		for _, obj := range objects {
			assert.True(t, obj.(sst.TermCollection).Member(0).(sst.IBNode).Fragment() == "China")
			assert.True(t, obj.(sst.TermCollection).Member(1).(sst.IBNode).Fragment() == "TianJin")
		}
		ng.Dump()
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})
}

// example 4: add several term collection
func TestWriteDiffAddSeveralTermCollections(t *testing.T) {
	// t.Skip("skip for now due to term collection is not supported yet")
	testName := filepath.Join("./testdata/" + t.Name())

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})

	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			ngA.CreateIRINode("John", lci.Organization)
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			dct := ngA.CreateIRINode("John", lci.Organization)
			country := ngA.CreateIRINode("China")
			city := ngA.CreateIRINode("TianJin")
			col := ngA.CreateCollection(country, city)

			spring := ngA.CreateIRINode("Spring")
			summer := ngA.CreateIRINode("Summer")
			col2 := ngA.CreateCollection(spring, summer)
			dct.AddStatement(lci.Contains, col)
			dct.AddStatement(rdfs.Label, col2)
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		readAndWriteDiff(t, testName)
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)

		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("John"), "Expected IRINode 'John' not found")

		// Check that the John node contains a blank node of type lci.Person
		dct := ng.GetIRINodeByFragment("John")
		assert.NotNil(t, dct, "Expected IRINode 'John' not found")
		objects := dct.GetObjects(lci.Contains)
		for _, obj := range objects {
			assert.True(t, obj.(sst.TermCollection).Member(0).(sst.IBNode).Fragment() == "China")
			assert.True(t, obj.(sst.TermCollection).Member(1).(sst.IBNode).Fragment() == "TianJin")
		}
		ng.Dump()
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})
}

// example 5: remove a new term collection
func TestWriteDiffRemoveATermCollection(t *testing.T) {
	// t.Skip("skip for now due to term collection is not supported yet")
	testName := filepath.Join("./testdata/" + t.Name())

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})

	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			dct := ngA.CreateIRINode("John", lci.Organization)
			country := ngA.CreateIRINode("China")
			city := ngA.CreateIRINode("TianJin")
			col := ngA.CreateCollection(country, city)

			dct.AddStatement(lci.Contains, col)
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			ngA.CreateIRINode("John", lci.Organization)
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		readAndWriteDiff(t, testName)
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)

		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("John"), "Expected IRINode 'John' not found")

		// Check that the John node contains a blank node of type lci.Person
		dct := ng.GetIRINodeByFragment("John")
		assert.NotNil(t, dct, "Expected IRINode 'John' not found")
		objects := dct.GetObjects(lci.Contains)
		for _, obj := range objects {
			assert.True(t, obj.(sst.TermCollection).Member(0).(sst.IBNode).Fragment() == "China")
			assert.True(t, obj.(sst.TermCollection).Member(1).(sst.IBNode).Fragment() == "TianJin")
		}
		ng.Dump()
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})
}

// example 6: remove several term collection
func TestWriteDiffRemoveSeveralTermCollections(t *testing.T) {
	// t.Skip("skip for now due to term collection is not supported yet")
	testName := filepath.Join("./testdata/" + t.Name())

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})

	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			dct := ngA.CreateIRINode("John", lci.Organization)
			country := ngA.CreateIRINode("China")
			city := ngA.CreateIRINode("TianJin")
			col := ngA.CreateCollection(country, city)

			spring := ngA.CreateIRINode("Spring")
			summer := ngA.CreateIRINode("Summer")
			col2 := ngA.CreateCollection(spring, summer)
			dct.AddStatement(lci.Contains, col)
			dct.AddStatement(rdfs.Label, col2)
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			ngA.CreateIRINode("John", lci.Organization)
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		readAndWriteDiff(t, testName)
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)

		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("John"), "Expected IRINode 'John' not found")

		// Check that the John node contains a blank node of type lci.Person
		dct := ng.GetIRINodeByFragment("John")
		assert.NotNil(t, dct, "Expected IRINode 'John' not found")
		objects := dct.GetObjects(lci.Contains)
		for _, obj := range objects {
			assert.True(t, obj.(sst.TermCollection).Member(0).(sst.IBNode).Fragment() == "China")
			assert.True(t, obj.(sst.TermCollection).Member(1).(sst.IBNode).Fragment() == "TianJin")
		}
		ng.Dump()
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})
}

// example 7: remove then add several term collections
func TestWriteDiffRemoveThenAddSeveralTermCollections(t *testing.T) {
	// t.Skip("skip for now due to term collection is not supported yet")
	testName := filepath.Join("./testdata/" + t.Name())

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})

	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			dct := ngA.CreateIRINode("John", lci.Organization)
			country := ngA.CreateIRINode("China")
			city := ngA.CreateIRINode("TianJin")
			col1 := ngA.CreateCollection(country, city)

			spring := ngA.CreateIRINode("Spring")
			summer := ngA.CreateIRINode("Summer")
			col2 := ngA.CreateCollection(spring, summer)
			dct.AddStatement(lci.Contains, col1)
			dct.AddStatement(rdfs.Label, col2)
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			dct := ngA.CreateIRINode("John", lci.Organization)

			country := ngA.CreateIRINode("Japan")
			city := ngA.CreateIRINode("Tokyo")
			col := ngA.CreateCollection(country, city)
			dct.AddStatement(lci.Contains, col)

		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		readAndWriteDiff(t, testName)
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)

		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("John"), "Expected IRINode 'John' not found")

		// Check that the John node contains a blank node of type lci.Person
		dct := ng.GetIRINodeByFragment("John")
		assert.NotNil(t, dct, "Expected IRINode 'John' not found")
		objects := dct.GetObjects(lci.Contains)
		for _, obj := range objects {
			assert.True(t, obj.(sst.TermCollection).Member(0).(sst.IBNode).Fragment() == "Japan")
			assert.True(t, obj.(sst.TermCollection).Member(1).(sst.IBNode).Fragment() == "Tokyo")
		}
		// ng.Dump()
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})
}

// example 8: add term collection members and change order
func TestWriteDiffAddSeveralTermCollectionMembers(t *testing.T) {
	// t.Skip("skip for now due to term collection is not supported yet")
	testName := filepath.Join("./testdata/" + t.Name())

	defer os.Remove(testName + "NGA_before.sst")
	defer os.Remove(testName + "NGA_after.sst")
	defer os.Remove(testName + "diff.sst")
	defer os.Remove(testName + "NGA_before.ttl")
	defer os.Remove(testName + "NGA_after.ttl")
	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})

	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			dct := ngA.CreateIRINode("John", lci.Organization)
			country := ngA.CreateIRINode("China")
			city := ngA.CreateIRINode("TianJin")
			col := ngA.CreateCollection(country, city)
			dct.AddStatement(lci.Contains, col)
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			dct := ngA.CreateIRINode("John", lci.Organization)
			country := ngA.CreateIRINode("China")
			direction := ngA.CreateIRINode("North")
			city := ngA.CreateIRINode("TianJin")
			temp := ngA.CreateIRINode("Hot")

			col := ngA.CreateCollection(country, direction, city, temp)
			dct.AddStatement(lci.Contains, col)

		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		readAndWriteDiff(t, testName)
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)

		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("John"), "Expected IRINode 'John' not found")

		// Check that the John node contains a blank node of type lci.Person
		dct := ng.GetIRINodeByFragment("John")
		assert.NotNil(t, dct, "Expected IRINode 'John' not found")
		// objects := dct.GetObjects(lci.Contains)
		// for _, obj := range objects {
		// assert.True(t, obj.(sst.TermCollection).Member(0).(sst.IBNode).Fragment() == "China")
		// assert.True(t, obj.(sst.TermCollection).Member(1).(sst.IBNode).Fragment() == "TianJin")
		// }
		ng.Dump()
	})
}

// example 9: Change TermCollection Members Order
func TestWriteDiffChangeTermCollectionMembersOrder(t *testing.T) {
	// t.Skip("skip for now due to term collection is not supported yet")
	testName := filepath.Join("./testdata/" + t.Name())

	defer os.Remove(testName + "NGA_before.sst")
	defer os.Remove(testName + "NGA_after.sst")
	defer os.Remove(testName + "diff.sst")
	defer os.Remove(testName + "NGA_before.ttl")
	defer os.Remove(testName + "NGA_after.ttl")
	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})

	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			dct := ngA.CreateIRINode("John", lci.Organization)
			country := ngA.CreateIRINode("China")
			city := ngA.CreateIRINode("TianJin")
			col := ngA.CreateCollection(country, city)
			dct.AddStatement(lci.Contains, col)
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			dct := ngA.CreateIRINode("John", lci.Organization)
			country := ngA.CreateIRINode("China")
			city := ngA.CreateIRINode("TianJin")

			col := ngA.CreateCollection(city, country)
			dct.AddStatement(lci.Contains, col)
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		// {0 :John lci:contains _:b0}
		// {-1 _:b0 rdf:first :China}
		// {1 _:b0 rdf:first :TianJin}
		// {-1 _:b0 rdf:first :TianJin}
		// {1 _:b0 rdf:first :China}
		readAndWriteDiff(t, testName)
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)

		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("John"), "Expected IRINode 'John' not found")

		// Check that the John node contains a blank node of type lci.Person
		dct := ng.GetIRINodeByFragment("John")
		assert.NotNil(t, dct, "Expected IRINode 'John' not found")
		// objects := dct.GetObjects(lci.Contains)
		// for _, obj := range objects {
		// 	assert.True(t, obj.(sst.TermCollection).Member(0).(sst.IBNode).Fragment() == "China")
		// 	assert.True(t, obj.(sst.TermCollection).Member(1).(sst.IBNode).Fragment() == "TianJin")
		// }
	})
}
