// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package validate

import (
	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
)

// ibNodeKey returns a string key that uniquely identifies an IBNode.
// Blank nodes use their UUID prefixed with "_:", while IRI nodes use their full IRI.
func ibNodeKey(n sst.IBNode) string {
	if n.IsBlankNode() {
		return "_:" + n.ID().String()
	}
	return string(n.IRI())
}

// graphIndex holds pre-computed domain/range/subClassOf information
// extracted from a NamedGraph so that late-binding validation can be
// performed without scanning the whole graph repeatedly.
type graphIndex struct {
	domainOf   map[string]sst.IBNode   // property key -> domain class
	rangeOf    map[string]sst.IBNode   // property key -> range class
	subClassOf map[string][]sst.IBNode // class key -> direct superclasses
}

// buildGraphIndex scans all triples in the given graph and populates
// domain, range and subClassOf lookup tables.
func buildGraphIndex(graph sst.NamedGraph) *graphIndex {
	idx := &graphIndex{
		domainOf:   make(map[string]sst.IBNode),
		rangeOf:    make(map[string]sst.IBNode),
		subClassOf: make(map[string][]sst.IBNode),
	}

	_ = graph.ForAllIBNodes(func(s sst.IBNode) error {
		_ = s.ForAll(func(_ int, subj, pred sst.IBNode, obj sst.Term) error {
			if s != subj {
				return nil
			}
			switch {
			case pred.Is(rdfs.Domain) && obj.TermKind() == sst.TermKindIBNode:
				idx.domainOf[ibNodeKey(s)] = obj.(sst.IBNode)
			case pred.Is(rdfs.Range) && obj.TermKind() == sst.TermKindIBNode:
				idx.rangeOf[ibNodeKey(s)] = obj.(sst.IBNode)
			case pred.Is(rdfs.SubClassOf) && obj.TermKind() == sst.TermKindIBNode:
				idx.subClassOf[ibNodeKey(s)] = append(idx.subClassOf[ibNodeKey(s)], obj.(sst.IBNode))
			}
			return nil
		})
		return nil
	})

	return idx
}

// domainFromGraph returns the rdfs:domain of property p as declared
// inside the graph itself. If no domain is declared, nil is returned.
func domainFromGraph(idx *graphIndex, p sst.IBNode) sst.IBNode {
	return idx.domainOf[ibNodeKey(p)]
}

// rangeFromGraph returns the rdfs:range of property p as declared
// inside the graph itself. If no range is declared, nil is returned.
func rangeFromGraph(idx *graphIndex, p sst.IBNode) sst.IBNode {
	return idx.rangeOf[ibNodeKey(p)]
}

// isKindFromGraph reports whether t is the same class as kind or is a
// subclass of kind according to rdfs:subClassOf chains found in the
// graph.
func isKindFromGraph(idx *graphIndex, t, kind sst.IBNode) bool {
	if ibNodeKey(t) == ibNodeKey(kind) {
		return true
	}
	for _, super := range idx.subClassOf[ibNodeKey(t)] {
		if isKindFromGraph(idx, super, kind) {
			return true
		}
	}
	return false
}
