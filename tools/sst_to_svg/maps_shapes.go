// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package ssttosvg

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/semanticstep/sst-core/vocabularies/sso"
)

func getCircleMap(o sst.Term, styleMap map[uuid.UUID]map[string]string) map[string]string {
	resultMap := make(map[string]string)
	resultMap["kind"] = "Circle"

	if styles, found := styleMap[o.(sst.IBNode).ID()]; found {
		for key, value := range styles {
			resultMap[key] = value
		}
	}

	o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		var radius, x, y float64
		if p.Is(rep.Radius) {
			radius = float64(o.(sst.Double))
			resultMap["r"] = fmt.Sprintf("%f", radius)
		}
		if p.Is(rep.Position) {
			o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
				if p.Is(rep.Location) {
					o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
						if p.Is(rep.Coordinates) {
							if o.TermKind() == sst.TermKindLiteralCollection {
								o := o.(sst.LiteralCollection)
								x = float64(o.Member(0).(sst.Double))
								y = -float64(o.Member(1).(sst.Double))
								// resultMap["x"] = fmt.Sprintf("%f", x)
								// resultMap["y"] = fmt.Sprintf("%f", y)
								resultMap["translate"] = fmt.Sprintf("%f,%f", x, y)
							}
						}
						return nil
					})
				}
				if p.Is(rep.RefDirectionDegree) {
					angle := -float64(o.(sst.Double))
					resultMap["rotation"] = fmt.Sprintf("%f", angle)
				}
				return nil
			})
		}
		return nil
	})
	return resultMap
}

func getRectangleMap(o sst.Term, styleMap map[uuid.UUID]map[string]string) map[string]string {
	resultMap := make(map[string]string)
	resultMap["kind"] = "Rectangle"

	if styles, found := styleMap[o.(sst.IBNode).ID()]; found {
		for key, value := range styles {
			resultMap[key] = value
		}
	}

	o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		var x, y, width, height, rx, ry float64
		if p.Is(rep.XLength) {
			width = float64(o.(sst.Double))
			resultMap["width"] = fmt.Sprintf("%f", width)
		}
		if p.Is(rep.YLength) {
			height = float64(o.(sst.Double))
			resultMap["height"] = fmt.Sprintf("%f", height)
		}
		if p.Is(rep.Position) {
			o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
				if p.Is(rep.Location) {
					o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
						if p.Is(rep.Coordinates) {
							if o.TermKind() == sst.TermKindLiteralCollection {
								o := o.(sst.LiteralCollection)
								x = float64(o.Member(0).(sst.Double))
								y = -float64(o.Member(1).(sst.Double))
								// resultMap["x"] = fmt.Sprintf("%f", x)
								// resultMap["y"] = fmt.Sprintf("%f", y)
								resultMap["translate"] = fmt.Sprintf("%f,%f", x, y)
							}
						}
						return nil
					})
				}
				if p.Is(rep.RefDirectionDegree) {
					angle := -float64(o.(sst.Double))
					resultMap["rotation"] = fmt.Sprintf("%f", angle)
				}
				return nil
			})
		}
		if p.Is(rep.RadiusX) {
			rx = float64(o.(sst.Double))
			resultMap["rx"] = fmt.Sprintf("%f", rx)
		}
		if p.Is(rep.RadiusY) {
			ry = float64(o.(sst.Double))
			resultMap["ry"] = fmt.Sprintf("%f", ry)
		}
		return nil
	})
	return resultMap
}

func getEllipseMap(o sst.Term, styleMap map[uuid.UUID]map[string]string) map[string]string {
	resultMap := make(map[string]string)
	resultMap["kind"] = "Ellipse"

	if styles, found := styleMap[o.(sst.IBNode).ID()]; found {
		for key, value := range styles {
			resultMap[key] = value
		}
	}

	o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		var rx, ry, x, y float64
		if p.Is(rep.SemiAxis1) {
			rx = float64(o.(sst.Double))
			resultMap["rx"] = fmt.Sprintf("%f", rx)
		}
		if p.Is(rep.SemiAxis2) {
			ry = float64(o.(sst.Double))
			resultMap["ry"] = fmt.Sprintf("%f", ry)
		}
		if p.Is(rep.Position) {
			o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
				if p.Is(rep.Location) {
					o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
						if p.Is(rep.Coordinates) {
							if o.TermKind() == sst.TermKindLiteralCollection {
								o := o.(sst.LiteralCollection)
								x = float64(o.Member(0).(sst.Double))
								y = -float64(o.Member(1).(sst.Double))
								resultMap["translate"] = fmt.Sprintf("%f,%f", x, y)
							}
						}
						return nil
					})
				}
				if p.Is(rep.RefDirectionDegree) {
					angle := -float64(o.(sst.Double))
					resultMap["rotation"] = fmt.Sprintf("%f", angle)
				}
				return nil
			})
		}
		return nil
	})
	return resultMap
}

func getPolylineMap(o sst.Term, styleMap map[uuid.UUID]map[string]string) map[string]string {
	resultMap := make(map[string]string)
	resultMap["kind"] = "Polyline"
	pointString := ""
	pathDString := ""

	if styles, found := styleMap[o.(sst.IBNode).ID()]; found {
		for key, value := range styles {
			resultMap[key] = value
		}
	}

	firstPoint := true

	o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if p.Is(rep.Points) {
			collection, ok := o.(sst.IBNode).AsCollection()
			if ok {
				collection.ForMembers(func(index int, o sst.Term) {
					o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
						if p.Is(rep.Coordinates) {
							if o.TermKind() == sst.TermKindLiteralCollection {
								o := o.(sst.LiteralCollection)
								x := float64(o.Member(0).(sst.Double))
								y := -float64(o.Member(1).(sst.Double))

								pointString += fmt.Sprintf("%f,%f ", x, y)

								if firstPoint {
									pathDString += fmt.Sprintf("M %f,%f ", x, y)
									firstPoint = false
								} else {
									pathDString += fmt.Sprintf("L %f,%f ", x, y)
								}
							}
						}
						return nil
					})
				})
			}

			resultMap["position"] = strings.TrimSpace(pointString)
			resultMap["path_d"] = strings.TrimSpace(pathDString)
		}
		return nil
	})
	return resultMap
}

func getPolygonMap(o sst.Term, styleMap map[uuid.UUID]map[string]string) map[string]string {
	resultMap := make(map[string]string)
	resultMap["kind"] = "Polygon"
	pointString := ""

	if styles, found := styleMap[o.(sst.IBNode).ID()]; found {
		for key, value := range styles {
			resultMap[key] = value
		}
	}

	o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if p.Is(rep.Points) {
			collection, ok := o.(sst.IBNode).AsCollection()
			if ok {
				collection.ForMembers(func(index int, o sst.Term) {
					o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
						if p.Is(rep.Coordinates) {
							if o.TermKind() == sst.TermKindLiteralCollection {
								o := o.(sst.LiteralCollection)
								x := float64(o.Member(0).(sst.Double))
								y := -float64(o.Member(1).(sst.Double))
								pointString += fmt.Sprintf("%f,%f ", x, y)
							}
						}
						return nil
					})
				})
			}
			resultMap["position"] = pointString
		}
		return nil
	})
	return resultMap
}

func getTrimmedCurveMap(o sst.Term, styleMap map[uuid.UUID]map[string]string) map[string]string {
	resultMap := make(map[string]string)
	resultMap["kind"] = "TrimmedCurve"

	if styles, found := styleMap[o.(sst.IBNode).ID()]; found {
		for key, value := range styles {
			resultMap[key] = value
		}
	}

	var trim1, trim2 []float64
	var center []float64
	var radius, radius_x, radius_y float64
	var angle float64
	var isClockWise bool
	o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if p.Is(rep.BasisCurve) {
			// if trim a circle
			if o.(sst.IBNode).TypeOf().Is(rep.Circle) {
				o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
					if p.Is(rep.Radius) {
						radius = float64(o.(sst.Double))
						radius_x = radius
						radius_y = radius
					}
					if p.Is(rep.Position) {
						o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
							if p.Is(rep.Location) {
								o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
									if p.Is(rep.Coordinates) {
										if o.TermKind() == sst.TermKindLiteralCollection {
											o := o.(sst.LiteralCollection)
											x := float64(o.Member(0).(sst.Double))
											y := -float64(o.Member(1).(sst.Double))
											center = append(center, x, y)
										}
									}
									return nil
								})
							}
							if p.Is(rep.RefDirectionDegree) {
								angle = -float64(o.(sst.Double))
								resultMap["rotation"] = fmt.Sprintf("%f", angle)
							}
							return nil
						})
					}
					return nil
				})
			} else if o.(sst.IBNode).TypeOf().Is(rep.Ellipse) {
				o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
					if p.Is(rep.SemiAxis1) {
						radius_x = float64(o.(sst.Double))
					}
					if p.Is(rep.SemiAxis2) {
						radius_y = float64(o.(sst.Double))
					}
					if p.Is(rep.Position) {
						o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
							if p.Is(rep.Location) {
								o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
									if p.Is(rep.Coordinates) {
										if o.TermKind() == sst.TermKindLiteralCollection {
											o := o.(sst.LiteralCollection)
											x := float64(o.Member(0).(sst.Double))
											y := -float64(o.Member(1).(sst.Double))
											center = append(center, x, y)
										}
									}
									return nil
								})
							}
							if p.Is(rep.RefDirectionDegree) {
								angle = -float64(o.(sst.Double))
								resultMap["rotation"] = fmt.Sprintf("%f", angle)
							}
							return nil
						})
					}
					return nil
				})
			}
		}
		if p.Is(rep.Trim1) {
			o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
				if p.Is(rep.Coordinates) {
					if o.TermKind() == sst.TermKindLiteralCollection {
						o := o.(sst.LiteralCollection)
						x := float64(o.Member(0).(sst.Double))
						y := -float64(o.Member(1).(sst.Double))
						trim1 = append(trim1, x, y)
					}
				}
				return nil
			})
		}
		if p.Is(rep.Trim2) {
			o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
				if p.Is(rep.Coordinates) {
					if o.TermKind() == sst.TermKindLiteralCollection {
						o := o.(sst.LiteralCollection)
						x := float64(o.Member(0).(sst.Double))
						y := -float64(o.Member(1).(sst.Double))
						trim2 = append(trim2, x, y)
					}
				}
				return nil
			})
		}

		if p.Is(rep.SenseAgreement) {
			isClockWise = !bool(o.(sst.Boolean))
		}
		return nil
	})
	d_path := workOutTrimmedCurveForPath(trim1, trim2, center, angle, radius_x, radius_y, isClockWise)
	resultMap["path_d"] = d_path
	return resultMap
}

func getBezierCurveMap(o sst.Term, styleMap map[uuid.UUID]map[string]string) map[string]string {
	resultMap := make(map[string]string)
	resultMap["kind"] = "BezierCurve"

	if styles, found := styleMap[o.(sst.IBNode).ID()]; found {
		for key, value := range styles {
			resultMap[key] = value
		}
	}

	var controlPoints [][]float64
	var degree int

	// Traverse nodes to extract control points and degree
	o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		// Check for the degree property
		if p.Is(rep.Degree) {
			degree = int(o.(sst.Integer))
		}

		// Check for the control points list
		if p.Is(rep.ControlPointsList) {
			// Extract control points from the literal collection
			collection, ok := o.(sst.IBNode).AsCollection()

			if ok {
				collection.ForMembers(func(index int, o sst.Term) {
					o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
						// Extract coordinates from the collection
						if p.Is(rep.Coordinates) {
							if o.TermKind() == sst.TermKindLiteralCollection {
								o := o.(sst.LiteralCollection)
								x := float64(o.Member(0).(sst.Double))
								y := -float64(o.Member(1).(sst.Double))
								controlPoints = append(controlPoints, []float64{x, y})
							}
						}
						return nil
					})
				})
			}
		}
		return nil
	})

	// Generate the Bézier curve path based on the degree
	var d_path string
	switch degree {
	case 2:
		// Quadratic Bézier curve (Q)
		if len(controlPoints) != 3 {
			return nil
		}
		d_path = workOutQuadraticBezierCurveForPath(controlPoints[0], controlPoints[2], controlPoints[1])
	case 3:
		// Cubic Bézier curve (C)
		if len(controlPoints) != 4 {
			return nil
		}
		d_path = workOutCubicBezierCurveForPath(controlPoints[0], controlPoints[3], controlPoints[1], controlPoints[2])
	default:
		return nil
	}

	resultMap["path_d"] = d_path
	return resultMap
}

func getTextMap(o sst.Term, styleMap map[uuid.UUID]map[string]string) map[string]string {
	resultMap := make(map[string]string)
	resultMap["kind"] = "TextLiteral"

	if styles, found := styleMap[o.(sst.IBNode).ID()]; found {
		for key, value := range styles {
			resultMap[key] = value
		}
	}

	o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if p.Is(rep.Literal) {
			text := string(o.(sst.String))
			resultMap["text"] = text
		}
		if p.Is(rep.Position) {
			o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
				if p.Is(rep.Location) {
					o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
						if p.Is(rep.Coordinates) {
							if o.TermKind() == sst.TermKindLiteralCollection {
								o := o.(sst.LiteralCollection)
								x := float64(o.Member(0).(sst.Double))
								y := -float64(o.Member(1).(sst.Double))
								// resultMap["x"] = fmt.Sprintf("%f", x)
								// resultMap["y"] = fmt.Sprintf("%f", y)
								resultMap["translate"] = fmt.Sprintf("%f,%f", x, y)
							}
						}
						return nil
					})
				}
				if p.Is(rep.RefDirectionDegree) {
					angle := -float64(o.(sst.Double))
					resultMap["rotation"] = fmt.Sprintf("%f", angle)
				}
				return nil
			})
		}
		if p.Is(rep.Font) {
			o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
				if p.Is(sso.ID) {
					fontFamily := string(o.(sst.String))
					resultMap["font-family"] = fontFamily
				}
				if p.Is(rep.FontSize) {
					if fontSize, ok := o.(sst.Double); ok {
						resultMap["font-size"] = fmt.Sprintf("%f", float64(fontSize))
					} else {
						fmt.Println("Expected DoubleLiteral for font-size, but got different type")
					}
				}
				return nil
			})
		}
		if p.Is(rep.FontModifier) {
			o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
				if p.Is(rep.TextModifer_bold) {
					resultMap["bold"] = "true"
				}
				if p.Is(rep.TextModifer_underscore) {
					resultMap["underscore"] = "true"
				}
				return nil
			})
		}
		return nil
	})
	return resultMap
}
