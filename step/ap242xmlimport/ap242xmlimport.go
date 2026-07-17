// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

// import STEP AP242 XML data (ISO 10303-242) into SST
package ap242xmlimport

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/semanticstep/sst-core/sst"
	_ "github.com/semanticstep/sst-core/vocabularies/dict" // register vocabulary map
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/owl"
	"github.com/semanticstep/sst-core/vocabularies/qau"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/semanticstep/sst-core/vocabularies/ssmeta"
	"github.com/semanticstep/sst-core/vocabularies/sso"
)

const (
	cIdMain    = iota //nolint:golint
	cIdVersion        //nolint:golint
	cIdView           //nolint:golint
)

const domainModel = "http://standards.iso.org/iso/ts/10303/-4442/-ed-1/tech/xml-schema/domain_model"

var logger = log.New(os.Stderr, "[ap242xmlimport] ", log.Lmsgprefix)

var (
	ErrInvalidPosition                         = errors.New("missing position inside AxisPlacement")
	ErrInvalidShapeFeatureType                 = errors.New("shape feature type not known")
	ErrOccurrenceNotFound                      = errors.New("occurrence not found")
	ErrParentOccurrenceNotFound                = errors.New("parent occurrence not found")
	ErrRepresentationContextNotFound           = errors.New("represented item not found")
	ErrRepresentationNotFound                  = errors.New("representation context not found")
	ErrRepresentationItemNotFound              = errors.New("representation item not found")
	ErrRepresentationRelationshipNotFound      = errors.New("representation relationship not found")
	ErrShapeElementDefinitionNotFound          = errors.New("shape element definition not found")
	ErrShapeElementNotFound                    = errors.New("shape element not found")
	ErrShapeElementRelationshipRelatedNotKnown = errors.New("shape element relationship related not known")
	ErrViewNotFound                            = errors.New("view not found")
	ErrViewOccurrenceRelationshipNotFound      = errors.New("view occurrence relationship not found")
	ErrOriginTargetLengthMismatch              = errors.New("Origin and Target length does not match")
)

type HeaderStruct struct {
	Name                string `xml:"Name"`
	TimeStamp           string `xml:"TimeStamp"`
	Organization        string `xml:"Organization"`
	PreprocessorVersion string
}

type IdentifierStruct struct {
	UID          string `xml:"uid,attr"`
	ID           string `xml:"id,attr"`
	IdContextRef string `xml:"idContextRef,attr"` //nolint:golint
}

type IDStruct struct {
	ID         string `xml:"id,attr"`
	Identifier []IdentifierStruct
}

type ViewContextStruct struct {
	UID               string `xml:"uid,attr"`
	ApplicationDomain string `xml:"ApplicationDomain>PredefinedApplicationDomainEnum"`
	LifeCycleStage    string `xml:"LifeCycleStage>ProxyString"`
}

type UnitStruct struct {
	UID      string   `xml:"uid,attr"`
	Id       IDStruct //nolint:golint
	Name     string   `xml:"Name>ClassString"`
	Quantity string   `xml:"Quantity>ClassString"`
}

type ClassStruct struct {
	UID string `xml:"uid,attr"`
}

type UIDRef struct {
	Ref string `xml:"uidRef,attr"`
}

type QuantityStruct struct {
	Uid            string  `xml:"uid,attr"` //nolint:golint
	XsiType        XSIType `xml:"http://www.w3.org/2001/XMLSchema-instance type,attr"`
	Unit           UIDRef
	ValueComponent float64 `xml:"ValueComponent"`
}

type ViewOccurrenceRelationshipStruct struct {
	Uid       string  `xml:"uid,attr"` //nolint:golint
	XsiType   XSIType `xml:"http://www.w3.org/2001/XMLSchema-instance type,attr"`
	Related   UIDRef
	Placement struct {
		GeometricRepresentationRelationship []UIDRef
		GeometryToTopologyModelAssociation  []UIDRef
	}
}

type ShapeElementRelationshipStruct struct {
	Uid     string  `xml:"uid,attr"` //nolint:golint
	XsiType XSIType `xml:"http://www.w3.org/2001/XMLSchema-instance type,attr"`
	Related UIDRef
	//	Definition   DefinitionStruct
}

type ShapeElementStruct struct {
	Uid                      string   `xml:"uid,attr"` //nolint:golint
	XsiType                  XSIType  `xml:"http://www.w3.org/2001/XMLSchema-instance type,attr"`
	Id                       IDStruct //nolint:golint
	Code                     UIDRef
	Definition               UIDRef
	PartDefinition           UIDRef
	ShapeElement             []ShapeElementStruct // recursive
	RepresentedGeometry      UIDRef
	ShapeElementRelationship []ShapeElementRelationshipStruct
	DomainType               string
	InterfaceOrJoinTerminal  string
	IntendedJointType        []string `xml:"IntendedJointType>TerminalJointTypeEnum"`
	JointType                string   `xml:"JointType"`
	ConnectedTerminals       []UIDRef `xml:"ConnectedTerminals>PartTerminal"`
}

type OccurrenceStruct struct {
	Uid          string   `xml:"uid,attr"` //nolint:golint
	XsiType      XSIType  `xml:"http://www.w3.org/2001/XMLSchema-instance type,attr"`
	Id           IDStruct //nolint:golint
	Quantity     QuantityStruct
	Occurrence   []OccurrenceStruct // recursive
	ShapeElement []ShapeElementStruct
	UpperUsage   UIDRef
}

type PartViewStruct struct {
	Uid                        string   `xml:"uid,attr"` //nolint:golint
	XsiType                    XSIType  `xml:"http://www.w3.org/2001/XMLSchema-instance type,attr"`
	Id                         IDStruct //nolint:golint
	InitialContext             UIDRef
	AdditionalContexts         []UIDRef `xml:"AdditionalContexts>ViewContext"`
	Occurrence                 []OccurrenceStruct
	ViewOccurrenceRelationship []ViewOccurrenceRelationshipStruct
	ShapeElement               []ShapeElementStruct
	DefiningGeometry           UIDRef
	AuxiliaryGeometry          []UIDRef
	Topology                   UIDRef
}

type PartVersionStruct struct {
	UID   string           `xml:"uid,attr"`
	Id    IDStruct         //nolint:golint
	Views []PartViewStruct `xml:"Views>PartView"`
}

type PartStruct struct {
	UID       string              `xml:"uid,attr"`
	Id        IDStruct            //nolint:golint
	Name      string              `xml:"Name>CharacterString"`
	PartTypes []string            `xml:"PartTypes>PartCategoryEnum"`
	Versions  []PartVersionStruct `xml:"Versions>PartVersion"`
}

type ProductConceptStruct struct {
	UID          string   `xml:"uid,attr"`
	XsiType      XSIType  `xml:"http://www.w3.org/2001/XMLSchema-instance type,attr"`
	ClassifiedAs []UIDRef `xml:"ClassifiedAs>Classification"`
	Id           IDStruct //nolint:golint
	Name         string   `xml:"Name>CharacterString"`
	// Types []string            `xml:"PartTypes>PartCategoryEnum"`  // missing in AP242 so far
	ProductConfiguration []ProductConfigurationStruct
}

type ProductConfigurationStruct struct {
	UID          string   `xml:"uid,attr"`
	XsiType      XSIType  `xml:"http://www.w3.org/2001/XMLSchema-instance type,attr"`
	ClassifiedAs []UIDRef `xml:"ClassifiedAs>Classificationr"`
	Id           IDStruct //nolint:golint
	Name         string   `xml:"Name>CharacterString"`
	Occurrence   []OccurrenceStruct
	ShapeElement []ShapeElementStruct
}

type OrganizationStruct struct {
	UID  string   `xml:"uid,attr"`
	Id   IDStruct //nolint:golint
	Name string   `xml:"Name>CharacterString"`
}

type PropertyDefinitionStruct struct {
	UID          string   `xml:"uid,attr"`
	XsiType      XSIType  `xml:"http://www.w3.org/2001/XMLSchema-instance type,attr"`
	Id           IDStruct //nolint:golint
	Name         string   `xml:"Name>CharacterString"`
	PropertyType string   `xml:"PropertyType>ClassString"`
}

type ShapeFeatureDefinitionStruct struct {
	UID              string   `xml:"uid,attr"`
	XsiType          XSIType  `xml:"http://www.w3.org/2001/XMLSchema-instance type,attr"`
	Id               IDStruct //nolint:golint
	Name             string   `xml:"Name>CharacterString"`
	ShapeFeatureType string   `xml:"ShapeFeatureType"`
	ShapeElement     []ShapeElementStruct
}

type RepresentationContextStruct struct {
	UID     string   `xml:"uid,attr"`
	XsiType XSIType  `xml:"http://www.w3.org/2001/XMLSchema-instance type,attr"`
	Id      IDStruct //nolint:golint
	// Units
	DimensionCount  *int
	Representations []RepresentationStruct     `xml:"Representations>Representation"`
	Items           []RepresentationItemStruct `xml:"Items>RepresentationItem"`
}

type RepresentationStruct struct {
	UID                        string                     `xml:"uid,attr"`
	XsiType                    XSIType                    `xml:"http://www.w3.org/2001/XMLSchema-instance type,attr"`
	Id                         IDStruct                   //nolint:golint
	Items                      []RepresentationItemStruct `xml:"Items>RepresentationItem"`
	RepresentationRelationship []RepresentationRelationshipStruct
}

type Origin struct {
	UidRef        string   `xml:"uidRef,attr"`
	AxisPlacement []UIDRef `xml:"AxisPlacement"`
	Vertex        UIDRef   `xml:"Vertex"`
	Edge          UIDRef   `xml:"Edge"`
}

type Target struct {
	UidRef        string   `xml:"uidRef,attr"`
	Path          UIDRef   `xml:"Path"`
	AxisPlacement []UIDRef `xml:"AxisPlacement"`
	Edge          UIDRef   `xml:"Edge"`
}

type RepresentationRelationshipStruct struct {
	Uid          string  `xml:"uid,attr"` //nolint:golint
	XsiType      XSIType `xml:"http://www.w3.org/2001/XMLSchema-instance type,attr"`
	Related      UIDRef
	Definitional bool    `xml:"Definitional"`
	Origin       *Origin `xml:"Origin"`
	Target       *Target `xml:"Target"`
}

type RepresentationItemStruct struct {
	UID            string   `xml:"uid,attr"`
	UidRef         string   `xml:"uidRef,attr"` //nolint:golint
	XsiType        XSIType  `xml:"http://www.w3.org/2001/XMLSchema-instance type,attr"`
	Name           string   `xml:"Name>CharacterString"`
	Id             IDStruct //nolint:golint
	ConnectedEdges struct {
		Edge []UIDRef
	}
	// for ConnectedEdgeSubSet
	ParentEdgeSet struct {
		UidRef string `xml:"uidRef,attr"` //nolint:golint
	}

	// for BoundedCurveWithLength
	CurveLength float64

	// for edge
	EdgeEnd   UIDRef
	EdgeStart UIDRef
	// for EdgeBoundedCurveWithLength
	EdgeGeometry UIDRef
	SameSense    bool
	// for SubEdge
	ParentEdge UIDRef

	// for VertexPoint
	VertexGeometry UIDRef

	// for PointOnCurve
	BasicCurve UIDRef
	Parameter  float64

	// for Path
	EdgeList struct {
		Edge []UIDRef
	}
	OrientationList struct {
		Boolean []bool
	}
	Axis         string
	Position     string
	RefDirection string
}

type DataContainerStruct struct {
	XsiType                string `xml:"type,attr"`
	ViewContext            []ViewContextStruct
	Unit                   []UnitStruct
	Organization           []OrganizationStruct
	PropertyDefinition     []PropertyDefinitionStruct
	Class                  []ClassStruct
	Part                   []PartStruct
	ProductConcept         []ProductConceptStruct
	ShapeFeatureDefinition []ShapeFeatureDefinitionStruct
	RepresentationContext  []RepresentationContextStruct
}

type Address struct {
	City, State string
}

type Result struct {
	XMLName       xml.Name `xml:"Uos"`
	Header        HeaderStruct
	DataContainer DataContainerStruct
}

// reference maps:

type conversionParameters struct {
	graph                         sst.NamedGraph
	viewMap                       map[string]sst.IBNode
	viewContextMap                map[string]ViewContextStruct
	occurrenceMap                 map[string]sst.IBNode
	viewOccurrenceRelationshipMap map[string]sst.IBNode
	unitMap                       map[string]sst.IBNode
	organizationMap               map[string]sst.IBNode
	idOwnerMap                    map[string]sst.IBNode
	partMap                       map[string]sst.IBNode
	partVersionMap                map[string]sst.IBNode
	propertyDefinitionMap         map[string]sst.IBNode
	shapeElementMap               map[string]sst.IBNode // also for shapeFeatureDefinition
	representationContextMap      map[string]sst.IBNode
	representationMap             map[string]sst.IBNode
	representationRelationshipMap map[string]sst.IBNode
	representationItemMap         map[string]sst.IBNode

	orientedEdgesTrue  map[sst.IBNode]sst.IBNode
	orientedEdgesFalse map[sst.IBNode]sst.IBNode

	enumerationCache map[string]sst.ElementInformer
}

func newConversionParameters(graph sst.NamedGraph) *conversionParameters {
	cp := new(conversionParameters)
	cp.graph = graph
	cp.viewMap = make(map[string]sst.IBNode)
	cp.viewContextMap = make(map[string]ViewContextStruct)
	cp.occurrenceMap = make(map[string]sst.IBNode)
	cp.viewOccurrenceRelationshipMap = make(map[string]sst.IBNode)
	cp.unitMap = make(map[string]sst.IBNode)
	cp.organizationMap = make(map[string]sst.IBNode)
	cp.idOwnerMap = make(map[string]sst.IBNode)
	cp.partMap = make(map[string]sst.IBNode)
	cp.partVersionMap = make(map[string]sst.IBNode)
	cp.propertyDefinitionMap = make(map[string]sst.IBNode)
	cp.shapeElementMap = make(map[string]sst.IBNode)
	cp.representationContextMap = make(map[string]sst.IBNode)
	cp.representationMap = make(map[string]sst.IBNode)
	cp.representationRelationshipMap = make(map[string]sst.IBNode)
	cp.representationItemMap = make(map[string]sst.IBNode)

	cp.orientedEdgesTrue = make(map[sst.IBNode]sst.IBNode)
	cp.orientedEdgesFalse = make(map[sst.IBNode]sst.IBNode)

	cp.enumerationCache = make(map[string]sst.ElementInformer)

	return cp
}

func (cp *conversionParameters) createOrientedEdge(edge sst.IBNode, dir bool) (sst.IBNode, error) {
	var oe sst.IBNode
	if dir {
		oe = cp.orientedEdgesTrue[edge]
		if oe == nil {
			oe = cp.graph.CreateIRINode("", rep.OrientedEdge)
			cp.orientedEdgesTrue[edge] = oe
			oe.AddStatement(rep.EdgeElement, edge)
			oe.AddStatement(rep.Orientation, sst.Boolean(true))
		}
	} else {
		oe = cp.orientedEdgesFalse[edge]
		if oe == nil {
			oe = cp.graph.CreateIRINode("", rep.OrientedEdge)
			cp.orientedEdgesFalse[edge] = oe
			oe.AddStatement(rep.EdgeElement, edge)
			oe.AddStatement(rep.Orientation, sst.Boolean(false))
		}
	}
	return oe, nil
}

// no, re-using shapeElement
// var shapeFeatureDefinitionMap = make(map[string]sst.IBNode)

// var unitMap = make(map[string]sst.IBNode)

func (cp *conversionParameters) processShapeElementLoop1(se ShapeElementStruct, parent sst.IBNode) error {
	var dt sst.Node
	switch se.XsiType {
	// PartShapeElements
	case XSIType{Space: domainModel, Local: "PartShapeElement"}:
		dt = sso.PartShapeElement
	case XSIType{Space: domainModel, Local: "AssemblyShapeJoint"}:
		dt = sso.AssemblyShapeJoint
	case XSIType{Space: domainModel, Local: "AssemblyShapeConstraint"}:
		dt = sso.AssemblyShapeConstraint
	case XSIType{Space: domainModel, Local: "PartConnectivityDefinition"}:
		dt = sso.PartConnectivityDefinition
	case XSIType{Space: domainModel, Local: "HarnessSegment"}:
		dt = sso.HarnessSegment
	case XSIType{Space: domainModel, Local: "HarnessNode"}:
		dt = sso.HarnessNode
	case XSIType{Space: domainModel, Local: "PartFeature"}:
		dt = sso.PartFeature
	case XSIType{Space: domainModel, Local: "PartContactFeature"}:
		dt = sso.PartContactFeature
	case XSIType{Space: domainModel, Local: "PartTerminal"}:
		dt = sso.PartTerminal
	case XSIType{Space: domainModel, Local: "PartTransportFeature"}:
		dt = sso.PartTransportFeature
	case XSIType{Space: domainModel, Local: "CrossSectionalPartShapeElement"}:
		dt = sso.CrossSectionalPartShapeElement
	case XSIType{Space: domainModel, Local: "CrossSectionalGroupShapeElement"}:
		dt = sso.CrossSectionalGroupShapeElement
	case XSIType{Space: domainModel, Local: "CrossSectionalGroupShapeElementWithTubularCover"}:
		dt = sso.CrossSectionalGroupShapeElementWithTubularCover
	case XSIType{Space: domainModel, Local: "CrossSectionalGroupShapeElementWithLacing"}:
		dt = sso.CrossSectionalGroupShapeElementWithLacing
	case XSIType{Space: domainModel, Local: "TwistedCrossSectionalGroupShapeElement"}:
		dt = sso.TwistedCrossSectionalGroupShapeElement
	case XSIType{Space: domainModel, Local: "CrossSectionalAlternativePartShapeElement"}:
		dt = sso.CrossSectionalAlternativePartShapeElement
	// OccurrenceShapeElements
	case XSIType{Space: domainModel, Local: "OccurrenceShapeElement"}:
		dt = sso.OccurrenceShapeElement
	case XSIType{Space: domainModel, Local: "OccurrenceContactFeature"}:
		dt = sso.OccurrenceContactFeature
	case XSIType{Space: domainModel, Local: "OccurrenceTerminal"}:
		dt = sso.OccurrenceTerminal
	case XSIType{Space: domainModel, Local: "WireOccurrenceTerminal"}:
		dt = sso.WireOccurrenceTerminal
	case XSIType{Space: domainModel, Local: "CableOccurrenceTerminalLocationGroup"}:
		dt = sso.CableOccurrenceTerminalLocationGroup
	case XSIType{Space: domainModel, Local: "CableOccurrenceTerminal"}:
		dt = sso.CableOccurrenceTerminal
	case XSIType{Space: domainModel, Local: "CrossSectionalOccurrenceShapeElement"}:
		dt = sso.CrossSectionalOccurrenceShapeElement
	case XSIType{Space: domainModel, Local: "OccurrenceTransportFeature"}:
		dt = sso.OccurrenceTransportFeature
	case XSIType{Space: domainModel, Local: "WireOccurrenceIdentification"}:
		dt = sso.WireOccurrenceIdentification
	case XSIType{Space: domainModel, Local: "ShapeFeatureDefinitionElement"}:
		dt = sso.ShapeFeatureDefinitionElement
	case XSIType{Space: domainModel, Local: "ShapeFeatureDefinitionOccurrenceElement"}:
		dt = sso.ShapeFeatureDefinitionOccurrenceElement
	default: // ShapeElement
		dt = sso.ShapeElement
	}
	shapeElement := cp.graph.CreateIRINode("", dt)

	cp.shapeElementMap[se.Uid] = shapeElement
	// logger.Printf("shapeElement uid=: %v\n", se.Uid)
	if se.Id.ID != "" {
		shapeElement.AddStatement(rdfs.Label, sst.String(se.Id.ID))
	}
	parent.AddStatement(lci.HasArrangedPart, shapeElement)
	for _, shapeElementLoop := range se.ShapeElement {
		if err := cp.processShapeElementLoop1(shapeElementLoop, shapeElement); err != nil {
			return err
		}
	}
	return nil
}

func (cp *conversionParameters) processShapeElementLoop2(se ShapeElementStruct) error {
	// logger.Printf("shapeElement Loop2 uid=: %v\n", se.Uid)
	shapeElement := cp.shapeElementMap[se.Uid]
	if shapeElement == nil {
		return ErrShapeElementNotFound
	}
	if se.Definition.Ref == "" && se.PartDefinition.Ref != "" {
		se.Definition.Ref = se.PartDefinition.Ref
		se.PartDefinition.Ref = ""
	}
	if se.Code.Ref != "" {
		wireColorCode := cp.propertyDefinitionMap[se.Code.Ref]
		if wireColorCode == nil {
			return ErrShapeElementNotFound
		}
		shapeElement.AddStatement(sso.ColourCode, wireColorCode)
	}
	if se.Definition.Ref != "" {
		// logger.Printf("shapeElementRef uidRef=: %v for shapeElement Uid=%v\n", se.Definition.Ref, se.Uid)
		shapeElementDef := cp.shapeElementMap[se.Definition.Ref]
		if shapeElementDef == nil {
			shapeElementDef = cp.shapeElementMap[se.Definition.Ref]
		}
		if shapeElementDef == nil {
			return ErrShapeElementDefinitionNotFound
		}

		shapeElementNow := cp.shapeElementMap[se.Uid]
		if shapeElementNow == nil {
			return ErrShapeElementNotFound
		}

		shapeElementNow.AddStatement(lci.IsDefinedBy, shapeElementDef)
	}
	if se.RepresentedGeometry.Ref != "" {
		repItem, found := cp.representationItemMap[se.RepresentedGeometry.Ref]
		if !found {
			return ErrRepresentationItemNotFound
		}
		shapeElement.AddStatement(sso.RepresentedGeometry, repItem)
	}

	for _, shapeElementLoop := range se.ShapeElement {
		if err := cp.processShapeElementLoop2(shapeElementLoop); err != nil {
			return err
		}
	}
	for _, shapeElementRelationshipLoop := range se.ShapeElementRelationship {
		var dt sst.Node
		switch shapeElementRelationshipLoop.XsiType {
		case XSIType{Space: domainModel, Local: "AssemblyShapeConstraintItemRelationship"}:
			dt = sso.AssemblyShapeConstraintItemRelationship // is Abstract
		case XSIType{Space: domainModel, Local: "AssemblyShapeJointItemRelationship"}:
			dt = sso.AssemblyShapeJointItemRelationship
		case XSIType{Space: domainModel, Local: "ShapeFeatureDefinitionElementRelationship"}:
			dt = sso.ShapeFeatureDefinitionElementRelationship
		case XSIType{Space: domainModel, Local: "ShapeFeatureDefinitionOccurrenceElementRelationship"}:
			dt = sso.ShapeFeatureDefinitionOccurrenceElementRelationship
		default:
			dt = sso.ShapeElementRelationship
		}
		shapeElementRef := cp.shapeElementMap[shapeElementRelationshipLoop.Related.Ref]
		// logger.Printf("occurrence uidRef=: %v\n", shapeElementRelationshipLoop.Related.Ref)
		if shapeElementRef == nil {
			return fmt.Errorf("%w: uidRef %v", ErrShapeElementRelationshipRelatedNotKnown, shapeElementRelationshipLoop.Related.Ref)
		}

		shapeElement.AddStatement(dt, shapeElementRef)
	}
	if se.JointType != "" {
		sJT := "AssemblyJointType_" + se.JointType
		dt, err := sso.SSOVocabulary.ElementInformer(sJT)
		if err != nil {
			return fmt.Errorf("%w for %v", err, sJT)
		}
		shapeElement.AddStatement(rdf.Type, dt)
	}
	if se.DomainType != "" {
		sDT := "TerminalAndTransportDomainType_" + se.DomainType
		dt, err := sso.SSOVocabulary.ElementInformer(sDT)
		if err != nil {
			return fmt.Errorf("%w for %v", err, sDT)
		}
		shapeElement.AddStatement(rdf.Type, dt)
	}
	if se.IntendedJointType != nil {
		for _, ijt := range se.IntendedJointType {
			sIJT := "TerminalJointType_" + ijt
			dt, err := sso.SSOVocabulary.ElementInformer(sIJT)
			if err != nil {
				return fmt.Errorf("%w for %v", err, sIJT)
			}
			shapeElement.AddStatement(rdf.Type, dt)
		}
	}
	if se.InterfaceOrJoinTerminal != "" {
		sIoJT := "InterfaceOrJoinTerminal_" + se.InterfaceOrJoinTerminal
		dt, err := sso.SSOVocabulary.ElementInformer(sIoJT)
		if err != nil {
			return fmt.Errorf("%w for %v", err, sIoJT)
		}
		shapeElement.AddStatement(rdf.Type, dt)
	}

	if se.ConnectedTerminals != nil {
		for _, ct := range se.ConnectedTerminals {

			shapeElementRef := cp.shapeElementMap[ct.Ref]
			if shapeElementRef == nil {
				return fmt.Errorf("%w: uidRef %v", ErrShapeElementRelationshipRelatedNotKnown, ct.Ref)
			}

			shapeElement.AddStatement(lci.IsDefinedBy, shapeElementRef)
		}
	}
	return nil
}

func (cp *conversionParameters) processOccurrenceLoop1(os OccurrenceStruct, parent sst.IBNode) error {
	var dt sst.Term
	switch os.XsiType {
	case XSIType{Space: domainModel, Local: "SingleOccurrence"}:
		dt = sso.SingleOccurrence
	case XSIType{Space: domainModel, Local: "QuantifiedOccurrence"}:
		dt = sso.QuantifiedOccurrence
	case XSIType{Space: domainModel, Local: "WireOccurrence"}:
		dt = sso.WireOccurrence
	case XSIType{Space: domainModel, Local: "CableOccurrence"}:
		dt = sso.CableOccurrence
	case XSIType{Space: domainModel, Local: "SpecifiedOccurrence"}:
		dt = sso.SpecifiedOccurrence
	default:
		dt = sso.ProductOccurrence
	}
	occurrence := cp.graph.CreateIRINode("", dt)

	cp.occurrenceMap[os.Uid] = occurrence
	// logger.Printf("occurrence uid=: %v\n", occurrenceLoop.Uid)
	occurrence.AddStatement(rdfs.Label, sst.String(os.Id.ID))
	occurrence.AddStatement(lci.IsDefinedBy, parent)
	if os.Quantity.ValueComponent != 0.0 {
		// for now only length in metre, other quantities and units TBD
		quantity := cp.graph.CreateIRINode("", qau.Length)

		occurrence.AddStatement(sso.OccurrenceQuantity, quantity)
		quantity.AddStatement(qau.Metre, sst.Double(os.Quantity.ValueComponent))
	}
	for _, shapeElementLoop := range os.ShapeElement {
		if err := cp.processShapeElementLoop1(shapeElementLoop, occurrence); err != nil {
			return err
		}
	}
	for _, occurrenceLoop := range os.Occurrence {
		if err := cp.processOccurrenceLoop1(occurrenceLoop, occurrence); err != nil {
			return err
		}
	}
	return nil
}

func (cp *conversionParameters) processOccurrenceLoop2(os OccurrenceStruct) error {
	occurrence := cp.occurrenceMap[os.Uid]
	if os.UpperUsage.Ref != "" {
		parentOccurrence, found := cp.occurrenceMap[os.UpperUsage.Ref]
		if !found {
			return ErrParentOccurrenceNotFound
		}
		occurrence.AddStatement(sso.HasHigherUsage, parentOccurrence)
	}
	for _, occurrenceLoop := range os.Occurrence {
		if err := cp.processOccurrenceLoop2(occurrenceLoop); err != nil {
			return err
		}
	}
	return nil
}

func setViewContext(view sst.IBNode, vc ViewContextStruct) error {
	if vc.ApplicationDomain != "" {
		sAD := "ApplicationDomain_" + vc.ApplicationDomain
		dtAD, err := sso.SSOVocabulary.ElementInformer(sAD)
		// logger.Printf("ViewContextRef: %v, %s, %v\n", vc, sAD, dtAD)
		if err != nil {
			return fmt.Errorf("%w for %v", err, vc.ApplicationDomain)
		}
		view.AddStatement(rdf.Type, dtAD)
	}

	if vc.LifeCycleStage != "" {
		sLCS := "LifeCycleStage_" + vc.LifeCycleStage
		dtLCS, err := sso.SSOVocabulary.ElementInformer(sLCS)
		// logger.Printf("ViewContextRef: %v, %s, %s %v\n", vc, sAD, sLCS, dtLCS)
		if err != nil {
			return fmt.Errorf("%w for %v", err, vc.LifeCycleStage)
		}
		view.AddStatement(rdf.Type, dtLCS)
	}
	return nil
}

// handles IdentifierSelect from AP242 BOModel
//
//	IdentifierSelect = (
//		IdentifierSet,
//		SingleIdentifierSelect
//	)
//	IdentifierSet = [2:*] Identifier;
//	SingleIdentifierSelect = (
//		Identifier,
//		IdentifierStringProxy
//	)
//	IdentifierStringProxy = IdentifierString;
//	IdentifierString = String;
func (cp *conversionParameters) processID(ib sst.IBNode, ids IDStruct, idKind int) error {
	// case SingleIdentifierSelect / IdentifierStringProxy / IdentifierString
	// logger.Printf("processID: %v\n", ids)
	if ids.ID != "" {
		var err error
		// logger.Printf("  processID1: %v, %v\n", ids, ids.ID)
		switch idKind {
		case cIdMain:
			ib.AddStatement(sso.ID, sst.String(ids.ID))
		case cIdVersion:
			ib.AddStatement(sso.VersionID, sst.String(ids.ID))
		case cIdView:
			ib.AddStatement(sso.ViewID, sst.String(ids.ID))
		}
		if err != nil {
			return fmt.Errorf("%w: with %v", err, ids)
		}
	} else if ids.Identifier != nil {
		for _, identifier := range ids.Identifier {
			// idOwnerMap
			logger.Printf("  processID2: %v, %v\n", identifier, identifier.ID)
			var idOwner, organization sst.IBNode
			if identifier.IdContextRef != "" {
				idOwner = cp.idOwnerMap[identifier.IdContextRef]
				organization = cp.organizationMap[identifier.IdContextRef]
			}
			if idOwner == nil {
				idOwner = cp.graph.CreateIRINode("", owl.ObjectProperty)

				switch idKind {
				case cIdMain:
					idOwner.AddStatement(rdfs.SubPropertyOf, sso.ID)
				case cIdVersion:
					idOwner.AddStatement(rdfs.SubPropertyOf, sso.VersionID)
				case cIdView:
					idOwner.AddStatement(rdfs.SubPropertyOf, sso.ViewID)
				}
				// logger.Printf("  processID3-2: %v\n", idOwner.Fragment())
				if organization != nil {
					// logger.Printf("  processID3-3: %v\n", organization.Fragment())
					idOwner.AddStatement(sso.IDOwner, organization)
					cp.idOwnerMap[identifier.IdContextRef] = idOwner
				}
			}
			ib.AddStatement(idOwner, sst.String(identifier.ID))
		}
	}
	return nil
}

func (cp *conversionParameters) addItemDefinedTransformation(shapeType string, ibRR sst.IBNode, ref1 *Origin, ref2 *Target) error {
	originRefs, targetRefs := collectNonEmptyUidRefs(ref1, ref2)
	// Check if both have the same number of non-empty UidRefs
	if len(originRefs) != len(targetRefs) {
		return ErrOriginTargetLengthMismatch
	}

	for i := 0; i < len(originRefs); i++ {
		ib1, found := cp.representationItemMap[originRefs[i]]
		if !found {
			return ErrRepresentationItemNotFound
		}
		ib2, found := cp.representationItemMap[targetRefs[i]]
		if !found {
			return ErrRepresentationItemNotFound
		}
		ibIDT := cp.graph.CreateIRINode("", owl.ObjectProperty)

		if shapeType == "geometryToTopologyModelAssociation" {
			ibIDT.AddStatement(rdfs.SubPropertyOf, rep.GeometryToTopologyItemAssociation)
		}
		if shapeType == "topologyToGeometryModelAssociation" {
			ibIDT.AddStatement(rdfs.SubPropertyOf, rep.TopologyToGeometryItemAssociation)
		}
		if shapeType == "shapeRepresentationRelationshipWithPlacementTransformation" {
			ibIDT.AddStatement(rdfs.SubPropertyOf, rep.ItemDefinedTransformation)
		}

		ib2.AddStatement(ibIDT, ib1)
		ibRR.AddStatement(rep.TransformationOperator, ibIDT)
	}
	return nil
}

// Collects all non-empty UidRefs from Origin and Target into slices for comparison.
func collectNonEmptyUidRefs(origin *Origin, target *Target) ([]string, []string) {
	var originRefs, targetRefs []string

	// Collect non-empty UidRefs from Origin
	if origin.UidRef != "" {
		originRefs = append(originRefs, origin.UidRef)
	}
	if origin.Vertex.Ref != "" {
		originRefs = append(originRefs, origin.Vertex.Ref)
	}
	if origin.Edge.Ref != "" {
		originRefs = append(originRefs, origin.Edge.Ref)
	}
	for _, ap := range origin.AxisPlacement {
		if ap.Ref != "" {
			originRefs = append(originRefs, ap.Ref)
		}
	}

	// Collect non-empty UidRefs from Target
	if target.UidRef != "" {
		targetRefs = append(targetRefs, target.UidRef)
	}
	if target.Path.Ref != "" {
		targetRefs = append(targetRefs, target.Path.Ref)
	}
	if target.Edge.Ref != "" {
		targetRefs = append(targetRefs, target.Edge.Ref)
	}
	for _, ap := range target.AxisPlacement {
		if ap.Ref != "" {
			targetRefs = append(targetRefs, ap.Ref)
		}
	}

	return originRefs, targetRefs
}

func parseFloats(s string) (sst.LiteralCollection, error) {
	var col sst.LiteralCollection
	if s != "" {
		ss := strings.Split(s, " ")
		if len(ss) == 1 {
			x, err := strconv.ParseFloat(ss[0], 64)
			if err != nil {
				return col, err
			}
			col = sst.NewLiteralCollection(sst.Double(x))
		} else if len(ss) == 2 {
			x, err := strconv.ParseFloat(ss[0], 64)
			if err != nil {
				return col, err
			}
			y, err := strconv.ParseFloat(ss[1], 64)
			if err != nil {
				return col, err
			}
			col = sst.NewLiteralCollection(sst.Double(x), sst.Double(y))
		} else if len(ss) == 3 {
			x, err := strconv.ParseFloat(ss[0], 64)
			if err != nil {
				return col, err
			}
			y, err := strconv.ParseFloat(ss[1], 64)
			if err != nil {
				return col, err
			}
			z, err := strconv.ParseFloat(ss[2], 64)
			if err != nil {
				return col, err
			}
			col = sst.NewLiteralCollection(sst.Double(x), sst.Double(y), sst.Double(z))
		}
		if col.MemberCount() == 0 {
			return col, fmt.Errorf("%w: <%v>, <%v> %v", ErrInvalidPosition, s, ss, len(ss))
		}
	}
	return col, nil
}

func AP242XmlImport(xmlFileName string) (sst.NamedGraph, error) {
	logger.Printf("AP242XmlImport file=%v\n", xmlFileName)
	xmlFile, err := os.Open(xmlFileName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = xmlFile.Close() }()
	return FromXMLReader(xmlFile)
}

func FromXMLReader(fromXML io.Reader) (sst.NamedGraph, error) {
	v := Result{}

	decoder := xml.NewDecoder(fromXML)
	if err := xmlDecodeWithXSIType(decoder, &v); err != nil {
		return nil, err
	}

	log.Printf("XMLName: %#v\n", v.XMLName)
	log.Printf("\n")
	log.Printf("Header: %v\n", v.Header)
	log.Printf("Unit: %v\n", v.DataContainer.Unit)
	log.Printf("Class: %v\n", v.DataContainer.Class)
	for _, p := range v.DataContainer.Part {
		log.Printf("Part: %v\n", p)
	}

	stage := sst.OpenStage(sst.DefaultTriplexMode)
	// graph := stage.DefaultGraph()

	var err error
	graph := stage.CreateNamedGraph("")
	cp := newConversionParameters(graph)

	// First Loop: Create IBNodes for definitions and create reference maps
	logger.Printf("#######  First Loop  #######\n")
	for _, p := range v.DataContainer.Part {
		part := graph.CreateIRINode("", sso.Part)
		cp.partMap[p.UID] = part
		if p.PartTypes != nil {
			for _, pt := range p.PartTypes {
				sPT := "PartCategory_" + pt
				dt, err := sso.SSOVocabulary.ElementInformer(sPT)
				if err != nil {
					return graph, fmt.Errorf("%w for %v", err, p.PartTypes)
				}
				part.AddStatement(rdf.Type, dt)
			}
		}
		part.AddStatement(rdfs.Label, sst.String(p.Name))
		for _, v := range p.Versions {
			ver := graph.CreateIRINode("", sso.PartVersion)
			cp.partVersionMap[v.UID] = ver
			part.AddStatement(sso.HasPartVersion, ver)

			for _, viewLoop := range v.Views {
				var dt sst.Term
				switch viewLoop.XsiType {
				case XSIType{Space: domainModel, Local: "AssemblyDefinition"}:
					dt = sso.AssemblyDesign
				case XSIType{Space: domainModel, Local: "CollectionDefinition"}:
					dt = sso.CollectionDefinition
				case XSIType{Space: domainModel, Local: "MatingDefinition"}:
					dt = sso.MatingDefinition
				case XSIType{Space: domainModel, Local: "WiringHarnessAssemblyDesign"}:
					dt = sso.WiringHarnessAssemblyDesign
				default:
					dt = sso.PartDesign
				}
				view := graph.CreateIRINode("", dt)
				cp.viewMap[viewLoop.Uid] = view
				// logger.Printf("view uid=: %v\n", viewLoop.Uid)
				view.AddStatement(rdfs.Label, sst.String(viewLoop.Id.ID))
				ver.AddStatement(sso.HasProductDefinition, view)
				for _, shapeElementLoop := range viewLoop.ShapeElement {
					if err := cp.processShapeElementLoop1(shapeElementLoop, view); err != nil {
						return graph, err
					}
				}
				for _, occurrenceLoop := range viewLoop.Occurrence {
					if err := cp.processOccurrenceLoop1(occurrenceLoop, view); err != nil {
						return graph, err
					}
				}
			}
		}
	}
	for _, p := range v.DataContainer.ProductConcept {
		var dt sst.Term
		switch p.XsiType {
		case XSIType{Space: domainModel, Local: "ProductClass"}:
			dt = sso.ProductClass
		default:
			dt = sso.ProductClass
		}
		pc := graph.CreateIRINode("", dt)
		cp.partMap[p.UID] = pc
		pc.AddStatement(rdfs.Label, sst.String(p.Name))
		for _, q := range p.ProductConfiguration {
			var dt sst.Term
			switch q.XsiType {
			default:
				dt = sso.ProductConfiguration
			}
			ps := graph.CreateIRINode("", dt)
			cp.viewMap[q.UID] = ps
			ps.AddStatement(rdfs.Label, sst.String(p.Name))
			for _, shapeElementLoop := range q.ShapeElement {
				if err := cp.processShapeElementLoop1(shapeElementLoop, ps); err != nil {
					return graph, err
				}
			}
			for _, occurrenceLoop := range q.Occurrence {
				if err := cp.processOccurrenceLoop1(occurrenceLoop, ps); err != nil {
					return graph, err
				}
			}
		}
	}
	for _, rc := range v.DataContainer.RepresentationContext {
		var dt sst.Term
		var dimensionCount *int
		switch rc.XsiType {
		case XSIType{Space: domainModel, Local: "GeometricCoordinateSpace"}:
			dt = rep.GeometricRepresentationContext
			dimensionCount = rc.DimensionCount
		default:
			dt = rep.RepresentationContext
		}
		rcIB := graph.CreateIRINode("", dt)
		if dimensionCount != nil {
			rcIB.AddStatement(rep.CoordinateSpaceDimension, sst.Integer(*dimensionCount))
		}
		cp.representationContextMap[rc.UID] = rcIB
		// logger.Printf("RepresentationContext: %v = %v\n", rc.UID, rcIB)
		// err = rc.AddStatement(rdfs.Label, literal.String(p.Name))
		if err := cp.processID(rcIB, rc.Id, cIdMain); err != nil {
			return graph, err
		}
		for _, representation := range rc.Representations {
			var dt sst.Term
			switch representation.XsiType {
			case XSIType{Space: domainModel, Local: "GeometricRepresentation"}:
				dt = rep.GeometricRepresentation
			case XSIType{Space: domainModel, Local: "StyledModel"}:
				dt = sso.StyledModel
			case XSIType{Space: domainModel, Local: "GeometricModel"}:
				dt = rep.GeometricModel
			case XSIType{Space: domainModel, Local: "ComposedGeometricModel"}:
				dt = sso.ComposedGeometricModel
			case XSIType{Space: domainModel, Local: "ExternalGeometricModel"}:
				dt = sso.ExternalGeometricModel
			case XSIType{Space: domainModel, Local: "ExternalCsgShapeRepresentation"}:
				dt = sso.ExternalCsgShapeRepresentation
			case XSIType{Space: domainModel, Local: "ExternalAdvancedBrepShapeRepresentation"}:
				dt = sso.ExternalAdvancedBrepShapeRepresentation
			case XSIType{Space: domainModel, Local: "ExternalEdgeBasedWireframeShapeRepresentation"}:
				dt = sso.ExternalEdgeBasedWireframeShapeRepresentation
			case XSIType{Space: domainModel, Local: "ExternalCurveSweptSolidShapeRepresentation"}:
				dt = sso.ExternalCurveSweptSolidShapeRepresentation
			case XSIType{Space: domainModel, Local: "ExternalGeometricallyBoundedSurfaceShapeRepresentation"}:
				dt = sso.ExternalGeometricallyBoundedSurfaceShapeRepresentation
			case XSIType{Space: domainModel, Local: "ExternalFacetedBrepShapeRepresentation"}:
				dt = sso.ExternalFacetedBrepShapeRepresentation
			case XSIType{Space: domainModel, Local: "ExternalGeometricallyBoundedWireframeShapeRepresentation"}:
				dt = sso.ExternalGeometricallyBoundedWireframeShapeRepresentation
			case XSIType{Space: domainModel, Local: "ExternalTessellatedShapeRepresentation"}:
				dt = sso.ExternalTessellatedShapeRepresentation
			case XSIType{Space: domainModel, Local: "ExternalShellBasedWireframeShapeRepresentation"}:
				dt = sso.ExternalShellBasedWireframeShapeRepresentation
			case XSIType{Space: domainModel, Local: "ExternalManifoldSurfaceShapeRepresentation"}:
				dt = sso.ExternalManifoldSurfaceShapeRepresentation
			case XSIType{Space: domainModel, Local: "ExternalElementaryBrepShapeRepresentation"}:
				dt = sso.ExternalElementaryBrepShapeRepresentation
			case XSIType{Space: domainModel, Local: "TransformedGeometricModel"}:
				dt = sso.TransformedGeometricModel
			case XSIType{Space: domainModel, Local: "ConstituentShapeRepresentation"}:
				dt = sso.ConstituentShapeRepresentation
			case XSIType{Space: domainModel, Local: "BeveledSheetRepresentation"}:
				dt = sso.BeveledSheetRepresentation
			case XSIType{Space: domainModel, Local: "CompositeSheetRepresentation"}:
				dt = sso.CompositeSheetRepresentation
			case XSIType{Space: domainModel, Local: "GeometricSheetRepresentation"}:
				dt = sso.GeometricSheetRepresentation
			case XSIType{Space: domainModel, Local: "FaceBasedSheetRepresentation"}:
				dt = sso.FaceBasedSheetRepresentation
			case XSIType{Space: domainModel, Local: "ThreeDGeometrySet"}:
				dt = sso.ThreeDGeometrySet
			case XSIType{Space: domainModel, Local: "LinkMotionAlongPath"}:
				dt = sso.LinkMotionAlongPath
			case XSIType{Space: domainModel, Local: "MechanismState"}:
				dt = sso.MechanismState
			case XSIType{Space: domainModel, Local: "InterpolatedConfigurationModel"}:
				dt = sso.InterpolatedConfigurationModel
			case XSIType{Space: domainModel, Local: "Mechanism"}:
				dt = sso.Mechanism
			case XSIType{Space: domainModel, Local: "KinematicLink"}:
				dt = sso.KinematicLink
			case XSIType{Space: domainModel, Local: "ReinforcementOrientationBasis"}:
				dt = sso.ReinforcementOrientationBasis
			case XSIType{Space: domainModel, Local: "PlyAngleRepresentation"}:
				dt = sso.PlyAngleRepresentation
			case XSIType{Space: domainModel, Local: "EdgeBasedTopologicalRepresentationWithLengthConstraint"}:
				dt = rep.EdgeBasedTopologicalRepresentationWithLengthConstraint
			default:
				dt = rep.Representation
			}
			repIB := graph.CreateIRINode("", dt)
			cp.representationMap[representation.UID] = repIB
			// logger.Printf("Representation: %v = %v\n", rep.UID, repIB)
			repIB.AddStatement(rep.ContextOfItems, rcIB)
			// err = repIB.AddStatement(rdfs.Label, literal.String(rep.Name))
			if err := cp.processID(repIB, representation.Id, cIdMain); err != nil {
				return graph, err
			}
		}
		for _, ri := range rc.Items {
			var dt sst.Term
			var positionCollection, axisCollection, refDirectionCollection sst.LiteralCollection
			_ = axisCollection
			_ = refDirectionCollection
			switch ri.XsiType {
			case XSIType{Space: domainModel, Local: "DetailedGeometricModelItem"}:
				dt = rep.GeometricRepresentationItem
			case XSIType{Space: domainModel, Local: "AdvancedFace"}:
				dt = rep.AdvancedFace
			case XSIType{Space: domainModel, Local: "EdgeCurve"}:
				dt = rep.EdgeCurve
			case XSIType{Space: domainModel, Local: "EdgeBoundedCurveWithLength"}:
				dt = rep.EdgeBoundedCurveWithLength
			case XSIType{Space: domainModel, Local: "Point"}:
				dt = rep.Point
			case XSIType{Space: domainModel, Local: "CartesianPoint"}:
				dt = rep.CartesianPoint
			case XSIType{Space: domainModel, Local: "PointOnSurface"}:
				dt = rep.PointOnSurface
			case XSIType{Space: domainModel, Local: "PointOnCurve"}:
				dt = rep.PointOnCurve
			case XSIType{Space: domainModel, Local: "DegeneratePcurve"}:
				dt = rep.DegeneratePcurve
			case XSIType{Space: domainModel, Local: "AxisPlacement"}:
				if ri.Position != "" {
					positionCollection, err = parseFloats(ri.Position)
					if err != nil {
						return graph, err
					}
				} else {
					fmt.Println("Warning - AxisPlacement without position, using 0.0")
					switch *rc.DimensionCount {
					case 2:
						positionCollection = sst.NewLiteralCollection(sst.Double(0.0), sst.Double(0.0))
					case 3:
						positionCollection = sst.NewLiteralCollection(sst.Double(0.0), sst.Double(0.0), sst.Double(0.0))
					default:
						return graph, fmt.Errorf("%w: <%v>", ErrInvalidPosition, ri.Position)
					}
				}
				if positionCollection.MemberCount() == 2 {
					dt = rep.Axis2Placement2D
				} else if positionCollection.MemberCount() == 3 {
					dt = rep.Axis2Placement3D
				} else {
					return graph, fmt.Errorf("%w: <%v>", ErrInvalidPosition, ri.Position)
				}
			case XSIType{Space: domainModel, Local: "ToolAttachmentPointFrame"}:
				dt = sso.ToolAttachmentPointFrame
			case XSIType{Space: domainModel, Local: "ToolCentrePointFrame"}:
				dt = sso.ToolCentrePointFrame
			case XSIType{Space: domainModel, Local: "Direction"}:
				dt = rep.Direction
			case XSIType{Space: domainModel, Local: "VertexPoint"}:
				dt = rep.VertexPoint
			case XSIType{Space: domainModel, Local: "Curve"}:
				dt = rep.Curve
			case XSIType{Space: domainModel, Local: "SurfaceCurve"}:
				dt = rep.SurfaceCurve
			case XSIType{Space: domainModel, Local: "Pcurve"}:
				dt = rep.Pcurve
			case XSIType{Space: domainModel, Local: "CompositeCurveOnSurface"}:
				dt = rep.CompositeCurveOnSurface
			case XSIType{Space: domainModel, Local: "BoundedCurve"}:
				dt = rep.BoundedCurve
			case XSIType{Space: domainModel, Local: "BoundedCurveWithLength"}:
				dt = rep.BoundedCurveWithLength
			case XSIType{Space: domainModel, Local: "TrimmedCurve"}:
				dt = rep.TrimmedCurve
			case XSIType{Space: domainModel, Local: "Surface"}:
				dt = rep.Surface
			case XSIType{Space: domainModel, Local: "OrientedSurface"}:
				dt = rep.OrientedSurface
			case XSIType{Space: domainModel, Local: "RectangularTrimmedSurface"}:
				dt = rep.RectangularTrimmedSurface
			case XSIType{Space: domainModel, Local: "Loop"}:
				dt = rep.Loop
			case XSIType{Space: domainModel, Local: "Cartesian11"}:
				dt = sso.Cartesian11
			case XSIType{Space: domainModel, Local: "Curve11"}:
				dt = sso.Curve11
			case XSIType{Space: domainModel, Local: "Cylindrical11"}:
				dt = sso.Cylindrical11
			case XSIType{Space: domainModel, Local: "PointArray"}:
				dt = sso.PointArray
			case XSIType{Space: domainModel, Local: "Polar11"}:
				dt = sso.Polar11
			case XSIType{Space: domainModel, Local: "PointAndVector"}:
				dt = sso.PointAndVector
			case XSIType{Space: domainModel, Local: "GeometricSet"}:
				dt = rep.GeometricSet
			case XSIType{Space: domainModel, Local: "GeometricCurveSet"}:
				dt = rep.GeometricCurveSet
			// case XSIType{Space: domainModel, Local: "BoundaryCurveSet"}:  // from composits ??
			//	dt = rep.BoundaryCurveSet
			case XSIType{Space: domainModel, Local: "KinematicPairValue"}:
				dt = sso.KinematicPairValue
			case XSIType{Space: domainModel, Local: "KinematicPair"}:
				dt = sso.KinematicPair
			case XSIType{Space: domainModel, Local: "LowOrderKinematicPair"}:
				dt = sso.LowOrderKinematicPair
			case XSIType{Space: domainModel, Local: "HighOrderKinematicPair"}:
				dt = sso.HighOrderKinematicPair
			case XSIType{Space: domainModel, Local: "LowOrderKinematicPairWithMotionCoupling"}:
				dt = sso.LowOrderKinematicPairWithMotionCoupling
			case XSIType{Space: domainModel, Local: "RotationAboutDirection"}:
				dt = sso.RotationAboutDirection
			case XSIType{Space: domainModel, Local: "KinematicPathDefinedByNodes"}:
				dt = sso.KinematicPathDefinedByNodes
			case XSIType{Space: domainModel, Local: "InterpolatedConfigurationSequence"}:
				dt = sso.InterpolatedConfigurationSequence
			case XSIType{Space: domainModel, Local: "ExternalRepresentationItem"}:
				dt = sso.ExternalRepresentationItem
			case XSIType{Space: domainModel, Local: "UserDefined11"}:
				dt = sso.UserDefined11
			case XSIType{Space: domainModel, Local: "TopologicalRepresentationItem"}:
				dt = rep.TopologicalRepresentationItem
			case XSIType{Space: domainModel, Local: "ConnectedEdgeSet"}:
				dt = rep.ConnectedEdgeSet
			case XSIType{Space: domainModel, Local: "ConnectedEdgeSubSet"}:
				dt = rep.ConnectedEdgeSubSet
			case XSIType{Space: domainModel, Local: "Path"}:
				dt = rep.Path
			case XSIType{Space: domainModel, Local: "Vertex"}:
				dt = rep.Vertex
			case XSIType{Space: domainModel, Local: "Edge"}:
				dt = rep.Edge
			case XSIType{Space: domainModel, Local: "SubEdge"}:
				dt = rep.SubEdge
			default:
				dt = rep.RepresentationItem
			}
			riIB := graph.CreateIRINode("", dt)
			cp.representationItemMap[ri.UID] = riIB
			if ri.Name != "" {
				riIB.AddStatement(rdfs.Label, sst.String(ri.Name))
			}
			switch ri.XsiType {
			case XSIType{Space: domainModel, Local: "AxisPlacement"}:
				c := graph.CreateBlankNode(rep.CartesianPoint)

				c.AddStatement(rep.Coordinates, positionCollection)
				riIB.AddStatement(rep.Location, c)
				logger.Printf("AxisPlacement: axis=<%v>, Position=<%v>, RefDirection=<%v>\n", ri.Axis, ri.Position, ri.RefDirection)
			}
			if err := cp.processID(riIB, ri.Id, cIdMain); err != nil {
				return graph, err
			}
			// logger.Printf("RepresentationItem: %v = %v\n", ri.UID, riIB)
		}
	}
	for _, p := range v.DataContainer.Organization {
		org := graph.CreateIRINode("", lci.Organization)

		cp.organizationMap[p.UID] = org
		org.AddStatement(rdfs.Label, sst.String(p.Name))
		// logger.Printf("organization uid=: %v, %v, %v\n", p.UID, p.Name, org.Fragment())
		if err := cp.processID(org, p.Id, cIdMain); err != nil {
			return graph, err
		}
	}

	for _, p := range v.DataContainer.PropertyDefinition {
		var dt sst.Term
		switch p.XsiType {
		case XSIType{Space: domainModel, Local: "WireColourBasedIdentificationCode"}:
			dt = sso.WireColourBasedIdentificationCode
		default:
			dt = lci.PropertyDefinition
		}
		propDef := graph.CreateIRINode("", dt)
		cp.propertyDefinitionMap[p.UID] = propDef
		propDef.AddStatement(sso.ID, sst.String(p.Id.ID))
		propDef.AddStatement(rdfs.Label, sst.String(p.PropertyType))
	}
	for _, p := range v.DataContainer.ShapeFeatureDefinition {
		var dt sst.Term
		switch p.XsiType {
		case XSIType{Space: domainModel, Local: "ContactFeatureDefinition"}:
			dt = sso.ContactFeatureDefinition
		default:
			dt = sso.ShapeFeatureDefinition
		}
		sfd := graph.CreateIRINode("", dt)
		cp.shapeElementMap[p.UID] = sfd
		sfd.AddStatement(rdfs.Label, sst.String(p.Name))
		if p.ShapeFeatureType != "" {
			sSF, err := cp.getEnumeration(sso.ShapeFeatureType.Element, p.ShapeFeatureType)
			if err != nil {
				return graph, fmt.Errorf("%w: <%v>", ErrInvalidShapeFeatureType, p.ShapeFeatureType)
			}

			sfd.AddStatement(rdf.Type, sSF)
		}
		for _, shapeElementLoop := range p.ShapeElement {
			if err := cp.processShapeElementLoop1(shapeElementLoop, sfd); err != nil {
				return graph, err
			}
		}
	}
	for _, p := range v.DataContainer.ViewContext {
		cp.viewContextMap[p.UID] = p
		// logger.Printf("ViewContext: %v, %v, %v\n", p.UID, p.ApplicationDomain, p.LifeCycleStage)
	}

	// Second Loop: Create missing statements for references using reference maps
	logger.Printf("#######  Second Loop  #######\n")
	for _, p := range v.DataContainer.Part {
		part := cp.partMap[p.UID]
		if err := cp.processID(part, p.Id, cIdMain); err != nil {
			return graph, err
		}
		for _, v := range p.Versions {
			ver := cp.partVersionMap[v.UID]
			if err := cp.processID(ver, v.Id, cIdVersion); err != nil {
				return graph, err
			}
			for _, viewLoop := range v.Views {
				view, found := cp.viewMap[viewLoop.Uid]
				if !found {
					return graph, err
				}
				if err := cp.processID(view, v.Id, cIdView); err != nil {
					return graph, err
				}

				vc := cp.viewContextMap[viewLoop.InitialContext.Ref]
				if err := setViewContext(view, vc); err != nil {
					return graph, err
				}

				if viewLoop.DefiningGeometry.Ref != "" {
					representationRef, found := cp.representationMap[viewLoop.DefiningGeometry.Ref]
					if !found {
						return graph, ErrRepresentationNotFound
					}
					view.AddStatement(sso.DefiningGeometry, representationRef)
				}
				for _, auxLoop := range viewLoop.AuxiliaryGeometry {
					representationRef, found := cp.representationMap[auxLoop.Ref]
					if !found {
						return graph, ErrRepresentationNotFound
					}
					view.AddStatement(sso.AuxiliaryGeometry, representationRef)
				}
				if viewLoop.Topology.Ref != "" {
					representationRef, found := cp.representationMap[viewLoop.Topology.Ref]
					if !found {
						return graph, ErrRepresentationNotFound
					}
					view.AddStatement(sso.Topology, representationRef)
				}

				for _, shapeElementLoop := range viewLoop.ShapeElement {
					if err := cp.processShapeElementLoop2(shapeElementLoop); err != nil {
						return graph, err
					}
				}
				for _, occurrenceLoop := range viewLoop.Occurrence {
					for _, shapeElementLoop := range occurrenceLoop.ShapeElement {
						if err := cp.processShapeElementLoop2(shapeElementLoop); err != nil {
							return graph, err
						}
					}
					if err = cp.processOccurrenceLoop2(occurrenceLoop); err != nil {
						return graph, err
					}
				}
				for _, viewOccurrenceRelationshipLoop := range viewLoop.ViewOccurrenceRelationship {
					var dt sst.Term
					switch viewOccurrenceRelationshipLoop.XsiType {
					// case XSIType{Space: domainModel, Local: "AssemblyOccurrenceRelationship"}:
					// 	dt = sso.AssemblyOccurrenceRelationship // is Abstract
					case XSIType{Space: domainModel, Local: "PromissoryAssemblyOccurrenceUsage"}:
						dt = sso.PromissoryAssemblyOccurrenceUsage
					case XSIType{Space: domainModel, Local: "NextAssemblyOccurrenceUsage"}:
						dt = sso.NextAssemblyOccurrenceUsage
					case XSIType{Space: domainModel, Local: "CollectedPartRelationship"}:
						dt = sso.CollectedPartRelationship
						// default:
						//	dt = sso.ViewOccurrenceRelationship
					}
					occurrenceRef, found := cp.occurrenceMap[viewOccurrenceRelationshipLoop.Related.Ref]
					// logger.Printf("occurrence uidRef=: %v\n", viewOccurrenceRelationshipLoop.Related.Ref)
					if !found {
						return graph, fmt.Errorf("%w: uidRef=%v", ErrOccurrenceNotFound, viewOccurrenceRelationshipLoop.Related.Ref)
					}

					subProp := graph.CreateIRINode("", owl.ObjectProperty)

					cp.viewOccurrenceRelationshipMap[viewOccurrenceRelationshipLoop.Uid] = subProp
					subProp.AddStatement(rdfs.SubPropertyOf, dt)
					view.AddStatement(subProp, occurrenceRef)
				}
			}
		}
	}
	for _, rc := range v.DataContainer.RepresentationContext {
		_, found := cp.representationContextMap[rc.UID]
		if !found {
			return graph, fmt.Errorf("%w: uid=%v", ErrRepresentationContextNotFound, rc.UID)
		}
		// err = rc.AddStatement(rdfs.Label, literal.String(p.Name))
		// cp.processID(rcIB, rc.Id, false)
		for _, representation := range rc.Representations {
			repIB, found := cp.representationMap[representation.UID]
			if !found {
				return graph, fmt.Errorf("%w: uid=%v", ErrRepresentationNotFound, representation.UID)
			}
			for _, ri := range representation.Items {
				riIB, found := cp.representationItemMap[ri.UidRef]
				if !found {
					return graph, fmt.Errorf("%w: uid=%v", ErrRepresentationItemNotFound, ri.UidRef)
				}
				repIB.AddStatement(rep.Item, riIB)
			}
			for _, representationRelationshipLoop := range representation.RepresentationRelationship {
				var dt sst.Term
				switch representationRelationshipLoop.XsiType {
				case XSIType{Space: domainModel, Local: "GeometryToTopologyModelAssociation"}:
					dt = rep.GeometryToTopologyModelAssociation
				case XSIType{Space: domainModel, Local: "TopologyToGeometryModelAssociation"}:
					dt = rep.TopologyToGeometryModelAssociation
				case XSIType{Space: domainModel, Local: "GeometricRepresentationRelationship"}:
					dt = rep.ShapeRepresentationRelationship
				case XSIType{Space: domainModel, Local: "GeometricRepresentationRelationshipWithPlacementTransformation"}:
					dt = rep.ShapeRepresentationRelationshipWithPlacementTransformation
				case XSIType{Space: domainModel, Local: "GeometricRepresentationRelationshipWithCartesianTransformation"}:
					dt = rep.ShapeRepresentationRelationshipWithCartesianTransformation
				case XSIType{Space: domainModel, Local: "GeneralGeometricRepresentationRelationship"}:
					dt = rep.ShapeRepresentationRelationshipWithTransformation
				case XSIType{Space: domainModel, Local: "GeometricRepresentationRelationshipWithSameCoordinateSpace"}:
					dt = rep.ShapeRepresentationRelationshipWithSameGeometricRepresentationContext
				// case XSIType{Space: domainModel, Local: "PlyOrientationAngle"}:
				//	dt = sso.
				// case XSIType{Space: domainModel, Local: "LaidOrientationAngle"}:
				//	dt = sso.
				// case XSIType{Space: domainModel, Local: "DrapedOrientationAngle"}:
				//	dt = sso.
				default:
					dt = rep.RepresentationRelationship
				}
				relatedRef, found := cp.representationMap[representationRelationshipLoop.Related.Ref]
				// logger.Printf("occurrence uidRef=: %v\n", viewOccurrenceRelationshipLoop.Related.Ref)
				if !found {
					return graph, fmt.Errorf("%w: uidRef=%v", ErrRepresentationNotFound, representationRelationshipLoop.Related.Ref)
				}

				subProp := graph.CreateIRINode("", owl.ObjectProperty)

				cp.representationRelationshipMap[representationRelationshipLoop.Uid] = subProp
				_, shapeType := dt.(sst.Node).IRI().Split()
				subProp.AddStatement(rdfs.SubPropertyOf, dt)
				repIB.AddStatement(subProp, relatedRef)
				if representationRelationshipLoop.Origin != nil && representationRelationshipLoop.Target != nil {
					if err = cp.addItemDefinedTransformation(
						shapeType,
						subProp,
						representationRelationshipLoop.Origin,
						representationRelationshipLoop.Target,
					); err != nil {
						return graph, err
					}
				}
			}
		}
		for _, ri := range rc.Items {
			riIB, found := cp.representationItemMap[ri.UID]
			if !found {
				return graph, fmt.Errorf("%w: uid=%v", ErrRepresentationItemNotFound, rc.UID)
			}
			// logger.Printf("RepresentationItem-Loop2 uidRef=: <%v> <%v>\n", ri.UID, riIB.Fragment())
			switch ri.XsiType {
			case XSIType{Space: domainModel, Local: "Path"}:
				// rep.ConnectedEdgeSet
				// logger.Printf("ConnectedEdgeSet-Loop2 uidRef=: <%v>\n", ri )
				sib := []sst.Term{}
				for i, data := range ri.EdgeList.Edge {
					ibEdge, found := cp.representationItemMap[data.Ref]
					if !found {
						return graph, fmt.Errorf("%w: for Path <%v>, <%v>", ErrRepresentationItemNotFound, ri.UID, data.Ref)
					}
					ibOrientedEdge, err := cp.createOrientedEdge(ibEdge, ri.OrientationList.Boolean[i])
					if err != nil {
						return graph, err
					}
					sib = append(sib, ibOrientedEdge)
				}
				co := graph.CreateCollection(sib...)
				riIB.AddStatement(rep.EdgeList, co)
				// TBD: create OrientedEdge
				//for _, data := range ri.OrientationList.Boolean {
				//p := data.Boolean
				//TBD: err = riIB.AddStatement(rep.EdgeList, literal.Double(p))
				//}
			}
			switch ri.XsiType {
			case XSIType{Space: domainModel, Local: "ConnectedEdgeSet"}, XSIType{Space: domainModel, Local: "ConnectedEdgeSubSet"}:
				// rep.ConnectedEdgeSet
				// logger.Printf("ConnectedEdgeSet-Loop2 uidRef=: <%v>\n", ri )
				for _, data := range ri.ConnectedEdges.Edge {
					ib, found := cp.representationItemMap[data.Ref]
					if !found {
						return graph, fmt.Errorf("%w: for ConnectedEdgeSet <%v>, <%v>", ErrRepresentationItemNotFound, ri.UID, data.Ref)
					}
					riIB.AddStatement(rep.CesEdges, ib)
				}
			}
			switch ri.XsiType {
			case XSIType{Space: domainModel, Local: "ConnectedEdgeSubSet"}:
				// rep.ConnectedEdgeSet
				ib, found := cp.representationItemMap[ri.ParentEdgeSet.UidRef]
				if !found {
					return graph, fmt.Errorf("%w: for ConnectedEdgeSubSet <%v>, <%v>", ErrRepresentationItemNotFound, ri.UID, ri.ParentEdgeSet.UidRef)
				}
				riIB.AddStatement(rep.ConnectedEdgeSubSet, ib)
			}
			switch ri.XsiType {
			case XSIType{Space: domainModel, Local: "BoundedCurveWithLength"}:
				p := ri.CurveLength
				riIB.AddStatement(rep.LengthValue, sst.Double(p))
				// logger.Printf("BoundedCurveWithLength-Loop2 = : <%v> <%v> <%v>\n", err, ri, p)
			}
			switch ri.XsiType {
			case XSIType{Space: domainModel, Local: "Edge"},
				XSIType{Space: domainModel, Local: "SubEdge"},
				XSIType{Space: domainModel, Local: "EdgeCurve"},
				XSIType{Space: domainModel, Local: "EdgeBoundedCurveWithLength"}:

				ibStart, found := cp.representationItemMap[ri.EdgeStart.Ref]
				if !found {
					return graph, fmt.Errorf("%w: for Edge <%v>, <%v>", ErrRepresentationItemNotFound, ri.UID, ri.EdgeStart.Ref)
				}
				ibEnd, found := cp.representationItemMap[ri.EdgeEnd.Ref]
				if !found {
					return graph, fmt.Errorf("%w: for Edge <%v>, <%v>", ErrRepresentationItemNotFound, ri.UID, ri.EdgeEnd.Ref)
				}
				riIB.AddStatement(rep.EdgeStart, ibStart)
				riIB.AddStatement(rep.EdgeEnd, ibEnd)
			}
			switch ri.XsiType {
			case XSIType{Space: domainModel, Local: "SubEdge"}:
				ib, found := cp.representationItemMap[ri.ParentEdge.Ref]
				if !found {
					return graph, fmt.Errorf("%w: for SubEdge <%v>, <%v>", ErrRepresentationItemNotFound, ri.UID, ri.ParentEdge.Ref)
				}
				riIB.AddStatement(rep.ParentEdge, ib)
			}
			switch ri.XsiType {
			case XSIType{Space: domainModel, Local: "EdgeCurve"},
				XSIType{Space: domainModel, Local: "EdgeBoundedCurveWithLength"}:
				ib, found := cp.representationItemMap[ri.EdgeGeometry.Ref]
				if !found {
					return graph, fmt.Errorf("%w: for EdgeBoundedCurveWithLength <%v>, <%v>", ErrRepresentationItemNotFound, ri.UID, ri.EdgeGeometry.Ref)
				}
				riIB.AddStatement(rep.EdgeGeometry, ib)
				p := ri.SameSense
				riIB.AddStatement(rep.SameSense, sst.Boolean(p))
			}

			switch ri.XsiType {
			case XSIType{Space: domainModel, Local: "PointOnCurve"}:
				ib, found := cp.representationItemMap[ri.BasicCurve.Ref]
				if !found {
					return graph, fmt.Errorf("%w: for PointOnCurve <%v>, <%v>", ErrRepresentationItemNotFound, ri.UID, ri.BasicCurve.Ref)
				}
				riIB.AddStatement(rep.BasisCurve, ib)
				p := ri.Parameter
				riIB.AddStatement(rep.PointParameter, sst.Double(p))
			}

			switch ri.XsiType {
			case XSIType{Space: domainModel, Local: "VertexPoint"}:
				ib, found := cp.representationItemMap[ri.VertexGeometry.Ref]
				if !found {
					return graph, fmt.Errorf("%w: for VertexPoint <%v>, <%v>", ErrRepresentationItemNotFound, ri.UID, ri.VertexGeometry.Ref)
				}
				riIB.AddStatement(rep.VertexGeometry, ib)
			}

			switch ri.XsiType {
			case XSIType{Space: domainModel, Local: "DetailedGeometricModelItem"}:
				// rep.GeometricRepresentationItem
			case XSIType{Space: domainModel, Local: "AdvancedFace"}:
				//  rep.AdvancedFace
			case XSIType{Space: domainModel, Local: "Point"}:
				// rep.Point
			case XSIType{Space: domainModel, Local: "CartesianPoint"}:
				// rep.CartesianPoint
			case XSIType{Space: domainModel, Local: "PointOnSurface"}:
				// rep.PointOnSurface
			case XSIType{Space: domainModel, Local: "PointOnCurve"}:
				// rep.PointOnCurve
			case XSIType{Space: domainModel, Local: "DegeneratePcurve"}:
				// rep.DegeneratePcurve
			case XSIType{Space: domainModel, Local: "AxisPlacement"}:
				// rep.Placement
			case XSIType{Space: domainModel, Local: "ToolAttachmentPointFrame"}:
				// sso.ToolAttachmentPointFrame
			case XSIType{Space: domainModel, Local: "ToolCentrePointFrame"}:
				// sso.ToolCentrePointFrame
			case XSIType{Space: domainModel, Local: "Direction"}:
				// rep.Direction
			case XSIType{Space: domainModel, Local: "VertexPoint"}:
				// rep.VertexPoint
			case XSIType{Space: domainModel, Local: "Curve"}:
				// rep.Curve
			case XSIType{Space: domainModel, Local: "SurfaceCurve"}:
				// rep.SurfaceCurve
			case XSIType{Space: domainModel, Local: "Pcurve"}:
				// rep.Pcurve
			case XSIType{Space: domainModel, Local: "CompositeCurveOnSurface"}:
				// rep.CompositeCurveOnSurface
			case XSIType{Space: domainModel, Local: "BoundedCurve"}:
				// rep.BoundedCurve
			case XSIType{Space: domainModel, Local: "BoundedCurveWithLength"}:
				// rep.BoundedCurveWithLength
			case XSIType{Space: domainModel, Local: "TrimmedCurve"}:
				// rep.TrimmedCurve
			case XSIType{Space: domainModel, Local: "Surface"}:
				// rep.Surface
			case XSIType{Space: domainModel, Local: "OrientedSurface"}:
				// rep.OrientedSurface
			case XSIType{Space: domainModel, Local: "RectangularTrimmedSurface"}:
				// rep.RectangularTrimmedSurface
			case XSIType{Space: domainModel, Local: "Loop"}:
				// rep.Loop
			case XSIType{Space: domainModel, Local: "Cartesian11"}:
				// sso.Cartesian11
			case XSIType{Space: domainModel, Local: "Curve11"}:
				// sso.Curve11
			case XSIType{Space: domainModel, Local: "Cylindrical11"}:
				// sso.Cylindrical11
			case XSIType{Space: domainModel, Local: "PointArray"}:
				// sso.PointArray
			case XSIType{Space: domainModel, Local: "Polar11"}:
				// sso.Polar11
			case XSIType{Space: domainModel, Local: "PointAndVector"}:
				// sso.PointAndVector
			case XSIType{Space: domainModel, Local: "GeometricSet"}:
				// rep.GeometricSet
			case XSIType{Space: domainModel, Local: "GeometricCurveSet"}:
				// rep.GeometricCurveSet
			// case XSIType{Space: domainModel, Local: "BoundaryCurveSet"}:  // from composits ??
			//	// rep.BoundaryCurveSet
			case XSIType{Space: domainModel, Local: "KinematicPairValue"}:
				// sso.KinematicPairValue
			case XSIType{Space: domainModel, Local: "KinematicPair"}:
				// sso.KinematicPair
			case XSIType{Space: domainModel, Local: "LowOrderKinematicPair"}:
				// sso.LowOrderKinematicPair
			case XSIType{Space: domainModel, Local: "HighOrderKinematicPair"}:
				// sso.HighOrderKinematicPair
			case XSIType{Space: domainModel, Local: "LowOrderKinematicPairWithMotionCoupling"}:
				// sso.LowOrderKinematicPairWithMotionCoupling
			case XSIType{Space: domainModel, Local: "RotationAboutDirection"}:
				// sso.RotationAboutDirection
			case XSIType{Space: domainModel, Local: "KinematicPathDefinedByNodes"}:
				// sso.KinematicPathDefinedByNodes
			case XSIType{Space: domainModel, Local: "InterpolatedConfigurationSequence"}:
				// sso.InterpolatedConfigurationSequence
			case XSIType{Space: domainModel, Local: "ExternalRepresentationItem"}:
				// sso.ExternalRepresentationItem
			case XSIType{Space: domainModel, Local: "UserDefined11"}:
				// sso.UserDefined11
			case XSIType{Space: domainModel, Local: "TopologicalRepresentationItem"}:
				// rep.TopologicalRepresentationItem
			case XSIType{Space: domainModel, Local: "ConnectedEdgeSet"}:
				// rep.ConnectedEdgeSet
			case XSIType{Space: domainModel, Local: "ConnectedEdgeSubSet"}:
				// rep.ConnectedEdgeSubSet
			case XSIType{Space: domainModel, Local: "Path"}:
				// rep.Path
				// create collection
				// col, err := graph.CreateCollection(n2, n3)
			default:
				// rep.RepresentationItem
			}
		}
	}
	for _, p := range v.DataContainer.ShapeFeatureDefinition {
		for _, shapeElementLoop := range p.ShapeElement {
			if err := cp.processShapeElementLoop2(shapeElementLoop); err != nil {
				return graph, err
			}
		}
	}

	// Third Loop: Create relationships of relationships
	logger.Printf("#######  Third Loop  #######\n")
	for _, p := range v.DataContainer.Part {
		// part := cp.partMap[p.UID]
		for _, v := range p.Versions {
			// ver := cp.partVersionMap[v.UID]
			for _, viewLoop := range v.Views {
				_, found := cp.viewMap[viewLoop.Uid]
				if !found {
					return graph, fmt.Errorf("%w: for <%v>", ErrViewNotFound, viewLoop.Uid)
				}

				for _, viewOccurrenceRelationshipLoop := range viewLoop.ViewOccurrenceRelationship {

					subProp, found := cp.viewOccurrenceRelationshipMap[viewOccurrenceRelationshipLoop.Uid]
					if !found {
						return graph, fmt.Errorf("%w: for <%v>", ErrViewOccurrenceRelationshipNotFound, viewOccurrenceRelationshipLoop.Uid)
					}
					for _, refLoop := range viewOccurrenceRelationshipLoop.Placement.GeometryToTopologyModelAssociation {
						representationRelationshipRef, found := cp.representationRelationshipMap[refLoop.Ref]
						if !found {
							return graph, fmt.Errorf("%w: for <%v>", ErrRepresentationRelationshipNotFound, refLoop.Ref)
						}
						subProp.AddStatement(sso.ContextDependentShapeRepresentation, representationRelationshipRef)
					}
					for _, refLoop := range viewOccurrenceRelationshipLoop.Placement.GeometricRepresentationRelationship {
						representationRelationshipRef, found := cp.representationRelationshipMap[refLoop.Ref]
						if !found {
							return graph, fmt.Errorf("%w: for <%v>", ErrRepresentationRelationshipNotFound, refLoop.Ref)
						}
						subProp.AddStatement(sso.ContextDependentShapeRepresentation, representationRelationshipRef)
					}
				}
			}
		}
	}
	return graph, nil
}

// get enumeration search in dictionary for particular enumeration value belongs to particular enumeration type
func (cp *conversionParameters) getEnumeration(enumerationType sst.Element, enumerationValue string) (sst.ElementInformer, error) {
	if result, found := cp.enumerationCache[enumerationValue]; found {
		return result, nil // Return cached result
	}

	var result sst.ElementInformer
	selectedEnumerationNode, err := sst.StaticDictionary().Element(enumerationType)
	if err != nil {
		panic(err)
	}

	selectedEnumerationNode.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		s.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
			if p.Is(ssmeta.StepDmEnumerationMap) {
				keyValue := strings.ToLower(string(o.(sst.String)))
				if keyValue != "" && enumerationValue == keyValue {
					result, err = sso.SSOVocabulary.ElementInformer(string(s.Fragment()))
					if err != nil {
						return err
					}
				}
			}
			return nil
		})
		return nil
	})
	cp.enumerationCache[enumerationValue] = result
	return result, nil
}

func Logger() *log.Logger {
	return logger
}
