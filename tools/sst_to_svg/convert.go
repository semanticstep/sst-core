// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package ssttosvg

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/semanticstep/sst-core/sst"
)

// ConvertGraphToSVG converts a graph representation into SVG and writes it to w.
func ConvertGraphToSVG(graph sst.NamedGraph, w io.Writer) error {
	if w == nil {
		return fmt.Errorf("nil writer")
	}

	list := getCurveAndIdListForSvg(graph)

	polylineList := []polyline{}
	polygonList := []polygon{}
	pathList := []path{}
	circleList := []circle{}
	rectList := []rectangle{}
	ellipseList := []ellipse{}
	textList := []text{}

	for _, v := range list["Polyline"] {
		poly := polyline{
			Points:      v["position"],
			Fill:        v["fillColour"],
			Stroke:      v["curveColour"],
			StrokeWidth: v["curveWidth"],
		}

		if rotation, ok := v["rotation"]; ok && rotation != "" {
			poly.Transform = fmt.Sprintf("rotate(%s)", rotation)
		}

		polylineList = append(polylineList, poly)
	}

	for _, v := range list["Polygon"] {
		poly := polygon{
			Points:      v["position"],
			Fill:        v["fillColour"],
			Stroke:      v["curveColour"],
			StrokeWidth: v["curveWidth"],
		}

		if rotation, ok := v["rotation"]; ok && rotation != "" {
			poly.Transform = fmt.Sprintf("rotate(%s)", rotation)
		}

		polygonList = append(polygonList, poly)
	}

	for _, v := range list["TrimmedCurve"] {
		p := path{
			D:           v["path_d"],
			Fill:        v["fillColour"],
			Stroke:      v["curveColour"],
			StrokeWidth: v["curveWidth"],
		}

		if rotation, ok := v["rotation"]; ok && rotation != "" {
			p.Transform = fmt.Sprintf("rotate(%s)", rotation)
		}

		pathList = append(pathList, p)
	}

	for _, v := range list["BezierCurve"] {
		p := path{
			D:           v["path_d"],
			Fill:        v["fillColour"],
			Stroke:      v["curveColour"],
			StrokeWidth: v["curveWidth"],
		}

		if rotation, ok := v["rotation"]; ok && rotation != "" {
			p.Transform = fmt.Sprintf("rotate(%s)", rotation)
		}

		pathList = append(pathList, p)
	}

	for _, v := range list["Circle"] {
		c := circle{
			Cx:          v["cx"],
			Cy:          v["cy"],
			R:           v["r"],
			Fill:        v["fillColour"],
			Stroke:      v["curveColour"],
			StrokeWidth: v["curveWidth"],
		}

		var transforms []string

		if translate, ok := v["translate"]; ok && translate != "" {
			transforms = append(transforms, fmt.Sprintf("translate(%s)", translate))
		}

		if rotation, ok := v["rotation"]; ok && rotation != "" {
			transforms = append(transforms, fmt.Sprintf("rotate(%s)", rotation))
		}

		if len(transforms) > 0 {
			c.Transform = strings.Join(transforms, " ")
		}

		circleList = append(circleList, c)
	}

	for _, v := range list["Rectangle"] {
		r := rectangle{
			X:           v["x"],
			Y:           v["y"],
			Width:       v["width"],
			Height:      v["height"],
			Rx:          v["rx"],
			Ry:          v["ry"],
			Fill:        v["fillColour"],
			Stroke:      v["curveColour"],
			StrokeWidth: v["curveWidth"],
		}

		var transforms []string

		if translate, ok := v["translate"]; ok && translate != "" {
			transforms = append(transforms, fmt.Sprintf("translate(%s)", translate))
		}

		if rotation, ok := v["rotation"]; ok && rotation != "" {
			transforms = append(transforms, fmt.Sprintf("rotate(%s)", rotation))
		}

		if len(transforms) > 0 {
			r.Transform = strings.Join(transforms, " ")
		}

		rectList = append(rectList, r)
	}

	for _, v := range list["Ellipse"] {
		e := ellipse{
			Cx:          v["cx"],
			Cy:          v["cy"],
			Rx:          v["rx"],
			Ry:          v["ry"],
			Fill:        v["fillColour"],
			Stroke:      v["curveColour"],
			StrokeWidth: v["curveWidth"],
		}

		var transforms []string

		if translate, ok := v["translate"]; ok && translate != "" {
			transforms = append(transforms, fmt.Sprintf("translate(%s)", translate))
		}

		if rotation, ok := v["rotation"]; ok && rotation != "" {
			transforms = append(transforms, fmt.Sprintf("rotate(%s)", rotation))
		}

		if len(transforms) > 0 {
			e.Transform = strings.Join(transforms, " ")
		}

		ellipseList = append(ellipseList, e)
	}

	for _, v := range list["TextLiteral"] {
		t := text{
			X:           v["x"],
			Y:           v["y"],
			Text:        v["text"],
			Fill:        v["fillColour"],
			Stroke:      v["curveColour"],
			StrokeWidth: v["curveWidth"],
			FontFamily:  v["font-family"],
			FontSize:    v["font-size"],
			Bold:        "",
			Underline:   "",
		}

		var transforms []string

		if translate, ok := v["translate"]; ok && translate != "" {
			transforms = append(transforms, fmt.Sprintf("translate(%s)", translate))
		}

		if rotation, ok := v["rotation"]; ok && rotation != "" {
			transforms = append(transforms, fmt.Sprintf("rotate(%s)", rotation))
		}

		if len(transforms) > 0 {
			t.Transform = strings.Join(transforms, " ")
		}

		if v["bold"] == "true" {
			t.Bold = "bold"
		}

		if v["underscore"] == "true" {
			t.Underline = "underline"
		}

		textList = append(textList, t)
	}

	for _, v := range list["CompositeCurve"] {
		// Extract path data and styles
		p := path{
			D:           v["path_d"],
			Fill:        v["fillColour"],
			Stroke:      v["curveColour"],
			StrokeWidth: v["curveWidth"],
		}

		if rotation, ok := v["rotation"]; ok && rotation != "" {
			p.Transform = fmt.Sprintf("rotate(%s)", rotation)
		}

		pathList = append(pathList, p)
	}

	result := xmlResult{
		Version: "1.1",
		// Width:     "1000",
		// Height:    "1000",
		Polyline:  polylineList,
		Polygon:   polygonList,
		Path:      pathList,
		Circle:    circleList,
		Rectangle: rectList,
		Ellipse:   ellipseList,
		Text:      textList,
	}

	xmlStr, err := xml.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	if _, err := w.Write(xmlStr); err != nil {
		return err
	}
	return nil
}
