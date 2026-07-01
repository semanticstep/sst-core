// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"bufio"
	"errors"
	"io"
	"sort"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	ErrMisalignedDiff        = errors.New("misaligned diff")
	ErrUnrecognizedDiffEntry = errors.New("unrecognized diff entry")
	ErrMissingBaseFile       = errors.New("missing base file")
)

type diffEntrySpan struct {
	diffEntry
	span uint
}

type deltaSpanT struct {
	// Tracks the net changes (added - removed) for each diff.
	// delta[0] for the base sst file
	// delta[1] for the diff file
	deltas []int

	// Cumulative sums of deltas for efficient indexing.
	// delta[0] + deltas[1]
	cumulativeCount *int

	// Number of entries to read from each diff
	// length = len(rDiffs), number of rDiffs[i] entries to read
	identicalCount *uint

	// Detailed information about each diff entry, including the type of change and its span.
	entries []diffEntrySpan
}

type originToTranslated struct{ original, translated uint }

// defined in sst\sstreader.go:
// type namedGraphAndIndexedNodes = struct {
// 	graph       NamedGraph
// 	sortedNodes []IBNode
// }

// main single structure for the temporary storage of the content for base and diff files
type graphDiffReadingContext struct {
	// manage and track the offsets of nodes (IBNodes) across
	// different graphs (base, imported, and referenced graphs)
	// The first entry (graphOffsets[0]) is always 0, representing the base graph.
	// Subsequent entries correspond to the cumulative IBNode counts for imported and referenced graphs.
	// For example:
	// 	graphOffsets[1] = Total IBNodes in the base graph.
	// 	graphOffsets[2] = Total IBNodes in the base graph + the first imported graph.
	//  and so on.
	graphOffsets []int

	// manage and track changes (additions, removals, or modifications) to IRI nodes
	// in the base NamedGraph during the processing of graph diffs
	// will be used in diffReadContent
	iriNodesDeltaSpan   deltaSpanT // for baseNG
	blankNodesDeltaSpan deltaSpanT // for baseNG
	sortedNodes         []IBNode   // for baseNG

	sortedImportedGraphs    []namedGraphAndIndexedNodes
	importedGraphsDeltaSpan deltaSpanT
	// TBD: blankNodesDeltaSpan for imported NamedGraphs is missing
	sortedReferencedGraphs    []namedGraphAndIndexedNodes
	referencedGraphsDeltaSpan deltaSpanT

	// for calculating the index
	// virtual graph means the constructed graph from the base and diffs
	// delta graph means the diff graph
	// translation of node indices from the virtual graph representation to the delta representation.
	pendingVirtualToDelta originToTranslated
	// translating node indices from the delta representation to the virtual graph representation
	pendingDeltaToVirtual originToTranslated

	// for multiple diffs
	virtualToDeltaTranslations []originToTranslated
	deltaToVirtualTranslations []originToTranslated
}

// SstReadDiff reads a base sst binary and a binary diff on this and returns the resulting NamedGraph.
// TBD: add a plusNotMinus parameter
// func SstReadDiff(mode TriplexMode, rBase *bufio.Reader, rDiff *bufio.Reader, plusNotMinus bool) (NamedGraph, error)
func SstReadDiff(mode TriplexMode, rBase *bufio.Reader, rDiff *bufio.Reader) (NamedGraph, error) {
	var graphID uuid.UUID
	if rBase != nil {
		err := readHeaderMagic(rBase)
		if err != nil {
			return nil, err
		}
		graphURL, err := readString(rBase)
		if err != nil {
			return nil, err
		}
		GlobalLogger.Debug("readString from base", zap.String("graphURL", graphURL))
		graphID, err = getUUID(graphURL)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, ErrMissingBaseFile
	}

	st := OpenStage(DefaultTriplexMode)
	// create the NamedGraph for the base sst file
	ng := st.CreateNamedGraph(IRI(graphID.URN()))

	var err error
	var gc graphDiffReadingContext
	GlobalLogger.Debug("diffReadHeader start")
	err = diffReadHeader(ng, &gc, rBase, rDiff)
	if err != nil {
		return ng, err
	}

	GlobalLogger.Debug("diffReadDictionary start")
	err = diffReadDictionary(ng, &gc, rBase, rDiff)
	if err != nil {
		return ng, err
	}

	GlobalLogger.Debug("diffReadContent start")
	err = diffReadContent(&gc, rBase, rDiff)
	if err != nil {
		return ng, err
	}

	// check if bufio.Reader still has left content to be read
	_, err = rBase.ReadByte()
	if !errors.Is(err, io.EOF) {
		// return graph, ErrEOFExpected
		panic(ErrEOFExpected)
	}

	_, err = rDiff.ReadByte()
	if !errors.Is(err, io.EOF) {
		// return graph, ErrEOFExpected
		panic(ErrEOFExpected)
	}

	return ng, nil
}

// creates empty imported and referenced NamedGraphs in the Stage of baseNG
// and start filling the gc structure
func diffReadHeader(
	baseNG NamedGraph, gc *graphDiffReadingContext,
	rBase *bufio.Reader, rDiff *bufio.Reader,
) error {
	tempImportedGraphsDeltaSpan := deltaSpanT{
		entries: []diffEntrySpan{},
	}
	totalImportedGraphsCount, err := tempImportedGraphsDeltaSpan.countDeltas(rBase, rDiff)
	if err != nil {
		return err
	}
	gc.sortedImportedGraphs = make([]namedGraphAndIndexedNodes, 0, totalImportedGraphsCount)
	gc.importedGraphsDeltaSpan = tempImportedGraphsDeltaSpan.copy()
	err = tempImportedGraphsDeltaSpan.readEntries(rBase, rDiff, func(_ int, r *bufio.Reader, f diffEntryWithFlags) error {
		// this callback function is updating gc and Stage
		if f.diffEntry() == diffEntrySame {
			return nil
		}
		if f.diffEntry() == diffEntryTripleModified {
			GlobalLogger.Panic(ErrUnrecognizedDiffEntry.Error())
		}
		importIRI, err := readString(r)
		if err != nil {
			return err
		}
		if r == rBase {
			GlobalLogger.Debug("readString from base", zap.String("importIRI", importIRI))
		} else {
			GlobalLogger.Debug("readString from diff", zap.String("importIRI", importIRI))
		}
		if f&diffEntryInheritRemoved == diffEntryInheritRemoved {
			return nil
		}
		importID, err := getUUID(importIRI)
		if err != nil {
			return err
		}
		ig := baseNG.Stage().CreateNamedGraph(IRI(importID.URN()))
		if ig == nil {
			panic("failed to create named graph")
		}
		baseNG.AddImport(ig)
		gc.sortedImportedGraphs = append(gc.sortedImportedGraphs, namedGraphAndIndexedNodes{graph: ig})
		return nil
	})
	if err != nil {
		return err
	}
	gc.importedGraphsDeltaSpan.entries = tempImportedGraphsDeltaSpan.entries

	tempReferencedGraphsDeltaSpan := deltaSpanT{
		entries: []diffEntrySpan{},
	}
	totalReferencedGraphsCount, err := tempReferencedGraphsDeltaSpan.countDeltas(rBase, rDiff)
	if err != nil {
		return err
	}
	gc.sortedReferencedGraphs = make([]namedGraphAndIndexedNodes, 0, totalReferencedGraphsCount)
	gc.referencedGraphsDeltaSpan = tempReferencedGraphsDeltaSpan.copy()
	err = tempReferencedGraphsDeltaSpan.readEntries(rBase, rDiff, func(_ int, r *bufio.Reader, f diffEntryWithFlags) error {
		if f.diffEntry() == diffEntrySame {
			return nil
		}
		if f.diffEntry() == diffEntryTripleModified {
			GlobalLogger.Panic(ErrUnrecognizedDiffEntry.Error())
		}
		referencedGraphIRI, err := readString(r)
		if err != nil {
			return err
		}
		if r == rBase {
			GlobalLogger.Debug("readString from base", zap.String("referencedGraph", referencedGraphIRI))
		} else {
			GlobalLogger.Debug("readString from diff", zap.String("referencedGraph", referencedGraphIRI))
		}

		if f&diffEntryInheritRemoved == diffEntryInheritRemoved {
			return nil
		}
		eg := baseNG.Stage().referencedGraphByURI(referencedGraphIRI)

		gc.sortedReferencedGraphs = append(gc.sortedReferencedGraphs, namedGraphAndIndexedNodes{graph: eg})
		return nil
	})
	gc.referencedGraphsDeltaSpan.entries = tempReferencedGraphsDeltaSpan.entries
	return err
}

// the invocation of gatherDiffTriplesForReading and gatherDiffBlankTriplesForReading
// creates all needed IBNodes in baseNG and return the total triple count.
// Then reserves the slices of triples/triplexes for baseNG.
// gatherDiffGraphTriplesForReading will handle the imported NamedGraphs and referenced NamedGraphs.
//
// extend gc structure for IBNodes
func diffReadDictionary(
	baseNG NamedGraph, gc *graphDiffReadingContext,
	rBase *bufio.Reader, rDiff *bufio.Reader,
) error {
	// read IRINodes deltaSpan
	tempIRINodesDeltaSpan := deltaSpanT{
		entries: []diffEntrySpan{},
	}
	totalIRINodeCount, err := tempIRINodesDeltaSpan.countDeltas(rBase, rDiff)
	if err != nil {
		return err
	}
	gc.iriNodesDeltaSpan = tempIRINodesDeltaSpan.copy()

	// read BlankNodes deltaSpan
	tempBlankNodesDeltaSpan := deltaSpanT{
		entries: []diffEntrySpan{},
	}
	totalBlankNodesCount, err := tempBlankNodesDeltaSpan.countDeltas(rBase, rDiff)
	if err != nil {
		return err
	}
	gc.blankNodesDeltaSpan = tempBlankNodesDeltaSpan.copy()

	// initialize
	gc.sortedNodes = make([]IBNode, totalIRINodeCount+totalBlankNodesCount)
	gc.deltaToVirtualTranslations = []originToTranslated{}
	gc.virtualToDeltaTranslations = []originToTranslated{}
	gc.pendingVirtualToDelta = originToTranslated{}
	gc.pendingDeltaToVirtual = originToTranslated{}

	GlobalLogger.Debug("diffReadDictionary for IRI Nodes")
	tripleCount, err := gatherDiffTriplesForReading(&tempIRINodesDeltaSpan, baseNG.(*namedGraph), gc.sortedNodes, gc, rBase, rDiff)
	if err != nil {
		return err
	}
	gc.iriNodesDeltaSpan.entries = tempIRINodesDeltaSpan.entries

	GlobalLogger.Debug("diffReadDictionary for Blank Nodes")
	tripleCount, err = gatherDiffBlankTriplesForReading(&tempBlankNodesDeltaSpan, baseNG.(*namedGraph), gc, totalIRINodeCount, tripleCount, rBase, rDiff)
	if err != nil {
		return err
	}
	gc.blankNodesDeltaSpan.entries = tempBlankNodesDeltaSpan.entries

	baseNG.allocateTriplexes(tripleCount)

	// initialize graph offsets
	gc.graphOffsets = make([]int, len(gc.sortedImportedGraphs)+len(gc.sortedReferencedGraphs)+1)
	// gc.graphOffsets[0] means baseNG
	gc.graphOffsets[0] = 0
	graphOffsetTop := totalIRINodeCount + totalBlankNodesCount

	// for imported graphs
	// TBD: missing invocation of gatherDiffBlankTriplesForReading in imported NGs
	err = gatherDiffGraphTriplesForReading(rBase, rDiff, gc, gc.importedGraphsDeltaSpan,
		func(ed deltaSpanT, i, iriNodeCount int, cBase *bufio.Reader, cDiffs []*bufio.Reader) error {
			gc.sortedImportedGraphs[i].sortedNodes = make([]IBNode, iriNodeCount)
			importedNamedGraph := gc.sortedImportedGraphs[i].graph
			tripleCount, err := gatherDiffTriplesForReading(&ed, importedNamedGraph.(*namedGraph), gc.sortedImportedGraphs[i].sortedNodes, gc, cBase, cDiffs[0])
			if err != nil {
				return err
			}
			importedNamedGraph.allocateTriplexes(tripleCount)
			gc.graphOffsets[i+1], graphOffsetTop = graphOffsetTop, graphOffsetTop+iriNodeCount
			return nil
		})
	if err != nil {
		return err
	}

	// for referenced graphs
	err = gatherDiffGraphTriplesForReading(rBase, rDiff, gc, gc.referencedGraphsDeltaSpan,
		func(ed deltaSpanT, i, iriNodeCount int, cBase *bufio.Reader, cDiffs []*bufio.Reader) error {
			gc.sortedReferencedGraphs[i].sortedNodes = make([]IBNode, iriNodeCount)

			ng := gc.sortedReferencedGraphs[i].graph.(*namedGraph)
			tripleCount, err := gatherDiffTriplesForReading(&ed, ng, gc.sortedReferencedGraphs[i].sortedNodes, gc, cBase, cDiffs[0])
			if err != nil {
				return err
			}
			ng.allocateTriplexes(tripleCount)
			gc.graphOffsets[i+len(gc.sortedImportedGraphs)+1], graphOffsetTop = graphOffsetTop, graphOffsetTop+iriNodeCount
			return nil
		})
	if err != nil {
		return err
	}
	return nil
}

// creates all needed IBNodes in baseNG and all other imported and referenced NGs,
// reserves the slices of triples/triplexes for each.
func gatherDiffTriplesForReading(
	d *deltaSpanT, ng *namedGraph, sortedNodes []IBNode,
	gc *graphDiffReadingContext, rBase *bufio.Reader, rDiff *bufio.Reader,
) (tripleCount int, err error) {
	i := 0
	allocatedTriplexCntDelta := 0
	err = d.readEntries(rBase, rDiff, func(readerSource int, r *bufio.Reader, f diffEntryWithFlags) error {
		switch e := f.diffEntry(); e {
		case diffEntryTripleModified:
			delta, err := readInt64(r)
			if err != nil {
				return err
			}
			if r == rBase {
				GlobalLogger.Debug("readUint from base", zap.Int64("delta", delta))
			} else {
				GlobalLogger.Debug("readUint from diff", zap.Int64("delta", delta))
			}

			allocatedTriplexCntDelta += int(delta)
		case diffEntryRemoved, diffEntryAdded:
			fragment, err := readString(r)
			if err != nil {
				return err
			}
			if r == rBase {
				GlobalLogger.Debug("readString from base", zap.String("fragment", fragment))
			} else {
				GlobalLogger.Debug("readString from diff", zap.String("fragment", fragment))
			}

			var allocatedTriplexCnt uint
			if r == rBase {
				allocatedTriplexCnt, err = readUint(r)
				GlobalLogger.Debug("readUint from base", zap.Uint("allocatedTriplexCnt", allocatedTriplexCnt))
			} else {
				var a int64
				a, err = readInt64(r)
				GlobalLogger.Debug("readInt64 from diff", zap.Int64("allocatedTriplexCnt", a))
				allocatedTriplexCnt = uint(a)
			}
			if err != nil {
				return err
			}
			if e == diffEntryRemoved {
				if readerSource == 0 {
					diffReadAddNodeTranslation(f.diffEntry(), gc)
				}
				return nil
			}
			a := allocatedTriplexCntDelta
			allocatedTriplexCntDelta = 0
			if f&diffEntryInheritRemoved == diffEntryInheritRemoved {
				if readerSource == 0 {
					diffReadAddNodeTranslation(f.diffEntry(), gc)
				}
				return nil
			}
			if ng == nil {
				return nil
			}
			GlobalLogger.Debug("createAllocatedNode", zap.String("Fragment", string(fragment)), zap.Int("tripleStart", tripleCount), zap.Int("triple Count", int(allocatedTriplexCnt)+a))
			sortedNodes[i], tripleCount = ng.createAllocatedNode(string(fragment), iriNodeType,
				tripleCount, int(allocatedTriplexCnt)+a)
			i++
		case diffEntrySame:
		}
		if readerSource == 0 {
			diffReadAddNodeTranslation(f.diffEntry(), gc)
		}
		return nil
	})
	return
}

func gatherDiffBlankTriplesForReading(
	d *deltaSpanT, ng *namedGraph, gc *graphDiffReadingContext,
	fragmentTreeCount, intc int, rBase *bufio.Reader, rDiff *bufio.Reader,
) (tripleCount int, err error) {
	tripleCount = intc
	i := 0
	allocatedTriplexCntDelta := 0
	err = d.readEntries(rBase, rDiff, func(readerSource int, r *bufio.Reader, f diffEntryWithFlags) error {
		switch e := f.diffEntry(); e {
		case diffEntryTripleModified:
			delta, err := readInt64(r)
			if r == rBase {
				GlobalLogger.Debug("readUint from base", zap.Int64("delta", delta))
			} else {
				GlobalLogger.Debug("readUint from diff", zap.Int64("delta", delta))
			}
			if err != nil {
				return err
			}
			allocatedTriplexCntDelta += int(delta)
		case diffEntryRemoved, diffEntryAdded:
			var allocatedTriplexCnt uint
			if r == rBase {
				allocatedTriplexCnt, err = readUint(r)
				GlobalLogger.Debug("readUint from base", zap.Uint("allocatedTriplexCnt", allocatedTriplexCnt))
			} else {
				var a int64
				a, err = readInt64(r)
				GlobalLogger.Debug("readInt64 from diff", zap.Int64("allocatedTriplexCnt", a))
				allocatedTriplexCnt = uint(a)
			}
			if err != nil {
				return err
			}
			if e == diffEntryRemoved {
				if readerSource == 0 {
					diffReadAddNodeTranslation(f.diffEntry(), gc)
				}
				return nil
			}
			a := allocatedTriplexCntDelta
			allocatedTriplexCntDelta = 0
			if f&diffEntryInheritRemoved == diffEntryInheritRemoved {
				if readerSource == 0 {
					diffReadAddNodeTranslation(f.diffEntry(), gc)
				}
				return nil
			}
			gc.sortedNodes[i+fragmentTreeCount], tripleCount = ng.createAllocatedNode("", blankNodeType,
				tripleCount, int(allocatedTriplexCnt)+a)
			i++
		case diffEntrySame:
		}
		if readerSource == 0 {
			diffReadAddNodeTranslation(f.diffEntry(), gc)
		}
		return nil
	})
	return
}

func diffReadAddNodeTranslation(e diffEntry, gc *graphDiffReadingContext) {
	vdn := gc.pendingVirtualToDelta
	vdc := originToTranslated{original: ^uint(0), translated: 0}
	if len(gc.virtualToDeltaTranslations) > 0 {
		vdc = gc.virtualToDeltaTranslations[len(gc.virtualToDeltaTranslations)-1]
	}
	if vdn.original-vdn.translated != vdc.original-vdc.translated {
		if len(gc.virtualToDeltaTranslations) == 0 {
			gc.virtualToDeltaTranslations = append(gc.virtualToDeltaTranslations, vdn)
		} else if gc.virtualToDeltaTranslations[len(gc.virtualToDeltaTranslations)-1].original != vdn.original {
			gc.virtualToDeltaTranslations = append(gc.virtualToDeltaTranslations, vdn)
		} else {
			gc.virtualToDeltaTranslations[len(gc.virtualToDeltaTranslations)-1] = vdn
		}

	}
	dvn := gc.pendingDeltaToVirtual
	dvc := originToTranslated{original: ^uint(0), translated: 0}
	if len(gc.deltaToVirtualTranslations) > 0 {
		dvc = gc.deltaToVirtualTranslations[len(gc.deltaToVirtualTranslations)-1]
	}
	if dvn.original-dvn.translated != dvc.original-dvc.translated {
		if len(gc.deltaToVirtualTranslations) == 0 {
			gc.deltaToVirtualTranslations = append(gc.deltaToVirtualTranslations, dvn)
		} else if gc.deltaToVirtualTranslations[len(gc.deltaToVirtualTranslations)-1].original != dvn.original {
			gc.deltaToVirtualTranslations = append(gc.deltaToVirtualTranslations, dvn)
		} else {
			gc.deltaToVirtualTranslations[len(gc.deltaToVirtualTranslations)-1] = dvn
		}
	}
	switch e {
	// IBNode count not changed
	case diffEntrySame, diffEntryTripleModified:
		gc.pendingVirtualToDelta.original++
		gc.pendingVirtualToDelta.translated++
		gc.pendingDeltaToVirtual.original++
		gc.pendingDeltaToVirtual.translated++
	case diffEntryRemoved:
		// Both original and translated are incremented
		// because the virtual-to-delta mapping must account for the removal of a node.
		gc.pendingVirtualToDelta.original++
		gc.pendingVirtualToDelta.translated++

		// This is because the removed node no longer exists in the virtual graph,
		// so there is no corresponding "translated" index in the virtual graph for the delta node.
		gc.pendingDeltaToVirtual.original++
		// gc.pendingDeltaToVirtual.translated++
	case diffEntryAdded:
		// This is because the added node does not yet exist in the virtual graph.
		// The virtual graph is updated after processing the diff,
		// so there is no corresponding "original" index in the virtual graph at this stage.
		// gc.pendingVirtualToDelta.original++
		gc.pendingVirtualToDelta.translated++

		// Both original and translated are incremented
		// because the delta-to-virtual mapping must account for the new node being added to the virtual graph.
		gc.pendingDeltaToVirtual.original++
		gc.pendingDeltaToVirtual.translated++
	}
}

func gatherDiffGraphTriplesForReading(
	rBase *bufio.Reader, rDiff *bufio.Reader, gc *graphDiffReadingContext, gds deltaSpanT,
	gathererForGraph func(ed deltaSpanT, i, ibNodeCount int, cBase *bufio.Reader, cDiffs []*bufio.Reader) error,
) error {
	i := 0
	cDiffs := make([]*bufio.Reader, 0, 1)
	return gds.forEntries(rBase, rDiff, func(p int, r *bufio.Reader, f diffEntryWithFlags) error {
		switch f.diffEntry() {
		case diffEntrySame, diffEntryTripleModified, diffEntryRemoved:
			cDiffs = append(cDiffs, r)
		case diffEntryAdded:
			var cBase *bufio.Reader
			if r == rBase {
				cBase = r
			} else {
				cDiffs = append(cDiffs, r)
			}
			for i, j := 0, len(cDiffs)-1; i < j; i, j = i+1, j-1 {
				cDiffs[i], cDiffs[j] = cDiffs[j], cDiffs[i]
			}
			ed := deltaSpanT{}
			ibNodeCount, err := ed.countDeltas(cBase, cDiffs[0])
			if err != nil {
				return err
			}
			if f&diffEntryInheritRemoved != diffEntryInheritRemoved {
				err := gathererForGraph(ed, i, ibNodeCount, cBase, cDiffs)
				if err != nil {
					return err
				}
				i++
			} else {
				_, err := gatherDiffTriplesForReading(&ed, nil, nil, gc, cBase, cDiffs[0])
				if err != nil {
					return err
				}
			}

			cDiffs = cDiffs[:0]
		}
		return nil
	})
}

func diffReadContent(gc *graphDiffReadingContext, rBase *bufio.Reader, rDiffs *bufio.Reader) error {
	var identicalNodeCount uint = 0

	i := 0

	sFunc := func(readerSource int, r *bufio.Reader, f diffEntryWithFlags) error {
		switch f.diffEntry() {
		case diffEntrySame, diffEntryTripleModified, diffEntryRemoved:

		case diffEntryAdded:
			var chosenIBNode IBNode
			if f&diffEntryInheritRemoved != diffEntryInheritRemoved {
				chosenIBNode = gc.sortedNodes[i]
				i++
			}
			// add triples for the chosenIBNode
			err := diffReadIBNode(gc, rBase, rDiffs, &identicalNodeCount, readerSource, chosenIBNode)
			if err != nil {
				return err
			}

		}
		return nil
	}

	GlobalLogger.Debug("diffReadContent iriNodesDeltaSpan start")
	err := gc.iriNodesDeltaSpan.forEntries(rBase, rDiffs, sFunc)
	if err != nil {
		return err
	}

	GlobalLogger.Debug("diffReadContent blankNodesDeltaSpan start")
	err = gc.blankNodesDeltaSpan.forEntries(rBase, rDiffs, sFunc)
	if err != nil {
		return err
	}
	return nil
}

// 2D slice of originToTranslated
type originGrid [][]originToTranslated

// for originToTranslated
func (o originToTranslated) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddUint("original", o.original)
	enc.AddUint("translated", o.translated)
	return nil
}

// slice of originToTranslated
type originSlice []originToTranslated

func (s originSlice) MarshalLogArray(enc zapcore.ArrayEncoder) error {
	for _, item := range s {
		enc.AppendObject(item)
	}
	return nil
}

func (g originGrid) MarshalLogArray(enc zapcore.ArrayEncoder) error {
	for _, row := range g {
		enc.AppendArray(zapcore.ArrayMarshalerFunc(func(inner zapcore.ArrayEncoder) error {
			return originSlice(row).MarshalLogArray(inner)
		}))
	}
	return nil
}

func diffReadIBNode(gc *graphDiffReadingContext, rBase *bufio.Reader, rDiff *bufio.Reader,
	identicalNodeCount *uint, readerSource int, chosenIBNode IBNode,
) error {
	if chosenIBNode != nil {
		if chosenIBNode.IsIRINode() {
			if chosenIBNode.Fragment() == "" {
				GlobalLogger.Debug("diffReadIBNode start", zap.Int("readerSource", readerSource), zap.String("fragment", "NG Node"))

			} else {
				GlobalLogger.Debug("diffReadIBNode start", zap.Int("readerSource", readerSource), zap.String("fragment", chosenIBNode.Fragment()))
			}
		} else {
			GlobalLogger.Debug("diffReadIBNode start", zap.Int("readerSource", readerSource), zap.String("ID", chosenIBNode.ID().String()))
		}
	}
	t := make(originGrid, 0, len(gc.virtualToDeltaTranslations)+len(gc.deltaToVirtualTranslations))
	nodeDeltaSpan := deltaNodeSpanT{}
	_, err := nodeDeltaSpan.countDeltas(rBase, rDiff, identicalNodeCount, readerSource)
	if err != nil {
		return err
	}
	GlobalLogger.Debug("identicalTripleCount", zap.Uint("non-literal", *identicalNodeCount))
	err = nodeDeltaSpan.readEntries(rBase, rDiff, identicalNodeCount, readerSource,
		func(p int, r *bufio.Reader, f diffEntryWithFlags, pred uint) error {
			removed := chosenIBNode == (nil) || f&diffEntryInheritRemoved == diffEntryInheritRemoved
			GlobalLogger.Debug("removed", zap.Bool("removed", removed))
			switch f.diffEntry() {
			case diffEntrySame:
				GlobalLogger.Debug("enter diffEntrySame")
				if !removed {
					t = append(t, gc.deltaToVirtualTranslations)
					t = append(t, gc.virtualToDeltaTranslations)
					GlobalLogger.Debug("translation struct modified to", zap.Array("translation", t))
				}
				return nil
			case diffEntryTripleModified:
				GlobalLogger.Debug("enter diffEntryTripleModified")
				if !removed {
					t = append(t, gc.deltaToVirtualTranslations)
					t = append(t, gc.virtualToDeltaTranslations)
					GlobalLogger.Debug("translation struct modified to", zap.Array("translation", t))
				}
				return nil
			case diffEntryRemoved:
				GlobalLogger.Debug("enter diffEntryRemoved")
				re, err := readUint(r)
				if r == rBase {
					GlobalLogger.Debug("readUint from base", zap.Uint("removed object index", re))
				} else {
					GlobalLogger.Debug("readUint from diff", zap.Uint("removed object index", re))
				}
				return err
			case diffEntryAdded:
				GlobalLogger.Debug("enter diffEntryAdded")
				obj, err := readUint(r)
				if r == rBase {
					GlobalLogger.Debug("readUint from base", zap.Uint("object index", obj))
				} else {
					GlobalLogger.Debug("readUint from diff", zap.Uint("object index", obj))
				}
				if err != nil {
					return err
				}
				if removed {
					GlobalLogger.Debug("removed skip")
					return nil
				}

				// p >=0 means that r == rDiff
				if p >= 0 {
					t = append(t, gc.deltaToVirtualTranslations)
					GlobalLogger.Debug("translation struct modified to", zap.Array("translation", t))
				}
				predIBNode := diffIndexToIBNode(pred, gc, t)
				objIBNode := diffIndexToIBNode(obj, gc, t)

				chosenIBNode.appendUniqueNodeTriple(predIBNode, objIBNode)

				t = t[:0]
			}
			return nil
		})
	if err != nil {
		return err
	}
	_, err = nodeDeltaSpan.countLiteralDeltas(rBase, rDiff, identicalNodeCount, readerSource)
	if err != nil {
		return err
	}
	t = t[:0]
	GlobalLogger.Debug("identicalTripleCount", zap.Uint("literal", *identicalNodeCount))
	err = nodeDeltaSpan.readEntries(rBase, rDiff, identicalNodeCount, readerSource,
		func(readerSource int, r *bufio.Reader, f diffEntryWithFlags, pred uint) error {
			removed := chosenIBNode == (nil) || f&diffEntryInheritRemoved == diffEntryInheritRemoved
			switch f.diffEntry() {
			case diffEntrySame:
				GlobalLogger.Debug("enter diffEntrySame")
				if !removed {
					t = append(t, gc.deltaToVirtualTranslations)
					t = append(t, gc.virtualToDeltaTranslations)
					GlobalLogger.Debug("translation struct modified to", zap.Array("translation", t))
				}
				return nil
			case diffEntryTripleModified:
				GlobalLogger.Debug("enter diffEntryTripleModified")
				if !removed {
					t = append(t, gc.deltaToVirtualTranslations)
					t = append(t, gc.virtualToDeltaTranslations)
					GlobalLogger.Debug("translation struct modified to", zap.Array("translation", t))
				}
				return nil
			case diffEntryRemoved:
				GlobalLogger.Debug("enter diffEntryRemoved")
				err := readLiteral(readerSource, r, true, nil, nil)
				return err
			case diffEntryAdded:
				GlobalLogger.Debug("enter diffEntryAdded")
				err := readLiteral(readerSource, r, true, func(obj Literal) {
					if removed {
						return
					}
					if readerSource >= 0 {
						t = append(t, gc.deltaToVirtualTranslations)
					}
					chosenIBNode.appendUniqueLiteralTriple(diffIndexToIBNode(pred, gc, t), obj)

					t = t[:0]
				}, func(l LiteralCollection) {
					if removed {
						return
					}
					if readerSource >= 0 {
						t = append(t, gc.deltaToVirtualTranslations)
					}
					chosenIBNode.appendUniqueLiteralCollectionTriple(diffIndexToIBNode(pred, gc, t), l)

					t = t[:0]
				})
				if err != nil {
					return err
				}
			}
			return nil
		})
	if err != nil {
		return err
	}
	if chosenIBNode != nil {
		chosenIBNode.extractCollectionMembers()
	}

	if chosenIBNode != nil {
		if chosenIBNode.IsIRINode() {
			if chosenIBNode.Fragment() == "" {
				GlobalLogger.Debug("diffReadIBNode end", zap.Int("readerSource", readerSource), zap.String("fragment", "NG Node"))
			} else {
				GlobalLogger.Debug("diffReadIBNode end", zap.Int("readerSource", readerSource), zap.String("fragment", chosenIBNode.Fragment()))
			}
		} else {
			GlobalLogger.Debug("diffReadIBNode end", zap.Int("readerSource", readerSource), zap.String("ID", chosenIBNode.ID().String()))
		}
	}

	return nil
}

func diffIndexToIBNode(index uint, gc *graphDiffReadingContext, t originGrid) IBNode {
	// IBNode Index
	translated := diffReadTranslateNode(index, t)
	GlobalLogger.Debug("translation struct", zap.Array("translation", t))
	GlobalLogger.Debug("diffIndexToIBNode", zap.Uint("index", index), zap.Uint("translated", translated))
	// Graph Index
	offset := sort.SearchInts(gc.graphOffsets, int(translated))
	if offset >= len(gc.graphOffsets) || gc.graphOffsets[offset] != int(translated) {
		offset--
	}
	if offset != len(gc.graphOffsets)-1 && gc.graphOffsets[offset] == gc.graphOffsets[offset+1] {
		offset++
	}
	switch {
	case offset == 0:
		return gc.sortedNodes[translated]
	case offset <= len(gc.sortedImportedGraphs):
		return gc.sortedImportedGraphs[offset-1].sortedNodes[translated-uint(gc.graphOffsets[offset])]
	default:
		if offset-len(gc.sortedImportedGraphs)-1 >= len(gc.sortedReferencedGraphs) {
			GlobalLogger.Panic("offset out of range")
		}
		if translated-uint(gc.graphOffsets[offset]) >= uint(len(gc.sortedReferencedGraphs[offset-len(gc.sortedImportedGraphs)-1].sortedNodes)) {
			GlobalLogger.Panic("translated out of range")
		}
		return gc.sortedReferencedGraphs[offset-len(gc.sortedImportedGraphs)-1].sortedNodes[translated-uint(gc.graphOffsets[offset])]
	}
}

// diffReadTranslateNode translates an index through a series of translation tables.
// It iteratively applies the translation tables in reverse order, starting from the last table
// in the slice and working towards the first. The function returns the final translated index.
//
// Parameters:
//   - index: The initial index to be translated.
//   - t: A slice of translation tables, where each table is a slice of originToTranslated mappings.
//
// Returns:
//   - uint: The final translated index after applying all translation tables.
func diffReadTranslateNode(index uint, t [][]originToTranslated) uint {
	r := index
	for i := len(t) - 1; i >= 0; i-- {
		r = diffTranslateNode(r, t[i])
	}
	return r
}
