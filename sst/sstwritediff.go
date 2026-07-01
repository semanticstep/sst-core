// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"go.uber.org/zap"
)

// SST Diff File
// ├── IMPORTED GRAPHS HEADER
// │   ├── [varint] Removed Count
// │   ├── [varint] Added Count
// │   └── For each merged graph (sorted by IRI):
// │       ├── [varint] Status (0=Same, 1=Removed, 2=Added)
// │       └── If Same: [varint] Span
// │           Removed: [str] Graph IRI
// │           Added:   [str] Graph IRI
// │
// ├── REFERENCED GRAPHS HEADER
// │   └── (same structure as imported)
// │
// ├── CURRENT GRAPH DICTIONARY
// │   ├── [varint] IRI Removed Count
// │   ├── [varint] IRI Added Count
// │   ├── [varint] Blank Removed Count
// │   ├── [varint] Blank Added Count
// │   │
// │   ├── IRI Nodes Status Sequence
// │   │   └── For each merged node:
// │   │       ├── [varint] Status (0-3)
// │   │       └── If Same:       [varint] Span
// │   │           Removed:       [str] Fragment, [int64] TriplexCntDelta
// │   │           Added:         [str] Fragment, [int64] TriplexCntDelta
// │   │           TripleModified:  [int64] TriplexCntDelta
// │   │
// │   └── Blank Nodes Status Sequence
// │       └── (same, no Fragment for Removed/Added)
// │
// ├── IMPORTED GRAPHS DICTIONARY
// │   └── For each import graph:
// │       ├── [varint] Deleted Count
// │       ├── [varint] Added Count
// │       └── Node Status Sequence (same as current graph)
// │
// ├── REFERENCED GRAPHS DICTIONARY
// │   └── (same structure as imported)
// │
// └── CONTENT
//     └── For each node (in dictionary order):
//         ├── If identical nodes span > 0:
//         │   └── [varint] span << 1  (even = unchanged)
//         │
//         └── If modified:
//             ├── NON-LITERAL TRIPLES
//             │   ├── [varint] (removed << 1) | 1  (odd = modified)
//             │   ├── [varint] added
//             │   └── If removed+added > 0, Triple Changes:
//             │       ├── [varint] sameCnt << 2     (flag 00 = same count)
//             │       ├── [varint] pred<<2 | 1, [varint] obj   (flag 01 = removed)
//             │       └── [varint] pred<<2 | 2, [varint] obj   (flag 10 = added)
//             │
//             └── LITERAL TRIPLES (if any)
//                 ├── [varint] (removed << 1) | 1
//                 ├── [varint] added
//                 └── If removed+added > 0, Literal Changes:
//                     ├── [varint] sameCnt << 2
//                     ├── [varint] pred<<2 | 1, [varint] kind, [value]  (removed)
//                     └── [varint] pred<<2 | 2, [varint] kind, [value]  (added)

// Node/Graph status flags
// diffEntrySame           = 0  // Unchanged
// diffEntryRemoved        = 1  // Removed
// diffEntryAdded          = 2  // Added
// diffEntryTripleModified = 3  // Triple count changed

// // Content area flag bits
// diffIdenticalOrModifiedOffset = 1       // Left shift bits
// diffModifiedFlag              = 0b01    // Modified marker
// diffSameRemovedOrAddedOffset  = 2       // Left shift bits
// diffSameRemovedOrAddedMask    = 0b11    // Low 2 bits mask
// diffRemovedFlag               = 0b01    // Removed marker
// diffAddedFlag                 = 0b10    // Added marker

// Diff File Structure
//
// Header Section
// 	for imported NamedGraphs
// 		number of removed NamedGraphs
// 		number of added NamedGraphs
// 	for referenced NamedGraphs
// 		number of removed NamedGraphs
// 		number of added NamedGraphs

// Dictionary Section
// 	for current NamedGraph
// 		number of removed IRI Nodes
// 		number of added IRI Nodes
// 		number of removed Blank Nodes
// 		number of added Blank Nodes
// 		Node Status Changes (diffEntrySame, diffEntryRemoved, diffEntryAdded and diffEntryTripleModified one by one listed)
//          diffEntrySame tag
//          	same Node count
//          diffEntryRemoved tag
//          	removed Node IRI
//              removed Triple Count
//          diffEntryAdded tag
//          	added Node IRI
//              added Triple Count
//          diffEntryTripleModified tag
//		  		modified Triple Count
// 	for each imported NamedGraph
// 	for each Referenced NamedGraph
// 		Node Status Changes (diffEntrySame, diffEntryRemoved, diffEntryAdded and diffEntryTripleModified one by one listed)

// Content Section
// 	for each Node in current NamedGraph
// 		identical Node count
// 		Node Triple Changes
// 			non-literal triple removed count
// 			non-literal triple added count
// 			modified triples
// 				predicate index
// 				object index
// 			literal triple removed count
// 			literal triple added count
// 			modified literal triples
// 				predicate index
// 				literal kind
// 				literal value

var ErrGraphIRIsMismatch = errors.New("graph URLs mismatch")

type translationOffsets struct{ from, to, combined uint }

const (
	diffEntrySame = diffEntry(iota)
	diffEntryRemoved
	diffEntryAdded
	diffEntryTripleModified
)

const (
	diffIdenticalOrModifiedOffset = 1 // for shifting identical Node count and triple modified flag
	diffModifiedFlag              = 0b01
	diffSameRemovedOrAddedOffset  = 2                                       // for shifting predicate index
	diffSameRemovedOrAddedMask    = (1 << diffSameRemovedOrAddedOffset) - 1 //0b11
	diffRemovedFlag               = 0b01
	diffAddedFlag                 = 0b10
)

// represent and manage information about graph imports or references when generating a diff between two SST files.
type diffWriteGraph struct {
	// A string representing the URI of the graph.
	graphIRI []string

	// An integer representing the index of the graph in the "from" SST file.
	// If the graph does not exist in the "from" file, this is set to -1.
	fromIndex int

	// An integer representing the index of the graph in the "to" SST file.
	// If the graph does not exist in the "to" file, this is set to -1.
	toIndex int

	// A uint representing the number of consecutive graphs that are identical or share the same characteristics.
	// This is used to optimize the representation of unchanged graphs by grouping them together.
	span uint
}

type (
	nodeTripleT = struct {
		predIndex    uint
		objIndex     uint
		tripleSource int
	}
	diffLiteralT = struct {
		kind  writtenLiteralKind
		value interface{}
	}
	predicateIndexWithLiteralT = struct {
		pred uint
		diffLiteralT
	}
)

type diffWriteNodeDetails struct {
	// for recording fragments of IBNodes with same delta triple count
	fragment        []string
	fragmentForText []string

	// delta triple count
	triplexCntDelta int

	// same count of IBNodes with same delta triple count
	// In most cases, this span will be 1.
	// This number will only be increased when there are adjacent IBNodes that are both existed in "from" and "to" SST
	// files and have the same delta triple count.
	span uint
}

// Describe a class of IBNodes with the same variation
type diffWriteCombinedNode struct {
	// details is used for recording
	details []diffWriteNodeDetails

	// fromIndexStart is the starting index of this node in the "from" SST file.
	// fromIndexEnd is the ending index of this kind(same, added or removed) of node in the "from" SST file.
	// If the node does not exist in the "from" file, fromIndexStart and fromIndexEnd are set to -1.
	// fromIndexEnd - fromIndexStart is the count of adjacent IBNodes that have the same change (same, added or removed).
	// If there is no adjacent IBNodes that have the same change, fromIndexEnd - fromIndexStart = 1
	// If there are 2 adjacent IBNodes that are both existed in "from" and "to" SST files, fromIndexEnd - fromIndexStart = 2
	// If there are 2 adjacent IBNodes that are removed in "from" SST files, fromIndexEnd - fromIndexStart = 2
	// ...
	fromIndexStart int
	fromIndexEnd   int

	// toIndexStart and toIndexEnd are for "to" SST file.
	toIndexStart int
	toIndexEnd   int
}

type diffWriteContext struct {
	sortedImportedGraphs   []diffWriteGraph
	sortedReferencedGraphs []diffWriteGraph
	iriNodes               []diffWriteCombinedNode
	blankNodes             []diffWriteCombinedNode
	fromTranslations       []originToTranslated
	toTranslations         []originToTranslated

	varintBuf varintBufT

	identicalContentSpan uint
	nodeTripleBuf        []nodeTripleT
	literalTripleBuf     []predicateIndexWithLiteralT

	// for text output
	fromNodesDict  map[int]string
	fromIndex      int
	combinedToFrom map[int]int
	// initialized to -1 to indicate that rdf:first is not found, otherwise it is the index of rdf:first in the fromNodesDict
	fromRdfFirstIndex int

	toNodesDict  map[int]string
	toIndex      int
	combinedToto map[int]int
	// initialized to -1 to indicate that rdf:first is not found, otherwise it is the index of rdf:first in the toNodesDict
	toRdfFirstIndex int

	diffTripleIndex        []diffTripleIndex
	diffLiteralTripleIndex []diffLiteralTripleIndex
	// end
}

// DiffTripleFlag is used in DiffTriple to indicate modifications.
// The possible values are
//
//	-1: Triple is removed, indicated by "-"
//	 0: Triple is not changed, indicated by "="
//	+1: Triple is added, indicated by "+"
type DiffTripleFlag int8

const (
	TripleAdded   DiffTripleFlag = 1
	TripleEqual   DiffTripleFlag = 0
	TripleRemoved DiffTripleFlag = -1
)

type diffTripleIndex struct {
	flag DiffTripleFlag
	subj string
	// use index will be better for blank Nodes
	// sub         uint
	pred uint
	obj  uint
	// if subj is TermCollection, then value of member is 0, 1, 2... indicating the position of the member in the TermCollection
	// otherwise member is -1.
	member int
}

type diffLiteralTripleIndex struct {
	flag DiffTripleFlag
	subj string
	// use index will be better for blank Nodes
	// sub         uint
	pred uint
	obj  Term
}

type DiffTriple struct {
	Flag DiffTripleFlag
	Sub  string
	Pred string
	Obj  string
}

// SstWriteDiff compares two binary SST files and generates a binary diff file that represents the differences between them.
// It can also optionally return a list of diff triples describing the changes.
// Parameters:
// rFrom: Reader for the original SST file.
// rTo: Reader for the target SST file.
// w: Writer to output the diff file.
// writeDiffTriple: If true, returns a slice of DiffTriple describing the changes.
// Returns:
// A slice of DiffTriple (if writeDiffTriple is true), otherwise nil.
// An error if the operation fails.
func SstWriteDiff(rFrom, rTo *bufio.Reader, w io.Writer, writeDiffTriple bool) ([]DiffTriple, error) {
	GlobalLogger.Debug("SstWriteDiff started", zap.Bool("writeDiffTriple", writeDiffTriple))
	err := readHeaderMagic(rFrom)
	if err != nil {
		GlobalLogger.Debug("SstWriteDiff failed to read header magic from rFrom", zap.Error(err))
		return nil, err
	}
	err = readHeaderMagic(rTo)
	if err != nil {
		GlobalLogger.Debug("SstWriteDiff failed to read header magic from rTo", zap.Error(err))
		return nil, err
	}
	// read graph IRI
	gFromIRI, err := readString(rFrom)
	if err != nil {
		GlobalLogger.Debug("SstWriteDiff failed to read graph IRI from rFrom", zap.Error(err))
		return nil, err
	}
	gToIRI, err := readString(rTo)
	if err != nil {
		GlobalLogger.Debug("SstWriteDiff failed to read graph IRI from rTo", zap.Error(err))
		return nil, err
	}
	GlobalLogger.Debug("SstWriteDiff graph IRIs", zap.String("from", gFromIRI), zap.String("to", gToIRI))
	// check if graph IRIs are the same
	if gFromIRI != gToIRI {
		GlobalLogger.Debug("SstWriteDiff graph IRI mismatch", zap.String("from", gFromIRI), zap.String("to", gToIRI))
		return nil, ErrGraphIRIsMismatch
	}
	var dc diffWriteContext
	dc.fromNodesDict = make(map[int]string)
	dc.fromRdfFirstIndex = -1
	dc.toNodesDict = make(map[int]string)
	dc.toRdfFirstIndex = -1
	dc.combinedToFrom = make(map[int]int)
	dc.combinedToto = make(map[int]int)

	err = diffWriteHeader(rFrom, rTo, &dc, w)
	if err != nil {
		return nil, err
	}
	err = diffWriteDictionary(rFrom, rTo, &dc, gFromIRI, w)
	if err != nil {
		return nil, err
	}

	// after Dictionary is written, we have fromNodesDict and toNodesDict
	// we can look for rdf:first in both dicts
	for key, val := range dc.fromNodesDict {
		if val == "rdf:first" {
			dc.fromRdfFirstIndex = key
			break
		}
	}

	for key, val := range dc.toNodesDict {
		if val == "rdf:first" {
			dc.toRdfFirstIndex = key
			break
		}
	}

	err = diffWriteContent(rFrom, rTo, &dc, w)
	if err != nil {
		return nil, err
	}

	if writeDiffTriple {
		modifiedBlankNodes := make(map[string]struct{})

		// diffTripleIndex now have all triples, including equal, added and removed
		// get the blank node which is modified and put it into a map
		for _, di := range dc.diffTripleIndex {
			if di.flag == TripleAdded || di.flag == TripleRemoved {
				if strings.HasPrefix(di.subj, "_:b") {
					modifiedBlankNodes[di.subj] = struct{}{}
				}
			}
		}

		for _, di := range dc.diffLiteralTripleIndex {
			if di.flag == TripleAdded || di.flag == TripleRemoved {
				if strings.HasPrefix(di.subj, "_:b") {
					modifiedBlankNodes[di.subj] = struct{}{}
				}
			}
		}

		// Group triples by subject, preserving first-seen order of subjects.
		subjMap := make(map[string][]DiffTriple)
		subjOrder := make([]string, 0)
		ensureSubj := func(subj string) {
			if _, ok := subjMap[subj]; !ok {
				subjMap[subj] = nil
				subjOrder = append(subjOrder, subj)
			}
		}

		for _, diffIndex := range dc.diffTripleIndex {
			switch diffIndex.flag {
			case TripleEqual:
				// add non modified TermCollection triples
				if _, found := modifiedBlankNodes[diffIndex.subj]; found {
					// member count >= 0 means this triple is a Term Collection triple
					if diffIndex.member >= 0 {
						ensureSubj(diffIndex.subj)
						subjMap[diffIndex.subj] = append(subjMap[diffIndex.subj], DiffTriple{
							Flag: diffIndex.flag,
							Sub:  diffIndex.subj,
							Pred: dc.toNodesDict[dc.combinedToto[int(diffIndex.pred)]],
							Obj:  dc.toNodesDict[dc.combinedToto[int(diffIndex.obj)]],
						})
					}
				}

				// add triples which have modified blank nodes as object
				if _, found := modifiedBlankNodes[dc.toNodesDict[dc.combinedToto[int(diffIndex.obj)]]]; found {
					ensureSubj(diffIndex.subj)
					subjMap[diffIndex.subj] = append(subjMap[diffIndex.subj], DiffTriple{
						Flag: diffIndex.flag,
						Sub:  diffIndex.subj,
						Pred: dc.toNodesDict[dc.combinedToto[int(diffIndex.pred)]],
						Obj:  dc.toNodesDict[dc.combinedToto[int(diffIndex.obj)]],
					})
				}

			case TripleAdded:
				ensureSubj(diffIndex.subj)
				subjMap[diffIndex.subj] = append(subjMap[diffIndex.subj], DiffTriple{
					Flag: diffIndex.flag,
					Sub:  diffIndex.subj,
					Pred: dc.toNodesDict[dc.combinedToto[int(diffIndex.pred)]],
					Obj:  dc.toNodesDict[dc.combinedToto[int(diffIndex.obj)]],
				})

			case TripleRemoved:
				ensureSubj(diffIndex.subj)
				subjMap[diffIndex.subj] = append(subjMap[diffIndex.subj], DiffTriple{
					Flag: diffIndex.flag,
					Sub:  diffIndex.subj,
					Pred: dc.fromNodesDict[dc.combinedToFrom[int(diffIndex.pred)]],
					Obj:  dc.fromNodesDict[dc.combinedToFrom[int(diffIndex.obj)]],
				})
			}
		}

		for _, diffIndex := range dc.diffLiteralTripleIndex {
			var objStringValue string
			switch diffIndex.obj.TermKind() {
			case TermKindLiteral:
				objStringValue = literalToString(diffIndex.obj.(Literal))
			case TermKindLiteralCollection:
				objStringValue += "( "
				diffIndex.obj.(LiteralCollection).ForMembers(func(index int, li Literal) {
					objStringValue += literalToString(li) + " "
				})
				objStringValue += ")"
			}

			switch diffIndex.flag {
			case TripleEqual:
				ensureSubj(diffIndex.subj)
				subjMap[diffIndex.subj] = append(subjMap[diffIndex.subj], DiffTriple{
					Flag: diffIndex.flag,
					Sub:  diffIndex.subj,
					Pred: dc.toNodesDict[dc.combinedToto[int(diffIndex.pred)]],
					Obj:  objStringValue,
				})

			case TripleAdded:
				ensureSubj(diffIndex.subj)
				subjMap[diffIndex.subj] = append(subjMap[diffIndex.subj], DiffTriple{
					Flag: diffIndex.flag,
					Sub:  diffIndex.subj,
					Pred: dc.toNodesDict[dc.combinedToto[int(diffIndex.pred)]],
					Obj:  objStringValue,
				})

			case TripleRemoved:
				ensureSubj(diffIndex.subj)
				subjMap[diffIndex.subj] = append(subjMap[diffIndex.subj], DiffTriple{
					Flag: diffIndex.flag,
					Sub:  diffIndex.subj,
					Pred: dc.fromNodesDict[dc.combinedToFrom[int(diffIndex.pred)]],
					Obj:  objStringValue,
				})
			}
		}

		// Assemble final slice: per-subject groups in original order,
		// each group internally sorted by (Pred, Flag, Obj) so identical
		// (Sub, Pred) rows become adjacent and can be compacted.
		var diffTriples []DiffTriple
		for _, subj := range subjOrder {
			list := subjMap[subj]
			sort.SliceStable(list, func(i, j int) bool {
				if list[i].Pred != list[j].Pred {
					return list[i].Pred < list[j].Pred
				}
				if list[i].Flag != list[j].Flag {
					return list[i].Flag < list[j].Flag
				}
				return list[i].Obj < list[j].Obj
			})
			diffTriples = append(diffTriples, list...)
		}

		returnedDiffTriples := reorderCollectionsAfterFirstObjectReference(renderDiffWithCollections(diffTriples))

		// Compact: hierarchical elision.
		// If Sub is the same as the previous row, omit Sub.
		// If both Sub and Pred are the same, omit both.
		if len(returnedDiffTriples) > 0 {
			prevSub, prevPred := returnedDiffTriples[0].Sub, returnedDiffTriples[0].Pred
			for i := 1; i < len(returnedDiffTriples); i++ {
				currSub, currPred := returnedDiffTriples[i].Sub, returnedDiffTriples[i].Pred
				if currSub == prevSub {
					returnedDiffTriples[i].Sub = ""
					if currPred == prevPred {
						returnedDiffTriples[i].Pred = ""
					}
				}
				prevSub = currSub
				prevPred = currPred
			}
		}

		return returnedDiffTriples, nil
	} else {
		return nil, nil
	}
}

// container: old collections, new collections, and de-duplicates under the same subject
type agg struct {
	old, new         []string
	seenOld, seenNew map[string]struct{}
}

func (a *agg) addOld(item string) {
	if a.seenOld == nil {
		a.seenOld = make(map[string]struct{})
	}
	if _, ok := a.seenOld[item]; ok {
		return
	}
	a.seenOld[item] = struct{}{}
	a.old = append(a.old, item)
}
func (a *agg) addNew(item string) {
	if a.seenNew == nil {
		a.seenNew = make(map[string]struct{})
	}
	if _, ok := a.seenNew[item]; ok {
		return
	}
	a.seenNew[item] = struct{}{}
	a.new = append(a.new, item)
}

func renderDiffWithCollections(triples []DiffTriple) []DiffTriple {
	res := make([]DiffTriple, 0, len(triples))

	// The first occurrence order of subject
	subjOrder := []string{}
	aggs := make(map[string]*agg) // subj -> *agg

	ensure := func(subj string) *agg {
		a, ok := aggs[subj]
		if !ok {
			a = &agg{}
			aggs[subj] = a
			subjOrder = append(subjOrder, subj)
		}
		return a
	}

	// 1) scan：reserve non rdf:first triple；add rdf:first into old/new
	for _, t := range triples {
		if t.Pred == "rdf:first" &&
			(t.Flag == TripleRemoved || t.Flag == TripleEqual || t.Flag == TripleAdded) {

			a := ensure(t.Sub)

			// old collection includes {TripleEqual, TripleRemoved}
			if t.Flag != TripleAdded {
				a.addOld(t.Obj)
			}
			// new collection includes {TripleEqual, TripleAdded}
			if t.Flag != TripleRemoved {
				a.addNew(t.Obj)
			}
			continue
		}
		// other predicate stay same
		res = append(res, t)
	}

	// 2) append to res: follow subject order，first (old) then (new)
	render := func(items []string) string {
		return "( " + strings.Join(items, " ") + " )"
	}
	for _, subj := range subjOrder {
		a := aggs[subj]
		if len(a.old) > 0 {
			res = append(res, DiffTriple{
				Flag: TripleRemoved,
				Sub:  subj,
				Pred: "<TermCollection>",
				Obj:  render(a.old),
			})
		}
		if len(a.new) > 0 {
			res = append(res, DiffTriple{
				Flag: TripleAdded,
				Sub:  subj,
				Pred: "<TermCollection>",
				Obj:  render(a.new),
			})
		}
	}

	return res
}

// reorderCollectionsAfterFirstObjectReference
// Goal: Keep non-collection triples in original order (stable).
// For each <TermCollection> subject S, place its collection lines
// right AFTER the *first* triple whose object == S.
// If no such anchor triple exists for S, append the collection at the end.
// Inside each subject's collection group, enforce line order: -1, 0, +1.
func reorderCollectionsAfterFirstObjectReference(triples []DiffTriple) []DiffTriple {
	// 1) Split anchors (non-collection) and collection groups by subject
	anchors := make([]DiffTriple, 0, len(triples))
	groupBySubj := map[string][]DiffTriple{}
	subjOrder := []string{} // preserve first-seen subject order for collections

	isCollection := func(t DiffTriple) bool { return t.Pred == "<TermCollection>" }

	for _, t := range triples {
		if isCollection(t) {
			if _, ok := groupBySubj[t.Sub]; !ok {
				subjOrder = append(subjOrder, t.Sub)
			}
			groupBySubj[t.Sub] = append(groupBySubj[t.Sub], t)
		} else {
			anchors = append(anchors, t)
		}
	}

	// 2) Normalize order inside each subject group: -1, 0, +1
	orderFlags := []DiffTripleFlag{TripleRemoved, TripleEqual, TripleAdded}
	for subj, lines := range groupBySubj {
		re := make([]DiffTriple, 0, len(lines))
		for _, f := range orderFlags {
			for _, t := range lines {
				if t.Flag == f {
					re = append(re, t)
				}
			}
		}
		groupBySubj[subj] = re
	}

	// 3) Build result: for each anchor in original order, append its group (by Obj) once
	res := make([]DiffTriple, 0, len(triples))
	consumed := map[string]bool{}

	for _, a := range anchors {
		res = append(res, a)
		if grp, ok := groupBySubj[a.Obj]; ok && !consumed[a.Obj] {
			res = append(res, grp...)
			consumed[a.Obj] = true
		}
	}

	// 4) Append any remaining (un-anchored) groups in subject-first-seen order
	for _, subj := range subjOrder {
		if !consumed[subj] {
			res = append(res, groupBySubj[subj]...)
		}
	}
	return res
}

func diffWriteHeader(from, to *bufio.Reader, dc *diffWriteContext, w io.Writer) error {
	importedGraphs, iRemoved, iAdded, err := diffGraphHeader(from, to)
	if err != nil {
		return err
	}
	GlobalLogger.Debug("diffWriteHeader imported graphs", zap.Int("count", len(importedGraphs)), zap.Uint("removed", iRemoved), zap.Uint("added", iAdded))
	err = diffWriteGraphsHeader(importedGraphs, iRemoved, iAdded, &dc.varintBuf, w)
	if err != nil {
		return err
	}
	dc.sortedImportedGraphs = importedGraphs
	referencedGraphs, rRemoved, rAdded, err := diffGraphHeader(from, to)
	if err != nil {
		return err
	}
	GlobalLogger.Debug("diffWriteHeader referenced graphs", zap.Int("count", len(referencedGraphs)), zap.Uint("removed", rRemoved), zap.Uint("added", rAdded))
	err = diffWriteGraphsHeader(referencedGraphs, rRemoved, rAdded, &dc.varintBuf, w)
	if err != nil {
		return err
	}
	dc.sortedReferencedGraphs = referencedGraphs
	return nil
}

// diffGraphHeader compares the graph imports from two bufio.Reader inputs and identifies the differences.
//
// Parameters:
//   - from: A bufio.Reader containing the "from" graph imports.
//   - to: A bufio.Reader containing the "to" graph imports.
//
// Returns:
//   - combinedImports: A slice of diffWriteCombinedGraph representing the combined results of the comparison.
//   - removed: The count of graph imports that are present in "from" but not in "to".
//   - added: The count of graph imports that are present in "to" but not in "from".
//   - err: An error if any issues occur during the comparison process.
//
// The function reads and compares graph URIs from the two inputs. It categorizes the differences into three cases:
//  1. Removed imports: Graph URIs present in "from" but not in "to".
//  2. Added imports: Graph URIs present in "to" but not in "from".
//  3. Same imports: Graph URIs present in both "from" and "to".
//
// The function processes the inputs sequentially and appends the results to the combinedImports slice.
// It also handles any errors encountered while reading the inputs.
func diffGraphHeader(from, to *bufio.Reader) (graphs []diffWriteGraph, removed, added uint, err error) {
	fromGraphsCount, err := readUint(from)
	if err != nil {
		return
	}
	toGraphsCount, err := readUint(to)
	if err != nil {
		return
	}
	GlobalLogger.Debug("diffGraphHeader", zap.Uint("fromGraphsCount", fromGraphsCount), zap.Uint("toGraphsCount", toGraphsCount))
	i, j := uint(0), uint(0)
	hi, hj := false, false
	var fromGraphURI, toGraphURI string
	for i < fromGraphsCount && j < toGraphsCount {
		if !hi {
			fromGraphURI, err = readNextGraphURI(from)
			if err != nil {
				return
			}
			hi = true
		}
		if !hj {
			toGraphURI, err = readNextGraphURI(to)
			if err != nil {
				return
			}
			hj = true
		}
		switch strings.Compare(fromGraphURI, toGraphURI) {
		case -1:
			graphs = diffAppendRemovedGraph(graphs, fromGraphURI, i)
			removed++
			i++
			hi = false
		case 0:
			graphs = diffAppendSameGraph(graphs, fromGraphURI, i, j)
			i++
			j++
			hi, hj = false, false
		default: // +1
			graphs = diffAppendAddedGraph(graphs, toGraphURI, j)
			added++
			j++
			hj = false
		}
	}
	for ; i < fromGraphsCount; i++ {
		if !hi {
			fromGraphURI, err = readNextGraphURI(from)
			if err != nil {
				return
			}
		}
		graphs = diffAppendRemovedGraph(graphs, fromGraphURI, i)
		removed++
		hi = false
	}
	for ; j < toGraphsCount; j++ {
		if !hj {
			toGraphURI, err = readNextGraphURI(to)
			if err != nil {
				return
			}
		}
		graphs = diffAppendAddedGraph(graphs, toGraphURI, j)
		added++
		hj = false
	}
	return
}

func readNextGraphURI(r *bufio.Reader) (graphURI string, err error) {
	graphURI, err = readString(r)
	return
}

func diffAppendRemovedGraph(graphs []diffWriteGraph, fromGraphURI string, fromIndex uint) []diffWriteGraph {
	graphs = append(graphs, diffWriteGraph{
		graphIRI:  []string{fromGraphURI},
		fromIndex: int(fromIndex),
		toIndex:   -1,
		span:      1,
	})
	return graphs
}

func diffAppendSameGraph(graphs []diffWriteGraph, fromGraphURI string, fromIndex, toIndex uint) []diffWriteGraph {
	if len(graphs) > 0 {
		last := &graphs[len(graphs)-1]
		if last.fromIndex >= 0 && last.toIndex >= 0 {
			last.graphIRI = append(last.graphIRI, fromGraphURI)
			last.span++
			return graphs
		}
	}
	graphs = append(graphs, diffWriteGraph{
		graphIRI:  []string{fromGraphURI},
		fromIndex: int(fromIndex),
		toIndex:   int(toIndex),
		span:      1,
	})
	return graphs
}

func diffAppendAddedGraph(graphs []diffWriteGraph, toGraphURI string, toIndex uint) []diffWriteGraph {
	graphs = append(graphs, diffWriteGraph{
		graphIRI:  []string{toGraphURI},
		fromIndex: -1,
		toIndex:   int(toIndex),
		span:      1,
	})
	return graphs
}

func diffWriteGraphsHeader(diffGraphs []diffWriteGraph, removed, added uint, varintBuf *varintBufT, w io.Writer) error {
	// write the number of removed graphs
	err := writeUint(removed, varintBuf, w)
	if err != nil {
		return err
	}
	// write the number of added graphs
	err = writeUint(added, varintBuf, w)
	if err != nil {
		return err
	}
	for _, gi := range diffGraphs {
		if gi.fromIndex >= 0 {
			if gi.toIndex >= 0 {
				err := writeUint(uint(diffEntrySame), varintBuf, w)
				if err != nil {
					return err
				}
				err = writeUint(gi.span, varintBuf, w)
				if err != nil {
					return err
				}
				continue
			}
			err = writeUint(uint(diffEntryRemoved), varintBuf, w)
			if err != nil {
				return err
			}
			err = writeString(gi.graphIRI[0], varintBuf, w)
			if err != nil {
				return err
			}
			continue
		}
		err = writeUint(uint(diffEntryAdded), varintBuf, w)
		if err != nil {
			return err
		}
		err = writeString(gi.graphIRI[0], varintBuf, w)
		if err != nil {
			return err
		}
	}
	return nil
}

func diffWriteDictionary(from, to *bufio.Reader, d *diffWriteContext, graphIRI string, w io.Writer) error {
	var translation translationOffsets
	// handle current NamedGraph
	err := diffWriteCurrentGraphDictionary(from, to, d, &translation, graphIRI, w)
	if err != nil {
		return err
	}
	// handle Imported NamedGraph
	err = diffWriteGraphsDictionary(from, to, d.sortedImportedGraphs, d, &translation, w)
	if err != nil {
		return err
	}
	// handle Referenced NamedGraph
	err = diffWriteGraphsDictionary(from, to, d.sortedReferencedGraphs, d, &translation, w)
	if err != nil {
		return err
	}
	return nil
}

func diffWriteGraphsDictionary(
	from, to *bufio.Reader, graphs []diffWriteGraph,
	d *diffWriteContext, translation *translationOffsets, w io.Writer,
) error {
	for _, gi := range graphs {
		for i := uint(0); i < gi.span; i++ {
			GlobalLogger.Debug("Diff Dictionary", zap.String("NG IRI", gi.graphIRI[i]))
			var fromTreeCount, toTreeCount uint
			var err error
			if gi.fromIndex >= 0 {
				fromTreeCount, err = readUint(from)
				if err != nil {
					return err
				}
			}
			if gi.toIndex >= 0 {
				toTreeCount, err = readUint(to)
				if err != nil {
					return err
				}
			}
			combinedNodes, deletedCount, addedCount, err := computeDiffNodesDictionary(from, to, fromTreeCount, toTreeCount, d, true, gi.graphIRI[i])
			if err != nil {
				return err
			}
			err = writeUint(deletedCount, &d.varintBuf, w)
			if err != nil {
				return err
			}
			err = writeUint(addedCount, &d.varintBuf, w)
			if err != nil {
				return err
			}
			diffAddNodeTranslations(combinedNodes, d, translation)
			err = writeDiffNodesDictionary(combinedNodes, &d.varintBuf, true, w)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func diffWriteCurrentGraphDictionary(
	from, to *bufio.Reader, d *diffWriteContext,
	translation *translationOffsets, graphIRI string, w io.Writer,
) error {
	GlobalLogger.Debug("Diff Dictionary", zap.String("NG IRI", graphIRI))
	fromIRINodeCount, fromBlankNodeCount, err := readNodeCounts(from)
	if err != nil {
		return err
	}
	toIRINodeCount, toBlankNodeCount, err := readNodeCounts(to)
	if err != nil {
		return err
	}
	GlobalLogger.Debug("diffWriteCurrentGraphDictionary node counts", zap.String("graphIRI", graphIRI), zap.Uint("fromIRI", fromIRINodeCount), zap.Uint("fromBlank", fromBlankNodeCount), zap.Uint("toIRI", toIRINodeCount), zap.Uint("toBlank", toBlankNodeCount))
	combinedIRINodes, IRINodeDeletedCount, IRINodeAddedCount, err := computeDiffNodesDictionary(from, to, fromIRINodeCount, toIRINodeCount, d, true, graphIRI)
	if err != nil {
		return err
	}
	combinedBlankNodes, blankNodeDeletedCount, blankNodeAddedCount, err := computeDiffNodesDictionary(from, to, fromBlankNodeCount, toBlankNodeCount, d, false, graphIRI)
	if err != nil {
		return err
	}
	GlobalLogger.Debug("diffWriteCurrentGraphDictionary diff results", zap.String("graphIRI", graphIRI), zap.Uint("IRIDeleted", IRINodeDeletedCount), zap.Uint("IRIAdded", IRINodeAddedCount), zap.Uint("blankDeleted", blankNodeDeletedCount), zap.Uint("blankAdded", blankNodeAddedCount))
	err = writeUint(IRINodeDeletedCount, &d.varintBuf, w)
	if err != nil {
		return err
	}
	err = writeUint(IRINodeAddedCount, &d.varintBuf, w)
	if err != nil {
		return err
	}
	err = writeUint(blankNodeDeletedCount, &d.varintBuf, w)
	if err != nil {
		return err
	}
	err = writeUint(blankNodeAddedCount, &d.varintBuf, w)
	if err != nil {
		return err
	}
	d.iriNodes, d.blankNodes = combinedIRINodes, combinedBlankNodes
	d.fromTranslations = make([]originToTranslated, 0, len(combinedIRINodes)+len(combinedBlankNodes))
	d.toTranslations = make([]originToTranslated, 0, len(combinedIRINodes)+len(combinedBlankNodes))
	diffAddNodeTranslations(combinedIRINodes, d, translation)
	diffAddNodeTranslations(combinedBlankNodes, d, translation)
	err = writeDiffNodesDictionary(combinedIRINodes, &d.varintBuf, true, w)
	if err != nil {
		return err
	}
	err = writeDiffNodesDictionary(combinedBlankNodes, &d.varintBuf, false, w)
	if err != nil {
		return err
	}
	return nil
}

func readNodeCounts(r *bufio.Reader) (fragmentTreeCount, blankTreeCount uint, err error) {
	fragmentTreeCount, err = readUint(r)
	if err != nil {
		return
	}
	blankTreeCount, err = readUint(r)
	return
}

func computeDiffNodesDictionary(
	from, to *bufio.Reader, fromNodeCount, toNodeCount uint, d *diffWriteContext,
	isIRINode bool, graphIRI string,
) (combinedNodes []diffWriteCombinedNode, deleted, added uint, err error) {
	GlobalLogger.Debug("computeDiffNodesDictionary started", zap.String("graphIRI", graphIRI), zap.Bool("isIRINode", isIRINode), zap.Uint("fromNodeCount", fromNodeCount), zap.Uint("toNodeCount", toNodeCount))
	fromIndex, toIndex := uint(0), uint(0)
	hi, hj := false, false
	var fromFragment, toFragment string
	var fromFragmentForText, toFragmentForText string

	var fromTriplexCnt, toTriplexCnt uint
	blankNodeFromCount := 0
	blankNodeToCount := 0

	for fromIndex < fromNodeCount && toIndex < toNodeCount {
		if !hi {
			fromFragment, fromTriplexCnt, err = readNextFragmentWithTripleCount(from, isIRINode)
			if err != nil {
				GlobalLogger.Panic(err.Error())
			}
			if !isIRINode {
				fromFragmentForText = fmt.Sprintf("_:b%d", blankNodeFromCount)
				blankNodeFromCount++
			} else {
				if prefix, found := NamespaceToPrefix(graphIRI); found {
					fromFragmentForText = prefix + ":" + fromFragment
				} else {
					// NG Node
					if fromFragment == "" {
						fromFragmentForText = ""
					} else {
						fromFragmentForText = ":" + fromFragment
					}
				}
			}
			d.fromNodesDict[d.fromIndex] = fromFragmentForText
			d.fromIndex++
			hi = true
		}
		if !hj {
			toFragment, toTriplexCnt, err = readNextFragmentWithTripleCount(to, isIRINode)
			if err != nil {
				GlobalLogger.Panic(err.Error())
			}
			if !isIRINode {
				toFragmentForText = fmt.Sprintf("_:b%d", blankNodeToCount)
				blankNodeToCount++
			} else {
				if prefix, found := NamespaceToPrefix(graphIRI); found {
					toFragmentForText = prefix + ":" + toFragment
				} else {
					// NG Node
					if toFragment == "" {
						toFragmentForText = ""
					} else {
						toFragmentForText = ":" + toFragment
					}
				}
			}
			d.toNodesDict[d.toIndex] = toFragmentForText
			d.toIndex++
			hj = true
		}
		switch strings.Compare(fromFragment, toFragment) {
		case -1:
			combinedNodes = diffAppendDeletedNode(combinedNodes, fromIndex, fromFragment, fromFragmentForText, fromTriplexCnt)
			deleted++
			fromIndex++
			hi = false
		case 0:
			combinedNodes = diffAppendSameNode(combinedNodes, fromFragment, fromFragmentForText, fromIndex, toIndex, fromTriplexCnt, toTriplexCnt)
			fromIndex++
			toIndex++
			hi, hj = false, false
		default: // +1
			combinedNodes = diffAppendAddedNode(combinedNodes, toIndex, toFragment, toFragmentForText, toTriplexCnt)
			added++
			toIndex++
			hj = false
		}
	}
	for ; fromIndex < fromNodeCount; fromIndex++ {
		if !hi {
			fromFragment, fromTriplexCnt, err = readNextFragmentWithTripleCount(from, isIRINode)
			if err != nil {
				GlobalLogger.Panic(err.Error())
			}
			if fromFragment == "" {
				fromFragmentForText = fmt.Sprintf("_:b%d", blankNodeFromCount)
				blankNodeFromCount++
			} else {
				if prefix, found := NamespaceToPrefix(graphIRI); found {
					fromFragmentForText = prefix + ":" + fromFragment
				} else {
					fromFragmentForText = ":" + fromFragment
				}
			}
			d.fromNodesDict[d.fromIndex] = fromFragmentForText
			d.fromIndex++
		}
		combinedNodes = diffAppendDeletedNode(combinedNodes, fromIndex, fromFragment, fromFragmentForText, fromTriplexCnt)
		deleted++
		hi = false
	}
	for ; toIndex < toNodeCount; toIndex++ {
		if !hj {
			toFragment, toTriplexCnt, err = readNextFragmentWithTripleCount(to, isIRINode)
			if err != nil {
				GlobalLogger.Panic(err.Error())
			}
			if toFragment == "" {
				toFragmentForText = fmt.Sprintf("_:b%d", blankNodeToCount)
				blankNodeToCount++
			} else {
				if prefix, found := NamespaceToPrefix(graphIRI); found {
					toFragmentForText = prefix + ":" + toFragment
				} else {
					toFragmentForText = ":" + toFragment
				}
			}
			d.toNodesDict[d.toIndex] = toFragmentForText
			d.toIndex++
		}
		combinedNodes = diffAppendAddedNode(combinedNodes, toIndex, toFragment, toFragmentForText, toTriplexCnt)
		added++
		hj = false
	}
	GlobalLogger.Debug("computeDiffNodesDictionary finished", zap.String("graphIRI", graphIRI), zap.Bool("isIRINode", isIRINode), zap.Int("combinedNodes", len(combinedNodes)), zap.Uint("deleted", deleted), zap.Uint("added", added))
	return
}

func diffAppendDeletedNode(
	combinedNodes []diffWriteCombinedNode, fromIndex uint,
	fromFragment string, fragmentForText string, fromTriplexCnt uint,
) []diffWriteCombinedNode {
	var l *diffWriteCombinedNode
	if len(combinedNodes) > 0 {
		l = &combinedNodes[len(combinedNodes)-1]
		if l.fromIndexStart < 0 || l.toIndexStart >= 0 || l.fromIndexEnd != int(fromIndex) {
			l = nil
		}
	}
	if l != nil {
		l.details = append(l.details, diffWriteNodeDetails{
			fragment:        []string{fromFragment},
			fragmentForText: []string{fragmentForText},
			triplexCntDelta: -int(fromTriplexCnt),
			span:            1,
		})
		l.fromIndexEnd = int(fromIndex + 1)
	} else {
		combinedNodes = append(combinedNodes, diffWriteCombinedNode{
			details: []diffWriteNodeDetails{{
				fragment:        []string{fromFragment},
				fragmentForText: []string{fragmentForText},
				triplexCntDelta: -int(fromTriplexCnt),
				span:            1,
			}},
			fromIndexStart: int(fromIndex),
			fromIndexEnd:   int(fromIndex + 1),
			// not exited in "to" SST file
			toIndexStart: -1,
			toIndexEnd:   -1,
		})
	}
	return combinedNodes
}

func diffAppendSameNode(
	combinedNodes []diffWriteCombinedNode, sameFragment string, fragmentForText string, fromIndex, toIndex uint,
	fromTriplexCnt, toTriplexCnt uint,
) []diffWriteCombinedNode {
	triplexCntDelta := int(toTriplexCnt) - int(fromTriplexCnt)
	var l *diffWriteCombinedNode
	if len(combinedNodes) > 0 {
		l = &combinedNodes[len(combinedNodes)-1]
		if l.fromIndexStart < 0 || l.toIndexStart < 0 || l.fromIndexEnd != int(fromIndex) || l.toIndexEnd != int(toIndex) {
			l = nil
		}
	}
	if l != nil {
		ld := &l.details[len(l.details)-1]
		if ld.triplexCntDelta == triplexCntDelta {
			ld.fragment = append(ld.fragment, sameFragment)
			ld.fragmentForText = append(ld.fragmentForText, fragmentForText)
			ld.span++
		} else {
			l.details = append(l.details, diffWriteNodeDetails{
				fragment:        []string{sameFragment},
				fragmentForText: []string{fragmentForText},
				triplexCntDelta: triplexCntDelta,
				span:            1,
			})
		}
		l.fromIndexEnd, l.toIndexEnd = int(fromIndex+1), int(toIndex+1)
	} else {
		combinedNodes = append(combinedNodes, diffWriteCombinedNode{
			details: []diffWriteNodeDetails{{
				fragment:        []string{sameFragment},
				fragmentForText: []string{fragmentForText},
				triplexCntDelta: int(toTriplexCnt) - int(fromTriplexCnt),
				span:            1,
			}},
			fromIndexStart: int(fromIndex),
			fromIndexEnd:   int(fromIndex + 1),
			toIndexStart:   int(toIndex),
			toIndexEnd:     int(toIndex + 1),
		})
	}
	return combinedNodes
}

func diffAppendAddedNode(
	combinedNodes []diffWriteCombinedNode, toIndex uint,
	toFragment string, fragmentForText string, toTriplexCnt uint,
) []diffWriteCombinedNode {
	var l *diffWriteCombinedNode
	if len(combinedNodes) > 0 {
		l = &combinedNodes[len(combinedNodes)-1]
		if l.fromIndexStart >= 0 || l.toIndexStart < 0 || l.toIndexEnd != int(toIndex) {
			l = nil
		}
	}
	if l != nil {
		l.details = append(l.details, diffWriteNodeDetails{
			fragment:        []string{toFragment},
			fragmentForText: []string{fragmentForText},
			triplexCntDelta: int(toTriplexCnt),
			span:            1,
		})
		l.toIndexEnd = int(toIndex + 1)
	} else {
		combinedNodes = append(combinedNodes, diffWriteCombinedNode{
			details: []diffWriteNodeDetails{{
				fragment:        []string{toFragment},
				fragmentForText: []string{fragmentForText},
				triplexCntDelta: int(toTriplexCnt),
				span:            1,
			}},
			fromIndexStart: -1,
			fromIndexEnd:   -1,
			toIndexStart:   int(toIndex),
			toIndexEnd:     int(toIndex + 1),
		})
	}
	return combinedNodes
}

func readNextFragmentWithTripleCount(
	r *bufio.Reader,
	isIRINode bool,
) (fragment string, triplexCnt uint, err error) {
	if isIRINode {
		fragment, err = readString(r)
		if err != nil {
			return
		}
	}
	triplexCnt, err = readUint(r)
	return
}

// analyze the combinedNodes and update d and translation
func diffAddNodeTranslations(combinedNodes []diffWriteCombinedNode, d *diffWriteContext, translation *translationOffsets) {
	for _, cs := range combinedNodes {
		fromInc, toInc, combinedInc := uint(0), uint(0), uint(0)
		if cs.fromIndexStart >= 0 { // existed in from
			appendTranslation := false
			if len(d.fromTranslations) == 0 {
				appendTranslation = true
			} else {
				l := d.fromTranslations[len(d.fromTranslations)-1]
				if translation.from-l.original+l.translated != translation.combined {
					appendTranslation = true
				}
			}
			if appendTranslation {
				d.fromTranslations = append(d.fromTranslations, originToTranslated{translation.from, translation.combined})
			}
			d.combinedToFrom[int(translation.combined)] = int(translation.from)
			fromInc = uint(cs.fromIndexEnd - cs.fromIndexStart)
			for inc := uint(1); inc < uint(fromInc); inc++ {
				d.combinedToFrom[int(translation.combined)+int(inc)] = int(translation.from) + int(inc)
			}
			combinedInc = uint(cs.fromIndexEnd - cs.fromIndexStart)
		}
		if cs.toIndexStart >= 0 {
			appendTranslation := false
			if len(d.toTranslations) == 0 {
				appendTranslation = true
			} else {
				l := d.toTranslations[len(d.toTranslations)-1]
				if translation.to-l.original+l.translated != translation.combined {
					appendTranslation = true
				}
			}
			if appendTranslation {
				d.toTranslations = append(d.toTranslations, originToTranslated{translation.to, translation.combined})
			}
			d.combinedToto[int(translation.combined)] = int(translation.to)
			toInc = uint(cs.toIndexEnd - cs.toIndexStart)
			for inc := uint(1); inc < uint(toInc); inc++ {
				d.combinedToto[int(translation.combined)+int(inc)] = int(translation.to) + int(inc)
			}
			combinedInc = uint(cs.toIndexEnd - cs.toIndexStart)
		}
		translation.from += fromInc
		translation.to += toInc
		translation.combined += combinedInc

		// if cs.fromIndexStart >= 0 {
		// 	d.combinedToFrom[int(translation.combined)] = int(translation.from)
		// 	for _, detail := range cs.details {
		// 		if detail.span > 1 {
		// 			for inc := uint(1); inc < uint(detail.span); inc++ {
		// 				d.combinedToFrom[int(translation.combined)+int(inc)] = int(translation.from) + int(inc)
		// 			}
		// 		}
		// 	}
		// }

		// if cs.toIndexStart >= 0 {
		// 	d.combinedToto[int(translation.combined)] = int(translation.to)
		// 	for _, detail := range cs.details {
		// 		if detail.span > 1 {
		// 			for inc := uint(1); inc < uint(detail.span); inc++ {
		// 				d.combinedToto[int(translation.combined)+int(inc)] = int(translation.to) + int(inc)
		// 			}
		// 		}
		// 	}
		// }
	}
}

func writeDiffNodesDictionary(
	nodes []diffWriteCombinedNode, varintBuf *varintBufT,
	isIRINode bool, w io.Writer,
) error {
	if isIRINode {
		GlobalLogger.Debug("writeDiffNodesDictionary - IRI Nodes ")
	} else {
		GlobalLogger.Debug("writeDiffNodesDictionary - Blank Nodes ")
	}
	for _, cs := range nodes {
		// fromIndexStart >= 0 means the node is in the from graph
		if cs.fromIndexStart >= 0 {
			// toIndexStart >= 0 means the node is in the to graph
			if cs.toIndexStart >= 0 {
				err := diffWriteSameCombinedNodes(cs, varintBuf, w)
				if err != nil {
					return err
				}
				continue
			}
			err := diffWriteDeletedCombinedNode(cs, varintBuf, w, isIRINode)
			if err != nil {
				return err
			}
			continue
		}
		err := diffWriteAddedCombinedNode(cs, varintBuf, w, isIRINode)
		if err != nil {
			return err
		}
	}
	return nil
}

func diffWriteSameCombinedNodes(cs diffWriteCombinedNode, varintBuf *varintBufT, w io.Writer) error {
	for _, csDetail := range cs.details {
		if csDetail.triplexCntDelta != 0 {
			for i := uint(0); i < csDetail.span; i++ {
				err := writeUint(uint(diffEntryTripleModified), varintBuf, w)
				if err != nil {
					return err
				}
				GlobalLogger.Debug("diffWriteSameCombinedNodes", zap.Strings("IBNode", csDetail.fragment), zap.Uint("flag", uint(diffEntryTripleModified)))
				err = writeInt64(int64(csDetail.triplexCntDelta), varintBuf, w)
				if err != nil {
					return err
				}
				GlobalLogger.Debug("diffWriteSameCombinedNodes", zap.Strings("IBNode", csDetail.fragment), zap.Int("triplexCntDelta", csDetail.triplexCntDelta))
			}
			continue
		}
		err := writeUint(uint(diffEntrySame), varintBuf, w)
		if err != nil {
			return err
		}
		GlobalLogger.Debug("diffWriteSameCombinedNodes", zap.Strings("IBNode", csDetail.fragment), zap.Uint("flag", uint(diffEntrySame)))
		err = writeUint(csDetail.span, varintBuf, w)
		if err != nil {
			return err
		}
		GlobalLogger.Debug("diffWriteSameCombinedNodes", zap.Strings("IBNode", csDetail.fragment), zap.Uint("same count", csDetail.span))
	}
	return nil
}

func diffWriteDeletedCombinedNode(cs diffWriteCombinedNode, varintBuf *varintBufT, w io.Writer, includeFragment bool) error {
	for _, csDetail := range cs.details {
		err := writeUint(uint(diffEntryRemoved), varintBuf, w)
		if err != nil {
			return err
		}
		GlobalLogger.Debug("diffWriteDeletedCombinedNode", zap.Strings("IBNode", csDetail.fragment), zap.Uint("flag", uint(diffEntryRemoved)))
		if includeFragment {
			err = writeString(csDetail.fragment[0], varintBuf, w)
			if err != nil {
				return err
			}
			GlobalLogger.Debug("diffWriteDeletedCombinedNode", zap.Strings("IBNode", csDetail.fragment))
		}
		err = writeInt64(int64(csDetail.triplexCntDelta), varintBuf, w)
		if err != nil {
			return err
		}
		GlobalLogger.Debug("diffWriteDeletedCombinedNode", zap.Int("triplexCntDelta", csDetail.triplexCntDelta))
	}
	return nil
}

func diffWriteAddedCombinedNode(cs diffWriteCombinedNode, varintBuf *varintBufT, w io.Writer, includeFragment bool) error {
	for _, csDetail := range cs.details {
		err := writeUint(uint(diffEntryAdded), varintBuf, w)
		if err != nil {
			return err
		}
		GlobalLogger.Debug("diffWriteAddedCombinedNode", zap.Strings("IBNode", csDetail.fragment), zap.Uint("flag", uint(diffEntryAdded)))

		if includeFragment {
			err = writeString(csDetail.fragment[0], varintBuf, w)
			if err != nil {
				return err
			}
			GlobalLogger.Debug("diffWriteAddedCombinedNode", zap.Strings("IBNode", csDetail.fragment))
		}
		err = writeInt64(int64(csDetail.triplexCntDelta), varintBuf, w)
		if err != nil {
			return err
		}
		GlobalLogger.Debug("diffWriteAddedCombinedNode", zap.Int("triplexCntDelta", csDetail.triplexCntDelta))
	}
	return nil
}

func diffWriteContent(from, to *bufio.Reader, d *diffWriteContext, w io.Writer) error {
	GlobalLogger.Debug("diffWriteContent started", zap.Int("iriNodes", len(d.iriNodes)), zap.Int("blankNodes", len(d.blankNodes)))
	GlobalLogger.Debug("diffWriteCombinedNodeRange for IRI Nodes")
	err := diffWriteNodesContent(from, to, d, w, d.iriNodes, false)
	if err != nil {
		return err
	}
	GlobalLogger.Debug("diffWriteCombinedNodeRange for Blank Nodes")
	return diffWriteNodesContent(from, to, d, w, d.blankNodes, true)
}

func diffWriteNodesContent(from *bufio.Reader, to *bufio.Reader, d *diffWriteContext, w io.Writer, combinedNodes []diffWriteCombinedNode, bBlankNode bool) error {
	d.identicalContentSpan = 0
	for _, cs := range combinedNodes {
		// if exist in from
		if cs.fromIndexStart >= 0 {
			// if exist in to
			if cs.toIndexStart >= 0 {
				for _, cd := range cs.details {
					for i := uint(0); i < cd.span; i++ {
						err := diffComputeAndWriteNodeContent(from, to, cd.fragmentForText[i], d, w, bBlankNode)
						if err != nil {
							return err
						}
					}
				}
			} else {
				// if not exist in to, means this is deleted
				for _, cd := range cs.details {
					for i := uint(0); i < cd.span; i++ {
						err := diffComputeAndWriteNodeContent(from, nil, cd.fragmentForText[i], d, w, bBlankNode)
						if err != nil {
							return err
						}
					}
				}
			}
		} else {
			// if not exist in from, means this is added
			for _, cd := range cs.details {
				for i := uint(0); i < cd.span; i++ {
					err := diffComputeAndWriteNodeContent(nil, to, cd.fragmentForText[i], d, w, bBlankNode)
					if err != nil {
						return err
					}
				}
			}
		}
	}
	var err error
	if d.identicalContentSpan != 0 {
		err = diffWriteAndResetIdenticalContent(d, w)
	}
	return err
}

func diffComputeAndWriteNodeContent(from *bufio.Reader, to *bufio.Reader, fragment string, d *diffWriteContext, w io.Writer, bBlankNode bool) error {
	GlobalLogger.Debug("diffComputeAndWriteNodeContent", zap.String("fragment", fragment), zap.Bool("bBlankNode", bBlankNode))
	removedTripleCount, addedTripleCount, err := diffComputeNodeTriples(from, to, fragment, d, bBlankNode) // non-literal triples
	if err != nil {
		return err
	}
	if removedTripleCount+addedTripleCount != 0 {
		if d.identicalContentSpan != 0 {
			err := diffWriteAndResetIdenticalContent(d, w)
			if err != nil {
				return err
			}
		}
		err = diffWriteNodeTriples(fragment, removedTripleCount, addedTripleCount, d, w)
		if err != nil {
			return err
		}
	}
	literalTripleRemoved, literalTripleAdded, err := diffComputeNodeLiteralTriples(from, to, d) // literal triples
	if err != nil {
		return err
	}
	if literalTripleRemoved+literalTripleAdded != 0 {
		if d.identicalContentSpan != 0 {
			err := diffWriteAndResetIdenticalContent(d, w)
			if err != nil {
				return err
			}
		}
		if removedTripleCount+addedTripleCount == 0 {
			err = diffWriteNodeTriples(fragment, removedTripleCount, addedTripleCount, d, w)
			if err != nil {
				return err
			}
		}
		err = diffWriteLiteralTriples(fragment, literalTripleRemoved, literalTripleAdded, d, w)
		if err != nil {
			return err
		}
	} else {
		if removedTripleCount+addedTripleCount != 0 {
			err = diffWriteLiteralTriples(fragment, literalTripleRemoved, literalTripleAdded, d, w)
			if err != nil {
				return err
			}
		} else {
			d.identicalContentSpan++
			GlobalLogger.Debug("identicalContentSpan++", zap.Uint("identicalNodeCount", d.identicalContentSpan))
		}
	}
	return nil
}

func diffComputeNodeTriples(from, to *bufio.Reader, fragment string, d *diffWriteContext, bBlankNode bool) (removed, added uint, err error) {
	GlobalLogger.Debug("diffComputeNodeTriples started", zap.String("fragment", fragment), zap.Bool("bBlankNode", bBlankNode))
	fromTripleCnt, toTripleCnt := uint(0), uint(0)
	if from != nil {
		fromTripleCnt, err = readUint(from)
		GlobalLogger.Debug("read from from", zap.Uint("fromTripleCnt", fromTripleCnt))
		if err != nil {
			return
		}
	}
	if to != nil {
		toTripleCnt, err = readUint(to)
		GlobalLogger.Debug("read from to", zap.Uint("toTripleCnt", toTripleCnt))
		if err != nil {
			return
		}
	}
	GlobalLogger.Debug("diffComputeNodeTriples counts", zap.String("fragment", fragment), zap.Uint("fromTripleCnt", fromTripleCnt), zap.Uint("toTripleCnt", toTripleCnt))
	d.nodeTripleBuf = d.nodeTripleBuf[:0]
	sameCnt := uint(0)
	i, j := uint(0), uint(0)
	hi, hj := false, false
	var fromTriple, toTriple nodeTripleT
	var fromTermCollectionTriple, toTermCollectionTriple bool
	memberCount := -1
	for i < fromTripleCnt && j < toTripleCnt {
		if !hi {
			fromTriple, err = diffNextNodeTripleRead(from, 0, d.fromTranslations)
			if err != nil {
				return
			}
			// check if the fromTriple has a rdf:first predicate and bBlankNode parameter -> this triple is for TermCollection
			hi = true
			fromTermCollectionTriple = false
			if bBlankNode && d.combinedToFrom[int(fromTriple.predIndex)] == d.fromRdfFirstIndex {
				fromTermCollectionTriple = true
			}

		}
		if !hj {
			toTriple, err = diffNextNodeTripleRead(to, 1, d.toTranslations)
			if err != nil {
				return
			}
			toTermCollectionTriple = false
			if bBlankNode && d.combinedToto[int(toTriple.predIndex)] == d.toRdfFirstIndex {
				toTermCollectionTriple = true
			}
			hj = true
		}

		if fromTermCollectionTriple && toTermCollectionTriple {
			memberCount++
			if fromTriple.objIndex == toTriple.objIndex {
				diffSameNodeTriple(fromTriple, fragment, memberCount, d)
				sameCnt++
				i++
				j++
				hi, hj = false, false
			} else {
				maybeDiffSameNodeTriple(&sameCnt, d)
				diffRemovedNodeTriple(fromTriple, fragment, memberCount, d)
				removed++
				i++
				hi = false

				// maybeDiffSameNodeTriple(&sameCnt, d)
				diffAddedNodeTriple(toTriple, fragment, memberCount, d)
				added++
				j++
				hj = false
			}
		} else {
			// compare fromTriple and toTriple
			// fromTriple.predIndex < toTriple.predIndex means fromTriple is removed
			if fromTriple.predIndex < toTriple.predIndex {
				maybeDiffSameNodeTriple(&sameCnt, d)
				diffRemovedNodeTriple(fromTriple, fragment, memberCount, d)
				removed++
				i++
				hi = false
			} else if fromTriple.predIndex > toTriple.predIndex {
				maybeDiffSameNodeTriple(&sameCnt, d)
				diffAddedNodeTriple(toTriple, fragment, memberCount, d)
				added++
				j++
				hj = false
			} else { // fromTriple.predIndex == toTriple.predIndex
				if fromTriple.objIndex < toTriple.objIndex {
					maybeDiffSameNodeTriple(&sameCnt, d)
					diffRemovedNodeTriple(fromTriple, fragment, memberCount, d)
					removed++
					i++
					hi = false
				} else if fromTriple.objIndex > toTriple.objIndex {
					maybeDiffSameNodeTriple(&sameCnt, d)
					diffAddedNodeTriple(toTriple, fragment, memberCount, d)
					added++
					j++
					hj = false
				} else { // fromTriple.objIndex == toTriple.objIndex
					diffSameNodeTriple(fromTriple, fragment, memberCount, d)
					sameCnt++
					i++
					j++
					hi, hj = false, false
				}
			}
		}
	}
	for ; i < fromTripleCnt; i++ {
		if !hi {
			fromTriple, err = diffNextNodeTripleRead(from, 0, d.fromTranslations)
			if err != nil {
				return
			}
		}
		if fromTermCollectionTriple && toTermCollectionTriple {
			memberCount++
		}
		maybeDiffSameNodeTriple(&sameCnt, d)
		diffRemovedNodeTriple(fromTriple, fragment, memberCount, d)
		removed++
		hi = false
	}
	for ; j < toTripleCnt; j++ {
		if !hj {
			toTriple, err = diffNextNodeTripleRead(to, 1, d.toTranslations)
			if err != nil {
				return
			}
		}
		if fromTermCollectionTriple && toTermCollectionTriple {
			memberCount++
		}
		maybeDiffSameNodeTriple(&sameCnt, d)
		diffAddedNodeTriple(toTriple, fragment, memberCount, d)
		added++
		hj = false
	}
	maybeDiffSameNodeTriple(&sameCnt, d)
	GlobalLogger.Debug("diffComputeNodeTriples finished", zap.String("fragment", fragment), zap.Uint("removed", removed), zap.Uint("added", added))
	return
}

// readerSource == 0 means from, 1 means to
func diffNextNodeTripleRead(r *bufio.Reader, readerSource int, t []originToTranslated) (n nodeTripleT, err error) {
	n.predIndex, err = readUint(r)
	if readerSource == 0 {
		GlobalLogger.Debug("read from from", zap.Uint("predIndex", n.predIndex))
	} else {
		GlobalLogger.Debug("read from to", zap.Uint("predIndex", n.predIndex))
	}
	if err != nil {
		return
	}
	n.objIndex, err = readUint(r)
	if readerSource == 0 {
		GlobalLogger.Debug("read from from", zap.Uint("objIndex", n.objIndex))
	} else {
		GlobalLogger.Debug("read from to", zap.Uint("objIndex", n.objIndex))
	}
	if err != nil {
		return
	}
	n.predIndex = diffTranslateNode(n.predIndex, t)
	n.objIndex = diffTranslateNode(n.objIndex, t)
	n.tripleSource = readerSource
	return
}

func maybeDiffSameNodeTriple(sameCnt *uint, d *diffWriteContext) {
	if *sameCnt == 0 {
		return
	}
	d.nodeTripleBuf = append(d.nodeTripleBuf, nodeTripleT{predIndex: *sameCnt << diffSameRemovedOrAddedOffset})
	*sameCnt = 0
}

func diffSameNodeTriple(sTriple nodeTripleT, fragment string, memberCnt int, d *diffWriteContext) {
	d.diffTripleIndex = append(d.diffTripleIndex, diffTripleIndex{
		flag:   TripleEqual,
		subj:   fragment,
		pred:   sTriple.predIndex,
		obj:    sTriple.objIndex,
		member: memberCnt,
	})
}

func diffRemovedNodeTriple(sTriple nodeTripleT, fragment string, memberCnt int, d *diffWriteContext) {
	d.nodeTripleBuf = append(d.nodeTripleBuf, nodeTripleT{
		predIndex: (sTriple.predIndex << diffSameRemovedOrAddedOffset) | diffRemovedFlag,
		objIndex:  sTriple.objIndex,
	})

	d.diffTripleIndex = append(d.diffTripleIndex, diffTripleIndex{
		flag:   TripleRemoved,
		subj:   fragment,
		pred:   sTriple.predIndex,
		obj:    sTriple.objIndex,
		member: memberCnt,
	})
}

func diffAddedNodeTriple(sTriple nodeTripleT, fragment string, memberCnt int, d *diffWriteContext) {
	d.nodeTripleBuf = append(d.nodeTripleBuf, nodeTripleT{
		predIndex: (sTriple.predIndex << diffSameRemovedOrAddedOffset) | diffAddedFlag,
		objIndex:  sTriple.objIndex,
	})

	d.diffTripleIndex = append(d.diffTripleIndex, diffTripleIndex{
		flag:   TripleAdded,
		subj:   fragment,
		pred:   sTriple.predIndex,
		obj:    sTriple.objIndex,
		member: memberCnt,
	})
}

func diffWriteAndResetIdenticalContent(d *diffWriteContext, w io.Writer) error {
	span := d.identicalContentSpan
	d.identicalContentSpan = 0
	err := writeUint(span<<diffIdenticalOrModifiedOffset, &d.varintBuf, w)
	GlobalLogger.Debug("writeDiff identical content", zap.Uint("spanValue", span), zap.Uint("spanShifted", span<<diffIdenticalOrModifiedOffset))
	return err
}

func diffWriteNodeTriples(fragment string, removed, added uint, d *diffWriteContext, w io.Writer) error {
	err := writeUint((removed<<diffIdenticalOrModifiedOffset)|diffModifiedFlag, &d.varintBuf, w)
	if err != nil {
		return err
	}
	err = writeUint(added, &d.varintBuf, w)
	if err != nil {
		return err
	}
	GlobalLogger.Debug("writeDiff node triples", zap.Uint("removed", removed), zap.Uint("removedShifted", (removed<<diffIdenticalOrModifiedOffset|diffModifiedFlag)), zap.Uint("added", added))
	// When there are no removed/added triples, the reader reads directly from base.
	// Do not write same triple counts to avoid diff stream misalignment.
	if removed+added == 0 {
		d.nodeTripleBuf = d.nodeTripleBuf[:0]
		return nil
	}
	for index, st := range d.nodeTripleBuf {
		switch st.predIndex & diffSameRemovedOrAddedMask {
		case 0:
			err := writeUint(st.predIndex, &d.varintBuf, w)
			GlobalLogger.Debug("writeDiff node triple", zap.Int("index", index), zap.Uint("same triple count", st.predIndex>>diffSameRemovedOrAddedOffset), zap.Uint("shifted", st.predIndex))
			if err != nil {
				return err
			}
		case diffRemovedFlag:
			err := writeUint(st.predIndex, &d.varintBuf, w)
			GlobalLogger.Debug("writeDiff removed node triple", zap.Int("index", index), zap.Uint("pred Index", st.predIndex>>diffSameRemovedOrAddedOffset), zap.Uint("shifted", st.predIndex))
			if err != nil {
				return err
			}
			err = writeUint(st.objIndex, &d.varintBuf, w)
			GlobalLogger.Debug("writeDiff removed node triple", zap.Int("index", index), zap.Uint("ObjIndex", st.objIndex))
			if err != nil {
				return err
			}
		case diffAddedFlag:
			err := writeUint(st.predIndex, &d.varintBuf, w)
			GlobalLogger.Debug("writeDiff added node triple", zap.Int("index", index), zap.Uint("pred Index", st.predIndex>>diffSameRemovedOrAddedOffset), zap.Uint("shifted", st.predIndex))
			if err != nil {
				return err
			}
			err = writeUint(st.objIndex, &d.varintBuf, w)
			GlobalLogger.Debug("writeDiff added node triple", zap.Int("index", index), zap.Uint("ObjIndex", st.objIndex))
			if err != nil {
				return err
			}
		}
	}
	d.nodeTripleBuf = d.nodeTripleBuf[:0]
	return nil
}

func diffComputeNodeLiteralTriples(from, to *bufio.Reader, d *diffWriteContext) (removed, added uint, err error) {
	GlobalLogger.Debug("diffComputeNodeLiteralTriples started")
	fromTripleCnt, toTripleCnt := uint(0), uint(0)
	if from != nil {
		fromTripleCnt, err = readUint(from)
		GlobalLogger.Debug("read from from", zap.Uint("Literal fromTripleCnt", fromTripleCnt))
		if err != nil {
			return
		}
	}
	if to != nil {
		toTripleCnt, err = readUint(to)
		GlobalLogger.Debug("read from to", zap.Uint("Literal toTripleCnt", toTripleCnt))
		if err != nil {
			return
		}
	}
	GlobalLogger.Debug("diffComputeNodeLiteralTriples counts", zap.Uint("fromTripleCnt", fromTripleCnt), zap.Uint("toTripleCnt", toTripleCnt))
	fromTriples := make([]predicateIndexWithLiteralT, 0, fromTripleCnt)
	toTriples := make([]predicateIndexWithLiteralT, 0, toTripleCnt)
	for i := uint(0); i < fromTripleCnt; i++ {
		var t predicateIndexWithLiteralT
		t, err = diffNextLiteralTriple(from, 0, d.fromTranslations)
		if err != nil {
			return
		}
		fromTriples = append(fromTriples, t)
	}
	for i := uint(0); i < toTripleCnt; i++ {
		var t predicateIndexWithLiteralT
		t, err = diffNextLiteralTriple(to, 1, d.toTranslations)
		if err != nil {
			return
		}
		toTriples = append(toTriples, t)
	}
	sort.Slice(fromTriples, func(i, j int) bool {
		if fromTriples[i].pred != fromTriples[j].pred {
			return fromTriples[i].pred < fromTriples[j].pred
		}
		if fromTriples[i].kind != fromTriples[j].kind {
			return fromTriples[i].kind < fromTriples[j].kind
		}
		cmp, _ := diffCompareLiteralValue(fromTriples[i].value, fromTriples[j].value)
		return cmp < 0
	})
	sort.Slice(toTriples, func(i, j int) bool {
		if toTriples[i].pred != toTriples[j].pred {
			return toTriples[i].pred < toTriples[j].pred
		}
		if toTriples[i].kind != toTriples[j].kind {
			return toTriples[i].kind < toTriples[j].kind
		}
		cmp, _ := diffCompareLiteralValue(toTriples[i].value, toTriples[j].value)
		return cmp < 0
	})

	d.literalTripleBuf = d.literalTripleBuf[:0]
	sameCnt := uint(0)
	i, j := 0, 0
	for i < len(fromTriples) && j < len(toTriples) {
		ft := fromTriples[i]
		tt := toTriples[j]
		GlobalLogger.Debug("diffComputeNodeLiteralTriples compare", zap.Uint("fromPred", ft.pred), zap.Uint("toPred", tt.pred), zap.Uint("fromKind", uint(ft.kind)), zap.Uint("toKind", uint(tt.kind)), zap.String("fromValue", fmt.Sprintf("%v", ft.value)), zap.String("toValue", fmt.Sprintf("%v", tt.value)))
		if ft.pred < tt.pred {
			GlobalLogger.Debug("diffComputeNodeLiteralTriples decision: from removed (pred <)")
			maybeDiffSameLiteralTriple(&sameCnt, d)
			diffRemovedLiteralTriple(ft, d)
			removed++
			i++
		} else if ft.pred > tt.pred {
			GlobalLogger.Debug("diffComputeNodeLiteralTriples decision: to added (pred >)")
			maybeDiffSameLiteralTriple(&sameCnt, d)
			diffAddedLiteralTriple(tt, d)
			added++
			j++
		} else { // pred equal
			if ft.kind < tt.kind {
				GlobalLogger.Debug("diffComputeNodeLiteralTriples decision: from removed (kind <)")
				maybeDiffSameLiteralTriple(&sameCnt, d)
				diffRemovedLiteralTriple(ft, d)
				removed++
				i++
			} else if ft.kind > tt.kind {
				GlobalLogger.Debug("diffComputeNodeLiteralTriples decision: to added (kind >)")
				maybeDiffSameLiteralTriple(&sameCnt, d)
				diffAddedLiteralTriple(tt, d)
				added++
				j++
			} else { // kind equal
				cmp, cmpErr := diffCompareLiteralValue(ft.value, tt.value)
				if cmpErr != nil {
					err = cmpErr
					return
				}
				if cmp < 0 {
					GlobalLogger.Debug("diffComputeNodeLiteralTriples decision: from removed (value <)")
					maybeDiffSameLiteralTriple(&sameCnt, d)
					diffRemovedLiteralTriple(ft, d)
					removed++
					i++
				} else if cmp > 0 {
					GlobalLogger.Debug("diffComputeNodeLiteralTriples decision: to added (value >)")
					maybeDiffSameLiteralTriple(&sameCnt, d)
					diffAddedLiteralTriple(tt, d)
					added++
					j++
				} else { // value equal
					GlobalLogger.Debug("diffComputeNodeLiteralTriples decision: same")
					sameCnt++
					i++
					j++
				}
			}
		}
	}
	for ; i < len(fromTriples); i++ {
		maybeDiffSameLiteralTriple(&sameCnt, d)
		diffRemovedLiteralTriple(fromTriples[i], d)
		removed++
	}
	for ; j < len(toTriples); j++ {
		maybeDiffSameLiteralTriple(&sameCnt, d)
		diffAddedLiteralTriple(toTriples[j], d)
		added++
	}
	maybeDiffSameLiteralTriple(&sameCnt, d)
	GlobalLogger.Debug("diffComputeNodeLiteralTriples finished", zap.Uint("removed", removed), zap.Uint("added", added))
	return
}

func diffNextLiteralTriple(r *bufio.Reader, readerSource int, t []originToTranslated) (n predicateIndexWithLiteralT, err error) {
	n.pred, err = readUint(r)
	if err != nil {
		return
	}
	origPred := n.pred
	n.pred = diffTranslateNode(n.pred, t)
	if readerSource == 0 {
		GlobalLogger.Debug("read from from", zap.Uint("origPred", origPred), zap.Uint("translatedPred", n.pred))
	} else {
		GlobalLogger.Debug("read from to", zap.Uint("origPred", origPred), zap.Uint("translatedPred", n.pred))
	}

	n.diffLiteralT, err = diffLiteral(r, readerSource)
	return
}

func diffLiteral(r *bufio.Reader, readerSource int) (n diffLiteralT, err error) {
	k, err := readUint(r)
	if err != nil {
		return
	}
	if readerSource == 0 {
		GlobalLogger.Debug("read from from", zap.Uint("literal kind", k))
	} else {
		GlobalLogger.Debug("read from to", zap.Uint("literal kind", k))
	}

	n.kind = writtenLiteralKind(k)
	switch n.kind {
	case writtenLiteralString:
		n.value, err = readString(r)
		if err != nil {
			return
		}
		if readerSource == 0 {
			GlobalLogger.Debug("read from from", zap.String("string", n.value.(string)))
		} else {
			GlobalLogger.Debug("read from to", zap.String("string", n.value.(string)))
		}
	case writtenLiteralLangString:
		var value, lang string
		value, err = readString(r)
		if err != nil {
			return
		}
		lang, err = readString(r)
		if err != nil {
			return
		}
		n.value = LangString{Val: value, LangTag: lang}
	case writtenLiteralInteger:
		n.value, err = readInt64(r)
		if err != nil {
			return
		}
	case writtenLiteralDouble:
		n.value, err = readFloat64(r)
		if err != nil {
			return
		}

	case writtenLiteralByte:
		n.value, err = readByte(r)
		if err != nil {
			return
		}
	case writtenLiteralShort:
		n.value, err = readShort(r)
		if err != nil {
			return
		}
	case writtenLiteralInt:
		n.value, err = readInt(r)
		if err != nil {
			return
		}
	case writtenLiteralLong:
		n.value, err = readLong(r)
		if err != nil {
			return
		}
	case writtenLiteralUnsignedByte:
		n.value, err = readUnsignedByte(r)
		if err != nil {
			return
		}
	case writtenLiteralUnsignedShort:
		n.value, err = readUnsignedShort(r)
		if err != nil {
			return
		}
	case writtenLiteralUnsignedInt:
		n.value, err = readUnsignedInt(r)
		if err != nil {
			return
		}
	case writtenLiteralUnsignedLong:
		n.value, err = readUnsignedLong(r)
		if err != nil {
			return
		}
	case writtenLiteralBoolean:
		n.value, err = readUint(r)
		if err != nil {
			return
		}
	case writtenLiteralCollection:
		var memberCnt uint
		memberCnt, err = readUint(r)
		if err != nil {
			return
		}
		lc := make([]diffLiteralT, memberCnt)
		for i := uint(0); i < memberCnt; i++ {
			lc[i], err = diffLiteral(r, readerSource)
		}
		n.value = lc
	default:
		err = fmt.Errorf("unsupported literal kind: %d", n.kind)
		return
	}
	return
}

// diffTranslateNode calculates the translated index of a node based on a mapping
// of original to translated indices.
//
// Parameters:
//   - index: The original index of the node to be translated.
//   - t: A slice of originToTranslated structs, where each struct contains
//     an `original` index and its corresponding `translated` index.
//
// The function uses binary search to find the closest mapping in `t` where
// the `original` index is less than or equal to the given `index`. If no such
// mapping exists, the function returns the original `index`. Otherwise, it
// calculates the translated index by adjusting the difference between the
// original and translated indices.
//
// Returns:
//   - The translated index of the node.
//
// originToTranslated struct{ original, translated uint }
func diffTranslateNode(index uint, t []originToTranslated) uint {
	found := sort.Search(len(t), func(i int) bool {
		return t[i].original >= index
	})
	if found == len(t) || t[found].original > index { // not found
		found--
	}
	if found < 0 {
		return index
	}
	return index - t[found].original + t[found].translated
}

func maybeDiffSameLiteralTriple(sameCnt *uint, d *diffWriteContext) {
	if *sameCnt == 0 {
		return
	}
	d.literalTripleBuf = append(d.literalTripleBuf, predicateIndexWithLiteralT{pred: *sameCnt << diffSameRemovedOrAddedOffset})
	*sameCnt = 0
}

func diffRemovedLiteralTriple(lTriple predicateIndexWithLiteralT, d *diffWriteContext) {
	d.literalTripleBuf = append(d.literalTripleBuf, predicateIndexWithLiteralT{
		pred: (lTriple.pred << diffSameRemovedOrAddedOffset) | diffRemovedFlag,
		diffLiteralT: diffLiteralT{
			kind:  lTriple.kind,
			value: lTriple.value,
		},
	})
}

func diffAddedLiteralTriple(lTriple predicateIndexWithLiteralT, d *diffWriteContext) {
	d.literalTripleBuf = append(d.literalTripleBuf, predicateIndexWithLiteralT{
		pred: (lTriple.pred << diffSameRemovedOrAddedOffset) | diffAddedFlag,
		diffLiteralT: diffLiteralT{
			kind:  lTriple.kind,
			value: lTriple.value,
		},
	})
}

func diffWriteLiteralTriples(fragment string, removed, added uint, d *diffWriteContext, w io.Writer) error {
	err := writeUint((removed<<diffIdenticalOrModifiedOffset)|diffModifiedFlag, &d.varintBuf, w)
	if err != nil {
		return err
	}
	err = writeUint(added, &d.varintBuf, w)
	if err != nil {
		return err
	}
	GlobalLogger.Debug("writeDiff literal triples", zap.Uint("removed", removed), zap.Uint("shifted", (removed<<diffIdenticalOrModifiedOffset|diffModifiedFlag)), zap.Uint("added", added))
	for _, lt := range d.literalTripleBuf {
		flag := lt.pred & diffSameRemovedOrAddedMask
		// if (lt.pred & diffSameRemovedOrAddedMask) == 0, it means this value a same triple counts
		if flag == 0 {
			err := writeUint(lt.pred, &d.varintBuf, w)
			GlobalLogger.Debug("writeDiff literal triple", zap.Uint("same triple counts", lt.pred>>diffSameRemovedOrAddedOffset), zap.Uint("shifted", lt.pred))
			if err != nil {
				return err
			}
		} else {
			var li Term
			switch value := lt.value.(type) {
			case string:
				li = String(value)
			case LangString:
				li = LangString{Val: value.Val, LangTag: value.LangTag}
			case int64:
				li = Integer(value)
			case float64:
				li = Double(value)
			case uint:
				if value == 0 {
					li = Boolean(false)
				} else {
					li = Boolean(true)
				}
			case []diffLiteralT:
				var literals []Literal
				for _, differLiteral := range value {
					var memberLiteral Literal
					switch value := differLiteral.value.(type) {
					case string:
						memberLiteral = String(value)
					case LangString:
						memberLiteral = LangString{Val: value.Val, LangTag: value.LangTag}
					case int64:
						memberLiteral = Integer(value)
					case float64:
						memberLiteral = Double(value)
					case uint:
						if value == 0 {
							memberLiteral = Boolean(false)
						} else {
							memberLiteral = Boolean(true)
						}
					}
					literals = append(literals, memberLiteral)
					if len(literals) > 1 {
						li = NewLiteralCollection(literals[0], literals[1:]...)
					} else {
						li = NewLiteralCollection(literals[0])
					}
				}

			default:
				return fmt.Errorf("unsupported literal value in diffWriteLiteralTriples: %T", value)
			}
			err := writeUint(lt.pred, &d.varintBuf, w)
			GlobalLogger.Debug("writeDiff literal triple", zap.Uint("pred index", lt.pred>>diffSameRemovedOrAddedOffset), zap.Uint("shifted", lt.pred))
			if err != nil {
				return err
			}
			err = diffWriteCombinedLiteral(lt.diffLiteralT, &d.varintBuf, w)
			GlobalLogger.Debug("writeDiff literal triple", zap.Uint("objKind", uint(lt.diffLiteralT.kind)), zap.String("ObjKind", literalKindUintToString(lt.diffLiteralT.kind)), zap.String("objValue", fmt.Sprintf("%v", lt.diffLiteralT.value)))
			if err != nil {
				return err
			}
			switch flag {
			case diffRemovedFlag:
				d.diffLiteralTripleIndex = append(d.diffLiteralTripleIndex, diffLiteralTripleIndex{
					flag: TripleRemoved,
					subj: fragment,
					// sub:         st.subIndex,
					pred: lt.pred >> diffSameRemovedOrAddedOffset,
					obj:  li,
				})
			case diffAddedFlag:
				d.diffLiteralTripleIndex = append(d.diffLiteralTripleIndex, diffLiteralTripleIndex{
					flag: TripleAdded,
					subj: fragment,
					// sub:         st.subIndex,
					pred: lt.pred >> diffSameRemovedOrAddedOffset,
					obj:  li,
				})
			}
		}
	}
	d.literalTripleBuf = d.literalTripleBuf[:0]
	return nil
}

func diffWriteCombinedLiteral(lt diffLiteralT, varintBuf *varintBufT, w io.Writer) error {
	err := writeUint(uint(lt.kind), varintBuf, w)
	if err != nil {
		return err
	}
	switch value := lt.value.(type) {
	case string:
		err := writeString(value, varintBuf, w)
		if err != nil {
			return err
		}
	case LangString:
		err := writeString(value.Val, varintBuf, w)
		if err != nil {
			return err
		}
		err = writeString(value.LangTag, varintBuf, w)
		if err != nil {
			return err
		}
	case int64:
		err := writeInt64(value, varintBuf, w)
		if err != nil {
			return err
		}
	case float64:
		err := writeFloat64(value, varintBuf, w)
		if err != nil {
			return err
		}
	case int8:
		err := writeByte(value, varintBuf, w)
		if err != nil {
			return err
		}
	case int16:
		err := writeShort(value, varintBuf, w)
		if err != nil {
			return err
		}
	case int32:
		err := writeInt(value, varintBuf, w)
		if err != nil {
			return err
		}
	case uint:
		err := writeUint(value, varintBuf, w)
		if err != nil {
			return err
		}
	case uint8:
		err := writeUnsignedByte(value, varintBuf, w)
		if err != nil {
			return err
		}
	case uint16:
		err := writeUnsignedShort(value, varintBuf, w)
		if err != nil {
			return err
		}
	case uint32:
		err := writeUnsignedInt(value, varintBuf, w)
		if err != nil {
			return err
		}
	case uint64:
		err := writeUnsignedLong(value, varintBuf, w)
		if err != nil {
			return err
		}
	case float32:
		err := writeFloat32(value, varintBuf, w)
		if err != nil {
			return err
		}
	case []diffLiteralT:
		err := writeUint(uint(len(value)), varintBuf, w)
		if err != nil {
			return err
		}
		for _, lt := range value {
			err := diffWriteCombinedLiteral(lt, varintBuf, w)
			if err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unsupported literal value in diffWriteCombinedLiteral: %T", value)
	}
	return nil
}

func diffCompareLiteralValue(v1, v2 interface{}) (int, error) {
	switch v1 := v1.(type) {
	case string:
		v2 := v2.(string)
		switch {
		case v1 < v2:
			return -1, nil
		case v1 == v2:
			return 0, nil
		default:
			return 1, nil
		}
	case LangString:
		v2 := v2.(LangString)
		switch {
		case v1.Val < v2.Val:
			return -1, nil
		case v1.Val == v2.Val:
			return 0, nil
		default:
			return 1, nil
		}
	case bool:
		v2 := v2.(bool)
		switch {
		case v1 != v2:
			return -1, nil
		case v1 == v2:
			return 0, nil
		default:
			return 1, nil
		}
	case int8:
		v2 := v2.(int8)
		switch {
		case v1 < v2:
			return -1, nil
		case v1 == v2:
			return 0, nil
		default:
			return 1, nil
		}
	case int16:
		v2 := v2.(int16)
		switch {
		case v1 < v2:
			return -1, nil
		case v1 == v2:
			return 0, nil
		default:
			return 1, nil
		}
	case int32:
		v2 := v2.(int32)
		switch {
		case v1 < v2:
			return -1, nil
		case v1 == v2:
			return 0, nil
		default:
			return 1, nil
		}
	case int64:
		v2 := v2.(int64)
		switch {
		case v1 < v2:
			return -1, nil
		case v1 == v2:
			return 0, nil
		default:
			return 1, nil
		}
	case uint:
		v2 := v2.(uint)
		switch {
		case v1 < v2:
			return -1, nil
		case v1 == v2:
			return 0, nil
		default:
			return 1, nil
		}
	case uint8:
		v2 := v2.(uint8)
		switch {
		case v1 < v2:
			return -1, nil
		case v1 == v2:
			return 0, nil
		default:
			return 1, nil
		}
	case uint16:
		v2 := v2.(uint16)
		switch {
		case v1 < v2:
			return -1, nil
		case v1 == v2:
			return 0, nil
		default:
			return 1, nil
		}
	case uint32:
		v2 := v2.(uint32)
		switch {
		case v1 < v2:
			return -1, nil
		case v1 == v2:
			return 0, nil
		default:
			return 1, nil
		}
	case uint64:
		v2 := v2.(uint64)
		switch {
		case v1 < v2:
			return -1, nil
		case v1 == v2:
			return 0, nil
		default:
			return 1, nil
		}
	case float32:
		v2 := v2.(float32)
		switch {
		case v1 < v2:
			return -1, nil
		case v1 == v2:
			return 0, nil
		default:
			return 1, nil
		}
	case float64:
		v2 := v2.(float64)
		switch {
		case v1 < v2:
			return -1, nil
		case v1 == v2:
			return 0, nil
		default:
			return 1, nil
		}
	case []diffLiteralT:
		v2 := v2.([]diffLiteralT)
		if len(v1) <= len(v2) {
			for i, lt1 := range v1 {
				lt2 := v2[i]
				switch {
				case lt1.kind < lt2.kind:
					return -1, nil
				case lt1.kind == lt2.kind:
					cmp, err := diffCompareLiteralValue(lt1.value, lt2.value)
					if err != nil {
						return 0, err
					}
					if cmp != 0 {
						return cmp, nil
					}
				}
			}
			if len(v1) < len(v2) {
				return -1, nil
			}
			return 0, nil
		}
		for i, lt2 := range v2 {
			lt1 := v1[i]
			switch {
			case lt1.kind < lt2.kind:
				return -1, nil
			case lt1.kind == lt2.kind:
				cmp, err := diffCompareLiteralValue(lt1.value, lt2.value)
				if err != nil {
					return 0, err
				}
				if cmp != 0 {
					return cmp, nil
				}
			}
		}
		return 1, nil
	default:
		return 0, fmt.Errorf("unsupported literal value: %T", v1)
	}
}

// Diff implements NamedGraph.Diff. See the interface documentation for the
// comparison semantics and output format.
func (g *namedGraph) Diff(graph2 NamedGraph) ([]DiffTriple, error) {
	otherNG, ok := graph2.(*namedGraph)
	if !ok {
		return nil, fmt.Errorf("other is not a namedGraph")
	}

	// 1. Serialize both graphs to SST.
	var fromBuf, toBuf bytes.Buffer
	if err := g.SstWrite(&fromBuf); err != nil {
		return nil, err
	}
	if err := otherNG.SstWrite(&toBuf); err != nil {
		return nil, err
	}

	// 2. SstWriteDiff for content-level differences.
	diffTriples, err := SstWriteDiff(
		bufio.NewReader(&fromBuf),
		bufio.NewReader(&toBuf),
		io.Discard,
		true,
	)
	if err != nil {
		return nil, err
	}

	// 3. Prepend import-level differences so they appear first.
	importDiffs := diffImports(g, otherNG)
	diffTriples = append(importDiffs, diffTriples...)

	return diffTriples, nil
}

// diffImports compares direct imports of two NamedGraphs and returns synthetic
// DiffTriple entries describing added, removed, and revision-changed imports.
// The Obj field is extended to include the hash value:
//   - added/removed: "<iri> #<hash>"
//   - modified old:  "<iri> #<oldHash>"
//   - modified new:  "#<newHash>"
func diffImports(from, to *namedGraph) []DiffTriple {
	fromImports := make(map[string]Hash)
	for _, ng := range from.directImports {
		fromImports[string(ng.IRI())] = ng.lastCheckedOutDSRevision()
	}
	toImports := make(map[string]Hash)
	for _, ng := range to.directImports {
		toImports[string(ng.IRI())] = ng.lastCheckedOutDSRevision()
	}

	var diffs []DiffTriple
	// Added imports: IRI and hash combined in Obj.
	for iri, rev := range toImports {
		if _, ok := fromImports[iri]; !ok {
			diffs = append(diffs, DiffTriple{
				Flag: TripleAdded,
				Sub:  string(from.IRI()),
				Pred: "owl:imports",
				Obj:  iri + " #" + rev.String(),
			})
		}
	}
	// Removed imports: IRI and hash combined in Obj.
	for iri, rev := range fromImports {
		if _, ok := toImports[iri]; !ok {
			diffs = append(diffs, DiffTriple{
				Flag: TripleRemoved,
				Sub:  string(from.IRI()),
				Pred: "owl:imports",
				Obj:  iri + " #" + rev.String(),
			})
		}
	}
	// Modified imports (same IRI, different revision).
	// Old revision as "<iri> #<oldHash>", new revision as "#<newHash>".
	for iri, fromRev := range fromImports {
		if toRev, ok := toImports[iri]; ok && fromRev != toRev {
			diffs = append(diffs, DiffTriple{
				Flag: TripleRemoved,
				Sub:  string(from.IRI()),
				Pred: "owl:imports",
				Obj:  iri + " #" + fromRev.String(),
			})
			diffs = append(diffs, DiffTriple{
				Flag: TripleAdded,
				Sub:  "",
				Pred: "",
				Obj:  strings.Repeat(" ", len(iri)+1) + "#" + toRev.String(),
			})
		}
	}
	return diffs
}
