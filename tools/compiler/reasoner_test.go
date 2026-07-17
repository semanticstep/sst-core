// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package main

import (
	"testing"

	"github.com/semanticstep/sst-core/sst"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newReasonerTestGraph creates a fresh in-memory graph for reasoner tests.
func newReasonerTestGraph(t *testing.T) sst.NamedGraph {
	t.Helper()
	stage := sst.OpenStage(sst.DefaultTriplexMode)
	return stage.CreateNamedGraph(sst.IRI("urn:uuid:12345678-1234-1234-1234-123456789abc"))
}

// newReasonerClass creates a named class node plus the sstToGo wrapper used by the compiler.
func newReasonerClass(t *testing.T, graph sst.NamedGraph, fragment string) (sst.IBNode, sstToGo) {
	t.Helper()
	node := graph.CreateIRINode(fragment)
	return node, sstToGo{ibs: node, vp: vocabProperties{isClass: true}}
}

// newReasonerDatatype creates a named datatype node plus the sstToGo wrapper used by the compiler.
func newReasonerDatatype(t *testing.T, graph sst.NamedGraph, fragment string) (sst.IBNode, sstToGo) {
	t.Helper()
	node := graph.CreateIRINode(fragment)
	return node, sstToGo{ibs: node, vp: vocabProperties{isDatatype: true}}
}

// hasSuperClass reports whether sub is a sub-class of super in the computed graph.
func hasSuperClass(g *subsumptionGraph, sub, super sst.IBNode) bool {
	_, ok := g.superClasses(ibNodeKey(sub))[ibNodeKey(super)]
	return ok
}

// TestReasonerTransitiveSubClassOf verifies that rdfs:subClassOf chains are followed.
func TestReasonerTransitiveSubClassOf(t *testing.T) {
	graph := newReasonerTestGraph(t)

	a, aEl := newReasonerClass(t, graph, "A")
	b, bEl := newReasonerClass(t, graph, "B")
	c, cEl := newReasonerClass(t, graph, "C")

	// C ⊑ B ⊑ A
	b.AddStatement(rdfsSubClassOf, a)
	bEl.vp.subtypeOf = append(bEl.vp.subtypeOf, a)
	c.AddStatement(rdfsSubClassOf, b)
	cEl.vp.subtypeOf = append(cEl.vp.subtypeOf, b)

	g := buildSubsumptionGraph([]sstToGo{aEl, bEl, cEl})

	assert.True(t, hasSuperClass(g, b, a), "B should be a sub-class of A")
	assert.True(t, hasSuperClass(g, c, b), "C should be a sub-class of B")
	assert.True(t, hasSuperClass(g, c, a), "C should transitively be a sub-class of A")
	assert.False(t, hasSuperClass(g, a, c), "A is not a sub-class of C")
}

// TestReasonerUnionOf verifies that each named member of an owl:unionOf becomes
// a sub-class of the union, and that existing sub-classes of members also
// subsume the union.
func TestReasonerUnionOf(t *testing.T) {
	graph := newReasonerTestGraph(t)

	union, unionEl := newReasonerClass(t, graph, "Union")
	memberA, memberAEl := newReasonerClass(t, graph, "MemberA")
	memberB, memberBEl := newReasonerClass(t, graph, "MemberB")
	subOfA, subOfAEl := newReasonerClass(t, graph, "SubOfA")

	listNode := graph.CreateCollection(memberA, memberB)
	union.AddStatement(owlUnionOf, listNode)

	subOfA.AddStatement(rdfsSubClassOf, memberA)
	subOfAEl.vp.subtypeOf = append(subOfAEl.vp.subtypeOf, memberA)

	g := buildSubsumptionGraph([]sstToGo{unionEl, memberAEl, memberBEl, subOfAEl})

	assert.True(t, hasSuperClass(g, memberA, union), "member A should sub-class union")
	assert.True(t, hasSuperClass(g, memberB, union), "member B should sub-class union")
	assert.True(t, hasSuperClass(g, subOfA, union), "sub-class of member A should sub-class union")
	assert.False(t, hasSuperClass(g, union, memberA), "union is not a sub-class of its member")
}

// TestReasonerDisjointUnionOf checks that owl:disjointUnionOf has the same
// subsumption semantics as owl:unionOf.
func TestReasonerDisjointUnionOf(t *testing.T) {
	graph := newReasonerTestGraph(t)

	union, unionEl := newReasonerClass(t, graph, "DisjointUnion")
	memberA, memberAEl := newReasonerClass(t, graph, "DUMemberA")
	memberB, memberBEl := newReasonerClass(t, graph, "DUMemberB")

	listNode := graph.CreateCollection(memberA, memberB)
	union.AddStatement(owlDisjointUnionOf, listNode)

	g := buildSubsumptionGraph([]sstToGo{unionEl, memberAEl, memberBEl})

	assert.True(t, hasSuperClass(g, memberA, union), "member A should sub-class disjoint union")
	assert.True(t, hasSuperClass(g, memberB, union), "member B should sub-class disjoint union")
}

// TestReasonerIntersectionOf verifies that an intersection class is a sub-class
// of each of its member classes.
func TestReasonerIntersectionOf(t *testing.T) {
	graph := newReasonerTestGraph(t)

	intersection, intersectionEl := newReasonerClass(t, graph, "Intersection")
	memberA, memberAEl := newReasonerClass(t, graph, "IntersectionMemberA")
	memberB, memberBEl := newReasonerClass(t, graph, "IntersectionMemberB")

	listNode := graph.CreateCollection(memberA, memberB)
	intersection.AddStatement(owlIntersectionOf, listNode)

	g := buildSubsumptionGraph([]sstToGo{intersectionEl, memberAEl, memberBEl})

	assert.True(t, hasSuperClass(g, intersection, memberA), "intersection should sub-class member A")
	assert.True(t, hasSuperClass(g, intersection, memberB), "intersection should sub-class member B")
	assert.False(t, hasSuperClass(g, memberA, intersection), "member A is not a sub-class of the intersection")
}

// TestReasonerEquivalentClass verifies that owl:equivalentClass creates
// bidirectional sub-class edges.
func TestReasonerEquivalentClass(t *testing.T) {
	graph := newReasonerTestGraph(t)

	a, aEl := newReasonerClass(t, graph, "EquivalentA")
	b, bEl := newReasonerClass(t, graph, "EquivalentB")

	a.AddStatement(owlEquivalentClass, b)

	g := buildSubsumptionGraph([]sstToGo{aEl, bEl})

	assert.True(t, hasSuperClass(g, a, b), "A should sub-class equivalent B")
	assert.True(t, hasSuperClass(g, b, a), "B should sub-class equivalent A")
}

// TestReasonerOnDatatype verifies that a derived datatype with owl:onDatatype
// is a sub-class of its base datatype.
func TestReasonerOnDatatype(t *testing.T) {
	graph := newReasonerTestGraph(t)

	base, baseEl := newReasonerDatatype(t, graph, "BaseDatatype")
	derived, derivedEl := newReasonerDatatype(t, graph, "DerivedDatatype")

	derived.AddStatement(owlOnDatatype, base)

	g := buildSubsumptionGraph([]sstToGo{baseEl, derivedEl})

	assert.True(t, hasSuperClass(g, derived, base), "derived datatype should sub-class base datatype")
	assert.False(t, hasSuperClass(g, base, derived), "base datatype is not a sub-class of derived datatype")
}

// TestReasonerNestedUnion verifies that unions containing other unions propagate
// subsumption transitively.
func TestReasonerNestedUnion(t *testing.T) {
	graph := newReasonerTestGraph(t)

	outer, outerEl := newReasonerClass(t, graph, "OuterUnion")
	inner, innerEl := newReasonerClass(t, graph, "InnerUnion")
	memberA, memberAEl := newReasonerClass(t, graph, "NestedMemberA")
	memberB, memberBEl := newReasonerClass(t, graph, "NestedMemberB")

	innerList := graph.CreateCollection(memberA, memberB)
	inner.AddStatement(owlUnionOf, innerList)

	outerList := graph.CreateCollection(memberA, inner)
	outer.AddStatement(owlUnionOf, outerList)

	g := buildSubsumptionGraph([]sstToGo{outerEl, innerEl, memberAEl, memberBEl})

	assert.True(t, hasSuperClass(g, memberA, outer), "direct outer member A should sub-class outer union")
	assert.True(t, hasSuperClass(g, memberB, inner), "member B should sub-class inner union")
	assert.True(t, hasSuperClass(g, memberB, outer), "member B should transitively sub-class outer union")
	assert.True(t, hasSuperClass(g, inner, outer), "inner union should sub-class outer union")
}

// TestReasonerAnonymousUnionMember verifies that blank-node members of a union
// still participate in reasoning, but they are not marked as named nodes.
func TestReasonerAnonymousUnionMember(t *testing.T) {
	graph := newReasonerTestGraph(t)

	union, unionEl := newReasonerClass(t, graph, "UnionWithAnonymous")
	namedMember, namedMemberEl := newReasonerClass(t, graph, "NamedMember")
	anonymousMember := graph.CreateBlankNode()

	listNode := graph.CreateCollection(namedMember, anonymousMember)
	union.AddStatement(owlUnionOf, listNode)

	g := buildSubsumptionGraph([]sstToGo{unionEl, namedMemberEl})

	require.True(t, hasSuperClass(g, namedMember, union), "named member should sub-class union")

	namedNode := g.nodes[ibNodeKey(namedMember)]
	anonymousNode := g.nodes[ibNodeKey(anonymousMember)]
	require.NotNil(t, namedNode, "named member should be present in the graph")
	require.NotNil(t, anonymousNode, "anonymous member should be present in the graph")

	assert.True(t, namedNode.named, "named member node should be flagged as named")
	assert.False(t, anonymousNode.named, "anonymous member node should not be flagged as named")
}

// TestReasonerCyclicSubClassOf ensures the transitive closure terminates when
// the ontology contains a cycle.
func TestReasonerCyclicSubClassOf(t *testing.T) {
	graph := newReasonerTestGraph(t)

	a, aEl := newReasonerClass(t, graph, "CyclicA")
	b, bEl := newReasonerClass(t, graph, "CyclicB")

	a.AddStatement(rdfsSubClassOf, b)
	aEl.vp.subtypeOf = append(aEl.vp.subtypeOf, b)
	b.AddStatement(rdfsSubClassOf, a)
	bEl.vp.subtypeOf = append(bEl.vp.subtypeOf, a)

	g := buildSubsumptionGraph([]sstToGo{aEl, bEl})

	assert.True(t, hasSuperClass(g, a, b), "A should sub-class B")
	assert.True(t, hasSuperClass(g, b, a), "B should sub-class A")
}

// TestReasonerKindMethodForNamedClass verifies that kindMethodFor generates a
// method signature for named super-classes.
func TestReasonerKindMethodForNamedClass(t *testing.T) {
	graph := newReasonerTestGraph(t)

	super, superEl := newReasonerClass(t, graph, "SuperForMethod")
	sub, subEl := newReasonerClass(t, graph, "SubForMethod")

	sub.AddStatement(rdfsSubClassOf, super)
	subEl.vp.subtypeOf = append(subEl.vp.subtypeOf, super)

	g := buildSubsumptionGraph([]sstToGo{superEl, subEl})

	superKey := ibNodeKey(super)
	superNode := g.nodes[superKey]
	require.NotNil(t, superNode)
	assert.True(t, superNode.named)

	method := kindMethodFor("subI", superNode)
	assert.Contains(t, method, "AsKind_", "generated method name should contain AsKind_")
	assert.Contains(t, method, "()", "generated method should be a nullary method")
}
