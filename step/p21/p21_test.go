// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package p21

import (
	"bufio"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/owl"
	"github.com/semanticstep/sst-core/vocabularies/qau"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/semanticstep/sst-core/vocabularies/ssmeta"
	"github.com/semanticstep/sst-core/vocabularies/sso"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const minimalP21 = `ISO-10303-21;
HEADER;
FILE_DESCRIPTION(('minimal'), '2;1');
FILE_NAME('demo', '2003-12-27T11:57:53', ('SST'), ('SST'), ' ', 'SST', ' ');
FILE_SCHEMA (('AUTOMOTIVE_DESIGN { 1 0 10303 214 2 1 1}'));
ENDSEC;
DATA;
ENDSEC;
END-ISO-10303-21;
`

const multilineHeaderStringP21 = "ISO-10303-21;\n" +
	"HEADER;\n" +
	"FILE_DESCRIPTION(('wrapped header stri\r\nng'), '2;1');\n" +
	"FILE_NAME('demo', '2003-12-27T11:57:53', ('SST'), ('SST'), ' ', 'SST', ' ');\n" +
	"FILE_SCHEMA (('AUTOMOTIVE_DESIGN { 1 0 10303 214 2 1 1}'));\n" +
	"ENDSEC;\n" +
	"DATA;\n" +
	"ENDSEC;\n" +
	"END-ISO-10303-21;\n"

const productDefinitionFormationWithSpecifiedSourceP21 = `ISO-10303-21;
HEADER;
FILE_DESCRIPTION(('minimal product chain'), '2;1');
FILE_NAME('demo', '2003-12-27T11:57:53', ('SST'), ('SST'), ' ', 'SST', ' ');
FILE_SCHEMA (('AP242_MANAGED_MODEL_BASED_3D_ENGINEERING_MIM_LF { 1 0 10303 442 1 1 4}'));
ENDSEC;
DATA;
#1=APPLICATION_CONTEXT('');
#2=PRODUCT_CONTEXT('',#1,'mechanical');
#3=PRODUCT('part-1','Part 1','demo part',(#2));
#4=PRODUCT_DEFINITION_FORMATION_WITH_SPECIFIED_SOURCE('v1','version comment',#3,.NOT_KNOWN.);
#5=PRODUCT_DEFINITION_CONTEXT('',#1,'design');
#6=PRODUCT_DEFINITION('view-1','view comment',#4,#5);
#7=PRODUCT_RELATED_PRODUCT_CATEGORY('','',(#3));
ENDSEC;
END-ISO-10303-21;
`

func TestParseDoesNotWriteRawDebugTurtleFile(t *testing.T) {
	tempDir := t.TempDir()
	debugDir := filepath.Join(tempDir, "step", "p21", "testdata")
	require.NoError(t, os.MkdirAll(debugDir, 0o755))

	previousWorkingDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(previousWorkingDir))
	})

	graph, err := Parse(bufio.NewReader(strings.NewReader(minimalP21)), log.New(io.Discard, "", 0))
	require.NoError(t, err)
	require.NotNil(t, graph)

	rawDebugTurtle := filepath.Join(debugDir, "first.ttl")
	_, err = os.Stat(rawDebugTurtle)
	assert.True(t, os.IsNotExist(err), "Parse should not write debug files; examples own that behavior")
}

func TestParseRawAcceptsWrappedHeaderStrings(t *testing.T) {
	graph, err := ParseRaw(bufio.NewReader(strings.NewReader(multilineHeaderStringP21)), log.New(io.Discard, "", 0))

	require.NoError(t, err)
	require.NotNil(t, graph)
}

func TestParseRawReturnsPreConversionGraph(t *testing.T) {
	file, err := os.Open("testdata/as1-ec-214.stp")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, file.Close())
	})

	graph, err := ParseRaw(bufio.NewReader(file), log.New(io.Discard, "", 0))
	require.NoError(t, err)
	require.NotNil(t, graph)

	assert.True(t, graphHasPredicate(t, graph, ssmeta.EntityInstanceType), "raw parse graph should still contain parser entity instance triples")
}

func TestParseImportsAS1EC214Fixture(t *testing.T) {
	graph := parseAS1EC214Fixture(t)
	require.NotNil(t, graph)
}

func TestParseDoesNotUseRDFListAsPredicate(t *testing.T) {
	graph := parseAS1EC214Fixture(t)

	var subjects []string
	require.NoError(t, graph.ForIRINodes(func(node sst.IBNode) error {
		node.ForAll(func(_ int, s, p sst.IBNode, _ sst.Term) error {
			if node == s && p.Is(rdf.List) {
				subjects = append(subjects, s.PrefixedFragment())
			}
			return nil
		})
		return nil
	}))

	assert.Empty(t, subjects, "rdf:List is a class used for collection ranges, not an attribute predicate")
}

func TestParseMapsRepresentationItemsAndContextFromOntology(t *testing.T) {
	graph := parseAS1EC214Fixture(t)

	for _, typ := range []sst.Elementer{rep.AdvancedBrepShapeRepresentation, rep.ShapeRepresentation} {
		nodes := nodesOfType(t, graph, typ)
		require.NotEmpty(t, nodes)

		for _, node := range nodes {
			items := node.GetObjects(rep.Item)
			require.NotEmpty(t, items, "%s should have rep:item values", node.PrefixedFragment())
			for _, item := range items {
				_, isCollection := item.(sst.IBNode).AsCollection()
				assert.False(t, isCollection, "%s rep:item is a SET and should be repeated triples", node.PrefixedFragment())
			}

			contexts := node.GetObjects(rep.ContextOfItems)
			require.Len(t, contexts, 1, "%s should have one rep:contextOfItems value", node.PrefixedFragment())
			_, isCollection := contexts[0].(sst.IBNode).AsCollection()
			assert.False(t, isCollection, "%s rep:contextOfItems should be a direct representation context", node.PrefixedFragment())
		}
	}
}

func TestParseMapsEdgeLoopEdgeListFromPathOntology(t *testing.T) {
	graph := parseAS1EC214Fixture(t)

	edgeLoops := nodesOfType(t, graph, rep.EdgeLoop)
	require.NotEmpty(t, edgeLoops)

	for _, edgeLoop := range edgeLoops {
		edgeLists := edgeLoop.GetObjects(rep.EdgeList)
		require.Len(t, edgeLists, 1, "%s should have one rep:edgeList value", edgeLoop.PrefixedFragment())
		_, isCollection := edgeLists[0].(sst.IBNode).AsCollection()
		assert.True(t, isCollection, "%s rep:edgeList should preserve the EXPRESS LIST as an RDF collection", edgeLoop.PrefixedFragment())
	}
}

func TestParseMapsGlobalUnitsFromOntology(t *testing.T) {
	graph := parseAS1EC214Fixture(t)

	contexts := nodesOfType(t, graph, rep.GlobalUnitAssignedContext)
	require.Len(t, contexts, 9, "fixture should contain nine global_unit_assigned_context layers")

	for _, context := range contexts {
		assert.True(t, hasObject(context, rep.GlobalUnit, qau.MilliMetre), "%s should use millimetre as length global unit", context.PrefixedFragment())
		assert.True(t, hasObject(context, rep.GlobalUnit, qau.Radian), "%s should use radian as angle global unit", context.PrefixedFragment())
		assert.True(t, hasObject(context, rep.GlobalUnit, qau.Steradian), "%s should use steradian as solid angle global unit", context.PrefixedFragment())
	}
}

func TestGlobalUnitElementUsesUnitEntityStructure(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)

	lengthUnit := graph.CreateIRINode("length-unit")
	namedUnit := graph.CreateIRINode("named-unit")
	siUnit := graph.CreateIRINode("si-unit")
	milli := graph.CreateIRINode("milli")
	metre := graph.CreateIRINode("metre")
	unit := graph.CreateIRINode("unit")

	cp.rawAttributeValuesMap[lengthUnit] = RawAttributeValues{name: LENGTH_UNIT}
	cp.rawAttributeValuesMap[namedUnit] = RawAttributeValues{name: NAMED_UNIT}
	cp.rawAttributeValuesMap[siUnit] = RawAttributeValues{
		name:        SI_UNIT,
		MixedValues: []interface{}{milli, metre},
	}
	cp.enumerationValueMap[milli] = EnumerationValue{ExpressObject: ExpressObject{name: "MILLI"}}
	cp.enumerationValueMap[metre] = EnumerationValue{ExpressObject: ExpressObject{name: "METRE"}}
	cp.complexInstanceValues[unit] = []sst.IBNode{lengthUnit, namedUnit, siUnit}

	unitElement, ok := cp.globalUnitElement(unit)

	require.True(t, ok)
	assert.Equal(t, qau.MilliMetre.Element, unitElement)
}

func TestHandleMeasureRepresentationItemUsesComplexLengthUnit(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)

	measureItem := graph.CreateIRINode("measure-item")
	lengthMeasure := graph.CreateIRINode("length-measure")
	lengthUnit := graph.CreateIRINode("length-unit")
	namedUnit := graph.CreateIRINode("named-unit")
	siUnit := graph.CreateIRINode("si-unit")
	milli := graph.CreateIRINode("milli")
	metre := graph.CreateIRINode("metre")
	unit := graph.CreateIRINode("unit")

	cp.rawAttributeValuesMap[measureItem] = RawAttributeValues{
		name: MEASURE_REPRESENTATION_ITEM,
		MixedValues: []interface{}{
			sst.String("tessellated curve length"),
			graph.CreateCollection(lengthMeasure, sst.Double(118.731752270833)),
			unit,
		},
	}
	cp.definedTypeMap[lengthMeasure] = DefinedType{ExpressObject: ExpressObject{name: "LENGTH_MEASURE"}}
	cp.rawAttributeValuesMap[lengthUnit] = RawAttributeValues{name: LENGTH_UNIT}
	cp.rawAttributeValuesMap[namedUnit] = RawAttributeValues{name: NAMED_UNIT}
	cp.rawAttributeValuesMap[siUnit] = RawAttributeValues{
		name:        SI_UNIT,
		MixedValues: []interface{}{milli, metre},
	}
	cp.enumerationValueMap[milli] = EnumerationValue{ExpressObject: ExpressObject{name: "MILLI"}}
	cp.enumerationValueMap[metre] = EnumerationValue{ExpressObject: ExpressObject{name: "METRE"}}
	cp.complexInstanceValues[unit] = []sst.IBNode{lengthUnit, namedUnit, siUnit}

	cp.handleMeasureRepresentationItem(measureItem)

	values := measureItem.GetObjects(rep.MeasureValue)
	require.Len(t, values, 1, "measure representation item should link its QAU value")
	quantity, ok := values[0].(sst.IBNode)
	require.True(t, ok)
	assert.True(t, hasObject(quantity, rdf.Type, lci.PhysicalQuantity))
	assert.True(t, hasObject(quantity, rdf.Type, qau.Length))
	assert.Contains(t, quantity.GetObjects(qau.MilliMetre), sst.Double(118.731752270833))
}

func TestParseMapsGlobalUncertaintyMeasures(t *testing.T) {
	graph := parseAS1EC214Fixture(t)

	contexts := nodesOfType(t, graph, rep.GlobalUncertaintyAssignedContext)
	require.Len(t, contexts, 9, "fixture should contain nine global_uncertainty_assigned_context layers")

	for _, context := range contexts {
		uncertainties := context.GetObjects(rep.GlobalUncertainty)
		require.Len(t, uncertainties, 1, "%s should link one global uncertainty measure", context.PrefixedFragment())

		uncertainty, ok := uncertainties[0].(sst.IBNode)
		require.True(t, ok, "%s global uncertainty should be an RDF node", context.PrefixedFragment())
		assert.True(t, hasObject(uncertainty, rdf.Type, qau.Length), "%s should type uncertainty as qau:Length", uncertainty.PrefixedFragment())
		assert.Contains(t, uncertainty.GetObjects(qau.MilliMetre), sst.Double(1e-07), "%s should keep the millimetre uncertainty value", uncertainty.PrefixedFragment())
		assert.Contains(t, uncertainty.GetObjects(rdfs.Label), sst.String("distance_accuracy_value"), "%s should keep the uncertainty label", uncertainty.PrefixedFragment())
		assert.Contains(t, uncertainty.GetObjects(rdfs.Comment), sst.String("confusion accuracy"), "%s should keep the uncertainty description", uncertainty.PrefixedFragment())
	}
}

func TestParseMapsProductDefinitionFormationWithSpecifiedSourceToPartVersion(t *testing.T) {
	graph, err := Parse(bufio.NewReader(strings.NewReader(productDefinitionFormationWithSpecifiedSourceP21)), log.New(io.Discard, "", 0))
	require.NoError(t, err)

	part := nodeOfTypeWithLiteral(t, graph, sso.Part, sso.ID, sst.String("part-1"))
	require.NotNil(t, part)

	partVersions := part.GetObjects(sso.HasPartVersion)
	require.Len(t, partVersions, 1, "product should link the formation subtype as a part version")
	partVersion, ok := partVersions[0].(sst.IBNode)
	require.True(t, ok)
	assert.True(t, hasObject(partVersion, rdf.Type, sso.PartVersion))

	partDefinitions := partVersion.GetObjects(sso.HasProductDefinition)
	require.Len(t, partDefinitions, 1, "part version should link product_definition as a part view/design")
	partDefinition, ok := partDefinitions[0].(sst.IBNode)
	require.True(t, ok)
	assert.True(t, hasObject(partDefinition, rdf.Type, sso.PartDesign))
}

func TestGetSSREPFindsEntityMapsOutsideRepresentationOntology(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)

	node := cp.getSSREP("FLATNESS_TOLERANCE")

	require.NotNil(t, node)
	assert.True(t, node.Is(sso.FlatnessTolerance))
}

func TestGetSSREPUsesSpecificMainClassForDuplicateEntityMap(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)

	node := cp.getSSREP("MODEL_GEOMETRIC_VIEW")

	require.NotNil(t, node)
	assert.True(t, node.Is(rep.ModelGeometricView))
}

func TestGetSSREPKeepsSuperclassWhenSubtypeAddsMappedAttributes(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)

	node := cp.getSSREP("SURFACE")

	require.NotNil(t, node)
	assert.True(t, node.Is(rep.Surface))
	assert.False(t, node.Is(rep.ElementarySurface))
}

func TestOntologyEntityCachesKindAndAttributeOrder(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)

	entity := cp.ontologyEntity("ADVANCED_FACE")

	require.NotNil(t, entity.ontologyObject)
	assert.True(t, entity.ontologyObject.Is(rep.AdvancedFace))
	assert.Equal(t, MainClass, entity.ontologyType)
	assert.NotEmpty(t, entity.attributeOrders)

	cached := cp.ontologyEntity("advanced_face")
	assert.Equal(t, entity.ontologyObject, cached.ontologyObject)
	assert.Len(t, cp.ontologyEntityCache, 1)
}

func TestExtractNodeUsesStepImMapSupertypeOrder(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)
	subject := graph.CreateIRINode("subject")
	firstSuper := graph.CreateIRINode("first-super")
	secondSuper := graph.CreateIRINode("second-super")
	firstAttribute := graph.CreateIRINode("first-attribute")
	secondAttribute := graph.CreateIRINode("second-attribute")

	subject.AddStatement(rdf.Type, ssmeta.MainClass)
	subject.AddStatement(rdfs.SubClassOf, secondSuper)
	subject.AddStatement(rdfs.SubClassOf, firstSuper)
	subject.AddStatement(ssmeta.StepImMapSupertypeOrder, graph.CreateCollection(firstSuper, secondSuper))
	firstSuper.AddStatement(ssmeta.StepImMapAttributeOrder, graph.CreateCollection(firstAttribute))
	secondSuper.AddStatement(ssmeta.StepImMapAttributeOrder, graph.CreateCollection(secondAttribute))

	var entity SingleEntity
	cp.extractNode(subject, &entity, "")

	var actual []string
	for _, attribute := range entity.attributeOrders {
		actual = append(actual, attribute.Fragment())
	}
	assert.Equal(t, []string{firstAttribute.Fragment(), secondAttribute.Fragment()}, actual)
}

func TestEnumerationElementUsesStepImEnumerationMapFromAttributeRange(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)

	element, ok := cp.enumerationElement(vocabularyNode(t, graph, rep.KnotSpec), "PIECEWISE_BEZIER_KNOTS")

	require.True(t, ok)
	assert.Equal(t, rep.KnotType_PiecewiseBezierKnots.Element, element)
}

func TestParseTypesQAUQuantitiesAsPhysicalQuantities(t *testing.T) {
	graph := parseAS1EC214Fixture(t)

	var quantityCount int
	for _, quantityType := range []sst.Elementer{qau.Area, qau.Length, qau.Volume} {
		for _, quantity := range allNodesOfType(t, graph, quantityType) {
			quantityCount++
			assert.True(t, hasObject(quantity, rdf.Type, lci.PhysicalQuantity), "%s should explicitly satisfy lci:PhysicalQuantity ranges", quantity.PrefixedFragment())
		}
	}

	require.Equal(t, 27, quantityCount, "fixture should contain QAU area, length, and volume quantity nodes")
}

func TestParseTypesItemDefinedTransformationOperatorsAsTransformationSelects(t *testing.T) {
	graph := parseAS1EC214Fixture(t)

	var transformationOperators []sst.IBNode
	require.NoError(t, graph.ForIRINodes(func(node sst.IBNode) error {
		for _, object := range node.GetObjects(rep.TransformationOperator) {
			if operator, ok := object.(sst.IBNode); ok {
				transformationOperators = append(transformationOperators, operator)
			}
		}
		return nil
	}))

	require.Len(t, transformationOperators, 13, "fixture should contain thirteen item_defined_transformation operators")
	for _, operator := range transformationOperators {
		assert.True(t, hasObject(operator, rdf.Type, owl.ObjectProperty), "%s should keep the dynamic item-defined transformation property", operator.PrefixedFragment())
		assert.True(t, hasObject(operator, rdfs.SubPropertyOf, rep.ItemDefinedTransformation), "%s should remain a subproperty of rep:itemDefinedTransformation", operator.PrefixedFragment())
	}
}

func TestParseMapsPropertyDefinitionRepresentationsAsDomainRangeProperties(t *testing.T) {
	graph := parseAS1EC214Fixture(t)

	var invalidPredicates []string
	var individualRepresentationCount int
	var individualShapeRepresentationCount int

	require.NoError(t, graph.ForIRINodes(func(node sst.IBNode) error {
		node.ForAll(func(_ int, s, p sst.IBNode, _ sst.Term) error {
			if node == s && p.Is(rep.Representation) {
				invalidPredicates = append(invalidPredicates, s.PrefixedFragment())
			}
			return nil
		})

		if !hasObject(node, rdf.Type, owl.ObjectProperty) {
			return nil
		}

		switch {
		case hasObject(node, rdfs.SubPropertyOf, rep.IndividualShapeRepresentation):
			individualShapeRepresentationCount++
		case hasObject(node, rdfs.SubPropertyOf, rep.IndividualRepresentation):
			individualRepresentationCount++
		default:
			return nil
		}

		return nil
	}))

	assert.Empty(t, invalidPredicates, "rep:Representation is a class and must not be used as a predicate")
	assert.Greater(t, individualRepresentationCount, 0, "fixture should contain property_definition_representation mappings")
	assert.Greater(t, individualShapeRepresentationCount, 0, "fixture should contain shape_definition_representation mappings")
}

func TestParseKeepsOrientedEdgeLabelSeparateFromDerivedEdgeVertices(t *testing.T) {
	graph := parseAS1EC214Fixture(t)

	var labeledEdges []sst.IBNode
	for _, edge := range nodesOfType(t, graph, rep.OrientedEdge) {
		if !containsLiteral(edge.GetObjects(rdfs.Label), sst.String("checkORIENTED")) {
			continue
		}

		labeledEdges = append(labeledEdges, edge)
		assert.NotEmpty(t, edge.GetObjects(rep.EdgeElement), "%s should keep the explicit edge_element", edge.PrefixedFragment())
		assert.Contains(t, edge.GetObjects(rep.Orientation), sst.Boolean(false), "%s should keep the explicit orientation", edge.PrefixedFragment())

		for _, edgeEnd := range edge.GetObjects(rep.EdgeEnd) {
			_, isLiteral := edgeEnd.(sst.Literal)
			assert.False(t, isLiteral, "%s should not map the label to rep:edgeEnd", edge.PrefixedFragment())
		}
	}

	require.Len(t, labeledEdges, 1, "fixture should contain the ORIENTED_EDGE named checkORIENTED")
}

func parseAS1EC214Fixture(t testing.TB) sst.NamedGraph {
	t.Helper()

	file, err := os.Open("testdata/as1-ec-214.stp")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, file.Close())
	})

	graph, err := Parse(bufio.NewReader(file), log.New(io.Discard, "", 0))
	require.NoError(t, err)
	return graph
}

func TestProcessLeaveMapsSSTLiteralsIntoStatements(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)
	node := graph.CreateIRINode("subject")

	cp.processLeave(
		node,
		[]sst.IBNode{
			vocabularyNode(t, graph, rdfs.Label),
			vocabularyNode(t, graph, rdfs.Comment),
			vocabularyNode(t, graph, rdf.Value),
		},
		[]interface{}{
			sst.String("part name"),
			sst.Double(12.5),
			sst.Integer(3),
		},
	)

	assert.Contains(t, node.GetObjects(rdfs.Label), sst.String("part name"))
	assert.Contains(t, node.GetObjects(rdfs.Comment), sst.Double(12.5))
	assert.Contains(t, node.GetObjects(rdf.Value), sst.Integer(3))
}

func TestProcessLeaveIgnoresExtraRawValuesWithoutMappedOntologyAttributes(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)
	node := graph.CreateIRINode("subject")
	extraCollection := graph.CreateCollection(graph.CreateIRINode("extra"))

	require.NotPanics(t, func() {
		cp.processLeave(
			node,
			[]sst.IBNode{
				vocabularyNode(t, graph, rdfs.Label),
				vocabularyNode(t, graph, rdfs.Comment),
				vocabularyNode(t, graph, rdf.Value),
			},
			[]interface{}{
				sst.String("mapped label"),
				sst.String("mapped comment"),
				sst.Integer(42),
				extraCollection,
			},
		)
	})

	assert.Contains(t, node.GetObjects(rdfs.Label), sst.String("mapped label"))
	assert.Contains(t, node.GetObjects(rdfs.Comment), sst.String("mapped comment"))
	assert.Contains(t, node.GetObjects(rdf.Value), sst.Integer(42))
}

func TestProcessCommonDataMapsSSTStringLiterals(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)
	node := graph.CreateIRINode("product")
	cp.rawAttributeValuesMap[node] = RawAttributeValues{
		MixedValues: []interface{}{
			sst.String("A0001"),
			sst.String("Part 1"),
			sst.String("demo comment"),
		},
	}

	cp.processCommonData(node, "")

	assert.Contains(t, node.GetObjects(sso.ID), sst.String("A0001"))
	assert.Contains(t, node.GetObjects(rdfs.Label), sst.String("Part 1"))
	assert.Contains(t, node.GetObjects(rdfs.Comment), sst.String("demo comment"))
}

func TestConvertPDMCollectionHandlesProductDefinitionFormationWithSpecifiedSource(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)
	product := graph.CreateIRINode("product")
	category := graph.CreateIRINode("category")
	formation := graph.CreateIRINode("formation")
	source := graph.CreateIRINode("source")
	definition := graph.CreateIRINode("definition")
	context := graph.CreateIRINode("context")

	cp.rawAttributeValuesMap[product] = RawAttributeValues{
		name: PRODUCT,
		MixedValues: []interface{}{
			sst.String("d06999-04_end_plate_NX"),
			sst.String("d06999-04_end_plate_NX"),
			sst.String("d06999-04_end_plate_NX"),
		},
	}
	cp.rawAttributeValuesMap[category] = RawAttributeValues{
		name: PRODUCT_RELATED_PRODUCT_CATEGORY,
		MixedValues: []interface{}{
			sst.String(""),
			sst.String(""),
			graph.CreateCollection(product),
		},
	}
	cp.rawAttributeValuesMap[formation] = RawAttributeValues{
		name: PRODUCT_DEFINITION_FORMATION_WITH_SPECIFIED_SOURCE,
		MixedValues: []interface{}{
			sst.String(""),
			sst.String(""),
			product,
			source,
		},
	}
	cp.rawAttributeValuesMap[definition] = RawAttributeValues{
		name: PRODUCT_DEFINITION,
		MixedValues: []interface{}{
			sst.String(""),
			sst.String(""),
			formation,
			context,
		},
	}

	cp.convertPDMCollection(product)

	assert.True(t, hasObject(product, rdf.Type, sso.Part))
	assert.Contains(t, product.GetObjects(sso.HasPartVersion), formation)
	assert.True(t, hasObject(formation, rdf.Type, sso.PartVersion))
	assert.Contains(t, formation.GetObjects(sso.HasProductDefinition), definition)
	assert.True(t, hasObject(definition, rdf.Type, sso.PartDesign))
}

func TestSearchNodeUsesReverseReferenceIndex(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)
	target := graph.CreateIRINode("target")
	directOwner := graph.CreateIRINode("direct-owner")
	collectionOwner := graph.CreateIRINode("collection-owner")
	other := graph.CreateIRINode("other")

	cp.rawAttributeValuesMap[directOwner] = RawAttributeValues{
		MixedValues: []interface{}{target},
	}
	cp.rawAttributeValuesMap[collectionOwner] = RawAttributeValues{
		MixedValues: []interface{}{graph.CreateCollection(other, target)},
	}

	assert.ElementsMatch(t, []sst.IBNode{directOwner, collectionOwner}, cp.searchNode(target))
	assert.ElementsMatch(t, []sst.IBNode{directOwner, collectionOwner}, cp.rawReferenceIndex[target])
}

func newTestGraph(t testing.TB) sst.NamedGraph {
	t.Helper()

	stage := sst.OpenStage(sst.DefaultTriplexMode)
	require.NotNil(t, stage)
	graph := stage.CreateNamedGraph("")
	require.NotNil(t, graph)
	return graph
}

func vocabularyNode(t testing.TB, graph sst.NamedGraph, element sst.Elementer) sst.IBNode {
	t.Helper()

	node, err := graph.Stage().IBNodeByVocabulary(element)
	require.NoError(t, err)
	require.NotNil(t, node)
	return node
}

func nodesOfType(t testing.TB, graph sst.NamedGraph, typ sst.Elementer) []sst.IBNode {
	t.Helper()

	var nodes []sst.IBNode
	require.NoError(t, graph.ForIRINodes(func(node sst.IBNode) error {
		for _, object := range node.GetObjects(rdf.Type) {
			if typeNode, ok := object.(sst.IBNode); ok && typeNode.Is(typ) {
				nodes = append(nodes, node)
				break
			}
		}
		return nil
	}))
	return nodes
}

func nodeOfTypeWithLiteral(t testing.TB, graph sst.NamedGraph, typ sst.Elementer, predicate sst.Node, literal sst.Literal) sst.IBNode {
	t.Helper()

	for _, node := range nodesOfType(t, graph, typ) {
		if containsLiteral(node.GetObjects(predicate), literal) {
			return node
		}
	}
	return nil
}

func allNodesOfType(t testing.TB, graph sst.NamedGraph, typ sst.Elementer) []sst.IBNode {
	t.Helper()

	var nodes []sst.IBNode
	require.NoError(t, graph.ForAllIBNodes(func(node sst.IBNode) error {
		for _, object := range node.GetObjects(rdf.Type) {
			if typeNode, ok := object.(sst.IBNode); ok && typeNode.Is(typ) {
				nodes = append(nodes, node)
				break
			}
		}
		return nil
	}))
	return nodes
}

func hasObject(node sst.IBNode, predicate sst.Node, expected sst.Elementer) bool {
	for _, object := range node.GetObjects(predicate) {
		if objectNode, ok := object.(sst.IBNode); ok && objectNode.Is(expected) {
			return true
		}
	}
	return false
}

func containsLiteral(objects []sst.Term, expected sst.Literal) bool {
	for _, object := range objects {
		if object == expected {
			return true
		}
	}
	return false
}

func graphHasPredicate(t testing.TB, graph sst.NamedGraph, predicate sst.Elementer) bool {
	t.Helper()

	found := false
	require.NoError(t, graph.ForAllIBNodes(func(node sst.IBNode) error {
		return node.ForAll(func(_ int, s, p sst.IBNode, _ sst.Term) error {
			if node == s && p.Is(predicate) {
				found = true
			}
			return nil
		})
	}))
	return found
}
