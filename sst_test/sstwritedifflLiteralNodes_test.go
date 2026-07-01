// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/stretchr/testify/assert"
)

// example 1: add a new IBNode with only a String Literal triple
func TestWriteDiffAddLiteralStringNode(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())
	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			ngA.CreateIRINode("John", lci.Person)
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			ngA.CreateIRINode("John", lci.Person)
			lisa := ngA.CreateIRINode("Lisa")
			lisa.AddStatement(rdfs.Label, sst.String("China"))
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		readAndWriteDiff(t, testName)
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)

		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("John"), "Expected IRINode 'John' not found")

		// Check that the DCT node contains the expected literals
		lisa := ng.GetIRINodeByFragment("Lisa")
		assert.NotNil(t, lisa, "Expected IRINode 'Lisa' not found")
		objects := lisa.GetObjects(rdfs.Label)

		// Verify each literal type
		foundString := false

		for _, obj := range objects {
			switch v := obj.(type) {
			case sst.String:
				if v == "China" {
					foundString = true
				}
			}
		}

		assert.True(t, foundString, "Expected literal of type String with value 'China' not found")
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})
}

// example 2: add a new String Literal triple on an existing IBNode
func TestWriteDiffAddStringLiteralTripleOnExistingIBNode(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())
	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			ngA.CreateIRINode("John", lci.Person)
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			john := ngA.CreateIRINode("John", lci.Person)
			john.AddStatement(rdfs.Label, sst.String("China"))
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		readAndWriteDiff(t, testName)
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)

		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("John"), "Expected IRINode 'John' not found")

		// Check that the DCT node contains the expected literals
		john := ng.GetIRINodeByFragment("John")
		assert.NotNil(t, john, "Expected IRINode 'Lisa' not found")
		objects := john.GetObjects(rdfs.Label)

		// Verify each literal type
		foundString := false

		for _, obj := range objects {
			switch v := obj.(type) {
			case sst.String:
				if v == "China" {
					foundString = true
				}
			}
		}

		assert.True(t, foundString, "Expected literal of type String with value 'China' found")
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})
}

// example 3: remove a String Literal triple from an existing IBNode
func TestWriteDiffRemoveStringLiteralTripleOnExistingIBNode(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())
	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			john := ngA.CreateIRINode("John", lci.Person)
			john.AddStatement(rdfs.Label, sst.String("China"))
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			ngA.CreateIRINode("John", lci.Person)
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		readAndWriteDiff(t, testName)
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)

		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("John"), "Expected IRINode 'John' not found")

		// Check that the DCT node contains the expected literals
		john := ng.GetIRINodeByFragment("John")
		assert.NotNil(t, john, "Expected IRINode 'Lisa' not found")
		objects := john.GetObjects(rdfs.Label)

		// Verify each literal type
		foundString := false

		for _, obj := range objects {
			switch v := obj.(type) {
			case sst.String:
				if v == "China" {
					foundString = true
				}
			}
		}

		assert.False(t, foundString, "Expected literal of type String with value 'China' not found")
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})
}

// example 4: add String Literal triples on an existing IBNode
func TestWriteDiffAddStringLiteralTriplesOnExistingIBNode(t *testing.T) {
	// t.Skip("Literal still has problems, so skip this test")
	testName := filepath.Join("./testdata/" + t.Name())
	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			ngA.CreateIRINode("John", lci.Person)
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			john := ngA.CreateIRINode("John", lci.Person)
			john.AddStatement(rdfs.Label, sst.String("China"))
			john.AddStatement(rdfs.Label, sst.String("Asian"))
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		readAndWriteDiff(t, testName)
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)

		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("John"), "Expected IRINode 'John' not found")

		// Check that the DCT node contains the expected literals
		john := ng.GetIRINodeByFragment("John")
		assert.NotNil(t, john, "Expected IRINode 'Lisa' not found")
		objects := john.GetObjects(rdfs.Label)

		// Verify each literal type
		foundStringCount := 0

		for _, obj := range objects {
			switch v := obj.(type) {
			case sst.String:
				if v == "China" {
					foundStringCount++
				}
				if v == "Asian" {
					foundStringCount++
				}
			}
		}

		assert.Equal(t, foundStringCount, 2)
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})
}

// example 5: remove String Literal triples from an existing IBNode
func TestWriteDiffRemoveStringLiteralTriplesOnExistingIBNode(t *testing.T) {
	// t.Skip("Literal still has problems, so skip this test")
	testName := filepath.Join("./testdata/" + t.Name())
	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			john := ngA.CreateIRINode("John", lci.Person)
			john.AddStatement(rdfs.Label, sst.String("China"))
			john.AddStatement(rdfs.Label, sst.String("Asian"))
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			ngA.CreateIRINode("John", lci.Person)
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		readAndWriteDiff(t, testName)
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)

		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("John"), "Expected IRINode 'John' not found")

		// Check that the DCT node contains the expected literals
		john := ng.GetIRINodeByFragment("John")
		assert.NotNil(t, john, "Expected IRINode 'Lisa' not found")
		objects := john.GetObjects(rdfs.Label)

		// Verify each literal type
		foundString := false

		for _, obj := range objects {
			switch v := obj.(type) {
			case sst.String:
				if v == "China" {
					foundString = true
				}
				if v == "Asian" {
					foundString = true
				}
			}
		}

		assert.False(t, foundString, "Expected literal of type String with value 'China' not found")
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})
}

// example 6: add all types of Literal triples on an existing IBNode
func TestWriteDiffAddDifferentTypeOfLiteralTriples(t *testing.T) {
	// t.Skip("Literal still has problems, so skip this test")
	testName := filepath.Join("./testdata/" + t.Name())
	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			ngA.CreateIRINode("John", lci.Person)
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			john := ngA.CreateIRINode("John", lci.Person)

			john.AddStatement(rdfs.Label, sst.String("China"))
			john.AddStatement(rdfs.Label, sst.LangString{Val: "Hello", LangTag: "en"})
			john.AddStatement(rdfs.Label, sst.Double(3.14))
			john.AddStatement(rdfs.Label, sst.Integer(42))
			john.AddStatement(rdfs.Label, sst.Boolean(true))
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		readAndWriteDiff(t, testName)
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)

		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("John"), "Expected IRINode 'John' not found")

		// Check that the DCT node contains the expected literals
		john := ng.GetIRINodeByFragment("John")
		assert.NotNil(t, john, "Expected IRINode 'John' not found")
		objects := john.GetObjects(rdfs.Label)

		// Verify each literal type
		foundString := false
		foundLangString := false
		foundDouble := false
		foundInteger := false
		foundBoolean := false

		for _, obj := range objects {
			switch v := obj.(type) {
			case sst.String:
				if v == "China" {
					foundString = true
				}
			case sst.LangString:
				if v.Val == "Hello" && v.LangTag == "en" {
					foundLangString = true
				}
			case sst.Double:
				if v == 3.14 {
					foundDouble = true
				}
			case sst.Integer:
				if v == 42 {
					foundInteger = true
				}
			case sst.Boolean:
				if v {
					foundBoolean = true
				}
			}
		}

		assert.True(t, foundString, "Expected literal of type String with value 'China' not found")
		assert.True(t, foundLangString, "Expected literal of type LangString with value 'Hello' and lang 'en' not found")
		assert.True(t, foundDouble, "Expected literal of type Double with value 3.14 not found")
		assert.True(t, foundInteger, "Expected literal of type Integer with value 42 not found")
		assert.True(t, foundBoolean, "Expected literal of type Boolean with value true not found")
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})
}

// example 7: remove different types of Literal triples on an existing IBNode
func TestWriteDiffRemoveDifferentTypeOfLiteralTriples(t *testing.T) {
	// t.Skip("Literal still has problems, so skip this test")
	testName := filepath.Join("./testdata/" + t.Name())
	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			john := ngA.CreateIRINode("John", lci.Person)
			john.AddStatement(rdfs.Label, sst.String("China"))
			john.AddStatement(rdfs.Label, sst.LangString{Val: "Hello", LangTag: "en"})
			john.AddStatement(rdfs.Label, sst.Double(3.14))
			john.AddStatement(rdfs.Label, sst.Integer(42))
			john.AddStatement(rdfs.Label, sst.Boolean(true))
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			ngA.CreateIRINode("John", lci.Person)
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		readAndWriteDiff(t, testName)
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)

		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("John"), "Expected IRINode 'John' not found")

		// Check that the DCT node contains the expected literals
		john := ng.GetIRINodeByFragment("John")
		assert.NotNil(t, john, "Expected IRINode 'John' not found")
		objects := john.GetObjects(rdfs.Label)

		// Verify each literal type
		foundString := false
		foundLangString := false
		foundDouble := false
		foundInteger := false
		foundBoolean := false

		for _, obj := range objects {
			switch v := obj.(type) {
			case sst.String:
				if v == "China" {
					foundString = true
				}
			case sst.LangString:
				if v.Val == "Hello" && v.LangTag == "en" {
					foundLangString = true
				}
			case sst.Double:
				if v == 3.14 {
					foundDouble = true
				}
			case sst.Integer:
				if v == 42 {
					foundInteger = true
				}
			case sst.Boolean:
				if v {
					foundBoolean = true
				}
			}
		}

		assert.False(t, foundString, "Expected literal of type String with value 'China' not found")
		assert.False(t, foundLangString, "Expected literal of type LangString with value 'Hello' and lang 'en' not found")
		assert.False(t, foundDouble, "Expected literal of type Double with value 3.14 not found")
		assert.False(t, foundInteger, "Expected literal of type Integer with value 42 not found")
		assert.False(t, foundBoolean, "Expected literal of type Boolean with value true not found")
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})
}

// example 8: add a Literal Collection triple on an existing IBNode
func TestWriteDiffAddLiteralCollectionTriple(t *testing.T) {
	// t.Skip("Literal still has problems, so skip this test")
	testName := filepath.Join("./testdata/" + t.Name())
	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			ngA.CreateIRINode("John", lci.Person)
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			john := ngA.CreateIRINode("John", lci.Person)
			john.AddStatement(rdfs.Label, sst.NewLiteralCollection(sst.String("China"), sst.String("TianJin")))
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		readAndWriteDiff(t, testName)
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)
		ng.Dump()
		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("John"), "Expected IRINode 'John' not found")

		// // Check that the DCT node contains the expected literals
		john := ng.GetIRINodeByFragment("John")
		assert.NotNil(t, john, "Expected IRINode 'John' not found")
		// objects := john.GetObjects(rdfs.Label)

		john.ForAll(func(i1 int, sub, pred sst.IBNode, obj sst.Term) error {
			switch o := obj.(type) {
			case sst.LiteralCollection:
				o.ForMembers(func(index int, literal sst.Literal) {
					switch index {
					case 0:
						assert.Equal(t, string(literal.(sst.String)), "China")
					case 1:
						assert.Equal(t, string(literal.(sst.String)), "TianJin")
					}
				})
				assert.Equal(t, string(o.Member(0).(sst.String)), "China")
				assert.Equal(t, string(o.Member(1).(sst.String)), "TianJin")
			}
			return nil
		})
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})
}

// example 9: remove a Literal Collection triple on an existing IBNode
func TestWriteDiffRemoveLiteralCollectionTriple(t *testing.T) {
	// t.Skip("Literal still has problems, so skip this test")
	testName := filepath.Join("./testdata/" + t.Name())
	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			john := ngA.CreateIRINode("John", lci.Person)
			john.AddStatement(rdfs.Label, sst.NewLiteralCollection(sst.Boolean(true), sst.Boolean(false)))
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			ngA.CreateIRINode("John", lci.Person)
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		readAndWriteDiff(t, testName)
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)
		ng.Dump()
		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("John"), "Expected IRINode 'John' not found")

		// // Check that the DCT node contains the expected literals
		john := ng.GetIRINodeByFragment("John")
		assert.NotNil(t, john, "Expected IRINode 'John' not found")
		// objects := john.GetObjects(rdfs.Label)

		john.ForAll(func(i1 int, sub, pred sst.IBNode, obj sst.Term) error {
			switch o := obj.(type) {
			case sst.LiteralCollection:
				o.ForMembers(func(index int, literal sst.Literal) {
					switch index {
					case 0:
						assert.Equal(t, bool(literal.(sst.Boolean)), true)
					case 1:
						assert.Equal(t, bool(literal.(sst.Boolean)), false)
					}
				})
			}
			return nil
		})
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})
}

// example 10: remove a Literal Collection triple on an existing IBNode and add Literal and LiteralCollection
func TestWriteDiffLiteralAndLiteralCollectionTriple(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())
	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			john := ngA.CreateIRINode("John", lci.Person)

			john.AddStatement(rdfs.Label, sst.String("Europe"))
			john.AddStatement(rdfs.Comment, sst.NewLiteralCollection(sst.Boolean(true), sst.Boolean(false)))
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			john := ngA.CreateIRINode("John", lci.Person)
			john.AddStatement(rdfs.Label, sst.String("China"))
			john.AddStatement(rdfs.Label, sst.String("Asian"))
			john.AddStatement(rdfs.Label, sst.NewLiteralCollection(sst.Double(3.14), sst.Double(3.15), sst.Double(3.16)))
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		diffTriples := readAndWriteDiff(t, testName)
		assert.Equal(t, 5, len(diffTriples))
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)
		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("John"), "Expected IRINode 'John' not found")

		// // Check that the DCT node contains the expected literals
		john := ng.GetIRINodeByFragment("John")
		assert.NotNil(t, john, "Expected IRINode 'John' not found")
		// objects := john.GetObjects(rdfs.Label)

		// Verify each literal type
		foundStringCount := 0
		john.ForAll(func(i1 int, sub, pred sst.IBNode, obj sst.Term) error {
			switch o := obj.(type) {
			case sst.LiteralCollection:
				o.ForMembers(func(index int, literal sst.Literal) {
					switch index {
					case 0:
						assert.Equal(t, float64(literal.(sst.Double)), 3.14)
					case 1:
						assert.Equal(t, float64(literal.(sst.Double)), 3.15)
					case 2:
						assert.Equal(t, float64(literal.(sst.Double)), 3.16)
					}
				})
			case sst.String:
				if o == "China" {
					foundStringCount++
				}
				if o == "Asian" {
					foundStringCount++
				}

			}
			return nil
		})

		assert.Equal(t, 2, foundStringCount)
	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})
}

// example 11: remove a Literal Collection triple on an existing IBNode and add Literal and LiteralCollection
func TestWriteDiffLiteralLiteralCollectionTriple(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())
	t.Run("CreateAndWrite", func(t *testing.T) {
		createAndWriteNamedGraph(testName+"NGA_before", func(stage sst.Stage, ngA sst.NamedGraph) {
			john := ngA.CreateIRINode("John", lci.Person)

			john.AddStatement(rdfs.Comment, sst.NewLiteralCollection(sst.String("a"), sst.String("b"), sst.String("c")))
		})
		createAndWriteNamedGraph(testName+"NGA_after", func(stage sst.Stage, ngA sst.NamedGraph) {
			john := ngA.CreateIRINode("John", lci.Person)
			john.AddStatement(rdfs.Comment, sst.NewLiteralCollection(sst.String("a"), sst.String("c"), sst.String("x")))
		})
	})

	t.Run("ReadAndWriteDiff", func(t *testing.T) {
		diffTriples := readAndWriteDiff(t, testName)
		assert.Equal(t, 2, len(diffTriples))
	})

	t.Run("ReadAndReadDiff", func(t *testing.T) {
		ng := readAndApplyDiff(t, testName)
		// Assert that the NamedGraph contains the expected nodes and relationships using assert package
		assert.NotNil(t, ng.GetIRINodeByFragment("John"), "Expected IRINode 'John' not found")

		// // Check that the DCT node contains the expected literals
		john := ng.GetIRINodeByFragment("John")
		assert.NotNil(t, john, "Expected IRINode 'John' not found")

		john.ForAll(func(i1 int, sub, pred sst.IBNode, obj sst.Term) error {
			switch o := obj.(type) {
			case sst.LiteralCollection:
				o.ForMembers(func(index int, literal sst.Literal) {
					switch index {
					case 0:
						assert.Equal(t, string(literal.(sst.String)), "a")
					case 1:
						assert.Equal(t, string(literal.(sst.String)), "c")
					case 2:
						assert.Equal(t, string(literal.(sst.String)), "x")
					}
				})
			}
			return nil
		})

	})

	t.Run("Cleanup", func(t *testing.T) {
		os.Remove(testName + "NGA_before.sst")
		os.Remove(testName + "NGA_after.sst")
		os.Remove(testName + "diff.sst")
		os.Remove(testName + "NGA_before.ttl")
		os.Remove(testName + "NGA_after.ttl")
	})
}

func T(f sst.DiffTripleFlag, sub, pred, obj string) sst.DiffTriple {
	return sst.DiffTriple{Flag: f, Sub: sub, Pred: pred, Obj: obj}
}

func Test_DiffOnLiterals(t *testing.T) {
	ttl1 := `@prefix sso:	<http://ontology.semanticstep.net/sso#> .
			@prefix rdf:	<http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
			@prefix owl:	<http://www.w3.org/2002/07/owl#> .
			@prefix lci:   <http://ontology.semanticstep.net/lci#> .
			@prefix qau:	<http://ontology.semanticstep.net/qau#> .
			@prefix :	<urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f613a361#> .

			<urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f613a361>	a	owl:Ontology .

			:uuid1 
				rdf:type lci:Individual ;
				sso:occurrenceQuantity _:b1 .
			:uuid2 
				rdf:type lci:Individual ;
				sso:occurrenceQuantity _:b2 .
			_:b1 
				rdf:type qau:Length ; 
				qau:metre 5.0 .
			_:b2 
				rdf:type qau:Length ; 
				qau:metre 5.0 .
			`

	ttl2 := `@prefix sso:	<http://ontology.semanticstep.net/sso#> .
			@prefix rdf:	<http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
			@prefix owl:	<http://www.w3.org/2002/07/owl#> .
			@prefix lci:   <http://ontology.semanticstep.net/lci#> .
			@prefix qau:	<http://ontology.semanticstep.net/qau#> .
			@prefix :	<urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f613a361#> .

			<urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f613a361>	a	owl:Ontology .

			:uuid1
				rdf:type lci:Individual  ;
				sso:occurrenceQuantity _:b3 .
			:uuid2
				rdf:type lci:Individual  ;
				sso:occurrenceQuantity _:b4 .
			_:b3
				rdf:type qau:Length ; 
				qau:metre 7.0 .
			_:b4
				rdf:type qau:Length ; 
				qau:metre 5.0 .
			`
	ttl3 := `@prefix sso:	<http://ontology.semanticstep.net/sso#> .
			@prefix rdf:	<http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
			@prefix owl:	<http://www.w3.org/2002/07/owl#> .
			@prefix lci:   <http://ontology.semanticstep.net/lci#> .
			@prefix qau:	<http://ontology.semanticstep.net/qau#> .
			@prefix :	<urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f613a361#> .

			<urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f613a361>	a	owl:Ontology .

			:uuid1
				rdf:type lci:Individual ;
				sso:occurrenceQuantity _:b5 .
			:uuid2
				rdf:type lci:Individual ;
				sso:occurrenceQuantity _:b5 .
			_:b5
				rdf:type qau:Length ; 
				qau:metre 5.0 .
			`

	ttl4 := `@prefix sso:	<http://ontology.semanticstep.net/sso#> .
			@prefix rdf:	<http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
			@prefix owl:	<http://www.w3.org/2002/07/owl#> .
			@prefix lci:   <http://ontology.semanticstep.net/lci#> .
			@prefix qau:	<http://ontology.semanticstep.net/qau#> .
			@prefix :	<urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f613a361#> .

			<urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f613a361>	a	owl:Ontology .

			:uuid1 
				rdf:type lci:Individual ;
				sso:occurrenceQuantity _:b6 .
			:uuid2 
				rdf:type lci:Individual ;
				sso:occurrenceQuantity _:b6 .
			_:b6
				rdf:type qau:Length ; 
				qau:metre 7.0 .`

	tests := []struct {
		name    string
		ttlFrom string
		ttlTo   string
		want    []sst.DiffTriple
	}{
		{
			name:    "modify one blank Node",
			ttlFrom: ttl1,
			ttlTo:   ttl2,
			want: []sst.DiffTriple{
				T(0, ":uuid1", "sso:occurrenceQuantity", "_:b0"),
				T(-1, "_:b0", "qau:metre", "5.00"),
				T(+1, "", "", "7.00"),
			},
		},
		{
			name:    "use a shared blank Node",
			ttlFrom: ttl1,
			ttlTo:   ttl3,
			want: []sst.DiffTriple{
				T(-1, ":uuid2", "sso:occurrenceQuantity", "_:b1"),
				T(+1, "", "", "_:b0"),
				T(-1, "_:b1", "qau:metre", "5.00"),
				T(-1, "", "rdf:type", "qau:Length"),
			},
		},
		{
			name:    "modify then use a shared blank Node",
			ttlFrom: ttl1,
			ttlTo:   ttl4,
			want: []sst.DiffTriple{
				T(0, ":uuid1", "sso:occurrenceQuantity", "_:b0"),
				T(-1, ":uuid2", "sso:occurrenceQuantity", "_:b1"),
				T(+1, "", "", "_:b0"),
				T(-1, "_:b1", "qau:metre", "5.00"),
				T(-1, "", "rdf:type", "qau:Length"),
				T(-1, "_:b0", "qau:metre", "5.00"),
				T(+1, "", "", "7.00"),
			},
		},

		// {0 :uuid1 sso:occurrenceQuantity _:b0}
		// {1 :uuid2 sso:occurrenceQuantity _:b0}
		// {-1 :uuid2 sso:occurrenceQuantity _:b1}
		// {-1 _:b1 rdf:type qau:Length}
		// {-1 _:b0 qau:metre 5.00}
		// {1 _:b0 qau:metre 7.00}
		// {-1 _:b1 qau:metre 5.00}
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stage1, err := sst.RdfRead(bufio.NewReader(strings.NewReader(tt.ttlFrom)), sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
			if err != nil {
				fmt.Printf("Error: Failed to read RDF file: %v\n", err)
				return
			}
			var buf1 bytes.Buffer
			err = stage1.NamedGraphs()[0].SstWrite(&buf1)
			if err != nil {
				log.Panic(err)
			}

			stage2, err := sst.RdfRead(bufio.NewReader(strings.NewReader(tt.ttlTo)), sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
			if err != nil {
				fmt.Printf("Error: Failed to read RDF file: %v\n", err)
				return
			}
			var buf2 bytes.Buffer
			err = stage2.NamedGraphs()[0].SstWrite(&buf2)
			if err != nil {
				log.Panic(err)
			}

			var out bytes.Buffer
			diffTriples, err := sst.SstWriteDiff(bufio.NewReader(&buf1), bufio.NewReader(&buf2), &out, true)
			assert.NoError(t, err)

			if diff := cmp.Diff(tt.want, diffTriples); diff != "" {
				t.Fatalf("mismatch (-want +got):\n%s", diff)
			}

		})
	}
}
