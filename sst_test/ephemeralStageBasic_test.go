// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

// The reason why there is a separate package is about RegisterVocabularyMap.
// In sst_test_dummyvoc package, it imports package compiler, which will register a test VocabularyMap.
// In sst_test_realvoc package, it imports package vocabularies, which will register a real
// VocabularyMap that includes lci, owl...

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/vocabularies/countrycodes"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/eed"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/owl"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/semanticstep/sst-core/vocabularies/sso"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func Test_NamedGraph_IBNode_Is(t *testing.T) {
	t.Run("create_masterPort", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361").URN()))
		mp := ng.CreateIRINode("mp", eed.MasterPort)

		assert.False(t, mp.Is(eed.MasterPort))
		fmt.Println("mp.Is(eed.MasterPort) = ", mp.Is(eed.MasterPort))

		assert.False(t, mp.IsKind(eed.MasterPort))
		fmt.Println("mp.IsKind(eed.MasterPort) = ", mp.IsKind(eed.MasterPort))

		assert.True(t, mp.TypeOf().Is(eed.MasterPort))
		fmt.Println("mp.TypeOf().Is(eed.MasterPort) = ", mp.TypeOf().Is(eed.MasterPort))

		assert.True(t, mp.TypeOf().IsKind(eed.MasterPort))
		fmt.Println("mp.TypeOf().IsKind(eed.MasterPort) = ", mp.TypeOf().IsKind(eed.MasterPort))

		assert.False(t, mp.GetObjects(rdf.Type)[0].(sst.IBNode).Is(lci.Individual))
		fmt.Println("mp.GetObjects(rdf.Type)[0].(sst.IBNode).Is(lci.Individual) = ", mp.GetObjects(rdf.Type)[0].(sst.IBNode).Is(lci.Individual))

		assert.True(t, mp.GetObjects(rdf.Type)[0].(sst.IBNode).IsKind(lci.Individual))
		fmt.Println("mp.GetObjects(rdf.Type)[0].(sst.IBNode).IsKind(lci.Individual) = ", mp.GetObjects(rdf.Type)[0].(sst.IBNode).IsKind(lci.Individual))
	})
}

func testFullKindsNodes(t *testing.T, testGraphUUID uuid.UUID, _ ...uuid.UUID) sst.NamedGraph {
	// @prefix lci:    <http://ontology.semanticstep.net/lci#> .
	// @prefix rdf:    <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
	// @prefix exp: <http:example.com#> .
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

	st := sst.OpenStage(sst.DefaultTriplexMode)
	graph := st.CreateNamedGraph(sst.IRI(testGraphUUID.URN()))
	exp := st.CreateNamedGraph("http:example.com")
	expEmployees := exp.CreateIRINode("employees")
	expStrings := exp.CreateIRINode("strings")

	ngNode := graph.GetIRINodeByFragment("")
	ngNode.AddStatement(lci.IntegerAsLiteral, sst.Integer(13))
	john := graph.CreateIRINode("John", lci.Person)
	linda := graph.CreateIRINode("Linda", lci.Person)
	bl := graph.CreateBlankNode(lci.Organization)
	bl.AddStatement(lci.IntegerAsLiteral, sst.Integer(14))

	comp1 := graph.CreateIRINode("Comp1", lci.Organization)
	col := graph.CreateCollection(john, linda)
	comp1.AddStatement(expEmployees, col)
	stringCol := sst.NewLiteralCollection(sst.String("st1"), sst.String("st2"))
	comp1.AddStatement(expStrings, stringCol)
	comp1.AddStatement(lci.HasPart, bl)

	return graph
}

func Test_SstWrite_fullKindsNodes(t *testing.T) {
	testName := filepath.Join(t.TempDir(), t.Name())
	defer func() {
		removeFolder(testName + ".sst")
		removeFolder(testName + ".ttl")
	}()
	t.Run("write", func(t *testing.T) {
		// AtomicLevel.SetLevel(zap.DebugLevel) // Enable debug logging for the test
		ng := testFullKindsNodes(t, uuid.MustParse("0c20d026-3fda-4e35-ab7d-88ba6cc0c5fc"))
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

func fullKindsNodesCheck(t *testing.T, ng sst.NamedGraph) {
	st := ng.Stage()
	assert.Equal(t, 3, len(st.ReferencedGraphs()))
	assert.Equal(t, 1, len(st.NamedGraphs()))
	assert.Equal(t, 4, ng.IRINodeCount())
	assert.Equal(t, 2, ng.BlankNodeCount())
	lciIntegerAsLiteral := st.ReferencedGraph("http://ontology.semanticstep.net/lci").GetIRINodeByFragment("integerAsLiteral")
	assert.NotNil(t, lciIntegerAsLiteral)
	lciOrganization := st.ReferencedGraph("http://ontology.semanticstep.net/lci").GetIRINodeByFragment("Organization")
	assert.NotNil(t, lciOrganization)
	lciPerson := st.ReferencedGraph("http://ontology.semanticstep.net/lci").GetIRINodeByFragment("Person")
	assert.NotNil(t, lciPerson)
	rdfType := st.ReferencedGraph("http://www.w3.org/1999/02/22-rdf-syntax-ns").GetIRINodeByFragment("type")
	assert.NotNil(t, rdfType)

	john := ng.GetIRINodeByFragment("John")
	assert.NotNil(t, john)
	assert.Equal(t, "Person", john.TypeOf().Fragment())
	assert.True(t, john.CheckTriple(rdfType, lciPerson))

	linda := ng.GetIRINodeByFragment("Linda")
	assert.NotNil(t, linda)
	comp1 := ng.GetIRINodeByFragment("Comp1")
	assert.NotNil(t, comp1)

	comp1.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if comp1 != s {
			return nil
		}
		switch p.Fragment() {
		case "type":
			assert.Equal(t, "Organization", o.(sst.IBNode).Fragment())
		case "employees":
			col, ok := o.(sst.TermCollection)
			assert.True(t, ok)
			assert.Equal(t, 2, col.MemberCount())
			assert.Equal(t, john, col.Member(0))
			assert.Equal(t, linda, col.Member(1))
		case "strings":
			col, ok := o.(sst.LiteralCollection)
			assert.True(t, ok)
			assert.Equal(t, 2, col.MemberCount())
			assert.Equal(t, sst.String("st1"), col.Member(0))
			assert.Equal(t, sst.String("st2"), col.Member(1))
		case "hasPart":
			bl, ok := o.(sst.IBNode)
			assert.True(t, ok)
			assert.Equal(t, "Organization", bl.TypeOf().Fragment())
			assert.True(t, bl.CheckTriple(rdfType, lciOrganization))
			assert.True(t, bl.CheckTriple(lciIntegerAsLiteral, sst.Integer(14)))

		}
		return nil
	})
}

func TestNamedGraph_CreateIndividual(t *testing.T) {
	t.Run("create_individual", func(t *testing.T) {
		type fields struct {
			namedGraph sst.NamedGraph
		}
		type args struct {
			types []sst.Term
		}
		newTemporalGraph := func() sst.NamedGraph {
			st := sst.OpenStage(sst.DefaultTriplexMode)
			id, _ := uuid.NewRandom()
			graph := st.CreateNamedGraph(sst.IRI(id.URN()))

			return graph
		}()
		tests := []struct {
			name      string
			fields    fields
			args      args
			wantD     func(t assert.TestingT, d sst.IBNode)
			assertion assert.ErrorAssertionFunc
		}{
			{
				name: "singe individual",
				fields: fields{
					namedGraph: newTemporalGraph,
				},
				args: args{
					types: []sst.Term{lci.ArrangedIndividual},
				},
				wantD: func(t assert.TestingT, d sst.IBNode) {
					var typ sst.IBNode
					assert.NoError(t, d.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
						if d == s && p.Is(rdf.Type) && sst.IsKindIBNode(o) {
							typ = o.(sst.IBNode)
						}
						return nil
					}))
					v1 := lci.ArrangedIndividual.VocabularyElement()
					v2 := typ.InVocabulary().VocabularyElement()
					assert.Equal(t, v1, v2)
				},
				assertion: assert.NoError,
			},
			{
				name: "several type individual",
				fields: fields{
					namedGraph: newTemporalGraph,
				},
				args: args{
					types: []sst.Term{lci.ArrangedIndividual, owl.Thing},
				},
				wantD: func(t assert.TestingT, d sst.IBNode) {
					var types []sst.IBNode
					assert.NoError(t, d.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
						if d == s && p.Is(rdf.Type) && sst.IsKindIBNode(o) {
							types = append(types, o.(sst.IBNode))
						}
						return nil
					}))
					var typesElements []sst.Element
					for _, typ := range types {
						typesElements = append(typesElements, typ.InVocabulary().VocabularyElement())
					}
					assert.ElementsMatch(t, []sst.Element{
						owl.Thing.Element,
						lci.ArrangedIndividual.Element,
					}, typesElements)
				},
				assertion: assert.NoError,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				gotD := tt.fields.namedGraph.CreateIRINode("", tt.args.types...)
				tt.wantD(t, gotD)
			})
		}
	})
}

func Test_CreateEphemeralStage_IsKind(t *testing.T) {
	testName := filepath.Join(t.TempDir(), t.Name())
	t.Run("write", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)

		ng := st.CreateNamedGraph(sst.IRI(uuid.MustParse("17944393-d757-462c-955f-d86ddb55d5d2").URN()))

		ib := ng.CreateIRINode("5200eed8-1438-45dc-87b6-dfedeced2c2a")
		ib2 := ng.CreateIRINode("5200eed8-1438-45dc-87b6-dfedeced2c2b")
		ib.AddStatement(owl.Members, ib2)

		rdfNil, err := st.IBNodeByVocabulary(rdf.Nil)
		if err != nil {
			panic(err)
		}
		assert.False(t, rdfNil.IsKind(rdf.List))

		// test for IsKind from sstjson\jsonSstRead.go
		vocab := owl.Members.Element
		vocabNode, err := sst.StaticDictionary().Element(vocab)
		if err != nil {
			panic(err)
		}
		objectType := vocabNode.InVocabulary().Range()
		if objectType != nil {
			rangeNode, err := sst.StaticDictionary().Element(objectType.VocabularyElement())
			if err != nil {
				log.Printf("Error: vocabulary node of %s is not found in the dictionary: %s", objectType.IRI().String(), err)
			}
			assert.True(t, rangeNode.IsKind(rdf.List))
		}

		writeToFile(ng, testName)
	})
	t.Run("read sst file", func(t *testing.T) {
		ng := readSSTFile(testName + ".sst")
		ng.Dump()
	})

}

func Test_CreateEphemeralStage_IBNodeFromObject(t *testing.T) {
	st := sst.OpenStage(sst.DefaultTriplexMode)

	ng := st.CreateNamedGraph(sst.IRI(uuid.MustParse("17944393-d757-462c-955f-d86ddb55d5d2").URN()))

	ib := ng.CreateIRINode("5200eed8-1438-45dc-87b6-dfedeced2c2a")
	ib2 := ng.CreateIRINode("5200eed8-1438-45dc-87b6-dfedeced2c2b")
	ib.AddStatement(owl.Members, ib2)
	ib.ForAll(func(index int, s, p sst.IBNode, o sst.Term) error {
		if ib != s {
			return nil
		}
		if o.TermKind() == sst.TermKindIBNode {
			assert.Equal(t, "urn:uuid:17944393-d757-462c-955f-d86ddb55d5d2#5200eed8-1438-45dc-87b6-dfedeced2c2b", o.(sst.IBNode).IRI().String())
		}
		return nil
	})
	ib3 := ng.CreateIRINode("5200eed8-1438-45dc-87b6-dfedeced2c2c", lci.Person)

	col := ng.CreateCollection(ib2, ib3)
	ib.AddStatement(rdfs.Label, col)
	ib.ForAll(func(index int, s, p sst.IBNode, o sst.Term) error {
		if ib != s {
			return nil
		}
		if index == 1 {
			assert.Panics(t, func() {
				fmt.Println(o.(sst.IBNode).IRI())
			})
		}
		return nil
	})
}

func Test_CreateEphemeralStage_uuidIRINode(t *testing.T) {
	testName := filepath.Join(t.TempDir(), t.Name())
	t.Run("write", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)

		ng := st.CreateNamedGraph(sst.IRI(uuid.MustParse("17944393-d757-462c-955f-d86ddb55d5d2").URN()))

		ib := ng.CreateIRINode("5200eed8-1438-45dc-87b6-dfedeced2c2a")
		ib.AddStatement(rdf.Type, lci.Person)

		ib2 := ng.CreateIRINode("5200eed8-1438-45dc-87b6-dfedeced2c2b", lci.Person)
		ib3 := ng.CreateIRINode("5200eed8-1438-45dc-87b6-dfedeced2c2c", lci.Person)

		col := ng.CreateCollection(ib2, ib3)
		ib.AddStatement(rdfs.Label, col)

		writeToFile(ng, testName)
	})
	t.Run("read sst file", func(t *testing.T) {
		ng := readSSTFile(testName + ".sst")
		ng.Dump()
	})

}

func Test_CreateEphemeralStage_IBNodeTTL(t *testing.T) {
	testName := filepath.Join(t.TempDir(), t.Name())
	t.Run("write", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)

		ngID := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361")
		ngID2 := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362")
		ngID3 := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363")

		// create a NamedGraph and get it
		ng := st.CreateNamedGraph(sst.IRI(ngID.URN()))
		ng2 := st.CreateNamedGraph(sst.IRI(ngID2.URN()))
		ng3 := st.CreateNamedGraph(sst.IRI(ngID3.URN()))

		ngNode := ng.GetIRINodeByFragment("")
		ngNode.AddStatement(rdfs.Label, sst.String("test"))

		mainA := ng.CreateIRINode("mainA")
		mainB := ng2.CreateIRINode("mainB")
		mainC := ng3.CreateIRINode("mainC")
		mainA.AddStatement(rdfs.Label, mainB)
		mainA.AddStatement(rdfs.Comment, mainC)

		// normal IRI
		mainA.AddStatement(rdf.Type, rep.SchematicPort)
		mainA.AddStatement(rdf.Type, lci.Person)

		// Literal
		mainA.AddStatement(rdfs.Label, sst.String("mainA"))
		mainA.AddStatement(rdfs.Label, sst.LangString{Val: "Hello", LangTag: "en"})
		mainA.AddStatement(rdfs.Label, sst.Double(3.14))
		mainA.AddStatement(rdfs.Label, sst.Integer(42))
		mainA.AddStatement(rdfs.Label, sst.Boolean(true))

		// Literal Collection
		mainA.AddStatement(rdfs.Label, sst.NewLiteralCollection(sst.String("ABC"), sst.String("DEF")))
		mainA.AddStatement(rdfs.Label, sst.NewLiteralCollection(sst.LangString{Val: "Hello", LangTag: "en"}, sst.LangString{Val: "你好", LangTag: "cn"}))
		mainA.AddStatement(rdfs.Label, sst.NewLiteralCollection(sst.Double(3.14), sst.Double(3.15)))
		mainA.AddStatement(rdfs.Label, sst.NewLiteralCollection(sst.Integer(42), sst.Integer(43)))
		mainA.AddStatement(rdfs.Label, sst.NewLiteralCollection(sst.Boolean(true), sst.Boolean(false)))

		// Blank Node
		bl := ng.CreateBlankNode(lci.Person)
		bl.AddStatement(rdfs.Label, sst.String("Mike"))

		// bl2 := ng.CreateBlankNode(lci.Person)
		// bl2.AddStatement(rdfs.Label, sst.String("Linda"))
		// bl2.AddStatement(rdfs.Comment, bl)

		mainA.AddStatement(rdfs.Comment, bl)

		// Term Collection
		n1 := ng.CreateIRINode("node1")
		n2 := ng.CreateIRINode("node2")
		col := ng.CreateCollection(n1, n2)
		mainA.AddStatement(rdfs.Comment, col)

		ng.ForAllIBNodes(func(ib sst.IBNode) error {
			for _, val := range ib.DumpTriples() {
				fmt.Println(val[0], val[1], val[2], " .")
			}
			return nil
		})
		// fmt.Println("mainA:", mainA.TTL())
		// for _, val := range mainA.TTL() {
		// 	fmt.Println(val[0], val[1], val[2], " .")
		// }

		f, err := os.Create(testName + ".ttl")
		if err != nil {
			panic(err)
		}
		defer f.Close()

		err = ng.RdfWrite(f, sst.RdfFormatTurtle)

		if err != nil {
			log.Panic(err)
		}
	})

}

func Test_CreateEphemeralStage_TTL(t *testing.T) {
	testName := filepath.Join(t.TempDir(), t.Name())
	t.Run("write", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)

		ngID := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361")
		ngID2 := uuid.MustParse("8897840e-1616-421c-a2d6-b21058e88eb5")
		ngID3 := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363")

		// create a NamedGraph and get it
		ng := st.CreateNamedGraph(sst.IRI(ngID.URN()))
		ng2 := st.CreateNamedGraph(sst.IRI(ngID2.URN()))
		ng3 := st.CreateNamedGraph(sst.IRI(ngID3.URN()))

		ngNode := ng.GetIRINodeByFragment("")
		ngNode.AddStatement(rdfs.Label, sst.String("test"))

		mainA := ng.CreateIRINode("01b28ca2-13f8-414d-8cf1-62c6971442ab ", sso.Part)

		mainB := ng2.CreateIRINode("3aa5d460-69cf-4c61-8d99-7133e73f5826", lci.Organization)
		mainB.AddStatement(rdfs.Label, sst.String("DCT"))
		dpB := ng2.CreateIRINode("598efd1f-0906-4d09-b5eb-cdbd794fa831", owl.DatatypeProperty)

		mainC := ng3.CreateIRINode("1db4100c-335c-4f02-a65e-a40b9f2e0053", lci.Organization)
		mainC.AddStatement(rdfs.Label, sst.String("LK SOFT"))
		dpC := ng3.CreateIRINode("8c46e8a9-a278-4d42-b3bc-d10c6b502453", owl.DatatypeProperty)

		mainA.AddStatement(dpB, sst.String("xyz5678"))
		mainA.AddStatement(dpC, sst.String("abc1234"))

		f, err := os.Create(testName + ".ttl")
		if err != nil {
			panic(err)
		}
		defer f.Close()

		err = ng.RdfWrite(f, sst.RdfFormatTurtle)

		if err != nil {
			log.Panic(err)
		}
	})

}

func Test_CreateEphemeralStage_UUIDNamedGraph(t *testing.T) {
	testName := filepath.Join(t.TempDir(), t.Name())
	t.Run("write", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)

		ngID := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361")

		// create a NamedGraph and get it
		ng := st.CreateNamedGraph(sst.IRI(ngID.URN()))

		mainA := ng.CreateIRINode("mainA")

		mainA.AddStatement(rdf.Type, rep.SchematicPort)

		_, _, err := st.Commit(context.TODO(), "commit message", sst.DefaultBranch)
		assert.ErrorIs(t, err, sst.ErrStageNotLinkToRepository)

		// ng.Dump()
		writeToFile(ng, testName)
	})
	t.Run("read", func(t *testing.T) {
		generatedFiles := []string{
			testName,
		}

		compareFiles(t, generatedFiles)
	})

}

func Test_CreateEphemeralStage_NodeWithoutTriples(t *testing.T) {
	testName := filepath.Join(t.TempDir(), t.Name())
	ngID := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a256")

	defer func() {
		removeFolder(testName + ".sst")
		removeFolder(testName + ".ttl")
	}()
	t.Run("write", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)

		// create a NamedGraph and get it
		ng := st.CreateNamedGraph(sst.IRI(ngID.URN()))

		uuid1Node := ng.CreateIRINode("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361")
		uuid2Node := ng.CreateIRINode("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362")
		ng.CreateIRINode("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363")

		uuid1Node.AddStatement(rdfs.Domain, uuid2Node)

		ng.Dump()
		writeToFile(ng, testName)

		fmt.Println("----------------------------------------")
	})

	t.Run("read", func(t *testing.T) {
		in, err := os.Open(testName + ".sst")
		if err != nil {
			log.Panic(err)
		}
		defer in.Close()

		graph, err := sst.SstRead(bufio.NewReader(in), sst.DefaultTriplexMode)
		if err != nil {
			log.Panic(err)
		}

		graph.Dump()

		graph.ForUndefinedIBNodes(func(i sst.IBNode) error {
			fmt.Println(i.Fragment())
			return nil
		})
	})
}

func Test_CreateEphemeralStage_IRINamedGraph(t *testing.T) {
	testName := filepath.Join(t.TempDir(), t.Name())
	t.Run("write", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)

		testIri := "http://ontology.semanticstep.net/abc#"
		// create a NamedGraph and get it
		ng := st.CreateNamedGraph(sst.IRI(testIri))

		mainA := ng.CreateIRINode("mainA")

		mainA.AddStatement(rdf.Type, rep.SchematicPort)

		// ng.Dump()
		writeToFile(ng, testName)
	})

	t.Run("read", func(t *testing.T) {
		generatedFiles := []string{
			testName,
		}
		compareFiles(t, generatedFiles)
	})


}

func Test_CreateEphemeralStage_NGAImportNGB(t *testing.T) {
	testName := filepath.Join(t.TempDir(), t.Name())
	t.Run("write", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)

		ngIDA := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361")
		ngIDB := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362")

		// create a NamedGraph and get it
		ngA := st.CreateNamedGraph(sst.IRI(ngIDA.URN()))

		ngB := st.CreateNamedGraph(sst.IRI(ngIDB.URN()))

		ngA.AddImport(ngB)

		mainB := ngB.CreateIRINode("mainB")
		mainB.AddStatement(rdf.Type, rep.SchematicPort)

		mainA := ngA.CreateIRINode("mainA")
		mainA.AddStatement(rep.MappingSource, mainB)

		writeToFile(ngB, testName+"NGB")
		writeToFile(ngA, testName+"NGA")
	})

	t.Run("read", func(t *testing.T) {
		generatedFiles := []string{
			testName + "NGB",
			testName + "NGA",
		}

		compareFiles(t, generatedFiles)
	})

}

func Test_CreateEphemeralStage_DimportsAimportsB(t *testing.T) {
	testName := filepath.Join(t.TempDir(), t.Name())
	t.Run("write", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)

		ngIDA := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361")
		ngIDB := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362")
		ngIDD := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a364")

		ngA := st.CreateNamedGraph(sst.IRI(ngIDA.URN()))

		ngB := st.CreateNamedGraph(sst.IRI(ngIDB.URN()))

		ngD := st.CreateNamedGraph(sst.IRI(ngIDD.URN()))

		// Create mainB node
		mainB := ngB.CreateIRINode("mainB")
		mainB.AddStatement(rdf.Type, rep.SchematicPort)

		// NG-A imports NG-B
		// Create mainA node references mainB
		ngA.AddImport(ngB)

		mainA := ngA.CreateIRINode("mainA")
		mainA.AddStatement(rep.MappingSource, mainB)

		// NG-D imports NG-A
		// creates mainD node references mainA
		ngD.AddImport(ngA)

		mainD := ngD.CreateIRINode("mainD")
		mainD.AddStatement(rep.MappingSource, mainA)

		writeToFile(ngB, testName+"NGB")
		writeToFile(ngA, testName+"NGA")
		writeToFile(ngD, testName+"NGD")
	})
	t.Run("read", func(t *testing.T) {
		generatedFiles := []string{
			testName + "NGB",
			testName + "NGA",
			testName + "NGD",
		}

		compareFiles(t, generatedFiles)
	})

}

func Test_CreateEphemeralStage_DiamondCase(t *testing.T) {
	testName := filepath.Join(t.TempDir(), t.Name())
	t.Run("write", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)

		ngIDA := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361")
		ngIDB := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362")
		ngIDC := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363")
		ngIDD := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a364")

		ngA := st.CreateNamedGraph(sst.IRI(ngIDA.URN()))

		ngB := st.CreateNamedGraph(sst.IRI(ngIDB.URN()))

		ngC := st.CreateNamedGraph(sst.IRI(ngIDC.URN()))

		ngD := st.CreateNamedGraph(sst.IRI(ngIDD.URN()))

		// Create mainC node
		mainC := ngC.CreateIRINode("mainC")
		mainC.AddStatement(rdf.Type, rep.SchematicPort)

		// NG-B imports NG-C
		// Create mainB node references mainC
		ngB.AddImport(ngC)

		mainB := ngB.CreateIRINode("mainB")
		mainB.AddStatement(rep.MappingSource, mainC)

		// NG-D imports NG-C
		// creates mainD node references mainC
		ngD.AddImport(ngC)

		mainD := ngD.CreateIRINode("mainD")
		mainD.AddStatement(rep.MappingSource, mainC)

		// NG-A imports NG-B
		// creates mainA node references mainB
		ngA.AddImport(ngB)

		mainA := ngA.CreateIRINode("mainA")
		mainA.AddStatement(rep.MappingSource, mainB)

		// NG-A imports NG-D
		// creates mainD node references mainC
		ngA.AddImport(ngD)

		mainA.AddStatement(rep.MappingSource, mainD)

		writeToFile(ngB, testName+"NGB")
		writeToFile(ngA, testName+"NGA")
		writeToFile(ngC, testName+"NGC")
		writeToFile(ngD, testName+"NGD")
	})

	t.Run("read", func(t *testing.T) {
		generatedFiles := []string{
			testName + "NGB",
			testName + "NGA",
			testName + "NGC",
			testName + "NGD",
		}
		compareFiles(t, generatedFiles)
	})

}

func writeToFile(ng sst.NamedGraph, fileName string) {
	f, err := os.Create(fileName + ".ttl")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	err = ng.RdfWrite(f, sst.RdfFormatTurtle)
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

func readSSTFile(fileName string) (graph sst.NamedGraph) {
	in, err := os.Open(fileName)
	if err != nil {
		log.Panic(err)
	}
	defer in.Close()
	graph, err = sst.SstRead(bufio.NewReader(in), sst.DefaultTriplexMode)
	if err != nil {
		log.Panic(err)
	}
	return graph
}

func readTTLFile(fileName string) (graph sst.NamedGraph) {
	file, err := os.Open(fileName)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// convert the content of the turtle file "helloworldwrite.ttl", which is RDF, to a default NamedGraph.
	st, err := sst.RdfRead(bufio.NewReader(file), sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
	if err != nil {
		log.Panic(err)
	}
	graph = st.NamedGraphs()[0]
	return graph
}

func compare(t *testing.T, standardFile string, generatedFile string) bool {
	standardFileP, err := os.Open(standardFile)
	if err != nil {
		t.Fatalf("can not open: %v", err)

	}
	defer standardFileP.Close()
	var standardContent string
	// use bufio.Scanner to read file to handle "\n" and "\r\n" problem
	scanner := bufio.NewScanner(standardFileP)
	for scanner.Scan() {
		line := scanner.Text()
		standardContent += line
	}

	generatedFileP, err := os.Open(generatedFile)
	if err != nil {
		t.Fatalf("can not open: %v", err)

	}
	defer generatedFileP.Close()
	var generatedContent string
	// use bufio.Scanner to read file to handle "\n" and "\r\n" problem
	scanner2 := bufio.NewScanner(generatedFileP)
	for scanner2.Scan() {
		line := scanner2.Text()
		generatedContent += line
	}

	return assert.Equal(t, standardContent, generatedContent)
}

func compareFiles(t *testing.T, generatedFiles []string) {
	for _, v := range generatedFiles {
		// get standard file from committed testdata
		standardFilePath := filepath.Join("./testdata", filepath.Base(v)+"_s")

		// equal := compare(t, standardFilePath+".ttl", v+".ttl")
		// assert.Equal(t, equal, true)

		equal := compare(t, standardFilePath+".sst", v+".sst")
		assert.Equal(t, equal, true)
	}
}

func Test_InVocabulary(t *testing.T) {
	// lci.ArrangedIndividual is a vocabulary element, so it should have a vocabulary element.
	t.Run("in_vocabulary", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)
		graph := st.CreateNamedGraph("")
		d := graph.CreateIRINode("", lci.ArrangedIndividual)

		d.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
			fmt.Printf("s: %s, p: %s, o: %s\n", s.IRI(), p.IRI(), o.(sst.IBNode).IRI())
			ei := o.(sst.IBNode).InVocabulary().VocabularyElement()
			fmt.Println(ei)
			return nil
		})
	})
	t.Run("in_vocabulary_not_found", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)
		graph := st.CreateNamedGraph("")
		d := graph.CreateIRINode("", countrycodes.Ad)

		d.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
			fmt.Printf("s: %s, p: %s, o: %s\n", s.IRI(), p.IRI(), o.(sst.IBNode).IRI())
			assert.Panics(t, func() {
				ei := o.(sst.IBNode).InVocabulary().VocabularyElement()
				fmt.Println(ei)
			})
			return nil
		})
	})
}

func Test_StaticStage(t *testing.T) {
	// lci.ArrangedIndividual is a vocabulary element, so it should have a vocabulary element.
	t.Run("in_vocabulary", func(t *testing.T) {
		// sst.StaticDictionary().Dump()
	})
}
