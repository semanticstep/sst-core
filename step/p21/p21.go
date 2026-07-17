// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

// import STEP file in p21/stp format containing AP242 data (ISO 10303-242) into SST
package p21

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/semanticstep/sst-core/sst"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/owl"
	"github.com/semanticstep/sst-core/vocabularies/qau"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/semanticstep/sst-core/vocabularies/ssmeta"
	"github.com/semanticstep/sst-core/vocabularies/sso"
)

// Install ebnf2y with the following command
// > go install modernc.org/ebnf2y@latest
//go:generate -command ebnf2y $GOPATH/bin/ebnf2y

// Install goyacc with the following command
// > go install golang.org/x/tools/cmd/goyacc@latest
//go:generate -command goyacc $GOPATH/bin/goyacc

// Install golex with the following command
// > go install modernc.org/golex@latest
//go:generate -command golex $GOPATH/bin/golex

//-go:generate ebnf2y -start EXCHANGE_FILE -o iso10303p21y2016-template.y iso10303p21y2016.ebnf
//go:generate golex -o iso10303p21y2016lexer.go iso10303p21y2016.l
//go:generate goyacc -l -o iso10303p21y2016parser.go iso10303p21y2016.y

// Parser state used by the yacc/lexer raw Part 21 import.
type parserInstanceMode int

const (
	parserNoInstanceMode parserInstanceMode = iota
	parserSimpleEntityInstance
	parserComplexEntityInstance
)

var ErrInvalidPosition = errors.New("invalid position")

type parserKeywordType int

const (
	parserKeywordNoType parserKeywordType = iota
	parserEntity
	parserDefinedType
)

type sstData struct {
	graph                sst.NamedGraph
	instanceMap          map[int64]sst.IBNode
	entityMap            map[string]sst.IBNode
	definedTypeMap       map[string]sst.IBNode
	enumerationValueMap  map[string]sst.IBNode
	instanceMode         parserInstanceMode
	instanceValue        sst.IBNode
	instanceActual       sst.IBNode
	entityActual         sst.IBNode
	entityValueActual    sst.IBNode
	definedTypeActual    sst.IBNode
	definedTypeNesting   int
	stringValue          string // also standard_keyword, enumeration, logical
	realValue            float64
	integerValue         int64
	booleanValue         bool
	attributeValues      []sst.Term
	attributeValuesStack [][]sst.Term
	keywordType          parserKeywordType
}

func (d *sstData) pushAttributeValues() {
	d.attributeValuesStack = append(d.attributeValuesStack, d.attributeValues)
	d.attributeValues = nil
}

func (d *sstData) popAttributeValues() []sst.Term {
	v := d.attributeValues
	d.attributeValues = d.attributeValuesStack[len(d.attributeValuesStack)-1]
	d.attributeValuesStack = d.attributeValuesStack[:len(d.attributeValuesStack)-1]
	return v
}

func newSSTData(graph sst.NamedGraph) *sstData {
	return &sstData{
		graph:               graph,
		instanceMap:         map[int64]sst.IBNode{},
		entityMap:           map[string]sst.IBNode{},
		definedTypeMap:      map[string]sst.IBNode{},
		enumerationValueMap: map[string]sst.IBNode{},
	}
}

var ErrParsingFailed = errors.New("parsing failed")

type ErrorReporter interface {
	Println(v ...interface{})
}

type (
	OntologyObjectType int
	OntologyType       int
)

const (
	EntityType OntologyObjectType = iota
	EnumerationValueType
	DefinedTypeType
)

const (
	UnknownOntologyType OntologyType = iota
	MainClass
	ObjectProperty
)

const (
	PART                                               = "part"
	SUPER                                              = "SUPER"
	RANGE                                              = "range"
	CUBIC                                              = "cubic"
	SQUARE                                             = "square"
	DOMAIN                                             = "domain"
	DESIGN                                             = "design"
	PRODUCT                                            = "PRODUCT"
	SI_UNIT                                            = "SI_UNIT"
	DERIVED_UNIT                                       = "DERIVED_UNIT"
	DERIVED_UNIT_ELEMENT                               = "DERIVED_UNIT_ELEMENT"
	LENGTH_UNIT                                        = "LENGTH_UNIT"
	VERSION                                            = "VERSION"
	EDGE_LOOP                                          = "EDGE_LOOP"
	ATTRIBUTE                                          = "ATTRIBUTE"
	NAMED_UNIT                                         = "NAMED_UNIT"
	INTEGER_REPRESENTATION_ITEM                        = "INTEGER_REPRESENTATION_ITEM"
	PLANE_ANGLE_UNIT                                   = "PLANE_ANGLE_UNIT"
	SOLID_ANGLE_UNIT                                   = "SOLID_ANGLE_UNIT"
	SHAPE_ASPECT                                       = "SHAPE_ASPECT"
	PROPERTY_DEFINITION                                = "PROPERTY_DEFINITION"
	PRODUCT_DEFINITION                                 = "PRODUCT_DEFINITION"
	SHAPE_REPRESENTATION                               = "SHAPE_REPRESENTATION"
	PRODUCT_DEFINITION_SHAPE                           = "PRODUCT_DEFINITION_SHAPE"
	PRODUCT_DEFINITION_CONTEXT                         = "PRODUCT_DEFINITION_CONTEXT"
	REPRESENTATION_RELATIONSHIP                        = "REPRESENTATION_RELATIONSHIP"
	MEASURE_WITH_UNIT                                  = "MEASURE_WITH_UNIT"
	MEASURE_QUALIFICATION                              = "MEASURE_QUALIFICATION"
	MEASURE_REPRESENTATION_ITEM                        = "MEASURE_REPRESENTATION_ITEM"
	ITEM_DEFINED_TRANSFORMATION                        = "ITEM_DEFINED_TRANSFORMATION"
	GLOBAL_UNIT_ASSIGNED_CONTEXT                       = "GLOBAL_UNIT_ASSIGNED_CONTEXT"
	UNCERTAINTY_MEASURE_WITH_UNIT                      = "UNCERTAINTY_MEASURE_WITH_UNIT"
	VALUE_FORMAT_TYPE_QUALIFIER                        = "VALUE_FORMAT_TYPE_QUALIFIER"
	QUALIFIED_REPRESENTATION_ITEM                      = "QUALIFIED_REPRESENTATION_ITEM"
	PRODUCT_DEFINITION_FORMATION                       = "PRODUCT_DEFINITION_FORMATION"
	NEXT_ASSEMBLY_USAGE_OCCURRENCE                     = "NEXT_ASSEMBLY_USAGE_OCCURRENCE"
	SHAPE_DEFINITION_REPRESENTATION                    = "SHAPE_DEFINITION_REPRESENTATION"
	GLOBAL_UNCERTAINTY_ASSIGNED_CONTEXT                = "GLOBAL_UNCERTAINTY_ASSIGNED_CONTEXT"
	PRODUCT_RELATED_PRODUCT_CATEGORY                   = "PRODUCT_RELATED_PRODUCT_CATEGORY"
	SHAPE_REPRESENTATION_RELATIONSHIP                  = "SHAPE_REPRESENTATION_RELATIONSHIP"
	PROPERTY_DEFINITION_REPRESENTATION                 = "PROPERTY_DEFINITION_REPRESENTATION"
	DRAUGHTING_MODEL_ITEM_ASSOCIATION                  = "DRAUGHTING_MODEL_ITEM_ASSOCIATION"
	DRAUGHTING_MODEL_ITEM_ASSOCIATION_WITH_PLACEHOLDER = "DRAUGHTING_MODEL_ITEM_ASSOCIATION_WITH_PLACEHOLDER"
	CONTEXT_DEPENDENT_SHAPE_REPRESENTATION             = "CONTEXT_DEPENDENT_SHAPE_REPRESENTATION"
	REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION    = "REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION"
	PRODUCT_DEFINITION_FORMATION_WITH_SPECIFIED_SOURCE = "PRODUCT_DEFINITION_FORMATION_WITH_SPECIFIED_SOURCE"
)

var p21MeasureTypeMap = map[string]sst.ElementInformer{
	"mass_measure":                      qau.Mass,
	"area_measure":                      qau.Area,
	"time_measure":                      qau.Time,
	"force_measure":                     qau.Force,
	"plane_angle_measure":               qau.Angle,
	"positive_plane_angle_measure":      qau.Angle,
	"power_measure":                     qau.Power,
	"volume_measure":                    qau.Volume,
	"energy_measure":                    qau.Energy,
	"length_measure":                    qau.Length,
	"non_negative_length_measure":       qau.Length,
	"positive_length_measure":           qau.Length,
	"radioactivity_measure":             qau.Activity,
	"velocity_measure":                  qau.Velocity,
	"frequency_measure":                 qau.Frequency,
	"solid_angle_measure":               qau.SolidAngle,
	"resistance_measure":                qau.Resistance,
	"inductance_measure":                qau.Inductance,
	"illuminance_measure":               qau.Illuminance,
	"luminous_flux_measure":             qau.LuminousFlux,
	"conductance_measure":               qau.Conductance,
	"magnetic_flux_measure":             qau.MagneticFlux,
	"capacitance_measure":               qau.Capacitance,
	"acceleration_measure":              qau.Acceleration,
	"absorbed_dose_measure":             qau.AbsorbedDose,
	"count_measure":                     qau.CountQuantity,
	"pressure_measure":                  qau.StaticPressure,
	"electric_charge_measure":           qau.ElectricCharge,
	"dose_equivalent_measure":           qau.DoseEquivalent,
	"electric_current_measure":          qau.ElectricCurrent,
	"luminous_intensity_measure":        qau.LuminousIntensity,
	"electric_potential_measure":        qau.ElectricPotential,
	"amount_of_substance_measure":       qau.AmountOfSubstance,
	"celsius_temperature_measure":       qau.ThermodynamicTemperature,
	"magnetic_flux_density_measure":     qau.MagneticFluxDensity,
	"thermodynamic_temperature_measure": qau.ThermodynamicTemperature,
	"ratio_measure":                     lci.NumericQuantityValue,
	"parameter_value":                   lci.NumericQuantityValue,
	"numeric_measure":                   lci.NumericQuantityValue,
	"positive_ratio_measure":            lci.NumericQuantityValue,
}

type measureWithUnit struct {
	measureType  string
	measureUnit  string
	measureValue float64
	hasValue     bool
}

type ExpressObject struct {
	name       string
	objectType OntologyObjectType
}

type DefinedType struct {
	ExpressObject
}

type EnumerationValue struct {
	ExpressObject
}

type RawAttributeValues struct {
	name        string
	MixedValues []interface{}
}

type SingleEntity struct {
	name            string
	superType       bool
	ontologyType    OntologyType
	ontologyObject  sst.IBNode
	attributeOrders []sst.IBNode
}
type ExtraEntity struct {
	name           string
	ontologyObject sst.IBNode
}

// TreeNode stores a term plus nested collection members.
type TreeNode struct {
	Value    sst.Term
	Children []*TreeNode
}

// Conversion state shared by the post-parse passes.
type conversionParameters struct {
	graph                 sst.NamedGraph                    // graph being converted in place
	singleEntityMap       map[sst.IBNode]SingleEntity       // raw STEP entity node -> ontology mapping metadata
	extraEntityMap        map[sst.IBNode]ExtraEntity        // raw extra entity node -> parser-only metadata
	enumerationValueMap   map[sst.IBNode]EnumerationValue   // raw enum node -> STEP enum name
	definedTypeMap        map[sst.IBNode]DefinedType        // raw defined-type node -> STEP defined type metadata
	rawAttributeValuesMap map[sst.IBNode]RawAttributeValues // raw node -> ordered Part 21 attribute values
	complexInstanceValues map[sst.IBNode][]sst.IBNode       // complex instance node -> member entity value nodes
	singleInstanceValues  map[sst.IBNode][]sst.IBNode       // entity instance node -> single entity value nodes
	collectComplexNodes   map[sst.IBNode][]sst.IBNode       // complex instance node -> ontology classes to type after merge
	enumerationCache      map[string]sst.IBNode             // normalized STEP enum name -> raw enum node
	enumerationElementMap map[string]sst.Element            // ontology range + enum name -> vocabulary element
	stepEntityMap         map[string]sst.IBNode             // normalized STEP entity name -> ontology node
	ontologyEntityCache   map[string]SingleEntity           // normalized STEP entity name -> ontology metadata
	rawReferenceIndex     map[sst.IBNode][]sst.IBNode       // raw referenced node -> raw owners
	singleOccurrenceMap   map[sst.IBNode]sst.IBNode         // assembly occurrence relationship -> sso:SingleOccurrence node
	consumedIRNodes       map[sst.IBNode]bool               // temporary IR nodes removed after normalization
}

// newConversionParameters initializes per-import indexes and conversion caches.
func newConversionParameters(graph sst.NamedGraph) *conversionParameters {
	cp := new(conversionParameters)
	cp.graph = graph
	cp.singleEntityMap = make(map[sst.IBNode]SingleEntity)
	cp.extraEntityMap = make(map[sst.IBNode]ExtraEntity)
	cp.enumerationValueMap = make(map[sst.IBNode]EnumerationValue)
	cp.definedTypeMap = make(map[sst.IBNode]DefinedType)
	cp.rawAttributeValuesMap = make(map[sst.IBNode]RawAttributeValues)
	cp.complexInstanceValues = make(map[sst.IBNode][]sst.IBNode)
	cp.singleInstanceValues = make(map[sst.IBNode][]sst.IBNode)
	cp.collectComplexNodes = make(map[sst.IBNode][]sst.IBNode)
	cp.enumerationCache = make(map[string]sst.IBNode)
	cp.enumerationElementMap = make(map[string]sst.Element)
	cp.ontologyEntityCache = make(map[string]SingleEntity)
	cp.singleOccurrenceMap = make(map[sst.IBNode]sst.IBNode)
	cp.consumedIRNodes = make(map[sst.IBNode]bool)

	return cp
}

func dumpParserResult(parserResult parserResultT) {
	s := fmt.Sprintf("%#v", parserResult)
	var buf bytes.Buffer
	subS := s
	var indent int
	for {
		i := strings.IndexAny(subS, "{}")
		if i < 0 {
			break
		}
		buf.WriteString(subS[0 : i+1])
		switch subS[i] {
		case '{':
			indent += 2
			buf.WriteByte('\n')
			for j := 0; j < indent; j++ {
				buf.WriteByte(' ')
			}
		case '}':
			indent -= 2
			buf.WriteByte('\n')
			for j := 0; j < indent; j++ {
				buf.WriteByte(' ')
			}
		}
		subS = subS[i+1:]
	}
	buf.WriteString("\n")
	a := strings.Split(buf.String(), "\n")
	for _, v := range a {
		if strings.HasSuffix(v, "(nil)") || strings.HasSuffix(v, "(nil),") {
			continue
		}

		fmt.Println(v)
	}
}

// Parse pipeline.
// The converter keeps raw ssmeta import, ontology mapping, semantic lifting,
// and cleanup as separate passes so temporary IR does not leak into final RDF.
// Parse imports a Part 21 stream and returns the converted SST graph.
func Parse(src *bufio.Reader, errorReporter ErrorReporter) (graph sst.NamedGraph, err error) {
	// Pass 1: parse Part 21 into traceable ssmeta entity/value nodes.
	graph, err = ParseRaw(src, errorReporter)
	if err != nil {
		return graph, err
	}

	if graph == nil {
		return graph, fmt.Errorf("graph is nil")
	}

	cp := newConversionParameters(graph)
	if cp == nil {
		return graph, fmt.Errorf("conversionParameters is nil")
	}

	// Pass 2: cache ontology mapping rules and raw Part 21 attribute values.
	// extractMetaDataFromP21Dataset indexes raw parser nodes and ontology entity metadata.
	cp.extractMetaDataFromP21Dataset(graph)
	// extractAttributeValues records each STEP instance's ordered attribute values and references.
	cp.extractAttributeValues(graph)

	// Pass 3: apply ssmeta mappings into draft ontology triples.
	// mapSingleEntitiesToDraftOntology converts simple STEP instances using ssmeta entity maps.
	cp.mapSingleEntitiesToDraftOntology()
	// mapComplexEntitiesToDraftOntology merges complex STEP instance members into one draft node.
	cp.mapComplexEntitiesToDraftOntology()

	// Pass 4: choose final ontology types for merged complex instances.
	// assignComplexOntologyTypes keeps the most specific final classes from complex member types.
	cp.assignComplexOntologyTypes()

	// Pass 5: lift known product/assembly patterns into SST semantic nodes.
	// mapExplicitSSTSemantics adds semantic Part, PartVersion, PartDesign, and occurrence links.
	cp.mapExplicitSSTSemantics()
	// replaceArrangedPartOfWithHasArrangedPart stores arranged-part navigation on the parent node.
	cp.replaceArrangedPartOfWithHasArrangedPart(graph)

	// Pass 6: remove parser scaffolding and consumed draft IR.
	// removeEntityInstance deletes raw ssmeta entity-instance wrappers.
	cp.removeEntityInstance(graph)
	// removeAttributeValues deletes raw ordered attribute-value lists after mapped triples exist.
	cp.removeAttributeValues(graph)
	// removeConsumedIRNodes deletes dummy nodes consumed by normalization rules.
	cp.removeConsumedIRNodes()
	// removeRemainingDummyIRNodes removes leftover dummy IR nodes or their dummy-only type triples.
	cp.removeRemainingDummyIRNodes(graph)
	// removeParserDefinedTypes deletes raw defined-type metadata that should not appear in final RDF.
	cp.removeParserDefinedTypes(graph)
	// collapseSingleUseObjectPropertyPunning removes one-off predicates after raw scaffolding is gone.
	cp.collapseSingleUseObjectPropertyPunning(graph)
	// removeConstantAngleGlobalUnits omits canonical angle scales from the final context output.
	cp.removeConstantAngleGlobalUnits(graph)
	// removeDisconnectedNodes drops parser helper nodes left with no useful graph connections.
	cp.removeDisconnectedNodes(graph)

	return graph, nil
}

// Pass 1: raw Part 21 import.
// ParseRaw imports a Part 21 stream into raw ssmeta parser nodes without SST conversion.
func ParseRaw(src *bufio.Reader, errorReporter ErrorReporter) (graph sst.NamedGraph, err error) {
	stage := sst.OpenStage(sst.DefaultTriplexMode)
	if stage == nil {
		return nil, fmt.Errorf("stage is nil after OpenStage")
	}

	graph = stage.CreateNamedGraph("")

	if graph == nil {
		return nil, fmt.Errorf("graph is nil after CreateNamedGraph")
	}

	if err != nil {
		return graph, err
	}

	l := newLexer(src, newSSTData(graph))
	if l == nil {
		return graph, fmt.Errorf("lexer is nil")
	}

	// yyDebug = 4
	yyErrorVerbose = true
	defer func() {
		if e := recover(); e != nil {
			if e, ok := e.(error); ok {
				err = e
				return
			}
			err = fmt.Errorf("p21.Parse failed: %v", e)
		}
	}()
	if y := yyParse(l); y != 0 {
		for _, err := range l.errs {
			errorReporter.Println(err)
		}
		return graph, ErrParsingFailed
	}

	return graph, nil
}

// Pass 2: metadata indexes.
// These helpers cache raw parser metadata and ontology mapping declarations.
// extractMetaDataFromP21Dataset scans the raw graph once and caches STEP entity,
// enumeration, and defined-type metadata used by later conversion passes.
func (cp *conversionParameters) extractMetaDataFromP21Dataset(graph sst.NamedGraph) {
	graph.ForIRINodes(func(node sst.IBNode) error {
		typeOf := node.TypeOf()
		if typeOf == nil {
			return nil
		}
		inVocab := typeOf.InVocabulary()
		if inVocab == nil {
			return nil
		}
		switch inVocab.(type) {
		case ssmeta.IsEntity:
			entityName := cp.nodeName(node)
			singleEntity := cp.ontologyEntity(entityName)
			if singleEntity.ontologyObject != nil {
				cp.singleEntityMap[node] = singleEntity
			} else {
				cp.extraEntityMap[node] = ExtraEntity{
					name:           entityName,
					ontologyObject: node,
				}
			}
		case ssmeta.IsSingleEntityValue:
			parentNode := node.GetObjects(ssmeta.SingleEntityValueType)
			for _, parent := range parentNode {
				parentNode, ok := parent.(sst.IBNode)
				if !ok {
					continue
				}
				cp.singleEntityMap[node] = cp.ontologyEntity(cp.nodeName(parentNode))
			}
		case ssmeta.IsEnumerationValue:
			ev := EnumerationValue{
				ExpressObject: ExpressObject{
					name:       cp.nodeName(node),
					objectType: EnumerationValueType,
				},
			}
			cp.enumerationValueMap[node] = ev
		case ssmeta.IsDefinedType:
			dt := DefinedType{
				ExpressObject: ExpressObject{
					name:       cp.nodeName(node),
					objectType: DefinedTypeType,
				},
			}
			cp.definedTypeMap[node] = dt
		}
		return nil
	})
}

// extractNode reads ontology mapping metadata for one STEP entity class.
// It records class/property kind, inherited attribute order, and explicit supertype order.
func (cp *conversionParameters) extractNode(node sst.IBNode, singleEntity *SingleEntity, orderType string) {
	var ownAttributeOrders []sst.IBNode
	var superNodes []sst.IBNode
	var orderedSuperNodes []sst.IBNode

	node.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if node != s {
			return nil
		}

		if o.TermKind() == sst.TermKindIBNode {
			if o.(sst.IBNode).Is(ssmeta.MainClass) {
				singleEntity.ontologyType = MainClass
			}
			if o.(sst.IBNode).Is(owl.ObjectProperty) {
				singleEntity.ontologyType = ObjectProperty
			}
		}

		if p.Is(ssmeta.StepImMapAttributeOrder) {
			ownAttributeOrders = cp.extractStepImMapAttributeOrder(o.(sst.IBNode))
		}
		if p.Is(ssmeta.StepImMapSupertypeOrder) {
			orderedSuperNodes = cp.extractStepImMapAttributeOrder(o.(sst.IBNode))
		}
		if p.Is(rdfs.SubClassOf) && !o.(sst.IBNode).Is(lci.Individual) {
			superNodes = append(superNodes, o.(sst.IBNode))
		}
		if p.Is(rdfs.SubPropertyOf) {
			superNodes = append(superNodes, o.(sst.IBNode))
		}
		return nil
	})

	if len(orderedSuperNodes) > 0 {
		superNodes = orderedSuperNodes
	}

	if len(superNodes) > 0 {
		singleEntity.superType = true
	}
	for _, superNode := range superNodes {
		cp.extractNode(superNode, singleEntity, SUPER)
	}
	cp.appendAttributeOrders(singleEntity, ownAttributeOrders)

	if orderType == SUPER && len(ownAttributeOrders) > 0 {
		singleEntity.superType = true
	}
}

func (cp *conversionParameters) extractStepImMapAttributeOrder(node sst.IBNode) []sst.IBNode {
	var attributeOrders []sst.IBNode
	node.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if node != s {
			return nil
		}

		if p.Is(rdf.First) && o.TermKind() == sst.TermKindIBNode {
			if o.(sst.IBNode).Fragment() != "" && o.(sst.IBNode).Fragment() != "nil" {
				attributeOrders = append(attributeOrders, o.(sst.IBNode))
			}
		}
		if p.Is(rdf.Rest) {
			if rest, ok := o.(sst.IBNode); ok && !rest.Is(rdf.Nil) {
				attributeOrders = append(attributeOrders, cp.extractStepImMapAttributeOrder(rest)...)
			}
		}
		return nil
	})

	return attributeOrders
}

func (cp *conversionParameters) extractStepImMapSupertypeOrder(node sst.IBNode) []sst.IBNode {
	return cp.extractStepImMapAttributeOrder(node)
}

func (cp *conversionParameters) appendAttributeOrders(singleEntity *SingleEntity, attributeOrders []sst.IBNode) {
	for _, attributeOrder := range attributeOrders {
		if !slices.Contains(singleEntity.attributeOrders, attributeOrder) {
			singleEntity.attributeOrders = append(singleEntity.attributeOrders, attributeOrder)
		}
	}
}

func (cp *conversionParameters) nodeName(node sst.IBNode) string {
	var name string
	node.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if o.TermKind() == sst.TermKindLiteral {
			name = string(o.(sst.String))
		}
		return nil
	})
	return name
}

func (cp *conversionParameters) ontologyEntity(name string) SingleEntity {
	key := strings.ToUpper(name)
	if cached, ok := cp.ontologyEntityCache[key]; ok {
		cached.name = name
		return cached
	}

	singleEntity := SingleEntity{name: name}
	nodeFound := cp.stepEntityOntologyNode(name)
	if nodeFound == nil {
		return singleEntity
	}

	singleEntity.ontologyObject = nodeFound
	cp.extractNode(nodeFound, &singleEntity, "")
	cp.ontologyEntityCache[key] = singleEntity
	return singleEntity
}

func (cp *conversionParameters) stepEntityOntologyNode(name string) sst.IBNode {
	if cp.stepEntityMap == nil {
		cp.stepEntityMap = cp.loadStepEntityMap()
	}

	return cp.stepEntityMap[strings.ToUpper(name)]
}

func (cp *conversionParameters) loadStepEntityMap() map[string]sst.IBNode {
	entityMap := make(map[string]sst.IBNode)

	addGraphEntityMaps := func(graph sst.NamedGraph, overwrite bool) {
		if graph == nil {
			return
		}

		graph.ForIRINodes(func(node sst.IBNode) error {
			node.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
				if !p.Is(ssmeta.StepImEntityMap) || o.TermKind() != sst.TermKindLiteral {
					return nil
				}

				keyValue := strings.ToUpper(string(o.(sst.String)))
				if keyValue == "" {
					return nil
				}
				existing, exists := entityMap[keyValue]
				if !exists || cp.shouldReplaceStepEntityMapNode(existing, s, overwrite) {
					entityMap[keyValue] = s
				}
				return nil
			})
			return nil
		})
	}

	dictionary := sst.StaticDictionary()
	repGraph, _ := dictionary.Vocabulary(rep.REPVocabulary)
	addGraphEntityMaps(repGraph, true)
	dictionary.ForNamedGraphs(func(graph sst.NamedGraph) error {
		addGraphEntityMaps(graph, false)
		return nil
	})

	return entityMap
}

func (cp *conversionParameters) shouldReplaceStepEntityMapNode(existing, candidate sst.IBNode, overwrite bool) bool {
	if stepEntityMapNodeIsMoreSpecific(candidate, existing) {
		return !cp.stepEntityMapNodeAddsOwnAttributes(candidate)
	}
	if stepEntityMapNodeIsMoreSpecific(existing, candidate) {
		return cp.stepEntityMapNodeAddsOwnAttributes(existing)
	}
	return overwrite
}

func (cp *conversionParameters) stepEntityMapNodeAddsOwnAttributes(node sst.IBNode) bool {
	addsAttributes := false
	node.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if addsAttributes || s != node || !p.Is(ssmeta.StepImMapAttributeOrder) {
			return nil
		}
		if list, ok := o.(sst.IBNode); ok {
			addsAttributes = len(cp.extractStepImMapAttributeOrder(list)) > 0
		}
		return nil
	})
	return addsAttributes
}

func stepEntityMapNodeIsMoreSpecific(candidate, existing sst.IBNode) bool {
	candidateInfo := candidate.InVocabulary()
	existingInfo := existing.InVocabulary()
	if candidateInfo == nil || existingInfo == nil {
		return false
	}

	candidateElement := candidateInfo.VocabularyElement()
	existingElement := existingInfo.VocabularyElement()
	if candidateElement == existingElement {
		return false
	}
	if candidateInfo.IsMainClass(existingElement) {
		return true
	}
	if candidateInfo.IsProperty() && existingInfo.IsProperty() {
		return vocabularyPropertyIsSubPropertyOf(candidateInfo, existingElement, map[sst.Element]bool{})
	}
	return false
}

func vocabularyPropertyIsSubPropertyOf(candidate sst.ElementInformer, existing sst.Element, seen map[sst.Element]bool) bool {
	if candidate == nil {
		return false
	}
	candidateElement := candidate.VocabularyElement()
	if seen[candidateElement] {
		return false
	}
	seen[candidateElement] = true

	parent := candidate.SubPropertyOf()
	if parent == nil {
		return false
	}
	if parent.VocabularyElement() == existing {
		return true
	}
	return vocabularyPropertyIsSubPropertyOf(parent, existing, seen)
}

func (cp *conversionParameters) enumerationElement(attribute sst.IBNode, name string) (sst.Element, bool) {
	inVocabulary := attribute.InVocabulary()
	if inVocabulary == nil || inVocabulary.Range() == nil {
		return sst.Element{}, false
	}

	rangeElement := inVocabulary.Range().VocabularyElement()
	key := rangeElement.Vocabulary.BaseIRI + "#" + rangeElement.Name + "\x00" + normalizeStepName(name)
	if element, ok := cp.enumerationElementMap[key]; ok {
		return element, true
	}

	var found sst.Element
	sst.StaticDictionary().ForNamedGraphs(func(graph sst.NamedGraph) error {
		if found != (sst.Element{}) {
			return nil
		}
		graph.ForIRINodes(func(candidate sst.IBNode) error {
			if found != (sst.Element{}) || !nodeHasType(candidate, rangeElement) {
				return nil
			}
			candidate.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
				if found != (sst.Element{}) || s != candidate || !p.Is(ssmeta.StepImEnumerationMap) {
					return nil
				}
				if literal, ok := o.(sst.String); ok && stepEnumerationNamesMatch(string(literal), name) {
					found = candidate.InVocabulary().VocabularyElement()
				}
				return nil
			})
			return nil
		})
		return nil
	})

	if found == (sst.Element{}) {
		return sst.Element{}, false
	}
	cp.enumerationElementMap[key] = found
	return found, true
}

// Pass 2 support: raw attribute value indexing.
// This records each STEP instance's ordered attributes and reference graph.
func (cp *conversionParameters) extractAttributeValues(graph sst.NamedGraph) {
	graph.ForIRINodes(func(node sst.IBNode) error {
		node.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
			if p.Is(ssmeta.AttributeValues) {
				var rawAttributeValues RawAttributeValues
				rawAttributeValues = cp.processCollection(o.(sst.IBNode), rawAttributeValues)
				cp.rawAttributeValuesMap[s] = rawAttributeValues
			}
			if p.Is(ssmeta.EntityInstanceType) {
				cp.addUnique(cp.singleInstanceValues, s, o.(sst.IBNode))
			}
			if p.Is(ssmeta.ComplexInstanceValue) {
				cp.addUnique(cp.complexInstanceValues, s, o.(sst.IBNode))
			}
			return nil
		})

		// Keep the STEP entity name beside its ordered raw values.
		node.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
			if o.TermKind() == sst.TermKindIBNode {
				if cp.singleEntityMap[s].name != "" {
					assignValue := cp.rawAttributeValuesMap[s]
					assignValue.name = cp.singleEntityMap[s].name
					cp.rawAttributeValuesMap[s] = assignValue
				} else if cp.singleEntityMap[o.(sst.IBNode)].name != "" {
					assignValue := cp.rawAttributeValuesMap[s]
					assignValue.name = cp.singleEntityMap[o.(sst.IBNode)].name
					cp.rawAttributeValuesMap[s] = assignValue
				} else if cp.extraEntityMap[s].name != "" {
					assignValue := cp.rawAttributeValuesMap[s]
					assignValue.name = cp.extraEntityMap[s].name
					cp.rawAttributeValuesMap[s] = assignValue
				} else if cp.extraEntityMap[o.(sst.IBNode)].name != "" {
					assignValue := cp.rawAttributeValuesMap[s]
					assignValue.name = cp.extraEntityMap[o.(sst.IBNode)].name
					cp.rawAttributeValuesMap[s] = assignValue
				}
			}
			return nil
		})
		return nil
	})
}

func (cp *conversionParameters) addUnique(values map[sst.IBNode][]sst.IBNode, key sst.IBNode, newValue sst.IBNode) {
	slice := values[key]
	if slices.Contains(slice, newValue) {
		return
	}
	values[key] = append(slice, newValue)
}

func (cp *conversionParameters) processCollection(node sst.IBNode, rawAttributeValues RawAttributeValues) RawAttributeValues {
	if literalCollection, ok := node.AsCollection(); ok {
		literalCollection.ForMembers(func(_ int, o sst.Term) {
			switch o.TermKind() {
			case sst.TermKindLiteral:
				rawAttributeValues.MixedValues = append(rawAttributeValues.MixedValues, o.(sst.Literal))
			case sst.TermKindIBNode, sst.TermKindTermCollection:
				rawAttributeValues.MixedValues = append(rawAttributeValues.MixedValues, o.(sst.IBNode))
			}
		})
	}
	return rawAttributeValues
}

// Unit and measure normalization.
// These helpers convert STEP unit/measure structures into QAU quantity nodes.
func (cp *conversionParameters) qauNodeByLabel(unitName string) sst.IBNode {
	if result, found := cp.enumerationCache[unitName]; found {
		return result
	}

	var result sst.IBNode
	Vocgraph, _ := sst.StaticDictionary().Vocabulary(qau.QAUVocabulary)
	Vocgraph.ForIRINodes(func(node sst.IBNode) error {
		node.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
			if p.Is(rdfs.Label) {
				var keyValue string
				if literal, ok := o.(sst.Literal); ok {
					switch l := literal.(type) {
					case sst.String:
						keyValue = string(l)
					case sst.LangString:
						keyValue = l.Val
					default:
						return nil
					}
				} else {
					return nil
				}
				if keyValue != "" && strings.EqualFold(unitName, keyValue) {
					result = s
					cp.enumerationCache[unitName] = result
				}
			}
			return nil
		})
		return nil
	})
	return result
}

func (cp *conversionParameters) qauElementForLabel(label string) (sst.Element, bool) {
	getReferenceNode := cp.qauNodeByLabel(strings.TrimSpace(label))
	if getReferenceNode == nil || !getReferenceNode.IsIRINode() {
		return sst.Element{}, false
	}
	return getReferenceNode.IRI().VocabularyElement(), true
}

func measureTypeElement(measureType string) (sst.Element, bool) {
	element, exists := p21MeasureTypeMap[strings.ToLower(strings.TrimSpace(measureType))]
	if !exists {
		return sst.Element{}, false
	}
	return element.VocabularyElement(), true
}

func addPhysicalQuantityType(node sst.IBNode, quantityType sst.Element) {
	if !node.CheckTriple(rdf.Type, quantityType) {
		node.AddStatement(rdf.Type, quantityType)
	}
	if !node.CheckTriple(rdf.Type, lci.PhysicalQuantity) {
		node.AddStatement(rdf.Type, lci.PhysicalQuantity)
	}
}

func (cp *conversionParameters) hasMappedMeasureQuantityType(node sst.IBNode) bool {
	for _, measureType := range p21MeasureTypeMap {
		if nodeHasTypeAssignableTo(node, measureType) {
			return true
		}
	}
	return false
}

func (cp *conversionParameters) handleGlobalUnitAssignedContext(parentNode sst.IBNode, node sst.IBNode) {
	cp.collectComplexOntologyClass(parentNode, node)

	rawAttrValues, exists := cp.rawAttributeValuesMap[node]
	if !exists || rawAttrValues.MixedValues == nil {
		return
	}
	for _, v := range rawAttrValues.MixedValues {
		if ibnodeVal, ok := v.(sst.IBNode); ok {
			if ibnodeCollection, ok := ibnodeVal.AsCollection(); ok {
				ibnodeCollection.ForMembers(func(_ int, o sst.Term) {
					if o.TermKind() == sst.TermKindIBNode || o.TermKind() == sst.TermKindTermCollection {
						if unitElement, ok := cp.globalUnitElement(o.(sst.IBNode)); ok {
							parentNode.AddStatement(rep.GlobalUnit, unitElement)
						}
					}
				})
			}
		}
	}
}

func (cp *conversionParameters) globalUnitElement(unitNode sst.IBNode) (sst.Element, bool) {
	if unitLabel := cp.structuralUnitName(unitNode); unitLabel != "" {
		return cp.qauElementForLabel(unitLabel)
	}

	for _, group := range cp.unitLabelPartGroups(unitNode) {
		if unitElement, ok := cp.qauElementForLabel(unitLabelFromParts(group)); ok {
			return unitElement, true
		}
	}

	return sst.Element{}, false
}

func (cp *conversionParameters) structuralUnitName(unitNode sst.IBNode) string {
	var hasUnitKind bool
	var unitLabel string

	for _, candidate := range cp.unitComponentNodes(unitNode) {
		rawAttrValues := cp.rawAttributeValuesMap[candidate]
		switch rawAttrValues.name {
		case LENGTH_UNIT, PLANE_ANGLE_UNIT, SOLID_ANGLE_UNIT:
			hasUnitKind = true
		case SI_UNIT:
			unitLabel = cp.siUnitName(rawAttrValues)
		}
	}

	if !hasUnitKind {
		return ""
	}
	return unitLabel
}

func (cp *conversionParameters) unitComponentNodes(unitNode sst.IBNode) []sst.IBNode {
	candidates := []sst.IBNode{unitNode}
	candidates = append(candidates, cp.complexInstanceValues[unitNode]...)

	if unitCollection, ok := unitNode.AsCollection(); ok {
		unitCollection.ForMembers(func(_ int, member sst.Term) {
			if memberNode, ok := member.(sst.IBNode); ok {
				candidates = append(candidates, memberNode)
				candidates = append(candidates, cp.complexInstanceValues[memberNode]...)
			}
		})
	}

	return candidates
}

func (cp *conversionParameters) unitLabelPartGroups(node sst.IBNode) [][]string {
	var groupedStrings [][]string

	node.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if o.TermKind() == sst.TermKindIBNode || o.TermKind() == sst.TermKindTermCollection {
			var currentGroup []string
			if len(cp.rawAttributeValuesMap[o.(sst.IBNode)].MixedValues) > 0 {
				for _, v := range cp.rawAttributeValuesMap[o.(sst.IBNode)].MixedValues {
					if ibnodeVal, ok := v.(sst.IBNode); ok {
						ibnodeVal.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
							if o.TermKind() == sst.TermKindLiteral {
								currentGroup = append(currentGroup, string(o.(sst.String)))
							}
							return nil
						})
					}
				}
			}
			if len(currentGroup) > 0 {
				groupedStrings = append(groupedStrings, currentGroup)
			}
		}
		return nil
	})

	return groupedStrings
}

func (cp *conversionParameters) handleGlobalUncertaintyAssignedContext(parentNode sst.IBNode, node sst.IBNode) {
	cp.collectComplexOntologyClass(parentNode, node)

	rawAttrValues, exists := cp.rawAttributeValuesMap[node]
	if !exists {
		return
	}

	for _, value := range rawAttrValues.MixedValues {
		uncertaintyNode, ok := value.(sst.IBNode)
		if !ok {
			continue
		}

		if uncertaintyCollection, ok := uncertaintyNode.AsCollection(); ok {
			uncertaintyCollection.ForMembers(func(_ int, o sst.Term) {
				node, ok := o.(sst.IBNode)
				if !ok || cp.isSkippedParameterNode(node) {
					return
				}
				cp.handleUncertaintyMeasureWithUnit(node)
				parentNode.AddStatement(rep.GlobalUncertainty, node)
			})
			continue
		}

		if cp.isSkippedParameterNode(uncertaintyNode) {
			continue
		}
		cp.handleUncertaintyMeasureWithUnit(uncertaintyNode)
		parentNode.AddStatement(rep.GlobalUncertainty, uncertaintyNode)
	}
}

func (cp *conversionParameters) handleUncertaintyMeasureWithUnit(node sst.IBNode) {
	rawAttrValues, exists := cp.rawAttributeValuesMap[node]
	if !exists || rawAttrValues.name != UNCERTAINTY_MEASURE_WITH_UNIT {
		return
	}

	addStatementIfMissing(node, rdf.Type, rep.UncertaintyMeasureWithUnit)
	measure := cp.extractMeasureWithUnit(node)
	includePhysicalQuantity := true
	if element, exists := measureTypeElement(measure.measureType); exists && element.Vocabulary.BaseIRI == qau.QAUVocabulary.BaseIRI {
		includePhysicalQuantity = false
	}
	cp.applyMeasureWithUnit(node, measure, includePhysicalQuantity)

	for i, value := range rawAttrValues.MixedValues {
		if i >= 2 {
			cp.applyLabelOrCommentAtPosition(node, value, i-2)
		}
	}
}

func (cp *conversionParameters) applyMeasureWithUnit(node sst.IBNode, measure measureWithUnit, includePhysicalQuantity bool) {
	element, exists := measureTypeElement(measure.measureType)
	if !exists {
		return
	}

	if includePhysicalQuantity {
		addPhysicalQuantityType(node, element)
	} else {
		addStatementIfMissing(node, rdf.Type, element)
	}

	if !measure.hasValue {
		return
	}

	if unitElement, ok := cp.qauElementForLabel(measure.measureUnit); ok {
		addStatementIfMissing(node, unitElement, sst.Double(measure.measureValue))
	}
}

func (cp *conversionParameters) extractMeasureWithUnit(node sst.IBNode) measureWithUnit {
	var measure measureWithUnit

	for _, value := range cp.rawAttributeValuesMap[node].MixedValues {
		ibnodeVal, ok := value.(sst.IBNode)
		if !ok {
			continue
		}

		if literalCollection, ok := ibnodeVal.AsCollection(); ok {
			hasMeasureLiteral := false
			literalCollection.ForMembers(func(_ int, o sst.Term) {
				switch v := o.(type) {
				case sst.Double:
					measure.measureValue = float64(v)
					measure.hasValue = true
					hasMeasureLiteral = true
				case sst.Integer:
					measure.measureValue = float64(v)
					measure.hasValue = true
					hasMeasureLiteral = true
				case sst.IBNode:
					if definedType, exists := cp.definedTypeMap[v]; exists {
						measure.measureType = strings.ToLower(definedType.name)
					}
				}
			})
			if !hasMeasureLiteral && measure.measureUnit == "" {
				measure.measureUnit = cp.measureUnitName(ibnodeVal)
			}
			continue
		}

		if cp.isSkippedParameterNode(ibnodeVal) {
			continue
		}
		if definedType, exists := cp.definedTypeMap[ibnodeVal]; exists {
			measure.measureType = strings.ToLower(definedType.name)
			continue
		}
		if measure.measureUnit == "" {
			measure.measureUnit = cp.measureUnitName(ibnodeVal)
		}
	}

	return measure
}

func (cp *conversionParameters) measureUnitName(unitNode sst.IBNode) string {
	if unitLabel := cp.derivedUnitName(unitNode); unitLabel != "" {
		return unitLabel
	}

	if unitLabel := cp.structuralUnitName(unitNode); unitLabel != "" {
		return unitLabel
	}

	for _, candidate := range cp.unitComponentNodes(unitNode) {
		rawAttrValues := cp.rawAttributeValuesMap[candidate]
		if rawAttrValues.name != SI_UNIT {
			continue
		}
		if unitLabel := cp.siUnitName(rawAttrValues); unitLabel != "" {
			return unitLabel
		}
	}

	for _, group := range cp.unitLabelPartGroups(unitNode) {
		if unitLabel := unitLabelFromParts(group); unitLabel != "" {
			return unitLabel
		}
	}
	return ""
}

func (cp *conversionParameters) derivedUnitName(unitNode sst.IBNode) string {
	rawAttrValues := cp.rawAttributeValuesMap[unitNode]
	if rawAttrValues.name != DERIVED_UNIT || len(rawAttrValues.MixedValues) == 0 {
		return ""
	}

	var elementNode sst.IBNode
	elementCount := 0
	for _, value := range rawAttrValues.MixedValues {
		node, ok := value.(sst.IBNode)
		if !ok {
			continue
		}
		if collection, ok := node.AsCollection(); ok {
			collection.ForMembers(func(_ int, member sst.Term) {
				if memberNode, ok := member.(sst.IBNode); ok {
					elementNode = memberNode
					elementCount++
				}
			})
			continue
		}
		elementNode = node
		elementCount++
	}
	if elementCount != 1 {
		return ""
	}

	rawAttrValues = cp.rawAttributeValuesMap[elementNode]
	if rawAttrValues.name != DERIVED_UNIT_ELEMENT || len(rawAttrValues.MixedValues) < 2 {
		return ""
	}

	baseUnitNode := cp.ibNodeAt(rawAttrValues.MixedValues, 0)
	if baseUnitNode == nil {
		return ""
	}
	baseUnitName := cp.measureUnitName(baseUnitNode)
	if baseUnitName == "" {
		return ""
	}

	exponent, ok := integerValue(rawAttrValues.MixedValues[1])
	if !ok || exponent < 1 {
		return ""
	}
	if exponent == 1 {
		return baseUnitName
	}
	switch exponent {
	case 2:
		return unitLabelFromParts([]string{SQUARE, baseUnitName})
	case 3:
		return unitLabelFromParts([]string{CUBIC, baseUnitName})
	}
	return ""
}

func (cp *conversionParameters) siUnitName(rawAttrValues RawAttributeValues) string {
	var parts []string
	for _, value := range rawAttrValues.MixedValues {
		part := cp.unitPartName(value)
		if part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, " ")
}

func (cp *conversionParameters) unitPartName(value interface{}) string {
	switch v := value.(type) {
	case string:
		return normalizeUnitPart(v)
	case sst.String:
		return normalizeUnitPart(string(v))
	case sst.IBNode:
		if cp.isSkippedParameterNode(v) {
			return ""
		}
		if enumerationValue, ok := cp.enumerationValueMap[v]; ok {
			return normalizeUnitPart(enumerationValue.name)
		}
		return normalizeUnitPart(cp.nodeName(v))
	}
	return ""
}

func unitLabelFromParts(parts []string) string {
	var normalizedParts []string
	for _, part := range parts {
		if normalizedPart := normalizeUnitPart(part); normalizedPart != "" {
			normalizedParts = append(normalizedParts, normalizedPart)
		}
	}
	return strings.Join(normalizedParts, " ")
}

func normalizeUnitPart(value string) string {
	value = strings.TrimSpace(strings.ToLower(strings.ReplaceAll(value, "_", " ")))
	if value == "*" || value == "$" {
		return ""
	}
	return value
}

func normalizeStepName(value string) string {
	return strings.ToLower(strings.Trim(strings.TrimSpace(value), "."))
}

func stepEnumerationNamesMatch(mappedName, valueName string) bool {
	mappedName = normalizeStepName(mappedName)
	valueName = normalizeStepName(valueName)
	return mappedName == valueName || strings.HasSuffix(mappedName, "."+valueName)
}

func (cp *conversionParameters) handleMeasureRepresentationItem(node sst.IBNode) {
	for _, value := range cp.rawAttributeValuesMap[node].MixedValues {
		if strVal, ok := value.(string); ok {
			if cp.isValid(strVal) {
				node.AddStatement(rdfs.Label, sst.String(strVal))
			}
		}
		if strVal, ok := value.(sst.String); ok {
			if cp.isValid(string(strVal)) {
				node.AddStatement(rdfs.Label, strVal)
			}
		}
	}

	cp.applyMeasureWithUnit(node, cp.extractMeasureWithUnit(node), false)
	if !cp.hasMappedMeasureQuantityType(node) {
		addStatementIfMissing(node, rdf.Type, rep.RepresentationItem)
	}
}

// Pass 3a: single-entity ontology mapping.
// mapSingleEntitiesToDraftOntology applies ssmeta mappings for non-complex STEP instances.
// Main classes become typed draft nodes; mapped object-property entities become predicates.
func (cp *conversionParameters) mapSingleEntitiesToDraftOntology() {
	for node, singleValue := range cp.singleInstanceValues {
		for _, instanceType := range singleValue {
			entity := cp.singleEntityMap[instanceType]
			if entity.ontologyObject == nil {
				continue
			}

			switch entity.ontologyType {
			case MainClass:
				cp.mapOntologyMainClassInstance(node, entity)
			case ObjectProperty:
				switch entity.name {
				case MEASURE_QUALIFICATION:
					cp.mapMeasureQualification(node)
				case PROPERTY_DEFINITION_REPRESENTATION, SHAPE_DEFINITION_REPRESENTATION:
					cp.processPropertyDefinitionRepresentation(node, entity)
				default:
					cp.mapOntologyObjectPropertyInstance(node, entity)
				}
			}
		}
	}
}

func (cp *conversionParameters) mapOntologyMainClassInstance(node sst.IBNode, entity SingleEntity) {
	node.AddStatement(rdf.Type, entity.ontologyObject.InVocabulary().VocabularyElement())
	if entity.name == MEASURE_REPRESENTATION_ITEM {
		cp.handleMeasureRepresentationItem(node)
		return
	}
	cp.applyMappedAttributes(node, entity.attributeOrders, cp.rawAttributeValuesMap[node].MixedValues)
}

// Pass 3b: complex-entity ontology mapping.
// mapComplexEntitiesToDraftOntology maps complex STEP instances by applying each
// member entity's ontology attributes to the shared complex-instance node.
func (cp *conversionParameters) mapComplexEntitiesToDraftOntology() {
	for node, complexValue := range cp.complexInstanceValues {
		for _, instanceType := range complexValue {
			cp.mapComplexOntologyMember(node, instanceType)
		}
	}
}

func (cp *conversionParameters) mapComplexOntologyMember(node sst.IBNode, instanceType sst.IBNode) {
	rawValues, exists := cp.rawAttributeValuesMap[instanceType]
	if !exists {
		return
	}

	if isMeasureWithUnitName(rawValues.name) {
		cp.applyMeasureWithUnit(node, cp.extractMeasureWithUnit(instanceType), false)
		return
	}

	entity := cp.singleEntityMap[instanceType]
	switch rawValues.name {
	case GLOBAL_UNIT_ASSIGNED_CONTEXT:
		cp.handleGlobalUnitAssignedContext(node, instanceType)
		return
	case GLOBAL_UNCERTAINTY_ASSIGNED_CONTEXT:
		cp.handleGlobalUncertaintyAssignedContext(node, instanceType)
		return
	case QUALIFIED_REPRESENTATION_ITEM:
		if entity.ontologyObject != nil {
			cp.collectComplexOntologyClass(node, instanceType)
		}
		if len(rawValues.MixedValues) > 0 {
			cp.applyValueQualifiers(node, rawValues.MixedValues[0])
		}
	case REPRESENTATION_RELATIONSHIP,
		SHAPE_REPRESENTATION_RELATIONSHIP,
		REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION:
		return
	}

	if entity.ontologyObject == nil {
		return
	}
	cp.collectComplexOntologyClass(node, instanceType)
	cp.applyMappedAttributes(node, entity.attributeOrders, rawValues.MixedValues)
}

func (cp *conversionParameters) collectComplexOntologyClass(node sst.IBNode, instanceType sst.IBNode) {
	ontologyObject := cp.singleEntityMap[instanceType].ontologyObject
	if ontologyObject == nil {
		return
	}
	if !slices.Contains(cp.collectComplexNodes[node], ontologyObject) {
		cp.collectComplexNodes[node] = append(cp.collectComplexNodes[node], ontologyObject)
	}
}

// processPropertyDefinitionRepresentation turns relationship entities into punned
// object properties and asserts them between resolved semantic owner and representation.
func (cp *conversionParameters) processPropertyDefinitionRepresentation(node sst.IBNode, entity SingleEntity) {
	ontologyObject := entity.ontologyObject
	if ontologyObject == nil {
		return
	}

	if entity.name == SHAPE_DEFINITION_REPRESENTATION && cp.shapeDefinitionRepresentationHasSemanticMapping(node) {
		return
	}

	inVocabulary := ontologyObject.InVocabulary()
	if inVocabulary == nil || !inVocabulary.IsProperty() {
		return
	}

	node.AddStatement(rdf.Type, owl.ObjectProperty)
	node.AddStatement(rdfs.SubPropertyOf, inVocabulary.VocabularyElement())

	mixedValues := cp.rawAttributeValuesMap[node].MixedValues
	if len(mixedValues) < 2 {
		return
	}

	owner := cp.semanticRepresentationOwner(mixedValues[0])
	representation, ok := mixedValues[1].(sst.IBNode)
	if owner == nil || !ok || cp.isSkippedParameterNode(representation) {
		return
	}
	if !owner.CheckTriple(node, representation) {
		owner.AddStatement(node, representation)
	}
}

func (cp *conversionParameters) shapeDefinitionRepresentationHasSemanticMapping(node sst.IBNode) bool {
	rawAttrValues, exists := cp.rawAttributeValuesMap[node]
	if !exists || len(rawAttrValues.MixedValues) < 2 {
		return false
	}

	definition := cp.ibNodeAt(rawAttrValues.MixedValues, 0)
	if definition == nil {
		return false
	}

	switch cp.rawAttributeValuesMap[definition].name {
	case PRODUCT_DEFINITION_SHAPE:
		// PRODUCT_DEFINITION_SHAPE + SHAPE_DEFINITION_REPRESENTATION becomes sso:definingGeometry.
		// TODO: route topology and auxiliaryGeometry here once their STEP constraints are finalized.
		return cp.ibNodeAt(rawAttrValues.MixedValues, 1) != nil
	}
	return false
}

func (cp *conversionParameters) mapOntologyObjectPropertyInstance(node sst.IBNode, entity SingleEntity) {
	ontologyObject := entity.ontologyObject
	if ontologyObject == nil {
		return
	}

	inVocabulary := ontologyObject.InVocabulary()
	if inVocabulary == nil || !inVocabulary.IsProperty() {
		return
	}
	if cp.rawAttributeValuesMap[node].name == MEASURE_QUALIFICATION {
		cp.mapMeasureQualification(node)
		return
	}
	if cp.mapItemIdentifiedRepresentationUsage(node, ontologyObject) {
		return
	}

	mixedValues := cp.rawAttributeValuesMap[node].MixedValues
	attributeOrders := entity.attributeOrders
	if len(attributeOrders) == 0 || len(attributeOrders) > len(mixedValues) {
		return
	}

	var subject sst.IBNode
	var object sst.IBNode
	var propertyAttributes []sst.IBNode
	var propertyValues []interface{}
	for i, attribute := range attributeOrders {
		value := mixedValues[i]
		switch {
		case attribute.Is(rdfs.Label):
			cp.applyLabelOrCommentAtPosition(node, value, 0)
		case attribute.Is(rdfs.Comment):
			cp.applyLabelOrCommentAtPosition(node, value, 1)
		case attribute.Is(rdfs.Domain):
			subject = cp.objectPropertyEndpoint(value)
		case attribute.Is(rdfs.Range):
			object = cp.objectPropertyEndpoint(value)
		case attribute.InVocabulary() != nil && attribute.InVocabulary().IsProperty():
			propertyAttributes = append(propertyAttributes, attribute)
			propertyValues = append(propertyValues, value)
		}
	}
	if subject == nil || object == nil {
		return
	}

	node.AddStatement(rdf.Type, owl.ObjectProperty)
	node.AddStatement(rdfs.SubPropertyOf, inVocabulary.VocabularyElement())
	cp.applyMappedAttributes(node, propertyAttributes, propertyValues)
	cp.applyMappedAttributes(node, cp.directStepAttributeMapAttributes(entity.name), mixedValues[len(attributeOrders):])
	if !subject.CheckTriple(node, object) {
		subject.AddStatement(node, object)
	}
}

func (cp *conversionParameters) mapItemIdentifiedRepresentationUsage(node sst.IBNode, property sst.IBNode) bool {
	if !propertyIsOrSubPropertyOf(property, sso.RepresentationItemUsage, map[sst.IBNode]bool{}) {
		return false
	}

	mixedValues := cp.rawAttributeValuesMap[node].MixedValues
	if len(mixedValues) < 5 {
		return false
	}

	subject := cp.objectPropertyEndpoint(mixedValues[2])
	object := cp.objectPropertyEndpoint(mixedValues[4])
	if subject == nil || object == nil {
		return false
	}

	node.AddStatement(rdf.Type, owl.ObjectProperty)
	node.AddStatement(rdfs.SubPropertyOf, property.InVocabulary().VocabularyElement())
	cp.applyLabelOrCommentAtPosition(node, mixedValues[0], 0)
	cp.applyLabelOrCommentAtPosition(node, mixedValues[1], 1)
	if usedRepresentation := cp.objectPropertyEndpoint(mixedValues[3]); usedRepresentation != nil {
		addStatementIfMissing(node, sso.ItemIdentifiedRepresentationUsage, usedRepresentation)
	}
	if len(mixedValues) > 5 {
		if placeholder := cp.objectPropertyEndpoint(mixedValues[5]); placeholder != nil {
			addStatementIfMissing(node, sso.DraughtingModelItemUsage_annotation_placeholder, placeholder)
		}
	}
	addStatementIfMissing(subject, node, object)
	return true
}

func propertyIsOrSubPropertyOf(node sst.IBNode, property sst.Elementer, seen map[sst.IBNode]bool) bool {
	if node == nil || seen[node] {
		return false
	}
	if node.Is(property) {
		return true
	}

	seen[node] = true
	for _, object := range node.GetObjects(rdfs.SubPropertyOf) {
		parent, ok := object.(sst.IBNode)
		if ok && propertyIsOrSubPropertyOf(parent, property, seen) {
			return true
		}
	}
	return false
}

func (cp *conversionParameters) mapMeasureQualification(node sst.IBNode) {
	rawAttrValues := cp.rawAttributeValuesMap[node]
	if len(rawAttrValues.MixedValues) < 4 {
		return
	}

	measureNode := cp.ibNodeAt(rawAttrValues.MixedValues, 2)
	if measureNode == nil {
		return
	}

	cp.applyMeasureWithUnit(measureNode, cp.extractMeasureWithUnit(measureNode), true)
	cp.applyLabelOrCommentAtPosition(measureNode, rawAttrValues.MixedValues[0], 0)
	cp.applyLabelOrCommentAtPosition(measureNode, rawAttrValues.MixedValues[1], 1)
	cp.applyValueQualifiers(measureNode, rawAttrValues.MixedValues[3])
}

func (cp *conversionParameters) applyValueQualifiers(node sst.IBNode, value interface{}) {
	qualifierNode, ok := value.(sst.IBNode)
	if !ok || cp.isSkippedParameterNode(qualifierNode) {
		return
	}

	if qualifierCollection, ok := qualifierNode.AsCollection(); ok {
		qualifierCollection.ForMembers(func(_ int, member sst.Term) {
			if memberNode, ok := member.(sst.IBNode); ok {
				cp.applyValueQualifierNode(node, memberNode)
			}
		})
		return
	}

	cp.applyValueQualifierNode(node, qualifierNode)
}

func (cp *conversionParameters) applyValueQualifierNode(node sst.IBNode, qualifier sst.IBNode) {
	rawAttrValues, exists := cp.rawAttributeValuesMap[qualifier]
	if !exists || rawAttrValues.name != VALUE_FORMAT_TYPE_QUALIFIER || len(rawAttrValues.MixedValues) == 0 {
		return
	}

	text, ok := stringValue(rawAttrValues.MixedValues[0])
	if ok && cp.isValid(text) {
		addStatementIfMissing(node, rep.ValueFormatTypeQualifier, sst.String(text))
	}
}

func (cp *conversionParameters) directStepAttributeMapAttributes(entityName string) []sst.IBNode {
	prefix := strings.ToLower(entityName) + "."
	type mappedAttribute struct {
		key  string
		node sst.IBNode
	}

	var attributes []mappedAttribute
	seen := map[sst.IBNode]bool{}
	sst.StaticDictionary().ForNamedGraphs(func(graph sst.NamedGraph) error {
		graph.ForIRINodes(func(node sst.IBNode) error {
			node.ForAll(func(_ int, subject sst.IBNode, predicate sst.IBNode, object sst.Term) error {
				if subject != node || seen[node] || !predicate.Is(ssmeta.StepImAttributeMap) || object.TermKind() != sst.TermKindLiteral {
					return nil
				}
				key := strings.ToLower(string(object.(sst.String)))
				if strings.HasPrefix(key, prefix) {
					attributes = append(attributes, mappedAttribute{key: key, node: node})
					seen[node] = true
				}
				return nil
			})
			return nil
		})
		return nil
	})

	sort.Slice(attributes, func(i, j int) bool {
		return attributes[i].key < attributes[j].key
	})

	nodes := make([]sst.IBNode, 0, len(attributes))
	for _, attribute := range attributes {
		nodes = append(nodes, attribute.node)
	}
	return nodes
}

func (cp *conversionParameters) objectPropertyEndpoint(value interface{}) sst.IBNode {
	node, ok := value.(sst.IBNode)
	if !ok || cp.isSkippedParameterNode(node) {
		return nil
	}
	return cp.resolveDummyIRReference(node)
}

func (cp *conversionParameters) semanticRepresentationOwner(value interface{}) sst.IBNode {
	node, ok := value.(sst.IBNode)
	if !ok || cp.isSkippedParameterNode(node) {
		return nil
	}
	return cp.resolveSemanticRepresentationOwner(node, map[sst.IBNode]bool{})
}

func (cp *conversionParameters) resolveSemanticRepresentationOwner(node sst.IBNode, seen map[sst.IBNode]bool) sst.IBNode {
	if node == nil || cp.isSkippedParameterNode(node) || seen[node] {
		return nil
	}
	seen[node] = true

	rawAttrValues, exists := cp.rawAttributeValuesMap[node]
	if !exists {
		return node
	}

	switch rawAttrValues.name {
	case PRODUCT_DEFINITION_SHAPE:
		return cp.resolveSemanticRepresentationOwner(cp.resolveDummyIRReference(node), seen)
	case PROPERTY_DEFINITION:
		return cp.resolveSemanticRepresentationOwner(cp.resolveDummyIRReference(cp.ibNodeAt(rawAttrValues.MixedValues, 2)), seen)
	case SHAPE_ASPECT:
		return cp.resolveSemanticRepresentationOwner(cp.resolveDummyIRReference(cp.ibNodeAt(rawAttrValues.MixedValues, 2)), seen)
	case NEXT_ASSEMBLY_USAGE_OCCURRENCE:
		return cp.singleOccurrenceForNextOccurrence(node)
	case PRODUCT_DEFINITION:
		return node
	default:
		entity := cp.ontologyEntity(rawAttrValues.name)
		if entity.ontologyObject != nil && entity.ontologyType == MainClass {
			return node
		}
		return nil
	}
}

func (cp *conversionParameters) ibNodeAt(values []interface{}, index int) sst.IBNode {
	if index < 0 || index >= len(values) {
		return nil
	}
	node, ok := values[index].(sst.IBNode)
	if !ok || cp.isSkippedParameterNode(node) {
		return nil
	}
	return node
}

// Attribute mapping and dummy IR normalization.
// These helpers write mapped attribute triples and preserve dummy wrapper meaning.
func (cp *conversionParameters) applyMappedAttributes(node sst.IBNode, ontologyAttributes []sst.IBNode, mixedValues []interface{}) {
	if len(mixedValues) == 0 || len(ontologyAttributes) == 0 {
		return
	}

	if len(ontologyAttributes) > len(mixedValues) {
		offset := len(ontologyAttributes) - len(mixedValues)
		ontologyAttributes = ontologyAttributes[offset:]
	}

	for i, mixedVal := range mixedValues {
		if i >= len(ontologyAttributes) {
			continue
		}
		attribute := ontologyAttributes[i]
		if attribute.Is(ssmeta.StepImMapAttributeSpecial) {
			continue
		}
		switch v := mixedVal.(type) {
		case string:
			if cp.isValid(v) {
				node.AddStatement(attribute.InVocabulary().VocabularyElement(), sst.String(v))
			}
		case sst.String:
			if cp.isValid(string(v)) {
				node.AddStatement(attribute.InVocabulary().VocabularyElement(), v)
			}
		case sst.IBNode:
			if _, ok := v.AsCollection(); ok {
				cp.applyMappedCollectionAttribute(node, v, attribute)
			} else if !cp.isSkippedParameterNode(v) {
				cp.applyMappedNodeAttribute(node, v, attribute)
			}
		case float64:
			node.AddStatement(attribute.InVocabulary().VocabularyElement(), sst.Double(v))
		case sst.Double:
			node.AddStatement(attribute.InVocabulary().VocabularyElement(), v)
		case int64:
			node.AddStatement(attribute.InVocabulary().VocabularyElement(), sst.Double(v))
		case sst.Integer:
			node.AddStatement(attribute.InVocabulary().VocabularyElement(), v)
		}
	}
}

func (cp *conversionParameters) applyMappedNodeAttribute(node sst.IBNode, ibnodeVal sst.IBNode, attribute sst.IBNode) {
	if value, ok := cp.enumerationValueMap[ibnodeVal]; ok {
		if element, exists := cp.enumerationElement(attribute, value.name); exists {
			node.AddStatement(attribute.InVocabulary().VocabularyElement(), element)
		} else if boolValue, ok := stepBooleanValue(value.name); ok {
			node.AddStatement(attribute.InVocabulary().VocabularyElement(), sst.Boolean(boolValue))
		}
	} else {
		if !cp.isSkippedParameterNode(ibnodeVal) {
			node.AddStatement(attribute.InVocabulary().VocabularyElement(), cp.resolveDummyIRReference(ibnodeVal))
		}
	}
}

// Dummy IR handling has three hooks:
// references are resolved here, collection structures are normalized in
// applyMappedCollectionAttribute, and leftover dummy nodes are removed in cleanup.
func (cp *conversionParameters) resolveDummyIRReference(node sst.IBNode) sst.IBNode {
	if node == nil {
		return nil
	}

	rawAttrValues, exists := cp.rawAttributeValuesMap[node]
	if !exists {
		return node
	}

	switch rawAttrValues.name {
	case PRODUCT_DEFINITION_SHAPE:
		if definition := cp.ibNodeAt(rawAttrValues.MixedValues, 2); definition != nil {
			return definition
		}
	}
	return node
}

func (cp *conversionParameters) applyMappedCollectionAttribute(node sst.IBNode, collectionNode sst.IBNode, attribute sst.IBNode) {
	integerPoints := cp.getIntegerCollection(collectionNode)
	floatPoints := cp.getFloatCollection(collectionNode)
	collectionTree := cp.collectionTree(collectionNode)

	// Owner-specific dummy IR normalizers preserve structure before cleanup.
	if attribute.Is(rep.EdgeList) && cp.normalizePathListsFromOrientedEdgeIR(node, collectionNode) {
		return
	}

	inVocabulary := attribute.InVocabulary()
	if inVocabulary == nil {
		return
	}
	predicate := inVocabulary.VocabularyElement()

	if cp.collectionHasNestedCollection(collectionNode) {
		col, err := cp.createMultiDimensionalCollectionFromTree(collectionTree)
		if err != nil {
			fmt.Println("Error creating collection:", err)
			return
		}
		if col != nil {
			node.AddStatement(predicate, col)
		}
	} else if len(floatPoints) > 0 {
		if cp.attributeRangeIsRDFList(attribute) {
			col := sst.NewLiteralCollection(floatPoints[0], floatPoints[1:]...)
			node.AddStatement(predicate, col)
		} else {
			cp.addCollectionLiteralsAsStatements(node, predicate, floatPoints)
		}
	} else if len(integerPoints) > 0 {
		if cp.attributeRangeIsRDFList(attribute) {
			col := sst.NewLiteralCollection(integerPoints[0], integerPoints[1:]...)
			node.AddStatement(predicate, col)
		} else {
			cp.addCollectionLiteralsAsStatements(node, predicate, integerPoints)
		}
	} else if collectionTree.Children != nil && len(collectionTree.Children) > 0 {
		if cp.attributeRangeIsRDFList(attribute) {
			col, err := cp.createMultiDimensionalCollectionFromTree(collectionTree)
			if err != nil {
				fmt.Println("Error creating collection:", err)
				return
			}
			if col != nil {
				node.AddStatement(predicate, col)
			}
		} else {
			cp.addCollectionTermsAsStatements(node, predicate, collectionNode)
		}
	}
}

// Path.edge_list stores oriented_edge dummy IR as edgeList plus orientationList.
func (cp *conversionParameters) normalizePathListsFromOrientedEdgeIR(node sst.IBNode, collectionNode sst.IBNode) bool {
	collection, ok := collectionNode.AsCollection()
	if !ok {
		return false
	}

	var edges []sst.Term
	var orientations []sst.Literal
	var consumed []sst.IBNode
	allMembersMatched := true
	memberCount := 0

	collection.ForMembers(func(_ int, member sst.Term) {
		if !allMembersMatched {
			return
		}
		memberCount++

		memberNode, ok := member.(sst.IBNode)
		if !ok {
			allMembersMatched = false
			return
		}

		edge, orientation, ok := cp.orientedEdgeIRParts(memberNode)
		if !ok {
			allMembersMatched = false
			return
		}

		edges = append(edges, edge)
		orientations = append(orientations, orientation)
		consumed = append(consumed, memberNode)
	})

	if !allMembersMatched || memberCount == 0 {
		return false
	}

	node.AddStatement(rep.EdgeList, cp.graph.CreateCollection(edges...))
	node.AddStatement(rep.OrientationList, sst.NewLiteralCollection(orientations[0], orientations[1:]...))
	for _, consumedNode := range consumed {
		cp.consumedIRNodes[consumedNode] = true
	}
	return true
}

// Future oriented_* dummy classes should add owner-specific normalizers.
func (cp *conversionParameters) orientedEdgeIRParts(node sst.IBNode) (sst.IBNode, sst.Literal, bool) {
	if !cp.isDummyIRNode(node) {
		return nil, nil, false
	}

	edgeValue, ok := cp.rawValueForAttribute(node, rep.EdgeElement)
	if !ok {
		return nil, nil, false
	}
	edge, ok := edgeValue.(sst.IBNode)
	if !ok || cp.isSkippedParameterNode(edge) {
		return nil, nil, false
	}

	orientationValue, ok := cp.rawValueForAttribute(node, rep.Orientation)
	if !ok {
		return nil, nil, false
	}
	orientation, ok := cp.booleanLiteral(orientationValue)
	if !ok {
		return nil, nil, false
	}

	return edge, orientation, true
}

func (cp *conversionParameters) isDummyIRNode(node sst.IBNode) bool {
	rawAttrValues, exists := cp.rawAttributeValuesMap[node]
	if !exists {
		return false
	}

	entity := cp.ontologyEntity(rawAttrValues.name)
	return entity.ontologyObject != nil && nodeHasType(entity.ontologyObject, ssmeta.DummyStepIrClass)
}

func (cp *conversionParameters) rawValueForAttribute(node sst.IBNode, target sst.Elementer) (interface{}, bool) {
	rawAttrValues, exists := cp.rawAttributeValuesMap[node]
	if !exists {
		return nil, false
	}

	entity := cp.ontologyEntity(rawAttrValues.name)
	attributes := entity.attributeOrders
	values := rawAttrValues.MixedValues
	if len(attributes) > len(values) {
		attributes = attributes[len(attributes)-len(values):]
	}

	for i, value := range values {
		if i >= len(attributes) {
			continue
		}
		if attributes[i].Is(target) {
			return value, true
		}
	}
	return nil, false
}

func (cp *conversionParameters) booleanLiteral(value interface{}) (sst.Literal, bool) {
	switch v := value.(type) {
	case bool:
		return sst.Boolean(v), true
	case sst.Boolean:
		return v, true
	case sst.IBNode:
		enumerationValue, ok := cp.enumerationValueMap[v]
		if !ok {
			return nil, false
		}
		boolValue, ok := stepBooleanValue(enumerationValue.name)
		if !ok {
			return nil, false
		}
		return sst.Boolean(boolValue), true
	}
	return nil, false
}

func (cp *conversionParameters) collectionHasNestedCollection(value sst.IBNode) bool {
	collection, ok := value.AsCollection()
	if !ok {
		return false
	}

	hasNestedCollection := false
	collection.ForMembers(func(_ int, member sst.Term) {
		memberNode, ok := member.(sst.IBNode)
		if !ok {
			return
		}
		if _, isCollection := memberNode.AsCollection(); isCollection {
			hasNestedCollection = true
		}
	})
	return hasNestedCollection
}

func (cp *conversionParameters) attributeRangeIsRDFList(attribute sst.IBNode) bool {
	inVocab := attribute.InVocabulary()
	if inVocab == nil || inVocab.Range() == nil {
		return false
	}
	_, ok := inVocab.Range().(rdf.KindList)
	return ok
}

func (cp *conversionParameters) addCollectionLiteralsAsStatements(node sst.IBNode, predicate sst.Node, literals []sst.Literal) {
	for _, literal := range literals {
		if !node.CheckTriple(predicate, literal) {
			node.AddStatement(predicate, literal)
		}
	}
}

func (cp *conversionParameters) addCollectionTermsAsStatements(node sst.IBNode, predicate sst.Node, collectionNode sst.IBNode) {
	collection, ok := collectionNode.AsCollection()
	if !ok {
		return
	}

	collection.ForMembers(func(_ int, member sst.Term) {
		if memberNode, ok := member.(sst.IBNode); ok && cp.isSkippedCollectionNode(memberNode) {
			return
		}
		if !node.CheckTriple(predicate, member) {
			node.AddStatement(predicate, member)
		}
	})
}

func (cp *conversionParameters) replaceArrangedPartOfWithHasArrangedPart(graph sst.NamedGraph) {
	type arrangedPartLink struct {
		parent sst.IBNode
		child  sst.IBNode
	}

	links := make([]arrangedPartLink, 0)
	graph.ForAllIBNodes(func(node sst.IBNode) error {
		node.ForDelete(func(_ int, _ sst.IBNode, predicate sst.IBNode, object sst.Term) bool {
			if !predicate.Is(lci.ArrangedPartOf) {
				return false
			}
			parent, ok := object.(sst.IBNode)
			if !ok {
				return false
			}
			if cp.shouldInvertArrangedPartOf(node, parent) {
				links = append(links, arrangedPartLink{parent: parent, child: node})
			}
			return true
		})
		return nil
	})

	for _, link := range links {
		addStatementIfMissing(link.parent, lci.HasArrangedPart, link.child)
	}
}

func (cp *conversionParameters) shouldInvertArrangedPartOf(child sst.IBNode, _ sst.IBNode) bool {
	return nodeHasTypeAssignableTo(child, lci.Individual)
}

func (cp *conversionParameters) collapseSingleUseObjectPropertyPunning(graph sst.NamedGraph) {
	type propertyUse struct {
		subject   sst.IBNode
		predicate sst.IBNode
		object    sst.Term
	}

	candidates := make(map[sst.IBNode]sst.IBNode)
	graph.ForIRINodes(func(node sst.IBNode) error {
		if baseProperty, ok := metadataFreeObjectPropertyBase(node); ok {
			candidates[node] = baseProperty
		}
		return nil
	})

	uses := make(map[sst.IBNode][]propertyUse)
	referencedAsObject := make(map[sst.IBNode]bool)
	graph.ForIRINodes(func(node sst.IBNode) error {
		node.ForAll(func(_ int, subject sst.IBNode, predicate sst.IBNode, object sst.Term) error {
			if subject != node {
				return nil
			}
			if _, ok := candidates[predicate]; ok {
				uses[predicate] = append(uses[predicate], propertyUse{
					subject:   node,
					predicate: predicate,
					object:    object,
				})
			}
			if objectNode, ok := object.(sst.IBNode); ok {
				_, referencedAsCandidate := candidates[objectNode]
				if !referencedAsCandidate {
					return nil
				}
				referencedAsObject[objectNode] = true
			}
			return nil
		})
		return nil
	})

	for property := range candidates {
		propertyUses := uses[property]
		if len(propertyUses) != 1 || referencedAsObject[property] {
			continue
		}

		use := propertyUses[0]
		use.subject.ForDelete(func(_ int, subject sst.IBNode, predicate sst.IBNode, _ sst.Term) bool {
			return subject == use.subject && predicate == use.predicate
		})
		baseProperty := candidates[property]
		addStatementIfMissing(use.subject, baseProperty, use.object)
		property.Delete()
	}
}

func metadataFreeObjectPropertyBase(node sst.IBNode) (sst.IBNode, bool) {
	if node.InVocabulary() != nil ||
		!nodeHasObject(node, rdf.Type, owl.ObjectProperty) {
		return nil, false
	}

	baseProperties := make([]sst.IBNode, 0, 1)
	metadataFree := true
	node.ForAll(func(_ int, subject sst.IBNode, predicate sst.IBNode, object sst.Term) error {
		if subject != node {
			return nil
		}
		if predicate.Is(rdf.Type) {
			typeNode, ok := object.(sst.IBNode)
			if ok && typeNode.Is(owl.ObjectProperty) {
				return nil
			}
		}
		if predicate.Is(rdfs.SubPropertyOf) {
			parentProperty, ok := object.(sst.IBNode)
			if ok && parentProperty.InVocabulary() != nil && parentProperty.InVocabulary().IsProperty() {
				baseProperties = append(baseProperties, parentProperty)
				return nil
			}
		}
		metadataFree = false
		return nil
	})
	if !metadataFree || len(baseProperties) != 1 {
		return nil, false
	}
	return baseProperties[0], true
}

func nodeHasObject(node sst.IBNode, predicate sst.Node, expected sst.Elementer) bool {
	for _, object := range node.GetObjects(predicate) {
		objectNode, ok := object.(sst.IBNode)
		if ok && objectNode.Is(expected) {
			return true
		}
	}
	return false
}

func (cp *conversionParameters) isSkippedParameterNode(node sst.IBNode) bool {
	return node.Is(ssmeta.IndeterminateValue) ||
		node.Is(ssmeta.DerivedValue) ||
		node.Is(ssmeta.EmptyAgggregateValue) ||
		nodeHasType(node, ssmeta.DefinedType)
}

func (cp *conversionParameters) isSkippedCollectionNode(node sst.IBNode) bool {
	return cp.isSkippedParameterNode(node) ||
		nodeHasType(node, ssmeta.EnumerationValue)
}

func nodeHasType(node sst.IBNode, typ sst.Elementer) bool {
	for _, object := range node.GetObjects(rdf.Type) {
		typeNode, ok := object.(sst.IBNode)
		if ok && typeNode.Is(typ) {
			return true
		}
	}
	return false
}

func nodeHasTypeAssignableTo(node sst.IBNode, typ sst.Elementer) bool {
	expected := typ.VocabularyElement()
	for _, object := range node.GetObjects(rdf.Type) {
		typeNode, ok := object.(sst.IBNode)
		if !ok {
			continue
		}
		if typeNode.Is(typ) {
			return true
		}
		if info := typeNode.InVocabulary(); info != nil && info.IsMainClass(expected) {
			return true
		}
	}
	return false
}

func addStatementIfMissing(node sst.IBNode, predicate sst.Node, object sst.Term) {
	if !node.CheckTriple(predicate, object) {
		node.AddStatement(predicate, object)
	}
}

// Collection conversion helpers.
// These functions preserve scalar and nested STEP aggregate values as RDF collections.
func (cp *conversionParameters) getFloatCollection(value sst.IBNode) []sst.Literal {
	var points []sst.Double
	if floatCollection, ok := value.AsCollection(); ok {
		floatCollection.ForMembers(func(_ int, o sst.Term) {
			if o.TermKind() == sst.TermKindLiteral {
				if point, ok := o.(sst.Double); ok {
					points = append(points, sst.Double(point))
				}
			}
		})
	}

	members := make([]sst.Literal, len(points))
	for i := 0; i < len(points); i++ {
		members[i] = sst.Double(points[i])
	}
	return members
}

func stepBooleanValue(boolValue string) (bool, bool) {
	switch strings.ToUpper(boolValue) {
	case "T":
		return true, true
	case "F":
		return false, true
	}
	return false, false
}

func (cp *conversionParameters) getIntegerCollection(value sst.IBNode) []sst.Literal {
	var points []sst.Integer
	if floatCollection, ok := value.AsCollection(); ok {
		floatCollection.ForMembers(func(_ int, o sst.Term) {
			if o.TermKind() == sst.TermKindLiteral {
				if point, ok := o.(sst.Integer); ok {
					points = append(points, sst.Integer(point))
				}
			}
		})
	}

	members := make([]sst.Literal, len(points))
	for i := 0; i < len(points); i++ {
		members[i] = sst.Integer(points[i])
	}
	return members
}

func (cp *conversionParameters) collectionTree(value sst.IBNode) *TreeNode {
	root := &TreeNode{Value: value}
	cp.appendCollectionTreeChildren(value, root)
	return root
}

func (cp *conversionParameters) appendCollectionTreeChildren(value sst.IBNode, currentNode *TreeNode) {
	if ibnodeCollection, ok := value.AsCollection(); ok {
		ibnodeCollection.ForMembers(func(_ int, o sst.Term) {
			child := &TreeNode{Value: o}
			currentNode.Children = append(currentNode.Children, child)
			if o.TermKind() == sst.TermKindIBNode || o.TermKind() == sst.TermKindTermCollection {
				if _, isCollection := o.(sst.IBNode).AsCollection(); isCollection {
					cp.appendCollectionTreeChildren(o.(sst.IBNode), child)
				}
			}
		})
	}
}

func (cp *conversionParameters) createMultiDimensionalCollectionFromTree(root *TreeNode) (sst.Term, error) {
	if root.Children == nil || len(root.Children) == 0 {
		if node, ok := root.Value.(sst.IBNode); ok && cp.isSkippedCollectionNode(node) {
			return nil, nil
		}
		return root.Value, nil
	}

	var innerCols []sst.Term
	for _, child := range root.Children {
		col, err := cp.createMultiDimensionalCollectionFromTree(child)
		if err != nil {
			return nil, err
		}
		if col != nil {
			innerCols = append(innerCols, col)
		}
	}

	if len(innerCols) == 0 {
		return nil, nil
	}

	outerCol := cp.graph.CreateCollection(innerCols...)

	return outerCol, nil
}

// Pass 4: complex-instance type finalization.
// assignComplexOntologyTypes assigns final types for complex instances after all
// member attributes were mapped, keeping the most specific compatible main classes.
func (cp *conversionParameters) assignComplexOntologyTypes() {
	parentMap := make(map[sst.IBNode]sst.IBNode)
	hierarchyLevelMap := make(map[sst.IBNode]int)

	// Build a local class hierarchy for complex member type selection.
	for _, arrayOfNode := range cp.collectComplexNodes {
		for _, node := range arrayOfNode {
			node.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
				if o.TermKind() == sst.TermKindIBNode || o.TermKind() == sst.TermKindTermCollection {
					if p.Is(rdfs.SubClassOf) && o.(sst.IBNode) != node {
						if o.(sst.IBNode) != node {
							parentMap[node] = o.(sst.IBNode)
						}
					}
				}
				return nil
			})
		}
	}

	for node := range parentMap {
		hierarchyLevelMap[node] = cp.getHierarchyLevel(node, parentMap)
	}

	cp.sortAndReverseNodes(hierarchyLevelMap)

	for selectedNode, arrayOfNode := range cp.collectComplexNodes {
		mainClasses := cp.complexMainClasses(selectedNode, arrayOfNode)
		for _, mainClass := range mainClasses {
			selectedNode.AddStatement(rdf.Type, mainClass.VocabularyElement())
		}
		if len(mainClasses) == 0 && !cp.hasMappedMeasureQuantityType(selectedNode) {
			continue
		}
		for _, node := range arrayOfNode {
			if nodeHasType(node, ssmeta.OptionClass) {
				addStatementIfMissing(selectedNode, rdf.Type, node.InVocabulary().VocabularyElement())
			}
		}
	}
}

func (cp *conversionParameters) complexMainClasses(selectedNode sst.IBNode, arrayOfNode []sst.IBNode) []sst.ElementInformer {
	var mainClasses []sst.ElementInformer
	for _, node := range arrayOfNode {
		inVocabulary := node.InVocabulary()
		if inVocabulary == nil || !inVocabulary.IsMainClass(sst.Element{}) {
			continue
		}
		if nodeHasType(node, ssmeta.OptionClass) && !nodeHasType(node, ssmeta.MainClass) {
			continue
		}
		if cp.shouldSkipMeasureRepresentationMainClass(selectedNode, node, arrayOfNode) {
			continue
		}

		candidate := inVocabulary.VocabularyElement()
		skipCandidate := false
		keptMainClasses := mainClasses[:0]
		for _, existing := range mainClasses {
			existingElement := existing.VocabularyElement()
			switch {
			case existingElement == candidate:
				skipCandidate = true
				keptMainClasses = append(keptMainClasses, existing)
			case inVocabulary.IsMainClass(existingElement):
				// Candidate is more specific than the existing main class.
			case existing.IsMainClass(candidate):
				// Existing main class is more specific than the candidate.
				skipCandidate = true
				keptMainClasses = append(keptMainClasses, existing)
			default:
				keptMainClasses = append(keptMainClasses, existing)
			}
		}
		mainClasses = keptMainClasses
		if !skipCandidate {
			mainClasses = append(mainClasses, inVocabulary)
		}
	}
	return mainClasses
}

func (cp *conversionParameters) shouldSkipMeasureRepresentationMainClass(selectedNode sst.IBNode, classNode sst.IBNode, complexClasses []sst.IBNode) bool {
	if !cp.hasMappedMeasureQuantityType(selectedNode) || !slices.ContainsFunc(complexClasses, func(node sst.IBNode) bool {
		return node.Is(rep.MeasureRepresentationItem)
	}) {
		return false
	}
	if classNode.Is(rep.MeasureRepresentationItem) {
		return false
	}

	inVocabulary := classNode.InVocabulary()
	return classNode.Is(rep.RepresentationItem) ||
		(inVocabulary != nil && inVocabulary.IsMainClass(rep.RepresentationItem.Element))
}

func (cp *conversionParameters) sortAndReverseNodes(hierarchyLevelMap map[sst.IBNode]int) {
	for key, arrayOfNode := range cp.collectComplexNodes {
		sort.Slice(arrayOfNode, func(i, j int) bool {
			return hierarchyLevelMap[arrayOfNode[i]] < hierarchyLevelMap[arrayOfNode[j]]
		})
		for i, j := 0, len(arrayOfNode)-1; i < j; i, j = i+1, j-1 {
			arrayOfNode[i], arrayOfNode[j] = arrayOfNode[j], arrayOfNode[i]
		}
		cp.collectComplexNodes[key] = arrayOfNode
	}
}

func (cp *conversionParameters) getHierarchyLevel(node sst.IBNode, parentMap map[sst.IBNode]sst.IBNode) int {
	level := 0
	for ; parentMap[node] != nil; node = parentMap[node] {
		level++
	}
	return level
}

// Pass 5: explicit SST semantic lifting.
// These helpers add SST concepts that are not direct ssmeta attribute mappings.
// mapExplicitSSTSemantics runs the explicit PDM, measure, and occurrence mappers.
func (cp *conversionParameters) mapExplicitSSTSemantics() {
	for node, singleValue := range cp.singleInstanceValues {
		for _, instanceType := range singleValue {
			if cp.singleEntityMap[instanceType].ontologyObject != nil {
				continue
			}
			if cp.isMeasureWithUnitNode(node) {
				cp.applyMeasureWithUnit(node, cp.extractMeasureWithUnit(node), true)
			}
			cp.mapExplicitSSTEntity(node, instanceType)
		}
	}
}

func (cp *conversionParameters) isMeasureWithUnitNode(node sst.IBNode) bool {
	rawAttrValues, exists := cp.rawAttributeValuesMap[node]
	if !exists {
		return false
	}
	return isMeasureWithUnitName(rawAttrValues.name)
}

func isMeasureWithUnitName(name string) bool {
	name = strings.ToUpper(name)
	return name == MEASURE_WITH_UNIT || strings.HasSuffix(name, "_MEASURE_WITH_UNIT")
}

func (cp *conversionParameters) mapExplicitSSTEntity(node sst.IBNode, instanceType sst.IBNode) {
	if cp.extraEntityMap[instanceType].ontologyObject == nil {
		return
	}

	switch cp.extraEntityMap[instanceType].name {
	case PRODUCT:
		cp.convertPDMCollection(node)
	case INTEGER_REPRESENTATION_ITEM:
		cp.convertIntegerRepresentationItem(node)
	case NEXT_ASSEMBLY_USAGE_OCCURRENCE:
		cp.convertNextOccurrence(node)
	}
}

func (cp *conversionParameters) convertIntegerRepresentationItem(node sst.IBNode) {
	rawAttrValues, ok := cp.rawAttributeValuesMap[node]
	if !ok || len(rawAttrValues.MixedValues) < 2 {
		return
	}

	node.AddStatement(rdf.Type, rep.ValueRepresentationItem)
	if label, ok := stringValue(rawAttrValues.MixedValues[0]); ok && cp.isValid(label) {
		node.AddStatement(rdfs.Label, sst.String(label))
	}
	if value, ok := integerValue(rawAttrValues.MixedValues[1]); ok {
		node.AddStatement(rdf.Value, value)
	}
}

func stringValue(value interface{}) (string, bool) {
	switch v := value.(type) {
	case string:
		return v, true
	case sst.String:
		return string(v), true
	default:
		return "", false
	}
}

func integerValue(value interface{}) (sst.Integer, bool) {
	switch v := value.(type) {
	case sst.Integer:
		return v, true
	case int64:
		return sst.Integer(v), true
	case float64:
		return integerFromFloat(v)
	case sst.Double:
		return integerFromFloat(float64(v))
	default:
		return 0, false
	}
}

func integerFromFloat(value float64) (sst.Integer, bool) {
	integer := int64(value)
	if float64(integer) != value {
		return 0, false
	}
	return sst.Integer(integer), true
}

// Pass 5a: PDM semantic lifting.
// convertPDMCollection follows product references and creates the SST PDM chain:
// product -> Part, formation -> PartVersion, definition -> PartDesign/AssemblyDesign.
func (cp *conversionParameters) convertPDMCollection(node sst.IBNode) {
	productReferences := cp.rawNodesReferencing(node)
	for _, reference := range productReferences {
		if reference == nil {
			continue
		}
		switch cp.rawAttributeValuesMap[reference].name {
		case PRODUCT_RELATED_PRODUCT_CATEGORY:
			cp.convertProductToPart(node)
		case PRODUCT_DEFINITION_FORMATION, PRODUCT_DEFINITION_FORMATION_WITH_SPECIFIED_SOURCE:
			cp.convertProductFormation(node, reference)
		}
	}
}

func (cp *conversionParameters) convertProductToPart(product sst.IBNode) {
	product.AddStatement(rdf.Type, sso.Part)
	cp.applyPDMIdentityText(product, "")
}

func (cp *conversionParameters) convertProductFormation(product sst.IBNode, formation sst.IBNode) {
	formation.AddStatement(rdf.Type, sso.PartVersion)
	product.AddStatement(sso.HasPartVersion, formation)
	cp.applyPDMIdentityText(formation, VERSION)

	for _, definition := range cp.rawNodesReferencing(formation) {
		cp.convertProductDefinition(formation, definition)
	}
}

func (cp *conversionParameters) convertProductDefinition(formation sst.IBNode, definition sst.IBNode) {
	if definition == nil {
		return
	}

	formation.AddStatement(sso.HasProductDefinition, definition)
	cp.applyPDMIdentityText(definition, DESIGN)

	if cp.addDefiningGeometryAndDetectAssembly(definition) {
		definition.AddStatement(rdf.Type, sso.AssemblyDesign)
	} else {
		definition.AddStatement(rdf.Type, sso.PartDesign)
	}
}

func (cp *conversionParameters) addDefiningGeometryAndDetectAssembly(definition sst.IBNode) bool {
	hasNextOccurrence := false
	for _, reference := range cp.rawNodesReferencing(definition) {
		if reference == nil {
			continue
		}
		switch cp.rawAttributeValuesMap[reference].name {
		case PRODUCT_DEFINITION_SHAPE:
			cp.addDefiningGeometryFromProductDefinitionShape(definition, reference)
			cp.addArrangedShapeElementsFromProductDefinitionShape(definition, reference)
		case NEXT_ASSEMBLY_USAGE_OCCURRENCE:
			hasNextOccurrence = true
		}
	}
	return hasNextOccurrence
}

func (cp *conversionParameters) addDefiningGeometryFromProductDefinitionShape(definition sst.IBNode, productDefinitionShape sst.IBNode) {
	for _, representation := range cp.rawNodesReferencing(productDefinitionShape) {
		if cp.rawAttributeValuesMap[representation].name != SHAPE_DEFINITION_REPRESENTATION {
			continue
		}
		if shapeRep := cp.ibNodeAt(cp.rawAttributeValuesMap[representation].MixedValues, 1); shapeRep != nil {
			addStatementIfMissing(definition, sso.DefiningGeometry, shapeRep)
		}
	}
}

func (cp *conversionParameters) addArrangedShapeElementsFromProductDefinitionShape(definition sst.IBNode, productDefinitionShape sst.IBNode) {
	for _, reference := range cp.rawNodesReferencing(productDefinitionShape) {
		if reference == nil || !nodeHasTypeAssignableTo(reference, sso.ShapeElement) {
			continue
		}
		addStatementIfMissing(definition, lci.HasArrangedPart, reference)
	}
}

func (cp *conversionParameters) applyLabelOrCommentAtPosition(node sst.IBNode, value interface{}, position int) {
	text, ok := stringValue(value)
	if !ok || !cp.isValid(text) {
		return
	}

	switch position {
	case 0:
		node.AddStatement(rdfs.Label, sst.String(text))
	case 1:
		node.AddStatement(rdfs.Comment, sst.String(text))
	}
}

func (cp *conversionParameters) applyPDMIdentityText(node sst.IBNode, partType string) {
	for i, partData := range cp.rawAttributeValuesMap[node].MixedValues {
		part, ok := stringValue(partData)
		if !ok || !cp.isValid(part) {
			continue
		}

		switch i {
		case 0:
			switch partType {
			case VERSION:
				node.AddStatement(sso.ViewID, sst.String(part))
			case DESIGN:
				node.AddStatement(sso.VersionID, sst.String(part))
			default:
				node.AddStatement(sso.ID, sst.String(part))
			}
		case 1:
			node.AddStatement(rdfs.Label, sst.String(part))
		case 2:
			node.AddStatement(rdfs.Comment, sst.String(part))
		}
	}
}

func (cp *conversionParameters) rawNodesReferencing(node sst.IBNode) []sst.IBNode {
	if cp.rawReferenceIndex == nil {
		cp.rawReferenceIndex = cp.buildRawReferenceIndex()
	}
	return cp.rawReferenceIndex[node]
}

func (cp *conversionParameters) buildRawReferenceIndex() map[sst.IBNode][]sst.IBNode {
	references := make(map[sst.IBNode][]sst.IBNode)
	for owner, values := range cp.rawAttributeValuesMap {
		for _, value := range values.MixedValues {
			referenced, ok := value.(sst.IBNode)
			if !ok {
				continue
			}
			if collection, ok := referenced.AsCollection(); ok {
				collection.ForMembers(func(_ int, member sst.Term) {
					if member.TermKind() == sst.TermKindIBNode {
						cp.addUnique(references, member.(sst.IBNode), owner)
					}
				})
				continue
			}
			cp.addUnique(references, referenced, owner)
		}
	}
	return references
}

// Pass 5b: assembly occurrence and representation relationship lifting.
// convertNextOccurrence maps a NEXT_ASSEMBLY_USAGE_OCCURRENCE into the SST
// occurrence predicate plus its sso:SingleOccurrence node and shape context links.
func (cp *conversionParameters) convertNextOccurrence(node sst.IBNode) {
	addStatementIfMissing(node, rdf.Type, owl.ObjectProperty)
	addStatementIfMissing(node, rdfs.SubPropertyOf, sso.NextAssemblyOccurrenceUsage)
	singleInstance := cp.singleOccurrenceForNextOccurrence(node)
	cp.populateSingleOccurrence(node, singleInstance)
	cp.attachContextDependentShapeRepresentations(node)
}

func (cp *conversionParameters) populateSingleOccurrence(node sst.IBNode, singleInstance sst.IBNode) {
	for i, nextOccurrence := range cp.rawAttributeValuesMap[node].MixedValues {
		var nextText string
		switch v := nextOccurrence.(type) {
		case string:
			nextText = v
		case sst.String:
			nextText = string(v)
		}
		if cp.isValid(nextText) {
			switch i {
			case 0, 5:
				singleInstance.AddStatement(sso.ID, sst.String(nextText))
			case 1:
				singleInstance.AddStatement(rdfs.Label, sst.String(nextText))
			case 2:
				singleInstance.AddStatement(rdfs.Comment, sst.String(nextText))
			}
		}
		if nextIbnode, ok := nextOccurrence.(sst.IBNode); ok {
			if i == 3 {
				nextIbnode.AddStatement(node, singleInstance)
			}
			if i == 4 {
				singleInstance.AddStatement(lci.IsDefinedBy, nextIbnode)
			}
		}
	}
}

func (cp *conversionParameters) attachContextDependentShapeRepresentations(node sst.IBNode) {
	nextOccurrenceUsage := cp.rawNodesReferencing(node)
	for _, nextOccurrence := range nextOccurrenceUsage {
		if nextOccurrence != nil {
			contextDependentShape := cp.rawNodesReferencing(nextOccurrence)
			for _, contextDependentShape := range contextDependentShape {
				cp.mapContextDependentShapeRepresentation(node, contextDependentShape)
			}
		}
	}
}

func (cp *conversionParameters) singleOccurrenceForNextOccurrence(node sst.IBNode) sst.IBNode {
	if cp.singleOccurrenceMap == nil {
		cp.singleOccurrenceMap = make(map[sst.IBNode]sst.IBNode)
	}
	if singleOccurrence := cp.singleOccurrenceMap[node]; singleOccurrence != nil {
		return singleOccurrence
	}

	singleOccurrence := cp.graph.CreateIRINode("", sso.SingleOccurrence)
	cp.singleOccurrenceMap[node] = singleOccurrence
	return singleOccurrence
}

func (cp *conversionParameters) mapContextDependentShapeRepresentation(node sst.IBNode, contextDependentShape sst.IBNode) {
	for _, complexInstance := range cp.rawAttributeValuesMap[contextDependentShape].MixedValues {
		if complexNode, ok := complexInstance.(sst.IBNode); ok && len(cp.complexInstanceValues[complexNode]) > 0 {
			cp.mapShapeRepresentationRelationship(node, complexNode)
		}
	}
}

func (cp *conversionParameters) mapShapeRepresentationRelationship(node sst.IBNode, complexNode sst.IBNode) {
	addStatementIfMissing(node, sso.ContextDependentShapeRepresentation, complexNode)
	addStatementIfMissing(complexNode, rdf.Type, owl.ObjectProperty)

	for _, relationship := range cp.complexInstanceValues[complexNode] {
		switch cp.rawAttributeValuesMap[relationship].name {
		case REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION:
			cp.mapRepresentationRelationshipWithTransformation(complexNode, relationship)
		case REPRESENTATION_RELATIONSHIP:
			cp.mapRepresentationRelationship(complexNode, relationship)
		}
	}
}

func (cp *conversionParameters) mapRepresentationRelationshipWithTransformation(complexNode sst.IBNode, relationship sst.IBNode) {
	addStatementIfMissing(complexNode, rdfs.SubPropertyOf, rep.ShapeRepresentationRelationshipWithPlacementTransformation)
	for _, transformationOperator := range cp.rawAttributeValuesMap[relationship].MixedValues {
		if transformation, ok := transformationOperator.(sst.IBNode); ok {
			cp.mapItemDefinedTransformation(complexNode, transformation)
		}
	}
}

func (cp *conversionParameters) mapItemDefinedTransformation(complexNode sst.IBNode, transformation sst.IBNode) {
	addStatementIfMissing(complexNode, rep.TransformationOperator, transformation)
	addStatementIfMissing(transformation, rdf.Type, owl.ObjectProperty)
	addStatementIfMissing(transformation, rdfs.SubPropertyOf, rep.ItemDefinedTransformation)

	for i, itemDefinedTransformation := range cp.rawAttributeValuesMap[transformation].MixedValues {
		cp.applyLabelOrCommentAtPosition(transformation, itemDefinedTransformation, i)
		if itemDefinedNode, ok := itemDefinedTransformation.(sst.IBNode); ok && cp.rawAttributeValuesMap[transformation].MixedValues[2].(sst.IBNode) != nil && i == 3 {
			addStatementIfMissing(itemDefinedNode, transformation, cp.rawAttributeValuesMap[transformation].MixedValues[2].(sst.IBNode))
		}
	}
}

func (cp *conversionParameters) mapRepresentationRelationship(complexNode sst.IBNode, relationship sst.IBNode) {
	for i, representationRelationship := range cp.rawAttributeValuesMap[relationship].MixedValues {
		cp.applyLabelOrCommentAtPosition(complexNode, representationRelationship, i)
		if representationNode, ok := representationRelationship.(sst.IBNode); ok && cp.rawAttributeValuesMap[relationship].MixedValues[2].(sst.IBNode) != nil && i == 3 {
			addStatementIfMissing(representationNode, complexNode, cp.rawAttributeValuesMap[relationship].MixedValues[2].(sst.IBNode))
		}
	}
}

func (cp *conversionParameters) isValid(s string) bool {
	testValues := []string{"", " ", "NONE", "none", "/NULL", "-"}
	for _, val := range testValues {
		if strings.TrimSpace(s) == val {
			return false
		}
	}
	return true
}

// Pass 6: cleanup of raw parser and temporary IR artifacts.
// removeEntityInstance removes ssmeta parser wrappers after their values were mapped.
func (cp *conversionParameters) removeEntityInstance(graph sst.NamedGraph) {
	// Collect nodes to delete separately to avoid modifying graph during ForDelete iteration
	nodesToDelete := make(map[sst.IBNode]bool)

	graph.ForIRINodes(func(node sst.IBNode) error {
		node.ForDelete(func(index int, s, p sst.IBNode, o sst.Term) bool {
			if o.TermKind() == sst.TermKindIBNode || o.TermKind() == sst.TermKindTermCollection {
				if p.Is(rdf.Type) && o.(sst.IBNode).Is(ssmeta.EntityInstance) {
					return true
				}
				if p.Is(ssmeta.EntityInstanceType) {
					return true
				}
				if p.Is(rdf.Type) && o.(sst.IBNode).Is(ssmeta.SingleEntityValue) {
					return true
				}
				if p.Is(ssmeta.SingleEntityValueType) {
					return true
				}
				if p.Is(ssmeta.ComplexInstanceValue) {
					return true
				}
				if p.Is(rdf.Type) && o.(sst.IBNode).Is(ssmeta.Entity) {
					nodesToDelete[s] = true
					return true
				}
				if p.Is(rdf.Type) && o.(sst.IBNode).Is(ssmeta.EnumerationValue) {
					nodesToDelete[s] = true
					return true
				}
			}
			return false
		})
		return nil
	})

	for nodeToDelete := range nodesToDelete {
		nodeToDelete.Delete()
	}
}

// removeAttributeValues deletes raw ordered attribute-value collections created by ParseRaw.
func (cp *conversionParameters) removeAttributeValues(graph sst.NamedGraph) {
	graph.ForIRINodes(func(node sst.IBNode) error {
		node.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
			if p.Is(ssmeta.AttributeValues) {
				cp.deleteCollectionRecursively(o.(sst.IBNode))
			}
			return nil
		})
		return nil
	})
}

// removeParserDefinedTypes drops raw ssmeta:DefinedType nodes after typed values are converted.
func (cp *conversionParameters) removeParserDefinedTypes(graph sst.NamedGraph) {
	nodesToDelete := make([]sst.IBNode, 0)
	graph.ForAllIBNodes(func(node sst.IBNode) error {
		if isParserDefinedTypeMetadataNode(node) {
			nodesToDelete = append(nodesToDelete, node)
		}
		return nil
	})

	for _, node := range nodesToDelete {
		node.Delete()
	}
}

func (cp *conversionParameters) removeConsumedIRNodes() {
	for node := range cp.consumedIRNodes {
		node.Delete()
	}
}

// Final safety net for dummy IR nodes whose meaning was already resolved or consumed.
func (cp *conversionParameters) removeRemainingDummyIRNodes(graph sst.NamedGraph) {
	nodesToDelete := make([]sst.IBNode, 0)
	graph.ForAllIBNodes(func(node sst.IBNode) error {
		if node.InVocabulary() != nil || !(cp.isDummyIRNode(node) || hasDummyIRType(node)) {
			return nil
		}
		if hasFinalNonDummyOntologyType(node) {
			removeDummyIRTypeStatements(node)
			return nil
		}
		nodesToDelete = append(nodesToDelete, node)
		return nil
	})

	for _, node := range nodesToDelete {
		node.Delete()
	}
}

func hasDummyIRType(node sst.IBNode) bool {
	for _, object := range node.GetObjects(rdf.Type) {
		typeNode, ok := object.(sst.IBNode)
		if ok && nodeHasType(typeNode, ssmeta.DummyStepIrClass) {
			return true
		}
	}
	return false
}

func hasFinalNonDummyOntologyType(node sst.IBNode) bool {
	for _, object := range node.GetObjects(rdf.Type) {
		typeNode, ok := object.(sst.IBNode)
		if !ok || nodeHasType(typeNode, ssmeta.DummyStepIrClass) {
			continue
		}
		if typeNode.Is(owl.ObjectProperty) || nodeHasType(typeNode, ssmeta.MainClass) {
			return true
		}
	}
	return false
}

func removeDummyIRTypeStatements(node sst.IBNode) {
	node.ForDelete(func(_ int, _ sst.IBNode, p sst.IBNode, o sst.Term) bool {
		typeNode, ok := o.(sst.IBNode)
		return p.Is(rdf.Type) && ok && nodeHasType(typeNode, ssmeta.DummyStepIrClass)
	})
}

func isParserDefinedTypeMetadataNode(node sst.IBNode) bool {
	for _, object := range node.GetObjects(rdf.Type) {
		typeNode, ok := object.(sst.IBNode)
		if ok && typeNode.Is(ssmeta.DefinedType) {
			return true
		}
	}
	return false
}

func (cp *conversionParameters) removeConstantAngleGlobalUnits(graph sst.NamedGraph) {
	graph.ForIRINodes(func(node sst.IBNode) error {
		node.ForDelete(func(_ int, _ sst.IBNode, predicate sst.IBNode, object sst.Term) bool {
			unit, ok := object.(sst.IBNode)
			return ok && predicate.Is(rep.GlobalUnit) && (unit.Is(qau.Radian) || unit.Is(qau.Steradian))
		})
		return nil
	})
}

func (cp *conversionParameters) removeDisconnectedNodes(graph sst.NamedGraph) {
	nodesToDelete := make([]sst.IBNode, 0)
	graph.ForAllIBNodes(func(node sst.IBNode) error {
		if node.IsIRINode() && node.Fragment() == "" {
			return nil
		}
		if node.TripleCount() == 0 {
			nodesToDelete = append(nodesToDelete, node)
		}
		return nil
	})

	for _, node := range nodesToDelete {
		node.Delete()
	}
}

func (cp *conversionParameters) deleteCollectionRecursively(node sst.IBNode) {
	if collection, ok := node.AsCollection(); ok {
		collection.ForMembers(func(_ int, o sst.Term) {
			if o.TermKind() == sst.TermKindIBNode || o.TermKind() == sst.TermKindTermCollection {
				cp.deleteCollectionRecursively(o.(sst.IBNode))
			}
		})
		node.Delete()
	}
}
