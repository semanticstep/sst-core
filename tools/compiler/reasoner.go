// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

// Package main (tools/compiler) contains the OWL micro-reasoner used to infer
// class subsumption relationships at compile time.
//
// The reasoner builds a directed graph where an edge A -> B means
// "A is a sub-class of B" (A ⊑ B). Edges are derived from rdfs:subClassOf,
// owl:unionOf, owl:disjointUnionOf, owl:intersectionOf, owl:equivalentClass
// and owl:onDatatype. A transitive closure is then computed so that every
// implicit super-class relationship becomes explicit.
//
// Finally, for each named class the compiler emits AsKind_* marker methods
// for all reachable named super-classes. Anonymous classes participate in
// reasoning but do not get marker methods because they have no stable IRI to
// encode into a Go method name.
package main

import (
	"fmt"

	"github.com/semanticstep/sst-core/sst"
)

// classNode represents a class or datatype in the reasoner graph.
// It can be a named class (with an IRI) or an anonymous class (blank node).
type classNode struct {
	key     string      // stable identifier: IRI string or blank node ID
	named   bool        // true for IRI nodes, false for blank nodes
	ibNode  sst.IBNode  // original ontology node (nil is allowed for synthetic nodes)
	element sst.Element // valid only for named nodes
}

// subsumptionGraph is a directed graph: edges[sub][super] == true means
// "sub is a sub-class of super" (sub ⊑ super).
type subsumptionGraph struct {
	nodes map[string]*classNode
	edges map[string]map[string]struct{}
}

// newSubsumptionGraph creates an empty subsumption graph.
func newSubsumptionGraph() *subsumptionGraph {
	return &subsumptionGraph{
		nodes: make(map[string]*classNode),
		edges: make(map[string]map[string]struct{}),
	}
}

// ibNodeKey returns a stable string key for an IBNode.
// Named nodes are keyed by their full IRI; blank nodes by their internal UUID.
func ibNodeKey(n sst.IBNode) string {
	if n.IsBlankNode() {
		return n.ID().String()
	}
	return n.IRI().String()
}

// addNode registers ib in the graph if it is not already present.
func (g *subsumptionGraph) addNode(ib sst.IBNode) {
	key := ibNodeKey(ib)
	if _, ok := g.nodes[key]; ok {
		return
	}
	n := &classNode{
		key:    key,
		named:  !ib.IsBlankNode(),
		ibNode: ib,
	}
	if n.named {
		n.element = sst.Element{
			Vocabulary: sst.Vocabulary{BaseIRI: ib.OwningGraph().IRI().String()},
			Name:       ib.Fragment(),
		}
	}
	g.nodes[key] = n
}

// addSubClass records that sub is a sub-class of super.
// Both keys must already refer to nodes in the graph (addNode is called
// automatically by the edge-building helpers).
func (g *subsumptionGraph) addSubClass(sub, super string) {
	if g.edges[sub] == nil {
		g.edges[sub] = make(map[string]struct{})
	}
	g.edges[sub][super] = struct{}{}
}

// buildSubsumptionGraph constructs the subsumption graph for all classes and
// datatypes in elements.
func buildSubsumptionGraph(elements []sstToGo) *subsumptionGraph {
	g := newSubsumptionGraph()

	// Step 1: register every named class and datatype as a node.
	for _, element := range elements {
		if element.vp.isClass || element.vp.isDatatype {
			g.addNode(element.ibs)
		}
	}

	// Step 2: add edges according to OWL/RDFS class constructors.
	for _, element := range elements {
		ib := element.ibs
		key := ibNodeKey(ib)

		// rdfs:subClassOf
		for _, super := range element.vp.subtypeOf {
			g.addNode(super)
			g.addSubClass(key, ibNodeKey(super))
		}

		// owl:unionOf: each member is a sub-class of the union class.
		addUnionEdges(g, ib, owlUnionOf)

		// owl:disjointUnionOf: same subsumption semantics as unionOf.
		addUnionEdges(g, ib, owlDisjointUnionOf)

		// owl:intersectionOf: the intersection class is a sub-class of each member.
		addIntersectionEdges(g, ib)

		// owl:equivalentClass: two equivalent classes are sub-classes of each other.
		addEquivalentClassEdges(g, ib)

		// owl:onDatatype: a derived datatype is a sub-class of its base datatype.
		addOnDatatypeEdges(g, ib)
	}

	// Step 3: compute the transitive closure.
	return g.transitiveClosure()
}

// addUnionEdges scans all triples "ib predicate listNode" where predicate is
// owl:unionOf or owl:disjointUnionOf, walks the rdf:List members and records
// that every named member is a sub-class of ib.
func addUnionEdges(g *subsumptionGraph, ib sst.IBNode, predicate sst.Element) {
	_ = ib.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if s != ib || !p.Is(predicate) {
			return nil
		}
		if o.TermKind() != sst.TermKindIBNode && o.TermKind() != sst.TermKindTermCollection {
			return nil
		}
		listNode := o.(sst.IBNode)
		collection, ok := listNode.AsCollection()
		if !ok {
			return nil
		}
		unionKey := ibNodeKey(ib)
		collection.ForMembers(func(_ int, member sst.Term) {
			memberNode, ok := member.(sst.IBNode)
			if !ok {
				return
			}
			g.addNode(memberNode)
			g.addSubClass(ibNodeKey(memberNode), unionKey)
		})
		return nil
	})
}

// addIntersectionEdges scans all triples "ib owl:intersectionOf listNode",
// walks the rdf:List members and records that ib is a sub-class of every
// named member.
func addIntersectionEdges(g *subsumptionGraph, ib sst.IBNode) {
	_ = ib.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if s != ib || !p.Is(owlIntersectionOf) {
			return nil
		}
		if o.TermKind() != sst.TermKindIBNode && o.TermKind() != sst.TermKindTermCollection {
			return nil
		}
		listNode := o.(sst.IBNode)
		collection, ok := listNode.AsCollection()
		if !ok {
			return nil
		}
		intersectionKey := ibNodeKey(ib)
		collection.ForMembers(func(_ int, member sst.Term) {
			memberNode, ok := member.(sst.IBNode)
			if !ok {
				return
			}
			g.addNode(memberNode)
			g.addSubClass(intersectionKey, ibNodeKey(memberNode))
		})
		return nil
	})
}

// addEquivalentClassEdges scans all triples "ib owl:equivalentClass other"
// and records bidirectional sub-class edges.
func addEquivalentClassEdges(g *subsumptionGraph, ib sst.IBNode) {
	_ = ib.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if s != ib || !p.Is(owlEquivalentClass) {
			return nil
		}
		other, ok := o.(sst.IBNode)
		if !ok {
			return nil
		}
		g.addNode(other)
		key := ibNodeKey(ib)
		otherKey := ibNodeKey(other)
		g.addSubClass(key, otherKey)
		g.addSubClass(otherKey, key)
		return nil
	})
}

// addOnDatatypeEdges scans all triples "ib owl:onDatatype base" and records
// that ib is a sub-class of the base datatype.
func addOnDatatypeEdges(g *subsumptionGraph, ib sst.IBNode) {
	_ = ib.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if s != ib || !p.Is(owlOnDatatype) {
			return nil
		}
		base, ok := o.(sst.IBNode)
		if !ok {
			return nil
		}
		g.addNode(base)
		g.addSubClass(ibNodeKey(ib), ibNodeKey(base))
		return nil
	})
}

// transitiveClosure computes the reachability closure of the subsumption graph.
// After this call, if there is a path A -> ... -> B, edges[A][B] is true.
// The algorithm is Floyd-Warshall style: for every intermediate node k,
// if A -> k and k -> B then add A -> B.
func (g *subsumptionGraph) transitiveClosure() *subsumptionGraph {
	for k := range g.nodes {
		for sub := range g.edges {
			if _, ok := g.edges[sub][k]; !ok {
				continue
			}
			for sup := range g.edges[k] {
				g.edges[sub][sup] = struct{}{}
			}
		}
	}
	return g
}

// superClasses returns the set of all super-class keys reachable from key.
// The returned map is the closure-computed edges[key]; it may be empty but
// never nil.
func (g *subsumptionGraph) superClasses(key string) map[string]struct{} {
	if supers, ok := g.edges[key]; ok {
		return supers
	}
	return map[string]struct{}{}
}

// kindMethodFor returns the generated AsKind_* method signature for the
// given receiver type and super-class node.
func kindMethodFor(receiver string, super *classNode) string {
	return fmt.Sprintf("func (%s) %s()", receiver, kindMethodOf(super.element))
}
