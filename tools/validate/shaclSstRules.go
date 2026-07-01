// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package validate

import (
	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/vocabularies/owl"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/sh"
)

// the extracted Node and Property Shapes.
type shapesStruct struct {
	nodeShapes       int
	propertyShapes   int
	targetNodes      map[sst.IBNode]sst.IBNode // key = target Node, value = node shape; TBD extend for several
	targetClasses    map[sst.IBNode]sst.IBNode // key = target Class, value = node shape; TBD extend for several
	targetSubjectsOf map[sst.IBNode]sst.IBNode // key = target path/property, value = property shape; TBD extend for several
	targetObjectsOf  map[sst.IBNode]sst.IBNode // key = target path/property, value = property shape; TBD extend for several
}

func ValidateStage(st sst.Stage) (ngResult sst.NamedGraph) {
	ngResult = sst.OpenStage(sst.DefaultTriplexMode).CreateNamedGraph("")
	ibNG := ngResult.GetIRINodeByFragment("")
	ibNG.AddStatement(rdf.Type, sh.ValidationReport)
	ibNG.AddStatement(sh.Conforms, sst.Boolean(false))
	shapes := ExtractShapes(sst.StaticDictionary())
	if shapes == nil {
		println("no Shapes found")
	} else {
		PrintShapes(shapes)
	}
	return ngResult
}

func PrintShapes(shapes *shapesStruct) {
	println("no of node shapes         =", shapes.nodeShapes)
	println("no of property shapes     =", shapes.propertyShapes)
	println("no of target nodes        =", len(shapes.targetNodes))
	println("no of target classes     =", len(shapes.targetClasses))
	println("no of target subjects of =", len(shapes.targetSubjectsOf))
	println("no of target objects of  =", len(shapes.targetObjectsOf))
	for _, targetClass := range shapes.targetClasses {
		print("Target class:", targetClass.IRI())
		nodeShape := shapes.targetClasses[targetClass]
		println("; node shape=", nodeShape.IRI())
	}
	for _, targetProperty := range shapes.targetSubjectsOf {
		print("Target property=")
		if targetProperty.IsBlankNode() {
			print(" _", targetProperty.ID().String())
		} else {
			print(targetProperty.IRI())
		}
		print(" property shape=")
		shape := shapes.targetSubjectsOf[targetProperty]
		if shape == nil {
			println("NIL")
		} else if shape.IsBlankNode() {
			println("_", shape.ID().String())
		} else {
			println(shape.IRI())
		}
	}
}

// Extract the Node and Property Shapes and return them as new allocated shapesStruct.
func ExtractShapes(stShapes sst.Stage) (shapes *shapesStruct) {
	shapes = new(shapesStruct)
	shapes.targetClasses = make(map[sst.IBNode]sst.IBNode)
	shapes.targetClasses = make(map[sst.IBNode]sst.IBNode)
	shapes.targetSubjectsOf = make(map[sst.IBNode]sst.IBNode)
	shapes.targetObjectsOf = make(map[sst.IBNode]sst.IBNode)

	// LoopTargets
	ngShapes := stShapes.NamedGraphs()
	for _, ng := range ngShapes {
		println(ng.IRI())
		ng.ForAllIBNodes(func(ib sst.IBNode) error {
			/* 			if ib.Fragment() == "CartesianPoint" {
				println("\t:: FOUND CARTESIANPOINT")
				println("\t\t TypeOf()=%s", ib.IRI())
				println("\t\t Is(rep.CartesianPoint)=", ib.Is(rep.CartesianPoint))
				println("\t\t Is(rep.Point)=", ib.Is(rep.Point))
				println("\t\t Is(rep.RepresentationItem)=", ib.Is(rep.RepresentationItem))
				println("\t\t IsKind(rep.CartesianPoint)=", ib.Is(rep.CartesianPoint))
				println("\t\t IsKind(rep.Point)=", ib.Is(rep.Point))
				println("\t\t IsKind(rep.RepresentationItem)=", ib.Is(rep.RepresentationItem))
				println("\t\t IsKind(owl.Class)=", ib.Is(owl.Class))
				println("\t\t IsKind(sh.NodeShape)=", ib.Is(sh.NodeShape))
				ibType := ib.TypeOf()
				if ibType == nil {
					println("\t:: TypeOf()=nil")
				} else if ibType.IsKind(sh.NodeShape) {
					println("\t:: TypeOf()==", ibType.Fragment())
				} else {
					println("\t:: TypeOf()===", ibType.IRI())
				}
			} */

			var nodeShape, targetClass, propertyShape, targetProperty sst.IBNode
			ib.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
				if s != ib { // skip inverses
					return nil
				} else if p.Is(rdf.Type) {
					if o.(sst.IBNode).Is(sh.NodeShape) {
						println("\t:: found Shape", s.IRI())
						nodeShape = s
						shapes.nodeShapes++
					} else if o.(sst.IBNode).Is(owl.Class) {
						targetClass = s
					}
				} else if p.Is(sh.TargetClass) {
					targetClass = o.(sst.IBNode)
				} else if p.Is(sh.Property) {
					shapes.propertyShapes++
					propertyShape = s
					targetProperty = o.(sst.IBNode)
				} else {
					return nil
				}
				return nil
			})
			if nodeShape != nil {
				if targetClass != nil {
					println("add TargetClass", targetClass.IRI(), " NodeShape", nodeShape.IRI())
					shapes.targetClasses[targetClass] = nodeShape
				}
			}
			if (propertyShape != nil) && (targetProperty != nil) {
				//				println("add TargetProperty", targetClass.IRI(), " PropertyShape", nodeShape.IRI())
				//				shapes.targetProperties[targetProperty] = propertyShape
			}
			return nil
		})
	}
	return shapes
}
