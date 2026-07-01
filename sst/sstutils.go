// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

func (g *namedGraph) allocateTriplexes(count int) {
	if g == nil {
		return
	}
	GlobalLogger.Debug("namedGraph", zap.Int("allocateTriplexes", count))
	g.triplexStorage = make([]triplex, count)
}

// createAllocatedNode creates or retrieves an IBNode of the given type in this NamedGraph
// and reserves the specified number of triplex slots for it.
// The node is positioned at triplexStart in the graph's triplex storage.
// It panics if the graph is nil, if the fragment is duplicated, or on any other fatal error.
func (g *namedGraph) createAllocatedNode(
	fragment string, ibFlag ibNodeFlag,
	triplexStart, allocatedTriplexCnt int,
) (sub IBNode, triplexEnd int) {
	if g == nil {
		panicf("createAllocatedNode: nil namedGraph for fragment=%q ibFlag=%v", fragment, ibFlag)
	}

	var s *ibNode
	var IsUuidFragment bool
	id, err := uuid.Parse(fragment)
	if err == nil {
		IsUuidFragment = true
	} else {
		IsUuidFragment = false
	}

	switch ibFlag {
	case iriNodeType:
		if g.stringNodes == nil {
			panicf("createAllocatedNode: stringNodes map is nil for fragment=%q", fragment)
		}
		if IsUuidFragment {
			if ib, found := g.uuidNodes[id]; found {
				s = &ib.ibNode
			} else {
				var err error
				s, err = g.createIriUUIDNode(id)
				if err != nil {
					panic(err)
				}
			}
		} else {
			if ib, found := g.stringNodes[fragment]; found {
				s = &ib.ibNode
			} else {
				var err error
				s, err = g.createIRIStringNode(fragment)
				if err != nil {
					panic(err)
				}
			}
		}
	case blankNodeType:
		if !g.IsReferenced() {
			s = g.createBlankUUIDNode()
		}
	}
	// triplexStart = end ?
	s.triplexStart = triplexOffset(triplexStart)
	s.triplexEnd = s.triplexStart
	return s, triplexStart + allocatedTriplexCnt
}

// Len implements sort.Interface.Len() method.
// Len returns the number of entries in the triplex structure so that they can be sorted by triplex kind.
func (t *ibNode) Len() int {
	return int(t.triplexEnd - t.triplexStart)
}

// Less implements sort.Interface.Less() method for the purpose of sorting the internal triplex structure by triplex kind.
// TODO: ensure that i and j are within 0 to Len().
func (t *ibNode) Less(i, j int) bool {
	hs := t.ng.triplexStorage
	h1, h2 := hs[int(t.triplexStart)+i], hs[int(t.triplexStart)+j]
	if h1.p == nil {
		return false
	}
	if h2.p == nil {
		return true
	}

	return (*ibNode)(t).triplexKindAt(i) < (*ibNode)(t).triplexKindAt(j)
}

// Swap implements sort.Interface.Swap() method for the purpose of sorting triplexes of an IBNode by their triplex kind.
// TODO: ensure that i and j are within 0 to Len().
func (t *ibNode) Swap(i, j int) {
	swapTriplexes((*ibNode)(t), i, j)
}

type ibNodeWriteIncs struct {
	incImportedRefCount, incExternalRefCount func(t IBNode)
}

func (t *ibNode) sortAndCountTriples(incImportedRefCount, incExternalRefCount func(t IBNode)) (c ibNodeWriteCounts) {
	if t.triplexEnd-t.triplexStart == 0 {
		return
	}
	triplexes := t.ng.triplexStorage[int(t.triplexStart):int(t.triplexEnd)]
	// sort by triplex kind
	sort.Sort((*ibNode)(t))
	owningGraph := t.ng
	ci := ibNodeWriteIncs{incImportedRefCount: incImportedRefCount, incExternalRefCount: incExternalRefCount}
	for i, tx := range triplexes {
		txp := tx.p
		if txp == nil {
			break
		}
		switch k := t.triplexKindAt(i); k {
		case subjectTriplexKind:
			incRefCount(txp, owningGraph, ci)
			if txp.Is(rdfFirst) {
				c.termCollectionMemberCount++
			}
			switch resourceTypeRecursive(tx.t) {
			case resourceTypeIBNode:
				if !txp.Is(rdfFirst) {
					c.nonTermCollectionIBNodeTripleCount++
				}
				incRefCount(tx.t.asIBNode(), owningGraph, ci)
			case resourceTypeLiteral:
				if txp.Is(rdfFirst) {
					c.termCollectionLiteralMemberCount++
				} else {
					c.nonTermCollectionLiteralTripleCount++
				}
			case resourceTypeLiteralCollection:
				if txp.Is(rdfFirst) {
					c.termCollectionLiteralMemberCount++
				} else {
					c.nonTermCollectionLiteralCollectionCount++
				}
			}
		case objectTriplexKind:
			if isResourceTypeIBNode(tx.t) {
				incExtraCount(tx.t.asIBNode(), owningGraph, &c)
			}
		case predicateTriplexKind:
			incExtraCount(txp, owningGraph, &c)
		default:
			panic(k)
		}
	}
	sort.SliceStable(
		triplexes[0:c.termCollectionMemberCount+c.nonTermCollectionIBNodeTripleCount+c.nonTermCollectionLiteralTripleCount+c.nonTermCollectionLiteralCollectionCount],
		triplexComparator(triplexes),
	)
	return
}

func (t *ibNode) sortAndCountTriplesForSSTWrite(gc *graphWritingContext) (c ibNodeWriteCounts) {
	if t.triplexEnd-t.triplexStart == 0 {
		return
	}
	triplexes := t.ng.triplexStorage[int(t.triplexStart):int(t.triplexEnd)]
	sort.Sort((*ibNode)(t))
	owningGraph := t.ng
	for i, tx := range triplexes {
		txp := tx.p
		if txp == nil {
			break
		}
		switch k := t.triplexKindAt(i); k {
		case subjectTriplexKind:
			incRefCountForSSTWrite(txp, owningGraph, gc)
			if txp.Is(rdfFirst) {
				c.termCollectionMemberCount++ // if sub is TermCollection, this will record the number of its members
			}
			// determined by the object type
			switch resourceTypeRecursive(tx.t) {
			// if object is type of IBNode
			case resourceTypeIBNode:
				if !txp.Is(rdfFirst) {
					c.nonTermCollectionIBNodeTripleCount++ // if sub is not a TermCollection, this will record the number of its predicates
				}
				incRefCountForSSTWrite(tx.t.asIBNode(), owningGraph, gc)

			// if object if type of Literal
			case resourceTypeLiteral:
				if txp.Is(rdfFirst) {
					c.termCollectionLiteralMemberCount++ // if sub is TermCollection, this will record the number of its literal members
				} else {
					c.nonTermCollectionLiteralTripleCount++ // if sub is not a TermCollection, this will record the number of its literal members
				}

			// if object is type of LiteralCollection
			case resourceTypeLiteralCollection:
				if txp.Is(rdfFirst) {
					c.termCollectionLiteralMemberCount++ // if sub is TermCollection, this will record the number of its literal members
				} else {
					c.nonTermCollectionLiteralCollectionCount++ // if sub is not TermCollection, this will record the number of its literal collection members
				}
			}
		case objectTriplexKind:
			if isResourceTypeIBNode(tx.t) {
				incExtraCountForSSTWrite(tx.t.asIBNode(), owningGraph, &c)
			}
		case predicateTriplexKind:
			incExtraCountForSSTWrite(txp, owningGraph, &c)
		default:
			panic(k)
		}
	}
	sort.SliceStable(
		triplexes[0:c.termCollectionMemberCount+c.nonTermCollectionIBNodeTripleCount+c.nonTermCollectionLiteralTripleCount+c.nonTermCollectionLiteralCollectionCount],
		triplexComparator(triplexes),
	)
	return
}

func incRefCountForSSTWrite(t *ibNode, owningGraph *namedGraph, gc *graphWritingContext) {
	if t.ng == owningGraph {
		// intern nodes, do not record here
		return
	} else if _, found := owningGraph.directImports[t.ng.id]; found {
		id := t.ng.id
		importedGraphContext, found := gc.importedNodeContexts[id]
		if !found {
			importedGraphContext = writingNodeContext{nodeCountIndexMap: map[IBNode]countAndIndex{}}
			gc.importedNodeContexts[id] = importedGraphContext
		}
		importedGraphContext.nodeCountIndexMap[t] = countAndIndex{count: importedGraphContext.nodeCountIndexMap[t].count + 1}
	} else {
		var referencedGraph NamedGraph
		// if t.ng != &staticRdfGraph {
		referencedGraph = t.OwningGraph()
		// } else {
		// 	referencedGraph = sstBaseReferencedGraph()
		// }
		baseURI := referencedGraph.IRI()
		referencedGraphContext, found := gc.referencedGraphNodeContexts[baseURI]
		if !found {
			referencedGraphContext = writingNodeContext{nodeCountIndexMap: map[IBNode]countAndIndex{}}
			gc.referencedGraphNodeContexts[baseURI] = referencedGraphContext
		}

		for ib := range referencedGraphContext.nodeCountIndexMap {
			if ib.IsUuidFragment() {

			} else {
				if ib.Fragment() == t.Fragment() && t != ib {
					panic("in one NamedGraph, there are different IBNodes have same fragment")
				}
			}
		}
		referencedGraphContext.nodeCountIndexMap[t] = countAndIndex{count: referencedGraphContext.nodeCountIndexMap[t].count + 1}

		found = false
		for _, ng := range gc.sortedReferencedGraphs {
			if ng == referencedGraph {
				found = true
				break
			}
		}
		if !found {
			gc.sortedReferencedGraphs = append(gc.sortedReferencedGraphs, referencedGraph)
		}
	}

}

func incExtraCountForSSTWrite(t *ibNode, owningGraph *namedGraph, c *ibNodeWriteCounts) {
	if t.ng == owningGraph {
		c.extraCount++
	}
}

func incRefCount(t *ibNode, owningGraph *namedGraph, c ibNodeWriteIncs) {
	var graph *namedGraph
	if t.ng != nil || len(t.ng.triplexStorage) != 0 {
		graph = t.ng
	}
	if t.ng == nil || len(t.ng.triplexStorage) == 0 {
		et := t
		c.incExternalRefCount(et)
	} else {
		if _, found := owningGraph.directImports[graph.id]; found {
			if graph != owningGraph {
				gt := t
				c.incImportedRefCount(gt)
			}
		} else {
			et := t
			c.incExternalRefCount(et)
		}
	}
}

func incExtraCount(t *ibNode, owningGraph *namedGraph, c *ibNodeWriteCounts) {
	if t.ng.triplexStorage != nil && t.ng == owningGraph {
		c.extraCount++
	}
}

// sort triplexes
func triplexComparator(triplexes []triplex) func(i int, j int) bool {
	return func(i, j int) bool {
		h1, h2 := triplexes[i], triplexes[j]

		// For TermCollection IBNodes
		// Put rdf:first triples first and keep their relative order unchanged
		// p1 o1
		// p2 o2
		// if p1 is rdf:first, p2 is also rdf:first, return false
		// if p1 is rdf:first, p2 is not rdf:first, return true
		if h1.p.Is(rdfFirst) {
			return !h2.p.Is(rdfFirst)
		}
		// if p1 is not rdf:first, p2 is rdf:first, return false
		if h2.p.Is(rdfFirst) {
			return false
		}

		// For non-TermCollection IBNodes
		// According to the resource type of object, sort by IBNode, Literal, LiteralCollection
		rt1, rt2 := resourceTypeRecursive(h1.t), resourceTypeRecursive(h2.t)
		switch rtCmp := int(rt1) - int(rt2); {
		case rtCmp < 0:
			return true
		case rtCmp > 0:
			return false
		}

		// If resource type of object are the same, sort by predicate IRI
		switch pCmp := compareIBNodes(h1.p, h2.p); {
		case pCmp < 0:
			return true
		case pCmp > 0:
			return false
		}

		// if resource type of object and predicate IRI both are the same, based on the resource type of object, sort by object value
		switch rt1 {
		case resourceTypeIBNode:
			return compareIBNodes(h1.t.asIBNode(), h2.t.asIBNode()) < 0
		case resourceTypeLiteral:
			return compareLiterals(h1.t.asLiteral(), h2.t.asLiteral()) < 0
		case resourceTypeLiteralCollection:
			return compareLiteralCollections(h1.t.asLiteralCollection(), h2.t.asLiteralCollection()) < 0
		}
		panicf("unhandled resource type %v in triplexComparator", rt1)
		panic("unreachable")
	}
}

func isEqualTerm(o1, o2 Term) bool {
	k1 := o1.TermKind()
	if k1 != o2.TermKind() {
		return false
	}
	switch k1 {
	case TermKindIBNode, TermKindTermCollection:
		return compareIBNodes(o1.(IBNode), o2.(IBNode)) == 0
	case TermKindLiteral:
		return compareLiterals(o1.(Literal), o2.(Literal)) == 0
	case TermKindLiteralCollection:
		return compareLiteralCollections(o1.(*literalCollection), o2.(*literalCollection)) == 0
	}
	panic(k1)
}

func compareLiterals(l1Interface, l2Interface Literal) int {
	l1 := objectToLiteral(l1Interface)
	l2 := objectToLiteral(l2Interface)
	switch typeCmp := compareIBNodes(l1.typeOf, l2.typeOf); {
	case typeCmp < 0:
		return -1
	case typeCmp > 0:
		return 1
	}
	return strings.Compare(l1.value, l2.value)
}

func compareLiteralCollections(c1, c2 *literalCollection) int {
	switch dataTypeCmp := compareIBNodes(c1.dataType, c2.dataType); {
	case dataTypeCmp < 0:
		return -1
	case dataTypeCmp > 0:
		return 1
	}
	if len(c1.members) <= len(c2.members) {
		for i, m := range c1.members {
			if m < c2.members[i] {
				return -1
			}
		}
		if len(c1.members) == len(c2.members) {
			return 0
		}
		return 1
	}
	for i, m := range c2.members {
		if c1.members[i] < m {
			return -1
		}
	}
	return 1
}

// ib1 and ib2 should be both either ibNodeString or ibNodeUuid.
func lessIBNodeThan(gc *graphWritingContext, ib1, ib2 IBNode) bool {
	if ib1.IsIRINode() && ib2.IsIRINode() {
		switch ib1 := ib1.ibNodeType().(type) {
		case *ibNodeString:
			ib2, ok := ib2.ibNodeType().(*ibNodeString)
			if !ok {
				return true
			}
			return ib1.fragment < ib2.fragment
		case *ibNodeUuid:
			ib2, ok := ib2.ibNodeType().(*ibNodeUuid)
			if !ok {
				return false
			}
			return bytes.Compare(ib1.id[:], ib2.id[:]) < 0
		}
	} else if ib1.IsIRINode() && ib2.IsBlankNode() {
		return true
	} else if ib1.IsBlankNode() && ib2.IsIRINode() {
		return false
	} else if ib1.IsBlankNode() && ib2.IsBlankNode() {
		switch ib1 := ib1.ibNodeType().(type) {
		case *ibNodeUuid:
			ib2, ok := ib2.ibNodeType().(*ibNodeUuid)
			if !ok {
				return false
			}
			if gc == nil {
				return bytes.Compare(ib1.id[:], ib2.id[:]) < 0
			} else {
				return gc.blankNodeSubPred[ib1.ID()] < gc.blankNodeSubPred[ib2.ID()]
			}
		}
	}

	panicf("unexpected IBNode type %T", ib1)
	panic("unreachable")
}

func asIBNode(IB IBNode) *ibNode {
	var ib *ibNode
	switch ib1Impl := IB.(type) {
	case *ibNode:
		ib = ib1Impl
	case *ibNodeString:
		ib = &ib1Impl.ibNode
	case *ibNodeUuid:
		ib = &ib1Impl.ibNode
	default:
		panicf("unexpected IBNode type %T", ib1Impl)
	}
	return ib
}

// put rdf:type first, then sort by baseIRI, then by fragment or UUID
func compareIBNodes(ibNode1, ibNode2 IBNode) int {
	ib1 := asIBNode(ibNode1)
	ib2 := asIBNode(ibNode2)

	if ib1.isVocabularyElement(rdfType) {
		if ib2.isVocabularyElement(rdfType) {
			return 0
		}
		return -1
	}
	if ib2.isVocabularyElement(rdfType) {
		return 1
	}
	if ib1.ng == nil {
		if ib2.ng == nil {
			return compareIBNodeFragments(ib1, ib2)
		}
		return -1
	}
	if ib2.ng == nil {
		return 1
	}
	switch bCmp := strings.Compare(ib1.ng.baseIRI, ib2.ng.baseIRI); {
	case bCmp < 0:
		return -1
	case bCmp > 0:
		return 1
	}
	return compareIBNodeFragments(ib1, ib2)
}

func compareIBNodeFragments(ib1 *ibNode, ib2 *ibNode) int {
	switch ib1 := ib1.ibNodeType().(type) {
	case *ibNodeString:
		return compareIBNodeString(ib1, ib2)
	case *ibNodeUuid:
		return compareIBNodeUuid(ib1, ib2)
	default:
		panic(ib1)
	}
}

func compareIBNodeString(ib1 *ibNodeString, ib2 *ibNode) int {
	switch ib2 := ib2.ibNodeType().(type) {
	case *ibNodeString:
		return strings.Compare(string(ib1.fragment), string(ib2.fragment))
	case *ibNodeUuid:
		return -1
	default:
		panic(ib2)
	}
}

func compareIBNodeUuid(ib1 *ibNodeUuid, ib2 *ibNode) int {
	switch s2 := ib2.ibNodeType().(type) {
	case *ibNodeString:
		return 1
	case *ibNodeUuid:
		return bytes.Compare(ib1.id[:], s2.id[:])
	default:
		panic(s2)
	}
}

func (t *ibNode) forMemberNodeRange(fromTriples, toTriple int, c func(p IBNode, o IBNode) error) error {
	rangeEnd := int(t.triplexStart) + toTriple
	if rangeEnd > int(t.triplexEnd) {
		panic("invalid range")
	}
	triplexes := t.ng.triplexStorage[int(t.triplexStart)+fromTriples : rangeEnd]
	if len(triplexes) != 0 && t.typeOf != &termCollectionResourceType.ibNode {
		GlobalLogger.Debug("iriOrID", zap.String("value", t.iriOrID()))
		panic("TermCollection expected")
	}
	for _, tx := range triplexes {
		switch resourceTypeRecursive(tx.t) {
		case resourceTypeIBNode:
			err := c(tx.p, tx.t.asIBNode())
			if err != nil {
				return err
			}
		case resourceTypeLiteral, resourceTypeLiteralCollection:
			err := c(tx.p, tx.p)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *ibNode) forPropNodeRange(fromTriple, toTriple int, writeIBNodePO func(p IBNode, o IBNode) error) error {
	rangeEnd := int(t.triplexStart) + toTriple
	if rangeEnd > int(t.triplexEnd) {
		panic("invalid range")
	}
	triplexes := t.ng.triplexStorage[int(t.triplexStart)+fromTriple : rangeEnd]
	for _, tx := range triplexes {
		if resourceTypeRecursive(tx.t) != resourceTypeIBNode {
			panic("invalid range")
		}
		err := writeIBNodePO(tx.p, tx.t.asIBNode())
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *ibNode) forMemberLiteralRange(fromTriple, toTriple, expectCnt int, c func(predicate IBNode, object Term) error) error {
	if expectCnt == 0 {
		return nil
	}
	if t.typeOf != &termCollectionResourceType.ibNode {
		panic("TermCollection expected")
	}
	rangeEnd := int(t.triplexStart) + toTriple
	if rangeEnd > int(t.triplexEnd) {
		panic("invalid range")
	}
	triplexes := t.ng.triplexStorage[int(t.triplexStart)+fromTriple : rangeEnd]
	var actualCnt int
	for _, tx := range triplexes {
		switch resourceTypeRecursive(tx.t) {
		case resourceTypeLiteral, resourceTypeLiteralCollection:
			err := c(tx.p, triplexToObject(tx))
			if err != nil {
				return err
			}
			actualCnt++
			if actualCnt == expectCnt {
				return nil
			}
		case resourceTypeIBNode:
		}
	}
	return nil
}

func (t *ibNode) forPropLiteralRange(fromTriple, toTriple int, c func(p IBNode, o Literal) error) error {
	rangeEnd := int(t.triplexStart) + toTriple
	if rangeEnd > int(t.triplexEnd) {
		panic("invalid range")
	}
	triplexes := t.ng.triplexStorage[int(t.triplexStart)+fromTriple : rangeEnd]
	for _, tx := range triplexes {
		if resourceTypeRecursive(tx.t) != resourceTypeLiteral {
			panic("invalid range")
		}
		err := c(tx.p, literalToObject(tx.t.asLiteral()).(Literal))
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *ibNode) forPropLiteralCollectionRange(fromTriple, toTriple int, c func(p IBNode, o *literalCollection) error) error {
	rangeEnd := int(t.triplexStart) + toTriple
	if rangeEnd > int(t.triplexEnd) {
		panic("invalid range")
	}
	triplexes := t.ng.triplexStorage[int(t.triplexStart)+fromTriple : rangeEnd]
	for _, tx := range triplexes {
		if resourceTypeRecursive(tx.t) != resourceTypeLiteralCollection {
			panic("invalid range")
		}
		err := c(tx.p, tx.t.asLiteralCollection())
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *ibNode) appendUniqueNodeTriple(p IBNode, o IBNode) {
	var subjectString string
	switch t := t.ibNodeType().(type) {
	case *ibNodeString:
		subjectString = t.IRI().String()
	case *ibNodeUuid:
		subjectString = t.asUuidIBNode().id.String()
	}

	var objectString string
	switch o := o.ibNodeType().(type) {
	case *ibNodeString:
		objectString = o.IRI().String()
	case *ibNodeUuid:
		objectString = o.asUuidIBNode().id.String()
	}

	GlobalLogger.Debug("appendUniqueNodeTriple", zap.String("sub", subjectString), zap.String("pred", p.iriOrID()), zap.String("obj", objectString))

	if p.Is(rdfFirst) {
		t.typeOf = &termCollectionResourceType.ibNode
		if o.Is(rdfFirst) {
			appendTriplex(t, triplex{p: p.(*ibNode), t: nil}, subjectTriplexKind)
			return
		}
	}
	appendTriplex(t, triplex{p: p.(*ibNode), t: &o.(*ibNode).typedResource}, subjectTriplexKind)
	appendTriplex(o.(*ibNode), triplex{p: p.(*ibNode), t: &t.typedResource}, objectTriplexKind)
	if p.arePredicateTriplexesTracked() {
		appendTriplex(p.(*ibNode), triplex{p: t, t: &o.(*ibNode).typedResource}, predicateTriplexKind)
	}
}

func (t *ibNode) appendUniqueLiteralTriple(p IBNode, o Literal) {
	var subjectString string
	switch t := t.ibNodeType().(type) {
	case *ibNodeString:
		subjectString = t.IRI().String()
	case *ibNodeUuid:
		subjectString = t.asUuidIBNode().id.String()
	}

	objectString := literalToString(o)

	GlobalLogger.Debug("appendUniqueLiteralTriple", zap.String("sub", subjectString), zap.String("pred", p.IRI().String()), zap.String("obj", objectString))

	literal := objectToLiteral(o)
	if p.Is(rdfFirst) {
		fillRdfFirstPlaceholder(t, &literal.typedResource)
		return
	}
	appendTriplex(t, triplex{p: p.(*ibNode), t: &literal.typedResource}, subjectTriplexKind)
	if p.arePredicateTriplexesTracked() {
		appendTriplex(p.(*ibNode), triplex{p: t, t: &literal.typedResource}, predicateTriplexKind)
	}
}

func (t *ibNode) appendUniqueLiteralCollectionTriple(p IBNode, c LiteralCollection) {
	var subjectString string
	switch t := t.ibNodeType().(type) {
	case *ibNodeString:
		subjectString = t.IRI().String()
	case *ibNodeUuid:
		subjectString = t.asUuidIBNode().id.String()
	}

	var objectString string
	objectString += "( "
	c.ForMembers(func(index int, li Literal) {
		objectString += literalToString(li) + " "
	})
	objectString += ")"

	GlobalLogger.Debug("appendUniqueLiteralTriple", zap.String("sub", subjectString), zap.String("pred", p.IRI().String()), zap.String("obj", objectString))

	if p.Is(rdfFirst) {
		fillRdfFirstPlaceholder(t, &c.(*literalCollection).typedResource)
		return
	}
	appendTriplex(t, triplex{p: p.(*ibNode), t: &c.(*literalCollection).typedResource}, subjectTriplexKind)
	if p.arePredicateTriplexesTracked() {
		appendTriplex(p.(*ibNode), triplex{p: t, t: &c.(*literalCollection).typedResource}, predicateTriplexKind)
	}
}

// fillRdfFirstPlaceholder searches for an rdf:first placeholder triplex (where tx.p is rdf:first
// and tx.t is nil) in the given ibNode and fills it with obj. It panics if no placeholder is found.
func fillRdfFirstPlaceholder(t *ibNode, obj *typedResource) {
	triplexes := t.ng.triplexStorage[t.triplexStart:t.triplexEnd]
	for i, tx := range triplexes {
		if tx.p.Is(rdfFirst) && tx.t == nil {
			rdfNg := t.ng.stage.referencedGraphByURI("http://www.w3.org/1999/02/22-rdf-syntax-ns")
			rdfFirstNode := rdfNg.GetIRINodeByFragment("first")
			if rdfFirstNode == nil {
				rdfFirstNode = rdfNg.CreateIRINode("first")
			}
			var rdfFirstIBNode *ibNode
			switch rf := rdfFirstNode.(type) {
			case *ibNodeString:
				rdfFirstIBNode = &rf.ibNode
			case *ibNode:
				rdfFirstIBNode = rf
			default:
				panic(fmt.Sprintf("unexpected rdfFirst type %T", rdfFirstNode))
			}
			triplexes[i] = triplex{p: rdfFirstIBNode, t: obj}
			return
		}
	}
	panic("rdf:first placeholder not found for term collection member")
}

type termCollectionSortByMembers ibNode

// Len implements sort.Interface.Len() method.
func (c *termCollectionSortByMembers) Len() int {
	return int(c.triplexEnd - c.triplexStart)
}

// Less implements sort.Interface.Less() method.
func (c *termCollectionSortByMembers) Less(i, j int) bool {
	hs := c.ng.triplexStorage
	h1, h2 := hs[int(c.triplexStart)+i], hs[int(c.triplexStart)+j]
	if h1.p.Is(rdfFirst) && (*ibNode)(c).triplexKindAt(i) == subjectTriplexKind {
		return !h2.p.Is(rdfFirst) || (*ibNode)(c).triplexKindAt(j) != subjectTriplexKind
	}
	return false
}

// Swap implements sort.Interface.Swap() method.
func (c *termCollectionSortByMembers) Swap(i, j int) {
	swapTriplexes((*ibNode)(c), i, j)
}

func swapTriplexes(t *ibNode, i int, j int) {
	namedGraph, io, jo := t.ng, t.triplexStart+triplexOffset(i), t.triplexStart+triplexOffset(j)
	namedGraph.triplexStorage[io], namedGraph.triplexStorage[jo] = namedGraph.triplexStorage[jo], namedGraph.triplexStorage[io]
	kindI := triplexKindAtAbs(namedGraph, io)
	setTriplexKindAbs(namedGraph, io, triplexKindAtAbs(namedGraph, jo))
	setTriplexKindAbs(namedGraph, jo, kindI)
}

func (t *ibNode) extractCollectionMembers() {
	if t != nil && t.typeOf == &termCollectionResourceType.ibNode {
		sort.Stable((*termCollectionSortByMembers)(t))
	}
}
