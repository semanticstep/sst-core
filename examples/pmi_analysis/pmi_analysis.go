// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

// PMIAnalysis reads an SST Turtle graph and prints its PartDesign PMI relationships.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/qau"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/semanticstep/sst-core/vocabularies/sso"

	_ "github.com/semanticstep/sst-core/vocabularies/dict"
)

type rdfEdge struct {
	subject   sst.IBNode
	predicate sst.IBNode
	object    sst.IBNode
}

type pmiReference struct {
	rdfEdge
	children    []pmiReference
	childAnchor sst.IBNode
}

type pmiTraversalRule struct {
	sourceKind sst.ElementInformer
	predicate  sst.ElementInformer
	targetKind sst.ElementInformer
	inverse    bool
	expand     bool
}

const (
	treeMiddle         = "├── "
	treeLast           = "└── "
	treeLine           = "│   "
	treeSpace          = "    "
	forwardArrow       = "→"
	inverseArrow       = "←"
	punnedArrow        = "⇒"
	inversePunnedArrow = "⇐"
)

// These rules are the PMI boundary; the walker does not enter unrelated CAD geometry.
var pmiTraversalRules = []pmiTraversalRule{
	{sourceKind: rep.ShapeRepresentation, predicate: rep.ShapeRepresentationRelationship, targetKind: rep.ShapeRepresentation, inverse: true, expand: true},
	{sourceKind: sso.ShapeElement, predicate: sso.ShapeElementRelationship, targetKind: sso.ShapeElement},
	{sourceKind: sso.ShapeElement, predicate: sso.DraughtingModelItemUsage, targetKind: rep.DraughtingCallout, expand: true},
	{sourceKind: sso.ShapeElement, predicate: sso.GeometricItemSpecificUsage, expand: true},
	{sourceKind: sso.ShapeElement, predicate: sso.TolerancedShapeAspect, targetKind: sso.GeometricTolerance, inverse: true, expand: true},
	{sourceKind: sso.ShapeElement, predicate: lci.PartOf, targetKind: sso.DimensionalSize, inverse: true, expand: true},
	{sourceKind: sso.GeometricTolerance, predicate: sso.GeometricToleranceMagnitude, expand: true},
	{sourceKind: sso.GeometricTolerance, predicate: sso.DatumSystemX, targetKind: sso.DatumSystem, expand: true},
	{sourceKind: sso.DimensionalSize, predicate: sso.DimensionalCharacteristicRepresentation, targetKind: rep.ShapeDimensionRepresentation, expand: true},
	{sourceKind: rep.ShapeDimensionRepresentation, predicate: rep.Item, expand: true},
	{sourceKind: rep.ModelGeometricView, predicate: rep.CharacterizedItemWithinRepresentation_rep, targetKind: rep.DraughtingModel},
	{sourceKind: sso.DatumSystem, predicate: sso.DatumSystemConstituents, targetKind: sso.DatumReferenceCompartment, expand: true},
	{sourceKind: sso.DatumReferenceCompartment, predicate: sso.BaseDatum, targetKind: sso.Datum},
	{sourceKind: rep.DraughtingCallout, predicate: rep.Item, targetKind: rep.DraughtingModel, inverse: true},
	{predicate: sso.ItemIdentifiedRepresentationUsage},
}

// Report flow: load each graph and walk from PartDesign to its ShapeElements.
// main reads the input graph and writes one PMI report per named graph.
func main() {
	inputPath := flag.String("in", "", "converted Turtle input file")
	outputPath := flag.String("out", "", "analysis report output file")
	flag.Parse()

	if *inputPath == "" && flag.NArg() > 0 {
		*inputPath = flag.Arg(0)
	}
	if *inputPath == "" {
		log.Fatal("missing input file; usage: go run ./examples/pmi_analysis -in converted.ttl")
	}

	file, err := os.Open(*inputPath)
	if err != nil {
		log.Panic(err)
	}
	defer file.Close()

	stage, err := sst.RdfRead(bufio.NewReader(file), sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
	if err != nil {
		log.Panic(err)
	}

	output := io.Writer(os.Stdout)
	if *outputPath != "" {
		outputFile, err := os.Create(*outputPath)
		if err != nil {
			log.Panic(err)
		}
		defer outputFile.Close()
		output = outputFile
	}

	graphs := stage.NamedGraphs()
	if len(graphs) == 0 {
		log.Fatal("input contains no named graph")
	}
	for index, graph := range graphs {
		if index > 0 {
			fmt.Fprintln(output)
		}
		fmt.Fprintln(output, "SST PMI Analysis (Draft)")
		fmt.Fprintf(output, "Analysed Named Graph: %s\n\n", graph.IRI())
		if err := printPartDesigns(output, graph); err != nil {
			log.Panic(err)
		}
	}
}

// printPartDesigns finds PartDesign instances and prints their PMI roots.
func printPartDesigns(output io.Writer, graph sst.NamedGraph) error {
	var partDesigns []sst.IBNode
	if err := graph.ForIRINodes(func(node sst.IBNode) error {
		if isKindOf(node, sso.PartDesign) {
			partDesigns = append(partDesigns, node)
		}
		return nil
	}); err != nil {
		return err
	}
	sort.Slice(partDesigns, func(i, j int) bool {
		return nodeSortKey(partDesigns[i]) < nodeSortKey(partDesigns[j])
	})

	fmt.Fprintln(output, "Reference legend:")
	fmt.Fprintf(output, "  %s forward reference\n", forwardArrow)
	fmt.Fprintf(output, "  %s inverse reference\n", inverseArrow)
	fmt.Fprintf(output, "  %s forward punned property reference\n", punnedArrow)
	fmt.Fprintf(output, "  %s inverse punned property reference\n\n", inversePunnedArrow)
	fmt.Fprintln(output, "Search PartDesign:")
	for index, node := range partDesigns {
		indent := treeIndent("", index, len(partDesigns))
		fmt.Fprintf(output, "%sfound: %s\n", treeBranch(index, len(partDesigns)), nodeSummary(node))
		printDefiningGeometry(output, indent+treeMiddle, indent+treeLine, node)
		printArrangedParts(output, indent+treeLast, indent+treeSpace, node)
	}
	return nil
}

// printDefiningGeometry prints the PartDesign's shape representations and relationships.
func printDefiningGeometry(output io.Writer, branch string, indent string, partDesign sst.IBNode) {
	var geometry []sst.IBNode
	for _, node := range objectNodes(partDesign.GetObjects(sso.DefiningGeometry)) {
		if isKindOf(node, rep.ShapeRepresentation) {
			geometry = append(geometry, node)
		}
	}
	fmt.Fprintf(output, "%s%s sso:definingGeometry (%d)\n", branch, forwardArrow, len(geometry))
	for index, representation := range geometry {
		fmt.Fprintf(output, "%s%s%s\n", indent, treeBranch(index, len(geometry)), nodeSummary(representation))
		references := buildPMIReferenceTree(representation, map[sst.IBNode]bool{representation: true}, map[rdfEdge]bool{})
		printReferenceTree(output, treeIndent(indent, index, len(geometry)), representation, references)
	}
}

// printArrangedParts prints all arranged parts and expands ShapeElement PMI references.
func printArrangedParts(output io.Writer, branch string, indent string, partDesign sst.IBNode) {
	arrangedParts := objectNodes(partDesign.GetObjects(lci.HasArrangedPart))

	fmt.Fprintf(output, "%s%s lci:hasArrangedPart (%d)\n", branch, forwardArrow, len(arrangedParts))
	for index, arrangedPart := range arrangedParts {
		fmt.Fprintf(output, "%s%s%s\n", indent, treeBranch(index, len(arrangedParts)), nodeSummary(arrangedPart))
		if isKindOf(arrangedPart, sso.ShapeElement) {
			references := buildPMIReferenceTree(arrangedPart, map[sst.IBNode]bool{arrangedPart: true}, map[rdfEdge]bool{})
			printReferenceTree(output, treeIndent(indent, index, len(arrangedParts)), arrangedPart, references)
		}
		if index != len(arrangedParts)-1 {
			fmt.Fprintln(output)
		}
	}
}

// buildPMIReferenceTree recursively follows references allowed by the PMI traversal rules.
func buildPMIReferenceTree(node sst.IBNode, visitedNodes map[sst.IBNode]bool, visitedEdges map[rdfEdge]bool) []pmiReference {
	var references []pmiReference
	for _, reference := range referencesConnectedTo(node) {
		rule, relatedNode, ok := matchingPMITraversalRule(node, reference)
		if !ok || visitedEdges[reference.rdfEdge] {
			continue
		}
		visitedEdges[reference.rdfEdge] = true
		if rule.expand && !visitedNodes[relatedNode] {
			visitedNodes[relatedNode] = true
			reference.childAnchor = relatedNode
			reference.children = buildPMIReferenceTree(relatedNode, visitedNodes, visitedEdges)
			if isPunnedUsageProperty(reference.predicate) && len(reference.children) == 0 && !visitedNodes[reference.predicate] {
				visitedNodes[reference.predicate] = true
				reference.childAnchor = reference.predicate
				reference.children = buildPMIReferenceTree(reference.predicate, visitedNodes, visitedEdges)
			}
		}
		references = append(references, reference)
	}
	return references
}

// matchingPMITraversalRule selects the rule and related node for an RDF reference.
func matchingPMITraversalRule(node sst.IBNode, reference pmiReference) (pmiTraversalRule, sst.IBNode, bool) {
	relatedNode := reference.object
	isInverse := reference.object == node && reference.subject != node
	if isInverse {
		relatedNode = reference.subject
	}

	for _, rule := range pmiTraversalRules {
		if rule.sourceKind != nil && !isKindOf(node, rule.sourceKind) {
			continue
		}
		if rule.inverse != isInverse || !isPropertyOrSubpropertyOf(reference.predicate, rule.predicate) ||
			(rule.targetKind != nil && !isKindOf(relatedNode, rule.targetKind)) {
			continue
		}
		return rule, relatedNode, true
	}
	return pmiTraversalRule{}, nil, false
}

// referencesConnectedTo returns forward and inverse RDF references touching a node.
func referencesConnectedTo(node sst.IBNode) []pmiReference {
	var references []pmiReference
	node.ForAll(func(_ int, subject sst.IBNode, predicate sst.IBNode, object sst.Term) error {
		if predicate.Is(rdf.Type) || predicate.Is(rdf.First) || predicate.Is(rdf.Rest) || predicate.Is(lci.HasArrangedPart) {
			return nil
		}
		for _, objectNode := range objectNodes([]sst.Term{object}) {
			if subject == node || objectNode == node {
				references = append(references, pmiReference{rdfEdge: rdfEdge{subject, predicate, objectNode}})
			}
		}
		return nil
	})
	return references
}

// Graph queries: classify RDF nodes and predicates and extract display values.
// isPunnedUsageProperty identifies generated properties that carry relationship metadata.
func isPunnedUsageProperty(predicate sst.IBNode) bool {
	if predicate.Is(sso.DraughtingModelItemUsage_annotation_placeholder) {
		return true
	}
	return predicate.InVocabulary() == nil && (len(predicate.GetObjects(rdfs.SubPropertyOf)) > 0 ||
		len(predicate.GetObjects(sso.ItemIdentifiedRepresentationUsage)) > 0)
}

// isPropertyOrSubpropertyOf checks the complete rdfs:subPropertyOf hierarchy.
func isPropertyOrSubpropertyOf(predicate sst.IBNode, parent sst.ElementInformer) bool {
	pending := []sst.IBNode{predicate}
	seen := map[sst.IBNode]bool{}
	for len(pending) > 0 {
		candidate := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if candidate == nil || seen[candidate] {
			continue
		}
		if candidate.Is(parent) {
			return true
		}
		seen[candidate] = true
		for _, object := range candidate.GetObjects(rdfs.SubPropertyOf) {
			if objectNode, ok := object.(sst.IBNode); ok {
				pending = append(pending, objectNode)
			}
		}
	}
	return false
}

// isKindOf checks whether any RDF type is the requested ontology class or a subtype.
func isKindOf(node sst.IBNode, kind sst.ElementInformer) bool {
	for _, object := range node.GetObjects(rdf.Type) {
		typeNode, ok := object.(sst.IBNode)
		if ok && (typeNode.Is(kind) || typeNode.IsKind(kind)) {
			return true
		}
	}
	return false
}

// isQAUPredicate reports whether a predicate belongs to the QAU vocabulary.
func isQAUPredicate(node sst.IBNode) bool {
	info := node.InVocabulary()
	return info != nil && info.VocabularyElement().Vocabulary.BaseIRI == qau.QAUVocabulary.BaseIRI
}

// label returns the first string rdfs:label attached to a node.
func label(node sst.IBNode) string {
	for _, object := range node.GetObjects(rdfs.Label) {
		if value, ok := object.(sst.String); ok {
			return string(value)
		}
	}
	return ""
}

// Report rendering: turn collected graph references into a directional tree.
// printReferenceTree renders reference values and descendants with tree branches.
func printReferenceTree(output io.Writer, indent string, node sst.IBNode, references []pmiReference) {
	values := qauValues(node)
	sort.Slice(references, func(i, j int) bool {
		return referenceSortKey(node, references[i]) < referenceSortKey(node, references[j])
	})
	total := len(values) + len(references)
	for index, value := range values {
		fmt.Fprintf(output, "%s%s%s\n", indent, treeBranch(index, total), value)
	}
	for index, reference := range references {
		itemIndex := len(values) + index
		branch := treeBranch(itemIndex, total)
		nextIndent := treeIndent(indent, itemIndex, total)
		arrow, relatedNode := referenceDirection(node, reference)
		fmt.Fprintf(output, "%s%s%s %s %s\n", indent, branch, arrow, propertySummary(reference.predicate), nodeSummary(relatedNode))
		childNode := relatedNode
		if reference.childAnchor != nil {
			childNode = reference.childAnchor
		}
		printReferenceTree(output, nextIndent, childNode, reference.children)
	}
}

// qauValues formats quantity values expressed through QAU predicates.
func qauValues(quantity sst.IBNode) []string {
	var values []string
	quantity.ForAll(func(_ int, subject sst.IBNode, predicate sst.IBNode, object sst.Term) error {
		if subject != quantity || !isQAUPredicate(predicate) {
			return nil
		}
		values = append(values, fmt.Sprintf("%v %s", object, prefixedNode(predicate)))
		return nil
	})
	sort.Strings(values)
	return values
}

// objectNodes extracts and sorts node objects, including RDF collection members.
func objectNodes(objects []sst.Term) []sst.IBNode {
	var nodes []sst.IBNode
	for _, object := range objects {
		if objectNode, ok := object.(sst.IBNode); ok {
			if collection, ok := objectNode.AsCollection(); ok {
				collection.ForMembers(func(_ int, member sst.Term) {
					if memberNode, ok := member.(sst.IBNode); ok {
						nodes = append(nodes, memberNode)
					}
				})
				continue
			}
			nodes = append(nodes, objectNode)
		}
	}
	if len(nodes) > 1 {
		sort.Slice(nodes, func(i, j int) bool {
			return nodeSortKey(nodes[i]) < nodeSortKey(nodes[j])
		})
	}
	return nodes
}

// Node and tree formatting: keep RDF names and tree layout separate from traversal.
// propertySummary formats an ontology or punned predicate for the report.
func propertySummary(predicate sst.IBNode) string {
	var parents []string
	for _, object := range predicate.GetObjects(rdfs.SubPropertyOf) {
		if objectNode, ok := object.(sst.IBNode); ok {
			parents = append(parents, prefixedNode(objectNode))
		}
	}
	if len(parents) == 0 {
		return prefixedNode(predicate)
	}
	sort.Strings(parents)
	if isPunnedUsageProperty(predicate) {
		return strings.Join(parents, " | ")
	}
	return strings.Join(parents, " | ") + " [" + predicate.Fragment() + "]"
}

// nodeSummary formats a node's label, ontology types, and identifier.
func nodeSummary(node sst.IBNode) string {
	var parts []string
	if value := label(node); value != "" {
		parts = append(parts, fmt.Sprintf("%q", value))
	}
	var types []string
	for _, object := range node.GetObjects(rdf.Type) {
		if typeNode, ok := object.(sst.IBNode); ok {
			types = append(types, prefixedNode(typeNode))
		}
	}
	sort.Strings(types)
	for _, typeName := range types {
		parts = append(parts, "<"+typeName+">")
	}
	parts = append(parts, "["+displayNodeIdentifier(node)+"]")
	return strings.Join(parts, " ")
}

// referenceDirection returns the display arrow and node at the other end of a reference.
func referenceDirection(node sst.IBNode, reference pmiReference) (string, sst.IBNode) {
	if reference.subject != node {
		if isPunnedUsageProperty(reference.predicate) {
			return inversePunnedArrow, reference.subject
		}
		return inverseArrow, reference.subject
	}
	if isPunnedUsageProperty(reference.predicate) {
		return punnedArrow, reference.object
	}
	return forwardArrow, reference.object
}

// referenceSortKey orders references by direction, property, and related node.
func referenceSortKey(node sst.IBNode, reference pmiReference) string {
	arrow, relatedNode := referenceDirection(node, reference)
	rank := "1"
	if arrow == punnedArrow {
		rank = "0"
	} else if arrow == inverseArrow {
		rank = "2"
	} else if arrow == inversePunnedArrow {
		rank = "3"
	}
	return rank + propertySummary(reference.predicate) + nodeSortKey(relatedNode)
}

// nodeSortKey orders nodes by their displayed summary with a stable name tie-breaker.
func nodeSortKey(node sst.IBNode) string {
	return nodeSummary(node) + nodeName(node)
}

// displayNodeIdentifier formats the identifier shown in square brackets.
func displayNodeIdentifier(node sst.IBNode) string {
	return nodeName(node)
}

// prefixedNode formats known vocabulary nodes with their short prefix.
func prefixedNode(node sst.IBNode) string {
	if node == nil {
		return ""
	}
	if node.IsBlankNode() {
		return "_:" + node.ID().String()
	}
	if info := node.InVocabulary(); info != nil {
		return prefixedElement(info)
	}
	return nodeName(node)
}

// prefixedElement formats a vocabulary element with its known short prefix.
func prefixedElement(elementer sst.Elementer) string {
	element := elementer.VocabularyElement()
	switch element.Vocabulary.BaseIRI {
	case lci.LCIVocabulary.BaseIRI:
		return "lci:" + string(element.Name)
	case qau.QAUVocabulary.BaseIRI:
		return "qau:" + string(element.Name)
	case rdf.RDFVocabulary.BaseIRI:
		return "rdf:" + string(element.Name)
	case rdfs.RDFSVocabulary.BaseIRI:
		return "rdfs:" + string(element.Name)
	case rep.REPVocabulary.BaseIRI:
		return "rep:" + string(element.Name)
	case sso.SSOVocabulary.BaseIRI:
		return "sso:" + string(element.Name)
	default:
		return string(element.Name)
	}
}

// nodeName formats a graph node as a Turtle-like local or blank-node name.
func nodeName(node sst.IBNode) string {
	if node.IsBlankNode() {
		return "_:" + node.ID().String()
	}
	return ":" + node.Fragment()
}

// treeBranch selects the middle or final branch marker for a sibling.
func treeBranch(index int, total int) string {
	if index == total-1 {
		return treeLast
	}
	return treeMiddle
}

// treeIndent extends indentation while preserving ancestor branch lines.
func treeIndent(indent string, index int, total int) string {
	if index == total-1 {
		return indent + treeSpace
	}
	return indent + treeLine
}
