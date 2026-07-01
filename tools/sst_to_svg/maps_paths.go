// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package ssttosvg

import (
	"fmt"
	"math"
)

// Generates the path for a quadratic Bézier curve (Q)
func workOutQuadraticBezierCurveForPath(startPoint, endPoint, controlPoint []float64) string {
	return fmt.Sprintf(
		"M %f,%f Q %f,%f %f,%f",
		startPoint[0], startPoint[1], // Start point
		controlPoint[0], controlPoint[1], // Control point
		endPoint[0], endPoint[1], // End point
	)
}

// Generates the path for a cubic Bézier curve (C)
func workOutCubicBezierCurveForPath(startPoint, endPoint, controlPoint1, controlPoint2 []float64) string {
	return fmt.Sprintf(
		"M %f,%f C %f,%f %f,%f %f,%f",
		startPoint[0], startPoint[1], // Start point
		controlPoint1[0], controlPoint1[1], // Control point 1
		controlPoint2[0], controlPoint2[1], // Control point 2
		endPoint[0], endPoint[1], // End point
	)
}

func workOutTrimmedCurveForPath(trim1 []float64, trim2 []float64, center []float64, xAxisRotation float64, radiusX float64, radiusY float64, isClockWise bool) string {
	isBigCurve := false

	if radiusX == radiusY && radiusX < 0 {
		isClockWise = true
		radiusX = -radiusX
		radiusY = -radiusY
	}

	var angle float64
	pathD := fmt.Sprintf("M %f,%f A %f,%f %f ", trim1[0], trim1[1], radiusX, radiusY, xAxisRotation)

	if radiusX == radiusY {
		length := math.Hypot(trim1[0]-trim2[0], trim1[1]-trim2[1])
		h := radiusX - math.Sqrt(radiusX*radiusX-math.Pow(length/2, 2))
		tan := 2 * h / length
		if isClockWise {
			tan = -tan
		}
		angle = 4 * math.Atan(tan) * 180 / math.Pi
		if angle < 0 {
			angle += 360
		}
	} else {
		dTrim1 := math.Hypot(trim1[0]-center[0], trim1[1]-center[1])
		dTrim2 := math.Hypot(trim2[0]-center[0], trim2[1]-center[1])

		newT1 := []float64{radiusX / dTrim1 * trim1[0], radiusX / dTrim1 * trim1[1]}
		newT2 := []float64{radiusX / dTrim2 * trim2[0], radiusX / dTrim2 * trim2[1]}

		length := math.Hypot(newT1[0]-newT2[0], newT1[1]-newT2[1])
		h := radiusX - math.Sqrt(radiusX*radiusX-math.Pow(length/2, 2))
		tan := 2 * h / length
		if isClockWise {
			tan = -tan
		}
		angle = 4 * math.Atan(tan) * 180 / math.Pi
		if angle < 0 {
			angle += 360
		}
	}

	if angle >= 180 {
		isBigCurve = true
	}

	if isBigCurve {
		pathD += "1,"
	} else {
		pathD += "0,"
	}

	if isClockWise {
		pathD += "1 "
	} else {
		pathD += "0 "
	}

	pathD += fmt.Sprintf("%f,%f", trim2[0], trim2[1])
	return pathD
}

