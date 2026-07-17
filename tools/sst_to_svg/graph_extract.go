// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package ssttosvg

import (
	"fmt"
	"strings"

	"github.com/semanticstep/sst-core/sst"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/google/uuid"
)

func getCurveAndIdListForSvg(graph sst.NamedGraph) map[string][]map[string]string {
	resultList := map[string][]map[string]string{}

	styleMap := generateStyleMap(graph)

	graph.ForIRINodes(func(d sst.IBNode) error {
		if d.TypeOf() != nil {
			if d.TypeOf().Is(rep.SchematicSymbolRepresentation) || d.TypeOf().Is(rep.SymbolRepresentation) || d.TypeOf().Is(rep.SchematicPortRepresentation) {
				d.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
					if p.Is(rep.Item) {
						if o.(sst.IBNode).TypeOf().Is(rep.Circle) {
							resultMap := getCircleMap(o, styleMap)
							resultList["Circle"] = append(resultList["Circle"], resultMap)
						} else if o.(sst.IBNode).TypeOf().Is(rep.Rectangle) || o.(sst.IBNode).TypeOf().Is(rep.RoundedRectangle) {
							resultMap := getRectangleMap(o, styleMap)
							resultList["Rectangle"] = append(resultList["Rectangle"], resultMap)
						} else if o.(sst.IBNode).TypeOf().Is(rep.Ellipse) {
							resultMap := getEllipseMap(o, styleMap)
							resultList["Ellipse"] = append(resultList["Ellipse"], resultMap)
						} else if o.(sst.IBNode).TypeOf().Is(rep.Polyline) {
							resultMap := getPolylineMap(o, styleMap)
							resultList["Polyline"] = append(resultList["Polyline"], resultMap)
						} else if o.(sst.IBNode).TypeOf().Is(rep.Polygon) {
							resultMap := getPolygonMap(o, styleMap)
							resultList["Polygon"] = append(resultList["Polygon"], resultMap)
						} else if o.(sst.IBNode).TypeOf().Is(rep.TrimmedCurve) {
							resultMap := getTrimmedCurveMap(o, styleMap)
							resultList["TrimmedCurve"] = append(resultList["TrimmedCurve"], resultMap)
						} else if o.(sst.IBNode).TypeOf().Is(rep.BezierCurve) {
							resultMap := getBezierCurveMap(o, styleMap)
							resultList["BezierCurve"] = append(resultList["BezierCurve"], resultMap)
						} else if o.(sst.IBNode).TypeOf().Is(rep.TextLiteral) {
							resultMap := getTextMap(o, styleMap)
							resultList["TextLiteral"] = append(resultList["TextLiteral"], resultMap)
						} else if o.(sst.IBNode).TypeOf().Is(rep.StyledItem) {
							resultMap := getStyledItemMap(o, styleMap)
							if kind, found := resultMap["kind"]; found {

								resultList[kind] = append(resultList[kind], resultMap)
							} else {
								fmt.Println("Error: No 'kind' found in resultMap, skipping...")
							}
						} else if o.(sst.IBNode).TypeOf().Is(rep.CompositeCurve) {
							resultMap := getCompositeCurveMap(o, styleMap)
							resultList["CompositeCurve"] = append(resultList["CompositeCurve"], resultMap)
						}
					}
					return nil
				})
			}
		}
		return nil
	})
	return resultList
}

func generateStyleMap(graph sst.NamedGraph) map[uuid.UUID]map[string]string {
	styleMap := make(map[uuid.UUID]map[string]string)

	graph.ForIRINodes(func(d sst.IBNode) error {
		if d.TypeOf() != nil {
			if d.TypeOf().Is(rep.StyledItem) {
				var key uuid.UUID
				attributeMap := make(map[string]string)

				d.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
					if p.Is(rep.ItemToStyle) {
						key = o.(sst.IBNode).ID()
					}

					if p.Is(rep.Style) {
						o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
							if p.Is(rep.FillColour) {
								colour := strings.TrimPrefix(string(o.(sst.String)), "color:")
								attributeMap["fillColour"] = colour
							} else if p.Is(rep.CurveColour) {
								colour := strings.TrimPrefix(string(o.(sst.String)), "color:")
								attributeMap["curveColour"] = colour
							} else if p.Is(rep.CurveWidth) {
								attributeMap["curveWidth"] = fmt.Sprintf("%f", float64(o.(sst.Double)))
							}
							return nil
						})
					}
					return nil
				})

				if key != uuid.Nil && len(attributeMap) > 0 {
					styleMap[key] = attributeMap
				}
			}
		}
		return nil
	})

	return styleMap
}

