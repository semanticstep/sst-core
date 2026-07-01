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
	MainClass OntologyType = iota
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
	LENGTH_UNIT                                        = "LENGTH_UNIT"
	VERSION                                            = "VERSION"
	EDGE_LOOP                                          = "EDGE_LOOP"
	ATTRIBUTE                                          = "ATTRIBUTE"
	NAMED_UNIT                                         = "NAMED_UNIT"
	PLANE_ANGLE_UNIT                                   = "PLANE_ANGLE_UNIT"
	SOLID_ANGLE_UNIT                                   = "SOLID_ANGLE_UNIT"
	SHAPE_ASPECT                                       = "SHAPE_ASPECT"
	PROPERTY_DEFINITION                                = "PROPERTY_DEFINITION"
	PRODUCT_DEFINITION                                 = "PRODUCT_DEFINITION"
	SHAPE_REPRESENTATION                               = "SHAPE_REPRESENTATION"
	PRODUCT_DEFINITION_SHAPE                           = "PRODUCT_DEFINITION_SHAPE"
	PRODUCT_DEFINITION_CONTEXT                         = "PRODUCT_DEFINITION_CONTEXT"
	REPRESENTATION_RELATIONSHIP                        = "REPRESENTATION_RELATIONSHIP"
	MEASURE_REPRESENTATION_ITEM                        = "MEASURE_REPRESENTATION_ITEM"
	ITEM_DEFINED_TRANSFORMATION                        = "ITEM_DEFINED_TRANSFORMATION"
	GLOBAL_UNIT_ASSIGNED_CONTEXT                       = "GLOBAL_UNIT_ASSIGNED_CONTEXT"
	UNCERTAINTY_MEASURE_WITH_UNIT                      = "UNCERTAINTY_MEASURE_WITH_UNIT"
	PRODUCT_DEFINITION_FORMATION                       = "PRODUCT_DEFINITION_FORMATION"
	NEXT_ASSEMBLY_USAGE_OCCURRENCE                     = "NEXT_ASSEMBLY_USAGE_OCCURRENCE"
	SHAPE_DEFINITION_REPRESENTATION                    = "SHAPE_DEFINITION_REPRESENTATION"
	GLOBAL_UNCERTAINTY_ASSIGNED_CONTEXT                = "GLOBAL_UNCERTAINTY_ASSIGNED_CONTEXT"
	PRODUCT_RELATED_PRODUCT_CATEGORY                   = "PRODUCT_RELATED_PRODUCT_CATEGORY"
	SHAPE_REPRESENTATION_RELATIONSHIP                  = "SHAPE_REPRESENTATION_RELATIONSHIP"
	PROPERTY_DEFINITION_REPRESENTATION                 = "PROPERTY_DEFINITION_REPRESENTATION"
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

// TreeNode is a structure that will represent either an individual IBNode or a collection of TreeNodes
type TreeNode struct {
	Value    sst.Term
	Children []*TreeNode
}

type conversionParameters struct {
	graph                 sst.NamedGraph
	singleEntityMap       map[sst.IBNode]SingleEntity
	extraEntityMap        map[sst.IBNode]ExtraEntity
	enumerationValueMap   map[sst.IBNode]EnumerationValue
	definedTypeMap        map[sst.IBNode]DefinedType
	rawAttributeValuesMap map[sst.IBNode]RawAttributeValues
	complexInstanceValues map[sst.IBNode][]sst.IBNode
	singleInstanceValues  map[sst.IBNode][]sst.IBNode
	collectComplexNodes   map[sst.IBNode][]sst.IBNode
	enumerationCache      map[string]sst.IBNode
	enumerationElementMap map[string]sst.Element
	stepEntityMap         map[string]sst.IBNode
	ontologyEntityCache   map[string]SingleEntity
	rawReferenceIndex     map[sst.IBNode][]sst.IBNode
	singleOccurrenceMap   map[sst.IBNode]sst.IBNode
}

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

func Parse(src *bufio.Reader, errorReporter ErrorReporter) (graph sst.NamedGraph, err error) {
	// Pass 1: parse Part 21 into raw ssmeta entity/value nodes.
	graph, err = ParseRaw(src, errorReporter)
	if err != nil {
		return graph, err
	}

	cp := newConversionParameters(graph)
	if cp == nil {
		return graph, fmt.Errorf("conversionParameters is nil")
	}

	// Pass 2.a: collect ontology mapping metadata and raw attribute values.
	cp.extractMetaDataFromP21Dataset(graph)
	cp.extractAttributeValues(graph)

	// Pass 2.b: apply explicit SST semantic conversions before generic mapping.
	cp.processEntityInstance()

	// Pass 2.a continued: apply draft ontology-driven STEP attribute mapping.
	cp.processOntologyOrder()

	// Pass 3: remove raw parser artifacts after conversion.
	cp.removeEntityInstance(graph)
	cp.removeAttributeValues(graph)
	cp.removeParserDefinedTypes(graph)
	// cp.removeDisconnectedNodes(graph)

	return graph, nil
}

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
			entityName := cp.getName(node)
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
				cp.singleEntityMap[node] = cp.ontologyEntity(cp.getName(parentNode))
			}
		case ssmeta.IsEnumerationValue:
			ev := EnumerationValue{
				ExpressObject: ExpressObject{
					name:       cp.getName(node),
					objectType: EnumerationValueType,
				},
			}
			cp.enumerationValueMap[node] = ev
		case ssmeta.IsDefinedType:
			dt := DefinedType{
				ExpressObject: ExpressObject{
					name:       cp.getName(node),
					objectType: DefinedTypeType,
				},
			}
			cp.definedTypeMap[node] = dt
		}
		return nil
	})
}

// Reads ontology class/property kind, rdfs:subClassOf, and ssmeta:stepImMapAttributeOrder.
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
			orderedSuperNodes = cp.extractStepImMapSupertypeOrder(o.(sst.IBNode))
		}
		if p.Is(rdfs.SubClassOf) && !o.(sst.IBNode).Is(lci.Individual) {
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
		if !cp.containsIBNode(singleEntity.attributeOrders, attributeOrder) {
			singleEntity.attributeOrders = append(singleEntity.attributeOrders, attributeOrder)
		}
	}
}

func (cp *conversionParameters) getName(node sst.IBNode) string {
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
	nodeFound := cp.getSSREP(name)
	if nodeFound == nil {
		return singleEntity
	}

	singleEntity.ontologyObject = nodeFound
	cp.extractNode(nodeFound, &singleEntity, "")
	cp.ontologyEntityCache[key] = singleEntity
	return singleEntity
}

func (cp *conversionParameters) getSSREP(passedValue string) sst.IBNode {
	if cp.stepEntityMap == nil {
		cp.stepEntityMap = cp.loadStepEntityMap()
	}

	return cp.stepEntityMap[strings.ToUpper(passedValue)]
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
	return candidateElement != existingElement && candidateInfo.IsMainClass(existingElement)
}

func (cp *conversionParameters) getqau(unitName string) sst.IBNode {
	if result, found := cp.enumerationCache[unitName]; found {
		return result // Return cached result
	}

	var result sst.IBNode
	Vocgraph, _ := sst.StaticDictionary().Vocabulary(qau.QAUVocabulary)
	Vocgraph.ForIRINodes(func(node sst.IBNode) error {
		node.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
			if p.Is(rdfs.Label) {
				var keyValue string
				// Safely extract string value from literal (could be String or LangString)
				if literal, ok := o.(sst.Literal); ok {
					switch l := literal.(type) {
					case sst.String:
						keyValue = string(l)
					case sst.LangString:
						keyValue = l.Val
					default:
						// Skip non-string literals
						return nil
					}
				} else {
					// Not a literal, skip
					return nil
				}
				if keyValue != "" && strings.EqualFold(unitName, keyValue) {
					result = s
					cp.enumerationCache[unitName] = result // Cache result
				}
			}
			return nil
		})
		return nil
	})
	return result
}

func measureTypeElement(measureType string) (sst.Element, bool) {
	element, exists := p21MeasureTypeMap[strings.ToLower(strings.TrimSpace(measureType))]
	if !exists {
		return sst.Element{}, false
	}
	return element.VocabularyElement(), true
}

func addPhysicalQuantityType(node sst.IBNode, quantityType sst.Element) {
	node.AddStatement(rdf.Type, quantityType)
	node.AddStatement(rdf.Type, lci.PhysicalQuantity)
}

func (cp *conversionParameters) qauElementForLabel(label string) (sst.Element, bool) {
	getReferenceNode := cp.getqau(strings.TrimSpace(label))
	if getReferenceNode == nil || !getReferenceNode.IsIRINode() {
		return sst.Element{}, false
	}
	return getReferenceNode.IRI().VocabularyElement(), true
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
				if literal, ok := o.(sst.String); ok && normalizeStepName(string(literal)) == normalizeStepName(name) {
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

func (cp *conversionParameters) addComplexOntologyNode(parentNode sst.IBNode, instanceType sst.IBNode) {
	ontologyObject := cp.singleEntityMap[instanceType].ontologyObject
	if ontologyObject == nil {
		return
	}
	if !cp.containsIBNode(cp.collectComplexNodes[parentNode], ontologyObject) {
		cp.collectComplexNodes[parentNode] = append(cp.collectComplexNodes[parentNode], ontologyObject)
	}
}

// --------------------- end collect and prepare attribute order including from super types ----------------------------------------

// --------------------- start extract attribute values ----------------------------------------
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

		// get family name for future reference
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
	// newValue was not found; append it to the slice
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

// --------------------- end extract attribute values ----------------------------------------

// ------------------------- start handle measure representation item and global unit assigned context -------------------------

func (cp *conversionParameters) handleGlobalUnitAssignedContext(parentNode sst.IBNode, node sst.IBNode) {
	cp.addComplexOntologyNode(parentNode, node)

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

	for _, group := range cp.processIBNodeForGlobalUnit(unitNode) {
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

func (cp *conversionParameters) processIBNodeForGlobalUnit(node sst.IBNode) [][]string {
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
	cp.addComplexOntologyNode(parentNode, node)

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

	measure := cp.extractMeasureWithUnit(node)
	if element, ok := measureTypeElement(measure.measureType); ok {
		addPhysicalQuantityType(node, element)
	}
	if measure.hasValue {
		if unitElement, ok := cp.qauElementForLabel(measure.measureUnit); ok {
			node.AddStatement(unitElement, sst.Double(measure.measureValue))
		}
	}

	for i, value := range rawAttrValues.MixedValues {
		if i >= 2 {
			cp.processCommonTextData(node, value, i-2)
		}
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

	for _, group := range cp.processIBNodeForGlobalUnit(unitNode) {
		if unitLabel := unitLabelFromParts(group); unitLabel != "" {
			return unitLabel
		}
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
		return normalizeUnitPart(cp.getName(v))
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
	return strings.ToLower(strings.TrimSpace(value))
}

func (cp *conversionParameters) handleMeasureRepresentationItem(node sst.IBNode) {
	var measureType string
	var measureUnit string
	var measureValue float64

	getMixedValues := cp.rawAttributeValuesMap[node].MixedValues

	// getPartDesign := cp.findMeasureRepresentationPartDesign(node)

	for _, value := range getMixedValues {
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

		if ibnodeVal, ok := value.(sst.IBNode); ok {
			if literalCollection, ok := ibnodeVal.AsCollection(); ok {
				literalCollection.ForMembers(func(_ int, o sst.Term) {
					switch o.TermKind() {
					case sst.TermKindLiteral:
						if v, ok := o.(sst.Double); ok {
							measureValue = float64(v)
						}
					case sst.TermKindIBNode, sst.TermKindTermCollection:
						measureType = strings.ToLower(cp.definedTypeMap[o.(sst.IBNode)].name)
					}
				})
			} else {
				if measureUnit == "" {
					measureUnit = cp.measureUnitName(ibnodeVal)
				}
				for _, v := range cp.rawAttributeValuesMap[ibnodeVal].MixedValues {
					if ibnodeVal, ok := v.(sst.IBNode); ok {
						if ibnodeCollection, ok := ibnodeVal.AsCollection(); ok {
							ibnodeCollection.ForMembers(func(_ int, o sst.Term) {
								if o.TermKind() == sst.TermKindIBNode || o.TermKind() == sst.TermKindTermCollection {
									var groupedStrings [][]string
									var numberType string

									// extract data from ibnode collection to process
									for _, v := range cp.rawAttributeValuesMap[o.(sst.IBNode)].MixedValues {
										if ibnodeVal, ok := v.(sst.IBNode); ok {
											groupedStrings = cp.processIBNodeForGlobalUnit(ibnodeVal)
										}
										if intVal, ok := v.(float64); ok {
											switch intVal {
											case 2:
												numberType = SQUARE
											case 3:
												numberType = CUBIC
											}
										}
										if intVal, ok := v.(sst.Double); ok {
											switch intVal {
											case 2:
												numberType = SQUARE
											case 3:
												numberType = CUBIC
											}
										}
										if intVal, ok := v.(sst.Integer); ok {
											switch intVal {
											case 2:
												numberType = SQUARE
											case 3:
												numberType = CUBIC
											}
										}
									}
									// handle measurement unit type with square and cubic
									var merged string
									for _, group := range groupedStrings {
										if len(group) == 2 {
											merged = strings.ToLower(group[0] + " " + group[1])
										}
									}
									if merged != "" {
										measureUnit = strings.TrimSpace(strings.ToLower(numberType + " " + merged))
									}
								}
							})
						}
					}
				}
			}
		}
	}

	// use values measureType and measureValue
	if element, exists := measureTypeElement(measureType); exists {
		if unitElement, ok := cp.qauElementForLabel(measureUnit); ok {
			measureItem := cp.graph.CreateBlankNode()
			addPhysicalQuantityType(measureItem, element)
			measureItem.AddStatement(unitElement, sst.Double(measureValue))
			node.AddStatement(rep.MeasureValue, measureItem)
		}
	}
}

// ------------------------- end handle measure representation item and global unit assigned context -------------------------

// ------------------------- start handling single leave and multi leave conversion --------------------------------

func (cp *conversionParameters) processEntityInstance() {
	cp.processSingleEntityInstances()
	cp.processComplexEntityInstances()
}

func (cp *conversionParameters) processSingleEntityInstances() {
	for node, singleValue := range cp.singleInstanceValues {
		for _, instanceType := range singleValue {
			if cp.singleEntityMap[instanceType].ontologyObject != nil {
				switch cp.singleEntityMap[instanceType].ontologyType {
				case MainClass:
					node.AddStatement(rdf.Type, cp.singleEntityMap[instanceType].ontologyObject.InVocabulary().VocabularyElement())
					if cp.singleEntityMap[instanceType].name == MEASURE_REPRESENTATION_ITEM {
						cp.handleMeasureRepresentationItem(node)
					} else {
						cp.processLeave(node, cp.singleEntityMap[instanceType].attributeOrders, cp.rawAttributeValuesMap[node].MixedValues)
					}
				case ObjectProperty:
					switch cp.singleEntityMap[instanceType].name {
					case PROPERTY_DEFINITION_REPRESENTATION, SHAPE_DEFINITION_REPRESENTATION:
						cp.processPropertyDefinitionRepresentation(node, cp.singleEntityMap[instanceType])
					}
				}
			} else {
				cp.processExplicitSSTEntity(node, instanceType)
			}
		}
	}
}

func (cp *conversionParameters) processExplicitSSTEntity(node sst.IBNode, instanceType sst.IBNode) {
	if cp.extraEntityMap[instanceType].ontologyObject == nil {
		return
	}

	switch cp.extraEntityMap[instanceType].name {
	case PRODUCT:
		cp.convertPDMCollection(node)
	case NEXT_ASSEMBLY_USAGE_OCCURRENCE:
		cp.convertNextOccurrence(node)
	default:
		// fmt.Println("---ff--", node.Fragment(), cp.extraEntityMap[instanceType].name, cp.extraEntityMap[instanceType].ontologyObject.Fragment())
	}
}

func (cp *conversionParameters) processComplexEntityInstances() {
	for node, complexValue := range cp.complexInstanceValues {
		for _, instanceType := range complexValue {
			switch cp.rawAttributeValuesMap[instanceType].name {
			case GLOBAL_UNIT_ASSIGNED_CONTEXT:
				cp.handleGlobalUnitAssignedContext(node, instanceType)
			case GLOBAL_UNCERTAINTY_ASSIGNED_CONTEXT:
				cp.handleGlobalUncertaintyAssignedContext(node, instanceType)
			case REPRESENTATION_RELATIONSHIP,
				SHAPE_REPRESENTATION_RELATIONSHIP,
				REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION:
			default:
				if cp.singleEntityMap[instanceType].ontologyObject != nil {
					if !cp.containsIBNode(cp.collectComplexNodes[node], cp.singleEntityMap[instanceType].ontologyObject) {
						cp.collectComplexNodes[node] = append(cp.collectComplexNodes[node], cp.singleEntityMap[instanceType].ontologyObject)
					}
					cp.processLeave(node, cp.singleEntityMap[instanceType].attributeOrders, cp.rawAttributeValuesMap[instanceType].MixedValues)
				}
			}
		}
	}
}

func (cp *conversionParameters) processPropertyDefinitionRepresentation(node sst.IBNode, entity SingleEntity) {
	ontologyObject := entity.ontologyObject
	if ontologyObject == nil {
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
	case PROPERTY_DEFINITION, PRODUCT_DEFINITION_SHAPE:
		return cp.resolveSemanticRepresentationOwner(cp.ibNodeAt(rawAttrValues.MixedValues, 2), seen)
	case SHAPE_ASPECT:
		return cp.resolveSemanticRepresentationOwner(cp.ibNodeAt(rawAttrValues.MixedValues, 2), seen)
	case NEXT_ASSEMBLY_USAGE_OCCURRENCE:
		return cp.singleOccurrenceForNextOccurrence(node)
	case PRODUCT_DEFINITION:
		return node
	default:
		return node
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

func (cp *conversionParameters) processLeave(node sst.IBNode, ontologyAttributes []sst.IBNode, mixedValues []interface{}) {
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
		switch v := mixedVal.(type) {
		case string:
			if cp.isValid(v) {
				node.AddStatement(ontologyAttributes[i].InVocabulary().VocabularyElement(), sst.String(v))
			}
		case sst.String:
			if cp.isValid(string(v)) {
				node.AddStatement(ontologyAttributes[i].InVocabulary().VocabularyElement(), v)
			}
		case sst.IBNode:
			if _, ok := v.AsCollection(); ok {
				cp.processCollectionIbNode(node, v, ontologyAttributes[i])
			} else if !cp.isSkippedParameterNode(v) {
				cp.processSingleIbNode(node, v, ontologyAttributes[i])
			}
		case float64:
			node.AddStatement(ontologyAttributes[i].InVocabulary().VocabularyElement(), sst.Double(v))
		case sst.Double:
			node.AddStatement(ontologyAttributes[i].InVocabulary().VocabularyElement(), v)
		case int64:
			node.AddStatement(ontologyAttributes[i].InVocabulary().VocabularyElement(), sst.Double(v))
		case sst.Integer:
			node.AddStatement(ontologyAttributes[i].InVocabulary().VocabularyElement(), v)
		}
	}
}

func (cp *conversionParameters) processSingleIbNode(node sst.IBNode, ibnodeVal sst.IBNode, attribute sst.IBNode) {
	if value, ok := cp.enumerationValueMap[ibnodeVal]; ok {
		if element, exists := cp.enumerationElement(attribute, value.name); exists {
			node.AddStatement(attribute.InVocabulary().VocabularyElement(), element)
		} else {
			node.AddStatement(attribute.InVocabulary().VocabularyElement(), sst.Boolean(cp.getBooleanValue(value.name)))
		}
	} else {
		if !cp.isSkippedParameterNode(ibnodeVal) {
			node.AddStatement(attribute.InVocabulary().VocabularyElement(), ibnodeVal)
		}
	}
}

func (cp *conversionParameters) processCollectionIbNode(node sst.IBNode, collectionNode sst.IBNode, attribute sst.IBNode) {
	integerPoints := cp.getIntegerCollection(collectionNode)
	floatPoints := cp.getFloatCollection(collectionNode)
	ibnodeCollection := cp.getIbnodeCollection(collectionNode)
	predicate := cp.attributePredicate(attribute)
	if predicate == nil {
		return
	}

	if cp.collectionHasNestedCollection(collectionNode) {
		col, err := cp.createMultiDimensionalCollectionFromTree(ibnodeCollection)
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
	} else if ibnodeCollection.Children != nil && len(ibnodeCollection.Children) > 0 {
		if cp.attributeRangeIsRDFList(attribute) {
			col, err := cp.createMultiDimensionalCollectionFromTree(ibnodeCollection)
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

func (cp *conversionParameters) attributePredicate(attribute sst.IBNode) sst.Node {
	if attribute.Is(ssmeta.StepImMapAttributeSpecial) {
		return rep.EdgeList
	}

	inVocab := attribute.InVocabulary()
	if inVocab == nil {
		return nil
	}
	return inVocab.VocabularyElement()
}

func (cp *conversionParameters) attributeRangeIsRDFList(attribute sst.IBNode) bool {
	if attribute.Is(ssmeta.StepImMapAttributeSpecial) {
		return true
	}

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

func (cp *conversionParameters) getBooleanValue(boolValue string) bool {
	var orientation bool
	switch boolValue {
	case "T":
		orientation = true
	case "F":
		orientation = false
	}
	return orientation
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

func (cp *conversionParameters) getIbnodeCollection(value sst.IBNode) *TreeNode {
	root := &TreeNode{Value: value}
	cp.helperIbnodeCollection(value, root)
	return root
}

func (cp *conversionParameters) helperIbnodeCollection(value sst.IBNode, currentNode *TreeNode) {
	if ibnodeCollection, ok := value.AsCollection(); ok {
		ibnodeCollection.ForMembers(func(_ int, o sst.Term) {
			child := &TreeNode{Value: o}
			currentNode.Children = append(currentNode.Children, child)
			if o.TermKind() == sst.TermKindIBNode || o.TermKind() == sst.TermKindTermCollection {
				if _, isCollection := o.(sst.IBNode).AsCollection(); isCollection {
					cp.helperIbnodeCollection(o.(sst.IBNode), child)
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

// ------------------------- end handling single leave and multi leave conversion --------------------------------

// -------------------------  PDM - start handle part, part version and part design -------------------------
func (cp *conversionParameters) convertPDMCollection(node sst.IBNode) {
	productReferences := cp.searchNode(node)
	for _, product := range productReferences {
		if product != nil {
			switch cp.rawAttributeValuesMap[product].name {
			case PRODUCT_RELATED_PRODUCT_CATEGORY:
				// handle part - PRODUCT - PRODUCT_RELATED_PRODUCT_CATEGORY
				node.AddStatement(rdf.Type, sso.Part)
				cp.processCommonData(node, "")
			case PRODUCT_DEFINITION_FORMATION, PRODUCT_DEFINITION_FORMATION_WITH_SPECIFIED_SOURCE:
				// handle part version - PRODUCT_DEFINITION_FORMATION - PRODUCT_DEFINITION_FORMATION_WITH_SPECIFIED_SOURCE
				product.AddStatement(rdf.Type, sso.PartVersion)
				node.AddStatement(sso.HasPartVersion, product)
				cp.processCommonData(product, VERSION)

				// handle part design - PRODUCT_DEFINITION - PRODUCT_DEFINITION_CONTEXT
				partDesign := cp.searchNode(product)
				for _, design := range partDesign {
					if design != nil {
						nextOccurrenceReferenceExist := false
						product.AddStatement(sso.HasProductDefinition, design)
						cp.processCommonData(design, DESIGN)

						// shape representation - PRODUCT_DEFINITION_SHAPE - SHAPE_DEFINITION_REPRESENTATION
						productDefinitionShape := cp.searchNode(design)
						for _, productShape := range productDefinitionShape {
							if productShape != nil {
								shapeDefinitionRepresentation := cp.searchNode(productShape)
								if cp.rawAttributeValuesMap[productShape].name == PRODUCT_DEFINITION_SHAPE {
									for _, shapeRepresentation := range shapeDefinitionRepresentation {
										if cp.rawAttributeValuesMap[shapeRepresentation].name == SHAPE_DEFINITION_REPRESENTATION {
											for i, shapeRep := range cp.rawAttributeValuesMap[shapeRepresentation].MixedValues {
												if shapeRep, ok := shapeRep.(sst.IBNode); ok && i == 1 {
													design.AddStatement(sso.DefiningGeometry, shapeRep)
												}
											}
										}
										if cp.rawAttributeValuesMap[shapeRepresentation].name == SHAPE_ASPECT {
										}
									}
								}
								if cp.rawAttributeValuesMap[productShape].name == NEXT_ASSEMBLY_USAGE_OCCURRENCE {
									nextOccurrenceReferenceExist = true
								}
							}
						}

						// if product_definition_shape reference exist inside next_assembly_usage_occurrence then use sso.AssemblyDesign
						if nextOccurrenceReferenceExist {
							design.AddStatement(rdf.Type, sso.AssemblyDesign)
						} else {
							design.AddStatement(rdf.Type, sso.PartDesign)
						}
					}
				}
			}
		}
	}
}

func (cp *conversionParameters) processCommonTextData(node sst.IBNode, representationRelationship interface{}, position int) {
	var representationText string
	switch v := representationRelationship.(type) {
	case string:
		representationText = v
	case sst.String:
		representationText = string(v)
	}
	if cp.isValid(representationText) {
		switch position {
		case 0:
			node.AddStatement(rdfs.Label, sst.String(representationText))
		case 1:
			node.AddStatement(rdfs.Comment, sst.String(representationText))
		}
	}
}

func (cp *conversionParameters) processCommonData(node sst.IBNode, partType string) {
	for i, partData := range cp.rawAttributeValuesMap[node].MixedValues {
		var part string
		switch v := partData.(type) {
		case string:
			part = v
		case sst.String:
			part = string(v)
		}
		if cp.isValid(part) {
			if i == 0 {
				switch partType {
				case VERSION:
					node.AddStatement(sso.ViewID, sst.String(part))
				case DESIGN:
					node.AddStatement(sso.VersionID, sst.String(part))
				default:
					node.AddStatement(sso.ID, sst.String(part))
				}
			}
			if i == 1 {
				node.AddStatement(rdfs.Label, sst.String(part))
			}
			if i == 2 {
				node.AddStatement(rdfs.Comment, sst.String(part))
			}
		}
	}
}

func (cp *conversionParameters) searchNode(searchNode sst.IBNode) []sst.IBNode {
	if cp.rawReferenceIndex == nil {
		cp.rawReferenceIndex = cp.buildRawReferenceIndex()
	}
	return cp.rawReferenceIndex[searchNode]
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

func (cp *conversionParameters) convertNextOccurrence(node sst.IBNode) error {
	node.AddStatement(rdf.Type, owl.ObjectProperty)
	node.AddStatement(rdfs.SubPropertyOf, sso.NextAssemblyOccurrenceUsage)

	// create single instance singleOccurrence
	singleInstance := cp.singleOccurrenceForNextOccurrence(node)

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
				if i == 5 { // if reference_designator exist on this position
					singleInstance.AddStatement(sso.ID, sst.String(nextText))
				} else {
					singleInstance.AddStatement(sso.ID, sst.String(nextText))
				}
			case 1:
				singleInstance.AddStatement(rdfs.Label, sst.String(nextText))
			case 2:
				singleInstance.AddStatement(rdfs.Comment, sst.String(nextText))
			}
		}
		if nextIbnode, ok := nextOccurrence.(sst.IBNode); ok {
			if i == 3 {
				// create punning for next_assembly_usage_occurrence and single_occurrence
				nextIbnode.AddStatement(node, singleInstance)
			}
			if i == 4 {
				singleInstance.AddStatement(lci.IsDefinedBy, nextIbnode)
			}
		}
	}

	// process complex leave instance
	nextOccurrenceUsage := cp.searchNode(node)
	for _, nextOccurrence := range nextOccurrenceUsage {
		if nextOccurrence != nil {
			contextDependentShape := cp.searchNode(nextOccurrence)
			for _, contextDependentShape := range contextDependentShape {
				cp.handleComplexLeave(node, contextDependentShape)
			}
		}
	}

	return nil
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

func (cp *conversionParameters) handleComplexLeave(node sst.IBNode, contextDependentShape sst.IBNode) {
	for _, complexInstance := range cp.rawAttributeValuesMap[contextDependentShape].MixedValues {
		if complexNode, ok := complexInstance.(sst.IBNode); ok && len(cp.complexInstanceValues[complexNode]) > 0 {
			node.AddStatement(rep.ContextDependentShapeRepresentation, complexNode)
			complexNode.AddStatement(rdf.Type, owl.ObjectProperty)

			for _, ibnodeVal := range cp.complexInstanceValues[complexNode] {
				switch cp.rawAttributeValuesMap[ibnodeVal].name {
				case REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION:
					complexNode.AddStatement(rdfs.SubPropertyOf, rep.ShapeRepresentationRelationshipWithPlacementTransformation)
					for _, transformationOperator := range cp.rawAttributeValuesMap[ibnodeVal].MixedValues {
						if transformation, ok := transformationOperator.(sst.IBNode); ok {
							// handle item_defined_transformation
							complexNode.AddStatement(rep.TransformationOperator, transformation)
							transformation.AddStatement(rdf.Type, owl.ObjectProperty)
							transformation.AddStatement(rdfs.SubPropertyOf, rep.ItemDefinedTransformation)

							for i, itemDefinedTransformation := range cp.rawAttributeValuesMap[transformation].MixedValues {
								// handle punning for item_defined_transformation
								cp.processCommonTextData(transformation, itemDefinedTransformation, i)
								if itemDefinedNode, ok := itemDefinedTransformation.(sst.IBNode); ok && cp.rawAttributeValuesMap[transformation].MixedValues[2].(sst.IBNode) != nil && i == 3 {
									itemDefinedNode.AddStatement(transformation, cp.rawAttributeValuesMap[transformation].MixedValues[2].(sst.IBNode))
								}
							}
						}
					}
				case REPRESENTATION_RELATIONSHIP:
					for i, representationRelationship := range cp.rawAttributeValuesMap[ibnodeVal].MixedValues {
						// handle punning for representation_relationship_with_placement_transformation
						cp.processCommonTextData(complexNode, representationRelationship, i)
						if representationNode, ok := representationRelationship.(sst.IBNode); ok && cp.rawAttributeValuesMap[ibnodeVal].MixedValues[2].(sst.IBNode) != nil && i == 3 {
							representationNode.AddStatement(complexNode, cp.rawAttributeValuesMap[ibnodeVal].MixedValues[2].(sst.IBNode))
						}
					}
				}
			}
		}
	}
}

func (cp *conversionParameters) containsIBNode(slice []sst.IBNode, item sst.IBNode) bool {
	for _, sliceItem := range slice {
		if sliceItem == item {
			return true
		}
	}
	return false
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

// ------------------------ PDM - end handle part, part version and part design -------------------------

// ------------------------- start handle complex instance type -------------------------
func (cp *conversionParameters) processOntologyOrder() {
	parentMap := make(map[sst.IBNode]sst.IBNode)
	hierarchyLevelMap := make(map[sst.IBNode]int)

	// Collecting parent and child relationships
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

	// Calculate hierarchy levels once
	for node := range parentMap {
		hierarchyLevelMap[node] = cp.getHierarchyLevel(node, parentMap)
	}

	// Sort each array in the map based on the hierarchy level
	cp.sortAndReverseNodes(hierarchyLevelMap)

	// add complexInstanceValue type to the node
	for selectedNode, arrayOfNode := range cp.collectComplexNodes {
		for _, mainClass := range cp.complexMainClasses(arrayOfNode) {
			selectedNode.AddStatement(rdf.Type, mainClass.VocabularyElement())
		}
		for _, node := range arrayOfNode {
			if nodeHasType(node, ssmeta.OptionClass) {
				selectedNode.AddStatement(rdf.Type, node.InVocabulary().VocabularyElement())
			}
		}
	}
}

func (cp *conversionParameters) complexMainClasses(arrayOfNode []sst.IBNode) []sst.ElementInformer {
	var mainClasses []sst.ElementInformer
	for _, node := range arrayOfNode {
		inVocabulary := node.InVocabulary()
		if inVocabulary == nil || !inVocabulary.IsMainClass(sst.Element{}) {
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

// ------------------------- end handle complex instance type -------------------------

// ---------------------------- remove entity instance and attribute values ----------------------------

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

func isParserDefinedTypeMetadataNode(node sst.IBNode) bool {
	for _, object := range node.GetObjects(rdf.Type) {
		typeNode, ok := object.(sst.IBNode)
		if ok && typeNode.Is(ssmeta.DefinedType) {
			return true
		}
	}
	return false
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
