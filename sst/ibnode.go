// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/bits"
	"sort"
	"strconv"
	"unsafe"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type (
	// TermKind is kind of triple object witch is either [TermKindIBNode], [TermKindLiteral],
	// [TermKindLiteralCollection] or [TermKindTermCollection].
	// These values are returned from the TermKind() method of all types implementing the [Object] interface.
	TermKind = int
)

// TermKindIBNode, TermKindLiteral, TermKindLiteralCollection and TermKindTermCollection
// denote respectively Object of either [IBNode], [Literal], [LiteralCollection] or [TermCollection] type.
// These values are returned from the TermKind() method of all types implementing the [Term] interface.
const (
	TermKindIBNode = TermKind(iota)
	TermKindLiteral
	TermKindLiteralCollection
	TermKindTermCollection
)

// IsKindIBNode returns true if given object is of IBNode kind.
// This is a convenience method equal to
//
//	o.TermKind() == sst.TermKindIBNode
func IsKindIBNode(o interface{ TermKind() TermKind }) bool { return o.TermKind() == TermKindIBNode }

// ForEachNode is a callback function that is invoked from IBNode iterating functions.
// Returning non nil error from this function indicates that iteration loop should be terminated.
// This type is used by NamedGraph method `NamedGraph.ForAllIBNodes` .
type ForEachNode func(d IBNode) error

var (
	ErrNodeTripleNotSet                   = errors.New("node triple not set")
	ErrTriplePredicateNotSet              = errors.New("triple predicate not set")
	ErrTripleObjectNotSet                 = errors.New("triple object not set")
	ErrDuplicateTriplex                   = errors.New("duplicate triplex")
	ErrNodeNotModifiable                  = errors.New("node not modifiable")
	ErrNodeDeletedOrInvalid               = errors.New("node deleted or invalid")
	ErrStatementsNotSupported             = errors.New("statements not supported")
	ErrLiteralDataTypesDiffer             = errors.New("literal data types differ")
	ErrCannotSetCollectionMemberPredicate = errors.New("can not set collection member predicate")
	ErrIndexOutOfRange                    = errors.New("index out of range")

	errBreakFor = errors.New("break for")
)

type (
	triplexOffset uint32
)

const (
	triplexAllocCount       = 6
	triplexMinGrow          = 64
	triplesMaxGrowThreshold = 8096
)

// The flag trackPredicates means that there will be triples where predicates of IBNode in this NamedGraph
// are tracked by triplex of predicateTriplexKind; by default predicateTriplexKind are not used.
type namedGraphFlags struct {
	isReferenced    bool
	trackPredicates bool
	modified        bool
	deleted         bool
}

func copyTriplexesAndImports(to, from *namedGraph) triplexOffset {
	offset := len(to.triplexStorage)
	to.triplexStorage = append(to.triplexStorage, from.triplexStorage...)
	to.triplexKinds = appendTriplexKinds(to.triplexKinds, offset<<(triplexKindBitCount-1), from.triplexKinds, 0, uintAllMask)
	// add "from" NG directImports to "to" NG
	for k, v := range from.directImports {
		to.directImports[k] = v
	}

	for k, v := range from.isImportedBy {
		to.isImportedBy[k] = v
	}

	return triplexOffset(offset)
}

func copyNodeToNamedGraph(to *namedGraph, t *ibNode) {
	toOffset := len(to.triplexStorage)
	fromOffset := toOffset - int(t.triplexStart)
	triplexes := t.ng.triplexStorage[t.triplexStart:t.triplexEnd]
	to.triplexStorage = append(to.triplexStorage, triplexes...)
	triplexKindStart, triplexKindEnd := t.triplexStart>>triplexKindShiftOffset, t.triplexEnd>>triplexKindShiftOffset
	var fromEndMask uint
	if t.triplexEnd&triplexKindShiftMask != 0 {
		shift := (t.triplexEnd & triplexKindShiftMask) << (triplexKindBitCount - 1)
		fromEndMask = 1<<shift - 1
		triplexKindEnd++
	} else {
		fromEndMask = uintAllMask
	}
	if triplexKindStart > triplexOffset(len(t.ng.triplexKinds)) {
		triplexKindStart = triplexOffset(len(t.ng.triplexKinds))
	}
	if triplexKindEnd > triplexOffset(len(t.ng.triplexKinds)) {
		triplexKindEnd = triplexOffset(len(t.ng.triplexKinds))
		fromEndMask = uintAllMask
	}
	to.triplexKinds = appendTriplexKinds(
		to.triplexKinds,
		toOffset<<(triplexKindBitCount-1),
		t.ng.triplexKinds[triplexKindStart:triplexKindEnd],
		uint(t.triplexStart<<(triplexKindBitCount-1))&uintShiftMask,
		fromEndMask,
	)
	t.ng = to
	t.triplexStart = triplexOffset(int(t.triplexStart) + fromOffset)
	t.triplexEnd = triplexOffset(int(t.triplexEnd) + fromOffset)
}

type (
	fromMatchFunc        func(s *ibNode) bool
	namedGraphsAndOffset struct {
		c      *namedGraph
		offset triplexOffset
	}
	nodeCopyReplacements map[*ibNode]*ibNode
)

func nodeMoveReplacements() nodeCopyReplacements { return nil }

func (r nodeCopyReplacements) mergeNodeToNamedGraph(
	s *ibNode,
	toC map[*namedGraph]namedGraphsAndOffset,
	fromMatch fromMatchFunc,
	fragment string,
) *ibNode {
	if fragment != "" {
		toCNamedGraph := toC[s.ng].c
		if toCNamedGraph == nil {
			toCNamedGraph = s.ng
		}
		if existing := toCNamedGraph.GetIRINodeByFragment(fragment); existing != nil {
			r.mergeNodes(existing.(*ibNode), s, toC, fromMatch)
			return existing.(*ibNode)
		}
	}
	t := r.reTargetNodeToNamedGraph(s, toC, fromMatch)
	if et, ok := t.ibNodeType().(*ibNodeString); ok {
		err := et.setFragment(fragment)
		if err != nil {
			panic(err)
		}
	}
	switch et := t.ibNodeType().(type) {
	case *ibNodeString:
		err := et.setFragment(fragment)
		if err != nil {
			panic(err)
		}
	case *ibNodeUuid:
		if et.flags&nodeTypeBitMask == blankNodeType {
		} else {
			et.id = uuid.MustParse(fragment)
		}

	}

	return t
}

func (r nodeCopyReplacements) mergeNodes(
	to, from *ibNode,
	toC map[*namedGraph]namedGraphsAndOffset,
	fromMatch fromMatchFunc,
) {
	mFrom := from
	if r != nil {
		prevTo := r[from]
		if prevTo != nil && prevTo != to {
			r.replaceNode(prevTo, toC, fromMatch)
			mFrom = prevTo
		}
		r[mFrom] = to
		r[from] = to
		r[to] = to
	}
	if err := mFrom.forAllTriplexes(func(i int, tx triplex, k triplexKind) (_ triplex, err error) {
		var tryIndexIBNodeVocabulary bool
		switch k {
		case subjectTriplexKind:
			var newP *ibNode
			mTxt := tx.t
			tType := resourceTypeRecursive(tx.t)
			switch tType {
			case resourceTypeIBNode:
				s := r.reTargetNodeToNamedGraph(tx.t.asIBNode(), toC, fromMatch)
				if s != to {
					newP, _ = r.maybeReplacePredicate(tx.p, toC, fromMatch)
					replaceOneOfTriplexes(s,
						tx.p, newP, &mFrom.typedResource, &from.typedResource,
						triplex{p: newP, t: &to.typedResource},
						objectTriplexKind,
					)
					if s == from {
						s = to
					}
				}
				mTxt = &s.typedResource
				tryIndexIBNodeVocabulary = true
			case resourceTypeLiteral, resourceTypeLiteralCollection:
			}
			if newP == nil {
				newP, _ = r.maybeReplacePredicate(tx.p, toC, fromMatch)
			}
			if newP != to && newP.arePredicateTriplexesTracked() {
				switch tType {
				case resourceTypeIBNode:
					replaceOneOfTriplexes(newP,
						mFrom, from, tx.t, mTxt,
						triplex{p: to, t: mTxt},
						predicateTriplexKind,
					)
				case resourceTypeLiteral:
					replaceOneOfTriplexesByValue(
						newP, mFrom, from, to, tx.t.asLiteral(), predicateTriplexKind,
					)
				case resourceTypeLiteralCollection:
					replaceOneOfTriplexesByValue(
						newP, mFrom, from, to, tx.t.asLiteralCollection(), predicateTriplexKind,
					)
				}
			}
		case objectTriplexKind:
			s := r.reTargetNodeToNamedGraph(tx.t.asIBNode(), toC, fromMatch)
			var newP *ibNode
			if s != to {
				newP, _ = r.maybeReplacePredicate(tx.p, toC, fromMatch)
				replaceOneOfTriplexes(s,
					tx.p, newP, &mFrom.typedResource, &from.typedResource,
					triplex{p: newP, t: &to.typedResource},
					subjectTriplexKind,
				)
				if s.typeOf == mFrom || s.typeOf == from {
					s.typeOf = to
				}
			}
			if newP == nil {
				newP, _ = r.maybeReplacePredicate(tx.p, toC, fromMatch)
			}
			if newP != to && newP.arePredicateTriplexesTracked() {
				if s == from {
					s = to
				}
				replaceOneOfTriplexes(newP,
					tx.t.asIBNode(), s, &mFrom.typedResource, &from.typedResource,
					triplex{p: s, t: &to.typedResource},
					predicateTriplexKind,
				)
			}
		case predicateTriplexKind:
			if to.arePredicateTriplexesTracked() {
				s := r.reTargetNodeToNamedGraph(tx.p, toC, fromMatch)
				if s == from {
					s = to
				}
				switch tType := resourceTypeRecursive(tx.t); tType {
				case resourceTypeIBNode:
					o := r.reTargetNodeToNamedGraph(tx.t.asIBNode(), toC, fromMatch)
					if o == from {
						o = to
					}
					replaceOneOfTriplexes(s,
						mFrom, from, tx.t, &o.typedResource,
						triplex{p: to, t: &o.typedResource},
						subjectTriplexKind,
					)
					replaceOneOfTriplexes(o,
						mFrom, from, &tx.p.typedResource, &s.typedResource,
						triplex{p: to, t: &s.typedResource},
						objectTriplexKind,
					)
				case resourceTypeLiteral:
					replaceOneOfTriplexesByValue(
						s, mFrom, from, to, tx.t.asLiteral(), subjectTriplexKind,
					)
				case resourceTypeLiteralCollection:
					replaceOneOfTriplexesByValue(
						s, mFrom, from, to, tx.t.asLiteralCollection(), subjectTriplexKind,
					)
				}
			}
		}
		addHT, _ := r.tripleReplacer(nil, to, from, toC, fromMatch)(i, tx, k)
		if addHT.p != nil {
			err = assertUniqueTriplex(to, 0, addHT, k)
			if err == nil {
				addTriplexAtOrAfter(to, 0, addHT, k)
			}
			if tryIndexIBNodeVocabulary {
				indexIBNodeVocabulary(to, addHT.p, addHT.t.asIBNode())
			}
		} else {
			err = assertUniqueTriplex(to, 0, tx, k)
			if err == nil {
				addTriplexAtOrAfter(to, 0, tx, k)
			}
			if tryIndexIBNodeVocabulary {
				indexIBNodeVocabulary(to, tx.p, tx.t.asIBNode())
			}
		}
		if errors.Is(err, ErrDuplicateTriplex) {
			err = nil
		}
		return
	}); err != nil {
		panic(err)
	}
}

func assertUniqueTriplex(t *ibNode, index triplexOffset, tx triplex, kind triplexKind) error {
	for i := t.triplexStart + index; i < t.triplexEnd; i++ {
		tTx := t.ng.triplexStorage[i]
		if tTx.p == nil || triplexKindAtAbs(t.ng, i) != kind {
			continue
		}
		if triplexesEqual(tTx, tx, false) {
			return ErrDuplicateTriplex
		}
	}
	return nil
}

func (r nodeCopyReplacements) copyRemainingNodes(toC map[*namedGraph]namedGraphsAndOffset, fromMatch fromMatchFunc) {
	replaced := true
	for len(r) > 0 && replaced {
		replaced = false
		for from, to := range r {
			if fromMatch(from) && toC[to.ng].c != nil {
				r.replaceNode(to, toC, fromMatch)
				replaced = true
			}
		}
	}
}

func (r nodeCopyReplacements) maybeClearGraphs(toC map[*namedGraph]namedGraphsAndOffset) {
	if r == nil {
		for c := range toC {
			c.clearGraph()
		}
	}
}

func (r nodeCopyReplacements) reTargetNodeToNamedGraph(
	s *ibNode,
	toC map[*namedGraph]namedGraphsAndOffset,
	fromMatch fromMatchFunc,
) *ibNode {
	t, _ := r.maybeReplaceNode(s, fromMatch)
	r.replaceNode(t, toC, fromMatch)
	return t
}

func (r nodeCopyReplacements) replaceNode(t *ibNode, toC map[*namedGraph]namedGraphsAndOffset, fromMatch fromMatchFunc) {
	if replaced := replaceIBNodeNamedGraph(t, toC); !replaced {
		return
	}
	t.typeOf, _ = r.maybeReplaceNode(t.typeOf, fromMatch)
	if err := t.forAllTriplexes(r.tripleReplacer(t, nil, nil, toC, fromMatch)); err != nil {
		panic(err)
	}
}

func replaceIBNodeNamedGraph(t *ibNode, toC map[*namedGraph]namedGraphsAndOffset) bool {
	toCOffset := toC[t.ng]
	if toCOffset.c == nil {
		return false
	}
	if t.triplexEnd != t.triplexStart {
		t.triplexStart += toCOffset.offset
		t.triplexEnd += toCOffset.offset
	}
	t.ng = toCOffset.c
	return true
}

// tripleReplacer returns triple replacing function.
// When to and from parameter are not nil then it is used in
// the context of node merging. Otherwise the triplexes are replaced only.
// If self parameter is provided then node is reindexed for vocabulary.
func (r nodeCopyReplacements) tripleReplacer(
	self, to, from *ibNode,
	toC map[*namedGraph]namedGraphsAndOffset,
	fromMatch fromMatchFunc,
) func(int, triplex, triplexKind) (out triplex, err error) {
	return func(_ int, tx triplex, kind triplexKind) (out triplex, err error) {
		var outReplaced, replaced bool
		if tx.p == from {
			out.p, replaced = to, true
		} else {
			out.p, replaced = r.maybeReplacePredicate(tx.p, toC, fromMatch)
		}
		outReplaced = outReplaced || replaced
		switch rt := resourceTypeRecursive(tx.t); rt {
		case resourceTypeIBNode:
			var sTo *ibNode
			if tx.t.asIBNode() == from {
				sTo, replaced = to, true
			} else {
				sTo, replaced = r.maybeReplaceNode(tx.t.asIBNode(), fromMatch)
			}
			outReplaced = outReplaced || replaced
			if replaced {
				if sTo.typeOf != nil && sTo.typeOf == from {
					sTo.typeOf = to
				} else {
					sTo.typeOf, _ = r.maybeReplaceNode(sTo.typeOf, fromMatch)
				}
				out.t = &sTo.typedResource
			} else {
				out.t = tx.t
			}
			if self != nil && kind == subjectTriplexKind {
				o := sTo
				if !replaced {
					o = tx.t.asIBNode()
				}
				indexIBNodeVocabulary(self, out.p, o)
			}
		case resourceTypeLiteral, resourceTypeLiteralCollection:
			htTypeOf := tx.t.typeOf
			if rt == resourceTypeLiteralCollection {
				htTypeOf = tx.t.asLiteralCollection().dataType
			}
			var typeOf *ibNode
			if htTypeOf == from {
				typeOf, replaced = to, true
			} else {
				typeOf, replaced = r.maybeReplaceNode(htTypeOf, fromMatch)
			}
			outReplaced = outReplaced || replaced
			if replaced {
				if rt == resourceTypeLiteral {
					lTo := new(literal)
					lTo.typeOf = typeOf
					lTo.value = tx.t.asLiteral().value
					out.t = &lTo.typedResource

				} else {
					cFrom := tx.t.asLiteralCollection()
					cTo := literalCollection{
						typedResource: typedResource{typeOf: cFrom.typeOf},
						dataType:      typeOf,
						members:       cFrom.members,
					}
					out.t = &cTo.typedResource

				}
			} else {
				out.t = tx.t
			}
		}
		if !outReplaced {
			out.p, out.t = nil, nil
		}
		return
	}
}

func (r nodeCopyReplacements) maybeReplacePredicate(
	s *ibNode,
	toC map[*namedGraph]namedGraphsAndOffset,
	fromMatch fromMatchFunc,
) (t *ibNode, replaced bool) {
	if s.isPredefinedIBNode() {
		return s, false
	}
	toCNamedGraph := toC[s.ng].c
	if toCNamedGraph == nil {
		toCNamedGraph = s.ng
	}
	predicateTriplexesTracked := s.arePredicateTriplexesTracked()
	if !predicateTriplexesTracked && toCNamedGraph != nil {
		if es, ok := s.ibNodeType().(*ibNodeString); ok {
			if existing := toCNamedGraph.GetIRINodeByFragment(es.fragment); existing != nil {
				return existing.(*ibNode), true
			}
		}
	}
	t, replaced = r.maybeReplaceNode(s, fromMatch)
	if predicateTriplexesTracked {
		r.replaceNode(t, toC, fromMatch)
	}
	return
}

// maybeReplaceNode attempts to replace the given ibNode `s` with a copy from the nodeCopyReplacements map `r`.
// If `r` is not nil and contains a replacement for `s`, it returns the replacement node and sets `replaced` to true.
// If `r` does not contain a replacement for `s`, it checks if `s` matches the criteria defined by `fromMatch`.
// If `s` matches, it creates a copy of `s` using either `ibNodeType().copy()` or `copyIBNode()`, stores the copy in `r`,
// and returns the copy with `replaced` set to true. If `s` does not match, it returns `s` with `replaced` set to false.
// If `r` is nil, it simply returns `s` with `replaced` set to false.
//
// Parameters:
// - s: The original ibNode to be potentially replaced.
// - fromMatch: A function that determines if `s` should be replaced.
//
// Returns:
// - t: The resulting ibNode after attempting replacement.
// - replaced: A boolean indicating whether the node was replaced.
func (r nodeCopyReplacements) maybeReplaceNode(s *ibNode, fromMatch fromMatchFunc) (t *ibNode, replaced bool) {
	if r != nil {
		replaced = true
		t = r[s]
		if t == nil {
			if fromMatch(s) {
				sfh := s.ibNodeType()
				if sfh != nil {
					t = sfh.copy()
				} else {
					t = s.copyIBNode()
				}
				r[s] = t
				r[t] = t
			} else {
				t = s
				replaced = false
			}
		}
	} else {
		t = s
	}
	return
}

type resourceType uint

const (
	resourceTypeIBNode = resourceType(iota)
	resourceTypeLiteral
	resourceTypeLiteralCollection
)

// used by ibNode and literal, literalCollection
// needed primarily for triple-objects as they might be ibNode, literal, ...
// but also used for ibNode to store the main type
// for literal and literalCollection set during creation; not possible to change afterwards
// for ibNode this is set whenever the triples are changed, check if RDF column type?
type typedResource struct {
	typeOf *ibNode
}

func (r *typedResource) resourceType(recurse bool) resourceType {
	switch rt := r.typeOf; rt {
	case nil:
		return resourceTypeIBNode
	case &literalResourceType.ibNode:
		return resourceTypeLiteral
	case &literalTypeLangString.ibNode:
		return resourceTypeLiteral
	case &literalCollectionResourceType.ibNode:
		return resourceTypeLiteralCollection
	case &termCollectionResourceType.ibNode:
		return resourceTypeIBNode // termCollection is a collection of IBNodes
	default:
		if recurse { // literal of a specific type such as xsd:integerLiteral
			return rt.resourceType(false) // single recursive call
		}
		return resourceTypeIBNode
	}
}

func resourceTypeRecursive(r *typedResource) resourceType {
	if r == nil {
		panic("resourceTypeRecursive called with nil typedResource")
	}
	return r.resourceType(true)
}

func isResourceTypeIBNode(r *typedResource) bool {
	return resourceTypeRecursive(r) == resourceTypeIBNode
}

func (r *typedResource) asIBNode() *ibNode {
	return (*ibNode)(unsafe.Pointer(uintptr(unsafe.Pointer(r)) - ibNodeToResourceOffset))
}

func (r *typedResource) asLiteral() *literal {
	return (*literal)(unsafe.Pointer(uintptr(unsafe.Pointer(r)) - literalToResourceOffset))
}

func (r *typedResource) asLiteralCollection() *literalCollection {
	return (*literalCollection)(unsafe.Pointer(uintptr(unsafe.Pointer(r)) - literalCollectionToResourceOffset))
}

type comparableResource[V any] interface {
	*literal | *literalCollection
	fromTypedResourcePtr(*typedResource) V
	isEqualTo(V) bool
}

type valueResource[V any] interface {
	comparableResource[V]
	typedResourcePtr() *typedResource
	validResourceType() resourceType
}

type ibNodeFlag uint8

const (
	// if last bit is set then this node is uuidNode
	structTypeBitMask ibNodeFlag = 0b00000001
	stringNode                   = ibNodeFlag(0b00000000)
	uuidNode                     = ibNodeFlag(0b00000001)

	// if second last bit is set then this node is blank node
	nodeTypeBitMask ibNodeFlag = 0b00000010
	blankNodeType              = ibNodeFlag(0b00000010)
	iriNodeType                = ibNodeFlag(0b00000000)

	ibNodeFlagModified = ibNodeFlag(1 << iota)
)

type (
	// ibNode is a IRI, blank node or a literal type (if namedGraph is staticLiteralGraph).
	ibNode struct {
		typedResource
		ng           *namedGraph
		triplexStart triplexOffset // inclusive start index in namedGraph.triplexStorage and namedGraph.triplexKinds
		triplexEnd   triplexOffset // exclusive end index in namedGraph.triplexStorage and namedGraph.triplexKinds
		flags        ibNodeFlag
	}

	ibNodeString struct {
		ibNode
		fragment string
	}

	ibNodeUuid struct {
		ibNode
		id uuid.UUID
	}

	// ibNodeType represents IBNode specialized type.
	ibNodeType interface {
		iri() IRI
		fragmentComponent() string
		copy() *ibNode
	}
)

// Node represents an interface for nodes that can be either a TermCollection or an IBNode.
type Node interface {
	Term
	// IsTermCollection returns true if the IBNode is a term collection.
	// In this case, the IBNode can be converted into a TermCollection.
	// e.g.
	// var ib IBNode
	// if ib.IsTermCollection() {
	//    col := ib.(TermCollection)
	// }
	IsTermCollection() bool

	// TODO:
	// IsIBNode() bool

	// IRI returns the IRI of this [Node].
	IRI() IRI
}

// func DebugIBNode(n IBNode) {
// 	fmt.Printf("=== %s ===\n", n.Fragment())
// 	fmt.Printf("iface var addr   = %p\n", &n)
// 	fmt.Printf("dynamic type     = %T\n", n)

// 	switch v := n.(type) {
// 	case *ibNode:
// 		fmt.Printf("underlying *ibNode      = %p\n", v)
// 	case *ibNodeString:
// 		fmt.Printf("underlying *ibNodeString= %p fragment=%q\n", v, v.fragment)
// 	case *ibNodeUuid:
// 		fmt.Printf("underlying *ibNodeUuid  = %p uuid=%v\n", v, v.id)
// 	default:
// 		fmt.Println("unknown underlying type")
// 	}
// }

// An IBNode represents either an IRI node or a blank node within an RDF graph.
// An IBNode provides access to the RDF triples associated to it either as subject, predicate or object.
// An IBNode should contain at least one subject triple, otherwise it cannot be written out into a rdf file.
// Every IBNode is owned by a NamedGraph with the same base URI.
type IBNode interface {
	Node
	// TypeOf returns the IBNode that represents the main type this IBNode is an rdf:type of.
	// An IBNode might be a type of several classes.
	// The main class is indicated in the SST Vocabularies by rdf:type ssmeta:MainClass.
	// Applications should ensure that only one main class is used for an IBNode.
	TypeOf() IBNode

	// IRI() returns the IRI (Internationalized Resource Identifier) of the IBNode.
	// If the IBNode is of type IRI Node, it will return its IRI.
	// If the IBNode is of type blank Node, it will panic.
	// If the IBNode is invalid or has been deleted, it will panic.
	IRI() IRI

	// Fragment returns the fragment part of the RDF resource identifier of the IBNode.
	// if the IBNode is of type IRI Node, it will return its fragment.
	// if the IBNode is of type blank Node, it will panic.
	Fragment() string

	// ID returns the UUID of the Blank IBNode.
	// If the IBNode is a blank node, it returns the UUID.
	// If the IBNode is an IRI node, it panics; use IRI() to get its identifier.
	ID() uuid.UUID

	// PrefixedFragment returns the IRI of the IBNode in shorthand notation as
	// it is used in Turtle syntax (https://www.w3.org/TR/turtle/).
	// It is legal to invoke this method for the zero valued IBNode as well.
	// In such case the predefined string "(nil)" is returned.
	//
	// Example:
	//
	// fmt.Print(ib.PrefixedFragment()) // prints "sso:Part"
	PrefixedFragment() string

	// TripleCount returns the number of triples this IBNode is used as either subject, predicate or object.
	// This TripleCount may used in combination with GetTriple method.
	TripleCount() int

	// OwningGraph returns the owning NamedGraph of the IBNode.
	OwningGraph() NamedGraph

	// ForAll iterates over all triples (subject, predicate or object triplex) of this IBNode.
	//
	// The triples of the owning graph that have receiver node as its subject are called forward triples.
	// The triples of the owning graph that have receiver node as its object are called inverse triples.
	// The triples of the owning graph that have receiver node as its predicate are called middle triples.
	// The triples of the owning graph that have receiver node both as its subject and object are called
	// self referenced triple.
	//
	// For each forward triple the given callback function c is invoked with s set to the receiver node and
	// p, o set to the triple's predicate and object accordingly.
	// For each inverse triple the given callback function c is invoked with s, p set to triple's subject
	// and object accordingly and o set to the receiver node.
	// For the self referenced triple the given callback function invoked with s and o set to
	// receiver node and p to the triple's predicate.
	// When the callback function returns an error, the ForAll loop is ended immediately and this error is returned.
	ForAll(c func(a int, subject IBNode, predicate IBNode, object Term) error) error

	// GetObjects returns all Objects for the given Subject(IBNode) with specified Node.
	// later on: support also subProperty of predicate
	// return values: a slice of objects which length may be 0 or 1 or more
	GetObjects(predicate Node) []Term

	// GetTriple returns particular triple that identified by index related to the IBNode.
	// The triple is returned as subject(s), predicate(p), and object(o).
	// The IBNode is used either as subject, predicate, or object.
	// The number of triples that are associated to an IBNode is available by TripleCount().
	// The order of triples is managed internally by SST and can not be influenced by the API.
	// The possible error codes are: ErrIndexOutOfRange, ErrNodeDeletedOrInvalid
	GetTriple(index int) (IBNode, IBNode, Term)

	// AddStatement() method adds a triple to the target IBNode that is used as subject with p for the predicate and o for the object.
	// User has to ensure that the triple to be added does not already exist; otherwise this method will panic.
	AddStatement(predicate Node, object Term)

	// AddStatementAlt() method adds a triple to the target IBNode that is used as subject with p for the predicate and o for the object.
	// This method is an alternative to AddStatement() with the difference that it is tolerant in case that the triple to be added already exist.
	// An error will be returned if the triple to be added already exist.
	AddStatementAlt(predicate Node, object Term) error

	// CheckTriple() checks is a triple with the given subject, predicate and object exist and returns TRUE in this case; otherwise FALSE is returned.
	CheckTriple(predicate Node, object Term) bool

	// Delete deletes the target IBNode and all triples
	// where the target is used as either subject, predicate or object.
	// After deletion this IBNode is no longer valid and IBNode.IsValid() will return false.
	Delete()

	// ForDelete iterates over all forward triples of this IBNode and invokes the fallback function c
	// for each triple with it's subject, predicate and object.
	// If the invoked callback function c returns true for one or several triples, then these triples are removed.
	ForDelete(c func(int, IBNode, IBNode, Term) bool)

	// DeleteTriples deletes all subject triples of this IBNode
	DeleteTriples()

	// InVocabulary returns a corresponding ElementInformer for the IBNode.
	// It is legal to invoke this method for the zero valued IBNode as well.
	// This method is in most cases invoked for IBNodes from ReferencedGraphs.
	//
	// This method often used to determine the actual ElementInformer interface
	// which can be done by using "Type Switch" or "Type Assertion" from the Golang specification.
	//
	// Example of using Type Switch:
	//
	//	var d sst.IBNode
	//	switch d.TypeOf().InVocabulary().(type) {
	//	case sso.KindPart:
	//		// d is sso:Part or any of its subtypes
	//		// ...
	//	}
	//
	// Example of using Type Assertion:
	//
	//	 if _, ok := p.InVocabulary().(lci.KindHasArrangedPart); ok {
	//		    // do something
	//
	//     }
	InVocabulary() ElementInformer

	// Is checks if the IBNode is the same as a given vocabulary element.
	// It is legal to invoke this method for the zero valued IBNode as well.
	// It is legal to invoke this method for the nil valued IBNode as well.
	// This method is in most cases invoked for IBNodes from ReferencedGraphs.
	// This method is in most cases invoked for IBNodes from ReferencedGraphs
	// for which a corresponding dictionary or vocabulary included in the SST ontologies.
	Is(e Elementer) bool

	// IsKind checks if the IBNode is of given vocabulary element type.
	IsKind(e ElementInformer) bool

	// IsValid return true if the IBNode represents a non-zero value of the IBNode type
	// that belongs to a valid NamedGraph.
	IsValid() bool

	// IsUuidFragment checks on whether the ibNode has a uuid fragment.
	// If it has, true is returned. Otherwise, false is returned.
	IsUuidFragment() bool

	// IsReferenced returns whether the NamedGraph of this IBNode is Referenced,
	// and as a consequences, it does not contain any subject triples.
	// It is not possible to modify an IBNode that is in a referenced state.
	IsReferenced() bool

	// IsIRINode returns true if the IBNode is an IRI node.
	IsIRINode() bool

	// IsBlankNode returns true if the IBNode is a blank node.
	IsBlankNode() bool

	// AsCollection() checks if an IBNode is in fact a TermCollections and returns it if successful.
	AsCollection() (TermCollection, bool)

	// DumpTriples returns a slice of all subject triples of this IBNode
	// consisting of [3]string for the subject, predicate and object of the triple.
	// All IRIs are given with the default or automatically assigned prefixes, similar to standard Turtle format.
	DumpTriples() [][3]string

	// Equal compares this IBNode with another IBNode in another NamedGraph and another Stage.
	// Equal compares the IRI of IRI Nodes and all subject triples of all IRI and BlankNodes.
	// Returns true if the two IBNodes are considered equal, false otherwise.
	// Note: In the case that the predicate or object of a triple exists in another NamedGraph, these must be IRI Nodes not BlankNodes;
	//       Equal only compares the IRIs of such IRI Nodes.
	// Typical use case:
	//   Use this method to determine if two IBNode instances represent the same logical node in the graph,
	//   regardless of their memory address or instantiation.
	// TBD: blankNode comparison
	Equal(ib IBNode) bool

	// implements sort.Interface method.
	// Len() returns the number of triples of this IBNode.
	Len() int
	// Less() compares the order of two triples of this IBNode.
	Less(i int, j int) bool
	// Swap() changes the position of two triples of this IBNode.
	Swap(i int, j int)

	sortAndCountTriples(incImportedRefCount func(IBNode), incExternalRefCount func(IBNode)) ibNodeWriteCounts
	forMemberNodeRange(fromTriples int, toTriple int, c func(IBNode, IBNode) error) error
	forPropNodeRange(fromTriple int, toTriple int, writeIBNodePO func(IBNode, IBNode) error) error
	forMemberLiteralRange(fromTriple int, toTriple int, expectCnt int, c func(IBNode, Term) error) error
	forPropLiteralRange(fromTriple int, toTriple int, c func(IBNode, Literal) error) error
	forPropLiteralCollectionRange(fromTriple int, toTriple int, c func(IBNode, *literalCollection) error) error
	appendUniqueNodeTriple(p IBNode, o IBNode)
	appendUniqueLiteralTriple(p IBNode, o Literal)
	appendUniqueLiteralCollectionTriple(p IBNode, c LiteralCollection)
	extractCollectionMembers()
	deleteCollectionMembers() int
	setCollectionMembers(members ...Term)
	copyIBNode() *ibNode
	ibNodeType() ibNodeType
	asUuidIBNode() *ibNodeUuid
	asStringIBNode() *ibNodeString
	assertModification()
	arePredicateTriplexesTracked() bool
	forAll(c func(int, *ibNode, *ibNode, Term) error) error
	forAllTriplexes(c func(int, triplex, triplexKind) (triplex, error)) error
	forDeleteTriplexes(c func(int, *ibNode, *ibNode, Term) bool)
	addAll(c func() (*ibNode, Term, bool), asCollection bool)
	addTriple(predicate *ibNode, object Term, asCollection bool, collectionIndex triplexOffset) error
	inVocabulary() ElementInformer
	isVocabularyElement(e Element) bool
	dump(simple bool)
	printTriples(level int, traversed map[*ibNode]struct{})
	iriOrID() string
	fragOrID() string
}

func (t *ibNode) printTriples(level int, traversed map[*ibNode]struct{}) {
	if _, found := traversed[t]; found {
		return
	}

	indent := ""
	for i := 0; i < level; i++ {
		indent += "\t"
	}

	t.ForAll(func(_ int, s, p IBNode, o Term) error {
		if t != s {
			return nil
		}
		traversed[t] = struct{}{}

		fmt.Printf("%sIBNode: %s\n", indent, t.iriOrID())

		switch o.TermKind() {
		case TermKindIBNode:
			var ib IBNode
			ib = o.(IBNode)
			fmt.Printf("%s%s %s\n", indent, p.IRI(), ib.iriOrID())
			ib.printTriples(level+1, traversed)
		case TermKindTermCollection:
			fmt.Printf("%s%s %s\n", indent, p.IRI(), o.(TermCollection).ID())
			o.(IBNode).printTriples(level+1, traversed)
		case TermKindLiteral:
			fmt.Printf("%s%s %s\n", indent, p.IRI(), literalToString(o.(Literal)))
		case TermKindLiteralCollection:
			var lc string
			lc += "( "
			o.(LiteralCollection).ForMembers(func(index int, li Literal) {
				lc += literalToString(li) + " "
			})
			lc += ")"
			fmt.Printf("%s%s %s\n", indent, p.IRI(), lc)
		default:
			panic(fmt.Sprintf("unknown object type %T", o))
		}
		return nil
	})
}

func (t *ibNode) DumpTriples() [][3]string {
	t.OwningGraph().Stage().numberNGs()
	returnedString := make([][3]string, 0)

	t.forAll(func(index int, subject, predicate *ibNode, object Term) error {
		var triple [3]string

		// for subject
		triple[0] = ibNodeToString(t.OwningGraph(), subject)

		// for predicate
		triple[1] = ibNodeToString(t.OwningGraph(), predicate)

		// for object
		switch object.TermKind() {
		case TermKindIBNode:
			ibo := object.(IBNode)
			triple[2] = ibNodeToString(t.OwningGraph(), ibo)
		case TermKindLiteral:
			triple[2] = literalToString(object.(Literal))
		case TermKindTermCollection:
			triple[2] += "( "
			object.(TermCollection).ForMembers(func(index int, li Term) {
				o := li.(IBNode)
				triple[2] += ibNodeToString(t.OwningGraph(), o) + " "
			})
			triple[2] += ")"
		case TermKindLiteralCollection:
			triple[2] += "( "
			object.(LiteralCollection).ForMembers(func(index int, li Literal) {
				triple[2] += literalToString(li) + " "
			})
			triple[2] += ")"
		default:
			return fmt.Errorf("unknown object type %T", object)
		}
		returnedString = append(returnedString, triple)

		return nil
	})

	return returnedString
}

func ibNodeToString(baseNg NamedGraph, ib IBNode) string {
	if ib.IsBlankNode() {
		return "_:" + ib.ID().String()
	} else {
		// if the baseNg is the same as the owning graph of the ibNode, means this is ibNode is belonging to the baseNg
		// so we can use the fragment directly
		if ib.OwningGraph() == baseNg {
			return ":" + ib.Fragment()
			// if the baseNg is from the dictionary, we can use PrefixFragment to get correct prefix
		} else if _, found := NamespaceToPrefix(ib.OwningGraph().IRI().String()); found {
			return ib.PrefixedFragment()
		} else {
			// else we need to use generated "ns" prefix and the fragment
			return "ns" + strconv.Itoa(ib.OwningGraph().getNgNumber()) + ":" + ib.Fragment()
		}
	}
}

const (
	ibNodeToResourceOffset = unsafe.Offsetof(ibNode{}.typedResource)
	ibNodeStringOffset     = unsafe.Offsetof(ibNodeString{}.ibNode)
	ibNodeUuidOffset       = unsafe.Offsetof(ibNodeUuid{}.ibNode)
)

type triplex struct {
	p *ibNode
	t *typedResource
}

// Term represents the object on an RDF triple, that is either of type:
//   - [IBNode]
//   - [Literal]
//   - [TermCollection])
//   - [LiteralCollection]
type Term interface {
	// Method [Term.TermKind] returns the kind of this Term which is one of
	//   - [TermKindIBNode]
	//   - [TermKindLiteral]
	//   - [TermKindTermCollection]
	//   - [TermKindLiteralCollection]
	TermKind() TermKind
}

// literal is used in triplex storage for literal values.
type literal struct {
	typedResource
	value string // literal value internal representation a.k.a []bytes
}

const literalToResourceOffset = unsafe.Offsetof(literal{}.typedResource)

var (
	_ Term    = (*literal)(nil)
	_ Literal = (*literal)(nil)
)

// This method ensures that a [Literal] is treated as an [Term]. It returns [TermKindLiteral].
func (literal) TermKind() TermKind {
	return TermKindLiteral
}

func (l literal) DataType() IBNode {
	return l.typeOf
}

func (l literal) apiValue() interface{} {
	return literalValue(l.typeOf, l.value)
}

func (l *literal) typedResourcePtr() *typedResource             { return &l.typedResource }    //nolint:unused
func (*literal) validResourceType() resourceType                { return resourceTypeLiteral } //nolint:unused
func (*literal) fromTypedResourcePtr(r *typedResource) *literal { return r.asLiteral() }       //nolint:unused
func (l *literal) isEqualTo(o *literal) bool                    { return *l == *o }            //nolint:unused

func literalValue(typeOf *ibNode, valueHolder string) interface{} {
	switch typeOf {
	case &literalTypeString.ibNode:
		return String(valueHolder)
	case &literalTypeDouble.ibNode:
		return Double(float64FromBytes(valueHolder))
	case &literalTypeFloat.ibNode:
		return Float(float32FromBytes(valueHolder))
	case &literalTypeInteger.ibNode:
		return Integer(int64FromBytes(valueHolder))
	case &literalTypeBoolean.ibNode:
		return Boolean(valueHolder != "")
	case &literalTypeLangString.ibNode:
		// last two bytes are language tag
		return LangString{Val: valueHolder[:len(valueHolder)-2], LangTag: valueHolder[len(valueHolder)-2:]}
	case &literalTypeByte.ibNode:
		return Byte(int8FromBytes(valueHolder))
	case &literalTypeShort.ibNode:
		return Short(int16FromBytes(valueHolder))
	case &literalTypeInt.ibNode:
		return Int(int32FromBytes(valueHolder))
	case &literalTypeLong.ibNode:
		return Long(int64FromBytes(valueHolder))
	case &literalTypeUnsignedByte.ibNode:
		return UnsignedByte(uint8FromBytes(valueHolder))
	case &literalTypeUnsignedShort.ibNode:
		return UnsignedShort(uint16FromBytes(valueHolder))
	case &literalTypeUnsignedInt.ibNode:
		return UnsignedInt(uint32FromBytes(valueHolder))
	case &literalTypeUnsignedLong.ibNode:
		return UnsignedLong(uint64FromBytes(valueHolder))
	case &literalTypeDateTime.ibNode:
		return TypedString{Val: valueHolder, Type: &literalTypeDateTime.ibNode}
	case &literalTypeDateTimeStamp.ibNode:
		return TypedString{Val: valueHolder, Type: &literalTypeDateTimeStamp.ibNode}
	default:
		return valueHolder
	}
}

func float64FromBytes(bytes string) float64 {
	floatBits := binary.BigEndian.Uint64([]byte(bytes))
	float := math.Float64frombits(floatBits)
	return float
}

func float64Bytes(float float64) string {
	floatBits := math.Float64bits(float)
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, floatBits)
	return *(*string)(unsafe.Pointer(&bytes))
}

func float32FromBytes(bytes string) float32 {
	floatBits := binary.BigEndian.Uint32([]byte(bytes))
	return math.Float32frombits(floatBits)
}

func float32Bytes(float float32) string {
	floatBits := math.Float32bits(float)
	bytes := make([]byte, 4)
	binary.BigEndian.PutUint32(bytes, floatBits)
	return *(*string)(unsafe.Pointer(&bytes))
}

func int64FromBytes(bytes string) int64 {
	return int64(binary.BigEndian.Uint64([]byte(bytes)))
}

func int64Bytes(integer int64) string {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, uint64(integer))
	return *(*string)(unsafe.Pointer(&bytes))
}

func int32Bytes(i int32) string {
	bytes := make([]byte, 4)
	binary.BigEndian.PutUint32(bytes, uint32(i))
	return *(*string)(unsafe.Pointer(&bytes))
}

func int32FromBytes(bytes string) int32 {
	return int32(binary.BigEndian.Uint32([]byte(bytes)))
}

func int16Bytes(i int16) string {
	bytes := make([]byte, 2)
	binary.BigEndian.PutUint16(bytes, uint16(i))
	return *(*string)(unsafe.Pointer(&bytes))
}

func int16FromBytes(bytes string) int16 {
	return int16(binary.BigEndian.Uint16([]byte(bytes)))
}

func int8Bytes(i int8) string {
	bytes := make([]byte, 1)
	bytes[0] = byte(i)
	return *(*string)(unsafe.Pointer(&bytes))
}

func int8FromBytes(bytes string) int8 {
	return int8(bytes[0])
}

func uint64Bytes(v uint64) string {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, v)
	return *(*string)(unsafe.Pointer(&bytes))
}

func uint64FromBytes(bytes string) uint64 {
	return binary.BigEndian.Uint64([]byte(bytes))
}

func uint32Bytes(v uint32) string {
	bytes := make([]byte, 4)
	binary.BigEndian.PutUint32(bytes, v)
	return *(*string)(unsafe.Pointer(&bytes))
}

func uint32FromBytes(bytes string) uint32 {
	return binary.BigEndian.Uint32([]byte(bytes))
}

func uint16Bytes(v uint16) string {
	bytes := make([]byte, 2)
	binary.BigEndian.PutUint16(bytes, v)
	return *(*string)(unsafe.Pointer(&bytes))
}

func uint16FromBytes(bytes string) uint16 {
	return binary.BigEndian.Uint16([]byte(bytes))
}

func uint8Bytes(v uint8) string {
	bytes := make([]byte, 1)
	bytes[0] = v
	return *(*string)(unsafe.Pointer(&bytes))
}

func uint8FromBytes(bytes string) uint8 {
	return uint8(bytes[0])
}

// LiteralCollection represents a restricted and memory optimized RDF Collection
// whose members are all [Literal]s of the same DataType.
// A LiteralCollection is implementing the [Term] interface and can therefor be used as object e.g. for AddStatement.
// A LiteralCollection is created by [NewLiteralCollection].
//
// LiteralCollection is only used as function/method parameter,
// it is not linked with the internal memory structure.
// So there is no need to delete it when no longer needed.
type LiteralCollection interface {
	// TermKind() returns TermKindLiteralCollection
	Term

	// ForMembers invokes callback function e for each member in the collection.
	ForMembers(e func(int, Literal))

	// MemberCount returns the number of members in this ObjectionCollection.
	MemberCount() int

	// Member returns the member as the indicated position starting with 0 for the first position.
	// It will panic if the index is negative or if it is equal or greater than MemberCount().
	Member(index int) Literal

	// Values returns a slice of interface{} containing the values of the LiteralCollection.
	Values() []interface{}

	// DataType returns the DataType of this LiteralCollection as an IBNode.
	DataType() IBNode
}

type literalCollection struct {
	typedResource
	dataType *ibNode
	members  []string
}

const literalCollectionToResourceOffset = unsafe.Offsetof(literalCollection{}.typedResource)

func (*literalCollection) TermKind() TermKind {
	return TermKindLiteralCollection
}

func (c *literalCollection) DataType() IBNode {
	return c.dataType
}

// NewLiteralCollection creates a collection where all members are Literals of the same DataType.
// Providing members of different DataTypes will result in panic.
func NewLiteralCollection(member0 Literal, members1N ...Literal) LiteralCollection {
	dataType := member0.DataType()
	listMembers := make([]string, len(members1N)+1)
	listMembers[0] = objectToLiteral(member0).value
	for i, l := range members1N {
		listMembers[i+1] = objectToLiteral(l).value
		if l.DataType() != dataType {
			panic(ErrLiteralDataTypesDiffer)
		}
	}
	return &literalCollection{
		typedResource: typedResource{typeOf: &literalCollectionResourceType.ibNode},
		dataType:      dataType.(*ibNode),
		members:       listMembers,
	}
}

// ForMember iterates over the members of a [literalCollection]] and invokes the call-back function e for each member.
func (c literalCollection) forInternalMembers(e func(index int, l Literal)) {
	for index, member := range c.members {
		e(index, literal{
			typedResource: typedResource{typeOf: c.dataType},
			value:         member,
		})
	}
}

func (c *literalCollection) ForMembers(e func(index int, l Literal)) {
	for index, member := range c.members {
		e(index, literal{
			typedResource: typedResource{typeOf: c.dataType},
			value:         member,
		}.apiValue().(Literal))
	}
}

// Values returns a slice of interface{} containing the values of the LiteralCollection.
// Each member of the LiteralCollection is converted to a literal and its value is extracted.
// The resulting slice has the same length as the number of members in the LiteralCollection.
func (c *literalCollection) Values() []interface{} {
	out := make([]interface{}, 0, len(c.members))
	for _, member := range c.members {
		out = append(out, literal{
			typedResource: typedResource{typeOf: c.dataType},
			value:         member,
		}.apiValue())
	}
	return out
}

func (c *literalCollection) MemberCount() int {
	return len(c.members)
}

func (c *literalCollection) Member(index int) Literal {
	return literal{
		typedResource: typedResource{typeOf: c.dataType},
		value:         c.members[index],
	}.apiValue().(Literal)
}

func (c literalCollection) typedResourcePtr() *typedResource { return &c.typedResource }              //nolint:unused
func (literalCollection) validResourceType() resourceType    { return resourceTypeLiteralCollection } //nolint:unused

func (*literalCollection) fromTypedResourcePtr(r *typedResource) *literalCollection { //nolint:unused
	return r.asLiteralCollection()
}

func (c *literalCollection) isEqualTo(o *literalCollection) bool { //nolint:unused
	if c.typedResource != o.typedResource {
		return false
	}
	if len(c.members) != len(o.members) {
		return false
	}
	for i, member := range c.members {
		if member != o.members[i] {
			return false
		}
	}
	return true
}

// TermCollection represents a full featured [RDF Collection] that may contain members of type
// [IBNode], any type of [literal], [literalCollection] or other nested [TermCollection].
//
// A TermCollection is a special type of blank node that is created by [NamedGraph.CreateCollection].
// A TermCollection is deleted by "casting" it to an IBNode using [TermCollection.AsIBNode]
// and then invoking the method [IBNode.Delete].
// An empty TermCollection is equivalent to the IBNode represented by rdf.Nil.
//
//	TermCollection struct {
//		IBNode
//	}
//
// [RDF Collection]: https://www.w3.org/TR/rdf12-semantics/#rdf-collections
type TermCollection interface {
	Node

	// ForMembers invokes callback function e for each member in the collection.
	ForMembers(e func(int, Term))

	// Members returns the members of this ObjectionCollection as a slice.
	Members() []Term

	// MemberCount returns the number of members in this ObjectionCollection.
	MemberCount() int

	// Member returns the member as the indicated position starting with 0 for the first position.
	// It will panic if the index is negative or if it is equal or greater than MemberCount().
	Member(index int) Term

	// SetMembers removes the current members of this TermCollection and set all members anew.
	SetMembers(members ...Term)

	// ID returns the UUID value of this TermCollection, which is a Random (Version 4) UUID .
	ID() uuid.UUID
}

// newTermCollection creates a TermCollection backed by a new blank node with a
// random type 4 UUID, as documented on the TermCollection.ID() interface.
func newTermCollection(graph *namedGraph, members ...Term) (TermCollection, error) {
	d := newUuidNode(graph, uuid.New(), &termCollectionResourceType.ibNode, blankNodeType|uuidNode)
	d.setCollectionMembers(members...)
	return &d.ibNode, nil
}

func (t *ibNode) ForMembers(e func(index int, object Term)) {
	var index int
	err := t.forAllTriplexes(func(_ int, tx triplex, k triplexKind) (_ triplex, err error) {
		switch k {
		case subjectTriplexKind:
			if tx.p.Is(rdfFirst) {
				e(index, triplexToObject(tx))
				index++
				return
			}
			err = errBreakFor
		case objectTriplexKind, predicateTriplexKind:
		}
		return
	})
	if err == errBreakFor { // nolint:errorlint
		err = nil
	}
	if err != nil {
		panic(err) // Error should be always nil
	}
}

func (t *ibNode) Members() []Term {
	out := make([]Term, 0)
	t.ForMembers(func(_ int, object Term) {
		out = append(out, object)
	})
	return out
}

func (t *ibNode) MemberCount() int {
	var i triplexOffset
	for i = t.triplexEnd; i > t.triplexStart; i-- {
		if t.ng.triplexStorage[i-1].p != nil &&
			t.ng.triplexStorage[i-1].p.Is(rdfFirst) &&
			triplexKindAtAbs(t.ng, i-1) == subjectTriplexKind {
			break
		}
	}
	return int(i - t.triplexStart)
}

func (t *ibNode) Member(index int) Term {
	s, _, o := t.getTriple(index)
	if s != t {
		_ = t.ng.triplexStorage[len(t.ng.triplexStorage)] // cause panic deliberately
	}
	return o
}

func (t *ibNode) SetMembers(members ...Term) {
	t.ng.flags.modified = true
	deletedCnt := t.deleteCollectionMembers()
	for i := 0; i < deletedCnt; i++ {
		t.deleteTriplexAt(i)
	}
	for i := deletedCnt; i <= len(members)-1; i++ {
		insertUniqueTriplex(t, triplexOffset(i), triplex{}, subjectTriplexKind)
	}

	t.setCollectionMembers(members...)
}

func (t *ibNode) deleteCollectionMembers() int {
	triplexes := t.ng.triplexStorage[int(t.triplexStart):int(t.triplexEnd)]
	var i int
	for i = 0; i < len(triplexes); i++ {
		pair := triplexes[i]
		if pair.p == nil {
			continue
		}
		if t.triplexKindAt(i) == subjectTriplexKind {
			if !pair.p.Is(rdfFirst) {
				break
			}
			if resourceTypeRecursive(pair.t) == resourceTypeIBNode {
				pair.t.asIBNode().deleteTriplex(triplex{
					p: pair.p, t: &t.typedResource,
				}, objectTriplexKind)
			}
			t.deleteTriplexAt(i)
		} else {
			break
		}
	}
	return i
}

func (t *ibNode) setCollectionMembers(members ...Term) {
	rdfNg := t.ng.stage.referencedGraphByURI("http://www.w3.org/1999/02/22-rdf-syntax-ns")
	var rdfFirst IBNode
	rdfFirst = rdfNg.GetIRINodeByFragment("first")
	if rdfFirst == nil {
		rdfFirst = rdfNg.CreateIRINode("first")
	}

	var i int
	t.addAll(func() (p *ibNode, object Term, ok bool) {
		switch rdfFirst := rdfFirst.(type) {
		case *ibNodeString:
			p = &rdfFirst.ibNode
		case *ibNode:
			p = rdfFirst
		default:
			panic(fmt.Sprintf("unexpected rdfFirst type %T", rdfFirst))
		}

		if len(members) > 0 && i < len(members) {
			object = members[i]
			ok = true
			i++
			return
		}
		return
	}, true)
}

func (t *ibNode) AsCollection() (col TermCollection, ok bool) {
	if t.typeOf == &termCollectionResourceType.ibNode {
		col, ok = t, true
	}
	return
}

// triplexesEqual compares two triplex structures for equality.
// It checks if the predicate and object types are equal, and optionally compares collection members.
// Returns true if the triplexes are considered equal, false otherwise.
//
// Parameters:
//
//	tx1, tx2: The two triplex instances to compare.
//	considerCollectionMembers: If true, collection members are compared for equality; if false, only basic properties are checked.
func triplexesEqual(tx1, tx2 triplex, considerCollectionMembers bool) bool {
	if tx1.p != tx2.p || (!considerCollectionMembers && tx1.p.Is(rdfFirst)) {
		return false
	}
	if tx1.t == tx2.t {
		return true
	}
	if tx1.t.typeOf != tx2.t.typeOf {
		return false
	}
	t1Type := resourceTypeRecursive(tx1.t)
	switch t1Type {
	case resourceTypeIBNode:
		return false
	case resourceTypeLiteral:
		l1 := tx1.t.asLiteral()
		l2 := tx2.t.asLiteral()
		return l1.value == l2.value
	case resourceTypeLiteralCollection:
		c1 := tx1.t.asLiteralCollection()
		c2 := tx2.t.asLiteralCollection()
		if c1.dataType != c2.dataType {
			return false
		}
		if len(c1.members) != len(c2.members) {
			return false
		}
		for i, m := range c1.members {
			if m != c2.members[i] {
				return false
			}
		}
		return true
	}
	return false
}

func newStringNode(graph *namedGraph, fragment string, typeOf *ibNode, flag ibNodeFlag) *ibNodeString {
	t := ibNodeString{
		ibNode: ibNode{
			typedResource: typedResource{
				typeOf: typeOf,
			},
			ng:    graph,
			flags: flag,
		},
		fragment: fragment,
	}
	return &t
}

func newUuidNode(graph *namedGraph, fragment uuid.UUID, typeOf *ibNode, flag ibNodeFlag) *ibNodeUuid {
	t := ibNodeUuid{
		ibNode: ibNode{
			typedResource: typedResource{
				typeOf: typeOf,
			},
			ng:    graph,
			flags: flag,
		},
		id: fragment,
	}
	return &t
}

var _ Term = (*ibNode)(nil)

func (t *ibNode) TermKind() TermKind {
	if t.typeOf == &termCollectionResourceType.ibNode {
		return TermKindTermCollection
	}

	return TermKindIBNode
}

func (t *ibNode) OwningGraph() NamedGraph {
	return t.ng
}

// IRI returns the IRI (Internationalized Resource Identifier) of the ibNode.
// If the ibNode is valid, it retrieves the IRI from the ibNodeType.
// If the ibNode is invalid or has been deleted, it panics with ErrNodeDeletedOrInvalid.
func (t *ibNode) IRI() IRI {
	if t.IsValid() {
		return t.ibNodeType().iri()
	} else {
		panic(ErrNodeDeletedOrInvalid)
	}
}

// iri generates an IRI (Internationalized Resource Identifier) for the ibNodeString.
// It constructs the IRI based on the node's fragment and base IRI.
// If the fragment is empty, it returns the NamedGraph baseIRI (used for the NamedGraph node itself).
// If the base IRI is empty, it returns the fragment as the IRI.
// If the fragment is ".", it removes the trailing '#' from the base IRI if present.
// Otherwise, it appends the fragment to the base IRI.
// The resulting IRI is returned as a type-casted IRI.
func (t *ibNodeString) iri() IRI {
	if t.fragment == "" {
		//the IBNode.IRI() method on the NG IBNode must return the IRI of the NamedGraph without any trailing "#" and fragment
		return *(*IRI)(unsafe.Pointer(&t.ng.baseIRI))

	}
	resourceIDString := t.ng.baseIRI + "#"
	resourceIDString += string(t.fragment)

	return *(*IRI)(unsafe.Pointer(&resourceIDString))
}

// a blank node is always ibNodeUuid type
// but an ibNodeUuid may not be a blankNode
// it also can be an IRI Node
func (t *ibNodeUuid) iri() IRI {
	if t.ibNode.flags&nodeTypeBitMask == blankNodeType {
		panicf("blank node %s in graph %q does not have an IRI", t.id, t.ng.baseIRI)
	}
	resourceIDString := t.ng.baseIRI + "#"
	resourceIDString += t.id.String()
	return *(*IRI)(unsafe.Pointer(&resourceIDString))
}

func (t *ibNode) Fragment() string {
	if t.IsValid() {
		return t.ibNodeType().fragmentComponent()
	} else {
		panic(ErrNodeDeletedOrInvalid)
	}
}

func (t *ibNode) PrefixedFragment() string {
	var ns, ln string
	// namespaceToPrefixMap != nil means the vocabulary is loaded
	if namespaceToPrefixMap != nil {
		if t.ng.IsValid() {
			ns = t.ng.baseIRI
			if t.IsIRINode() {
				ln = string(t.Fragment())
			} else {
				ln = t.ID().String()
			}
		}
	}
	if ns != "" {
		if prefix, found := NamespaceToPrefix(ns); found {
			return prefix + ":" + ln
		}
	}

	if t.IsBlankNode() {
		GlobalLogger.Warn("blank node does not have fragment, it only has an ID")
		return ""
	} else {
		return "<" + t.IRI().String() + ">"
	}
}

func (t *ibNodeString) fragmentComponent() string {
	return t.fragment
}

func (t *ibNodeUuid) fragmentComponent() string {
	if t.flags&nodeTypeBitMask == blankNodeType {
		panicf("blank node %s in graph %q does not have a fragment", t.id, t.ng.baseIRI)
	}
	return t.id.String()
}

func (t *ibNode) IsUuidFragment() bool {
	return t.flags&uuidNode == uuidNode
}

// ID returns the UUID of the Blank IBNode.
// If the ibNode is a blank node, it returns the UUID.
// If the ibNode is an IRI node, it panics with a message indicating that IRI nodes
// should use IRI() to get their identifier.
func (t *ibNode) ID() uuid.UUID {
	if t.flags&nodeTypeBitMask == blankNodeType {
		return t.asUuidIBNode().id
	}
	panic(fmt.Sprintf("IBNode %s in graph %q is an IRI node, use IRI() to get its identifier", t.iriOrID(), t.ng.baseIRI))
}

func (t *ibNode) copyIBNode() *ibNode {
	c := new(ibNode)
	*c = *t
	return c
}

func (t *ibNodeUuid) copy() *ibNode {
	c := new(ibNodeUuid)
	*c = *t
	return &c.ibNode
}

func (t *ibNodeString) copy() *ibNode {
	c := new(ibNodeString)
	*c = *t
	return &c.ibNode
}

func (t *ibNode) ibNodeType() ibNodeType {
	// if t.flags&blankNode == blankNode {
	// 	return t.asIBNodeBlank()
	// }
	// if t.flags&iriNodeString == iriNodeString {
	// 	return t.asIBNodeString()
	// }
	// return nil

	if t.flags&nodeTypeBitMask == blankNodeType {
		return t.asUuidIBNode()
	} else {
		if t.flags&structTypeBitMask == uuidNode {
			return t.asUuidIBNode()
		} else {
			return t.asStringIBNode()
		}
	}
}

func (t *ibNode) asUuidIBNode() *ibNodeUuid {
	return (*ibNodeUuid)(unsafe.Pointer(uintptr(unsafe.Pointer(t)) - ibNodeUuidOffset))
}

func (t *ibNode) asStringIBNode() *ibNodeString {
	return (*ibNodeString)(unsafe.Pointer(uintptr(unsafe.Pointer(t)) - ibNodeStringOffset))
}

// setFragment returns error if not-unique.
func (t *ibNodeUuid) setFragment(fragment uuid.UUID) error {
	t.assertModification()

	return t.ng.renameIBNodeUuid(t, fragment)
}

// setFragment returns error if not-unique, string parameter can be empty for blank node.
func (t *ibNodeString) setFragment(fragment string) error {
	t.assertModification()
	if t.fragment == fragment {
		return nil
	}
	if fragment == "" {
		panic(ErrDuplicatedFragment)
	}

	return t.ng.renameIBNodeString(t, fragment)
}

func (t *ibNode) TripleCount() int {
	count := 0
	t.forAllTriplexes(func(_ int, tx triplex, _ triplexKind) (_ triplex, err error) {
		count++
		return tx, nil
	})
	return count
}

func (t *ibNode) IsReferenced() bool {
	return t.ng.IsReferenced()
}

func (t *ibNode) IsBlankNode() bool {
	return t.flags&nodeTypeBitMask == blankNodeType
}

func (t *ibNode) IsIRINode() bool {
	return t.flags&nodeTypeBitMask == iriNodeType
}

func (t *ibNode) IsTermCollection() bool {
	if t.flags&uuidNode == uuidNode {
		if t.typeOf == &termCollectionResourceType.ibNode {
			return true
		}
	}

	return false
}

func (t *ibNode) assertModification() {
	if !t.IsValid() {
		panic(ErrNodeDeletedOrInvalid)
	}
	if t.isPredefinedIBNode() || t.IsReferenced() {
		panic(ErrNodeNotModifiable)
	}
}

func (t *ibNode) isPredefinedIBNode() bool {
	return t.ng == &staticRdfGraph || t.ng == &staticSSTGraph || t.ng == &staticXSDGraph
}

// TODO: replace this method by just check t.ng.flags.trackPredicates
func (t *ibNode) arePredicateTriplexesTracked() bool {
	return t.ng != nil && t.ng.triplexStorage != nil && t.ng.flags.trackPredicates
}

func (t *ibNode) TypeOf() IBNode {
	if t.typeOf == nil {
		// There are 2 types of nil, have to ensure that "IBNode nil" is returned,
		// but not a "*ibNode nil" is returned.
		return nil
	} else {
		return t.typeOf
	}
}

func (t *ibNode) ForAll(callback func(index int, subject, predicate IBNode, object Term) error) error {
	if ok := t.IsValid(); !ok {
		panic(ErrNodeDeletedOrInvalid)
	}
	return t.forAll(func(index int, subject, predicate *ibNode, object Term) error {
		return callback(index, subject, predicate, object)
	})
}

// forAll iterates over all triplexes of the ibNode and applies the provided callback function to each triplex.
// The callback function receives the index of the triplex, the subject ibNode, the predicate ibNode, and the object.
// The iteration is done through the forAllTriplexes method, which categorizes each triplex by its kind (subject, object, or predicate).
// Depending on the kind of the triplex, the callback function is called with different parameters:
// - For subject triplexes, the callback is called with the current ibNode as the subject, the predicate, and the object derived from the triplex.
// - For object triplexes, the callback is called with the term as the subject, the predicate, and the current ibNode as the object, unless it's a self-reference.
// - For predicate triplexes, the callback is called with the predicate as the subject, the current ibNode as the predicate, and the object derived from the triplex.
// If the callback function returns an error, the iteration stops and the error is returned.
func (t *ibNode) forAll(callback func(index int, subject, predicate *ibNode, object Term) error) error {
	return t.forAllTriplexes(func(index int, tx triplex, k triplexKind) (_ triplex, err error) {
		switch k {
		case subjectTriplexKind:
			// Forward reference
			err = callback(index, t, tx.p, triplexToObject(tx))
		case objectTriplexKind:
			term := tx.t.asIBNode()
			if term != t {
				// Inverse reference except for self-reference triple
				err = callback(index, term, tx.p, t)
			}
		case predicateTriplexKind:
			if tx.p != t {
				err = callback(index, tx.p, t, triplexToObject(tx))
			}
		default:
			panicf("unexpected triplexKind %v for node %s", k, t.iriOrID())
		}
		return
	})
}

func triplexToObject(tx triplex) Term {
	var term Term
	switch resourceTypeRecursive(tx.t) {
	case resourceTypeIBNode:
		term = tx.t.asIBNode()
	case resourceTypeLiteral:
		term = literalToObject(tx.t.asLiteral())
	case resourceTypeLiteralCollection:
		term = tx.t.asLiteralCollection()
	}
	return term
}

// forAllTriplexes will call provided callback function on each triplex, allow to delete or modify the triplex.
// callback function should take care of different tripleKind, callback function will look like:
// switch k {
// case subjectTriplexKind:
// ...
// case objectTriplexKind:
// ...
// case predicateTriplexKind:
// ...
// default:
//
//		panic(k)
//	}
func (t *ibNode) forAllTriplexes(callback func(index int, tx triplex, k triplexKind) (triplex, error)) error {
	if t.triplexEnd-t.triplexStart == 0 {
		return nil
	}
	triplexes := t.ng.triplexStorage[int(t.triplexStart):int(t.triplexEnd)]
	for i, tx := range triplexes {
		if tx.p == nil {
			continue
		}
		out, err := callback(i, tx, t.triplexKindAt(i))
		if err != nil {
			return err
		}
		if out.p != nil {
			if out.p == &deleteMarker {
				out.p = nil
			}
			triplexes[i] = out
		}
	}
	return nil
}

func (t *ibNode) ForDelete(c func(index int, subject, predicate IBNode, object Term) bool) {
	t.assertModification()
	t.forDeleteTriplexes(func(index int, subject, predicate *ibNode, object Term) bool {
		return c(index, subject, predicate, object)
	})
}

func (t *ibNode) forDeleteTriplexes(c func(index int, subject, predicate *ibNode, object Term) bool) {
	if t.triplexEnd-t.triplexStart == 0 {
		return
	}
	triplexes := t.ng.triplexStorage[int(t.triplexStart):int(t.triplexEnd)]
	for i, tx := range triplexes {
		if tx.p == nil {
			continue
		}

		switch t.triplexKindAt(i) {
		case subjectTriplexKind: // Forward reference

			toDelete := c(i, t, tx.p, triplexToObject(tx))
			if toDelete {
				if isResourceTypeIBNode(tx.t) {
					tx.t.asIBNode().deleteTriplex(triplex{
						p: tx.p,
						t: &t.typedResource,
					}, objectTriplexKind)
				}
				if tx.p.arePredicateTriplexesTracked() {
					tx.p.deleteTriplex(triplex{
						p: t,
						t: tx.t,
					}, predicateTriplexKind)
				}
				t.deleteTriplexAt(i)
			}
		}
	}
}

func (t *ibNode) DeleteTriples() {
	triplexes := t.ng.triplexStorage[int(t.triplexStart):int(t.triplexEnd)]
	for i := range triplexes {
		if t.triplexKindAt(i) == subjectTriplexKind {
			pair := triplexes[i]
			if pair.p != nil && pair.p.Is(rdfFirst) {
				continue
			}
			if pair.p != nil {
				if isResourceTypeIBNode(pair.t) {
					pair.t.asIBNode().deleteTriplex(triplex{
						p: pair.p, t: &t.typedResource,
					}, objectTriplexKind)
				}
				if pair.p.arePredicateTriplexesTracked() {
					pair.p.deleteTriplex(triplex{
						p: t, t: pair.t,
					}, predicateTriplexKind)
				}
			}
			t.deleteTriplexAt(i)
		}
	}
	if t.typeOf != &termCollectionResourceType.ibNode {
		t.typeOf = nil
	}
}

// This method adds a number of triples to the IBNode t by invoking a callback function c that
// provides with predicate and object to add. If c returns ok == false, this method is ending.
// If asCollection == true, all triples are treated as collection members
func (t *ibNode) addAll(c func() (predicate *ibNode, object Term, ok bool), asCollection bool) {
	var collectionIndex triplexOffset
	for {
		predicate, object, ok := c()
		if !ok {
			break
		}

		predicate, object = t.fromStatementPredObj(predicate, object)

		err := t.addTriple(predicate, object, asCollection, collectionIndex)
		if err != nil {
			panic(err)
		}
		if asCollection {
			collectionIndex++
		}
	}
}

func (t *ibNode) GetObjects(p Node) []Term {
	var returnSlice []Term

	ontGraph := t.ng
	var tp *ibNode

	if p != nil {
		switch p := p.(type) {
		case IBNode:
			tp = p.(*ibNode)
		case Elementer:
			tp = ontGraph.stage.vocabularyElementToIBNode(p.VocabularyElement())
		default:
			tp = ontGraph.stage.vocabularyElementToIBNode(IRI(p.IRI()).VocabularyElement())
		}
	}

	t.ForAll(func(_ int, s, predicate IBNode, object Term) error {
		if object != nil {
			if t == s { // forward triple
				if s != nil && predicate != nil && tp == predicate {
					returnSlice = append(returnSlice, object)
				}
			}
		}
		return nil
	})

	return returnSlice
}

func (t *ibNode) GetTriple(index int) (s, predicate IBNode, object Term) {
	if ok := t.IsValid(); !ok {
		panic(ErrNodeDeletedOrInvalid)
	}

	if index < 0 || index >= t.TripleCount() {
		panic(ErrIndexOutOfRange)
	}

	is, ip, io := t.getTriple(index)
	s, predicate, object = is, ip, io
	return
}

func (t *ibNode) getTriple(index int) (subject, predicate *ibNode, object Term) {
	if t.triplexEnd-t.triplexStart == 0 {
		panic(ErrNodeTripleNotSet)
	}
	triplexes := t.ng.triplexStorage[int(t.triplexStart):int(t.triplexEnd)]
	tx := triplexes[index]
	if tx.p == nil {
		panic(ErrNodeTripleNotSet)
	}
	predicate = tx.p
	switch k := t.triplexKindAt(index); k {
	case subjectTriplexKind:
		subject = t
		object = triplexToObject(tx)
	case objectTriplexKind:
		subject = tx.t.asIBNode()
		object = t
	case predicateTriplexKind:
		predicate = t
		subject = tx.p
		object = triplexToObject(tx)
	default:
		panic(k)
	}
	return
}

func (t *ibNode) AddStatement(predicate Node, object Term) {
	t.assertModification()
	// mark ibNode's ng modified
	t.ng.flags.modified = true

	outP, outO := t.fromStatementPredObj(predicate, object)

	if outP == nil {
		panic(ErrTriplePredicateNotSet)
	}
	var err error
	if outP.Is(rdfFirst) {
		err = t.addTriple(outP, outO, true, 0)
	} else {
		err = t.addTriple(outP, outO, false, 0)
	}
	if err != nil {
		panic(err)
	}
}

func (t *ibNode) AddStatementAlt(predicate Node, object Term) error {
	t.assertModification()
	// mark ibNode's ng modified
	t.ng.flags.modified = true

	outP, outO := t.fromStatementPredObj(predicate, object)

	if outP.Is(rdfFirst) {
		return t.addTriple(outP, outO, true, 0)
	} else {
		return t.addTriple(outP, outO, false, 0)
	}
}

func (t *ibNode) CheckTriple(predicate Node, object Term) bool {
	tripleSame := false
	ibNodePredicate, termObject := t.fromStatementPredObj(predicate, object)

	t.forAllTriplexes(func(index int, tx triplex, k triplexKind) (triplex, error) {
		txp := tx.p
		txt := triplexToObject(tx)

		txpString := txp.iriOrID()
		ibNodePredicateString := ibNodePredicate.iriOrID()
		predicateSame := txpString == ibNodePredicateString

		txtString := termToString(txt)
		termObjectString := termToString(termObject)
		objectSame := txtString == termObjectString

		if predicateSame && objectSame {
			tripleSame = true
		}

		return tx, nil
	})

	return tripleSame
}

// it converts vocabulary Elements into ibNodes
func (t *ibNode) fromStatementPredObj(predicate Node, object Term) (*ibNode, Term) {
	ontGraph := t.ng
	var tp *ibNode
	if predicate != nil {
		switch p := predicate.(type) {
		case IBNode:
			tp = p.(*ibNode)
		case Elementer:
			tp = ontGraph.stage.vocabularyElementToIBNode(p.VocabularyElement())
		default:
			tp = ontGraph.stage.vocabularyElementToIBNode(IRI(p.IRI()).VocabularyElement())
		}
	}
	var to Term
	if object != nil {
		switch o := object.(type) {
		case Elementer:
			to = ontGraph.stage.vocabularyElementToIBNode(o.VocabularyElement())
		case Term:
			to = o
		// case Node:
		// 	to = ontGraph.Stage().vocabularyElementToIBNode(IRI(o.IRI()).VocabularyElement())
		default:
			panic(o)
		}
	}
	return tp, to
}

// If asCollection == false then this method adds a new Triple to the IBNode t that is treated as subject with p as predicate and o as object.
// If asCollection == true then this method adds a new collection member to the collection t.
// Note that a TermCollection is an IBNode as well.
// In case of collection, the parameter collectionIndex defines the index of the collection member.
func (t *ibNode) addTriple(predicate *ibNode, object Term, asCollection bool, collectionIndex triplexOffset) error {
	if predicate == nil {
		return ErrTriplePredicateNotSet
	}
	if object == nil {
		return ErrTripleObjectNotSet
	}
	if predicate.ng != &staticRdfGraph {
		if t.ng.stage != predicate.ng.stage {
			return ErrStagesAreNotTheSame
		}
	}
	if !asCollection && predicate.Is(rdfFirst) {
		// return ErrCannotSetCollectionMemberPredicate
		panic(ErrCannotSetCollectionMemberPredicate)
	}
	switch object.TermKind() {
	case TermKindIBNode, TermKindTermCollection:
		var oIBNode *ibNode
		switch v := object.(type) {
		case *ibNode:
			oIBNode = v
		case *ibNodeUuid:
			oIBNode = &v.ibNode
		case *ibNodeString:
			oIBNode = &v.ibNode
		default:
			panic("no correct type")
		}

		if oIBNode.ng != &staticRdfGraph {
			if t.ng.stage != oIBNode.ng.stage {
				return ErrStagesAreNotTheSame
			}
		}

		pair := triplex{p: predicate, t: &oIBNode.typedResource}
		if err := assertUniqueSubjectTriplex(t, collectionIndex, pair); err != nil {
			return err
		}
		addTriplexAtOrAfter(t, collectionIndex, pair, subjectTriplexKind)
		indexIBNodeVocabulary(t, predicate, oIBNode)
		if oIBNode.isPredefinedIBNode() { // Predefined IBNode can not have triples
			return nil
		}
		addTriplexAtOrAfter(oIBNode, 0, triplex{p: predicate, t: &t.typedResource}, objectTriplexKind)
		if predicate.arePredicateTriplexesTracked() {
			addTriplexAtOrAfter(predicate, 0, triplex{p: t, t: &oIBNode.typedResource}, predicateTriplexKind)
		}
		return nil
	case TermKindLiteral:
		o := object.(Literal)
		literal := objectToLiteral(o)
		pair := triplex{p: predicate, t: &literal.typedResource}
		if err := assertUniqueSubjectTriplex(t, collectionIndex, pair); err != nil {
			return err
		}
		addTriplexAtOrAfter(t, collectionIndex, pair, subjectTriplexKind)
		if predicate.arePredicateTriplexesTracked() {
			addTriplexAtOrAfter(predicate, 0, triplex{p: t, t: &literal.typedResource}, predicateTriplexKind)
		}
		return nil
	case TermKindLiteralCollection:
		o := object.(*literalCollection)
		pair := triplex{p: predicate, t: &o.typedResource}
		if err := assertUniqueSubjectTriplex(t, collectionIndex, pair); err != nil {
			return err
		}
		addTriplexAtOrAfter(t, collectionIndex, pair, subjectTriplexKind)
		if predicate.arePredicateTriplexesTracked() {
			addTriplexAtOrAfter(predicate, 0, triplex{p: t, t: &o.typedResource}, predicateTriplexKind)
		}
		return nil
	default:
		panicf("unexpected term kind %v", object.TermKind())
		panic("unreachable")
	}
}

func (t *ibNode) Delete() {
	t.assertModification()
	t.delete()
}

func (t *ibNode) delete() {
	if err := t.forAllTriplexes(func(index int, tx triplex, k triplexKind) (out triplex, err error) {
		switch k {
		case subjectTriplexKind:
			if isResourceTypeIBNode(tx.t) {
				tx.t.asIBNode().deleteTriplex(triplex{
					p: tx.p,
					t: &t.typedResource,
				}, objectTriplexKind)
			}
			if tx.p.arePredicateTriplexesTracked() {
				tx.p.deleteTriplex(triplex{
					p: t,
					t: tx.t,
				}, predicateTriplexKind)
			}
		case objectTriplexKind:
			tx.t.asIBNode().deleteTriplex(triplex{
				p: tx.p,
				t: &t.typedResource,
			}, subjectTriplexKind)
			if tx.p.arePredicateTriplexesTracked() {
				tx.p.deleteTriplex(triplex{
					p: tx.t.asIBNode(),
					t: &t.typedResource,
				}, predicateTriplexKind)
			}
		case predicateTriplexKind:
			tx.p.deleteTriplex(triplex{
				p: t,
				t: tx.t,
			}, subjectTriplexKind)
			if isResourceTypeIBNode(tx.t) {
				tx.t.asIBNode().deleteTriplex(triplex{
					p: t,
					t: &tx.p.typedResource,
				}, objectTriplexKind)
			}
		}
		t.deleteTriplexAt(index)
		return
	}); err != nil {
		panic(err)
	}
	t.ng.deleteNode(t)
	t.ng.flags.modified = true
	t.ng = nil
	t.triplexStart, t.triplexEnd = 0, 0
}

func (t *ibNode) deleteSubjectTriplex() error {
	t.assertModification()
	if err := t.forAllTriplexes(func(index int, tx triplex, k triplexKind) (out triplex, err error) {
		switch k {
		case subjectTriplexKind:
			if isResourceTypeIBNode(tx.t) {
				tx.t.asIBNode().deleteTriplex(triplex{
					p: tx.p,
					t: &t.typedResource,
				}, objectTriplexKind)
			}
			if tx.p.arePredicateTriplexesTracked() {
				tx.p.deleteTriplex(triplex{
					p: t,
					t: tx.t,
				}, predicateTriplexKind)
			}
			t.deleteTriplexAt(index)
		}
		return
	}); err != nil {
		return err
	}

	return nil
}

// InVocabulary returns a corresponding ElementInformer for the IBNode.
// It is legal to invoke this method for the zero valued IBNode as well.
// This method is in most cases invoked for IBNodes from ReferencedGraphs.
//
// This method often used to determine the actual ElementInformer interface
// which can be done by using "Type Switch" or "Type Assertion" from the Golang specification.
//
// Example of using Type Switch:
//
//	var d sst.IBNode
//	switch d.TypeOf().InVocabulary().(type) {
//	case sso.KindPart:
//		// d is sso:Part or any of its subtypes
//		// ...
//	}
//
// Example of using Type Assertion:
//
//	 if _, ok := p.InVocabulary().(lci.KindHasArrangedPart); ok {
//		    // do something
//
//     }
func (t *ibNode) InVocabulary() ElementInformer {
	return t.inVocabulary()
}

func (t *ibNode) inVocabulary() ElementInformer {
	if t.ng == nil {
		return nil
	}
	ve := vocabularyElementOf(t)
	i, err := ve.Vocabulary.ElementInformer(ve.Name)
	if err != nil {
		GlobalLogger.Debug("IBNode.InVocabulary: cannot get ElementInformer for IBNode",
			zap.String("baseIRI", ve.Vocabulary.BaseIRI), zap.String("name", string(ve.Name)))
		// return ElementInfo{Element: ve}
		return nil
	}

	return i
}

func vocabularyElementOf(t *ibNode) (e Element) {
	if t == nil {
		return
	}
	if t.ng.triplexStorage != nil {
		e.Vocabulary.BaseIRI = t.ng.baseIRI
	}
	if !t.IsBlankNode() {
		sf := t.Fragment()
		e.Name = string(sf)
	}
	return
}

// Is checks if the IBNode is of given vocabulary element type.
// It is legal to invoke this method for the zero valued IBNode as well.
// This method is in most cases invoked for IBNodes from ReferencedGraphs.
func (t *ibNode) Is(e Elementer) bool {
	return t.isVocabularyElement(e.VocabularyElement())
}

func (t *ibNode) isVocabularyElement(e Element) bool {
	if t.ng == nil {
		return false
	}
	if e.Vocabulary.BaseIRI != t.ng.baseIRI {
		return false
	}

	sf := t.Fragment()
	return string(e.Name) == sf
}

func (t *ibNode) IsKind(e ElementInformer) bool {
	te := t.inVocabulary()
	return isKind(te, e)
}

func (t *ibNode) IsValid() bool {
	return t.ng != nil
}

// objectToLiteral converts a Literal to a literal.
// Literal can be of type LangString, String, Double, Integer, Boolean.
func objectToLiteral(ol Literal) *literal {
	if ol, ok := ol.(interface{ asLiteral() *literal }); ok {
		return ol.asLiteral()
	}
	literal := literal{
		typedResource: typedResource{typeOf: ol.DataType().(*ibNode)},
	}
	switch dt := ol.DataType(); dt {
	case &literalTypeLangString.ibNode:
		ls := ol.(LangString)
		// add language tag to the language value as suffix
		literal.value = ls.Val + ls.LangTag
	case &literalTypeString.ibNode:
		literal.value = string(ol.apiValue().(String))
	case &literalTypeDouble.ibNode:
		literal.value = float64Bytes(float64(ol.apiValue().(Double)))
	case &literalTypeFloat.ibNode:
		literal.value = float32Bytes(float32(ol.apiValue().(Float)))
	case &literalTypeInteger.ibNode:
		literal.value = int64Bytes(int64(ol.apiValue().(Integer)))
	case &literalTypeBoolean.ibNode:
		bs := ""
		if bool(ol.apiValue().(Boolean)) {
			bs = "t"
		}
		literal.value = bs
	case &literalTypeByte.ibNode:
		literal.value = int8Bytes(int8(ol.apiValue().(Byte)))
	case &literalTypeShort.ibNode:
		literal.value = int16Bytes(int16(ol.apiValue().(Short)))
	case &literalTypeInt.ibNode:
		literal.value = int32Bytes(int32(ol.apiValue().(Int)))
	case &literalTypeLong.ibNode:
		literal.value = int64Bytes(int64(ol.apiValue().(Long)))
	case &literalTypeUnsignedByte.ibNode:
		literal.value = uint8Bytes(uint8(ol.apiValue().(UnsignedByte)))
	case &literalTypeUnsignedShort.ibNode:
		literal.value = uint16Bytes(uint16(ol.apiValue().(UnsignedShort)))
	case &literalTypeUnsignedInt.ibNode:
		literal.value = uint32Bytes(uint32(ol.apiValue().(UnsignedInt)))
	case &literalTypeUnsignedLong.ibNode:
		literal.value = uint64Bytes(uint64(ol.apiValue().(UnsignedLong)))
	case &literalTypeDateTime.ibNode:
		literal.value = ol.apiValue().(TypedString).Val
	case &literalTypeDateTimeStamp.ibNode:
		literal.value = ol.apiValue().(TypedString).Val
	default:
		panic(fmt.Sprintf("unsupported literal data type: %v", ol.DataType().IRI()))
	}
	return &literal
}

type triplexKind int8

const (
	subjectTriplexKind = triplexKind(iota)
	objectTriplexKind
	predicateTriplexKind
)
const (
	uintShiftCount = bits.UintSize/32 + 4                               // 6
	_, _           = uint(uintShiftCount - 5), uint(6 - uintShiftCount) // Assert that uintShiftCount is either 6 or 5
	uintShiftMask  = 1<<uintShiftCount - 1
	uintAllMask    = 1<<bits.UintSize - 1
)

const (
	triplexKindBitCount    = 2
	triplexKindBitMask     = (1 << triplexKindBitCount) - 1           // 3
	triplexKindShiftOffset = uintShiftCount - triplexKindBitCount + 1 // 5
	triplexKindShiftMask   = 1<<triplexKindShiftOffset - 1            // 31
)

// the function checks on whether the IBNode t already contains the triplex tx
// for speed optimization, the comparison might not start for all triples but at an offset named index
func assertUniqueSubjectTriplex(t *ibNode, index triplexOffset, tx triplex) error {
	for i := t.triplexStart + index; i < t.triplexEnd; i++ {
		tTx := t.ng.triplexStorage[i]
		if tTx.p == nil || triplexKindAtAbs(t.ng, i) != subjectTriplexKind {
			continue
		}
		if triplexesEqual(tTx, tx, false) {
			return ErrDuplicateTriplex
		}
	}
	return nil
}

// insertUniqueTriplex function ensures that a triplex is inserted into the triplexStorage of an ibNode at a specific index,
// while maintaining uniqueness and handling storage expansion if necessary.
func insertUniqueTriplex(t *ibNode, index triplexOffset, tx triplex, kind triplexKind) {
	// reach to the end of tripleEnd, expand and add the triplex
	if t.triplexStart+index == t.triplexEnd {
		addGrowTriplex(t, tx, kind)
		return
	}
	// if the index position is already occupied by a triplex,
	// move this triplex to the next position
	current := t.ng.triplexStorage[t.triplexStart:t.triplexEnd][index]
	if current.p != nil {
		addTriplexAtOrAfter(t, index+1, current, t.triplexKindAt(int(index)))
	}

	// insert the tx triplex at the index
	t.ng.triplexStorage[t.triplexStart+index] = tx
	setTriplexKindAbs(t.ng, t.triplexStart+index, kind)
}

// Find an appropriate spot in an ibNode type object to insert a triplex,
// and if a suitable location is found, insert it,
// otherwise expand the storage space to accommodate the new triplex
func addTriplexAtOrAfter(t *ibNode, index triplexOffset, tx triplex, kind triplexKind) {
	// This variable, holeIndex, represents the calculated insertion position
	// and is calculated based on the given index and the triplexStart of node t.
	holeIndex := t.triplexStart + index

	// The purpose of this is to confirm whether the currently calculated holeIndex is already triplexEnd,
	// that is, the insertion position has reached the end of the array,
	// or the insertion point triplexStorage[holeIndex].p already has data.
	// If the above conditions are met, it means that the current point cannot be inserted directly,
	// and a new location needs to be found.
	if holeIndex == t.triplexEnd || t.ng.triplexStorage[holeIndex].p != nil {
		// Here we use sort. Search method to search for free insert locations starting from holeIndex, sort.
		// Search traverses the triplexStorage array, starting with holeIndex,
		// to find the first free position (i.e., p is nil).
		i := sort.Search(int(t.triplexEnd-holeIndex), func(i int) bool {
			tTx := t.ng.triplexStorage[holeIndex+triplexOffset(i)]
			return tTx.p == nil
		})
		holeIndex += triplexOffset(i)
	}
	// If the found holeIndex is valid and the location is indeed empty,
	// then insert tx into the location and set the type of triplex.
	if holeIndex < t.triplexEnd && t.ng.triplexStorage[holeIndex].p == nil {
		t.ng.triplexStorage[holeIndex] = tx
		setTriplexKindAbs(t.ng, holeIndex, kind)
		return
	}

	// If no suitable insertion point is found,
	// the function calls addGrowTriplex to expand the storage space and insert the triplex.
	addGrowTriplex(t, tx, kind)
}

// The main purpose of this function, addGrowTriplex, is to expand the triplexStorage array
// and insert a new triplex into the ibNode if no free place can be found to insert it.
func addGrowTriplex(t *ibNode, tx triplex, kind triplexKind) {
	// 	namedGraph holds the reference t.ng of ibNode.
	// triplexes are slices of the active triplex in the current triplexStorage array.
	// prevNodeTriplexLen is the length of the triplex stored by the current node.
	// prevTriplexStorageLen is the current length of the entire triplexStorage, including the free portion.
	namedGraph := t.ng
	triplexes := t.ng.triplexStorage[t.triplexStart:t.triplexEnd]
	prevNodeTriplexLen := len(triplexes)
	prevTriplexStorageLen := triplexOffset(len(namedGraph.triplexStorage))
	newTriplexAllocCount := triplexOffset(triplexAllocCount)

	// newTriplexAllocCount is the new allocation space size,
	// and the initial value is the constant triplexAllocCount.
	// If the current slice size (t.triplexEnd - t.triplexStart) exceeds the newTriplexAllocCount,
	// set the new allocation space size to twice the current slice size.
	if t.triplexEnd-t.triplexStart > newTriplexAllocCount {
		newTriplexAllocCount = (t.triplexEnd - t.triplexStart) << 1
	}

	// appendLen represents the length of the actual growth required.
	// If the existing storage space has been exhausted (i.e., prevTriplexStorageLen == t.triplexEnd),
	// only the new allocation size of newTriplexAllocCount is allocated.
	appendLen := t.triplexEnd - t.triplexStart + newTriplexAllocCount
	if prevTriplexStorageLen == t.triplexEnd {
		appendLen = newTriplexAllocCount
	}

	// 	growLen represents the growth length of the new storage space.
	// This growth length is influenced by two conditions:
	// If the growLen exceeds a certain maximum growth threshold, triplesMaxGrowThreshold,
	// the growth length is reduced.
	// If growLen is less than a certain minimum growth value, triplexMinGrow, increase the growth length.
	// Eventually, growLen takes the maximum value of appendLen and the calculated growLen.
	growLen := int(prevTriplexStorageLen)
	switch {
	case growLen > triplesMaxGrowThreshold:
		growLen = int(prevTriplexStorageLen >> 1)
	case growLen < triplexMinGrow:
		growLen = triplexMinGrow
	}
	if growLen < int(appendLen) {
		growLen = int(appendLen)
	}

	// Use append to extend the triplexStorage array to increase the space for growLen triplex.
	// Then, take a new triplexStorage length to make sure the space expands correctly.
	namedGraph.triplexStorage = append(namedGraph.triplexStorage, make([]triplex, growLen)...)
	namedGraph.triplexStorage = namedGraph.triplexStorage[:prevTriplexStorageLen+appendLen]

	// If the end of triplexStorage has already reached the end of the storage space, update triplexEnd directly.
	// Otherwise, copy the existing triplexes to the new storage space and adjust the start and end indexes.
	if prevTriplexStorageLen == t.triplexEnd {
		t.triplexEnd += newTriplexAllocCount
	} else {
		triplexes = namedGraph.triplexStorage[int(t.triplexStart):int(t.triplexEnd)]
		copy(namedGraph.triplexStorage[prevTriplexStorageLen:len(namedGraph.triplexStorage)], triplexes)
		for i := range triplexes {
			setTriplexKindAbs(namedGraph, prevTriplexStorageLen+triplexOffset(i), t.triplexKindAt(i))
			triplexes[i] = triplex{}
		}
		t.triplexStart, t.triplexEnd = prevTriplexStorageLen, triplexOffset(len(namedGraph.triplexStorage))
	}

	// In the new storage space, insert the incoming triplex and set its type.
	triplexes = namedGraph.triplexStorage[int(t.triplexStart):int(t.triplexEnd)]
	triplexes[prevNodeTriplexLen] = tx
	setTriplexKindAbs(namedGraph, t.triplexStart+triplexOffset(prevNodeTriplexLen), kind)
}

func appendTriplex(t *ibNode, tx triplex, kind triplexKind) {
	if int(t.triplexEnd) >= len(t.ng.triplexStorage) {
		var objStr string
		if tx.t == nil {
			objStr = "nil"
		} else {
			switch tx.t.resourceType(false) {
			case resourceTypeIBNode:
				objStr = tx.t.asIBNode().iriOrID()
			case resourceTypeLiteral:
				objStr = literalToString(tx.t.asLiteral())
			case resourceTypeLiteralCollection:
				lc := tx.t.asLiteralCollection()
				members := make([]string, 0, lc.MemberCount())
				lc.ForMembers(func(_ int, l Literal) {
					members = append(members, literalToString(l))
				})
				objStr = fmt.Sprintf("LiteralCollection(%v)", members)
			default:
				objStr = "unknown"
			}
		}
		panic(fmt.Sprintf(
			"appendTriplex: triplex storage overflow in graph %q node %q: "+
				"triplexStart=%d triplexEnd=%d storageLen=%d kind=%v pred=%q obj=%s",
			t.ng.baseIRI, t.iriOrID(), t.triplexStart, t.triplexEnd, len(t.ng.triplexStorage), kind, tx.p.iriOrID(), objStr,
		))
	}
	t.ng.triplexStorage[t.triplexEnd] = tx
	t.triplexEnd++
	switch kind {
	case subjectTriplexKind:
		if tx.t != nil && isResourceTypeIBNode(tx.t) {
			o := tx.t.asIBNode()
			indexIBNodeVocabulary(t, tx.p, o)
		}
	case objectTriplexKind, predicateTriplexKind:
		setTriplexKindAbs(t.ng, t.triplexEnd-1, kind)
	}
}

// set/update ibNode.typeOf after any addition of triples => AddStatement() and others , reading TTL ...
// t is triple subject which ibNode.typeOf is updated
// p is triple predicate
// o is triple object
func indexIBNodeVocabulary(t, p, o *ibNode) {
	if vocabularyMap != nil {
		if p.isVocabularyElement(rdfType) {
			var prevMainClass Element
			if t.typeOf != nil {
				pi := t.typeOf.inVocabulary()
				if pi != nil {
					if !pi.IsDatatypeProperty() {
						prevMainClass = pi.VocabularyElement()
					}
				}
			}
			ot := o.inVocabulary()
			if ot != nil {
				if ot.IsMainClass(prevMainClass) {
					t.typeOf = o
				}
			} else {
				// this vocabulary is not saved in vocabularyMap
				// e.g. countrycodes
			}
		} else if p.isVocabularyElement(rdfsSubPropertyOf) {
			ot := o.inVocabulary()
			if ot != nil {
				if ot.IsDatatypeProperty() {
					if t.typeOf == nil {
						t.typeOf = o
					}
				}
			} else {
				// this vocabulary is not saved in vocabularyMap
				// e.g. countrycodes
			}
		}
	}
}

func replaceOneOfTriplexes(
	t *ibNode,
	oldTx1, oldTx2 *ibNode,
	oldT1, oldT2 *typedResource,
	newPair triplex,
	kind triplexKind,
) {
	triplexes := t.ng.triplexStorage[int(t.triplexStart):int(t.triplexEnd)]
	var replaced bool
	for i, tx := range triplexes {
		if tx.p == nil {
			continue
		}
		if kind == t.triplexKindAt(i) {
			if (tx.p == oldTx1 || tx.p == oldTx2) && (tx.t == oldT1 || tx.t == oldT2) {
				triplexes[i] = newPair
				replaced = maybeDeleteDuplicatedTriplexAfterReplaceAt(t, replaced, i)
			} else if tx.p == newPair.p && tx.t == newPair.t {
				replaced = maybeDeleteDuplicatedTriplexAfterReplaceAt(t, replaced, i)
			}
		}
	}
	if !replaced {
		panic(ErrNodeTripleNotSet)
	}
}

func replaceOneOfTriplexesByValue[V valueResource[V]](
	t *ibNode, oldTx1, oldTx2 *ibNode, newP *ibNode, v V, kind triplexKind,
) {
	triplexes := t.ng.triplexStorage[int(t.triplexStart):int(t.triplexEnd)]
	var replaced bool
	for i, tx := range triplexes {
		if tx.p == nil {
			continue
		}
		if t.triplexKindAt(i) == kind && resourceTypeRecursive(tx.t) == v.validResourceType() {
			if (tx.p == oldTx1 || tx.p == oldTx2) && areComparableResourcesEqual(tx.t, v) {
				triplexes[i] = triplex{p: newP, t: v.typedResourcePtr()}
				replaced = maybeDeleteDuplicatedTriplexAfterReplaceAt(t, replaced, i)
			} else if tx.p == newP && areComparableResourcesEqual(tx.t, v) {
				replaced = maybeDeleteDuplicatedTriplexAfterReplaceAt(t, replaced, i)
			}
		}
	}
	if !replaced {
		panic(ErrNodeTripleNotSet)
	}
}

func areComparableResourcesEqual[V comparableResource[V]](r1 *typedResource, v2 V) bool {
	return (*new(V)).fromTypedResourcePtr(r1).isEqualTo(v2)
}

func maybeDeleteDuplicatedTriplexAfterReplaceAt(t *ibNode, replaced bool, index int) bool {
	if replaced {
		t.deleteTriplexAt(index)
	} else {
		replaced = true
	}
	return replaced
}

func (t *ibNode) deleteTriplex(tx triplex, kind triplexKind) {
	triplexes := t.ng.triplexStorage[int(t.triplexStart):int(t.triplexEnd)]
	for i := range triplexes {
		if kind == t.triplexKindAt(i) && triplexesEqual(triplexes[i], tx, true) {
			t.deleteTriplexAt(i)
			return
		}
	}
	panic(ErrNodeTripleNotSet)
}

// clean out a triplex of an ibNode at a specified location
func (t *ibNode) deleteTriplexAt(index int) {
	t.ng.triplexStorage[int(t.triplexStart):int(t.triplexEnd)][index] = triplex{}
	setTriplexKindAbs(t.ng, t.triplexStart+triplexOffset(index), subjectTriplexKind)
}

// triplexKindAt returns the triplexKind at a specified offset from triplexStart.
func (t *ibNode) triplexKindAt(i int) triplexKind {
	return triplexKindAtAbs(t.ng, t.triplexStart+triplexOffset(i))
}

// absIndex is an uint32 that is used to
// - indicate the position of the uint in the packed triplexKinds slice
// - and which 2 bits of the uint to use for the triplexKind
// for 64 bit machine, 32 triplex kinds can be stored in one uint
// to tell a number between 0 and 31, we need 5 bits
func triplexKindAtAbs(namedGraph *namedGraph, absIndex triplexOffset) triplexKind {
	// pos said which uint in the packed triplexKinds slice to use
	pos := int(absIndex) >> triplexKindShiftOffset
	kinds := namedGraph.triplexKinds
	// check if the pos is valid, if not, it must be a subject triple kind
	// that means there is no triplex kind information stored yet
	if pos >= len(kinds) {
		return subjectTriplexKind
	}
	// mask out the lower 5 bits and shift them by 1 to the left
	shift := (absIndex & triplexKindShiftMask) << (triplexKindBitCount - 1)
	return triplexKind((kinds[pos] >> shift) & triplexKindBitMask)
}

func setTriplexKindAbs(namedGraph *namedGraph, absIndex triplexOffset, kind triplexKind) {
	// Calculate the index position pos in the triplexKinds array,
	// by shifting the triplexKindShiftOffset number to the right.
	// This represents the position of the triplex in the triplexKinds array.
	pos := int(absIndex) >> triplexKindShiftOffset

	// If the pos exceeds the length of the triplexKinds array, and kind is not equal to subjectTriplexKind,
	// you will need to expand the triplexKinds array to accommodate the new triplex type information.
	if pos >= len(namedGraph.triplexKinds) {
		// If the kind is subjectTriplexKind, it means that the triplex type information is not needed,
		// so return without expanding the array.
		if kind == subjectTriplexKind {
			return
		}
		namedGraph.triplexKinds = append(namedGraph.triplexKinds, make([]uint, pos-len(namedGraph.triplexKinds)+1)...)
	}
	// shift: Calculates which bit position in the triplexKinds array should be set to kind, and obtains the specific offset by bit arithmetic.
	// mask: Generates a mask that clears the existing value of the bit position.
	shift := (absIndex & triplexKindShiftMask) << (triplexKindBitCount - 1)
	mask := uint(triplexKindBitMask) << shift

	// Use mask to clear the existing value for the corresponding location in triplexKinds[pos].
	// Then place the value of kind in the corresponding bit position.
	value := uint(kind) << shift
	namedGraph.triplexKinds[pos] = (namedGraph.triplexKinds[pos] & ^mask) | value
}

func appendTriplexKinds(to []uint, toOffset int, from []uint, fromShift, fromEndMask uint) []uint {
	highBitsShift := uint(toOffset & uintShiftMask)
	prevPos := toOffset >> uintShiftCount
	curPos := prevPos + 1
	for i, kindsLow := range from {
		if i+1 == len(from) {
			kindsLow &= fromEndMask
		}
		kinds := kindsLow >> fromShift
		if i+1 < len(from) && fromShift != 0 {
			fromNext := from[i+1]
			if i+2 == len(from) {
				fromNext &= fromEndMask
			}
			kinds |= fromNext << (bits.UintSize - fromShift)
		}
		highBits := kinds << highBitsShift
		if highBits != 0 {
			to = setGrowableUintAt(to, prevPos, growableUintAt(to, prevPos)|highBits)
		}
		lowBits := kinds >> (uint(bits.UintSize) - highBitsShift)
		to = setGrowableUintAt(to, curPos, lowBits)
		curPos, prevPos = curPos+1, curPos
	}
	return to
}

func growableUintAt(s []uint, i int) uint {
	if len(s) <= i {
		return 0
	}
	return s[i]
}

func setGrowableUintAt(s []uint, i int, v uint) []uint {
	a := i - len(s) + 1
	if a > 0 {
		if v == 0 {
			return s
		}
		s = append(s, make([]uint, a)...)
	}
	s[i] = v
	return s
}
