// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package svgtosst

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)
func transformShape(shape shape, transform string) shape {
	switch s := shape.(type) {
	case text:
		return transformText(s, transform)
	case rect:
		return transformRect(s, transform)
	case circle:
		return transformCircle(s, transform)
	case ellipse:
		return transformEllipse(s, transform)
	case polygon:
		return transformPolygon(s, transform)
	case polyline:
		return transformPolyline(s, transform)
	case line:
		return transformLine(s, transform)
	case path:
		return transformPath(s, transform)
	default:
		return shape
	}
}

// TransformText applies the transformation and returns the transformed Text
func transformText(t text, transform string) text {
	// Parse the transform to get the transformation matrix
	transformMatrix := parseTransform(transform)

	// Apply the transform to the text's position
	matrix := applyMatrixToPoint(float64(t.X), float64(t.Y), transformMatrix)

	// Calculate the direction from the matrix
	direction := math.Atan2(matrix[1][0], matrix[0][0]) * (180 / math.Pi)

	// Extract new position from the transformation matrix
	newX := matrix[0][2]
	newY := matrix[1][2]

	// Calculate the scale factor (assuming uniform scaling for simplicity)
	scale := math.Sqrt(transformMatrix[0][0]*transformMatrix[0][0] + transformMatrix[1][0]*transformMatrix[1][0])

	// Apply scale to the FontSize
	newFontSize := float64(t.FontSize) * scale

	// Return the transformed Text with updated coordinates and FontSize
	return text{
		X:              newX,
		Y:              newY,
		Content:        t.Content,
		Transform:      t.Transform,
		Direction:      direction,
		shapeStyle:     t.shapeStyle,
		Style:          t.Style,
		FontFamily:     t.FontFamily,
		FontSize:       newFontSize,
		FontStyle:      t.FontStyle,
		FontWeight:     t.FontWeight,
		TextDecoration: t.TextDecoration,
	}
}

// TransformRect applies the transformation and returns Rect or Square depending on the transformed dimensions
func transformRect(r rect, transform string) shape {
	// Parse the transform to get the transformation matrix
	transformMatrix := parseTransform(transform)

	// Apply the transform to the rectangle's top-left corner
	matrix := applyMatrixToPoint(float64(r.X), float64(r.Y), transformMatrix)

	// Calculate the rotation direction from the matrix elements
	direction := math.Atan2(matrix[1][0], matrix[0][0]) * (180 / math.Pi)

	// Extract the transformed position for the top-left corner from the resulting matrix
	newX := matrix[0][2]
	newY := matrix[1][2]

	// Calculate the scale factors
	scaleX := math.Sqrt(transformMatrix[0][0]*transformMatrix[0][0] + transformMatrix[1][0]*transformMatrix[1][0])
	scaleY := math.Sqrt(transformMatrix[0][1]*transformMatrix[0][1] + transformMatrix[1][1]*transformMatrix[1][1])

	// Apply scale to XLength and YLength
	newXLength := float64(r.XLength) * scaleX
	newYLength := float64(r.YLength) * scaleY

	// Apply scale to corner radii (RX and RY)
	newRX := float64(r.RX) * scaleX
	newRY := float64(r.RY) * scaleY

	// Return the transformed Rect with updated coordinates and rotation direction
	return rect{
		X:          newX,
		Y:          newY,
		XLength:    newXLength,
		YLength:    newYLength,
		RX:         newRX,
		RY:         newRY,
		Transform:  r.Transform,
		Direction:  direction,
		shapeStyle: r.shapeStyle,
	}
}

// TransformCircle applies the transformation and returns Circle or Ellipse depending on the radii
func transformCircle(c circle, transform string) shape {
	// Parse the transform to get the transformation matrix
	transformMatrix := parseTransform(transform)

	// Apply the transform to the center
	matrix := applyMatrixToPoint(float64(c.CX), float64(c.CY), transformMatrix)

	// Calculate the rotation direction from the matrix elements
	direction := math.Atan2(matrix[1][0], matrix[0][0]) * (180 / math.Pi)

	// Extract the transformed position for the top-left corner from the resulting matrix
	newX := matrix[0][2]
	newY := matrix[1][2]

	// Calculate scale factors from the transformation matrix
	scaleX := math.Sqrt(transformMatrix[0][0]*transformMatrix[0][0] + transformMatrix[1][0]*transformMatrix[1][0])
	scaleY := math.Sqrt(transformMatrix[0][1]*transformMatrix[0][1] + transformMatrix[1][1]*transformMatrix[1][1])

	// Apply the average scale to the radius
	newRadius := float64(c.Radius) * (scaleX + scaleY) / 2

	return circle{
		CX:         newX,
		CY:         newY,
		Radius:     newRadius,
		Transform:  c.Transform,
		Direction:  direction,
		shapeStyle: c.shapeStyle,
	}
}

// TransformEllipse applies the transformation and returns Ellipse or Circle depending on the radii
func transformEllipse(e ellipse, transform string) shape {
	// Parse the transform to get the transformation matrix
	transformMatrix := parseTransform(transform)

	// Apply the transform to the center point of the ellipse
	matrix := applyMatrixToPoint(float64(e.CX), float64(e.CY), transformMatrix)

	// Calculate the rotation direction from the matrix elements
	direction := math.Atan2(matrix[1][0], matrix[0][0]) * (180 / math.Pi)

	// Extract the transformed position for the top-left corner from the resulting matrix
	newX := matrix[0][2]
	newY := matrix[1][2]

	// Calculate scale factors for RX and RY from the transformation matrix
	scaleX := math.Sqrt(transformMatrix[0][0]*transformMatrix[0][0] + transformMatrix[1][0]*transformMatrix[1][0])
	scaleY := math.Sqrt(transformMatrix[0][1]*transformMatrix[0][1] + transformMatrix[1][1]*transformMatrix[1][1])

	// Apply the scale to RX and RY
	newRX := float64(e.RX) * scaleX
	newRY := float64(e.RY) * scaleY

	// Create and return a new Ellipse object with the transformed center and scaled RX, RY
	return ellipse{
		CX:         newX,
		CY:         newY,
		RX:         newRX,
		RY:         newRY,
		Transform:  e.Transform,
		Direction:  direction,
		shapeStyle: e.shapeStyle,
	}
}

// TransformPolygon applies the transformation and returns the transformed Polygon
func transformPolygon(p polygon, transform string) polygon {
	// Parse the transform to get the transformation matrix
	transformMatrix := parseTransform(transform)

	// Calculate the overall rotation direction from the matrix
	direction := math.Atan2(transformMatrix[1][0], transformMatrix[0][0]) * (180 / math.Pi)

	// Parse the points string into a slice of coordinates
	points := parseCoordinatePairs(p.Points)

	// Apply the transform to each point
	var transformedPoints []string
	for _, point := range points {
		matrix := applyMatrixToPoint(point[0], point[1], transformMatrix)
		transformedPoints = append(transformedPoints, fmt.Sprintf("%f,%f", matrix[0][2], matrix[1][2]))
	}

	// Reconstruct the points string
	newPoints := strings.Join(transformedPoints, " ")

	// Return the transformed Polygon with updated points
	return polygon{
		Points:     newPoints,
		Transform:  p.Transform,
		Direction:  direction,
		shapeStyle: p.shapeStyle,
	}
}

// TransformPolyline applies the transformation and returns the transformed Polyline
func transformPolyline(pl polyline, transform string) polyline {
	// Parse the transform to get the transformation matrix
	transformMatrix := parseTransform(transform)

	// Calculate the overall rotation direction from the matrix
	direction := math.Atan2(transformMatrix[1][0], transformMatrix[0][0]) * (180 / math.Pi)

	// Parse the points string into a slice of coordinates
	points := parseCoordinatePairs(pl.Points)

	// Apply the transform to each point
	var transformedPoints []string
	for _, point := range points {
		matrix := applyMatrixToPoint(point[0], point[1], transformMatrix)
		transformedPoints = append(transformedPoints, fmt.Sprintf("%f,%f", matrix[0][2], matrix[1][2]))
	}

	// Reconstruct the points string
	newPoints := strings.Join(transformedPoints, " ")

	// Return the transformed Polyline with updated points
	return polyline{
		Points:     newPoints,
		Transform:  pl.Transform,
		Direction:  direction,
		shapeStyle: pl.shapeStyle,
	}
}

// parsePoints parses a points string into a slice of coordinate pairs
func parseCoordinatePairs(points string) [][]float64 {
	var result [][]float64
	coords := strings.Fields(string(points)) // Split by spaces

	for _, coord := range coords {
		xy := strings.Split(coord, ",")
		if len(xy) == 2 {
			x, errX := strconv.ParseFloat(xy[0], 64)
			y, errY := strconv.ParseFloat(xy[1], 64)
			if errX == nil && errY == nil {
				result = append(result, []float64{x, y})
			}
		}
	}
	return result
}

// TransformLine applies the transformation and returns the transformed Line
func transformLine(l line, transform string) line {
	// Parse the transform to get the transformation matrix
	transformMatrix := parseTransform(transform)

	// Calculate the overall rotation direction from the matrix
	direction := math.Atan2(transformMatrix[1][0], transformMatrix[0][0]) * (180 / math.Pi)

	// Apply the transform to both endpoints of the line
	newStart := applyMatrixToPoint(float64(l.X1), float64(l.Y1), transformMatrix)
	newEnd := applyMatrixToPoint(float64(l.X2), float64(l.Y2), transformMatrix)

	// Return the transformed Line with updated coordinates and direction
	return line{
		X1:         newStart[0][2],
		Y1:         newStart[1][2],
		X2:         newEnd[0][2],
		Y2:         newEnd[1][2],
		Transform:  l.Transform,
		Direction:  direction,
		shapeStyle: l.shapeStyle,
	}
}

func transformPath(p path, transform string) path {
	fmt.Printf("original path.D: %v", p.D)
	transformMatrix := parseTransform(transform)

	// Parse the path data ("d" attribute) into commands
	commands := parsePathCommands(p.D)

	var transformedCommands []string
	var lastX, lastY float64               // Tracks the logical position
	var lastControlX, lastControlY float64 // For S and T commands

	for _, cmd := range commands {
		var transformedParams []string
		params := cmd.Params

		switch cmd.Command {
		case "M", "m":
			for i := 0; i < len(params); i += 2 {
				x, y := params[i], params[i+1]
				if cmd.Command == "m" {
					x += lastX
					y += lastY
					cmd.Command = "M"
				}
				transformedX := transformMatrix[0][0]*x + transformMatrix[0][1]*y + transformMatrix[0][2]
				transformedY := transformMatrix[1][0]*x + transformMatrix[1][1]*y + transformMatrix[1][2]
				transformedParams = append(transformedParams, fmt.Sprintf("%.6f", transformedX), fmt.Sprintf("%.6f", transformedY))
				lastX, lastY = x, y
			}
		case "L", "l":
			for i := 0; i < len(params); i += 2 {
				x, y := params[i], params[i+1]
				if cmd.Command == "l" {
					x += lastX
					y += lastY
					cmd.Command = "L"
				}
				transformedX := transformMatrix[0][0]*x + transformMatrix[0][1]*y + transformMatrix[0][2]
				transformedY := transformMatrix[1][0]*x + transformMatrix[1][1]*y + transformMatrix[1][2]
				transformedParams = append(transformedParams, fmt.Sprintf("%.6f", transformedX), fmt.Sprintf("%.6f", transformedY))
				lastX, lastY = x, y
			}

		case "H", "h":
			for _, x := range params {
				if cmd.Command == "h" {
					x += lastX
					cmd.Command = "H"
				}
				transformedX := transformMatrix[0][0]*x + transformMatrix[0][2]
				transformedParams = append(transformedParams, fmt.Sprintf("%.6f", transformedX))
				lastX = x
			}

		case "V", "v":
			for _, y := range params {
				if cmd.Command == "v" {
					y += lastY
					cmd.Command = "V"
				}
				transformedY := transformMatrix[1][1]*y + transformMatrix[1][2]
				transformedParams = append(transformedParams, fmt.Sprintf("%.6f", transformedY))
				lastY = y
			}

		case "C", "c":
			for i := 0; i < len(params); i += 6 {
				control1X, control1Y := params[i], params[i+1]
				control2X, control2Y := params[i+2], params[i+3]
				endX, endY := params[i+4], params[i+5]
				if cmd.Command == "c" {
					control1X += lastX
					control1Y += lastY
					control2X += lastX
					control2Y += lastY
					endX += lastX
					endY += lastY
					cmd.Command = "C"
				}
				transformedControl1X := transformMatrix[0][0]*control1X + transformMatrix[0][1]*control1Y + transformMatrix[0][2]
				transformedControl1Y := transformMatrix[1][0]*control1X + transformMatrix[1][1]*control1Y + transformMatrix[1][2]
				transformedControl2X := transformMatrix[0][0]*control2X + transformMatrix[0][1]*control2Y + transformMatrix[0][2]
				transformedControl2Y := transformMatrix[1][0]*control2X + transformMatrix[1][1]*control2Y + transformMatrix[1][2]
				transformedEndX := transformMatrix[0][0]*endX + transformMatrix[0][1]*endY + transformMatrix[0][2]
				transformedEndY := transformMatrix[1][0]*endX + transformMatrix[1][1]*endY + transformMatrix[1][2]
				transformedParams = append(transformedParams,
					fmt.Sprintf("%.6f", transformedControl1X), fmt.Sprintf("%.6f", transformedControl1Y),
					fmt.Sprintf("%.6f", transformedControl2X), fmt.Sprintf("%.6f", transformedControl2Y),
					fmt.Sprintf("%.6f", transformedEndX), fmt.Sprintf("%.6f", transformedEndY))
				lastX, lastY = endX, endY
				lastControlX, lastControlY = control2X, control2Y
			}

		case "S", "s":
			for i := 0; i < len(params); i += 4 {
				control2X, control2Y := params[i], params[i+1]
				endX, endY := params[i+2], params[i+3]
				if cmd.Command == "s" {
					control2X += lastX
					control2Y += lastY
					endX += lastX
					endY += lastY
					cmd.Command = "S"
				}
				control1X := 2*lastX - lastControlX
				control1Y := 2*lastY - lastControlY
				transformedControl1X := transformMatrix[0][0]*control1X + transformMatrix[0][1]*control1Y + transformMatrix[0][2]
				transformedControl1Y := transformMatrix[1][0]*control1X + transformMatrix[1][1]*control1Y + transformMatrix[1][2]
				transformedControl2X := transformMatrix[0][0]*control2X + transformMatrix[0][1]*control2Y + transformMatrix[0][2]
				transformedControl2Y := transformMatrix[1][0]*control2X + transformMatrix[1][1]*control2Y + transformMatrix[1][2]
				transformedEndX := transformMatrix[0][0]*endX + transformMatrix[0][1]*endY + transformMatrix[0][2]
				transformedEndY := transformMatrix[1][0]*endX + transformMatrix[1][1]*endY + transformMatrix[1][2]
				transformedParams = append(transformedParams,
					fmt.Sprintf("%.6f", transformedControl1X), fmt.Sprintf("%.6f", transformedControl1Y),
					fmt.Sprintf("%.6f", transformedControl2X), fmt.Sprintf("%.6f", transformedControl2Y),
					fmt.Sprintf("%.6f", transformedEndX), fmt.Sprintf("%.6f", transformedEndY))
				lastX, lastY = endX, endY
				lastControlX, lastControlY = control2X, control2Y
			}
		case "T", "t":
			for i := 0; i < len(params); i += 2 {
				endX, endY := params[i], params[i+1]
				if cmd.Command == "t" {
					endX += lastX
					endY += lastY
					cmd.Command = "T"
				}
				controlX := 2*lastX - lastControlX
				controlY := 2*lastY - lastControlY
				transformedEndX := transformMatrix[0][0]*endX + transformMatrix[0][1]*endY + transformMatrix[0][2]
				transformedEndY := transformMatrix[1][0]*endX + transformMatrix[1][1]*endY + transformMatrix[1][2]
				transformedParams = append(transformedParams,
					fmt.Sprintf("%.6f", transformedEndX), fmt.Sprintf("%.6f", transformedEndY))
				lastX, lastY = endX, endY
				lastControlX, lastControlY = controlX, controlY
			}
		case "A", "a":
			for i := 0; i < len(params); i += 7 {
				rx, ry := params[i], params[i+1]
				rotation := params[i+2]
				largeArcFlag := int(params[i+3])
				sweepFlag := int(params[i+4])
				x, y := params[i+5], params[i+6]
				if cmd.Command == "a" {
					x += lastX
					y += lastY
					cmd.Command = "A"
				}
				transformedX := transformMatrix[0][0]*x + transformMatrix[0][1]*y + transformMatrix[0][2]
				transformedY := transformMatrix[1][0]*x + transformMatrix[1][1]*y + transformMatrix[1][2]
				transformedParams = append(transformedParams,
					fmt.Sprintf("%.6f", rx), fmt.Sprintf("%.6f", ry),
					fmt.Sprintf("%.6f", rotation), fmt.Sprintf("%d", largeArcFlag),
					fmt.Sprintf("%d", sweepFlag), fmt.Sprintf("%.6f", transformedX), fmt.Sprintf("%.6f", transformedY))
				lastX, lastY = x, y
			}
		case "Z", "z":
			transformedCommands = append(transformedCommands, cmd.Command)
			continue
		}

		transformedCommands = append(transformedCommands, fmt.Sprintf("%s %s", cmd.Command, strings.Join(transformedParams, " ")))
	}

	newD := strings.Join(transformedCommands, " ")
	fmt.Printf("New path.D: %v", newD)

	return path{
		D:          newD,
		Transform:  p.Transform,
		shapeStyle: p.shapeStyle,
	}
}

// parsePathCommands parses the "d" attribute of an SVG path into PathCommand objects
func parsePathCommands(d string) []pathCommand {
	var commands []pathCommand
	var currentParams []float64

	// Regular expression to match commands and numbers
	tokens := regexp.MustCompile(`[MmZzLlHhVvCcSsQqTtAa]|-?\d*\.?\d+(?:[eE][-+]?\d+)?`).FindAllString(string(d), -1)
	var currentCommand string

	for _, token := range tokens {
		// Check if the token is a command (like M, L, A, etc.)
		if strings.ContainsAny(token, "MmZzLlHhVvCcSsQqTtAa") {
			// If there's a current command, add it with its parameters
			if currentCommand != "" && len(currentParams) > 0 {
				commands = append(commands, pathCommand{
					Command: currentCommand,
					Params:  currentParams,
				})
			}
			currentCommand = token
			currentParams = []float64{}

			if token == "Z" || token == "z" {
				commands = append(commands, pathCommand{
					Command: token,
					Params:  nil,
				})
				currentCommand = ""
			}
		} else {
			// Parse numeric parameters
			if value, err := strconv.ParseFloat(token, 64); err == nil {
				currentParams = append(currentParams, value)
			}
		}
	}

	// Add the final command and its parameters
	if currentCommand != "" && len(currentParams) > 0 {
		commands = append(commands, pathCommand{
			Command: currentCommand,
			Params:  currentParams,
		})
	}

	return commands
}

func transform() {
	circle := circle{
		CX:        100,                                             // Initial center x
		CY:        100,                                             // Initial center y
		Radius:    50,                                              // Initial radius
		Transform: "scale(3,4) scale(1,2), scale(8,6), rotate(45)", // Translation and scaling transform
	}
	shape := transformCircle(circle, "")
	fmt.Printf("Shape Direction: %.2f degrees\n", shape.GetDirection())
}

