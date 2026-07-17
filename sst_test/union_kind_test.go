// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"testing"

	"github.com/semanticstep/sst-core/sst"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/eed"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/owl"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// vocabularyNode returns the dictionary IBNode for a vocabulary Element.
// IsKind is based on the canonical IRI, so we test against the dictionary
// entries rather than arbitrary instance nodes.
func vocabularyNode(t *testing.T, elem sst.Element) sst.IBNode {
	t.Helper()
	node, err := sst.StaticDictionary().Element(elem)
	require.NoError(t, err)
	return node
}

// TestIsKindUnionClass verifies that union members and sub-classes of union
// members are recognized as kinds of the union class.
func TestIsKindUnionClass(t *testing.T) {
	// rep:Point is a direct member of the rep:GeometricSetSelect union.
	point := vocabularyNode(t, rep.Point.Element)
	assert.True(t, point.IsKind(rep.GeometricSetSelect), "direct union member should be a kind of the union")

	// rep:CartesianPoint is a sub-class of rep:Point.
	cartesianPoint := vocabularyNode(t, rep.CartesianPoint.Element)
	assert.True(t, cartesianPoint.IsKind(rep.Point), "sub-class should be a kind of its super-class")
	assert.True(t, cartesianPoint.IsKind(rep.GeometricSetSelect),
		"sub-class of a union member should transitively be a kind of the union")

	// rep:GeometricSet is the domain of rep:element, not a member of the
	// GeometricSetSelect union.
	geomSet := vocabularyNode(t, rep.GeometricSet.Element)
	assert.False(t, geomSet.IsKind(rep.GeometricSetSelect), "unrelated class should not be a kind of the union")
}

// TestIsKindCrossVocabulary verifies that class hierarchies spanning multiple
// vocabularies are resolved correctly by the compiler's global reasoner.
func TestIsKindCrossVocabulary(t *testing.T) {
	// eed:BidirectionalMasterPort ⊑ eed:MasterPort ⊑ lci:Individual ⊑ lci:Thing ⊑ owl:Thing
	port := vocabularyNode(t, eed.BidirectionalMasterPort.Element)
	assert.True(t, port.IsKind(eed.BidirectionalMasterPort), "a node should be a kind of its own type")
	assert.True(t, port.IsKind(eed.MasterPort), "direct super-class should be recognized")
	assert.True(t, port.IsKind(lci.Individual), "cross-vocabulary super-class should be recognized")
	assert.True(t, port.IsKind(lci.Thing), "transitive cross-vocabulary super-class should be recognized")
	assert.True(t, port.IsKind(owl.Thing), "top-level cross-vocabulary super-class should be recognized")
}

// TestIsKindReflexivityAndTopClasses verifies basic IsKind behavior for
// identity and top-level classes.
func TestIsKindReflexivityAndTopClasses(t *testing.T) {
	point := vocabularyNode(t, rep.Point.Element)
	assert.True(t, point.IsKind(rep.Point), "IsKind should be reflexive")
	assert.True(t, point.IsKind(owl.Thing), "every class should be a kind of owl:Thing")
}
