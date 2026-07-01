// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"math/bits"
	"sort"
	"strings"
	"unsafe"

	"github.com/google/uuid"
	fs "github.com/relab/wrfs"
	"go.uber.org/zap"
)

var (
	ErrHeaderExpected  = errors.New("header expected")
	ErrEOFExpected     = errors.New("eof expected")
	ErrDuplicatedNodes = errors.New("duplicated nodes")
	ErrNoContent       = errors.New("no content")
	errNotaUUID        = errors.New("not a UUID")
)

type namedGraphAndIndexedNodes = struct {
	graph       NamedGraph
	sortedNodes []IBNode
}

type graphReadingContext struct {
	graphOffsets           []int
	sortedNodes            []IBNode
	sortedImportedGraphs   []namedGraphAndIndexedNodes
	sortedReferencedGraphs []namedGraphAndIndexedNodes
}

// SstRead reads an SST binary file and returns the contained main NamedGraph.
func SstRead(r *bufio.Reader, mode TriplexMode) (NamedGraph, error) {
	st := OpenStage(mode)
	graphID, graphIRI, err := sstReadGraphID(r)
	if err != nil {
		return nil, err
	}
	GlobalLogger.Debug("SstRead", zap.String("graphIRI", graphIRI))

	var ng NamedGraph
	if graphIRI == "" {
		ng = st.CreateNamedGraph(IRI(graphID.URN()))
	} else {
		ng = st.CreateNamedGraph(IRI(graphIRI))
	}

	var gc graphReadingContext
	err = readHeader(r, st, ng, &gc)
	if err != nil {
		panic(err)
	}
	// reads the dictionary section of the binary SST file
	err = readDictionary(r, ng, &gc)
	if err != nil {
		panic(err)
	}
	// reads the content section of a binary SST file
	err = readContent(r, &gc)
	if err != nil {
		panic(err)
	}
	_, err = r.ReadByte()
	if !errors.Is(err, io.EOF) {
		return ng, ErrEOFExpected
	}
	return ng, nil
}

func sstReadGraphImports(r *bufio.Reader) (NamedGraph, error) {
	graphID, graphIRI, err := sstReadGraphID(r)
	if err != nil {
		return nil, err
	}
	stage := OpenStage(DefaultTriplexMode)

	var graph NamedGraph
	if graphIRI == "" {
		graph = stage.CreateNamedGraph(IRI(graphID.URN()))
	} else {
		graph = stage.CreateNamedGraph(IRI(graphIRI))
	}
	var gc graphReadingContext
	err = readHeaderImports(r, stage, graph, &gc)
	return graph, err
}

// reads header of a binary SST file(includes SST-1.0 AND urn:uuid)
func sstReadGraphID(r *bufio.Reader) (graphID uuid.UUID, iri string, _ error) {
	// ensures that the file starts with the string "SST-1.0"
	err := readHeaderMagic(r)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return graphID, "", ErrNoContent
		}
		return graphID, "", err
	}
	// reads the IRI (i.e. URN-UUID for application NamedGraphs or the standard URL/URN) string
	// of the NamedGraph stored in the SST binary file
	graphURI, err := readString(r)
	if err != nil {
		return graphID, "", err
	}
	graphID, err = getUUID(graphURI)
	var notUUID bool
	if err != nil {
		if !errors.Is(err, errNotaUUID) {
			return graphID, "", err
		}
		notUUID = true
	}
	if notUUID {
		graphIRI, err := NewIRI(graphURI)
		if err != nil {
			panic(err)
		}
		graphID = uuid.NewSHA1(uuid.NameSpaceURL, ([]byte)(graphIRI.String()))
		return graphID, graphURI, nil
	}
	return graphID, "", nil
}

// sstReadGraphURI reads only the NamedGraph IRI from a binary SST file.
// Returns ErrNoContent is the file is otherwise empty.
func sstReadGraphURI(r *bufio.Reader) (graphURI string, _ error) {
	err := readHeaderMagic(r)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return graphURI, ErrNoContent
		}
		return graphURI, err
	}
	return readString(r)
}

// check that the file starts with "SST-1.0"
func readHeaderMagic(r *bufio.Reader) error {
	header := make([]byte, len(sstFileHeader))
	_, err := io.ReadFull(r, header)
	if err != nil {
		return err
	}
	if !bytes.Equal(header, sstFileHeader[:]) {
		return ErrHeaderExpected
	}
	return nil
}

// fills gc.sortedReferencedGraphs
func readHeader(
	r *bufio.Reader, stage Stage, graph NamedGraph, gc *graphReadingContext,
) error {
	err := readHeaderImports(r, stage, graph, gc)
	if err != nil {
		return err
	}
	// number of other referenced NamedGraphs (that are not imported, e.g. rdf, rdfs, owl, lci, ...)
	referencedGraphCount, err := readUint(r)
	if err != nil {
		return err
	}
	gc.sortedReferencedGraphs = make([]namedGraphAndIndexedNodes, referencedGraphCount)
	// loop over other referenced NamedGraph, sorted alphabetically:
	for i := uint(0); i < referencedGraphCount; i++ {
		// the IRI string of the referenced NamedGraphs
		referencedGraphIRI, err := readString(r)
		if err != nil {
			return err
		}
		eg := stage.referencedGraphByURI(referencedGraphIRI)
		gc.sortedReferencedGraphs[i] = namedGraphAndIndexedNodes{graph: eg}
	}
	return nil
}

// fills gc.sortedGraphImports
func readHeaderImports(
	r *bufio.Reader, sourceStage Stage, graph NamedGraph, gc *graphReadingContext,
) error {
	// the number of explicitly **imported** NamedGraphs
	graphImportCount, err := readUint(r)
	if err != nil {
		return err
	}
	gc.sortedImportedGraphs = make([]namedGraphAndIndexedNodes, graphImportCount)
	// loop over all imported NamedGraph, sorted alphabetically according the IRI (e.g. a URN-UUID)
	for i := uint(0); i < graphImportCount; i++ {
		// the IRI (i.e. URN-UUID for application NamedGraphs or the standard URL/URN) string of the imported NamedGraph;
		// might be an sortedReferencedGraphs IRI managed by another organization
		importURI, err := readString(r)
		if err != nil {
			return err
		}
		ig := sourceStage.referencedGraphByURI(importURI)
		graph.AddImport(ig)
		gc.sortedImportedGraphs[i] = namedGraphAndIndexedNodes{graph: ig}
	}
	return nil
}

// read urn:uuid: beginning string, extract as uuid
func getUUID(urnStr string) (parsedUUID uuid.UUID, err error) {
	uuidStr := strings.TrimPrefix(urnStr, "urn:uuid:")
	if len(uuidStr) == len(urnStr) {
		err = errNotaUUID
		return
	}
	return uuid.Parse(uuidStr)
}

// Reads the dictionary section of a binary SST file and fills gc.sortedNodes
func readDictionary(r *bufio.Reader, graph NamedGraph, gc *graphReadingContext) error {
	var err error
	iriNodeCount, err := readUint(r)
	if err != nil {
		panic(err)
	}
	GlobalLogger.Debug("readDictionary: ", zap.Uint("iriNodeCount", iriNodeCount))

	blankNodeCount, err := readUint(r)
	if err != nil {
		panic(err)
	}
	GlobalLogger.Debug("readDictionary: ", zap.Uint("blankNodeCount", blankNodeCount))
	gc.sortedNodes = make([]IBNode, iriNodeCount+blankNodeCount)

	var tripleCount int
	tripleCount, err = gatherTriplesForReading(r, graph.(*namedGraph), iriNodeCount, gc.sortedNodes)
	if err != nil {
		return err
	}

	// handle blank nodes
	// SST files do not persist blank node UUIDs. To ensure that multiple reads
	// of the same file produce identical in-memory state (aiding debugging and
	// in-memory diff alignment), each blank node is reconstructed with a
	// deterministic type 5 UUID derived from the graph ID and its index position.
	for i := uint(0); i < blankNodeCount; i++ {
		allocatedTriplexCnt, err := readUint(r)
		if err != nil {
			panic(err)
		}
		var d IBNode
		d, tripleCount = graph.createAllocatedNode("", blankNodeType, tripleCount, int(allocatedTriplexCnt))
		uuidData := [11]byte{'b'}
		uuidDataLen := binary.PutUvarint(uuidData[1:], (uint64)(i))
		err = d.asUuidIBNode().setFragment(uuid.NewSHA1(graph.ID(), uuidData[0:uuidDataLen+1]))
		if err != nil {
			return err
		}
		gc.sortedNodes[iriNodeCount+i] = d
	}
	graph.allocateTriplexes(tripleCount)
	gc.graphOffsets = make([]int, len(gc.sortedImportedGraphs)+len(gc.sortedReferencedGraphs)+1)
	gc.graphOffsets[0] = 0
	graphOffsetTop := int(iriNodeCount + blankNodeCount)
	// imported NamedGraphs
	for i := range gc.sortedImportedGraphs {
		referencedTreeCount, err := readUint(r)
		if err != nil {
			return err
		}
		gc.sortedImportedGraphs[i].sortedNodes = make([]IBNode, referencedTreeCount)
		importedNamedGraph := gc.sortedImportedGraphs[i].graph
		_, err = gatherTriplesForReading(r, importedNamedGraph.(*namedGraph), referencedTreeCount, gc.sortedImportedGraphs[i].sortedNodes)
		if err != nil {
			return err
		}
		importedNamedGraph.allocateTriplexes(tripleCount)
		gc.graphOffsets[i+1], graphOffsetTop = graphOffsetTop, graphOffsetTop+int(referencedTreeCount)
	}
	for i := range gc.sortedReferencedGraphs {
		referencedTreeCount, err := readUint(r)
		if err != nil {
			return err
		}
		gc.sortedReferencedGraphs[i].sortedNodes = make([]IBNode, referencedTreeCount)
		ng := gc.sortedReferencedGraphs[i].graph.(*namedGraph)
		tripleCount, err := gatherTriplesForReading(r, ng, referencedTreeCount, gc.sortedReferencedGraphs[i].sortedNodes)
		if err != nil {
			return err
		}
		ng.allocateTriplexes(tripleCount)
		gc.graphOffsets[i+len(gc.sortedImportedGraphs)+1], graphOffsetTop = graphOffsetTop, graphOffsetTop+int(referencedTreeCount)
	}
	return nil
}

func gatherTriplesForReading(
	r *bufio.Reader,
	ng *namedGraph,
	nodeCount uint,
	sortedNodes []IBNode,
) (tripleCount int, err error) {
	if ng != nil {
		GlobalLogger.Debug("gatherTriplesForReading: ", zap.String("NG BaseIRI", ng.baseIRI))
	} else {
		GlobalLogger.Debug("gatherTriplesForReading with ng == nil")
	}
	for i := uint(0); i < nodeCount; i++ {
		fragment, err := readString(r)
		if err != nil {
			panic(err)
		}

		allocatedTriplexCnt, err := readUint(r)
		if err != nil {
			panic(err)
		}
		GlobalLogger.Debug("gatherTriplesForReading: ", zap.String("fragment", fragment), zap.Uint("allocatedTriplexCnt", allocatedTriplexCnt))

		sortedNodes[i], tripleCount = ng.createAllocatedNode(
			fragment,
			iriNodeType,
			tripleCount,
			int(allocatedTriplexCnt),
		)
	}
	return tripleCount, nil
}

// reads the content section of a binary SST file;
// loop over gc.sortedNodes
func readContent(r *bufio.Reader, gc *graphReadingContext) error {
	for _, s := range gc.sortedNodes {
		nodeTripleCount, err := readUint(r)
		if err != nil {
			return err
		}
		for i := uint(0); i < nodeTripleCount; i++ {
			p, err := readIBNode(r, gc)
			if err != nil {
				return err
			}
			o, err := readIBNode(r, gc)
			if err != nil {
				return err
			}
			s.appendUniqueNodeTriple(p, o)
		}
		literalTripleCount, err := readUint(r)
		if err != nil {
			return err
		}
		for i := uint(0); i < literalTripleCount; i++ {
			p, err := readIBNode(r, gc)
			if err != nil {
				return err
			}
			err = readLiteral(-1, r, true, func(o Literal) {
				s.appendUniqueLiteralTriple(p, o)
			}, func(l LiteralCollection) {
				s.appendUniqueLiteralCollectionTriple(p, l)
			})
			if err != nil {
				return err
			}
		}
		if s != nil {
			s.extractCollectionMembers()
		}
	}
	return nil
}

func readIBNode(r *bufio.Reader, gc *graphReadingContext) (IBNode, error) {
	index, err := readUint(r)
	if err != nil {
		return nil, err
	}
	offset := sort.SearchInts(gc.graphOffsets, int(index))
	if offset >= len(gc.graphOffsets) || gc.graphOffsets[offset] != int(index) {
		offset--
	}
	for offset != len(gc.graphOffsets)-1 && gc.graphOffsets[offset] == gc.graphOffsets[offset+1] {
		offset++
	}
	var returnIB IBNode
	switch {
	case offset == 0:
		returnIB = gc.sortedNodes[index]
	case offset <= len(gc.sortedImportedGraphs):
		returnIB = gc.sortedImportedGraphs[offset-1].sortedNodes[index-uint(gc.graphOffsets[offset])]
	default:
		returnIB = gc.sortedReferencedGraphs[offset-len(gc.sortedImportedGraphs)-1].sortedNodes[index-uint(gc.graphOffsets[offset])]
	}
	GlobalLogger.Debug("readIBNode",
		zap.Uint("index", index),
		zap.Int("offset", offset),
		zap.String("returnIB fragment", returnIB.iriOrID()))
	return returnIB, nil
}

func readLiteral(readerSource int, r *bufio.Reader, allowCollection bool, literalCallback func(Literal), literalCollectionCallback func(l LiteralCollection)) error {
	lc, cc := literalCallback, literalCollectionCallback
	if lc == nil {
		lc = func(Literal) {}
	}
	if cc == nil {
		cc = func(LiteralCollection) {}
	}
	kind, err := readUint(r)
	if readerSource == -1 {
		switch kind {
		case uint(writtenLiteralString):
			GlobalLogger.Debug("readUint from base", zap.String("literal type", "String"))
		case uint(writtenLiteralLangString):
			GlobalLogger.Debug("readUint from base", zap.String("literal type", "LangString"))
		case uint(writtenLiteralBoolean):
			GlobalLogger.Debug("readUint from base", zap.String("literal type", "LiteralBoolean"))
		case uint(writtenLiteralInteger):
			GlobalLogger.Debug("readUint from base", zap.String("literal type", "LiteralInteger"))
		case uint(writtenLiteralDouble):
			GlobalLogger.Debug("readUint from base", zap.String("literal type", "LiteralDouble"))
		case uint(writtenLiteralFloat):
			GlobalLogger.Debug("readUint from base", zap.String("literal type", "LiteralFloat"))
		case uint(writtenLiteralByte):
			GlobalLogger.Debug("readUint from base", zap.String("literal type", "LiteralByte"))
		case uint(writtenLiteralShort):
			GlobalLogger.Debug("readUint from base", zap.String("literal type", "LiteralShort"))
		case uint(writtenLiteralInt):
			GlobalLogger.Debug("readUint from base", zap.String("literal type", "LiteralInt"))
		case uint(writtenLiteralLong):
			GlobalLogger.Debug("readUint from base", zap.String("literal type", "LiteralLong"))
		case uint(writtenLiteralUnsignedByte):
			GlobalLogger.Debug("readUint from base", zap.String("literal type", "LiteralUnsignedByte"))
		case uint(writtenLiteralUnsignedShort):
			GlobalLogger.Debug("readUint from base", zap.String("literal type", "LiteralUnsignedShort"))
		case uint(writtenLiteralUnsignedInt):
			GlobalLogger.Debug("readUint from base", zap.String("literal type", "LiteralUnsignedInt"))
		case uint(writtenLiteralUnsignedLong):
			GlobalLogger.Debug("readUint from base", zap.String("literal type", "LiteralUnsignedLong"))
		case uint(writtenLiteralDateTime):
			GlobalLogger.Debug("readUint from base", zap.String("literal type", "DateTime"))
		case uint(writtenLiteralDateTimeStamp):
			GlobalLogger.Debug("readUint from base", zap.String("literal type", "DateTimeStamp"))
		case uint(writtenLiteralCollection):
			GlobalLogger.Debug("readUint from base", zap.String("literal type", "LiteralCollection"))
		default:
			panic("not recognized type")
		}
	} else {
		switch kind {
		case uint(writtenLiteralString):
			GlobalLogger.Debug("readUint from diff", zap.String("literal type", "String"))
		case uint(writtenLiteralLangString):
			GlobalLogger.Debug("readUint from diff", zap.String("literal type", "LangString"))
		case uint(writtenLiteralBoolean):
			GlobalLogger.Debug("readUint from diff", zap.String("literal type", "LiteralBoolean"))
		case uint(writtenLiteralInteger):
			GlobalLogger.Debug("readUint from diff", zap.String("literal type", "LiteralInteger"))
		case uint(writtenLiteralByte):
			GlobalLogger.Debug("readUint from diff", zap.String("literal type", "LiteralByte"))
		case uint(writtenLiteralShort):
			GlobalLogger.Debug("readUint from diff", zap.String("literal type", "LiteralShort"))
		case uint(writtenLiteralInt):
			GlobalLogger.Debug("readUint from diff", zap.String("literal type", "LiteralInt"))
		case uint(writtenLiteralLong):
			GlobalLogger.Debug("readUint from diff", zap.String("literal type", "LiteralLong"))
		case uint(writtenLiteralUnsignedByte):
			GlobalLogger.Debug("readUint from diff", zap.String("literal type", "LiteralUnsignedByte"))
		case uint(writtenLiteralUnsignedShort):
			GlobalLogger.Debug("readUint from diff", zap.String("literal type", "LiteralUnsignedShort"))
		case uint(writtenLiteralUnsignedInt):
			GlobalLogger.Debug("readUint from diff", zap.String("literal type", "LiteralUnsignedInt"))
		case uint(writtenLiteralUnsignedLong):
			GlobalLogger.Debug("readUint from diff", zap.String("literal type", "LiteralUnsignedLong"))
		case uint(writtenLiteralDouble):
			GlobalLogger.Debug("readUint from diff", zap.String("literal type", "LiteralDouble"))
		case uint(writtenLiteralFloat):
			GlobalLogger.Debug("readUint from diff", zap.String("literal type", "LiteralFloat"))
		case uint(writtenLiteralDateTime):
			GlobalLogger.Debug("readUint from diff", zap.String("literal type", "DateTime"))
		case uint(writtenLiteralDateTimeStamp):
			GlobalLogger.Debug("readUint from diff", zap.String("literal type", "DateTimeStamp"))
		case uint(writtenLiteralCollection):
			GlobalLogger.Debug("readUint from diff", zap.String("literal type", "LiteralCollection"))
		default:
			panic("not recognized type")
		}
	}
	if err != nil {
		return err
	}
	switch k := writtenLiteralKind(kind); k {
	case writtenLiteralString:
		value, err := readString(r)
		if err != nil {
			return err
		}
		if readerSource == -1 {
			GlobalLogger.Debug("readUint from base", zap.String("value", value))
		} else {
			GlobalLogger.Debug("readUint from diff", zap.String("value", value))
		}
		lc(String(value))

	case writtenLiteralByte:
		value, err := readByte(r)
		if err != nil {
			return err
		}
		if readerSource == -1 {
			GlobalLogger.Debug("readUint from base", zap.Int8("value", value))
		} else {
			GlobalLogger.Debug("readUint from diff", zap.Int8("value", value))
		}
		lc(Byte(value))
	case writtenLiteralShort:
		value, err := readShort(r)
		if err != nil {
			return err
		}
		if readerSource == -1 {
			GlobalLogger.Debug("readUint from base", zap.Int16("value", value))
		} else {
			GlobalLogger.Debug("readUint from diff", zap.Int16("value", value))
		}
		lc(Short(value))
	case writtenLiteralInt:
		value, err := readInt(r)
		if err != nil {
			return err
		}
		if readerSource == -1 {
			GlobalLogger.Debug("readUint from base", zap.Int32("value", value))
		} else {
			GlobalLogger.Debug("readUint from diff", zap.Int32("value", value))
		}
		lc(Int(value))
	case writtenLiteralLong:
		value, err := readLong(r)
		if err != nil {
			return err
		}
		if readerSource == -1 {
			GlobalLogger.Debug("readUint from base", zap.Int64("value", value))
		} else {
			GlobalLogger.Debug("readUint from diff", zap.Int64("value", value))
		}
		lc(Long(value))
	case writtenLiteralUnsignedByte:
		value, err := readUnsignedByte(r)
		if err != nil {
			return err
		}
		if readerSource == -1 {
			GlobalLogger.Debug("readUint from base", zap.Uint8("value", value))
		} else {
			GlobalLogger.Debug("readUint from diff", zap.Uint8("value", value))
		}
		lc(UnsignedByte(value))
	case writtenLiteralUnsignedShort:
		value, err := readUnsignedShort(r)
		if err != nil {
			return err
		}
		if readerSource == -1 {
			GlobalLogger.Debug("readUint from base", zap.Uint16("value", value))
		} else {
			GlobalLogger.Debug("readUint from diff", zap.Uint16("value", value))
		}
		lc(UnsignedShort(value))
	case writtenLiteralUnsignedInt:
		value, err := readUnsignedInt(r)
		if err != nil {
			return err
		}
		if readerSource == -1 {
			GlobalLogger.Debug("readUint from base", zap.Uint32("value", value))
		} else {
			GlobalLogger.Debug("readUint from diff", zap.Uint32("value", value))
		}
		lc(UnsignedInt(value))
	case writtenLiteralUnsignedLong:
		value, err := readUnsignedLong(r)
		if err != nil {
			return err
		}
		if readerSource == -1 {
			GlobalLogger.Debug("readUint from base", zap.Uint64("value", value))
		} else {
			GlobalLogger.Debug("readUint from diff", zap.Uint64("value", value))
		}
		lc(UnsignedLong(value))
	case writtenLiteralLangString:
		value, err := readString(r)
		if err != nil {
			return err
		}
		lang, err := readString(r)
		if err != nil {
			return err
		}
		if readerSource == -1 {
			GlobalLogger.Debug("readUint from base", zap.String("langTag", lang), zap.String("value", value))
		} else {
			GlobalLogger.Debug("readUint from diff", zap.String("langTag", lang), zap.String("value", value))
		}
		lc(LangString{Val: value, LangTag: lang})
	case writtenLiteralInteger:
		value, err := readInt64(r)
		if err != nil {
			return err
		}
		if readerSource == -1 {
			GlobalLogger.Debug("readUint from base", zap.Int64("value", value))
		} else {
			GlobalLogger.Debug("readUint from diff", zap.Int64("value", value))
		}
		lc(Integer(value))
	case writtenLiteralDouble:
		value, err := readFloat64(r)
		if err != nil {
			return err
		}
		if readerSource == -1 {
			GlobalLogger.Debug("readUint from base", zap.Float64("value", value))
		} else {
			GlobalLogger.Debug("readUint from diff", zap.Float64("value", value))
		}
		lc(Double(value))
	case writtenLiteralFloat:
		value, err := readFloat32(r)
		if err != nil {
			return err
		}
		if readerSource == -1 {
			GlobalLogger.Debug("readUint from base", zap.Float32("value", value))
		} else {
			GlobalLogger.Debug("readUint from diff", zap.Float32("value", value))
		}
		lc(Float(value))
	case writtenLiteralBoolean:
		value, err := readUint(r)
		if err != nil {
			return err
		}
		var b bool
		if value != 0 {
			b = true
		}
		if readerSource == -1 {
			GlobalLogger.Debug("readUint from base", zap.Bool("value", b))
		} else {
			GlobalLogger.Debug("readUint from diff", zap.Bool("value", b))
		}
		lc(Boolean(b))
	case writtenLiteralDateTime:
		value, err := readString(r)
		if err != nil {
			return err
		}
		if readerSource == -1 {
			GlobalLogger.Debug("readUint from base", zap.String("value", value))
		} else {
			GlobalLogger.Debug("readUint from diff", zap.String("value", value))
		}
		lc(TypedString{Val: value, Type: &literalTypeDateTime.ibNode})
	case writtenLiteralDateTimeStamp:
		value, err := readString(r)
		if err != nil {
			return err
		}
		if readerSource == -1 {
			GlobalLogger.Debug("readUint from base", zap.String("value", value))
		} else {
			GlobalLogger.Debug("readUint from diff", zap.String("value", value))
		}
		lc(TypedString{Val: value, Type: &literalTypeDateTimeStamp.ibNode})
	case writtenLiteralCollection:
		if allowCollection {
			memberCount, err := readUint(r)
			if err != nil {
				return err
			}
			if readerSource == -1 {
				GlobalLogger.Debug("readUint from base", zap.Uint("memberCount", memberCount))
			} else {
				GlobalLogger.Debug("readUint from diff", zap.Uint("memberCount", memberCount))
			}
			members := make([]Literal, memberCount)
			for i := uint(0); i < memberCount; i++ {
				err := readLiteral(readerSource, r, false, func(m Literal) { members[i] = m }, nil)
				if err != nil {
					return err
				}
			}
			list := NewLiteralCollection(members[0], members[1:]...)
			cc(list)
			break
		}
		fallthrough
	default:
		panic(fmt.Sprintf("unsupported literal kind %v", k))
	}
	return nil
}

// readString reads a string from a binary SST file
func readString(r *bufio.Reader) (string, error) {
	strLen, err := readUint(r)
	if err != nil {
		panic(err)
	}
	strBytes := make([]byte, int(strLen))
	_, err = io.ReadFull(r, strBytes)
	if err == io.EOF {
		// io.EOF means read 0 byte
		// this is normal when handling NG Node
		// because NG Node fragment is an empty string
		err = nil
	} else if err != nil {
		panic(err)
	}
	// fmt.Println("readString: ", string(strBytes))

	return *(*string)(unsafe.Pointer(&strBytes)), nil
}

func readInt64(r io.ByteReader) (int64, error) {
	return binary.ReadVarint(r)
}

func readByte(r io.Reader) (int8, error) {
	var buf [1]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	return int8(buf[0]), nil
}

func readShort(r io.Reader) (int16, error) {
	var buf [2]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	return int16(binary.BigEndian.Uint16(buf[:])), nil
}

func readInt(r io.Reader) (int32, error) {
	var buf [4]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	return int32(binary.BigEndian.Uint32(buf[:])), nil
}

func readLong(r io.Reader) (int64, error) {
	var buf [8]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	return int64(binary.BigEndian.Uint64(buf[:])), nil
}

func readUnsignedByte(r io.Reader) (uint8, error) {
	var buf [1]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	return buf[0], nil
}

func readUnsignedShort(r io.Reader) (uint16, error) {
	var buf [2]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(buf[:]), nil
}

func readUnsignedInt(r io.Reader) (uint32, error) {
	var buf [4]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(buf[:]), nil
}

func readUnsignedLong(r io.Reader) (uint64, error) {
	var buf [8]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(buf[:]), nil
}

func readFloat64(r io.Reader) (float64, error) {
	// New format: read 8 bytes as big-endian IEEE 754
	var buf [8]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	u := binary.BigEndian.Uint64(buf[:])
	return math.Float64frombits(u), nil
}

func readFloat64Old(r io.ByteReader) (float64, error) {
	i, err := binary.ReadUvarint(r)
	if err != nil {
		return 0, err
	}
	return bitsToFloat64(i), nil
}

// copied from encoding/gob/float64FromBits
func bitsToFloat64(u uint64) float64 {
	v := bits.ReverseBytes64(u)
	return math.Float64frombits(v)
}

func readFloat32(r io.Reader) (float32, error) {
	// Read 4 bytes as big-endian IEEE 754
	var buf [4]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	u := binary.BigEndian.Uint32(buf[:])
	return math.Float32frombits(u), nil
}

// readUint reads a variable length integer value from a binary SST file
func readUint(r io.ByteReader) (uint, error) {
	i, err := binary.ReadUvarint(r)
	// fmt.Println("readUint: ", i, err)
	return uint(i), err
}

// ReadStageFromSstFiles read sst binary SST files in a provided file system directory into a new Stage and returns it.
func ReadStageFromSstFiles(stageDir fs.FS, mode TriplexMode) (Stage, error) {
	stage := OpenStage(DefaultTriplexMode)
	err := fs.WalkDir(stageDir, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			// do something if d is a directory
		} else {
			dgFile, err := stageDir.Open(path)
			if err != nil {
				panic(err)
			}
			defer dgFile.Close()

			ng, err := SstRead(bufio.NewReader(dgFile), DefaultTriplexMode)
			if err != nil {
				panic(err)
			}
			// merge temp info into main stage
			_, err = stage.MoveAndMerge(context.TODO(), ng.Stage())
			if err != nil {
				panic(err)
			}
		}
		return nil
	})

	return stage, err
}

// SstReadOld reads an SST binary file using the old float64 format (Uvarint encoded).
// This is useful for reading legacy SST files that were written before the format change.
// It is identical to SstRead except it uses readFloat64Old instead of readFloat64.
func SstReadOld(r *bufio.Reader, mode TriplexMode) (NamedGraph, error) {
	st := OpenStage(mode)
	graphID, graphIRI, err := sstReadGraphID(r)
	if err != nil {
		return nil, err
	}
	GlobalLogger.Debug("SstReadOld", zap.String("graphIRI", graphIRI))

	var ng NamedGraph
	if graphIRI == "" {
		ng = st.CreateNamedGraph(IRI(graphID.URN()))
	} else {
		ng = st.CreateNamedGraph(IRI(graphIRI))
	}

	var gc graphReadingContext
	err = readHeader(r, st, ng, &gc)
	if err != nil {
		panic(err)
	}
	// reads the dictionary section of the binary SST file
	err = readDictionary(r, ng, &gc)
	if err != nil {
		panic(err)
	}
	// reads the content section of a binary SST file (using old float64 format)
	err = readContentOld(r, &gc)
	if err != nil {
		panic(err)
	}
	_, err = r.ReadByte()
	if !errors.Is(err, io.EOF) {
		return ng, ErrEOFExpected
	}
	return ng, nil
}

// readContentOld is identical to readContent but uses readLiteralOld instead of readLiteral.
// This is used for reading legacy SST files that used the old float64 encoding.
func readContentOld(r *bufio.Reader, gc *graphReadingContext) error {
	for _, s := range gc.sortedNodes {
		nodeTripleCount, err := readUint(r)
		if err != nil {
			return err
		}
		for i := uint(0); i < nodeTripleCount; i++ {
			p, err := readIBNode(r, gc)
			if err != nil {
				return err
			}
			o, err := readIBNode(r, gc)
			if err != nil {
				return err
			}
			s.appendUniqueNodeTriple(p, o)
		}
		literalTripleCount, err := readUint(r)
		if err != nil {
			return err
		}
		for i := uint(0); i < literalTripleCount; i++ {
			p, err := readIBNode(r, gc)
			if err != nil {
				return err
			}
			err = readLiteralOld(-1, r, true, func(o Literal) {
				s.appendUniqueLiteralTriple(p, o)
			}, func(l LiteralCollection) {
				s.appendUniqueLiteralCollectionTriple(p, l)
			})
			if err != nil {
				return err
			}
		}
		if s != nil {
			s.extractCollectionMembers()
		}
	}
	return nil
}

// readLiteralOld is identical to readLiteral but uses readFloat64Old instead of readFloat64.
// This is used for reading legacy SST files that used the old float64 encoding.
func readLiteralOld(readerSource int, r *bufio.Reader, allowCollection bool, literalCallback func(Literal), literalCollectionCallback func(l LiteralCollection)) error {
	lc, cc := literalCallback, literalCollectionCallback
	if lc == nil {
		lc = func(Literal) {}
	}
	if cc == nil {
		cc = func(LiteralCollection) {}
	}
	kind, err := readUint(r)
	if readerSource == -1 {
		switch kind {
		case uint(writtenLiteralString):
			GlobalLogger.Debug("readUint from base", zap.String("literal type", "String"))
		case uint(writtenLiteralLangString):
			GlobalLogger.Debug("readUint from base", zap.String("literal type", "LangString"))
		case uint(writtenLiteralBoolean):
			GlobalLogger.Debug("readUint from base", zap.String("literal type", "LiteralBoolean"))
		case uint(writtenLiteralInteger):
			GlobalLogger.Debug("readUint from base", zap.String("literal type", "LiteralInteger"))
		case uint(writtenLiteralDouble):
			GlobalLogger.Debug("readUint from base", zap.String("literal type", "LiteralDouble"))
		case uint(writtenLiteralShort):
			GlobalLogger.Debug("readUint from base", zap.String("literal type", "LiteralShort"))
		case uint(writtenLiteralDateTime):
			GlobalLogger.Debug("readUint from base", zap.String("literal type", "DateTime"))
		case uint(writtenLiteralDateTimeStamp):
			GlobalLogger.Debug("readUint from base", zap.String("literal type", "DateTimeStamp"))
		case uint(writtenLiteralCollection):
			GlobalLogger.Debug("readUint from base", zap.String("literal type", "LiteralCollection"))
		default:
			panic("not recognized type")
		}
	} else {
		switch kind {
		case uint(writtenLiteralString):
			GlobalLogger.Debug("readUint from diff", zap.String("literal type", "String"))
		case uint(writtenLiteralLangString):
			GlobalLogger.Debug("readUint from diff", zap.String("literal type", "LangString"))
		case uint(writtenLiteralBoolean):
			GlobalLogger.Debug("readUint from diff", zap.String("literal type", "LiteralBoolean"))
		case uint(writtenLiteralInteger):
			GlobalLogger.Debug("readUint from diff", zap.String("literal type", "LiteralInteger"))
		case uint(writtenLiteralShort):
			GlobalLogger.Debug("readUint from diff", zap.String("literal type", "LiteralShort"))
		case uint(writtenLiteralDouble):
			GlobalLogger.Debug("readUint from diff", zap.String("literal type", "LiteralDouble"))
		case uint(writtenLiteralDateTime):
			GlobalLogger.Debug("readUint from diff", zap.String("literal type", "DateTime"))
		case uint(writtenLiteralDateTimeStamp):
			GlobalLogger.Debug("readUint from diff", zap.String("literal type", "DateTimeStamp"))
		case uint(writtenLiteralCollection):
			GlobalLogger.Debug("readUint from diff", zap.String("literal type", "LiteralCollection"))
		default:
			panic("not recognized type")
		}
	}
	if err != nil {
		return err
	}
	switch k := writtenLiteralKind(kind); k {
	case writtenLiteralString:
		value, err := readString(r)
		if err != nil {
			return err
		}
		if readerSource == -1 {
			GlobalLogger.Debug("readUint from base", zap.String("value", value))
		} else {
			GlobalLogger.Debug("readUint from diff", zap.String("value", value))
		}
		lc(String(value))

	case writtenLiteralByte:
		value, err := readByte(r)
		if err != nil {
			return err
		}
		if readerSource == -1 {
			GlobalLogger.Debug("readUint from base", zap.Int8("value", value))
		} else {
			GlobalLogger.Debug("readUint from diff", zap.Int8("value", value))
		}
		lc(Byte(value))
	case writtenLiteralShort:
		value, err := readShort(r)
		if err != nil {
			return err
		}
		if readerSource == -1 {
			GlobalLogger.Debug("readUint from base", zap.Int16("value", value))
		} else {
			GlobalLogger.Debug("readUint from diff", zap.Int16("value", value))
		}
		lc(Short(value))
	case writtenLiteralInt:
		value, err := readInt(r)
		if err != nil {
			return err
		}
		if readerSource == -1 {
			GlobalLogger.Debug("readUint from base", zap.Int32("value", value))
		} else {
			GlobalLogger.Debug("readUint from diff", zap.Int32("value", value))
		}
		lc(Int(value))
	case writtenLiteralLong:
		value, err := readLong(r)
		if err != nil {
			return err
		}
		if readerSource == -1 {
			GlobalLogger.Debug("readUint from base", zap.Int64("value", value))
		} else {
			GlobalLogger.Debug("readUint from diff", zap.Int64("value", value))
		}
		lc(Long(value))
	case writtenLiteralUnsignedByte:
		value, err := readUnsignedByte(r)
		if err != nil {
			return err
		}
		if readerSource == -1 {
			GlobalLogger.Debug("readUint from base", zap.Uint8("value", value))
		} else {
			GlobalLogger.Debug("readUint from diff", zap.Uint8("value", value))
		}
		lc(UnsignedByte(value))
	case writtenLiteralUnsignedShort:
		value, err := readUnsignedShort(r)
		if err != nil {
			return err
		}
		if readerSource == -1 {
			GlobalLogger.Debug("readUint from base", zap.Uint16("value", value))
		} else {
			GlobalLogger.Debug("readUint from diff", zap.Uint16("value", value))
		}
		lc(UnsignedShort(value))
	case writtenLiteralUnsignedInt:
		value, err := readUnsignedInt(r)
		if err != nil {
			return err
		}
		if readerSource == -1 {
			GlobalLogger.Debug("readUint from base", zap.Uint32("value", value))
		} else {
			GlobalLogger.Debug("readUint from diff", zap.Uint32("value", value))
		}
		lc(UnsignedInt(value))
	case writtenLiteralUnsignedLong:
		value, err := readUnsignedLong(r)
		if err != nil {
			return err
		}
		if readerSource == -1 {
			GlobalLogger.Debug("readUint from base", zap.Uint64("value", value))
		} else {
			GlobalLogger.Debug("readUint from diff", zap.Uint64("value", value))
		}
		lc(UnsignedLong(value))
	case writtenLiteralLangString:
		value, err := readString(r)
		if err != nil {
			return err
		}
		lang, err := readString(r)
		if err != nil {
			return err
		}
		if readerSource == -1 {
			GlobalLogger.Debug("readUint from base", zap.String("langTag", lang), zap.String("value", value))
		} else {
			GlobalLogger.Debug("readUint from diff", zap.String("langTag", lang), zap.String("value", value))
		}
		lc(LangString{Val: value, LangTag: lang})
	case writtenLiteralInteger:
		value, err := readInt64(r)
		if err != nil {
			return err
		}
		if readerSource == -1 {
			GlobalLogger.Debug("readUint from base", zap.Int64("value", value))
		} else {
			GlobalLogger.Debug("readUint from diff", zap.Int64("value", value))
		}
		lc(Integer(value))
	case writtenLiteralDouble:
		// This is the key difference: use readFloat64Old instead of readFloat64
		value, err := readFloat64Old(r)
		if err != nil {
			return err
		}
		if readerSource == -1 {
			GlobalLogger.Debug("readUint from base", zap.Float64("value", value))
		} else {
			GlobalLogger.Debug("readUint from diff", zap.Float64("value", value))
		}
		lc(Double(value))
	case writtenLiteralBoolean:
		value, err := readUint(r)
		if err != nil {
			return err
		}
		var b bool
		if value != 0 {
			b = true
		}
		if readerSource == -1 {
			GlobalLogger.Debug("readUint from base", zap.Bool("value", b))
		} else {
			GlobalLogger.Debug("readUint from diff", zap.Bool("value", b))
		}
		lc(Boolean(b))
	case writtenLiteralDateTime:
		value, err := readString(r)
		if err != nil {
			return err
		}
		if readerSource == -1 {
			GlobalLogger.Debug("readUint from base", zap.String("value", value))
		} else {
			GlobalLogger.Debug("readUint from diff", zap.String("value", value))
		}
		lc(TypedString{Val: value, Type: &literalTypeDateTime.ibNode})
	case writtenLiteralDateTimeStamp:
		value, err := readString(r)
		if err != nil {
			return err
		}
		if readerSource == -1 {
			GlobalLogger.Debug("readUint from base", zap.String("value", value))
		} else {
			GlobalLogger.Debug("readUint from diff", zap.String("value", value))
		}
		lc(TypedString{Val: value, Type: &literalTypeDateTimeStamp.ibNode})
	case writtenLiteralCollection:
		if allowCollection {
			memberCount, err := readUint(r)
			if err != nil {
				return err
			}
			if readerSource == -1 {
				GlobalLogger.Debug("readUint from base", zap.Uint("memberCount", memberCount))
			} else {
				GlobalLogger.Debug("readUint from diff", zap.Uint("memberCount", memberCount))
			}
			members := make([]Literal, memberCount)
			for i := uint(0); i < memberCount; i++ {
				err := readLiteralOld(readerSource, r, false, func(m Literal) { members[i] = m }, nil)
				if err != nil {
					return err
				}
			}
			list := NewLiteralCollection(members[0], members[1:]...)
			cc(list)
			break
		}
		fallthrough
	default:
		panic(fmt.Sprintf("unsupported literal kind %v", k))
	}
	return nil
}
