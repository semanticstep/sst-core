// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

// export SST data to STEP AP242 XML data (ISO 10303-242)
package ap242xmlexport

import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/step/ap242xmlimport"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/qau"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/semanticstep/sst-core/vocabularies/ssmeta"
	"github.com/semanticstep/sst-core/vocabularies/sso"
)

type Header struct {
	Name                *string `xml:"Name"`
	TimeStamp           *string `xml:"TimeStamp"`
	Organization        *string `xml:"Organization"`
	PreprocessorVersion *string
}
type IdentifierStruct struct {
	UID          string `xml:"uid,attr"`
	ID           string `xml:"id,attr"`
	IdContextRef string `xml:"idContextRef,attr"` //nolint:golint
	IdRoleRef    string `xml:"idRoleRef,attr"`    //nolint:golint
}
type IDStruct struct {
	ID         *string `xml:"id,attr,omitempty"`
	Identifier *[]IdentifierStruct
}
type ViewContext struct {
	UID               string `xml:"uid,attr"`
	ApplicationDomain string `xml:"ApplicationDomain>PredefinedApplicationDomainEnum"`
	LifeCycleStage    string `xml:"LifeCycleStage>ProxyString"`
}
type Unit struct {
	UID string `xml:"uid,attr"`
	// Id        IDStruct //nolint:golint
	Name     string `xml:"Name>ClassString"`
	Quantity string `xml:"Quantity>ClassString"`
}
type ClassStruct struct {
	UID string `xml:"uid,attr"`
}
type UIDRef struct {
	Ref *string `xml:"uidRef,attr,omitempty"`
}
type Quantity struct {
	XsiType        string `xml:"xsi:type,attr"`
	UID            string `xml:"uid,attr"` //nolint:golint
	Unit           *UIDRef
	ValueComponent float64 `xml:"ValueComponent"`
}
type ViewOccurrenceRelationship struct {
	XsiType      string `xml:"xsi:type,attr"`
	UID          string `xml:"uid,attr"` //nolint:golint
	Related      UIDRef
	RelationType *string    `xml:"RelationType>ClassString,omitempty"`
	Placement    *Placement `xml:"Placement>placement"`
}
type Placement struct {
	XMLName xml.Name
	UIDRef  string `xml:"uidRef,attr"`
}
type ShapeElementRelationship struct {
	XsiType string `xml:"xsi:type,attr"`
	UID     string `xml:"uid,attr"` //nolint:golint
	Related *UIDRef
}
type ShapeElement struct {
	XsiType                    string    `xml:"xsi:type,attr"`
	UID                        string    `xml:"uid,attr"` //nolint:golint
	Id                         *IDStruct //nolint:golint
	Name                       *string   `xml:"Name>CharacterString,omitempty"`
	PartDefinition             *UIDRef
	RepresentedGeometry        *UIDRef
	ShapeElement               []ShapeElement // recursive
	IntendedJointType          *string        `xml:"IntendedJointType>TerminalJointTypeEnum,omitempty"`
	InterfaceOrJoinTerminal    *string        `xml:"InterfaceOrJoinTerminal,omitempty"`
	ShapeElementRelationship   []ShapeElementRelationship
	Definition                 *UIDRef
	JointType                  *string   `xml:"JointType,omitempty"`
	ConnectedTerminals         *[]UIDRef `xml:"ConnectedTerminals>PartTerminal"`
	AssociatedTransportFeature *UIDRef   `xml:"AssociatedTransportFeature"`
	PropertyValueAssignment    *[]PropertyValueAssignment
}
type NestedShapeElement struct {
	ShapeElement *ShapeElement
}
type Occurrence struct {
	XsiType                 string             `xml:"xsi:type,attr"`
	UID                     string             `xml:"uid,attr"` //nolint:golint
	Id                      *IDStruct          //nolint:golint
	Occurrence              []NestedOccurrence `xml:"Occurrence,omitempty"`
	ShapeElement            []ShapeElement
	Quantity                *Quantity `xml:"Quantity,omitempty"`
	UpperUsage              UIDRef    `xml:"-"`
	PropertyValueAssignment *[]PropertyValueAssignment
}
type NestedOccurrence struct {
	Occurrence *Occurrence
}
type PartView struct {
	XsiType                    *string `xml:"xsi:type,attr"`
	UID                        string  `xml:"uid,attr"` //nolint:golint
	DefiningGeometry           *UIDRef
	Id                         *IDStruct //nolint:golint
	InitialContext             UIDRef
	Occurrence                 []Occurrence
	ShapeElement               []ShapeElement
	ViewOccurrenceRelationship []ViewOccurrenceRelationship
	Topology                   *UIDRef
	PropertyValueAssignment    *[]PropertyValueAssignment
}
type PartVersion struct {
	UID                 string     `xml:"uid,attr"`
	Id                  IDStruct   //nolint:golint
	Views               []PartView `xml:"Views>PartView"`
	assemblyDesignFound bool       // flag to check if assembly design is found for p21 file
}
type Part struct {
	UID       string        `xml:"uid,attr"`
	Id        IDStruct      //nolint:golint
	Name      string        `xml:"Name>CharacterString"`
	PartTypes []string      `xml:"PartTypes>PartCategoryEnum"`
	Versions  []PartVersion `xml:"Versions>PartVersion"`
}
type ProductConceptStruct struct {
	XsiType              ap242xmlimport.XSIType `xml:"http://www.w3.org/2001/XMLSchema-instance type,attr"`
	UID                  string                 `xml:"uid,attr"`
	ClassifiedAs         []UIDRef               `xml:"ClassifiedAs>Classification"`
	Id                   IDStruct               //nolint:golint
	Name                 string                 `xml:"Name>CharacterString"`
	ProductConfiguration []ProductConfigurationStruct
}
type ProductConfigurationStruct struct {
	XsiType      ap242xmlimport.XSIType `xml:"http://www.w3.org/2001/XMLSchema-instance type,attr"`
	UID          string                 `xml:"uid,attr"`
	ClassifiedAs []UIDRef               `xml:"ClassifiedAs>Classificationr"`
	Id           IDStruct               //nolint:golint
	Name         string                 `xml:"Name>CharacterString"`
	Occurrence   []Occurrence
	ShapeElement []ShapeElement
}
type Organization struct {
	UID  string   `xml:"uid,attr"`
	Id   IDStruct //nolint:golint
	Name string   `xml:"Name>CharacterString"`
}
type PropertyDefinition struct {
	XsiType      string   `xml:"xsi:type,attr"`
	UID          string   `xml:"uid,attr"`
	Id           IDStruct //nolint:golint
	PropertyType string   `xml:"PropertyType>ClassString"`
}
type ShapeFeatureDefinition struct {
	XsiType          string `xml:"xsi:type,attr"`
	UID              string `xml:"uid,attr"`
	Name             string `xml:"Name>CharacterString"`
	ShapeElement     []ShapeElement
	ShapeFeatureType *string `xml:"ShapeFeatureType"`
}
type RepresentationContext struct {
	XsiType         string               `xml:"xsi:type,attr"`
	UID             string               `xml:"uid,attr"`
	Id              IDStruct             //nolint:golint
	Representations []Representation     `xml:"Representations>Representation"`
	Items           []RepresentationItem `xml:"Items>RepresentationItem"`
	DimensionCount  int64                `xml:"DimensionCount,omitempty"`
}
type Representation struct {
	XsiType                    string               `xml:"xsi:type,attr"`
	UID                        string               `xml:"uid,attr"`
	Id                         IDStruct             //nolint:golint
	Items                      []RepresentationItem `xml:"Items>RepresentationItem"`
	RepresentationRelationship []RepresentationRelationship
	ExternalFile               *UIDRef
}
type RepresentationRelationship struct {
	XsiType      string `xml:"xsi:type,attr"`
	UID          string `xml:"uid,attr"` //nolint:golint
	Definitional string `xml:"Definitional"`
	Related      UIDRef
	Origin       *Origin `xml:"Origin"`
	Target       *Target `xml:"Target"`
}
type Origin struct {
	UidRef        *string  `xml:"uidRef,attr"`
	Path          []UIDRef `xml:"Path"`
	Edge          []UIDRef `xml:"Edge"`
	Vertex        []UIDRef `xml:"Vertex"`
	AxisPlacement []UIDRef `xml:"AxisPlacement"`
}
type Target struct {
	UidRef        *string  `xml:"uidRef,attr"`
	Edge          []UIDRef `xml:"Edge"`
	Path          []UIDRef `xml:"Path"`
	Vertex        []UIDRef `xml:"Vertex"`
	AxisPlacement []UIDRef `xml:"AxisPlacement"`
}
type Edge struct {
	UidRef string `xml:"uidRef,attr"`
}
type Curve struct {
	UidRef string `xml:"uidRef,attr"`
}
type OrientationList struct {
	Boolean *[]bool
}
type PropertyValueAssignment struct {
	UID                    string        `xml:"uid,attr"`
	AssignedPropertyValues PropertyValue `xml:"AssignedPropertyValues>PropertyValue"`
}
type PropertyValue struct {
	XMLName        xml.Name `xml:"PropertyValue"`
	XSIType        string   `xml:"xsi:type,attr"`
	UID            string   `xml:"uid,attr"`
	Unit           UnitDef  `xml:"Unit"`
	ValueComponent float64  `xml:"ValueComponent"`
}
type Definition struct {
	PropertyDefinition string `xml:"uidRef,attr"`
}
type UnitDef struct {
	UID string `xml:"uidRef,attr"`
}
type RepresentationItem struct {
	XsiType        *string  `xml:"xsi:type,attr"`
	UID            *string  `xml:"uid,attr"`
	UidRef         *string  `xml:"uidRef,attr"`
	Name           *string  `xml:"Name>CharacterString"`
	ConnectedEdges *[]Edge  `xml:"ConnectedEdges>Edge"`
	Elements       *[]Curve `xml:"Elements>Curve"`
	ParentEdgeSet  *UIDRef

	// for edge
	EdgeEnd   *UIDRef
	EdgeStart *UIDRef

	// for Path
	EdgeList        *[]Edge `xml:"EdgeList>Edge"`
	OrientationList *OrientationList

	// for EdgeBoundedCurveWithLength
	EdgeGeometry *UIDRef
	SameSense    *bool
	ParentEdge   *UIDRef

	EdgeElement *UIDRef
	Orientation *bool

	// for BoundedCurveWithLength
	CurveLength *float64

	// for PointOnCurve
	BasicCurve *UIDRef
	Parameter  *float64

	// for VertexPoint
	VertexGeometry *UIDRef

	// For AxisPlacement
	Axis         *string
	Position     *string
	Coordinates  *string
	RefDirection *string
}

type StepData struct {
	ID     int
	Type   string
	Label  string
	Points []float64
}

type DataContainer struct {
	XsiType                string `xml:"xsi:type,attr"`
	ViewContext            []ViewContext
	Unit                   []Unit
	FormatProperty         []FormatProperty
	Organization           []Organization
	PropertyDefinition     []PropertyDefinition
	Class                  []ClassStruct
	Part                   []Part
	ProductConcept         []ProductConceptStruct
	ShapeFeatureDefinition []ShapeFeatureDefinition
	RepresentationContext  []RepresentationContext
	File                   []File
}
type Address struct {
	City, State string
}
type DataFormat struct {
	ClassString string `xml:"ClassString"`
}
type FormatProperty struct {
	XMLName    xml.Name   `xml:"FormatProperty"`
	UID        string     `xml:"uid,attr"`
	DataFormat DataFormat `xml:"DataFormat"`
}
type FileLocationIdentification struct {
	UID        string `xml:"uid,attr"`
	SourceId   string `xml:"SourceId"`
	SourceType string `xml:"SourceType"`
}
type FileLocations struct {
	FileLocationIdentification FileLocationIdentification `xml:"FileLocationIdentification"`
}
type File struct {
	XMLName       xml.Name      `xml:"File"`
	UID           string        `xml:"uid,attr"`
	XsiType       string        `xml:"xsi:type,attr"`
	Format        UIDRef        `xml:"FileFormat"`
	FileLocations FileLocations `xml:"FileLocations"`
}

type Result struct {
	XMLName           xml.Name `xml:"n0:Uos"`
	XmlnsXsi          string   `xml:"xmlns:xsi,attr"`
	XmlnsCmn          string   `xml:"xmlns:cmn,attr"`
	XmlnsN0           string   `xml:"xmlns:n0,attr"`
	XsiSchemaLocation string   `xml:"xsi:schemaLocation,attr"`
	Header            Header
	DataContainer     DataContainer
}

type extractedData struct {
	graph                     sst.NamedGraph
	partMap                   map[sst.IBNode]Part
	unitMap                   map[string]Unit
	formatPropertyMap         map[string]FormatProperty
	organizationMap           map[sst.IBNode]Organization
	propertyDefinitionMap     map[sst.IBNode]PropertyDefinition
	ViewContextMap            map[string]ViewContext
	shapeFeatureDefinitionMap map[sst.IBNode]ShapeFeatureDefinition
	representationContextMap  map[string]RepresentationContext
	representationItemMap     map[string][]RepresentationItem
	fileMap                   map[string]File
	uniformCurveElements      []sst.IBNode
}

func newExtractedData(graph sst.NamedGraph) *extractedData {
	ex := new(extractedData)
	ex.graph = graph
	ex.partMap = make(map[sst.IBNode]Part)
	ex.unitMap = make(map[string]Unit)
	ex.formatPropertyMap = make(map[string]FormatProperty)
	ex.organizationMap = make(map[sst.IBNode]Organization)
	ex.propertyDefinitionMap = make(map[sst.IBNode]PropertyDefinition)
	ex.ViewContextMap = make(map[string]ViewContext)
	ex.shapeFeatureDefinitionMap = make(map[sst.IBNode]ShapeFeatureDefinition)
	ex.representationContextMap = make(map[string]RepresentationContext)
	ex.representationItemMap = make(map[string][]RepresentationItem)
	ex.fileMap = make(map[string]File)
	ex.uniformCurveElements = []sst.IBNode{}
	return ex
}

func AP242XmlExport(graph sst.NamedGraph, xmlfile io.Writer) error {
	ex := newExtractedData(graph)

	result := new(Result)
	result.DataContainer.XsiType = "n0:AP242DataContainer"
	result.XmlnsXsi = "http://www.w3.org/2001/XMLSchema-instance"
	result.XmlnsCmn = "http://standards.iso.org/iso/ts/10303/-3000/-ed-1/tech/xml-schema/common"
	result.XmlnsN0 = "http://standards.iso.org/iso/ts/10303/-4442/-ed-1/tech/xml-schema/domain_model"
	result.XsiSchemaLocation = "http://standards.iso.org/iso/ts/10303/-4442/-ed-1/tech/xml-schema/domain_model DomainModel.xsd"

	// assign static data to view context
	ex.ViewContextMap["default_electrical_design_view_context"] = ViewContext{
		UID:               "_" + uuid.New().String(),
		ApplicationDomain: "electrical",
		LifeCycleStage:    "Design",
	}

	ex.unitMap["metre"] = Unit{
		UID:      "_" + uuid.New().String(),
		Name:     "metre",
		Quantity: "length",
	}

	ex.unitMap["milliMetre"] = Unit{
		UID:      "_" + uuid.New().String(),
		Name:     "milli metre",
		Quantity: "length",
	}

	ex.unitMap["squareMilliMetre"] = Unit{
		UID:      "_" + uuid.New().String(),
		Name:     "square milli metre",
		Quantity: "length",
	}

	// Create an instance of FormatProperty
	ex.formatPropertyMap["formatProperty"] = FormatProperty{
		UID: "_" + uuid.New().String(),
		DataFormat: DataFormat{
			ClassString: "ISO 10303-214",
		},
	}

	// Get the outer nodes
	outerNodes, _ := getOuterNodes(graph)

	// Process the outer nodes
	for _, outerNode := range outerNodes {
		ex.processParentNodes(outerNode)
	}

	// prepare the data for the xml output
	assignDataToContainer(ex, result)

	// Generate the XML
	xmlBytes, err := xml.MarshalIndent(result, "", "    ")
	if err != nil {
		return err
	}

	_, err = xmlfile.Write(append([]byte(xml.Header), xmlBytes...))
	return err
}

func stringPtr(s string) *string {
	return &s
}

func handleUUID(node sst.IBNode) *string {
	uuid := string("_" + node.Fragment())
	return &uuid
}

// main object entry point
func getOuterNodes(graph sst.NamedGraph) (map[string]sst.IBNode, error) {
	nodeMap := make(map[string]sst.IBNode)
	err := graph.ForIRINodes(func(t sst.IBNode) error {
		if t.TypeOf() != nil {
			switch t.TypeOf().InVocabulary().(type) {
			case sso.KindPart:
				nodeMap[string(t.Fragment())] = t
			case lci.KindOrganization:
				nodeMap[string(t.Fragment())] = t
			case sso.KindShapeFeatureDefinition:
				nodeMap[string(t.Fragment())] = t
			case rep.KindRepresentationContext:
				nodeMap[string(t.Fragment())] = t
			case lci.KindPropertyDefinition:
				nodeMap[string(t.Fragment())] = t
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return nodeMap, nil
}

// start processing Parent nodes here
func (ex *extractedData) processParentNodes(node sst.IBNode) {
	switch node.TypeOf().InVocabulary().(type) {
	case sso.KindPart:
		ex.addPartData(node)
	case lci.KindOrganization:
		ex.addOrganizationData(node)
	case lci.KindPropertyDefinition:
		ex.addPropertyDefinitionData(node)
	case sso.KindShapeFeatureDefinition:
		ex.addShapeFeatureDefinitionData(node)
	case rep.KindRepresentationContext:
		ex.addRepresentationContextData(node)
	}
}

func (ex *extractedData) addPartData(node sst.IBNode) {
	part := Part{}
	part.UID = *handleUUID(node)

	results := ex.processEnumerationValues(node)
	if value, ok := results["PartCategory"].([]string); ok {
		part.PartTypes = value
	}

	node.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		// only look for forward triples
		if node != s {
			return nil
		}

		if o.TermKind() == sst.TermKindLiteral {
			switch p.InVocabulary().(type) {
			// sub property of ID sso.id
			case sso.IsID:
				if !p.OwningGraph().IsReferenced() {
					uid := "_" + uuid.New().String()
					idroleref := ""
					value := string(o.(sst.String))
					part.Id = IDStruct{Identifier: &[]IdentifierStruct{{ID: value}}}
					organizationID := ex.extractOrganizationID(p)
					if organizationID != "" {
						part.Id = IDStruct{Identifier: &[]IdentifierStruct{{UID: uid, IdContextRef: organizationID, ID: value, IdRoleRef: idroleref}}}
					}
				}
				if p.OwningGraph().IsReferenced() {
					value := string(o.(sst.String))
					part.Id = IDStruct{ID: &value}
				}
			case rdfs.IsLabel:
				part.Name = string(o.(sst.String))
			}
		}

		if p.Is(sso.HasPartVersion) {
			ex.handlePartVersion(node, o.(sst.IBNode), &part)
		}

		return nil
	})
	if part.Versions != nil {
		// handle empty part_category case like p21 file
		if len(part.PartTypes) == 0 {
			if part.Versions[0].assemblyDesignFound {
				part.PartTypes = []string{"assembly"}
			} else {
				part.PartTypes = []string{"detail"}
			}
		}
		ex.partMap[node] = part
	}
}

func (ex *extractedData) handlePartVersion(parentNode sst.IBNode, partVersionNode sst.IBNode, part *Part) {
	partVersion := PartVersion{}
	partVersion.UID = *handleUUID(partVersionNode)
	partVersionNode.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		// consider only forward triplets not inverse or predicate triples
		if partVersionNode != s {
			return nil
		}

		if o.TermKind() == sst.TermKindLiteral {
			value := string(o.(sst.String))
			partVersion.Id = IDStruct{ID: &value}
		}
		if p.Is(sso.HasProductDefinition) {
			if o.(sst.IBNode).TypeOf().Is(sso.AssemblyDesign) {
				partVersion.assemblyDesignFound = true
			}
			ex.handlePartView(parentNode, o.(sst.IBNode), &partVersion)
		}
		return nil
	})
	part.Versions = append(part.Versions, partVersion)
}

func (ex *extractedData) handlePartView(parentNode sst.IBNode, partViewNode sst.IBNode, partVersion *PartVersion) {
	partView := PartView{}
	partView.UID = "_" + uuid.New().String()
	defaultViewContextID := "default_electrical_design_view_context"
	partViewNode.ForAll(func(i int, s, p sst.IBNode, o sst.Term) error {
		partView.InitialContext = UIDRef{Ref: &defaultViewContextID}

		if o.TermKind() == sst.TermKindLiteral {
			if p.Is(sso.ViewID) {
				value := string(o.(sst.String))
				partView.Id = &IDStruct{ID: &value}
			}
		}

		if o.TermKind() == sst.TermKindIBNode {
			if p.Is(rdf.Type) && o.(sst.IBNode).Is(sso.WiringHarnessAssemblyDesign) {
				partView.XsiType = stringPtr("n0:WiringHarnessAssemblyDesign")
			}

			if p.OwningGraph().IsReferenced() {
				switch p.InVocabulary().(type) {
				case sso.IsDefiningGeometry:
					partView.DefiningGeometry = &UIDRef{Ref: handleUUID(o.(sst.IBNode))}
				case lci.IsHasArrangedPart, lci.IsHasFeature:
					shape := &ShapeElement{}
					shape.UID = *handleUUID(o.(sst.IBNode))
					ex.extractShapeElement(o.(sst.IBNode), shape)
					if p.Is(lci.HasFeature) {
						ex.extractSubShapeElement(o.(sst.IBNode), shape, &partView)
					}
					partView.ShapeElement = append(partView.ShapeElement, *shape)
				case lci.IsIsDefinedBy:
					occurrence := &Occurrence{}
					occurrence.UID = *handleUUID(s)
					occurrence.XsiType = "n0:" + string(parentNode.TypeOf().Fragment())
					ex.extractOccurrence(s, occurrence, &partView)
					partView.Occurrence = append(partView.Occurrence, *occurrence)
				case sso.IsTopology:
					partView.Topology = &UIDRef{Ref: handleUUID(o.(sst.IBNode))}
				case sso.IsNextAssemblyOccurrenceUsage:
					viewRelationship := &ViewOccurrenceRelationship{}
					viewRelationship.UID = *handleUUID(o.(sst.IBNode)) + "_" + strconv.Itoa(i)
					viewRelationship.XsiType = "n0:NextAssemblyOccurrenceUsage"
					viewRelationship.Related = UIDRef{Ref: handleUUID(o.(sst.IBNode))}
					viewRelationship.RelationType = stringPtr("next assembly occurrence")
					partView.ViewOccurrenceRelationship = append(partView.ViewOccurrenceRelationship, *viewRelationship)
				}
			} else if !p.OwningGraph().IsReferenced() {
				// handle punning and connection
				viewRelationship := &ViewOccurrenceRelationship{}
				viewRelationship.UID = *handleUUID(p)
				viewRelationship.XsiType = "n0:NextAssemblyOccurrenceUsage"
				viewRelationship.Related = UIDRef{Ref: handleUUID(o.(sst.IBNode))}
				viewRelationship.RelationType = stringPtr("next assembly occurrence")

				// figure out placement
				ex.handleViewPlacement(p, viewRelationship)
				partView.ViewOccurrenceRelationship = append(partView.ViewOccurrenceRelationship, *viewRelationship)
			}
		}
		return nil
	})
	partVersion.Views = append(partVersion.Views, partView)
}

func (ex *extractedData) extractShapeElement(node sst.IBNode, shape *ShapeElement) {
	node.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		switch p.InVocabulary().(type) {
		case rdf.IsType:
			if !strings.Contains(string(o.(sst.IBNode).Fragment()), "_") {
				shape.XsiType = "n0:" + string(o.(sst.IBNode).Fragment())
			}
		case rdfs.IsLabel:
			if value, ok := o.(sst.Double); ok {
				str := strconv.FormatFloat(float64(value), 'f', -1, 64)
				shape.Id = &IDStruct{ID: stringPtr(str)}
			} else {
				shape.Id = &IDStruct{ID: stringPtr(string(o.(sst.String)))}
			}
		case sso.IsID:
			shape.Id = &IDStruct{ID: stringPtr(string(o.(sst.String)))}
		case lci.IsIsDefinedBy:
			if node != o.(sst.IBNode) {
				if node.TypeOf().Fragment() == "PartConnectivityDefinition" {
					collectEdges := s.GetObjects(lci.IsDefinedBy)
					edges := []UIDRef{}
					for _, edge := range collectEdges {
						edges = append(edges, UIDRef{Ref: handleUUID(edge.(sst.IBNode))})
					}
					shape.ConnectedTerminals = &edges
				} else {
					shape.PartDefinition = &UIDRef{Ref: handleUUID(o.(sst.IBNode))}
				}
			}
		case sso.IsRepresentedGeometry:
			if o.TermKind() == sst.TermKindIBNode {
				shape.RepresentedGeometry = &UIDRef{Ref: handleUUID(o.(sst.IBNode))}
			}
			if o.TermKind() == sst.TermKindLiteral {
				shape.RepresentedGeometry = &UIDRef{Ref: stringPtr("_" + string(o.(sst.String)))}
			}
		case sso.IsAssemblyShapeJointItemRelationship:
			shape.ShapeElementRelationship = append(shape.ShapeElementRelationship, ShapeElementRelationship{
				UID:     "_" + uuid.New().String(),
				Related: &UIDRef{Ref: handleUUID(o.(sst.IBNode))},
				XsiType: "n0:AssemblyShapeJointItemRelationship",
			})
		case lci.IsHasArrangedPart:
			results := ex.processEnumerationValues(o.(sst.IBNode))
			if value, ok := results["IntendedJointType"].(string); ok {
				shape.IntendedJointType = &value
			}
			if value, ok := results["InterfaceOrJoinTerminal"].(string); ok {
				shape.InterfaceOrJoinTerminal = &value
			}
			if value, ok := results["JointType"].(string); ok {
				shape.JointType = &value
			}
		}
		return nil
	})
}

func (ex *extractedData) extractOccurrence(node sst.IBNode, occurrence *Occurrence, partView *PartView) {
	var propertyValueAssignments []PropertyValueAssignment

	node.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		switch p.InVocabulary().(type) {
		case rdf.IsType:
			occurrence.XsiType = "n0:" + string(node.TypeOf().Fragment())
		case rdfs.IsLabel, sso.IsID:
			occurrence.Id = &IDStruct{ID: stringPtr(string(o.(sst.String)))}
		case sso.IsOccurrenceQuantity:
			quantity := &Quantity{}
			ex.extractQuantity(o.(sst.IBNode), quantity)
			occurrence.Quantity = quantity
		case lci.IsHasArrangedPart, lci.IsHasFeature:
			shapeElement := &ShapeElement{}
			ex.extractSubShapeElement(o.(sst.IBNode), shapeElement, partView)
			occurrence.ShapeElement = append(occurrence.ShapeElement, *shapeElement)
		case rep.IsLengthValue, sso.IsCrossSectionArea, sso.IsCrossSectionDiameter:
			propertyValueAssignment := PropertyValueAssignment{}
			ex.extractPropertyValueAssignment(o.(sst.IBNode), &propertyValueAssignment)
			propertyValueAssignments = append(propertyValueAssignments, propertyValueAssignment)
		}
		return nil
	})

	if len(propertyValueAssignments) > 0 {
		occurrence.PropertyValueAssignment = &propertyValueAssignments
	}
}

func (ex *extractedData) extractQuantity(node sst.IBNode, quantity *Quantity) {
	node.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if node != s {
			return nil
		}

		if o.TermKind() == sst.TermKindLiteral {
			quantity.UID = "_" + uuid.New().String()
			quantity.XsiType = "n0:NumericalValue"
			quantity.Unit = &UIDRef{Ref: stringPtr(ex.unitMap["metre"].UID)}

			if value, ok := o.(sst.Double); ok {
				quantity.ValueComponent = float64(value)
			}
		}
		return nil
	})
}

func (ex *extractedData) extractSubShapeElement(node sst.IBNode, shapeElement *ShapeElement, partView *PartView) {
	var propertyValueAssignments []PropertyValueAssignment

	node.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if node != s {
			return nil
		}

		if o.TermKind() == sst.TermKindLiteral {
			shapeElement.Name = stringPtr(string(o.(sst.String)))
		}

		if o.TermKind() == sst.TermKindIBNode && o.(sst.IBNode).Is(sso.CableOccurrenceTerminalLocationGroup) {
			ex.extractSubShapeElement(o.(sst.IBNode), shapeElement, partView)
		}

		switch p.InVocabulary().(type) {
		case rdf.IsType:
			shapeElement.UID = *handleUUID(s)
			if !strings.Contains(string(o.(sst.IBNode).Fragment()), "_") {
				if o.(sst.IBNode).Fragment() == "ShapeElement" {
					shapeElement.XsiType = "n0:OccurrenceShapeFeature"
				} else if o.(sst.IBNode).Fragment() == "CrossSectionalPartTransportFeature" {
					shapeElement.XsiType = "n0:CrossSectionalPartShapeElement"
				} else {
					shapeElement.XsiType = "n0:" + string(o.(sst.IBNode).Fragment())
				}
			}
		case lci.IsIsDefinedBy:
			shapeElement.Definition = &UIDRef{Ref: stringPtr(*handleUUID(o.(sst.IBNode)))}
		case lci.IsHasArrangedPart, lci.IsHasFeature:
			subShapeElement := &ShapeElement{}
			ex.extractSubShapeElement(o.(sst.IBNode), subShapeElement, partView)
			shapeElement.ShapeElement = append(shapeElement.ShapeElement, *subShapeElement)
		case lci.IsDirectlyConnectedTo:
			shapeElement.AssociatedTransportFeature = &UIDRef{Ref: handleUUID(o.(sst.IBNode))}
		case rep.IsLengthValue, sso.IsCrossSectionArea, sso.IsCrossSectionDiameter:
			propertyValueAssignment := PropertyValueAssignment{}
			ex.extractPropertyValueAssignment(o.(sst.IBNode), &propertyValueAssignment)
			propertyValueAssignments = append(propertyValueAssignments, propertyValueAssignment)
		}

		// attach any enumeration values that exist
		results := ex.processEnumerationValues(s)
		if value, ok := results["IntendedJointType"].(string); ok {
			shapeElement.IntendedJointType = &value
		}
		if value, ok := results["InterfaceOrJoinTerminal"].(string); ok {
			shapeElement.InterfaceOrJoinTerminal = &value
		}
		if value, ok := results["JointType"].(string); ok {
			shapeElement.JointType = &value
		}

		return nil
	})

	if len(propertyValueAssignments) > 0 {
		partView.PropertyValueAssignment = &propertyValueAssignments
	}
}

func (ex *extractedData) extractPropertyValueAssignment(node sst.IBNode, propertyValueAssignment *PropertyValueAssignment) {
	propertyValueAssignment.UID = "_" + uuid.New().String()
	propertyValueAssignment.AssignedPropertyValues.UID = "_" + uuid.New().String()
	propertyValueAssignment.AssignedPropertyValues.XSIType = "n0:NumericalValue"

	node.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if o.TermKind() == sst.TermKindLiteral {
			if value, ok := o.(sst.Double); ok {
				propertyValueAssignment.AssignedPropertyValues.ValueComponent = float64(value)
			}

			if value, ok := ex.unitMap[string(p.Fragment())]; ok {
				propertyValueAssignment.AssignedPropertyValues.Unit.UID = value.UID
			} else {
				handleUnit := ex.getSSQAU(string(p.Fragment()))
				if handleUnit.UID != "" {
					propertyValueAssignment.AssignedPropertyValues.Unit.UID = handleUnit.UID
				}
			}
		}

		return nil
	})
}

func (ex *extractedData) getSSQAU(unitName string) Unit {
	if result, found := ex.unitMap[unitName]; found {
		return result // Return cached result
	}

	result := Unit{}
	Vocgraph, _ := sst.StaticDictionary().Vocabulary(qau.QAUVocabulary)
	Vocgraph.ForIRINodes(func(node sst.IBNode) error {
		node.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
			if string(s.Fragment()) == unitName {

				unit := Unit{}
				unit.UID = "_" + uuid.New().String()
				if p.Is(rdfs.Domain) {
					unit.Quantity = string(o.(sst.IBNode).Fragment())
				}
				if o.TermKind() == sst.TermKindLiteral {
					if p.Is(rdfs.Label) {
						unit.Name = string(o.(sst.String))
					}
				}
				ex.unitMap[unitName] = unit
				result = ex.unitMap[unitName]
			}
			return nil
		})
		return nil
	})
	return result
}

func (ex *extractedData) handleViewPlacement(node sst.IBNode, viewRelationship *ViewOccurrenceRelationship) {
	node.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if node != s {
			return nil
		}

		if p.Is(rep.ContextDependentShapeRepresentation) {
			o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
				if p.Is(rdfs.SubPropertyOf) {
					if o.(sst.IBNode).Is(rep.GeometryToTopologyModelAssociation) {
						placement := Placement{XMLName: xml.Name{Local: "GeometryToTopologyModelAssociation"}, UIDRef: *handleUUID(s)}
						viewRelationship.Placement = &placement
					}
					if o.(sst.IBNode).Is(rep.ShapeRepresentationRelationshipWithPlacementTransformation) {
						placement := Placement{XMLName: xml.Name{Local: "GeometricRepresentationRelationship"}, UIDRef: *handleUUID(s)}
						viewRelationship.Placement = &placement
					}
				}
				return nil
			})
		}
		return nil
	})
}

func (ex *extractedData) addOrganizationData(node sst.IBNode) {
	organization := Organization{}
	organization.UID = *handleUUID(node)
	node.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if o.TermKind() == sst.TermKindLiteral {
			switch p.InVocabulary().(type) {
			case sso.IsID:
				value := string(o.(sst.String))
				organization.Id = IDStruct{ID: &value}
			case rdfs.IsLabel:
				organization.Name = string(o.(sst.String))
			}
		}
		return nil
	})
	ex.organizationMap[node] = organization
}

func (ex *extractedData) addPropertyDefinitionData(node sst.IBNode) {
	propertyDefinition := PropertyDefinition{}
	propertyDefinition.UID = *handleUUID(node)
	node.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if node != s {
			return nil
		}

		if o.TermKind() == sst.TermKindIBNode {
			propertyDefinition.XsiType = string("n0:" + o.(sst.IBNode).Fragment())
		}

		if o.TermKind() == sst.TermKindLiteral {
			switch p.InVocabulary().(type) {
			case sso.IsID:
				propertyDefinition.Id = IDStruct{ID: stringPtr(string(o.(sst.String)))}
			case rdfs.IsLabel:
				propertyDefinition.PropertyType = string(o.(sst.String))
			}
		}
		return nil
	})
	ex.propertyDefinitionMap[node] = propertyDefinition
}

// add shape feature definition data
func (ex *extractedData) addShapeFeatureDefinitionData(node sst.IBNode) {
	shapeFeatureDefinition := ShapeFeatureDefinition{}
	shapeFeatureDefinition.UID = *handleUUID(node)
	node.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if node != s {
			return nil
		}

		if p.Is(rdfs.Label) {
			shapeFeatureDefinition.Name = string(o.(sst.String))
		}

		if p.Is(rdf.Type) {
			if !strings.Contains(string(o.(sst.IBNode).Fragment()), "_") {
				shapeFeatureDefinition.XsiType = "n0:" + string(o.(sst.IBNode).Fragment())
			}
		}

		if p.Is(lci.HasArrangedPart) {
			shape := &ShapeElement{}
			ex.extractShapeElementRelationship(o.(sst.IBNode), shape)
			shapeFeatureDefinition.ShapeElement = append(shapeFeatureDefinition.ShapeElement, *shape)
		}
		return nil
	})
	ex.shapeFeatureDefinitionMap[node] = shapeFeatureDefinition
}

func (ex *extractedData) extractShapeElementRelationship(node sst.IBNode, shape *ShapeElement) {
	node.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if node != s {
			return nil
		}

		if p.Is(rdf.Type) {
			shape.UID = *handleUUID(node)
			shape.XsiType = "n0:" + string(o.(sst.IBNode).Fragment())
		}

		if o.TermKind() == sst.TermKindIBNode {
			if node != o.(sst.IBNode) {
				if p.Is(lci.IsDefinedBy) {
					shape.Definition = &UIDRef{Ref: handleUUID(o.(sst.IBNode))}
				}

				if p.Is(sso.ShapeFeatureDefinitionOccurrenceElementRelationship) {
					shapeShapeElementRelationship := ShapeElementRelationship{
						UID:     "_" + uuid.New().String(),
						XsiType: "n0:ShapeFeatureDefinitionOccurrenceElementRelationship",
						Related: &UIDRef{Ref: handleUUID(o.(sst.IBNode))},
					}
					shape.ShapeElementRelationship = append(shape.ShapeElementRelationship, shapeShapeElementRelationship)
				}
			}
		}
		return nil
	})
}

// looking for application node corresponding enumeration fields data are defined
func (ex *extractedData) processEnumerationValues(appNode sst.IBNode) map[string]any {
	results := make(map[string]any)
	appNode.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if o.TermKind() == sst.TermKindIBNode || o.TermKind() == sst.TermKindTermCollection {
			if inVocab := o.(sst.IBNode).InVocabulary(); inVocab != nil {
				dictNode, err := sst.StaticDictionary().Element(o.(sst.IBNode).InVocabulary().VocabularyElement())
				if err != nil {
					return err
				}
				if dictNode != nil {
					fPartCategory := false
					fJointType := false
					fTerminalJointType := false
					fInterfaceOrJoinTerminal := false
					fShapeFeatureType := false
					dictNode.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
						if p.Is(rdf.Type) {
							if o.(sst.IBNode).Is(sso.PartCategory) {
								fPartCategory = true
							}
							if o.(sst.IBNode).Is(sso.AssemblyJointType) {
								fJointType = true
							}
							if o.(sst.IBNode).Is(sso.TerminalJointType) {
								fTerminalJointType = true
							}
							if o.(sst.IBNode).Is(sso.InterfaceOrJoinTerminal) {
								fInterfaceOrJoinTerminal = true
							}
							if o.(sst.IBNode).Is(sso.ShapeFeatureType) {
								fShapeFeatureType = true
							}
						}
						return nil
					})

					dictNode.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
						if p.Is(ssmeta.StepDmEnumerationMap) {
							if fPartCategory {
								if res, ok := results["PartCategory"].([]string); ok {
									results["PartCategory"] = append(res, string(o.(sst.String)))
								} else {
									results["PartCategory"] = []string{string(o.(sst.String))}
								}
							}
							if fJointType {
								results["JointType"] = string(o.(sst.String))
							}
							if fTerminalJointType {
								results["IntendedJointType"] = string(o.(sst.String))
							}
							if fInterfaceOrJoinTerminal {
								results["InterfaceOrJoinTerminal"] = string(o.(sst.String))
							}
							if fShapeFeatureType {
								results["ShapeFeatureType"] = string(o.(sst.String))
							}
						}
						return nil
					})
				}
			}
		}
		return nil
	})
	return results
}

func (ex *extractedData) extractOrganizationID(node sst.IBNode) string {
	value := ""
	node.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if node != s {
			return nil
		}

		if o.TermKind() == sst.TermKindIBNode || o.TermKind() == sst.TermKindTermCollection {
			if p.Is(sso.IDOwner) {
				value = *handleUUID(o.(sst.IBNode))
			}
		}
		return nil
	})
	return value
}

func (ex *extractedData) addRepresentationContextData(node sst.IBNode) {
	representationContext := &RepresentationContext{}
	collectedItems := make(map[string][]sst.IBNode)

	node.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		representationContext.UID = *handleUUID(node)

		switch p.InVocabulary().(type) {
		case rdf.IsType:
			representationContext.XsiType = "n0:GeometricCoordinateSpace"
		case rep.IsCoordinateSpaceDimension:
			if value, ok := o.(sst.Integer); ok {
				representationContext.DimensionCount = int64(value)
			} else if value, ok := o.(sst.Double); ok {
				representationContext.DimensionCount = int64(value)
			}
		case sso.IsID:
			representationContext.Id = IDStruct{ID: stringPtr(string(o.(sst.String)))}
		case rep.IsContextOfItems:
			representation := &Representation{}
			ex.extractRepresentations(s, representation, representationContext, collectedItems)

			// check if representation has items or not
			if len(representation.Items) > 0 {
				representationContext.Representations = append(representationContext.Representations, *representation)
			}
		}
		return nil
	})

	// handle items under RepresentationContext
	for key, items := range collectedItems {
		for _, item := range items {
			ex.addRepresentationItem(item, representationContext, key)
		}
		representationContext.Items = append(representationContext.Items, ex.representationItemMap[key]...)
	}

	ex.representationContextMap[string(node.Fragment())] = *representationContext
}

func (ex *extractedData) extractRepresentations(node sst.IBNode, representation *Representation, representationContext *RepresentationContext, collectedItems map[string][]sst.IBNode) {
	node.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		representation.UID = *handleUUID(node)
		if !p.IsUuidFragment() {
			switch p.InVocabulary().(type) {
			case sso.IsID:
				representation.Id.ID = stringPtr(string(o.(sst.String)))
			case rdf.IsType:
				if o.(sst.IBNode).Is(rep.ShapeRepresentation) {
					representation.XsiType = "n0:ComposedGeometricModel"
				} else if o.(sst.IBNode).Is(rep.AdvancedBrepShapeRepresentation) {
					representation.XsiType = "n0:ExternalAdvancedBrepShapeRepresentation"
					ex.createExternalFileReference(representation) // create dummy ExternalFileReference
				} else if o.(sst.IBNode).Is(rep.GeometricallyBounded2dWireframeRepresentation) ||
					o.(sst.IBNode).Is(rep.GeometricallyBoundedWireframeShapeRepresentation) {
					representation.XsiType = string("n0:ExternalGeometricallyBoundedWireframeShapeRepresentation")
					ex.createExternalFileReference(representation) // create dummy ExternalFileReference

				} else {
					representation.XsiType = string("n0:" + o.(sst.IBNode).Fragment())
				}
			case rep.IsItem:
				// handle creating RepresentationItem
				if itemList, ok := o.(sst.IBNode).AsCollection(); ok {
					itemList.ForMembers(func(_ int, o sst.Term) {
						if o.TermKind() == sst.TermKindIBNode {
							collectedItems[representationContext.UID] = append(collectedItems[representationContext.UID], o.(sst.IBNode))
							representation.Items = append(representation.Items, RepresentationItem{UidRef: handleUUID(o.(sst.IBNode))})
						}
					})
				} else {
					collectedItems[representationContext.UID] = append(collectedItems[representationContext.UID], o.(sst.IBNode))
					representation.Items = append(representation.Items, RepresentationItem{UidRef: handleUUID(o.(sst.IBNode))})
				}
			}
		}

		// create RepresentationRelationship
		if p.IsUuidFragment() {
			if node == s {
				representationRelationship := RepresentationRelationship{}
				representationRelationship.UID = *handleUUID(p)
				subProperty := p.GetObjects(rdfs.SubPropertyOf)
				for _, prop := range subProperty {
					if prop.(sst.IBNode).Is(rep.GeometryToTopologyModelAssociation) {
						representationRelationship.XsiType = "n0:GeometryToTopologyModelAssociation"
					}
					if prop.(sst.IBNode).Is(rep.TopologyToGeometryModelAssociation) {
						representationRelationship.XsiType = "n0:TopologyToGeometryModelAssociation"
					}
					if prop.(sst.IBNode).Is(rep.ShapeRepresentationRelationshipWithPlacementTransformation) {
						representationRelationship.XsiType = "n0:GeometricRepresentationRelationshipWithPlacementTransformation"
					}
				}
				representationRelationship.Related = UIDRef{Ref: handleUUID(o.(sst.IBNode))}
				representationRelationship.Definitional = "false"

				relationType := ""
				originMap := map[string][]UIDRef{}
				targetMap := map[string][]UIDRef{}

				// get placement for RepresentationRelationship
				getTransformationOperator := p.GetObjects(rep.TransformationOperator)
				for _, operator := range getTransformationOperator {
					operator.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
						if p.IsUuidFragment() {
							switch representationRelationship.XsiType {
							case "n0:GeometricRepresentationRelationshipWithPlacementTransformation":
								relationType = "first"
								itemTypeOrigin := ex.determineItemType(s)
								itemTypeTarget := ex.determineItemType(o.(sst.IBNode))
								originMap[itemTypeOrigin] = append(originMap[itemTypeOrigin], UIDRef{Ref: handleUUID(s)})
								targetMap[itemTypeTarget] = append(targetMap[itemTypeTarget], UIDRef{Ref: handleUUID(o.(sst.IBNode))})
							case "n0:TopologyToGeometryModelAssociation":
								relationType = "second"
								itemTypeOrigin := ex.determineItemType(s)
								itemTypeTarget := ex.determineItemType(o.(sst.IBNode))
								originMap[itemTypeTarget] = append(originMap[itemTypeTarget], UIDRef{Ref: handleUUID(o.(sst.IBNode))})
								targetMap[itemTypeOrigin] = append(targetMap[itemTypeOrigin], UIDRef{Ref: handleUUID(s)})
							case "n0:GeometryToTopologyModelAssociation":
								relationType = "second"
								itemTypeOrigin := ex.determineItemType(o.(sst.IBNode))
								itemTypeTarget := ex.determineItemType(s)
								originMap[itemTypeOrigin] = append(originMap[itemTypeOrigin], UIDRef{Ref: handleUUID(o.(sst.IBNode))})
								targetMap[itemTypeTarget] = append(targetMap[itemTypeTarget], UIDRef{Ref: handleUUID(s)})
							}
						}
						return nil
					})
				}

				representationRelationship.Origin = &Origin{
					AxisPlacement: originMap["AxisPlacement"],
					Edge:          originMap["Edge"],
					Vertex:        originMap["Vertex"],
					Path:          originMap["Path"],
				}
				representationRelationship.Target = &Target{
					AxisPlacement: targetMap["AxisPlacement"],
					Edge:          targetMap["Edge"],
					Vertex:        targetMap["Vertex"],
					Path:          targetMap["Path"],
				}

				// Process based on relationType and the counts of origin and target items.
				switch relationType {
				case "first":
					for i, origin := range originMap {
						representationRelationship.Origin = &Origin{UidRef: origin[0].Ref}
						representationRelationship.Target = &Target{UidRef: targetMap[i][0].Ref}
					}
					representation.RepresentationRelationship = append(representation.RepresentationRelationship, representationRelationship)
				case "second":
					representation.RepresentationRelationship = append(representation.RepresentationRelationship, representationRelationship)
				}
			}
		}
		return nil
	})
}

func (ex *extractedData) createExternalFileReference(representation *Representation) {
	// create dummy external file reference
	getId := ex.formatPropertyMap["formatProperty"].UID
	getIdRef := "_" + uuid.New().String()
	ex.fileMap["dummyFile"] = File{
		UID:     getIdRef,
		XsiType: "n0:DigitalFile",
		Format:  UIDRef{Ref: &getId},
		FileLocations: FileLocations{
			FileLocationIdentification: FileLocationIdentification{
				UID:        "_" + uuid.New().String(),
				SourceId:   "step-output.stp",
				SourceType: "file",
			},
		},
	}
	representation.ExternalFile = &UIDRef{Ref: &getIdRef}
}

func (ex *extractedData) determineItemType(item sst.IBNode) string {
	switch item.TypeOf().InVocabulary().(type) {
	case rep.IsEdgeBoundedCurveWithLength, rep.IsBoundedCurve, sso.IsHarnessSegment_EdgeBoundedCurveWithLength:
		return "Edge"
	case rep.IsVertexPoint, sso.IsHarnessNode_VertexPoint:
		return "Vertex"
	case rep.IsPath:
		return "Path"
	case rep.IsAxis2Placement2D, rep.IsAxis2Placement3D, rep.IsCartesianPoint, rep.IsUniformCurve:
		return "AxisPlacement"
	}
	return ""
}

func (ex *extractedData) addRepresentationItem(node sst.IBNode, representationContext *RepresentationContext, key string) {
	if items, exists := ex.representationItemMap[key]; exists {
		for _, item := range items {
			if item.UID != nil && *item.UID == *handleUUID(node) {
				return // Node already processed for this key, so return early
			}
		}
	}

	collectItems := []sst.IBNode{}
	representationItem := &RepresentationItem{}
	representationItem.UID = handleUUID(node)
	curves := []Curve{}

	node.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if node != s {
			return nil
		}

		switch p.InVocabulary().(type) {
		case rdf.IsType:
			representationItem.XsiType = stringPtr("n0:" + string(o.(sst.IBNode).Fragment()))
		case rdfs.IsLabel, sso.IsID:
			representationItem.Name = stringPtr(string(o.(sst.String)))
		case rep.IsVertexGeometry:
			representationItem.VertexGeometry = &UIDRef{Ref: handleUUID(o.(sst.IBNode))}
			if !containsNode(collectItems, o.(sst.IBNode)) {
				collectItems = append(collectItems, o.(sst.IBNode))
			}
		case rep.IsEdgeGeometry:
			representationItem.EdgeGeometry = &UIDRef{Ref: handleUUID(o.(sst.IBNode))}
			if !containsNode(collectItems, o.(sst.IBNode)) {
				collectItems = append(collectItems, o.(sst.IBNode))
			}
		case sso.IsRepresentedGeometry:
			if !containsNode(collectItems, o.(sst.IBNode)) {
				collectItems = append(collectItems, o.(sst.IBNode))
			}
		case rep.IsEdgeStart:
			representationItem.EdgeStart = &UIDRef{Ref: handleUUID(o.(sst.IBNode))}
			if !containsNode(collectItems, o.(sst.IBNode)) {
				collectItems = append(collectItems, o.(sst.IBNode))
			}
		case rep.IsEdgeEnd:
			representationItem.EdgeEnd = &UIDRef{Ref: handleUUID(o.(sst.IBNode))}
			if !containsNode(collectItems, o.(sst.IBNode)) {
				collectItems = append(collectItems, o.(sst.IBNode))
			}
		case rep.IsParentEdge:
			representationItem.ParentEdge = &UIDRef{Ref: handleUUID(o.(sst.IBNode))}
			if !containsNode(collectItems, o.(sst.IBNode)) {
				collectItems = append(collectItems, o.(sst.IBNode))
			}
		case rep.IsEdgeElement:
			representationItem.EdgeElement = &UIDRef{Ref: handleUUID(o.(sst.IBNode))}
			if !containsNode(collectItems, o.(sst.IBNode)) {
				collectItems = append(collectItems, o.(sst.IBNode))
			}
		case rep.IsBasisCurve:
			representationItem.BasicCurve = &UIDRef{Ref: handleUUID(o.(sst.IBNode))}
			if !containsNode(collectItems, o.(sst.IBNode)) {
				collectItems = append(collectItems, o.(sst.IBNode))
			}
		case rep.IsPointParameter:
			if o, ok := o.(sst.Double); ok {
				value := float64(o)
				representationItem.Parameter = &value
			}
		case rep.IsOrientation:
			if o, ok := o.(sst.Boolean); ok {
				orientation := bool(o)
				representationItem.Orientation = &orientation
			}
		case rep.IsSameSense:
			if o, ok := o.(sst.Boolean); ok {
				sense := bool(o)
				representationItem.SameSense = &sense
			}
		case rep.IsLocation:
			representationItem.XsiType = stringPtr("n0:AxisPlacement")
			ex.extractAxisRelatedCollection(o.(sst.IBNode), representationItem, "location")
		case rep.IsAxis:
			ex.extractAxisRelatedCollection(o.(sst.IBNode), representationItem, "axis")
		case rep.IsRefDirection:
			ex.extractAxisRelatedCollection(o.(sst.IBNode), representationItem, "refDirection")
		case rep.IsEdgeList:
			edges := []Edge{}
			orientationList := OrientationList{}
			edgeOrientations := []bool{}

			if edgeList, ok := o.(sst.IBNode).AsCollection(); ok {
				edgeList.ForMembers(func(_ int, o sst.Term) {
					if o.TermKind() == sst.TermKindIBNode {
						edges = append(edges, Edge{UidRef: *handleUUID(o.(sst.IBNode))})
						o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
							if o.TermKind() == sst.TermKindLiteral {
								if orientation, ok := o.(sst.Boolean); ok {
									edgeOrientations = append(edgeOrientations, bool(orientation))
								}
							}
							return nil
						})

						if !containsNode(collectItems, o.(sst.IBNode)) {
							collectItems = append(collectItems, o.(sst.IBNode))
						}
					}
				})
			}
			representationItem.EdgeList = &edges
			if len(edgeOrientations) > 0 {
				orientationList.Boolean = &edgeOrientations
				representationItem.OrientationList = &orientationList
			} else {
				for i := 0; i < len(edges); i++ {
					edgeOrientations = append(edgeOrientations, false)
					orientationList.Boolean = &edgeOrientations
					representationItem.OrientationList = &orientationList
				}
			}
		case rep.IsCesEdges:
			collectEdges := s.GetObjects(rep.CesEdges)
			edges := []Edge{}
			for _, edge := range collectEdges {
				edges = append(edges, Edge{UidRef: *handleUUID(edge.(sst.IBNode))})

				if !containsNode(collectItems, edge.(sst.IBNode)) {
					collectItems = append(collectItems, edge.(sst.IBNode))
				}
			}
			representationItem.ConnectedEdges = &edges
		case rep.IsConnectedEdgeSubSet:
			representationItem.ParentEdgeSet = &UIDRef{Ref: handleUUID(o.(sst.IBNode))}
		case rep.IsCoordinates:
			representationItem.XsiType = stringPtr("n0:CartesianPoint")
			var coordinateParts []string
			for i, v := range o.(sst.LiteralCollection).Values() {
				if node.TypeOf().Fragment() == "Axis2Placement2D" && i == 2 {
					continue
				}
				coordinateParts = append(coordinateParts, fmt.Sprintf("%v", v))
			}
			coordinates := strings.Join(coordinateParts, " ")

			if coordinates != "" {
				representationItem.Coordinates = &coordinates
			}
		case rep.IsLengthValue:
			if o.TermKind() == sst.TermKindLiteral {
				if value, ok := o.(sst.Double); ok {
					floatValue := float64(value)
					representationItem.CurveLength = &floatValue
				}
			}
		case rep.IsElement:
			curves = append(curves, Curve{UidRef: *handleUUID(o.(sst.IBNode))})
			representationItem.Elements = &curves
			ex.uniformCurveElements = append(ex.uniformCurveElements, o.(sst.IBNode))
		}
		return nil
	})

	// Add the representationItem to the map for the specified key
	if *representationItem.XsiType != "n0:OrientedEdge" {
		ex.representationItemMap[key] = append(ex.representationItemMap[key], *representationItem)
	}

	// Loop over the collected items to add them
	for _, childNode := range collectItems {
		ex.addRepresentationItem(childNode, representationContext, key)
	}
}

func (ex *extractedData) extractAxisRelatedCollection(node sst.IBNode, representationItem *RepresentationItem, axisType string) {
	node.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if o.TermKind() == sst.TermKindLiteralCollection {
			var positionParts []string
			for i, v := range o.(sst.LiteralCollection).Values() {
				if node.TypeOf().Fragment() == "Axis2Placement2D" && i == 2 {
					continue
				}
				positionParts = append(positionParts, fmt.Sprintf("%v", v))
			}
			position := strings.Join(positionParts, " ")
			if position != "" {
				if axisType == "location" {
					representationItem.Position = &position
				} else if axisType == "axis" {
					representationItem.Axis = &position
				} else if axisType == "refDirection" {
					representationItem.RefDirection = &position
				}
			}
		}
		return nil
	})
}

func containsNode(nodes []sst.IBNode, node sst.IBNode) bool {
	for _, n := range nodes {
		if n == node { // Assuming equality check is sufficient based on your node structure
			return true
		}
	}
	return false
}

// assign extracted data to result
func assignDataToContainer(ex *extractedData, result *Result) {
	// Add the Part to the result
	for _, part := range ex.partMap {
		result.DataContainer.Part = append(result.DataContainer.Part, part)
	}

	// attach view context data
	for _, viewContext := range ex.ViewContextMap {
		result.DataContainer.ViewContext = append(result.DataContainer.ViewContext, viewContext)
	}

	// attach organization data
	for _, organization := range ex.organizationMap {
		result.DataContainer.Organization = append(result.DataContainer.Organization, organization)
	}

	// attach unit data
	for _, unit := range ex.unitMap {
		result.DataContainer.Unit = append(result.DataContainer.Unit, unit)
	}

	// attach format property data
	for _, format := range ex.formatPropertyMap {
		result.DataContainer.FormatProperty = append(result.DataContainer.FormatProperty, format)
	}

	// attach propertyDefinition data
	for _, propDefinition := range ex.propertyDefinitionMap {
		result.DataContainer.PropertyDefinition = append(result.DataContainer.PropertyDefinition, propDefinition)
	}

	// Add the ShapeDefinition to the result
	for _, shapeFeature := range ex.shapeFeatureDefinitionMap {
		result.DataContainer.ShapeFeatureDefinition = append(result.DataContainer.ShapeFeatureDefinition, shapeFeature)
	}

	// Add the RepresentationContext to the result
	for _, context := range ex.representationContextMap {
		result.DataContainer.RepresentationContext = append(result.DataContainer.RepresentationContext, context)
	}

	// Add the File to the result
	for _, file := range ex.fileMap {
		result.DataContainer.File = append(result.DataContainer.File, file)
	}
}
