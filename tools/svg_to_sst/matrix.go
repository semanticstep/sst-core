// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package svgtosst

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

func combineShapeStyles(parentStyle, currentStyle shapeStyle) shapeStyle {
	combinedStyle := parentStyle

	if currentStyle.Stroke != "" {
		combinedStyle.Stroke = currentStyle.Stroke
	}
	if currentStyle.StrokeWidth != "" {
		combinedStyle.StrokeWidth = currentStyle.StrokeWidth
	}
	if currentStyle.Fill != "" {
		combinedStyle.Fill = currentStyle.Fill
	}

	return combinedStyle
}

type Matrix [3][3]float64

// Identity matrix
var IdentityMatrix = Matrix{
	{1, 0, 0},
	{0, 1, 0},
	{0, 0, 1},
}

// Matrix multiplication
func multiplyMatrix(m1, m2 Matrix) Matrix {
	var result Matrix
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			result[i][j] = 0
			for k := 0; k < 3; k++ {
				result[i][j] += m1[i][k] * m2[k][j]
			}
		}
	}
	return result
}

// Parse the transform attribute and generate a transformation matrix
func parseTransform(transform string) Matrix {
	// Initialize as the identity matrix
	result := IdentityMatrix

	// Match different transform operations
	re := regexp.MustCompile(`(\w+)\(([^)]+)\)`)
	matches := re.FindAllStringSubmatch(transform, -1)

	for _, match := range matches {
		op := match[1]
		args := parseArgs(match[2])

		switch op {
		case "translate":
			result = multiplyMatrix(result, translateMatrix(args))
		case "scale":
			result = multiplyMatrix(result, scaleMatrix(args))
		case "rotate":
			result = multiplyMatrix(result, rotateMatrix(args))
		case "matrix":
			result = multiplyMatrix(result, matrixTransform(args))
		}
		// fmt.Println("Resulting Matrix:")
		// for _, row := range result {
		// 	fmt.Println(row)
		// }
	}
	return result
}

// Parse arguments into a float64 array
func parseArgs(argStr string) []float64 {
	argStrs := strings.Split(argStr, ",")
	if len(argStrs) == 1 {
		argStrs = strings.Fields(argStr)
	}
	args := make([]float64, len(argStrs))
	for i, s := range argStrs {
		args[i], _ = strconv.ParseFloat(strings.TrimSpace(s), 64)
	}
	return args
}

// Translation transformation matrix
func translateMatrix(args []float64) Matrix {
	tx := args[0]
	ty := 0.0
	if len(args) > 1 {
		ty = args[1]
	}
	return Matrix{
		{1, 0, tx},
		{0, 1, ty},
		{0, 0, 1},
	}
}

// Scaling transformation matrix
func scaleMatrix(args []float64) Matrix {
	sx := args[0]
	sy := sx
	if len(args) > 1 {
		sy = args[1]
	}
	return Matrix{
		{sx, 0, 0},
		{0, sy, 0},
		{0, 0, 1},
	}
}

func rotateMatrix(args []float64) Matrix {
	if len(args) < 1 {
		panic("RotateMatrix requires at least one argument: the rotation angle.")
	}

	angle := args[0] * math.Pi / 180
	cos := math.Cos(angle)
	sin := math.Sin(angle)

	return Matrix{
		{cos, -sin, 0},
		{sin, cos, 0},
		{0, 0, 1},
	}
}

// MatrixTransform creates a transformation matrix from matrix(a, b, c, d, e, f) parameters.
func matrixTransform(args []float64) Matrix {
	if len(args) != 6 {
		panic("matrix transform requires 6 parameters")
	}
	return Matrix{
		{args[0], args[2], args[4]},
		{args[1], args[3], args[5]},
		{0, 0, 1},
	}
}

// applyMatrixToPoint applies a matrix to a point (x, y) and returns the new matrix
func applyMatrixToPoint(x, y float64, matrix Matrix) Matrix {
	// Perform the matrix multiplication
	newX := matrix[0][0]*x + matrix[0][1]*y + matrix[0][2]
	newY := matrix[1][0]*x + matrix[1][1]*y + matrix[1][2]

	// Return a new matrix representing the transformed point in homogeneous coordinates
	return Matrix{
		{matrix[0][0], matrix[0][1], newX},
		{matrix[1][0], matrix[1][1], newY},
		{0, 0, 1},
	}
}

func combineTransforms(parentTransform, childTransform string) string {
	if parentTransform == "" {
		return childTransform
	}
	if childTransform == "" {
		return parentTransform
	}
	return parentTransform + " " + childTransform
}
