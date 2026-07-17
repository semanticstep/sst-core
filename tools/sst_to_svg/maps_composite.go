// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package ssttosvg

import (
	"fmt"
	"log"
	"strings"

	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/google/uuid"
)

func getStyledItemMap(o sst.Term, styleMap map[uuid.UUID]map[string]string) map[string]string {
	resultMap := make(map[string]string)
	resultMap["kind"] = "styledItem"

	if styles, found := styleMap[o.(sst.IBNode).ID()]; found {
		for key, value := range styles {
			resultMap[key] = value
		}
	}

	var shapeMap map[string]string

	o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if p.Is(rep.ItemToStyle) {
			shapeNode := o.(sst.IBNode)
			if shapeNode.TypeOf().Is(rep.Circle) {
				shapeMap = getCircleMap(o, styleMap)
			} else if shapeNode.TypeOf().Is(rep.Rectangle) || shapeNode.TypeOf().Is(rep.RoundedRectangle) {
				shapeMap = getRectangleMap(o, styleMap)
			} else if shapeNode.TypeOf().Is(rep.Ellipse) {
				shapeMap = getEllipseMap(o, styleMap)
			} else if shapeNode.TypeOf().Is(rep.Polyline) {
				shapeMap = getPolylineMap(o, styleMap)
			} else if shapeNode.TypeOf().Is(rep.Polygon) {
				shapeMap = getPolygonMap(o, styleMap)
			} else if shapeNode.TypeOf().Is(rep.TrimmedCurve) {
				shapeMap = getTrimmedCurveMap(o, styleMap)
			} else if shapeNode.TypeOf().Is(rep.BezierCurve) {
				shapeMap = getBezierCurveMap(o, styleMap)
			} else if shapeNode.TypeOf().Is(rep.TextLiteral) {
				shapeMap = getTextMap(o, styleMap)
			} else if shapeNode.TypeOf().Is(rep.CompositeCurve) {
				shapeMap = getCompositeCurveMap(o, styleMap)
			}
		}
		return nil
	})

	for key, value := range shapeMap {
		resultMap[key] = value
	}

	return resultMap
}

func getCompositeCurveMap(o sst.Term, styleMap map[uuid.UUID]map[string]string) map[string]string {
	resultMap := make(map[string]string)
	resultMap["kind"] = "CompositeCurve"

	var segments []map[string]string

	if styles, found := styleMap[o.(sst.IBNode).ID()]; found {
		for key, value := range styles {
			resultMap[key] = value
		}
	}

	// Traverse the nodes to extract segments
	err := o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if p.Is(rep.Segments) {
			// Process each CompositeCurveSegment in the collection
			if collection, ok := o.(sst.IBNode).AsCollection(); ok {
				collection.ForMembers(func(index int, segmentNode sst.Term) {
					segmentMap := processCompositeCurveSegment(segmentNode, styleMap)
					if segmentMap != nil {
						segments = append(segments, segmentMap)
					}
				})
			} else {
				log.Println("Warning: Expected a collection for segments, but could not cast.")
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("Error during node traversal: %v\n", err)
	}

	// Combine all segment paths into a single path data
	var combinedPath string
	// firstSegment := true
	isContinuous := "false"
	for _, segment := range segments {
		if path, exists := segment["path_d"]; exists {
			// log.Printf("path: %v, isContinuous: %v", path, segment["isContinuous"])
			if isContinuous == "true" {
				// Remove the "M" if the segment is continuous
				trimmedPath := strings.TrimSpace(path[findNextCommandIndex(path):])
				combinedPath += trimmedPath + " "
			} else {
				// Include the full path with "M" if discontinuous
				combinedPath += path + " "
			}
			isContinuous = segment["isContinuous"]
		}
	}

	// Check if the last segment's Transition is Continuous
	if len(segments) > 0 {
		lastSegment := segments[len(segments)-1]
		if isContinuous, exists := lastSegment["isContinuous"]; exists && isContinuous == "true" {
			combinedPath = strings.TrimSpace(combinedPath) + " z"
		}
	}

	// Trim any trailing space
	resultMap["path_d"] = strings.TrimSpace(combinedPath)
	return resultMap
}

func findNextCommandIndex(path string) int {
	for i := 1; i < len(path); i++ {
		if isSVGCommand(path[i]) {
			return i
		}
	}
	return len(path)
}

func isSVGCommand(ch byte) bool {
	switch ch {
	case 'M', 'L', 'C', 'Q', 'T', 'S', 'A', 'H', 'V', 'Z':
		return true
	default:
		return false
	}
}

func processCompositeCurveSegment(segmentNode sst.Term, styleMap map[uuid.UUID]map[string]string) map[string]string {
	var parentCurveNode sst.Term
	segmentResult := make(map[string]string)

	segmentNode.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {

		if p.Is(rep.ParentCurve) {
			parentCurveNode = o
		}

		if p.Is(rep.Transition) {
			isContinuous := o.(sst.IBNode).Is(rep.TransitionCode_Continuous)
			segmentResult["isContinuous"] = fmt.Sprintf("%t", isContinuous)
		}
		// if p.Is(rep.SameSense) {
		// 	segmentResult["sameSense"] = fmt.Sprintf("%t", o.(sst.BooleanLiteral).BooleanValue())
		// 	log.Printf("SameSense property found: %v\n", o.(sst.BooleanLiteral).BooleanValue())
		// }
		return nil
	})

	// Determine the type of the parent curve and process it
	if parentCurveNode != nil {
		switch {
		case parentCurveNode.(sst.IBNode).TypeOf().Is(rep.BezierCurve):
			bezierMap := getBezierCurveMap(parentCurveNode, styleMap)
			if bezierMap != nil {
				segmentResult["path_d"] = bezierMap["path_d"]
			} else {
				log.Println("Failed to process BezierCurve.")
			}

		case parentCurveNode.(sst.IBNode).TypeOf().Is(rep.TrimmedCurve):
			trimmedMap := getTrimmedCurveMap(parentCurveNode, styleMap)
			if trimmedMap != nil {
				segmentResult["path_d"] = trimmedMap["path_d"]
			} else {
				log.Println("Failed to process TrimmedCurve.")
			}

		case parentCurveNode.(sst.IBNode).TypeOf().Is(rep.Circle):
			circleMap := getCircleMap(parentCurveNode, styleMap)
			if circleMap != nil {
				segmentResult["path_d"] = circleMap["path_d"]
			} else {
				log.Println("Failed to process Circle.")
			}

		case parentCurveNode.(sst.IBNode).TypeOf().Is(rep.Rectangle):
			rectangleMap := getRectangleMap(parentCurveNode, styleMap)
			if rectangleMap != nil {
				segmentResult["path_d"] = rectangleMap["path_d"]
			} else {
				log.Println("Failed to process Rectangle.")
			}

		case parentCurveNode.(sst.IBNode).TypeOf().Is(rep.Ellipse):
			ellipseMap := getEllipseMap(parentCurveNode, styleMap)
			if ellipseMap != nil {
				segmentResult["path_d"] = ellipseMap["path_d"]
			} else {
				log.Println("Failed to process Ellipse.")
			}

		case parentCurveNode.(sst.IBNode).TypeOf().Is(rep.Polyline):
			polylineMap := getPolylineMap(parentCurveNode, styleMap)
			if polylineMap != nil {
				segmentResult["path_d"] = polylineMap["path_d"]
			} else {
				log.Println("Failed to process Polyline.")
			}

		case parentCurveNode.(sst.IBNode).TypeOf().Is(rep.Polygon):
			polygonMap := getPolygonMap(parentCurveNode, styleMap)
			if polygonMap != nil {
				segmentResult["path_d"] = polygonMap["path_d"]
			} else {
				log.Println("Failed to process Polygon.")
			}

		case parentCurveNode.(sst.IBNode).TypeOf().Is(rep.TextLiteral):
			textMap := getTextMap(parentCurveNode, styleMap)
			if textMap != nil {
				segmentResult["text"] = textMap["text"]
			} else {
				log.Println("Failed to process Text.")
			}

		default:
			log.Println("Unknown curve type encountered.")
		}
	} else {
		log.Println("No parentCurve found in this CompositeCurveSegment.")
	}

	return segmentResult
}

