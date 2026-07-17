// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package svgtosst

import (
	"fmt"
	"log"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/google/uuid"
)
func createNodeForPath(graph sst.NamedGraph, path path) (sst.IBNode, error) {
	commands, err := parsePathData(string(path.D))
	if err != nil {
		log.Panic(err)
	}

	isSingle, shapeType := isSingleShape(commands)
	var node sst.IBNode

	if isSingle {
		fmt.Printf("Single shape detected: %s\n", shapeType)
		node, err = handleSingleShape(graph, commands, shapeType)
	} else {
		fmt.Println("Composite curve detected")
		node, err = handleCompositeCurve(graph, commands)
	}

	if err != nil {
		log.Panic(err)
	}

	return node, nil
}

func handleSingleShape(graph sst.NamedGraph, commands []pathCommand, shapeType string) (sst.IBNode, error) {
	currentPoint := point{0, 0}
	var points []point
	var node sst.IBNode
	var err error

	for _, command := range commands {
		switch command.Command {
		case "M":
			currentPoint.X = command.Params[0]
			currentPoint.Y = command.Params[1]
			points = append(points, currentPoint)
		case "m":
			currentPoint.X += command.Params[0]
			currentPoint.Y += command.Params[1]
			points = append(points, currentPoint)
		case "L":
			currentPoint.X = command.Params[0]
			currentPoint.Y = command.Params[1]
			points = append(points, currentPoint)
		case "l":
			currentPoint.X += command.Params[0]
			currentPoint.Y += command.Params[1]
			points = append(points, currentPoint)
		case "H":
			currentPoint.X = command.Params[0]
			points = append(points, currentPoint)
		case "h":
			currentPoint.X += command.Params[0]
			points = append(points, currentPoint)
		case "V":
			currentPoint.Y = command.Params[0]
			points = append(points, currentPoint)
		case "v":
			currentPoint.Y += command.Params[0]
			points = append(points, currentPoint)
		case "Z", "z":
			if len(points) > 2 {
				polygon := polygon{Points: pointsToString(points)}
				node, err = createNodeForPolygon(graph, polygon)
				if err != nil {
					return nil, err
				}
			}
		case "A":
			arc := ellipticalArc{
				RX:            command.Params[0],
				RY:            command.Params[1],
				StartX:        currentPoint.X,
				StartY:        currentPoint.Y,
				EndX:          command.Params[5],
				EndY:          command.Params[6],
				XAxisRotation: command.Params[2],
				LargeArcFlag:  command.Params[3] != 0,
				SweepFlag:     command.Params[4] != 0,
			}
			node, err = createNodesForTrimmedCurve(graph, arc)
			if err != nil {
				return nil, err
			}
			currentPoint.X = command.Params[5]
			currentPoint.Y = command.Params[6]
		case "a":
			arc := ellipticalArc{
				RX:            command.Params[0],
				RY:            command.Params[1],
				StartX:        currentPoint.X,
				StartY:        currentPoint.Y,
				EndX:          currentPoint.X + command.Params[5],
				EndY:          currentPoint.Y + command.Params[6],
				XAxisRotation: command.Params[2],
				LargeArcFlag:  command.Params[3] != 0,
				SweepFlag:     command.Params[4] != 0,
			}
			node, err = createNodesForTrimmedCurve(graph, arc)
			if err != nil {
				return nil, err
			}
			currentPoint.X += command.Params[5]
			currentPoint.Y += command.Params[6]
		case "C":
			bezier := cubicBezier{
				StartX:    currentPoint.X,
				StartY:    currentPoint.Y,
				ControlX1: command.Params[0],
				ControlY1: command.Params[1],
				ControlX2: command.Params[2],
				ControlY2: command.Params[3],
				EndX:      command.Params[4],
				EndY:      command.Params[5],
			}
			node, err = createNodeForCubicBezier(graph, bezier)
			if err != nil {
				return nil, err
			}
			currentPoint.X = command.Params[4]
			currentPoint.Y = command.Params[5]
		case "c":
			bezier := cubicBezier{
				StartX:    currentPoint.X,
				StartY:    currentPoint.Y,
				ControlX1: currentPoint.X + command.Params[0],
				ControlY1: currentPoint.Y + command.Params[1],
				ControlX2: currentPoint.X + command.Params[2],
				ControlY2: currentPoint.Y + command.Params[3],
				EndX:      currentPoint.X + command.Params[4],
				EndY:      currentPoint.Y + command.Params[5],
			}
			node, err = createNodeForCubicBezier(graph, bezier)
			if err != nil {
				return nil, err
			}
			currentPoint.X += command.Params[4]
			currentPoint.Y += command.Params[5]
		case "Q":
			bezier := quadraticBezier{
				StartX:   currentPoint.X,
				StartY:   currentPoint.Y,
				ControlX: command.Params[0],
				ControlY: command.Params[1],
				EndX:     command.Params[2],
				EndY:     command.Params[3],
			}
			node, err = createNodeForQuadraticBezier(graph, bezier)
			if err != nil {
				return nil, err
			}
			currentPoint.X = command.Params[2]
			currentPoint.Y = command.Params[3]
		case "q":
			bezier := quadraticBezier{
				StartX:   currentPoint.X,
				StartY:   currentPoint.Y,
				ControlX: currentPoint.X + command.Params[0],
				ControlY: currentPoint.Y + command.Params[1],
				EndX:     currentPoint.X + command.Params[2],
				EndY:     currentPoint.Y + command.Params[3],
			}
			node, err = createNodeForQuadraticBezier(graph, bezier)
			if err != nil {
				return nil, err
			}
			currentPoint.X += command.Params[2]
			currentPoint.Y += command.Params[3]
		}
	}

	if shapeType == "polyline" && len(points) > 1 {
		polyline := polyline{Points: pointsToString(points)}
		node, err = createNodeForPolyline(graph, polyline)
		if err != nil {
			return nil, err
		}
	}

	return node, nil
}

func handleCompositeCurve(graph sst.NamedGraph, commands []pathCommand) (sst.IBNode, error) {
	compositeCurveNode := graph.CreateIRINode(uuid.New().String())

	compositeCurveNode.AddStatement(rdf.Type, rep.CompositeCurve)

	var segmentNodes []sst.Term
	initialPoint := point{0, 0}
	currentPoint := point{0, 0}
	lastControlPoint := point{0, 0}
	var points []point
	var previousCommand string
	var lastSegmentNode sst.Term

	for i, command := range commands {
		fmt.Printf("i: %v Command: %s, Points: %v\n", i, command.Command, command.Params)

		switch command.Command {
		case "M", "m":
			if len(points) > 1 {
				segmentNode, err := createNodesForPolyOrLine(graph, points)
				if err != nil {
					log.Panic(err)
				}
				lastSegmentNode = segmentNode
			}
			if lastSegmentNode != nil {
				compositeCurveSegmentNode := createCompositeCurveSegment(graph, lastSegmentNode, false)
				segmentNodes = append(segmentNodes, compositeCurveSegmentNode)
				lastSegmentNode = nil
			}
			points = []point{}
			if command.Command == "M" {
				currentPoint.X = command.Params[0]
				currentPoint.Y = command.Params[1]
			} else {
				currentPoint.X += command.Params[0]
				currentPoint.Y += command.Params[1]
			}
			initialPoint = currentPoint
			points = append(points, currentPoint)
		case "L", "l", "H", "h", "V", "v":
			switch command.Command {
			case "L":
				currentPoint.X = command.Params[0]
				currentPoint.Y = command.Params[1]
			case "l":
				currentPoint.X += command.Params[0]
				currentPoint.Y += command.Params[1]
			case "H":
				currentPoint.X = command.Params[0]
			case "h":
				currentPoint.X += command.Params[0]
			case "V":
				currentPoint.Y = command.Params[0]
			case "v":
				currentPoint.Y += command.Params[0]
			}
			points = append(points, currentPoint)
		case "A", "a":
			if len(points) > 1 {
				segmentNode, err := createNodesForPolyOrLine(graph, points)
				if err != nil {
					log.Panic(err)
				}
				lastSegmentNode = segmentNode
			}
			if lastSegmentNode != nil {
				compositeCurveSegmentNode := createCompositeCurveSegment(graph, lastSegmentNode, true)
				segmentNodes = append(segmentNodes, compositeCurveSegmentNode)
				lastSegmentNode = nil
			}
			var arc ellipticalArc
			if command.Command == "A" {
				arc = ellipticalArc{
					RX:            command.Params[0],
					RY:            command.Params[1],
					StartX:        currentPoint.X,
					StartY:        currentPoint.Y,
					EndX:          command.Params[5],
					EndY:          command.Params[6],
					XAxisRotation: command.Params[2],
					LargeArcFlag:  command.Params[3] != 0,
					SweepFlag:     command.Params[4] != 0,
				}
				currentPoint.X = command.Params[5]
				currentPoint.Y = command.Params[6]
			} else {
				arc = ellipticalArc{
					RX:            command.Params[0],
					RY:            command.Params[1],
					StartX:        currentPoint.X,
					StartY:        currentPoint.Y,
					EndX:          currentPoint.X + command.Params[5],
					EndY:          currentPoint.Y + command.Params[6],
					XAxisRotation: command.Params[2],
					LargeArcFlag:  command.Params[3] != 0,
					SweepFlag:     command.Params[4] != 0,
				}
				currentPoint.X += command.Params[5]
				currentPoint.Y += command.Params[6]
			}

			points = []point{}
			points = append(points, currentPoint)

			segmentNode, err := createNodesForTrimmedCurve(graph, arc)
			if err != nil {
				log.Panic(err)
			}
			lastSegmentNode = segmentNode

		case "C", "c", "S", "s", "Q", "q", "T", "t":
			if len(points) > 1 {
				segmentNode, err := createNodesForPolyOrLine(graph, points)
				if err != nil {
					log.Panic(err)
				}
				lastSegmentNode = segmentNode
			}
			if lastSegmentNode != nil {
				compositeCurveSegmentNode := createCompositeCurveSegment(graph, lastSegmentNode, true)
				segmentNodes = append(segmentNodes, compositeCurveSegmentNode)
				lastSegmentNode = nil
			}
			points = []point{}

			var segmentNode sst.Term
			var err error

			switch command.Command {
			case "C", "c":
				var bezier cubicBezier
				if command.Command == "C" {
					bezier = cubicBezier{
						StartX:    currentPoint.X,
						StartY:    currentPoint.Y,
						ControlX1: command.Params[0],
						ControlY1: command.Params[1],
						ControlX2: command.Params[2],
						ControlY2: command.Params[3],
						EndX:      command.Params[4],
						EndY:      command.Params[5],
					}
					lastControlPoint.X = command.Params[2]
					lastControlPoint.Y = command.Params[3]
					currentPoint.X = command.Params[4]
					currentPoint.Y = command.Params[5]
				} else {
					bezier = cubicBezier{
						StartX:    currentPoint.X,
						StartY:    currentPoint.Y,
						ControlX1: currentPoint.X + command.Params[0],
						ControlY1: currentPoint.Y + command.Params[1],
						ControlX2: currentPoint.X + command.Params[2],
						ControlY2: currentPoint.Y + command.Params[3],
						EndX:      currentPoint.X + command.Params[4],
						EndY:      currentPoint.Y + command.Params[5],
					}
					lastControlPoint.X = currentPoint.X + command.Params[2]
					lastControlPoint.Y = currentPoint.Y + command.Params[3]
					currentPoint.X += command.Params[4]
					currentPoint.Y += command.Params[5]
				}

				segmentNode, err = createNodeForCubicBezier(graph, bezier)
				if err != nil {
					log.Panic(err)
				}
				lastSegmentNode = segmentNode

				points = append(points, currentPoint)

			case "S", "s":
				var controlX1, controlY1 float64
				if lastCommandIsCurve(previousCommand) {
					controlX1 = 2*currentPoint.X - lastControlPoint.X
					controlY1 = 2*currentPoint.Y - lastControlPoint.Y
				} else {
					controlX1 = currentPoint.X
					controlY1 = currentPoint.Y
				}
				var bezier cubicBezier
				if command.Command == "S" {
					bezier = cubicBezier{
						StartX:    currentPoint.X,
						StartY:    currentPoint.Y,
						ControlX1: controlX1,
						ControlY1: controlY1,
						ControlX2: command.Params[2],
						ControlY2: command.Params[3],
						EndX:      command.Params[4],
						EndY:      command.Params[5],
					}
					lastControlPoint.X = command.Params[0]
					lastControlPoint.Y = command.Params[1]
					currentPoint.X = command.Params[2]
					currentPoint.Y = command.Params[3]
				} else {
					bezier = cubicBezier{
						StartX:    currentPoint.X,
						StartY:    currentPoint.Y,
						ControlX1: controlX1,
						ControlY1: controlY1,
						ControlX2: currentPoint.X + command.Params[2],
						ControlY2: currentPoint.Y + command.Params[3],
						EndX:      currentPoint.X + command.Params[4],
						EndY:      currentPoint.Y + command.Params[5],
					}
					lastControlPoint.X = currentPoint.X + command.Params[0]
					lastControlPoint.Y = currentPoint.Y + command.Params[1]
					currentPoint.X += command.Params[2]
					currentPoint.Y += command.Params[3]
				}
				segmentNode, err = createNodeForCubicBezier(graph, bezier)
				if err != nil {
					log.Panic(err)
				}
				lastSegmentNode = segmentNode

				points = append(points, currentPoint)

			case "Q", "q":
				var bezier quadraticBezier
				if command.Command == "Q" {
					bezier = quadraticBezier{
						StartX:   currentPoint.X,
						StartY:   currentPoint.Y,
						ControlX: command.Params[0],
						ControlY: command.Params[1],
						EndX:     command.Params[2],
						EndY:     command.Params[3],
					}
					lastControlPoint.X = command.Params[0]
					lastControlPoint.Y = command.Params[1]
					currentPoint.X = command.Params[2]
					currentPoint.Y = command.Params[3]
				} else {
					bezier = quadraticBezier{
						StartX:   currentPoint.X,
						StartY:   currentPoint.Y,
						ControlX: currentPoint.X + command.Params[0],
						ControlY: currentPoint.Y + command.Params[1],
						EndX:     currentPoint.X + command.Params[2],
						EndY:     currentPoint.Y + command.Params[3],
					}
					lastControlPoint.X = currentPoint.X + command.Params[0]
					lastControlPoint.Y = currentPoint.Y + command.Params[1]
					currentPoint.X += command.Params[2]
					currentPoint.Y += command.Params[3]
				}

				segmentNode, err = createNodeForQuadraticBezier(graph, bezier)
				if err != nil {
					log.Panic(err)
				}
				lastSegmentNode = segmentNode

				points = append(points, currentPoint)

			case "T", "t":
				var controlX, controlY float64
				if lastCommandIsCurve(previousCommand) {
					controlX = 2*currentPoint.X - lastControlPoint.X
					controlY = 2*currentPoint.Y - lastControlPoint.Y
				} else {
					controlX = currentPoint.X
					controlY = currentPoint.Y
				}

				if command.Command == "T" {
					bezier := quadraticBezier{
						StartX:   currentPoint.X,
						StartY:   currentPoint.Y,
						ControlX: controlX,
						ControlY: controlY,
						EndX:     command.Params[0],
						EndY:     command.Params[1],
					}
					segmentNode, err = createNodeForQuadraticBezier(graph, bezier)
					if err != nil {
						log.Panic(err)
					}
					lastControlPoint.X = controlX
					lastControlPoint.Y = controlY
					currentPoint.X = command.Params[0]
					currentPoint.Y = command.Params[1]
				} else {
					bezier := quadraticBezier{
						StartX:   currentPoint.X,
						StartY:   currentPoint.Y,
						ControlX: controlX,
						ControlY: controlY,
						EndX:     currentPoint.X + command.Params[0],
						EndY:     currentPoint.Y + command.Params[1],
					}
					segmentNode, err = createNodeForQuadraticBezier(graph, bezier)
					if err != nil {
						log.Panic(err)
					}
					lastControlPoint.X = controlX
					lastControlPoint.Y = controlY
					currentPoint.X += command.Params[0]
					currentPoint.Y += command.Params[1]
				}
				lastSegmentNode = segmentNode

				points = append(points, currentPoint)
			}

		case "Z", "z":
			points = append(points, initialPoint)
			if len(points) > 1 {
				segmentNode, err := createNodesForPolyOrLine(graph, points)
				if err != nil {
					log.Panic(err)
				}
				lastSegmentNode = segmentNode
			}
			if lastSegmentNode != nil {
				compositeCurveSegmentNode := createCompositeCurveSegment(graph, lastSegmentNode, false)
				segmentNodes = append(segmentNodes, compositeCurveSegmentNode)
				lastSegmentNode = nil
			}
			points = []point{}
		}

		previousCommand = command.Command
		fmt.Printf("Current Point: %v\n", currentPoint)
	}

	if len(points) > 1 {
		segmentNode, err := createNodesForPolyOrLine(graph, points)
		if err != nil {
			log.Panic(err)
		}

		lastSegmentNode = segmentNode
	}
	if lastSegmentNode != nil {
		compositeCurveSegmentNode := createCompositeCurveSegment(graph, lastSegmentNode, false)
		segmentNodes = append(segmentNodes, compositeCurveSegmentNode)
		lastSegmentNode = nil
	}

	collection := graph.CreateCollection(segmentNodes...)

	compositeCurveNode.AddStatement(rep.Segments, collection)

	fmt.Println()
	return compositeCurveNode, nil
}

func createCompositeCurveSegment(graph sst.NamedGraph, segmentNode sst.Term, isContinuous bool) sst.Term {
	compositeCurveSegmentNode := graph.CreateIRINode(uuid.New().String())

	compositeCurveSegmentNode.AddStatement(rdf.Type, rep.CompositeCurveSegment)

	compositeCurveSegmentNode.AddStatement(rep.ParentCurve, segmentNode)

	if isContinuous {
		compositeCurveSegmentNode.AddStatement(rep.Transition, rep.TransitionCode_Continuous)
	} else {
		compositeCurveSegmentNode.AddStatement(rep.Transition, rep.TransitionCode_Discontinuous)
	}

	compositeCurveSegmentNode.AddStatement(rep.SameSense, sst.Boolean(true))

	return compositeCurveSegmentNode
}

func lastCommandIsCurve(command string) bool {
	return command == "C" || command == "c" || command == "S" || command == "s" || command == "Q" || command == "q" || command == "T" || command == "t"
}

func createNodesForPolyOrLine(graph sst.NamedGraph, points []point) (sst.IBNode, error) {
	var node sst.IBNode
	var err error

	if len(points) == 2 {
		line := line{
			X1: points[0].X,
			Y1: points[0].Y,
			X2: points[1].X,
			Y2: points[1].Y,
		}
		node, err = createNodeForLine(graph, line)
		if err != nil {
			return nil, err
		}
	} else if len(points) > 2 {
		polyline := polyline{Points: pointsToString(points)}
		node, err = createNodeForPolyline(graph, polyline)
		if err != nil {
			return nil, err
		}
	}
	return node, nil
}

func isSingleShape(commands []pathCommand) (bool, string) {
	if len(commands) == 0 {
		return false, "empty"
	}

	var hasLine, hasPoly, hasCubic, hasQuadratic, hasArc bool
	var shapeType string

	for i, command := range commands {
		switch command.Command {
		case "M", "m":
			if i > 0 {
				return false, "compositeCurve"
			}
		case "L", "l", "H", "h", "V", "v":
			if hasCubic || hasQuadratic || hasArc || hasPoly {
				return false, "compositeCurve"
			}
			hasLine = true
		case "Z", "z":
			if !hasLine && !hasPoly {
				return false, "compositeCurve"
			}
			hasPoly = true
		case "A", "a":
			if hasCubic || hasQuadratic || hasArc || hasLine || hasPoly {
				return false, "compositeCurve"
			}
			hasArc = true
		case "C", "c", "S", "s":
			if hasCubic || hasQuadratic || hasArc || hasLine || hasPoly {
				return false, "compositeCurve"
			}
			hasCubic = true
		case "Q", "q", "T", "t":
			if hasCubic || hasQuadratic || hasArc || hasLine || hasPoly {
				return false, "compositeCurve"
			}
			hasQuadratic = true
		default:
			return false, "compositeCurve"
		}
	}

	if hasCubic {
		shapeType = "cubicBezierCurve"
	} else if hasQuadratic {
		shapeType = "quadraticBezierCurve"
	} else if hasArc {
		shapeType = "trimmedCurve"
	} else if hasPoly {
		shapeType = "polygon"
	} else if hasLine {
		shapeType = "polyline"
	}

	return true, shapeType
}

func createNodesForTrimmedCurve(graph sst.NamedGraph, arc ellipticalArc) (sst.IBNode, error) {
	cx, cy := getArcCenterPoint(arc)
	node := graph.CreateIRINode(uuid.New().String())

	node.AddStatement(rdf.Type, rep.TrimmedCurve)

	if arc.RX == arc.RY {
		circle := circle{
			CX:        cx,
			CY:        cy,
			Radius:    arc.RX,
			Direction: float64(arc.XAxisRotation),
		}
		log.Printf("Detected Circle: CenterX=%f, CenterY=%f, Radius=%f, Direction=%f", circle.CX, circle.CY, circle.Radius, circle.Direction)

		basisCurve, err := createNodeForCircle(graph, circle)
		if err != nil {
			log.Panic(err)
		}
		node.AddStatement(rep.BasisCurve, basisCurve)
	} else {
		ellipse := ellipse{
			CX:        cx,
			CY:        cy,
			RX:        arc.RX,
			RY:        arc.RY,
			Direction: float64(arc.XAxisRotation),
		}
		log.Printf("Detected Ellipse: CenterX=%f, CenterY=%f, RX=%f, RY=%f, Direction=%f", ellipse.CX, ellipse.CY, ellipse.RX, ellipse.RY, ellipse.Direction)

		basisCurve, err := createNodeForEllipse(graph, ellipse)
		if err != nil {
			log.Panic(err)
		}
		node.AddStatement(rep.BasisCurve, basisCurve)
	}

	trim1 := createCartesianPoint(graph, arc.StartX, arc.StartY)
	trim2 := createCartesianPoint(graph, arc.EndX, arc.EndY)
	node.AddStatement(rep.Trim1, trim1)
	node.AddStatement(rep.Trim2, trim2)

	startAngle := math.Atan2(float64(arc.StartY)-cy, float64(arc.StartX)-cx)
	endAngle := math.Atan2(float64(arc.EndY)-cy, float64(arc.EndX)-cx)
	log.Printf("Debug: startAngle=%f, endAngle=%f", startAngle, endAngle)

	deltaAngle := endAngle - startAngle
	log.Printf("Debug: Initial deltaAngle (before SweepFlag adjustment)=%f", deltaAngle)

	if arc.SweepFlag {
		if deltaAngle < 0 {
			deltaAngle += 2 * math.Pi
		}
		log.Printf("Debug: Adjusted deltaAngle for SweepFlag (Clockwise)=%f", deltaAngle)
	} else {
		if deltaAngle > 0 {
			deltaAngle -= 2 * math.Pi
		}
		log.Printf("Debug: Adjusted deltaAngle for SweepFlag (Counter-Clockwise)=%f", deltaAngle)
	}

	if arc.LargeArcFlag && math.Abs(deltaAngle) < math.Pi {
		if deltaAngle > 0 {
			deltaAngle -= 2 * math.Pi
		} else {
			deltaAngle += 2 * math.Pi
		}
		log.Printf("Debug: Adjusted deltaAngle for LargeArcFlag=%f", deltaAngle)
	}

	senseAgreement := deltaAngle <= 0
	log.Printf("Debug: Final senseAgreement=%v (Clockwise=%v)", senseAgreement, arc.SweepFlag)

	node.AddStatement(rep.SenseAgreement, sst.Boolean(senseAgreement))

	return node, nil
}

func getArcCenterPoint(arc ellipticalArc) (float64, float64) {
	x1, y1 := float64(arc.StartX), float64(arc.StartY)
	x2, y2 := float64(arc.EndX), float64(arc.EndY)
	rx, ry := float64(arc.RX), float64(arc.RY)
	phi := float64(arc.XAxisRotation) * math.Pi / 180.0
	fA := bool(arc.LargeArcFlag)
	fs := bool(arc.SweepFlag)

	log.Printf("StartX: %f, StartY: %f\n", x1, y1)
	log.Printf("EndX: %f, EndY: %f\n", x2, y2)
	log.Printf("RX: %f, RY: %f\n", rx, ry)
	log.Printf("XAxisRotation: %f\n", phi)
	log.Printf("LargeArcFlag: %t, SweepFlag: %t\n", fA, fs)

	x1_ := math.Cos(phi)*(x1-x2)/2 + math.Sin(phi)*(y1-y2)/2
	y1_ := -math.Sin(phi)*(x1-x2)/2 + math.Cos(phi)*(y1-y2)/2

	radiusCheck := math.Pow(x1_/rx, 2) + math.Pow(y1_/ry, 2)
	if radiusCheck > 1 {
		scale := math.Sqrt(radiusCheck)
		rx *= scale
		ry *= scale
	}

	a := math.Pow(rx, 2)*math.Pow(ry, 2) - math.Pow(rx, 2)*math.Pow(y1_, 2) - math.Pow(ry, 2)*math.Pow(x1_, 2)
	if a < 0 {
		a = 0 // Avoid negative sqrt
	}
	b := math.Pow(rx, 2)*math.Pow(y1_, 2) + math.Pow(ry, 2)*math.Pow(x1_, 2)
	c := math.Sqrt(a / b)
	if fA == fs {
		c = -c
	}

	cx_ := c * (rx * y1_ / ry)
	cy_ := c * (-ry * x1_ / rx)

	cx := math.Cos(phi)*cx_ - math.Sin(phi)*cy_ + (x1+x2)/2
	cy := math.Sin(phi)*cx_ + math.Cos(phi)*cy_ + (y1+y2)/2

	return cx, cy
}

func createNodeForQuadraticBezier(graph sst.NamedGraph, bezier quadraticBezier) (sst.IBNode, error) {
	startPointNode := createCartesianPoint(graph, bezier.StartX, bezier.StartY)

	endPointNode := createCartesianPoint(graph, bezier.EndX, bezier.EndY)

	controlPointNode := createCartesianPoint(graph, bezier.ControlX, bezier.ControlY)

	bezierNodeUUID := uuid.New().String()
	bezierNode := graph.CreateIRINode(bezierNodeUUID)

	bezierNode.AddStatement(rdf.Type, rep.BezierCurve)
	collection := graph.CreateCollection(startPointNode, controlPointNode, endPointNode)
	bezierNode.AddStatement(rep.Degree, sst.Integer(2))
	bezierNode.AddStatement(rep.ControlPointsList, collection)

	return bezierNode, nil
}

func createNodeForCubicBezier(graph sst.NamedGraph, bezier cubicBezier) (sst.IBNode, error) {

	startPointNode := createCartesianPoint(graph, bezier.StartX, bezier.StartY)
	if startPointNode == nil {
		return nil, fmt.Errorf("failed to create start point node")
	}

	endPointNode := createCartesianPoint(graph, bezier.EndX, bezier.EndY)
	if endPointNode == nil {
		return nil, fmt.Errorf("failed to create end point node")
	}

	controlPoint1Node := createCartesianPoint(graph, bezier.ControlX1, bezier.ControlY1)
	if controlPoint1Node == nil {
		return nil, fmt.Errorf("failed to create control point 1 node")
	}

	controlPoint2Node := createCartesianPoint(graph, bezier.ControlX2, bezier.ControlY2)
	if controlPoint2Node == nil {
		return nil, fmt.Errorf("failed to create control point 2 node")
	}

	bezierNodeUUID := uuid.New().String()
	bezierNode := graph.CreateIRINode(bezierNodeUUID)

	bezierNode.AddStatement(rdf.Type, rep.BezierCurve)

	collection := graph.CreateCollection(startPointNode, controlPoint1Node, controlPoint2Node, endPointNode)

	bezierNode.AddStatement(rep.Degree, sst.Integer(3))
	bezierNode.AddStatement(rep.ControlPointsList, collection)

	return bezierNode, nil
}

func createStyledItemForShape(graph sst.NamedGraph, shape styledShape, shapeNode sst.IBNode, combinedStyle shapeStyle) (sst.IBNode, error) {
	var style shapeStyle

	if combinedStyle != (shapeStyle{}) {
		style = combinedStyle
	} else {
		style = shape.GetStyle()
	}

	if style.Stroke == "" && style.StrokeWidth == "" && style.Fill == "" {
		return shapeNode, nil
	}

	styledItemNode := graph.CreateIRINode(uuid.New().String())

	styledItemNode.AddStatement(rdf.Type, rep.StyledItem)

	var styleNodes []sst.Term

	if style.Stroke != "" || style.StrokeWidth != "" {
		curveStyleNode, err := createCurveStyle(graph, style.Stroke, style.StrokeWidth)
		if err != nil {
			return nil, err
		}
		if curveStyleNode != nil {
			styleNodes = append(styleNodes, curveStyleNode)
		}
	}

	if style.Fill != "" {
		fillStyleNode, err := createFillAreaStyle(graph, style.Fill)
		if err != nil {
			return nil, err
		}
		if fillStyleNode != nil {
			styleNodes = append(styleNodes, fillStyleNode)
		}
	}

	for _, styleNode := range styleNodes {
		styledItemNode.AddStatement(rep.Style, styleNode)
	}

	styledItemNode.AddStatement(rep.ItemToStyle, shapeNode)

	return styledItemNode, nil
}

func createCurveStyle(graph sst.NamedGraph, color string, width string) (sst.IBNode, error) {
	curveStyleNode := graph.CreateIRINode(uuid.New().String())

	curveStyleNode.AddStatement(rdf.Type, rep.CurveStyle)

	if width != "" {
		strokeWidth, _ := strconv.ParseFloat(width, 64)
		curveStyleNode.AddStatement(rep.CurveWidth, sst.Double(strokeWidth))
	}

	if color != "" {
		curveStyleNode.AddStatement(rep.CurveColour, sst.String("color:"+color))
	}

	return curveStyleNode, nil
}

func createFillAreaStyle(graph sst.NamedGraph, color string) (sst.IBNode, error) {
	if color == "" {
		return nil, nil
	}

	fillStyleNode := graph.CreateIRINode(uuid.New().String())

	fillStyleNode.AddStatement(rdf.Type, rep.FillAreaStyle)

	fillStyleNode.AddStatement(rep.FillColour, sst.String("color:"+color))

	return fillStyleNode, nil
}

func createGeometricRepresentationContext(graph sst.NamedGraph) (sst.IBNode, error) {
	contextNode := graph.CreateIRINode(uuid.New().String())
	contextNode.AddStatement(rdf.Type, rep.GeometricRepresentationContext)
	contextNode.AddStatement(rep.CoordinateSpaceDimension, sst.Integer(2))
	return contextNode, nil
}

func createSymbolRepresentation(graph sst.NamedGraph, contextNode sst.IBNode, itemUUIDs []sst.Term, label string, comment string) (sst.IBNode, error) {
	symbolNode := graph.CreateIRINode(uuid.New().String())
	symbolNode.AddStatement(rdf.Type, rep.SymbolRepresentation)
	if label != "" {
		symbolNode.AddStatement(rdfs.Label, sst.String(label))
	}
	if comment != "" {
		symbolNode.AddStatement(rdfs.Comment, sst.String(comment))
	}
	symbolNode.AddStatement(rep.ContextOfItems, contextNode)

	for _, itemUUID := range itemUUIDs {
		symbolNode.AddStatement(rep.Item, itemUUID)
	}
	return symbolNode, nil
}

func createAxis2Placement2D(graph sst.NamedGraph, x, y, directionDegree float64) sst.IBNode {
	axis2Placement2D := graph.CreateIRINode(uuid.New().String())
	axis2Placement2D.AddStatement(rdf.Type, rep.Axis2Placement2D)

	location := createCartesianPoint(graph, x, y)
	axis2Placement2D.AddStatement(rep.Location, location)

	if directionDegree != 0 {
		axis2Placement2D.AddStatement(rep.RefDirectionDegree, -sst.Double(directionDegree))
	}

	return axis2Placement2D
}

var pointCache = make(map[sst.NamedGraph]map[string]sst.IBNode)

func generateCacheKey(x, y float64) string {
	return fmt.Sprintf("%f:%f", x, y)
}

func createCartesianPoint(graph sst.NamedGraph, x, y float64) sst.IBNode {
	if pointCache[graph] == nil {
		pointCache[graph] = make(map[string]sst.IBNode)
	}

	cacheKey := generateCacheKey(x, y)

	if existingPoint, found := pointCache[graph][cacheKey]; found {
		return existingPoint
	}

	cartesianPoint := graph.CreateIRINode(uuid.New().String())
	cartesianPoint.AddStatement(rdf.Type, rep.CartesianPoint)

	coordinate := sst.NewLiteralCollection(sst.Double(x), -sst.Double(y))
	cartesianPoint.AddStatement(rep.Coordinates, coordinate)

	pointCache[graph][cacheKey] = cartesianPoint

	return cartesianPoint
}

func createCartesianPoints(graph sst.NamedGraph, points ...point) ([]sst.IBNode, error) {
	var cartesianPoints []sst.IBNode
	for _, point := range points {
		cartesianPoint := createCartesianPoint(graph, point.X, point.Y)
		cartesianPoints = append(cartesianPoints, cartesianPoint)
	}
	return cartesianPoints, nil
}

func parsePoints(points string) ([]point, error) {
	var result []point
	pairs := strings.Fields(points)
	for _, pair := range pairs {
		coords := strings.Split(pair, ",")
		if len(coords) != 2 {
			return nil, fmt.Errorf("invalid point format: %s", pair)
		}
		x, err := strconv.ParseFloat(coords[0], 64)
		if err != nil {
			return nil, err
		}
		y, err := strconv.ParseFloat(coords[1], 64)
		if err != nil {
			return nil, err
		}
		result = append(result, point{X: x, Y: y})
	}
	return result, nil
}

func parsePathData(data string) ([]pathCommand, error) {
	var commands []pathCommand

	re := regexp.MustCompile(`([MmLlHhVvCcSsQqTtAaZz])|([-+]?[0-9]*\.?[0-9]+(?:[eE][-+]?[0-9]+)?)`)
	matches := re.FindAllString(data, -1)

	if matches == nil {
		return nil, fmt.Errorf("invalid path data")
	}

	var currentCommand string
	var params []float64

	for _, match := range matches {
		if len(match) == 1 && strings.ContainsAny(match, "MmLlHhVvCcSsQqTtAaZz") {
			if currentCommand != "" {
				commands = append(commands, pathCommand{Command: currentCommand, Params: params})
				params = nil
			}
			currentCommand = match
			if match == "Z" || match == "z" {
				commands = append(commands, pathCommand{Command: currentCommand, Params: nil})
				currentCommand = ""
			}
		} else {
			param, err := strconv.ParseFloat(match, 64)
			if err != nil {
				return nil, err
			}
			params = append(params, param)
		}
	}

	if currentCommand != "" && len(params) > 0 {
		commands = append(commands, pathCommand{Command: currentCommand, Params: params})
	}

	return commands, nil
}

func pointsToString(points []point) string {
	var sb strings.Builder
	for _, p := range points {
		sb.WriteString(fmt.Sprintf("%f,%f ", p.X, p.Y))
	}
	return strings.TrimSpace(sb.String())
}

