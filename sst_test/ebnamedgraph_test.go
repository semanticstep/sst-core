// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"fmt"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func Test_ExampleNamedGraph_CreateNode(t *testing.T) {
	stage := sst.OpenStage(sst.DefaultTriplexMode)
	id, _ := uuid.NewRandom()
	graph := stage.CreateNamedGraph(sst.IRI(id.URN()))
	org := graph.CreateIRINode("", lci.Organization, lci.ArrangedIndividual)
	ind := graph.CreateBlankNode(lci.Individual)

	ind.AddStatement(lci.PartOf, org)

	fmt.Printf("IRI Node Count: %d\n", graph.IRINodeCount())
	// Output:
	//  IRI Node Count: 1
}

func Test_literalCollectionObject(t *testing.T) {
	t.Run("literal_collection_object", func(t *testing.T) {
		stage := sst.OpenStage(sst.DefaultTriplexMode)
		id, _ := uuid.NewRandom()
		graph := stage.CreateNamedGraph(sst.IRI(id.URN()))
		d := graph.CreateBlankNode()
		list := sst.NewLiteralCollection(sst.String("c1"))
		d.AddStatement(rdfs.Comment, list)
		list2 := sst.NewLiteralCollection(sst.String("c1"))
		assert.Panics(t, func() { d.AddStatement(rdfs.Comment, list2) })
		assert.Panics(t, func() { d.AddStatement(sst.IRI(""), sst.String("")) })
		assert.Panics(t, func() { d.AddStatement(rdfs.Comment, sst.IRI("")) })
		var gotP sst.IBNode
		var gotO sst.Term
		assert.NoError(t, d.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
			assert.Equal(t, nil, gotP)
			gotP = p
			assert.Nil(t, gotO)
			gotO = o
			return nil
		}))
		assert.Equal(t, rdfs.RDFSVocabulary.BaseIRI+"#"+rdfs.Comment.Name, gotP.IRI().String())
		if gotO, ok := gotO.(sst.LiteralCollection); ok {
			assert.Equal(t, 1, gotO.MemberCount())
			assert.Equal(t, sst.String("c1"), gotO.Member(0))
			assert.True(t, sst.String("").DataType() == gotO.Member(0).DataType())
			// create new list by inserting a new literal at position 0, set it as new triple object and have assertions
			exm := make([]sst.Literal, gotO.MemberCount()+1)
			// mm := gotO.Members(exm[1:])
			// if assert.Len(t, mm, 1) {
			// 	assert.Equal(t, "c1", mm[0].Value())
			// 	assert.True(t, sst.String("").DataType() == mm[0].DataType())
			// }
			exm[0] = sst.String("c0")
			exm[1] = gotO.Member(0).(sst.String)
			listEx := sst.NewLiteralCollection(exm[0], exm[1:]...)
			d.AddStatement(rdfs.Comment, listEx)
			assert.NotPanics(t, func() {
				d.DeleteTriples()
				d.AddStatement(rdfs.Comment, listEx)
			})
			d.OwningGraph().Dump()
			var gotPEx sst.IBNode
			var gotOEx sst.Term
			assert.NoError(t, d.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
				assert.Equal(t, nil, gotPEx)
				gotPEx = p
				assert.Nil(t, gotOEx)
				gotOEx = o
				return nil
			}))
			assert.Equal(t, gotP.IRI().String(), gotPEx.IRI().String())
			if gotOEx, ok := gotOEx.(sst.LiteralCollection); ok {
				assert.Equal(t, 2, gotOEx.MemberCount())
				assert.Equal(t, sst.String("c0"), gotOEx.Member(0))
				assert.Equal(t, sst.String("c1"), gotOEx.Member(1))
			}
			// assert triple with collection duplications
			list3 := sst.NewLiteralCollection(sst.String("c1"), sst.String("c0"))
			d.AddStatement(rdfs.Comment, list3)
		}
	})
}
