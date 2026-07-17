// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"bufio"
	"strings"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func fromTurtleContent(
	_ *testing.T,
	content string,
) (rdr *bufio.Reader) {
	return bufio.NewReader(strings.NewReader(content))
}

func Test_TermCollection_RdfRead_test(t *testing.T) {
	type args struct {
		bReader *bufio.Reader
	}
	toArgs := func(rdr *bufio.Reader) args {
		return args{bReader: rdr}
	}
	tests := []struct {
		name           string
		args           args
		expected       string
		graphAssertion func(t *testing.T, graph sst.NamedGraph)
		errorAssertion assert.ErrorAssertionFunc
	}{
		{
			name: "term_collection_three_members",
			args: toArgs(fromTurtleContent(t,
				`
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix rdf:    <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:   <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:    <http://www.w3.org/2002/07/owl#> .
@prefix ex:   <http://example.com/demo#> .

<http://example.com/demo#>    a   owl:Ontology .

ex:Pump a lci:SpaceTimeIndividual . 
ex:Impeller1 a lci:SpaceTimeIndividual . 
ex:Shaft1 a lci:SpaceTimeIndividual . 
ex:Cover1 a lci:SpaceTimeIndividual . 

ex:Pump1 a ex:Pump ;
    lci:hasPart (
        ex:Impeller1
        ex:Shaft1
        ex:Cover1
    ) .

`)),

			expected: `{
  "kinds": [
    "rdf_type"
  ],
  "passed": true,
  "generated": "2025-11-07T11:14:49.473987+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {

			},

			errorAssertion: assert.NoError,
		},

		{
			name: "term_collection_no_member",
			args: toArgs(fromTurtleContent(t,
				`
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix rdf:    <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:   <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:    <http://www.w3.org/2002/07/owl#> .
@prefix ex:   <http://example.com/demo#> .

<http://example.com/demo#>    a   owl:Ontology .

ex:Pump a lci:SpaceTimeIndividual . 

ex:Pump1 a ex:Pump ;
    lci:hasPart () .

`)),

			expected: `{
  "kinds": [
    "rdf_type"
  ],
  "passed": true,
  "generated": "2025-11-07T11:14:49.473987+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
				pump1 := graph.GetIRINodeByFragment("Pump1")
				assert.True(t, pump1.CheckTriple(lci.HasPart, rdf.Nil))
			},

			errorAssertion: assert.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, err := sst.RdfRead(tt.args.bReader, sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
			assert.NoError(t, err)

			tt.graphAssertion(t, st.NamedGraphs()[0])

			// st.Dump()
		})
	}
}

func Test_TermCollection_Create_test(t *testing.T) {
	t.Run("EmptyCollection", func(t *testing.T) {
		ngID := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361")

		stage := sst.OpenStage(sst.DefaultTriplexMode)
		ng := stage.CreateNamedGraph(sst.IRI(ngID.URN()))

		ib := ng.CreateIRINode("")
		col := ng.CreateCollection()
		ib.AddStatement(lci.HasPart, col)

		assert.True(t, ib.CheckTriple(lci.HasPart, rdf.Nil))
	})

	t.Run("Element Collection", func(t *testing.T) {
		ngID := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361")

		stage := sst.OpenStage(sst.DefaultTriplexMode)
		ng := stage.CreateNamedGraph(sst.IRI(ngID.URN()))

		ib := ng.CreateIRINode("")
		col := ng.CreateCollection(lci.NominalQuantity, lci.MinMaxQuantity, lci.MinNomMaxQuantity)
		ib.AddStatement(lci.HasPart, col)
		assert.True(t, ib.CheckTriple(lci.HasPart, col))
		assert.Equal(t, 3, col.MemberCount())
	})
}

func Test_LiteralCollection_Create_test(t *testing.T) {
	t.Run("EmptyCollection", func(t *testing.T) {
		ngID := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361")

		stage := sst.OpenStage(sst.DefaultTriplexMode)
		ng := stage.CreateNamedGraph(sst.IRI(ngID.URN()))

		ib := ng.CreateIRINode("")
		_ = ib
		// lcol := sst.NewLiteralCollection()
		// ib.AddStatement(lci.HasPart, lcol)

		// assert.True(t, ib.CheckTriple(lci.HasPart, rdf.Nil))
	})
}
