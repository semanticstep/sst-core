// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package p21

import (
	"bufio"
	"bytes"
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

const shapeAspectOfProductDefinitionShapeP21 = `ISO-10303-21;
HEADER;
FILE_DESCRIPTION(('minimal shape aspect chain'), '2;1');
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
#8=PRODUCT_DEFINITION_SHAPE('','',#6);
#9=SHAPE_ASPECT('feature','feature comment',#8,.T.);
ENDSEC;
END-ISO-10303-21;
`

const toleranceValueWithMeasureUnitsP21 = `ISO-10303-21;
HEADER;
FILE_DESCRIPTION(('minimal tolerance value'), '2;1');
FILE_NAME('demo', '2003-12-27T11:57:53', ('SST'), ('SST'), ' ', 'SST', ' ');
FILE_SCHEMA (('AP242_MANAGED_MODEL_BASED_3D_ENGINEERING_MIM_LF { 1 0 10303 442 1 1 4}'));
ENDSEC;
DATA;
#1=(LENGTH_UNIT() NAMED_UNIT(*) SI_UNIT(.MILLI.,.METRE.));
#2=MEASURE_WITH_UNIT(LENGTH_MEASURE(-0.1),#1);
#3=MEASURE_WITH_UNIT(LENGTH_MEASURE(0.1),#1);
#4=TOLERANCE_VALUE(#2,#3);
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
		edgeMembers := collectionMembers(t, edgeLists[0])
		require.NotEmpty(t, edgeMembers, "%s rep:edgeList should preserve the EXPRESS LIST as an RDF collection", edgeLoop.PrefixedFragment())

		orientationLists := edgeLoop.GetObjects(rep.OrientationList)
		require.Len(t, orientationLists, 1, "%s should have one rep:orientationList value paired with rep:edgeList", edgeLoop.PrefixedFragment())
		orientationMembers := collectionMembers(t, orientationLists[0])
		require.Len(t, orientationMembers, len(edgeMembers), "%s should preserve one orientation per edge", edgeLoop.PrefixedFragment())

		for _, member := range edgeMembers {
			edge, ok := member.(sst.IBNode)
			require.True(t, ok, "%s rep:edgeList should contain edge nodes", edgeLoop.PrefixedFragment())
			assert.False(t, hasObject(edge, rdf.Type, rep.OrientedEdge), "%s rep:edgeList should contain edge elements, not oriented-edge wrappers", edgeLoop.PrefixedFragment())
		}
		for _, member := range orientationMembers {
			_, ok := member.(sst.Boolean)
			assert.True(t, ok, "%s rep:orientationList should contain booleans", edgeLoop.PrefixedFragment())
		}
	}
}

func TestParseMapsGlobalUnitsFromOntology(t *testing.T) {
	graph := parseAS1EC214Fixture(t)

	contexts := nodesOfType(t, graph, rep.GlobalUnitAssignedContext)
	require.Len(t, contexts, 9, "fixture should contain nine global_unit_assigned_context layers")

	for _, context := range contexts {
		assert.True(t, hasObject(context, rep.GlobalUnit, qau.MilliMetre), "%s should use millimetre as length global unit", context.PrefixedFragment())
		assert.False(t, hasObject(context, rep.GlobalUnit, qau.Radian), "%s should omit the constant radian global unit", context.PrefixedFragment())
		assert.False(t, hasObject(context, rep.GlobalUnit, qau.Steradian), "%s should omit the constant steradian global unit", context.PrefixedFragment())
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

	assert.True(t, hasObject(measureItem, rdf.Type, qau.Length))
	assert.False(t, hasObject(measureItem, rdf.Type, lci.PhysicalQuantity), "qau:Length already carries the physical quantity meaning")
	assert.False(t, hasObject(measureItem, rdf.Type, rep.RepresentationItem), "qau:Length is the final main type")
	assert.Empty(t, measureItem.GetObjects(rep.MeasureValue), "measure value should be applied to the item itself")
	assert.Contains(t, measureItem.GetObjects(qau.MilliMetre), sst.Double(118.731752270833))
}

func TestHandleMeasureRepresentationItemUsesDerivedAreaUnit(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)

	measureItem := graph.CreateIRINode("measure-item")
	areaMeasure := graph.CreateIRINode("area-measure")
	derivedUnit := graph.CreateIRINode("derived-unit")
	derivedElement := graph.CreateIRINode("derived-unit-element")
	unit := graph.CreateIRINode("unit")
	lengthUnit := graph.CreateIRINode("length-unit")
	namedUnit := graph.CreateIRINode("named-unit")
	siUnit := graph.CreateIRINode("si-unit")
	milli := graph.CreateIRINode("milli")
	metre := graph.CreateIRINode("metre")

	cp.rawAttributeValuesMap[measureItem] = RawAttributeValues{
		name: MEASURE_REPRESENTATION_ITEM,
		MixedValues: []interface{}{
			sst.String("affected area"),
			graph.CreateCollection(areaMeasure, sst.Double(6567.1634)),
			derivedUnit,
		},
	}
	cp.definedTypeMap[areaMeasure] = DefinedType{ExpressObject: ExpressObject{name: "AREA_MEASURE"}}
	cp.rawAttributeValuesMap[derivedUnit] = RawAttributeValues{
		name:        DERIVED_UNIT,
		MixedValues: []interface{}{graph.CreateCollection(derivedElement)},
	}
	cp.rawAttributeValuesMap[derivedElement] = RawAttributeValues{
		name:        DERIVED_UNIT_ELEMENT,
		MixedValues: []interface{}{unit, sst.Double(2)},
	}
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

	assert.True(t, hasObject(measureItem, rdf.Type, qau.Area))
	assert.False(t, hasObject(measureItem, rdf.Type, rep.RepresentationItem), "qau:Area is the final main type")
	assert.Contains(t, measureItem.GetObjects(qau.SquareMilliMetre), sst.Double(6567.1634))
	assert.Contains(t, measureItem.GetObjects(rdfs.Label), sst.String("affected area"))
}

func TestHandleMeasureRepresentationItemAddsRepresentationItemMainType(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)
	measureItem := graph.CreateIRINode("measure-item")

	cp.rawAttributeValuesMap[measureItem] = RawAttributeValues{
		name:        MEASURE_REPRESENTATION_ITEM,
		MixedValues: []interface{}{sst.String("affected area")},
	}

	cp.handleMeasureRepresentationItem(measureItem)

	assert.True(t, hasObject(measureItem, rdf.Type, rep.RepresentationItem), "standalone measure items need a final main class")
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

func TestParseKeepsToleranceValueMeasureBounds(t *testing.T) {
	graph, err := Parse(bufio.NewReader(strings.NewReader(toleranceValueWithMeasureUnitsP21)), log.New(io.Discard, "", 0))
	require.NoError(t, err)

	toleranceValues := nodesOfType(t, graph, sso.ToleranceValue)
	require.Len(t, toleranceValues, 1)

	lowerBounds := toleranceValues[0].GetObjects(sso.ToleranceValueLowerBound)
	require.Len(t, lowerBounds, 1)
	assertLengthMeasureNode(t, lowerBounds[0], -0.1)

	upperBounds := toleranceValues[0].GetObjects(sso.ToleranceValueUpperBound)
	require.Len(t, upperBounds, 1)
	assertLengthMeasureNode(t, upperBounds[0], 0.1)
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

func TestParseResolvesShapeAspectProductDefinitionShapeWrapper(t *testing.T) {
	graph, err := Parse(bufio.NewReader(strings.NewReader(shapeAspectOfProductDefinitionShapeP21)), log.New(io.Discard, "", 0))
	require.NoError(t, err)

	shapeElements := nodesOfType(t, graph, sso.ShapeElement)
	require.Len(t, shapeElements, 1)

	assert.Empty(t, shapeElements[0].GetObjects(lci.ArrangedPartOf), "final RDF should navigate from part design to shape element")

	partDesigns := nodesOfType(t, graph, sso.PartDesign)
	require.Len(t, partDesigns, 1)
	partDesign := partDesigns[0]
	assert.True(t, hasObject(partDesign, rdf.Type, sso.PartDesign))
	assert.Contains(t, partDesign.GetObjects(lci.HasArrangedPart), shapeElements[0])
	assert.Empty(t, nodesOfType(t, graph, sso.ProductDefinitionShape), "dummy product_definition_shape wrapper should not survive final RDF")
}

func TestStepEntityOntologyNodeSelectsExpectedOntologyClass(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)
	cases := []struct {
		stepName    string
		expected    sst.Elementer
		unexpected  sst.Elementer
		description string
	}{
		{"FLATNESS_TOLERANCE", sso.FlatnessTolerance, nil, "non-REP ontology lookup"},
		{"MODEL_GEOMETRIC_VIEW", rep.ModelGeometricView, nil, "duplicate entity map keeps specific main class"},
		{"SURFACE", rep.Surface, rep.ElementarySurface, "superclass wins when subtype adds mapped attributes"},
	}

	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			node := cp.stepEntityOntologyNode(tc.stepName)

			require.NotNil(t, node)
			assert.True(t, node.Is(tc.expected))
			if tc.unexpected != nil {
				assert.False(t, node.Is(tc.unexpected))
			}
		})
	}
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

func TestEnumerationElementUsesOntologyEnumerationMaps(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)
	cases := []struct {
		attribute sst.Elementer
		enumName  string
		expected  sst.Element
	}{
		{rep.KnotSpec, "PIECEWISE_BEZIER_KNOTS", rep.KnotType_PiecewiseBezierKnots.Element},
		{rep.SurfaceStyleUsage_side, "BOTH", rep.SurfaceSide_both.Element},
	}

	for _, tc := range cases {
		element, ok := cp.enumerationElement(vocabularyNode(t, graph, tc.attribute), tc.enumName)

		require.True(t, ok)
		assert.Equal(t, tc.expected, element)
	}
}

func TestParseTypesQAUQuantitiesByMeasureShape(t *testing.T) {
	graph := parseAS1EC214Fixture(t)

	var quantityCount int
	for _, quantityType := range []sst.Elementer{qau.Area, qau.Length, qau.Volume} {
		for _, quantity := range allNodesOfType(t, graph, quantityType) {
			quantityCount++
			if hasObject(quantity, rdf.Type, rep.MeasureRepresentationItem) ||
				hasObject(quantity, rdf.Type, rep.UncertaintyMeasureWithUnit) {
				assert.False(t, hasObject(quantity, rdf.Type, lci.PhysicalQuantity), "%s should use its QAU class as the quantity main type", quantity.PrefixedFragment())
				continue
			}
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

func TestParseCollapsesMetadataFreeRepresentationRelationshipProperties(t *testing.T) {
	graph := parseAS1EC214Fixture(t)

	var invalidPredicates []string
	var propertyDefinitionRepresentationUses int
	var shapeDefinitionRepresentationUses int

	require.NoError(t, graph.ForIRINodes(func(node sst.IBNode) error {
		node.ForAll(func(_ int, s, p sst.IBNode, _ sst.Term) error {
			if node == s && p.Is(rep.Representation) {
				invalidPredicates = append(invalidPredicates, s.PrefixedFragment())
			}
			if node == s && p.Is(sso.PropertyDefinitionRepresentation) {
				propertyDefinitionRepresentationUses++
			}
			if node == s && p.Is(sso.ShapeDefinitionRepresentation) {
				shapeDefinitionRepresentationUses++
			}
			return nil
		})

		return nil
	}))

	assert.Empty(t, invalidPredicates, "rep:Representation is a class and must not be used as a predicate")
	assert.Greater(t, propertyDefinitionRepresentationUses, 0, "fixture should contain direct property_definition_representation mappings")
	assert.Greater(t, shapeDefinitionRepresentationUses, 0, "fixture should contain direct shape_definition_representation mappings")
}

func TestParseDoesNotDuplicateDefiningGeometryWithShapeDefinitionPunning(t *testing.T) {
	graph := parseAS1EC214Fixture(t)

	var duplicates []string
	require.NoError(t, graph.ForIRINodes(func(node sst.IBNode) error {
		for _, geometry := range node.GetObjects(sso.DefiningGeometry) {
			node.ForAll(func(_ int, _ sst.IBNode, predicate sst.IBNode, object sst.Term) error {
				if predicate.InVocabulary() != nil || !hasObject(predicate, rdfs.SubPropertyOf, sso.ShapeDefinitionRepresentation) {
					return nil
				}
				if object == geometry {
					duplicates = append(duplicates, node.PrefixedFragment())
				}
				return nil
			})
		}
		return nil
	}))

	assert.Empty(t, duplicates, "sso:definingGeometry already represents shape_definition_representation and should not be duplicated by a punned predicate")
}

func TestProcessPropertyDefinitionRepresentationSkipsUnresolvedRawOwner(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)
	propertyDefinitionRepresentation := graph.CreateIRINode("property-definition-representation")
	propertyDefinition := graph.CreateIRINode("property-definition")
	rawWrapper := graph.CreateIRINode("raw-wrapper")
	representation := graph.CreateIRINode("representation")

	cp.rawAttributeValuesMap[propertyDefinitionRepresentation] = RawAttributeValues{
		MixedValues: []interface{}{propertyDefinition, representation},
	}
	cp.rawAttributeValuesMap[propertyDefinition] = RawAttributeValues{
		name: PROPERTY_DEFINITION,
		MixedValues: []interface{}{
			sst.String("validation property"),
			sst.String(""),
			rawWrapper,
		},
	}
	cp.rawAttributeValuesMap[rawWrapper] = RawAttributeValues{name: "UNSUPPORTED_WRAPPER"}

	entity := cp.ontologyEntity(PROPERTY_DEFINITION_REPRESENTATION)
	require.NotNil(t, entity.ontologyObject)

	cp.processPropertyDefinitionRepresentation(propertyDefinitionRepresentation, entity)

	assert.True(t, hasObject(propertyDefinitionRepresentation, rdf.Type, owl.ObjectProperty))
	assert.False(t, rawWrapper.CheckTriple(propertyDefinitionRepresentation, representation), "unresolved raw parser wrappers must not become final assertion subjects")
}

func TestProcessPropertyDefinitionRepresentationKeepsResolvedOwner(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)
	propertyDefinitionRepresentation := graph.CreateIRINode("property-definition-representation")
	propertyDefinition := graph.CreateIRINode("property-definition")
	productDefinition := graph.CreateIRINode("product-definition")
	representation := graph.CreateIRINode("representation")

	cp.rawAttributeValuesMap[propertyDefinitionRepresentation] = RawAttributeValues{
		MixedValues: []interface{}{propertyDefinition, representation},
	}
	cp.rawAttributeValuesMap[propertyDefinition] = RawAttributeValues{
		name: PROPERTY_DEFINITION,
		MixedValues: []interface{}{
			sst.String("validation property"),
			sst.String(""),
			productDefinition,
		},
	}
	cp.rawAttributeValuesMap[productDefinition] = RawAttributeValues{name: PRODUCT_DEFINITION}

	entity := cp.ontologyEntity(PROPERTY_DEFINITION_REPRESENTATION)
	require.NotNil(t, entity.ontologyObject)

	cp.processPropertyDefinitionRepresentation(propertyDefinitionRepresentation, entity)

	assert.True(t, productDefinition.CheckTriple(propertyDefinitionRepresentation, representation))
}

func TestCollapseSingleUseObjectPropertyPunningReplacesWithBaseProperty(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)
	owner := graph.CreateIRINode("owner")
	representation := graph.CreateIRINode("representation")
	property := graph.CreateIRINode("generated-relationship")
	propertyFragment := property.Fragment()

	property.AddStatement(rdf.Type, owl.ObjectProperty)
	property.AddStatement(rdfs.SubPropertyOf, rep.ShapeRepresentationRelationship)
	owner.AddStatement(property, representation)

	baseProperty, ok := metadataFreeObjectPropertyBase(property)
	require.True(t, ok)
	assert.True(t, baseProperty.Is(rep.ShapeRepresentationRelationship))
	cp.collapseSingleUseObjectPropertyPunning(graph)

	assert.True(t, owner.CheckTriple(rep.ShapeRepresentationRelationship, representation))

	var turtle bytes.Buffer
	require.NoError(t, graph.RdfWrite(&turtle, sst.RdfFormatTurtle))
	assert.NotContains(t, turtle.String(), propertyFragment)
}

func TestCollapseSingleUseObjectPropertyPunningKeepsMetadataProperty(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)
	owner := graph.CreateIRINode("owner")
	representation := graph.CreateIRINode("representation")
	property := graph.CreateIRINode("property-definition-representation")

	property.AddStatement(rdf.Type, owl.ObjectProperty)
	property.AddStatement(rdfs.SubPropertyOf, sso.PropertyDefinitionRepresentation)
	property.AddStatement(rdfs.Label, sst.String("validation property"))
	owner.AddStatement(property, representation)

	cp.collapseSingleUseObjectPropertyPunning(graph)

	assert.False(t, owner.CheckTriple(sso.PropertyDefinitionRepresentation, representation))
	assert.True(t, owner.CheckTriple(property, representation))
	assert.True(t, hasObject(property, rdf.Type, owl.ObjectProperty))
}

func TestCollapseSingleUseObjectPropertyPunningDoesNotMaterializeSelectTypes(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)
	owner := graph.CreateIRINode("owner", sso.ShapeElement)
	callout := graph.CreateIRINode("callout", rep.DraughtingCallout)
	property := graph.CreateIRINode("generated-draughting-model-item-usage")

	property.AddStatement(rdf.Type, owl.ObjectProperty)
	property.AddStatement(rdfs.SubPropertyOf, sso.DraughtingModelItemUsage)
	owner.AddStatement(property, callout)

	cp.collapseSingleUseObjectPropertyPunning(graph)

	assert.True(t, owner.CheckTriple(sso.DraughtingModelItemUsage, callout))
	assert.False(t, hasObject(owner, rdf.Type, sso.DraughtingModelItemDefinition), "select-domain classes are ontology constraints, not final instance types")
	assert.False(t, hasObject(callout, rdf.Type, sso.DraughtingModelItemAssociationSelect), "select-range classes are ontology constraints, not final instance types")
}

func TestDraughtingModelItemAssociationWithPlaceholderKeepsUsedRepresentation(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)
	association := graph.CreateIRINode("draughting-association-with-placeholder")
	definition := graph.CreateIRINode("definition", sso.ShapeElement)
	representation := graph.CreateIRINode("draughting-model", rep.DraughtingModel)
	callout := graph.CreateIRINode("callout", rep.DraughtingCallout)
	placeholder := graph.CreateIRINode("placeholder", rep.AnnotationPlaceholderOccurrence)

	cp.rawAttributeValuesMap[association] = RawAttributeValues{
		name: "DRAUGHTING_MODEL_ITEM_ASSOCIATION_WITH_PLACEHOLDER",
		MixedValues: []interface{}{
			sst.String("PMI representation to presentation link"),
			sst.String(""),
			definition,
			representation,
			callout,
			placeholder,
		},
	}
	entity := cp.ontologyEntity("DRAUGHTING_MODEL_ITEM_ASSOCIATION_WITH_PLACEHOLDER")
	require.NotNil(t, entity.ontologyObject)

	cp.mapOntologyObjectPropertyInstance(association, entity)

	assert.True(t, definition.CheckTriple(association, callout))
	assert.Contains(t, association.GetObjects(sso.ItemIdentifiedRepresentationUsage), representation)
	assert.Contains(t, association.GetObjects(sso.DraughtingModelItemUsage_annotation_placeholder), placeholder)
	assert.True(t, hasObject(association, rdfs.SubPropertyOf, sso.DraughtingModelItemAssociationWithPlaceholder))
}

func TestParseConsumesOrientedEdgeWrappersFromFinalGraph(t *testing.T) {
	graph := parseAS1EC214Fixture(t)

	assert.Empty(t, nodesOfType(t, graph, rep.OrientedEdge), "rep:OrientedEdge is ssmeta:DummyStepIrClass and should be consumed into edgeList/orientationList")
}

func TestParseRemovesConsumedOrientedEdgeRawAttributeCollections(t *testing.T) {
	graph := parseAS1EC214Fixture(t)
	var turtle bytes.Buffer

	require.NoError(t, graph.RdfWrite(&turtle, sst.RdfFormatTurtle))
	assert.NotContains(t, turtle.String(), "rdf:first \"\" ;\n  rdf:rest ( ssmeta:derivedValue ssmeta:derivedValue :", "consumed oriented-edge parser attribute collections should not leak into final RDF")
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

func TestApplyMappedAttributesMapsSSTLiteralsIntoStatements(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)
	node := graph.CreateIRINode("subject")

	cp.applyMappedAttributes(
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

func TestApplyMappedAttributesIgnoresExtraRawValuesWithoutMappedOntologyAttributes(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)
	node := graph.CreateIRINode("subject")
	extraCollection := graph.CreateCollection(graph.CreateIRINode("extra"))

	require.NotPanics(t, func() {
		cp.applyMappedAttributes(
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

func TestApplyMappedAttributesDoesNotEmitSpecialAttributeMarkers(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)
	node := graph.CreateIRINode("subject")
	specialValue := graph.CreateCollection(graph.CreateIRINode("qualifier"))

	cp.applyMappedAttributes(
		node,
		[]sst.IBNode{vocabularyNode(t, graph, ssmeta.StepImMapAttributeSpecial)},
		[]interface{}{specialValue},
	)

	assert.Empty(t, node.GetObjects(ssmeta.StepImMapAttributeSpecial), "special mapping markers should be consumed, not emitted as predicates")
}

func TestAssignComplexOntologyTypesDoesNotEmitOptionClassAsFinalType(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)
	node := graph.CreateIRINode("complex")
	repGraph := graph.Stage().CreateNamedGraph(sst.IRI(rep.REPVocabulary.BaseIRI))
	optionClass := repGraph.CreateIRINode("MeasureRepresentationItem", ssmeta.OptionClass)
	cp.collectComplexNodes[node] = []sst.IBNode{optionClass}

	cp.assignComplexOntologyTypes()

	assert.False(t, hasObject(node, rdf.Type, rep.MeasureRepresentationItem), "option classes should not survive as final rdf:type")
}

func TestApplyPDMIdentityTextMapsSSTStringLiterals(t *testing.T) {
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

	cp.applyPDMIdentityText(node, "")

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

func TestConvertProductDefinitionArrangesDefaultModelGeometricView(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)
	formation := graph.CreateIRINode("formation")
	definition := graph.CreateIRINode("definition")
	productDefinitionShape := graph.CreateIRINode("product-definition-shape")
	defaultView := graph.CreateIRINode("default-model-geometric-view", sso.DefaultModelGeometricView)

	cp.rawAttributeValuesMap[definition] = RawAttributeValues{name: PRODUCT_DEFINITION}
	cp.rawAttributeValuesMap[productDefinitionShape] = RawAttributeValues{
		name:        PRODUCT_DEFINITION_SHAPE,
		MixedValues: []interface{}{sst.String(""), sst.String(""), definition},
	}
	cp.rawAttributeValuesMap[defaultView] = RawAttributeValues{
		name:        "DEFAULT_MODEL_GEOMETRIC_VIEW",
		MixedValues: []interface{}{sst.String("view"), productDefinitionShape},
	}

	cp.convertProductDefinition(formation, definition)

	assert.Contains(t, definition.GetObjects(lci.HasArrangedPart), defaultView, "PartDesign should own default model views through the product_definition_shape wrapper")
	assert.Empty(t, defaultView.GetObjects(lci.ArrangedPartOf), "final RDF should keep parent-to-child arranged-part navigation")
}

func TestReplaceArrangedPartOfWithHasArrangedPartOnlyKeepsIndividualChildren(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)
	parent := graph.CreateIRINode("part-design", sso.PartDesign)
	shapeElement := graph.CreateIRINode("shape-element", sso.ShapeElement)
	propertyDefinition := graph.CreateIRINode("property-definition", sso.PropertyDefinition)

	shapeElement.AddStatement(lci.ArrangedPartOf, parent)
	propertyDefinition.AddStatement(lci.ArrangedPartOf, parent)

	cp.replaceArrangedPartOfWithHasArrangedPart(graph)

	assert.Contains(t, parent.GetObjects(lci.HasArrangedPart), shapeElement)
	assert.NotContains(t, parent.GetObjects(lci.HasArrangedPart), propertyDefinition, "property definitions are not arranged parts")
	assert.Empty(t, shapeElement.GetObjects(lci.ArrangedPartOf), "final RDF should keep parent-to-child arranged-part navigation")
	assert.Empty(t, propertyDefinition.GetObjects(lci.ArrangedPartOf), "invalid arrangedPartOf mappings should be consumed, not inverted")
}

func TestMapDraughtingModelItemAssociationUsesInheritedItemUsageShape(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)
	association := graph.CreateIRINode("draughting-association")
	definition := graph.CreateIRINode("definition", sso.ShapeElement)
	representation := graph.CreateIRINode("draughting-model", rep.DraughtingModel)
	callout := graph.CreateIRINode("callout", rep.DraughtingCallout)

	cp.rawAttributeValuesMap[association] = RawAttributeValues{
		name: DRAUGHTING_MODEL_ITEM_ASSOCIATION,
		MixedValues: []interface{}{
			sst.String(""),
			sst.String(""),
			definition,
			representation,
			callout,
		},
	}
	entity := cp.ontologyEntity(DRAUGHTING_MODEL_ITEM_ASSOCIATION)
	require.NotNil(t, entity.ontologyObject)

	cp.mapOntologyObjectPropertyInstance(association, entity)
	cp.collapseSingleUseObjectPropertyPunning(graph)

	assert.True(t, definition.CheckTriple(association, callout))
	assert.Contains(t, association.GetObjects(sso.ItemIdentifiedRepresentationUsage), representation)
	assert.True(t, hasObject(association, rdfs.SubPropertyOf, sso.DraughtingModelItemUsage))
}

func TestMapDraughtingModelItemAssociationWithPlaceholderKeepsPlaceholderMetadata(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)
	association := graph.CreateIRINode("draughting-association-with-placeholder")
	definition := graph.CreateIRINode("definition", sso.ShapeElement)
	representation := graph.CreateIRINode("draughting-model", rep.DraughtingModel)
	callout := graph.CreateIRINode("callout", rep.DraughtingCallout)
	placeholder := graph.CreateIRINode("placeholder", rep.AnnotationPlaceholderOccurrence)

	cp.rawAttributeValuesMap[association] = RawAttributeValues{
		name: DRAUGHTING_MODEL_ITEM_ASSOCIATION_WITH_PLACEHOLDER,
		MixedValues: []interface{}{
			sst.String(""),
			sst.String(""),
			definition,
			representation,
			callout,
			placeholder,
		},
	}
	entity := cp.ontologyEntity(DRAUGHTING_MODEL_ITEM_ASSOCIATION_WITH_PLACEHOLDER)
	require.NotNil(t, entity.ontologyObject)

	cp.mapOntologyObjectPropertyInstance(association, entity)
	cp.collapseSingleUseObjectPropertyPunning(graph)

	assert.True(t, definition.CheckTriple(association, callout), "placeholder metadata requires the punned property to remain")
	assert.True(t, hasObject(association, rdfs.SubPropertyOf, sso.DraughtingModelItemAssociationWithPlaceholder))
	assert.Contains(t, association.GetObjects(sso.DraughtingModelItemUsage_annotation_placeholder), placeholder)
}

func TestMapMeasureQualificationAppliesValueFormatQualifierToMeasure(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)
	qualification := graph.CreateIRINode("measure-qualification")
	measure := graph.CreateIRINode("measure")
	lengthMeasure := graph.CreateIRINode("length-measure")
	unit := graph.CreateIRINode("unit")
	lengthUnit := graph.CreateIRINode("length-unit")
	namedUnit := graph.CreateIRINode("named-unit")
	siUnit := graph.CreateIRINode("si-unit")
	milli := graph.CreateIRINode("milli")
	metre := graph.CreateIRINode("metre")
	qualifier := graph.CreateIRINode("value-format")

	cp.rawAttributeValuesMap[qualification] = RawAttributeValues{
		name: MEASURE_QUALIFICATION,
		MixedValues: []interface{}{
			sst.String(""),
			sst.String(""),
			measure,
			graph.CreateCollection(qualifier),
		},
	}
	cp.rawAttributeValuesMap[measure] = RawAttributeValues{
		name:        MEASURE_WITH_UNIT,
		MixedValues: []interface{}{graph.CreateCollection(lengthMeasure, sst.Double(-0.1)), unit},
	}
	cp.definedTypeMap[lengthMeasure] = DefinedType{ExpressObject: ExpressObject{name: "LENGTH_MEASURE"}}
	cp.rawAttributeValuesMap[lengthUnit] = RawAttributeValues{name: LENGTH_UNIT}
	cp.rawAttributeValuesMap[namedUnit] = RawAttributeValues{name: NAMED_UNIT}
	cp.rawAttributeValuesMap[siUnit] = RawAttributeValues{name: SI_UNIT, MixedValues: []interface{}{milli, metre}}
	cp.enumerationValueMap[milli] = EnumerationValue{ExpressObject: ExpressObject{name: "MILLI"}}
	cp.enumerationValueMap[metre] = EnumerationValue{ExpressObject: ExpressObject{name: "METRE"}}
	cp.complexInstanceValues[unit] = []sst.IBNode{lengthUnit, namedUnit, siUnit}
	cp.rawAttributeValuesMap[qualifier] = RawAttributeValues{
		name:        VALUE_FORMAT_TYPE_QUALIFIER,
		MixedValues: []interface{}{sst.String("NR2 1.2")},
	}

	entity := cp.ontologyEntity(MEASURE_QUALIFICATION)
	require.NotNil(t, entity.ontologyObject)

	cp.mapOntologyObjectPropertyInstance(qualification, entity)

	assert.True(t, hasObject(measure, rdf.Type, lci.PhysicalQuantity))
	assert.True(t, hasObject(measure, rdf.Type, qau.Length))
	assert.False(t, hasObject(measure, rdf.Type, rep.ValueQualifierSubjects))
	assert.Contains(t, measure.GetObjects(qau.MilliMetre), sst.Double(-0.1))
	assert.Contains(t, measure.GetObjects(rep.ValueFormatTypeQualifier), sst.String("NR2 1.2"))
}

func TestQualifiedRepresentationItemAppliesValueFormatQualifier(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)
	representationItem := graph.CreateIRINode("representation-item", rep.RepresentationItem)
	qualifiedRepresentationItem := graph.CreateIRINode("qualified-representation-item")
	qualifier := graph.CreateIRINode("value-format")

	cp.rawAttributeValuesMap[qualifiedRepresentationItem] = RawAttributeValues{
		name:        QUALIFIED_REPRESENTATION_ITEM,
		MixedValues: []interface{}{graph.CreateCollection(qualifier)},
	}
	cp.rawAttributeValuesMap[qualifier] = RawAttributeValues{
		name:        VALUE_FORMAT_TYPE_QUALIFIER,
		MixedValues: []interface{}{sst.String("NR2 3.1")},
	}
	cp.singleEntityMap[qualifiedRepresentationItem] = cp.ontologyEntity(QUALIFIED_REPRESENTATION_ITEM)

	cp.mapComplexOntologyMember(representationItem, qualifiedRepresentationItem)

	assert.False(t, hasObject(representationItem, rdf.Type, rep.ValueQualifierSubjects))
	assert.Contains(t, representationItem.GetObjects(rep.ValueFormatTypeQualifier), sst.String("NR2 3.1"))
}

func TestComplexMeasureRepresentationItemUsesSameQuantityNode(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)
	measureItem := graph.CreateIRINode("measure-item")
	measureWithUnit := graph.CreateIRINode("measure-with-unit")
	measureRepresentationItem := graph.CreateIRINode("measure-representation-item")
	representationItem := graph.CreateIRINode("representation-item")
	qualifiedRepresentationItem := graph.CreateIRINode("qualified-representation-item")
	lengthMeasure := graph.CreateIRINode("length-measure")
	unit := graph.CreateIRINode("unit")
	lengthUnit := graph.CreateIRINode("length-unit")
	namedUnit := graph.CreateIRINode("named-unit")
	siUnit := graph.CreateIRINode("si-unit")
	milli := graph.CreateIRINode("milli")
	metre := graph.CreateIRINode("metre")
	qualifier := graph.CreateIRINode("value-format")

	cp.rawAttributeValuesMap[measureWithUnit] = RawAttributeValues{
		name:        MEASURE_WITH_UNIT,
		MixedValues: []interface{}{graph.CreateCollection(lengthMeasure, sst.Double(0.05)), unit},
	}
	cp.rawAttributeValuesMap[measureRepresentationItem] = RawAttributeValues{name: MEASURE_REPRESENTATION_ITEM}
	cp.rawAttributeValuesMap[representationItem] = RawAttributeValues{
		name:        "REPRESENTATION_ITEM",
		MixedValues: []interface{}{sst.String("nominal value")},
	}
	cp.rawAttributeValuesMap[qualifiedRepresentationItem] = RawAttributeValues{
		name:        QUALIFIED_REPRESENTATION_ITEM,
		MixedValues: []interface{}{graph.CreateCollection(qualifier)},
	}
	cp.rawAttributeValuesMap[qualifier] = RawAttributeValues{
		name:        VALUE_FORMAT_TYPE_QUALIFIER,
		MixedValues: []interface{}{sst.String("NR2 1.2")},
	}
	cp.definedTypeMap[lengthMeasure] = DefinedType{ExpressObject: ExpressObject{name: "POSITIVE_LENGTH_MEASURE"}}
	cp.rawAttributeValuesMap[lengthUnit] = RawAttributeValues{name: LENGTH_UNIT}
	cp.rawAttributeValuesMap[namedUnit] = RawAttributeValues{name: NAMED_UNIT}
	cp.rawAttributeValuesMap[siUnit] = RawAttributeValues{name: SI_UNIT, MixedValues: []interface{}{milli, metre}}
	cp.enumerationValueMap[milli] = EnumerationValue{ExpressObject: ExpressObject{name: "MILLI"}}
	cp.enumerationValueMap[metre] = EnumerationValue{ExpressObject: ExpressObject{name: "METRE"}}
	cp.complexInstanceValues[unit] = []sst.IBNode{lengthUnit, namedUnit, siUnit}
	cp.singleEntityMap[measureRepresentationItem] = cp.ontologyEntity(MEASURE_REPRESENTATION_ITEM)
	cp.singleEntityMap[representationItem] = cp.ontologyEntity("REPRESENTATION_ITEM")
	cp.singleEntityMap[qualifiedRepresentationItem] = cp.ontologyEntity(QUALIFIED_REPRESENTATION_ITEM)

	cp.mapComplexOntologyMember(measureItem, measureWithUnit)
	cp.mapComplexOntologyMember(measureItem, measureRepresentationItem)
	cp.mapComplexOntologyMember(measureItem, qualifiedRepresentationItem)
	cp.mapComplexOntologyMember(measureItem, representationItem)
	cp.assignComplexOntologyTypes()

	assert.True(t, hasObject(measureItem, rdf.Type, qau.Length))
	assert.True(t, hasObject(measureItem, rdf.Type, rep.MeasureRepresentationItem))
	assert.False(t, hasObject(measureItem, rdf.Type, rep.ValueQualifierSubjects))
	assert.False(t, hasObject(measureItem, rdf.Type, rep.QualifiedRepresentationItem))
	assert.False(t, hasObject(measureItem, rdf.Type, rep.RepresentationItem))
	assert.False(t, hasObject(measureItem, rdf.Type, lci.PhysicalQuantity))
	assert.Empty(t, measureItem.GetObjects(rep.MeasureValue))
	assert.Contains(t, measureItem.GetObjects(qau.MilliMetre), sst.Double(0.05))
	assert.Contains(t, measureItem.GetObjects(rep.ValueFormatTypeQualifier), sst.String("NR2 1.2"))
	assert.Contains(t, measureItem.GetObjects(rdfs.Label), sst.String("nominal value"))
}

func TestHandleUncertaintyMeasureWithUnitAddsUncertaintyType(t *testing.T) {
	graph := newTestGraph(t)
	cp := newConversionParameters(graph)
	uncertainty := graph.CreateIRINode("uncertainty")
	lengthMeasure := graph.CreateIRINode("length-measure")
	unit := graph.CreateIRINode("unit")
	lengthUnit := graph.CreateIRINode("length-unit")
	namedUnit := graph.CreateIRINode("named-unit")
	siUnit := graph.CreateIRINode("si-unit")
	milli := graph.CreateIRINode("milli")
	metre := graph.CreateIRINode("metre")

	cp.rawAttributeValuesMap[uncertainty] = RawAttributeValues{
		name: UNCERTAINTY_MEASURE_WITH_UNIT,
		MixedValues: []interface{}{
			graph.CreateCollection(lengthMeasure, sst.Double(0.0508)),
			unit,
			sst.String("distance_accuracy_value"),
			sst.String("confusion accuracy"),
		},
	}
	cp.definedTypeMap[lengthMeasure] = DefinedType{ExpressObject: ExpressObject{name: "LENGTH_MEASURE"}}
	cp.rawAttributeValuesMap[lengthUnit] = RawAttributeValues{name: LENGTH_UNIT}
	cp.rawAttributeValuesMap[namedUnit] = RawAttributeValues{name: NAMED_UNIT}
	cp.rawAttributeValuesMap[siUnit] = RawAttributeValues{name: SI_UNIT, MixedValues: []interface{}{milli, metre}}
	cp.enumerationValueMap[milli] = EnumerationValue{ExpressObject: ExpressObject{name: "MILLI"}}
	cp.enumerationValueMap[metre] = EnumerationValue{ExpressObject: ExpressObject{name: "METRE"}}
	cp.complexInstanceValues[unit] = []sst.IBNode{lengthUnit, namedUnit, siUnit}

	cp.handleUncertaintyMeasureWithUnit(uncertainty)

	assert.True(t, hasObject(uncertainty, rdf.Type, rep.UncertaintyMeasureWithUnit))
	assert.False(t, hasObject(uncertainty, rdf.Type, lci.PhysicalQuantity))
	assert.True(t, hasObject(uncertainty, rdf.Type, qau.Length))
	assert.Contains(t, uncertainty.GetObjects(qau.MilliMetre), sst.Double(0.0508))
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

	assert.ElementsMatch(t, []sst.IBNode{directOwner, collectionOwner}, cp.rawNodesReferencing(target))
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

func collectionMembers(t testing.TB, term sst.Term) []sst.Term {
	t.Helper()

	if literalCollection, ok := term.(sst.LiteralCollection); ok {
		members := make([]sst.Term, 0, literalCollection.MemberCount())
		literalCollection.ForMembers(func(_ int, literal sst.Literal) {
			members = append(members, literal)
		})
		return members
	}

	node, ok := term.(sst.IBNode)
	require.True(t, ok)
	collection, ok := node.AsCollection()
	require.True(t, ok)
	return collection.Members()
}

func assertLengthMeasureNode(t testing.TB, term sst.Term, expectedValue float64) {
	t.Helper()

	node, ok := term.(sst.IBNode)
	require.True(t, ok)
	assert.True(t, hasObject(node, rdf.Type, lci.PhysicalQuantity))
	assert.True(t, hasObject(node, rdf.Type, qau.Length))
	assert.Contains(t, node.GetObjects(qau.MilliMetre), sst.Double(expectedValue))
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
