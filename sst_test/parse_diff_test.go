// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	diffEntrySame = iota
	diffEntryRemoved
	diffEntryAdded
	diffEntryTripleModified
)

const (
	diffIdenticalOrModifiedOffset = 1
	diffModifiedFlag              = 0b01
	diffSameRemovedOrAddedOffset  = 2
	diffRemovedFlag               = 0b01
	diffAddedFlag                 = 0b10
)

func diffReadUint(r *bufio.Reader) (uint64, error) {
	return binary.ReadUvarint(r)
}

func diffReadInt64(r *bufio.Reader) (int64, error) {
	return binary.ReadVarint(r)
}

func diffReadString(r *bufio.Reader) (string, error) {
	length, err := diffReadUint(r)
	if err != nil {
		return "", err
	}
	bytes := make([]byte, length)
	_, err = r.Read(bytes)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

type sstNode struct {
	Fragment string
	Triplex  uint64
}

type sstGraphNodes struct {
	IRI   string
	Nodes []sstNode
}

type sstGraphInfo struct {
	GraphIRI        string
	Imported        []string
	Referenced      []string
	IRINodes        []sstNode
	BlankNodes      []sstNode
	ImportedNodes   []sstGraphNodes
	ReferencedNodes []sstGraphNodes
}

// readSSTGraphInfo reads SST file header and dictionary.
func readSSTGraphInfo(path string) (*sstGraphInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := bufio.NewReader(f)

	// Skip magic "SST\n"
	magic := make([]byte, 4)
	_, err = r.Read(magic)
	if err != nil {
		return nil, err
	}

	info := &sstGraphInfo{}

	// Graph IRI
	info.GraphIRI, err = diffReadString(r)
	if err != nil {
		return nil, err
	}

	// Imported graphs
	impCount, err := diffReadUint(r)
	if err != nil {
		return nil, err
	}
	for i := uint64(0); i < impCount; i++ {
		iri, _ := diffReadString(r)
		info.Imported = append(info.Imported, iri)
	}

	// Referenced graphs (other, not imported)
	refCount, err := diffReadUint(r)
	if err != nil {
		return nil, err
	}
	for i := uint64(0); i < refCount; i++ {
		iri, _ := diffReadString(r)
		info.Referenced = append(info.Referenced, iri)
	}

	// Read dictionary
	iriCount, err := diffReadUint(r)
	if err != nil {
		return nil, err
	}
	blankCount, err := diffReadUint(r)
	if err != nil {
		return nil, err
	}

	for i := uint64(0); i < iriCount; i++ {
		fragment, _ := diffReadString(r)
		triplex, _ := diffReadUint(r)
		info.IRINodes = append(info.IRINodes, sstNode{Fragment: fragment, Triplex: triplex})
	}
	for i := uint64(0); i < blankCount; i++ {
		fragment, _ := diffReadString(r)
		triplex, _ := diffReadUint(r)
		info.BlankNodes = append(info.BlankNodes, sstNode{Fragment: fragment, Triplex: triplex})
	}

	// Read imported graph dictionaries
	for i := uint64(0); i < impCount; i++ {
		count, err := diffReadUint(r)
		if err != nil {
			return nil, err
		}
		nodes := make([]sstNode, count)
		for j := uint64(0); j < count; j++ {
			fragment, err := diffReadString(r)
			if err != nil {
				return nil, err
			}
			triplex, err := diffReadUint(r)
			if err != nil {
				return nil, err
			}
			nodes[j] = sstNode{Fragment: fragment, Triplex: triplex}
		}
		info.ImportedNodes = append(info.ImportedNodes, sstGraphNodes{IRI: info.Imported[i], Nodes: nodes})
	}

	// Read referenced graph dictionaries
	for i := uint64(0); i < refCount; i++ {
		count, err := diffReadUint(r)
		if err != nil {
			return nil, err
		}
		nodes := make([]sstNode, count)
		for j := uint64(0); j < count; j++ {
			fragment, err := diffReadString(r)
			if err != nil {
				return nil, err
			}
			triplex, err := diffReadUint(r)
			if err != nil {
				return nil, err
			}
			nodes[j] = sstNode{Fragment: fragment, Triplex: triplex}
		}
		info.ReferencedNodes = append(info.ReferencedNodes, sstGraphNodes{IRI: info.Referenced[i], Nodes: nodes})
	}

	return info, nil
}

// diffGraph represents one entry in the diff header.
type diffGraph struct {
	IRI       string
	FromIndex int
	ToIndex   int
	Span      uint64
}

// computeDiffGraphs compares two sorted graph lists and returns delta entries.
func computeDiffGraphs(from, to []string) []diffGraph {
	var graphs []diffGraph
	i, j := 0, 0
	hi, hj := false, false
	var fromURI, toURI string

	for i < len(from) && j < len(to) {
		if !hi {
			fromURI = from[i]
			hi = true
		}
		if !hj {
			toURI = to[j]
			hj = true
		}
		switch strings.Compare(fromURI, toURI) {
		case -1:
			graphs = append(graphs, diffGraph{IRI: fromURI, FromIndex: i, ToIndex: -1, Span: 1})
			i++
			hi = false
		case 0:
			if len(graphs) > 0 {
				last := &graphs[len(graphs)-1]
				if last.FromIndex >= 0 && last.ToIndex >= 0 {
					last.Span++
					i++
					j++
					hi, hj = false, false
					continue
				}
			}
			graphs = append(graphs, diffGraph{IRI: fromURI, FromIndex: i, ToIndex: j, Span: 1})
			i++
			j++
			hi, hj = false, false
		default:
			graphs = append(graphs, diffGraph{IRI: toURI, FromIndex: -1, ToIndex: j, Span: 1})
			j++
			hj = false
		}
	}
	for ; i < len(from); i++ {
		if !hi {
			fromURI = from[i]
		}
		graphs = append(graphs, diffGraph{IRI: fromURI, FromIndex: i, ToIndex: -1, Span: 1})
		hi = false
	}
	for ; j < len(to); j++ {
		if !hj {
			toURI = to[j]
		}
		graphs = append(graphs, diffGraph{IRI: toURI, FromIndex: -1, ToIndex: j, Span: 1})
		hj = false
	}
	return graphs
}

// computeDiffNodeCount calculates the number of combined nodes for a graph.
func computeDiffNodeCount(fromNodes, toNodes []sstNode) int {
	i, j := 0, 0
	count := 0
	var fromFrag, toFrag string
	hi, hj := false, false

	for i < len(fromNodes) && j < len(toNodes) {
		if !hi {
			fromFrag = fromNodes[i].Fragment
			hi = true
		}
		if !hj {
			toFrag = toNodes[j].Fragment
			hj = true
		}
		switch strings.Compare(fromFrag, toFrag) {
		case -1:
			count++
			i++
			hi = false
		case 0:
			count++
			i++
			j++
			hi, hj = false, false
		default:
			count++
			j++
			hj = false
		}
	}
	for ; i < len(fromNodes); i++ {
		count++
	}
	for ; j < len(toNodes); j++ {
		count++
	}
	return count
}

func diffParseHeader(t *testing.T, r *bufio.Reader, graphs []diffGraph, label string) {
	removed, err := diffReadUint(r)
	require.NoError(t, err)
	added, err := diffReadUint(r)
	require.NoError(t, err)

	fmt.Printf("\n=== %s Graphs ===\n", label)
	fmt.Printf("Removed: %d, Added: %d\n", removed, added)

	for i, g := range graphs {
		if g.FromIndex >= 0 && g.ToIndex >= 0 {
			fmt.Printf("  [%d] Same, span=%d\n", i+1, g.Span)
			flag, err := diffReadUint(r)
			require.NoError(t, err)
			span, err := diffReadUint(r)
			require.NoError(t, err)
			require.Equal(t, uint64(diffEntrySame), flag, "expected Same flag")
			require.Equal(t, g.Span, span, "span mismatch")
		} else if g.FromIndex >= 0 {
			fmt.Printf("  [%d] Removed, IRI=%q\n", i+1, g.IRI)
			flag, err := diffReadUint(r)
			require.NoError(t, err)
			iri, err := diffReadString(r)
			require.NoError(t, err)
			require.Equal(t, uint64(diffEntryRemoved), flag, "expected Removed flag")
			require.Equal(t, g.IRI, iri, "IRI mismatch")
		} else {
			fmt.Printf("  [%d] Added, IRI=%q\n", i+1, g.IRI)
			flag, err := diffReadUint(r)
			require.NoError(t, err)
			iri, err := diffReadString(r)
			require.NoError(t, err)
			require.Equal(t, uint64(diffEntryAdded), flag, "expected Added flag")
			require.Equal(t, g.IRI, iri, "IRI mismatch")
		}
	}
}

type diffParsedEntry struct {
	EntryType int
	Span      uint64
	Fragment  string
	Delta     int64
}

func diffParseNodeEntries(t *testing.T, r *bufio.Reader, count int, label string) []diffParsedEntry {
	var entries []diffParsedEntry
	if count == 0 {
		return entries
	}
	fmt.Printf("-- %s Node Entries (%d) --\n", label, count)
	for i := 0; i < count; i++ {
		entryType, err := diffReadUint(r)
		require.NoError(t, err)
		switch entryType {
		case diffEntrySame:
			span, err := diffReadUint(r)
			require.NoError(t, err)
			fmt.Printf("  [%d] Same, span=%d\n", i+1, span)
			entries = append(entries, diffParsedEntry{EntryType: diffEntrySame, Span: span})
		case diffEntryRemoved:
			fragment, err := diffReadString(r)
			require.NoError(t, err)
			delta, err := diffReadInt64(r)
			require.NoError(t, err)
			fmt.Printf("  [%d] Removed, fragment=%q, triplexDelta=%d\n", i+1, fragment, delta)
			entries = append(entries, diffParsedEntry{EntryType: diffEntryRemoved, Fragment: fragment, Delta: delta})
		case diffEntryAdded:
			fragment, err := diffReadString(r)
			require.NoError(t, err)
			delta, err := diffReadInt64(r)
			require.NoError(t, err)
			fmt.Printf("  [%d] Added, fragment=%q, triplexDelta=%d\n", i+1, fragment, delta)
			entries = append(entries, diffParsedEntry{EntryType: diffEntryAdded, Fragment: fragment, Delta: delta})
		case diffEntryTripleModified:
			delta, err := diffReadInt64(r)
			require.NoError(t, err)
			fmt.Printf("  [%d] TripleModified, triplexDelta=%d\n", i+1, delta)
			entries = append(entries, diffParsedEntry{EntryType: diffEntryTripleModified, Delta: delta})
		default:
			fmt.Printf("  [%d] Unknown entry type: %d\n", i+1, entryType)
		}
	}
	return entries
}

func diffParseCurrentGraphDictionary(t *testing.T, r *bufio.Reader, fromInfo, toInfo *sstGraphInfo) (iriEntries, blankEntries []diffParsedEntry) {
	fmt.Println("\n=== Current Graph Dictionary ===")
	iriDeleted, err := diffReadUint(r)
	require.NoError(t, err)
	iriAdded, err := diffReadUint(r)
	require.NoError(t, err)
	blankDeleted, err := diffReadUint(r)
	require.NoError(t, err)
	blankAdded, err := diffReadUint(r)
	require.NoError(t, err)
	fmt.Printf("IRI: deleted=%d, added=%d\n", iriDeleted, iriAdded)
	fmt.Printf("Blank: deleted=%d, added=%d\n", blankDeleted, blankAdded)

	iriCount := computeDiffNodeCount(fromInfo.IRINodes, toInfo.IRINodes)
	blankCount := computeDiffNodeCount(fromInfo.BlankNodes, toInfo.BlankNodes)
	iriEntries = diffParseNodeEntries(t, r, iriCount, "IRI")
	blankEntries = diffParseNodeEntries(t, r, blankCount, "Blank")
	return
}

func diffParseGraphDictionary(t *testing.T, r *bufio.Reader, graphType string, idx int, graphIRI string, fromNodes, toNodes []sstNode) []diffParsedEntry {
	fmt.Printf("\n=== %s Graph %d Dictionary ===\n", graphType, idx)
	fmt.Printf("IRI: %s\n", graphIRI)
	deleted, err := diffReadUint(r)
	require.NoError(t, err)
	added, err := diffReadUint(r)
	require.NoError(t, err)
	fmt.Printf("Deleted: %d, Added: %d\n", deleted, added)
	count := computeDiffNodeCount(fromNodes, toNodes)
	return diffParseNodeEntries(t, r, count, "")
}

type combinedNodeInfo struct {
	Source   string
	GraphIRI string
	Fragment string
	NodeType string
}

func diffParseContent(t *testing.T, r *bufio.Reader, combinedNodes []combinedNodeInfo) {
	fmt.Println("\n=== Content Section ===")

	val, err := diffReadUint(r)
	require.NoError(t, err)
	if val&diffModifiedFlag == 0 {
		span := val >> diffIdenticalOrModifiedOffset
		fmt.Printf("Identical span=%d\n", span)
	} else {
		r.UnreadByte()
	}

	// Non-literal header
	removedShifted, err := diffReadUint(r)
	require.NoError(t, err)
	removed := removedShifted >> diffIdenticalOrModifiedOffset
	added, err := diffReadUint(r)
	require.NoError(t, err)
	fmt.Printf("\nNode modified: removed=%d, added=%d\n", removed, added)

	// Non-literal triples
	for {
		tripleVal, err := diffReadUint(r)
		if err != nil {
			break
		}
		flag := tripleVal & 0b11
		if flag == 0 {
			count := tripleVal >> diffSameRemovedOrAddedOffset
			fmt.Printf("  Non-literal same count=%d\n", count)
		} else {
			r.UnreadByte()
			break
		}
	}

	// Literal header
	literalRemovedShifted, err := diffReadUint(r)
	require.NoError(t, err)
	literalRemoved := literalRemovedShifted >> diffIdenticalOrModifiedOffset
	literalAdded, err := diffReadUint(r)
	require.NoError(t, err)
	fmt.Printf("  Literal removed=%d, added=%d\n", literalRemoved, literalAdded)

	// Literal same count
	literalSameShifted, err := diffReadUint(r)
	require.NoError(t, err)
	literalSame := literalSameShifted >> diffSameRemovedOrAddedOffset
	fmt.Printf("  Literal same count=%d\n", literalSame)

	// Removed literal triples
	for i := uint64(0); i < literalRemoved; i++ {
		predShifted, err := diffReadUint(r)
		require.NoError(t, err)
		pred := predShifted >> diffSameRemovedOrAddedOffset
		kind, err := diffReadUint(r)
		require.NoError(t, err)

		var strVal string
		if kind == 0 { // String
			length, err := diffReadUint(r)
			require.NoError(t, err)
			bytes := make([]byte, length)
			_, err = r.Read(bytes)
			require.NoError(t, err)
			strVal = string(bytes)
		}

		nodeInfo := combinedNodes[pred]
		fmt.Printf("  Removed literal: pred=%d -> %s#%s (%s, %s), kind=%d, value=%q\n",
			pred, nodeInfo.GraphIRI, nodeInfo.Fragment, nodeInfo.Source, nodeInfo.NodeType, kind, strVal)
	}
}

// cleanGraphIRI strips prefix like ".0-" and keeps only urn:uuid:... part.
func buildCombinedNodes(entries []diffParsedEntry, source, graphIRI string, fromNodes []sstNode) []combinedNodeInfo {
	var result []combinedNodeInfo
	sourceIdx := 0
	for _, e := range entries {
		switch e.EntryType {
		case diffEntrySame:
			for s := uint64(0); s < e.Span && sourceIdx < len(fromNodes); s++ {
				result = append(result, combinedNodeInfo{Source: source, GraphIRI: graphIRI, Fragment: fromNodes[sourceIdx].Fragment, NodeType: "Same"})
				sourceIdx++
			}
		case diffEntryTripleModified:
			if sourceIdx < len(fromNodes) {
				result = append(result, combinedNodeInfo{Source: source, GraphIRI: graphIRI, Fragment: fromNodes[sourceIdx].Fragment, NodeType: "TripleModified"})
				sourceIdx++
			}
		case diffEntryRemoved:
			result = append(result, combinedNodeInfo{Source: source, GraphIRI: graphIRI, Fragment: e.Fragment, NodeType: "Removed"})
			sourceIdx++
		case diffEntryAdded:
			result = append(result, combinedNodeInfo{Source: source, GraphIRI: graphIRI, Fragment: e.Fragment, NodeType: "Added"})
		}
	}
	return result
}

func cleanGraphIRI(iri string) string {
	idx := strings.Index(iri, "urn:uuid:")
	if idx >= 0 {
		return iri[idx:]
	}
	return iri
}

// TestParseDiffBinaryFormat is a standalone inspection test that parses a binary diff SST file.
//
// It reads two source SST files (from/to) and the diff file, then:
//   1. Parses the diff header (imported/referenced graph differences).
//   2. Parses the diff dictionary for all graphs (current, imported, referenced).
//   3. Dynamically builds a combined node map by matching diff entries with
//      source SST node fragments (Same/TripleModified from from-nodes;
//      Removed/Added from diff entries themselves).
//   4. Parses the content section and maps pred indices to actual nodes using
//      the combined node map.
//
// This test is useful for debugging diff format issues: when a diff read panics,
// run this test to verify the combined node map and pred index mapping are correct.
func TestParseDiffBinaryFormat(t *testing.T) {
	repoDir := "./Test_RemoteRepositoryReadAndWrite"
	fromPath := repoDir + "/graph_1.sst"
	toPath := repoDir + "/graph_2.sst"
	diffPath := repoDir + "/diff_output.sst"

	require.FileExists(t, fromPath)
	require.FileExists(t, toPath)
	require.FileExists(t, diffPath)

	fromInfo, err := readSSTGraphInfo(fromPath)
	require.NoError(t, err)
	toInfo, err := readSSTGraphInfo(toPath)
	require.NoError(t, err)

	fromInfo.GraphIRI = cleanGraphIRI(fromInfo.GraphIRI)
	toInfo.GraphIRI = cleanGraphIRI(toInfo.GraphIRI)

	fmt.Printf("From graph: %s\n", fromInfo.GraphIRI)
	fmt.Printf("From imported: %v\n", fromInfo.Imported)
	fmt.Printf("From referenced: %v\n", fromInfo.Referenced)
	fmt.Printf("From IRI nodes: %d, Blank nodes: %d\n", len(fromInfo.IRINodes), len(fromInfo.BlankNodes))
	for i, n := range fromInfo.IRINodes {
		fmt.Printf("  IRI[%d]: fragment=%q, triplex=%d\n", i, n.Fragment, n.Triplex)
	}
	fmt.Printf("To graph: %s\n", toInfo.GraphIRI)
	fmt.Printf("To imported: %v\n", toInfo.Imported)
	fmt.Printf("To referenced: %v\n", toInfo.Referenced)
	fmt.Printf("To IRI nodes: %d, Blank nodes: %d\n", len(toInfo.IRINodes), len(toInfo.BlankNodes))
	for i, n := range toInfo.IRINodes {
		fmt.Printf("  IRI[%d]: fragment=%q, triplex=%d\n", i, n.Fragment, n.Triplex)
	}

	fmt.Printf("\nFrom imported nodes:\n")
	for i, g := range fromInfo.ImportedNodes {
		fmt.Printf("  Imported[%d]: %s, nodes=%d\n", i, g.IRI, len(g.Nodes))
		for j, n := range g.Nodes {
			fmt.Printf("    Node[%d]: fragment=%q, triplex=%d\n", j, n.Fragment, n.Triplex)
		}
	}
	fmt.Printf("From referenced nodes:\n")
	for i, g := range fromInfo.ReferencedNodes {
		fmt.Printf("  Referenced[%d]: %s, nodes=%d\n", i, g.IRI, len(g.Nodes))
		for j, n := range g.Nodes {
			fmt.Printf("    Node[%d]: fragment=%q, triplex=%d\n", j, n.Fragment, n.Triplex)
		}
	}

	fmt.Println("\n--- Diff File Start ---")

	importedDiff := computeDiffGraphs(fromInfo.Imported, toInfo.Imported)
	referencedDiff := computeDiffGraphs(fromInfo.Referenced, toInfo.Referenced)

	fmt.Printf("\nImported diff graphs: %d\n", len(importedDiff))
	for i, g := range importedDiff {
		fmt.Printf("  [%d] IRI=%q, span=%d\n", i+1, g.IRI, g.Span)
	}
	fmt.Printf("\nReferenced diff graphs: %d\n", len(referencedDiff))
	for i, g := range referencedDiff {
		fmt.Printf("  [%d] IRI=%q, span=%d\n", i+1, g.IRI, g.Span)
	}

	f, err := os.Open(diffPath)
	require.NoError(t, err)
	defer f.Close()
	r := bufio.NewReader(f)

	// Parse diff file
	diffParseHeader(t, r, importedDiff, "Imported")
	diffParseHeader(t, r, referencedDiff, "Referenced")
	iriEntries, blankEntries := diffParseCurrentGraphDictionary(t, r, fromInfo, toInfo)

	var combinedNodes []combinedNodeInfo
	combinedNodes = append(combinedNodes, buildCombinedNodes(iriEntries, "current", fromInfo.GraphIRI, fromInfo.IRINodes)...)
	combinedNodes = append(combinedNodes, buildCombinedNodes(blankEntries, "current", fromInfo.GraphIRI, fromInfo.BlankNodes)...)

	// Imported graph dictionaries
	for i, g := range importedDiff {
		for s := uint64(0); s < g.Span; s++ {
			var graphIRI string
			var fromNodes, toNodes []sstNode
			if g.FromIndex >= 0 && int(g.FromIndex)+int(s) < len(fromInfo.Imported) {
				graphIRI = fromInfo.Imported[g.FromIndex+int(s)]
				fromNodes = fromInfo.ImportedNodes[g.FromIndex+int(s)].Nodes
			}
			if g.ToIndex >= 0 && int(g.ToIndex)+int(s) < len(toInfo.Imported) {
				if graphIRI == "" {
					graphIRI = toInfo.Imported[g.ToIndex+int(s)]
				}
				toNodes = toInfo.ImportedNodes[g.ToIndex+int(s)].Nodes
			}
			if graphIRI == "" {
				graphIRI = g.IRI
			}
			entries := diffParseGraphDictionary(t, r, "Imported", i+1, graphIRI, fromNodes, toNodes)
			combinedNodes = append(combinedNodes, buildCombinedNodes(entries, "imported", graphIRI, fromNodes)...)
		}
	}

	// Referenced graph dictionaries
	for i, g := range referencedDiff {
		for s := uint64(0); s < g.Span; s++ {
			var graphIRI string
			var fromNodes, toNodes []sstNode
			if g.FromIndex >= 0 && int(g.FromIndex)+int(s) < len(fromInfo.Referenced) {
				graphIRI = fromInfo.Referenced[g.FromIndex+int(s)]
				fromNodes = fromInfo.ReferencedNodes[g.FromIndex+int(s)].Nodes
			}
			if g.ToIndex >= 0 && int(g.ToIndex)+int(s) < len(toInfo.Referenced) {
				if graphIRI == "" {
					graphIRI = toInfo.Referenced[g.ToIndex+int(s)]
				}
				toNodes = toInfo.ReferencedNodes[g.ToIndex+int(s)].Nodes
			}
			if graphIRI == "" {
				graphIRI = g.IRI
			}
			entries := diffParseGraphDictionary(t, r, "Referenced", i+1, graphIRI, fromNodes, toNodes)
			combinedNodes = append(combinedNodes, buildCombinedNodes(entries, "referenced", graphIRI, fromNodes)...)
		}
	}

	fmt.Println("\n=== Combined Node Map ===")
	for i, n := range combinedNodes {
		fmt.Printf("  [%d] %s#%s (%s, %s)\n", i, n.GraphIRI, n.Fragment, n.Source, n.NodeType)
	}

	// Content
	diffParseContent(t, r, combinedNodes)
}
