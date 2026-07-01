// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package svgtosst

import (
	"encoding/xml"
	"fmt"
	"io"

	"github.com/semanticstep/sst-core/sst"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	_ "github.com/semanticstep/sst-core/vocabularies/lci"
)

func decodeSVG(r io.Reader) (svg, error) {
	var doc svg
	dec := xml.NewDecoder(r)
	if err := dec.Decode(&doc); err != nil {
		return svg{}, fmt.Errorf("error unmarshalling SVG: %v", err)
	}
	return doc, nil
}

// ConvertSvgToGraph decodes SVG from r and returns a new in-memory SST named graph.
// The graph lives on a fresh stage. If graphIRI is empty, a random urn:uuid IRI is assigned.
func ConvertSvgToGraph(r io.Reader, graphIRI sst.IRI) (sst.NamedGraph, error) {
	svg, err := decodeSVG(r)
	if err != nil {
		return nil, err
	}

	iri := graphIRI
	if iri != "" {
		iri, err = sst.NewIRI(string(graphIRI))
		if err != nil {
			return nil, fmt.Errorf("invalid graph IRI: %w", err)
		}
	}

	stage := sst.OpenStage(sst.DefaultTriplexMode)
	graph := stage.CreateNamedGraph(iri)

	var itemUUIDs []sst.Term

	createNodesForGroups(graph, svg.Groups, &itemUUIDs, "", shapeStyle{})
	createNodesForTexts(graph, svg.Texts, &itemUUIDs, "", shapeStyle{})
	createNodesForRects(graph, svg.Rects, &itemUUIDs, "", shapeStyle{})
	createNodesForCircles(graph, svg.Circles, &itemUUIDs, "", shapeStyle{})
	createNodesForEllipses(graph, svg.Ellipses, &itemUUIDs, "", shapeStyle{})
	createNodesForPolygons(graph, svg.Polygons, &itemUUIDs, "", shapeStyle{})
	createNodesForPolylines(graph, svg.Polylines, &itemUUIDs, "", shapeStyle{})
	createNodesForLines(graph, svg.Lines, &itemUUIDs, "", shapeStyle{})
	createNodesForPaths(graph, svg.Paths, &itemUUIDs, "", shapeStyle{})

	contextNode, err := createGeometricRepresentationContext(graph)
	if err != nil {
		return nil, fmt.Errorf("error creating geometric representation context: %v", err)
	}

	createSymbolRepresentation(graph, contextNode, itemUUIDs, svg.Title, svg.Desc)
	return graph, nil
}
