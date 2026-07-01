// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"bufio"
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func Test_RdfRead(t *testing.T) {
	type args struct {
		reader    *bufio.Reader
		f         RdfFormat
		readError readErrorHandler
	}
	type test struct {
		name           string
		args           args
		graphAssertion assert.ValueAssertionFunc
		errorAssertion assert.ErrorAssertionFunc
	}
	t0 := func() test {
		var readErrors []error
		file, _ := os.Open("testdata/shacl-test.ttl")
		tt := test{
			args: args{
				reader: bufio.NewReader(file),
				f:      RdfFormatTurtle,
				readError: func(err error) error {
					readErrors = append(readErrors, err)
					return nil
				},
			},
			graphAssertion: func(t assert.TestingT, g interface{}, _ ...interface{}) bool {
				var readErrorText strings.Builder
				for _, err := range readErrors {
					readErrorText.WriteString(err.Error())
					readErrorText.WriteRune('\n')
				}
				// After the fix for trailing semicolons in property lists,
				// this file should parse without errors
				assert.Equal(t, "", readErrorText.String())
				return assert.NotEqual(t, nil, g)
			},
			errorAssertion: assert.NoError,
		}
		return tt
	}()
	tests := []test{
		{name: "shacl-test", args: t0.args, graphAssertion: t0.graphAssertion, errorAssertion: t0.errorAssertion},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotGraph, err := RdfRead(tt.args.reader, tt.args.f, tt.args.readError, DefaultTriplexMode)
			if tt.errorAssertion(t, err) {
				tt.graphAssertion(t, gotGraph)
			}
		})
	}
}

func Test_FromReaderToNamedGraph_TurtleFiles(t *testing.T) {
	type args struct {
		decoder    rdfReader
		parseError readErrorHandler
	}
	toArgs := func(decoder rdfReader, parseError readErrorHandler) args {
		return args{decoder: decoder, parseError: parseError}
	}
	tests := []struct {
		name           string
		args           args
		graphAssertion func(t *testing.T, graph NamedGraph)
		errorAssertion assert.ErrorAssertionFunc
	}{
		{
			name: "s-ten",
			args: toArgs(fromTurtleFile(t, "testdata/0f8b94cb-a412-4bd1-b614-eaf3c3b4c47e.ttl")),
			graphAssertion: func(t *testing.T, graph NamedGraph) {
				assert.Equal(t, uuid.MustParse("0f8b94cb-a412-4bd1-b614-eaf3c3b4c47e"), graph.ID())
				assert.Equal(t, 185, graph.IRINodeCount())
				assert.Equal(t, 97, len(graph.DirectImports()))
				// ng := graph.Stage().NamedGraph(uuid.MustParse("48375242-8cef-4f05-91d7-cea41ca37924"))
				// assert.Equal(t, 2, ng.IRINodeCount())
			},
			errorAssertion: assert.NoError,
		},
		{
			name: "lci-test1",
			args: toArgs(fromTurtleFile(t, "testdata/lci-test1.ttl")),
			graphAssertion: func(t *testing.T, graph NamedGraph) {
				expectedIRI, err := NewIRI("http://ontology.semanticstep.net/lci")
				assert.NoError(t, err)
				assert.Equal(t, expectedIRI, graph.IRI())
				assert.Equal(t, 2, graph.IRINodeCount())
			},
			errorAssertion: assert.NoError,
		},
		{
			name: "lci-test2",
			args: toArgs(fromTurtleFile(t, "testdata/lci-test2.ttl")),
			graphAssertion: func(t *testing.T, graph NamedGraph) {
				expectedIRI, err := NewIRI("http://ontology.semanticstep.net/lci")
				assert.NoError(t, err)
				assert.Equal(t, expectedIRI, graph.IRI())
				assert.Equal(t, 4, graph.IRINodeCount())
			},
			errorAssertion: assert.NoError,
		},
		{
			name: "sso-test1",
			args: toArgs(fromTurtleFile(t, "testdata/sso-test1.ttl")),
			graphAssertion: func(t *testing.T, graph NamedGraph) {
				expectedIRI, err := NewIRI("http://ontology.semanticstep.net/sso")
				assert.NoError(t, err)
				assert.Equal(t, expectedIRI, graph.IRI())
				assert.Equal(t, 2, graph.IRINodeCount())
			},
			errorAssertion: assert.NoError,
		},
		{
			name: "sso-test2",
			args: toArgs(fromTurtleFile(t, "testdata/sso-test2.ttl")),
			graphAssertion: func(t *testing.T, graph NamedGraph) {
				expectedIRI, err := NewIRI("http://ontology.semanticstep.net/sso")
				assert.NoError(t, err)
				assert.Equal(t, expectedIRI, graph.IRI())
				if assert.Equal(t, 4, graph.IRINodeCount()) {
					assert.Equal(t, "http://ontology.semanticstep.net/sso", graph.IRI().String())
					assert.Equal(t, 1, len(graph.DirectImports()))
					im := graph.DirectImports()[0]
					assert.NotNil(t, im)
					assert.Equal(t, "http://ontology.semanticstep.net/lci", im.IRI().String())
					self := graph.GetIRINodeByFragment("")
					if assert.NotNil(t, self) {
						assert.Equal(t, string(""), self.Fragment())
						var forwardCnt, inverseCnt uint
						assert.NoError(t, self.ForAll(func(_ int, subject, predicate IBNode, object Term) error {
							if subject == self {
								forwardCnt++
							} else {
								inverseCnt++
							}
							return nil
						}))
						assert.Equal(t, uint(1), forwardCnt)
						assert.Equal(t, uint(1), inverseCnt)
					}
				}
				assert.Equal(t, 1, len(graph.DirectImports()))
			},
			errorAssertion: assert.NoError,
		},
		{
			name: "owl-test",
			args: toArgs(fromTurtleFile(t, "testdata/owl-test.ttl")),
			graphAssertion: func(t *testing.T, graph NamedGraph) {
				expectedIRI, err := NewIRI("http://www.w3.org/2002/07/owl")
				assert.NoError(t, err)
				assert.Equal(t, expectedIRI, graph.IRI())
				assert.Equal(t, 5, graph.IRINodeCount())
				importedGraphs := graph.DirectImports()
				if assert.Len(t, importedGraphs, 1) {
					ig := importedGraphs[0]
					assert.NoError(t, err)
					assert.Equal(t, 3, ig.IRINodeCount())
				}
			},
			errorAssertion: assert.NoError,
		},
		{
			name: "object-collection",
			args: toArgs(fromTurtleContent(t, `@prefix : <http://content.example.org/> .
		@prefix stuff: <http://example.org/stuff/1.0/> .
		:node1 stuff:p (
			[ stuff:name "b1" ]
			[ stuff:name "b2" ]
		) .`)),
			graphAssertion: func(t *testing.T, graph NamedGraph) {
				assert.Equal(t, 1, graph.IRINodeCount())
				assert.NoError(t, graph.ForIRINodes(func(d IBNode) error {
					return d.ForAll(func(_ int, s, p IBNode, o Term) error {
						if s == d {
							assert.Equal(t, TermKindTermCollection, o.TermKind())
						}
						return nil
					})
				}))
			},
			errorAssertion: assert.NoError,
		},
		{
			name: "standalone-test-ontology",
			args: toArgs(fromTurtleContent(t, `@prefix owl:    <http://www.w3.org/2002/07/owl#> .
		@prefix :       <urn:uuid:b8374c09-1347-4bdf-94dd-aa121e4882b6#> .
		@prefix rdfs:    <http://www.w3.org/2000/01/rdf-schema#> .
		@prefix test:    <http://example.com/ontology/test#> .

		<urn:uuid:b8374c09-1347-4bdf-94dd-aa121e4882b6> a       owl:Ontology .
		:3250e7fa-ac2a-442c-88f0-5221a66e291c   rdfs:subPropertyOf       :ad57b9b5-95dd-41aa-aeab-a15cc158856d .
		:834bdca0-20ca-4c15-8c21-fa64e94a1349   a       test:t11 ,
		                        test:t2 ;
		        rdfs:subPropertyOf       test:t3 .
		:ad57b9b5-95dd-41aa-aeab-a15cc158856d   rdfs:subPropertyOf       test:t3 .
		:c38d5433-42aa-4b9e-90a4-9b1b99855d21   a       test:t11 ;
		        rdfs:subPropertyOf       test:t3 .
		:c9328f94-bccf-419f-8191-e5ef3257c2b3   rdfs:subPropertyOf       :3250e7fa-ac2a-442c-88f0-5221a66e291c .`)),
			graphAssertion: func(t *testing.T, graph NamedGraph) {
				b3 := graph.GetIRINodeByFragment("ad57b9b5-95dd-41aa-aeab-a15cc158856d")
				assert.NotNil(t, b3)
				// _, ok := b3.InVocabulary().(testVocabulary.KindT3)
				// assert.True(t, ok, "expected b3.InVocabulary().(testVocabulary.KindT3)")
			},
			errorAssertion: assert.NoError,
		},

		{
			name: "merge-subcontent",
			args: toArgs(fromTurtleContent(t, `@prefix sso:	<http://ontology.semanticstep.net/sso#> .
		@prefix ssqau:	<http://ontology.semanticstep.net/ssqau#> .
		@prefix rdfs:	<http://www.w3.org/2000/01/rdf-schema#> .
		@prefix owl:	<http://www.w3.org/2002/07/owl#> .
		@prefix :	<http://ontology.semanticstep.net/unitclassifications#> .
		@prefix ns0:	<http://ontology.semanticstep.net/> .
		@prefix ns1:	<http://ontology.semanticstep.net/unitclassifications#kg/> .

		ns0:unitclassifications	a	owl:Ontology ;
			owl:versionInfo	"0.01" .
		:\~	a	sso:ClassificationSystem ;
			rdfs:label	"Quantity & unit classification system" .
		LinearDensity	a	:\~ ,
					owl:Class ;
			owl:sameAs	ssqau:LinearDensity ;
			rdfs:label	"LinearDensity" .
		ns1:km	a	:\~ ,
					owl:Class ;
			rdfs:subClassOf	:LinearDensity ;
			rdfs:label	"kg/km" .`)),
			graphAssertion: func(t *testing.T, graph NamedGraph) {
				ib := graph.GetIRINodeByFragment("kg/km")
				assert.NotNil(t, ib)
			},
			errorAssertion: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.args.decoder == nil {
				t.Skip()
			}
			st, err := fromReaderToStage(tt.args.decoder, tt.args.parseError)
			if tt.errorAssertion(t, err) {
				tt.graphAssertion(t, st.NamedGraphs()[0])
			}
		})
	}
}

// TestPropertyListWithTrailingSemicolon tests the case where a property list
// has a trailing semicolon before the closing bracket, followed by a dot.
// This is the pattern used in SHACL definitions like:
//
//	:CartesianPoint
//	    sh:property [
//	        sh:path :coordinates ;
//	        sh:minCount 1 ;
//	        sh:maxCount 1 ;
//	    ] .
func TestPropertyListWithTrailingSemicolon(t *testing.T) {
	// This is the problematic case from the issue - semicolon before closing bracket
	ttl := `@prefix : <http://example.org/> .
@prefix sh: <http://www.w3.org/ns/shacl#> .

:CartesianPoint
    a sh:NodeShape ;
    sh:property [
        sh:path :coordinates ;
        sh:minCount 1 ;
        sh:maxCount 1 ;
    ] .`

	dec := newTripleReader(bytes.NewBufferString(ttl), RdfFormatTurtle)
	triples, err := dec.readAll()
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// We should have 4 triples:
	// 1. :CartesianPoint a sh:NodeShape
	// 2. :CartesianPoint sh:property _:b1
	// 3. _:b1 sh:path :coordinates
	// 4. _:b1 sh:minCount 1
	// 5. _:b1 sh:maxCount 1
	if len(triples) != 5 {
		t.Fatalf("Expected 5 triples, got %d: %v", len(triples), triples)
	}
}

// TestPropertyListWithTrailingSemicolonOnSameLine tests when the closing
// bracket is on the same line as the semicolon
func TestPropertyListWithTrailingSemicolonOnSameLine(t *testing.T) {
	// Semicolon and closing bracket on same line
	ttl := `@prefix : <http://example.org/> .
@prefix sh: <http://www.w3.org/ns/shacl#> .

:CartesianPoint sh:property [ sh:path :coordinates ; sh:maxCount 1 ; ] .`

	dec := newTripleReader(bytes.NewBufferString(ttl), RdfFormatTurtle)
	triples, err := dec.readAll()
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// We should have 3 triples
	if len(triples) != 3 {
		t.Fatalf("Expected 3 triples, got %d: %v", len(triples), triples)
	}
}

// TestPropertyListWithoutTrailingSemicolon tests the working case
// (without trailing semicolon before closing bracket)
func TestPropertyListWithoutTrailingSemicolon(t *testing.T) {
	// This should work - no semicolon before closing bracket
	ttl := `@prefix : <http://example.org/> .
@prefix sh: <http://www.w3.org/ns/shacl#> .

:CartesianPoint
    a sh:NodeShape ;
    sh:property [
        sh:path :coordinates ;
        sh:minCount 1 ;
        sh:maxCount 1 ] .`

	dec := newTripleReader(bytes.NewBufferString(ttl), RdfFormatTurtle)
	triples, err := dec.readAll()
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// We should have 5 triples
	if len(triples) != 5 {
		t.Fatalf("Expected 5 triples, got %d: %v", len(triples), triples)
	}
}

// TestNestedPropertyList tests nested property lists
func TestNestedPropertyList(t *testing.T) {
	// Nested property lists
	ttl := `@prefix : <http://example.org/> .
@prefix sh: <http://www.w3.org/ns/shacl#> .

:Shape
    sh:property [
        sh:path :prop ;
        sh:property [
            sh:path :nested ;
        ] ;
    ] .`

	dec := newTripleReader(bytes.NewBufferString(ttl), RdfFormatTurtle)
	triples, err := dec.readAll()
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// We should have 4 triples:
	// 1. :Shape sh:property _:b1
	// 2. _:b1 sh:path :prop
	// 3. _:b1 sh:property _:b2
	// 4. _:b2 sh:path :nested
	if len(triples) != 4 {
		t.Fatalf("Expected 4 triples, got %d: %v", len(triples), triples)
	}
}

// TestPropertyListAsSubjectWithTrailingSemicolon tests property list as subject with trailing semicolon
func TestPropertyListAsSubjectWithTrailingSemicolon(t *testing.T) {
	// Property list as subject with trailing semicolon
	ttl := `@prefix : <http://example.org/> .
@prefix foaf: <http://xmlns.com/foaf/0.1#> .

[ foaf:name "Alice" ; ] foaf:knows [ foaf:name "Bob" ; ] .`

	dec := newTripleReader(bytes.NewBufferString(ttl), RdfFormatTurtle)
	triples, err := dec.readAll()
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// We should have 3 triples:
	// 1. _:b1 foaf:name "Alice"
	// 2. _:b1 foaf:knows _:b2
	// 3. _:b2 foaf:name "Bob"
	if len(triples) != 3 {
		t.Fatalf("Expected 3 triples, got %d: %v", len(triples), triples)
	}
}

// TestMultipleTrailingSemicolons tests multiple trailing semicolons
func TestMultipleTrailingSemicolons(t *testing.T) {
	// Multiple semicolons in a row (empty predicate/object pairs)
	ttl := `@prefix : <http://example.org/> .
@prefix sh: <http://www.w3.org/ns/shacl#> .

:CartesianPoint sh:property [
    sh:path :coordinates ;
    sh:minCount 1 ;
    ;
    ;
] .`

	dec := newTripleReader(bytes.NewBufferString(ttl), RdfFormatTurtle)
	triples, err := dec.readAll()
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// We should have 3 triples (multiple semicolons should be skipped)
	if len(triples) != 3 {
		t.Fatalf("Expected 3 triples, got %d: %v", len(triples), triples)
	}
}

// TestTildeAsNodeFragment tests using '~' as node fragment in prefixed names
func TestTildeAsNodeFragment(t *testing.T) {
	// Test using '~' in prefixed name as node fragment
	ttl := `@prefix ex: <http://example.org/> .
ex:test\~node ex:predicate ex:object .`

	dec := newTripleReader(bytes.NewBufferString(ttl), RdfFormatTurtle)
	triples, err := dec.readAll()
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// We should have 1 triple
	if len(triples) != 1 {
		t.Fatalf("Expected 1 triple, got %d: %v", len(triples), triples)
	}

	// Verify the subject has the tilde in the IRI
	subj, ok := triples[0].Subj.(IRI)
	if !ok {
		t.Fatalf("Expected IRI subject, got %T", triples[0].Subj)
	}
	expected := "http://example.org/test~node"
	if string(subj) != expected {
		t.Fatalf("Expected subject %q, got %q", expected, subj)
	}
}

// TestTildeInLocalName tests using escaped '~' (\~) in various positions in local names
func TestTildeInLocalName(t *testing.T) {
	// Test various uses of escaped '~' in local names
	// Note: ~ must be escaped as \~ in Turtle local names
	ttl := `@prefix ex: <http://example.org/> .
ex:node\~1 ex:prop\~erty ex:value\~test .
<http://example.org/iri~test> ex:predicate <http://example.org/other~iri> .`

	dec := newTripleReader(bytes.NewBufferString(ttl), RdfFormatTurtle)
	triples, err := dec.readAll()
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// We should have 2 triples
	if len(triples) != 2 {
		t.Fatalf("Expected 2 triples, got %d: %v", len(triples), triples)
	}

	// Verify first triple subject has tilde (unescaped in IRI)
	subj1, ok := triples[0].Subj.(IRI)
	if !ok {
		t.Fatalf("Expected IRI subject, got %T", triples[0].Subj)
	}
	if string(subj1) != "http://example.org/node~1" {
		t.Fatalf("Expected subject 'http://example.org/node~1', got %q", subj1)
	}

	// Verify first triple predicate has tilde (unescaped in IRI)
	pred1, ok := triples[0].Pred.(IRI)
	if !ok {
		t.Fatalf("Expected IRI predicate, got %T", triples[0].Pred)
	}
	if string(pred1) != "http://example.org/prop~erty" {
		t.Fatalf("Expected predicate 'http://example.org/prop~erty', got %q", pred1)
	}

	// Verify first triple object has tilde (unescaped in IRI)
	obj1, ok := triples[0].Obj.(IRI)
	if !ok {
		t.Fatalf("Expected IRI object, got %T", triples[0].Obj)
	}
	if string(obj1) != "http://example.org/value~test" {
		t.Fatalf("Expected object 'http://example.org/value~test', got %q", obj1)
	}
}
