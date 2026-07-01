// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestSstWrite(t *testing.T) {
	tests := []struct {
		name         string
		graphUUID    []uuid.UUID
		graphCreator func(*testing.T, uuid.UUID, ...uuid.UUID) NamedGraph
		wantOutput   string
		assertion    assert.ErrorAssertionFunc
	}{
		{
			name:         "emptyGraph",
			graphUUID:    []uuid.UUID{uuid.MustParse("e38c8811-1028-4442-af79-2cce833feb4e")},
			graphCreator: testEmptyGraph,
			// \x00 imported namedgraph count
			// \x00 referenced namedgraph count
			// \x01 iri Nodes count
			// \x00 blank Nodes count
			// \x00 NG Node fragment lens
			// \x00 NG Node all triples count
			// \x00 NG Node non-literal triple count
			// \x00 NG Node literal triple count
			wantOutput: "SST-1.0\x00-urn:uuid:e38c8811-1028-4442-af79-2cce833feb4e\x00\x00\x01\x00\x00\x00\x00\x00",
			assertion:  assert.NoError,
		},
		{
			name:         "emptyURIGraph",
			graphUUID:    []uuid.UUID{uuid.Nil},
			graphCreator: testEmptyGraphWithURI(mustIRI("http://www.w3.org/1999/02/22-rdf-syntax-ns#")),
			wantOutput:   "SST-1.0\x00*http://www.w3.org/1999/02/22-rdf-syntax-ns\x00\x00\x01\x00\x00\x00\x00\x00",
			assertion:    assert.NoError,
		},
		{
			name:         "graphWithNodesAndBlankNodes",
			graphUUID:    []uuid.UUID{uuid.MustParse("546a46db-4273-4a6c-8031-1726e869b3d2")},
			graphCreator: testGraphWithNodesNoImports,
			// \x00\x01
			// \x1ehttp://semanticstep.net/schema
			// \x03\x01
			// \x00\x00
			// \x02s1\x01
			// \x02s2\x02
			// \x01
			// \x02
			// \x02p1\x01
			// \x02p2\x01
			// \x00\x00
			// \x01\x04\x02\x00
			// \x01\x05\x03\x00
			// \x00\x00
			wantOutput: "SST-1.0\x00-urn:uuid:546a46db-4273-4a6c-8031-1726e869b3d2\x00\x01\x1ehttp://semanticstep.net/schema\x03\x01\x00\x00\x02s1\x01\x02s2\x02\x01\x02\x02p1\x01\x02p2\x01\x00\x00\x01\x04\x02\x00\x01\x05\x03\x00\x00\x00",
			assertion:  assert.NoError,
		},
		{
			name: "withNodesAndImport",
			graphUUID: []uuid.UUID{
				uuid.MustParse("39c59163-2526-4fc4-8c56-644d1b3b81d4"),
				uuid.MustParse("91bedd7d-8f59-48e8-98fa-c06ec328e4a1"),
			},
			graphCreator: testGraphWithNodesAndImport,
			wantOutput:   "SST-1.0\x00-urn:uuid:39c59163-2526-4fc4-8c56-644d1b3b81d4\x01-urn:uuid:91bedd7d-8f59-48e8-98fa-c06ec328e4a1\x01\x1ehttp://semanticstep.net/schema\x03\x01\x00\x00\x02s1\x01\x02s2\x02\x01\x00\x02\x02p1\x01\x02p2\x01\x00\x00\x01\x04\x02\x00\x01\x05\x03\x00\x00\x00",
			assertion:    assert.NoError,
		},
		{
			name: "withNodesAndImports",
			graphUUID: []uuid.UUID{
				uuid.MustParse("39c59163-2526-4fc4-8c56-644d1b3b81d4"),
				uuid.MustParse("91bedd7d-8f59-48e8-98fa-c06ec328e4a1"),
				uuid.MustParse("c9137479-b4bf-4870-8e16-be6936513d58"),
			},
			graphCreator: testGraphWithNodesAndImports,
			wantOutput:   "SST-1.0\x00-urn:uuid:39c59163-2526-4fc4-8c56-644d1b3b81d4\x02-urn:uuid:91bedd7d-8f59-48e8-98fa-c06ec328e4a1-urn:uuid:c9137479-b4bf-4870-8e16-be6936513d58\x01\x1ehttp://semanticstep.net/schema\x03\x01\x00\x00\x02s1\x01\x02s2\x02\x01\x00\x00\x02\x02p1\x01\x02p2\x01\x00\x00\x01\x04\x02\x00\x01\x05\x03\x00\x00\x00",
			assertion:    assert.NoError,
		},
		{
			name: "withImportedGraphs",
			graphUUID: []uuid.UUID{
				uuid.MustParse("43e199ee-ac39-46c6-852f-50704fdccaef"),
				uuid.MustParse("5184e8b3-0649-493d-8b61-a2a2b42c4f24"),
				uuid.MustParse("d7bb18e3-b830-42dc-97cc-3f3a14317caf"),
			},
			graphCreator: testGraphWithImportedGraphs,
			wantOutput:   "SST-1.0\x00-urn:uuid:43e199ee-ac39-46c6-852f-50704fdccaef\x02-urn:uuid:5184e8b3-0649-493d-8b61-a2a2b42c4f24-urn:uuid:d7bb18e3-b830-42dc-97cc-3f3a14317caf\x01\x1ehttp://semanticstep.net/schema\x03\x00\x00\x00\x03sA1\x03\x03sA2\x01\x01\x03sB1\x01\x01\x03sC1\x01\x04\x02p1\x02\x02p2\x02\x02t1\x01\x02t2\x01\x00\x00\x03\x05\a\x06\x03\x06\x04\x00\x01\x05\b\x00",
			assertion:    assert.NoError,
		},
		{
			name:      "uriGraphWithImport",
			graphUUID: []uuid.UUID{uuid.Nil},
			graphCreator: func(t *testing.T, u0 uuid.UUID, u1n ...uuid.UUID) NamedGraph {
				graph := testEmptyGraphWithURI(mustIRI("http://www.w3.org/2002/07/owl#"))(t, u0, u1n...)
				graphB := graph.Stage().CreateNamedGraph(IRI("http://www.w3.org/2000/01/rdf-schema#"))
				graph.AddImport(graphB)
				return graph
			},
			wantOutput: "SST-1.0\x00\x1dhttp://www.w3.org/2002/07/owl\x01$http://www.w3.org/2000/01/rdf-schema\x00\x01\x00\x00\x00\x00\x00\x00",
			assertion:  assert.NoError,
		},
		{
			name:         "graphWithLiterals",
			graphUUID:    []uuid.UUID{uuid.MustParse("d270203d-2598-4a71-80b1-50576a1fda84")},
			graphCreator: testGraphWithLiterals,
			wantOutput:   "SST-1.0\x00-urn:uuid:d270203d-2598-4a71-80b1-50576a1fda84\x00\x01\x1ehttp://semanticstep.net/schema\x02\x00\x00\x00\x02s1\x03\x03\x02p1\x01\x02p2\x01\x02p3\x01\x00\x00\x00\x03\x02\x00\x04str1\x03\x04\x85ҩ\x9a!\x04\x05?\xf3333333",
			assertion:    assert.NoError,
		},
		{
			name:         "graphWithLiteralAndLiteralCollection",
			graphUUID:    []uuid.UUID{uuid.MustParse("2a49b2d9-ddbd-49bc-ba58-2fbea7cf6792")},
			graphCreator: testGraphWithLiteralAndLiteralCollection,
			wantOutput:   "SST-1.0\x00-urn:uuid:2a49b2d9-ddbd-49bc-ba58-2fbea7cf6792\x00\x01\x1ehttp://semanticstep.net/schema\x03\x00\x00\x00\x02s1\x03\x02s2\x01\x03\x02p1\x01\x02p2\x01\x02p3\x01\x00\x00\x01\x05\x02\x02\x03\x04\x14\x04\x7f\x02\x00\x03cl1\x00\x03cl2\x00\x00",
			assertion:    assert.NoError,
		},
		{
			name:         "writeGraphWithTermCollection",
			graphUUID:    []uuid.UUID{uuid.MustParse("0c20d026-3fda-4e35-ab7d-88ba6cc0c5fc")},
			graphCreator: testGraphWithTermCollection,
			wantOutput:   "SST-1.0\x00-urn:uuid:0c20d026-3fda-4e35-ab7d-88ba6cc0c5fc\x00\x02\x1ehttp://semanticstep.net/schema*http://www.w3.org/1999/02/22-rdf-syntax-ns\x03\x01\x00\x00\x02s1\x01\x02s2\x01\x02\x01\x02p1\x01\x01\x05first\x01\x00\x00\x01\x04\x03\x00\x00\x00\x01\x05\x02\x00",
			assertion:    assert.NoError,
		},
		{
			name:         "writeGraphWithTermCollectionAndTriple",
			graphUUID:    []uuid.UUID{uuid.MustParse("0c20d026-3fda-4e35-ab7d-88ba6cc0c5fc")},
			graphCreator: testGraphWithTermCollectionAndTriple,
			wantOutput:   "SST-1.0\x00-urn:uuid:0c20d026-3fda-4e35-ab7d-88ba6cc0c5fc\x00\x02\x1ehttp://semanticstep.net/schema*http://www.w3.org/1999/02/22-rdf-syntax-ns\x03\x01\x00\x00\x02s1\x01\x02s2\x01\x03\x01\x02p1\x02\x01\x05first\x01\x00\x00\x01\x04\x03\x00\x00\x00\x01\x05\x02\x01\x04\x00\x04val1",
			assertion:    assert.NoError,
		},
		{
			name:         "writeGraphWithTermCollectionMembers",
			graphUUID:    []uuid.UUID{uuid.MustParse("0c20d026-3fda-4e35-ab7d-88ba6cc0c5fc")},
			graphCreator: testGraphWithTermCollectionMembers,
			wantOutput:   "SST-1.0\x00-urn:uuid:0c20d026-3fda-4e35-ab7d-88ba6cc0c5fc\x00\x02\x1ehttp://semanticstep.net/schema*http://www.w3.org/1999/02/22-rdf-syntax-ns\x03\x01\x00\x00\x02s1\x01\x02s2\x02\x04\x01\x02p1\x01\x01\x05first\x03\x00\x00\x01\x04\x03\x00\x00\x00\x03\x05\x02\x05\x05\x05\x02\x01\x05\x00\x04val2",
			assertion:    assert.NoError,
		},
		{
			name:         "writeGraphWithMemberCollection",
			graphUUID:    []uuid.UUID{uuid.MustParse("0c20d026-3fda-4e35-ab7d-88ba6cc0c5fc")},
			graphCreator: testGraphWithMemberCollection,
			wantOutput:   "SST-1.0\x00-urn:uuid:0c20d026-3fda-4e35-ab7d-88ba6cc0c5fc\x00\x02\x1ehttp://semanticstep.net/schema*http://www.w3.org/1999/02/22-rdf-syntax-ns\x03\x02\x00\x00\x02s1\x01\x02s2\x02\x02\x03\x01\x02p1\x01\x01\x05first\x03\x00\x00\x01\x05\x04\x00\x00\x00\x01\x06\x02\x00\x02\x06\x02\x06\x03\x00",
			assertion:    assert.NoError,
		},

		{
			name:         "fullKindsNodes",
			graphUUID:    []uuid.UUID{uuid.MustParse("0c20d026-3fda-4e35-ab7d-88ba6cc0c5fc")},
			graphCreator: testFullKindsNodes,
			wantOutput:   "SST-1.0\x00-urn:uuid:0c20d026-3fda-4e35-ab7d-88ba6cc0c5fc\x00\x03\x13http://example.com/$http://ontology.semanticstep.net/lci*http://www.w3.org/1999/02/22-rdf-syntax-ns\x04\x02\x00\x01\x05Comp1\x04\x04John\x02\x05Linda\x02\x03\x03\x02\temployees\x01\astrings\x01\x04\fOrganization\x02\x06Person\x02\ahasPart\x01\x10integerAsLiteral\x02\x02\x05first\x02\x04type\x04\x00\x01\v\x04\x1a\x03\r\b\x06\x04\n\x05\x01\a\x7f\x02\x00\x03st1\x00\x03st2\x01\r\t\x00\x01\r\t\x00\x02\f\x02\f\x03\x00\x01\r\b\x01\v\x04\x1c",
			assertion:    assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AtomicLevel.SetLevel(zap.DebugLevel) // Enable debug logging for the test
			graph := tt.graphCreator(t, tt.graphUUID[0], tt.graphUUID[1:]...)
			var output bytes.Buffer
			tt.assertion(t, graph.SstWrite(&output))
			assert.Equal(t, tt.wantOutput, output.String())
		})
	}
}

func testEmptyGraph(t *testing.T, testGraphUUID uuid.UUID, _ ...uuid.UUID) NamedGraph {
	t.Helper()
	stage := OpenStage(DefaultTriplexMode)
	graph := stage.CreateNamedGraph(IRI(testGraphUUID.URN()))
	return graph
}

func mustIRI(iriStr string) IRI {
	iri, err := NewIRI(iriStr)
	if err != nil {
		panic(err)
	}
	return iri
}

func testEmptyGraphWithURI(testGraphURI IRI,
) func(t *testing.T, _ uuid.UUID, _ ...uuid.UUID) NamedGraph {
	return func(t *testing.T, testGraphUUID uuid.UUID, _ ...uuid.UUID) NamedGraph {
		t.Helper()
		stage := OpenStage(DefaultTriplexMode)
		graph := stage.CreateNamedGraph(testGraphURI)
		return graph
	}
}

func testGraphWithNodesNoImports(t *testing.T, testGraphUUID uuid.UUID, _ ...uuid.UUID) NamedGraph {
	t.Helper()
	stage, graph := testStageAndGraph(t, testGraphUUID)
	schema := stage.referencedGraphByURI("http://semanticstep.net/schema#")
	p1 := schema.CreateIRINode("p1")
	p2 := schema.CreateIRINode("p2")
	s1 := graph.CreateIRINode("s1")
	s2 := graph.CreateIRINode("s2")
	b1 := graph.CreateBlankNode()
	s1.AddStatement(p1, s2)
	s2.AddStatement(p2, b1)
	return graph
}

func testGraphWithLiterals(t *testing.T, testGraphUUID uuid.UUID, _ ...uuid.UUID) NamedGraph {
	t.Helper()
	stage, graph := testStageAndGraph(t, testGraphUUID)
	schema := stage.referencedGraphByURI("http://semanticstep.net/schema#")
	p1 := schema.CreateIRINode("p1")
	p2 := schema.CreateIRINode("p2")
	p3 := schema.CreateIRINode("p3")
	s1 := graph.CreateIRINode("s1")
	s1.AddStatement(p1, String("str1"))
	s1.AddStatement(p2, Integer(-4456789123))
	s1.AddStatement(p3, Double(1.2))
	return graph
}

func testGraphWithLiteralAndLiteralCollection(t *testing.T, testGraphUUID uuid.UUID, _ ...uuid.UUID) NamedGraph {
	t.Helper()
	stage, graph := testStageAndGraph(t, testGraphUUID)
	schema := stage.referencedGraphByURI("http://semanticstep.net/schema#")
	p1 := schema.CreateIRINode("p1")
	p2 := schema.CreateIRINode("p2")
	p3 := schema.CreateIRINode("p3")
	s1 := graph.CreateIRINode("s1")
	s2 := graph.CreateIRINode("s2")
	s1.AddStatement(p1, Integer(10))
	l := NewLiteralCollection(String("cl1"), String("cl2"))
	s1.AddStatement(p2, l)
	s1.AddStatement(p3, s2)
	return graph
}

func testGraphWithTermCollection(t *testing.T, testGraphUUID uuid.UUID, _ ...uuid.UUID) NamedGraph {
	t.Helper()
	graph, _, _ := testGraphAndCollection(t, testGraphUUID)
	return graph
}

func testGraphWithTermCollectionAndTriple(t *testing.T, testGraphUUID uuid.UUID, _ ...uuid.UUID) NamedGraph {
	t.Helper()
	graph, p1, col1 := testGraphAndCollection(t, testGraphUUID)
	col1.(*ibNode).AddStatement(p1, String("val1"))
	return graph
}

func testGraphWithTermCollectionMembers(t *testing.T, testGraphUUID uuid.UUID, _ ...uuid.UUID) NamedGraph {
	t.Helper()
	graph, _, col1 := testGraphAndCollection(t, testGraphUUID)
	assert.NotPanics(t, func() { col1.SetMembers(col1.Member(0), String("val2"), col1.Member(0)) })
	return graph
}

func testGraphWithMemberCollection(t *testing.T, testGraphUUID uuid.UUID, _ ...uuid.UUID) NamedGraph {
	t.Helper()
	graph, _, col1 := testGraphAndCollection(t, testGraphUUID)
	col2 := graph.CreateCollection(col1.Member(0))
	assert.NoError(t, col1.(*ibNode).ibNodeType().(*ibNodeUuid).setFragment(uuid.MustParse("707e31d4-3a66-4de0-b98c-0ed8a661b9da")))
	assert.NoError(t, col2.(*ibNode).ibNodeType().(*ibNodeUuid).setFragment(uuid.MustParse("aaf29eaf-25da-4b67-a942-a65970a4be8b")))
	assert.NotPanics(t, func() { col1.SetMembers(col1.Member(0), col2) })
	return graph
}

func testFullKindsNodes(t *testing.T, testGraphUUID uuid.UUID, _ ...uuid.UUID) NamedGraph {
	// @prefix lci:    <http://ontology.semanticstep.net/lci#> .
	// @prefix rdf:    <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
	// @prefix exp: <http:example.com> .
	// @prefix :   <urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f613a361#> .

	// <urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f613a361> a   owl:Ontology ;
	//          lci:integerAsLiteral 14 . # a triple for the NG-Node
	// :John   a   lci:Person .
	// :Linda  a   lci:Person .
	// :Comp1  a  lci:Organization ;
	//   exp:employees ( :John :Linda ) ;  # TermCollection
	//   exp:strings ( "st1", "st2" ) ;         # LiteralCollection
	//   lci:hasPart [     # Blank Node
	//       a  lci:Organization ;
	//       lci:integerAsLiteral 14 .
	//   ] .

	stage := OpenStage(DefaultTriplexMode)
	graph := stage.CreateNamedGraph(IRI(testGraphUUID.URN()))

	lciGraph := stage.referencedGraphByURI("http://ontology.semanticstep.net/lci#")
	lciPerson := lciGraph.CreateIRINode("Person")
	lciOrganization := lciGraph.CreateIRINode("Organization")
	lciIntergerAsLiteral := lciGraph.CreateIRINode("integerAsLiteral")
	lciHashPart := lciGraph.CreateIRINode("hasPart")

	expGraph := stage.referencedGraphByURI("http://example.com/")
	expEmployees := expGraph.CreateIRINode("employees")
	expStrings := expGraph.CreateIRINode("strings")

	ngNode := graph.GetIRINodeByFragment("")
	ngNode.AddStatement(lciIntergerAsLiteral, Integer(13))
	john := graph.CreateIRINode("John", lciPerson)
	linda := graph.CreateIRINode("Linda", lciPerson)
	bl := graph.CreateBlankNode(lciOrganization)
	bl.AddStatement(lciIntergerAsLiteral, Integer(14))

	comp1 := graph.CreateIRINode("Comp1", lciOrganization)
	col := graph.CreateCollection(john, linda)
	comp1.AddStatement(expEmployees, col)
	stringCol := NewLiteralCollection(String("st1"), String("st2"))
	comp1.AddStatement(expStrings, stringCol)
	comp1.AddStatement(lciHashPart, bl)

	return graph
}

func PrintNamedGraphInfo(info NamedGraphInfo) {
	fmt.Printf("- IRI: %s\n", info.Iri)
	fmt.Printf("- ID: %s\n", info.Id)
	fmt.Printf("- Is Referenced?: %t\n", info.IsReferenced)
	fmt.Printf("- Is Empty?: %t\n", info.IsEmpty)
	fmt.Printf("- Is Modified?: %t\n", info.IsModified)
	fmt.Printf("- Number of IRI Nodes: %d\n", info.NumberOfIRINodes)
	fmt.Printf("- Number of Blank Nodes: %d\n", info.NumberOfBlankNodes)
	fmt.Printf("- Number of Term Collections: %d\n", info.NumberOfTermCollections)
	fmt.Printf("- Number of Direct Imported Graphs: %d\n", info.NumberOfDirectImportedGraphs)
	fmt.Printf("- Number of All Imported Graphs: %d\n", info.NumberOfAllImportedGraphs)
	fmt.Printf("- Number of Subject Triples: %d\n", info.NumberOfSubjectTriples)
	fmt.Printf("- Number of Predicate Triples: %d\n", info.NumberOfPredicateTriples)
	fmt.Printf("- Number of Object Triples: %d\n", info.NumberOfObjectTriples)
	fmt.Printf("- Number of TermCollection Triples: %d\n", info.NumberOfTermCollectionTriples)

	fmt.Printf("- Commit Hash: %s\n", info.CheckedOutCommits)
	fmt.Printf("- NamedGraph Revision Hash: %s\n", info.NamedGraphRevision)
	fmt.Printf("- Dataset Revision Hash: %s\n", info.DatasetRevision)
}

func Test_SstWrite_fullKindsNodes(t *testing.T) {
	testName := filepath.Join(t.TempDir(), t.Name())
	t.Run("write", func(t *testing.T) {
		// AtomicLevel.SetLevel(zap.DebugLevel) // Enable debug logging for the test
		ng := testFullKindsNodes(t, uuid.MustParse("0c20d026-3fda-4e35-ab7d-88ba6cc0c5fc"))
		ngInfo := ng.Info()
		PrintNamedGraphInfo(ngInfo)
		_ = ngInfo // for debugging
		assert.Equal(t, 4, ngInfo.NumberOfIRINodes)
		assert.Equal(t, 2, ngInfo.NumberOfBlankNodes)
		assert.Equal(t, 1, ngInfo.NumberOfTermCollections)
		assert.Equal(t, 11, ngInfo.NumberOfSubjectTriples)
		assert.Equal(t, 4, ngInfo.NumberOfObjectTriples)
		assert.Equal(t, 2, ngInfo.NumberOfTermCollectionTriples)
		writeToFile(ng, testName)
	})
	t.Run("readSSTFile", func(t *testing.T) {
		ng := readSSTFile(testName + ".sst")
		fullKindsNodesCheck(t, ng)

		ng.Dump()
	})

	t.Run("readTTLFile", func(t *testing.T) {
		ng := readTTLFile(testName + ".ttl")
		fullKindsNodesCheck(t, ng)

		ng.Dump()
	})
}

func fullKindsNodesCheck(t *testing.T, ng NamedGraph) {
	st := ng.Stage()
	assert.Equal(t, 3, len(st.ReferencedGraphs()))
	assert.Equal(t, 1, len(st.NamedGraphs()))
	assert.Equal(t, 4, ng.IRINodeCount())
	assert.Equal(t, 2, ng.BlankNodeCount())
	lciIntergerAsLiteral := st.ReferencedGraph("http://ontology.semanticstep.net/lci").GetIRINodeByFragment("integerAsLiteral")
	assert.NotNil(t, lciIntergerAsLiteral)
	lciOrganization := st.ReferencedGraph("http://ontology.semanticstep.net/lci").GetIRINodeByFragment("Organization")
	assert.NotNil(t, lciOrganization)
	lciPerson := st.ReferencedGraph("http://ontology.semanticstep.net/lci").GetIRINodeByFragment("Person")
	assert.NotNil(t, lciPerson)
	rdfType := st.ReferencedGraph("http://www.w3.org/1999/02/22-rdf-syntax-ns").GetIRINodeByFragment("type")
	assert.NotNil(t, rdfType)

	john := ng.GetIRINodeByFragment("John")
	assert.NotNil(t, john)
	// assert.Equal(t, "Organization", john.TypeOf().Fragment())
	assert.True(t, john.CheckTriple(rdfType, lciPerson))

	linda := ng.GetIRINodeByFragment("Linda")
	assert.NotNil(t, linda)
	comp1 := ng.GetIRINodeByFragment("Comp1")
	assert.NotNil(t, comp1)

	comp1.ForAll(func(_ int, s, p IBNode, o Term) error {
		if comp1 != s {
			return nil
		}
		switch p.Fragment() {
		case "type":
			assert.Equal(t, "Organization", o.(IBNode).Fragment())
		case "employees":
			col, ok := o.(TermCollection)
			assert.True(t, ok)
			assert.Equal(t, 2, col.MemberCount())
			assert.Equal(t, john, col.Member(0))
			assert.Equal(t, linda, col.Member(1))
		case "strings":
			col, ok := o.(LiteralCollection)
			assert.True(t, ok)
			assert.Equal(t, 2, col.MemberCount())
			assert.Equal(t, String("st1"), col.Member(0))
			assert.Equal(t, String("st2"), col.Member(1))
		case "hasPart":
			bl, ok := o.(IBNode)
			assert.True(t, ok)
			// assert.Equal(t, "Organization", bl.TypeOf().Fragment())
			assert.True(t, bl.CheckTriple(rdfType, lciOrganization))
			assert.True(t, bl.CheckTriple(lciIntergerAsLiteral, Integer(14)))

		}
		return nil
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

func writeToFile(ng NamedGraph, fileName string) {
	f, err := os.Create(fileName + ".ttl")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	err = ng.RdfWrite(f, RdfFormatTurtle)
	if err != nil {
		log.Panic(err)
	}

	out, err := os.Create(fileName + ".sst")
	if err != nil {
		panic(err)
	}
	defer out.Close()

	err = ng.SstWrite(out)
	if err != nil {
		log.Panic(err)
	}
}

func readSSTFile(fileName string) (graph NamedGraph) {
	in, err := os.Open(fileName)
	if err != nil {
		log.Panic(err)
	}
	defer in.Close()
	graph, err = SstRead(bufio.NewReader(in), DefaultTriplexMode)
	if err != nil {
		log.Panic(err)
	}
	return graph
}

func readTTLFile(fileName string) (graph NamedGraph) {
	file, err := os.Open(fileName)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// convert the content of the turtle file "helloworldwrite.ttl", which is RDF, to a default NamedGraph.
	st, err := RdfRead(bufio.NewReader(file), RdfFormatTurtle, StrictHandler, DefaultTriplexMode)
	if err != nil {
		log.Panic(err)
	}
	graph = st.NamedGraphs()[0]
	return graph
}

func testGraphAndCollection(t *testing.T, testGraphUUID uuid.UUID) (NamedGraph, IBNode, TermCollection) {
	t.Helper()
	stage, graphA := testStageAndGraph(t, testGraphUUID)
	graphB := stage.referencedGraphByURI("http://semanticstep.net/schema#")
	p1 := graphB.CreateIRINode("p1")
	s1 := graphA.CreateIRINode("s1")
	s2 := graphA.CreateIRINode("s2")
	c1 := graphA.CreateCollection(s2)
	s1.AddStatement(p1, c1)
	return graphA, p1, c1
}

func testGraphWithNodesAndImport(t *testing.T, testGraphUUID uuid.UUID, testGraphUUIDs ...uuid.UUID) NamedGraph {
	t.Helper()
	graph := testGraphWithNodesNoImports(t, testGraphUUID)
	stage := graph.Stage()
	if len(testGraphUUIDs) == 1 {
		stage.CreateNamedGraph(IRI(testGraphUUIDs[0].URN()))
	}
	graph.AddImport(stage.NamedGraph(IRI(testGraphUUIDs[0].URN())))

	return graph
}

func testGraphWithNodesAndImports(t *testing.T, testGraphUUID uuid.UUID, testGraphUUIDs ...uuid.UUID) NamedGraph {
	t.Helper()
	graph := testGraphWithNodesNoImports(t, testGraphUUID)

	st := graph.Stage()
	ng2 := st.CreateNamedGraph(IRI(testGraphUUIDs[0].URN()))
	graph.AddImport(ng2)

	ng3 := st.CreateNamedGraph(IRI(testGraphUUIDs[1].URN()))
	graph.AddImport(ng3)

	return graph
}

func testGraphWithImportedGraphs(t *testing.T, testGraphUUID uuid.UUID, testGraphUUIDs ...uuid.UUID) NamedGraph {
	t.Helper()
	st, ngA := testStageAndGraph(t, testGraphUUID)
	ngB := st.CreateNamedGraph(IRI(testGraphUUIDs[0].URN()))
	ng := st.NamedGraph(IRI(testGraphUUID.URN()))
	ng.AddImport(ngB)

	ngC := st.CreateNamedGraph(IRI(testGraphUUIDs[1].URN()))
	ng.AddImport(ngC)

	schema := st.referencedGraphByURI("http://semanticstep.net/schema#")
	p1 := schema.CreateIRINode("p1")
	p2 := schema.CreateIRINode("p2")
	t1 := schema.CreateIRINode("t1")
	t2 := schema.CreateIRINode("t2")
	sA1 := ngA.CreateIRINode("sA1")
	sA1.AddStatement(p1, t1)
	sA2 := ngA.CreateIRINode("sA2")
	sA2.AddStatement(p1, t2)
	sB1 := ngB.CreateIRINode("sB1")
	sB1.AddStatement(p1, t2)
	sC1 := ngC.CreateIRINode("sC1")
	sC1.AddStatement(p1, t2)
	sA1.AddStatement(p2, sB1)
	sA1.AddStatement(p2, sC1)
	return ngA
}

func testStageAndGraph(t *testing.T, testGraphUUID uuid.UUID) (Stage, NamedGraph) {
	t.Helper()
	st := OpenStage(DefaultTriplexMode)
	ng := st.CreateNamedGraph(IRI(testGraphUUID.URN()))
	return st, ng
}
