// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package svgtosst

import (
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/semanticstep/sst-core/vocabularies/sso"
)

func createNodesForGroups(graph sst.NamedGraph, groups []group, itemUUIDs *[]sst.Term, parentTransform string, parentStyle shapeStyle) {
	for _, group := range groups {
		combinedStyle := combineShapeStyles(parentStyle, group.shapeStyle)
		combinedTransform := combineTransforms(parentTransform, group.Transform)

		createNodesForTexts(graph, group.Texts, itemUUIDs, combinedTransform, combinedStyle)
		createNodesForRects(graph, group.Rects, itemUUIDs, combinedTransform, combinedStyle)
		createNodesForCircles(graph, group.Circles, itemUUIDs, combinedTransform, combinedStyle)
		createNodesForEllipses(graph, group.Ellipses, itemUUIDs, combinedTransform, combinedStyle)
		createNodesForPolygons(graph, group.Polygons, itemUUIDs, combinedTransform, combinedStyle)
		createNodesForPolylines(graph, group.Polylines, itemUUIDs, combinedTransform, combinedStyle)
		createNodesForLines(graph, group.Lines, itemUUIDs, combinedTransform, combinedStyle)
		createNodesForPaths(graph, group.Paths, itemUUIDs, combinedTransform, combinedStyle)

		if len(group.Groups) > 0 {
			createNodesForGroups(graph, group.Groups, itemUUIDs, combinedTransform, combinedStyle)
		}
	}
}

func createNodeForShape(graph sst.NamedGraph, shape shape) (sst.IBNode, error) {
	var node sst.IBNode
	var err error

	switch s := shape.(type) {
	case text:
		node, err = createNodeForText(graph, s)
	case rect:
		node, err = createNodeForRect(graph, s)
	case circle:
		node, err = createNodeForCircle(graph, s)
	case ellipse:
		node, err = createNodeForEllipse(graph, s)
	case polygon:
		node, err = createNodeForPolygon(graph, s)
	case polyline:
		node, err = createNodeForPolyline(graph, s)
	case line:
		node, err = createNodeForLine(graph, s)
	case path:
		node, err = createNodeForPath(graph, s)
	default:
		err = fmt.Errorf("unsupported shape type: %T", s)
	}

	return node, err
}

func createNodesForTexts(graph sst.NamedGraph, texts []text, itemUUIDs *[]sst.Term, parentTransform string, parentStyle shapeStyle) {
	for _, text := range texts {
		combinedTransform := combineTransforms(parentTransform, text.Transform)

		combinedStyle := combineShapeStyles(parentStyle, text.shapeStyle)

		if combinedStyle.Fill == "#ffffff" || combinedStyle.Fill == "#FFFFFF" {
			continue
		}

		shape := transformShape(text, combinedTransform)
		node, err := createNodeForShape(graph, shape)
		if err != nil {
			log.Panic(err)
		}

		styledNode, err := createStyledItemForShape(graph, shape, node, combinedStyle)
		if err != nil {
			log.Panic(err)
		}
		*itemUUIDs = append(*itemUUIDs, styledNode)
	}
}

func createNodeForText(graph sst.NamedGraph, text text) (sst.IBNode, error) {
	position := createAxis2Placement2D(graph, text.X, text.Y, text.Direction)

	textNode := graph.CreateIRINode(uuid.New().String())

	textNode.AddStatement(rdf.Type, rep.TextLiteral)
	textNode.AddStatement(rep.Alignment, rep.TextAlignment_baseline_left)
	textNode.AddStatement(rep.Position, position)
	textNode.AddStatement(rep.TextDirection, rep.TextPath_right)
	textNode.AddStatement(rep.Literal, sst.String(text.Content))

	fontNode, err := createTextFont(graph, text)
	if err != nil {
		log.Panic(err)
	}

	textNode.AddStatement(rep.Font, fontNode)

	return textNode, nil
}

func createTextFont(graph sst.NamedGraph, text text) (sst.IBNode, error) {

	fontNode := graph.CreateIRINode(uuid.New().String())

	fontNode.AddStatement(rdf.Type, rep.TextFont)
	if text.FontFamily != "" {
		fontNode.AddStatement(sso.ID, sst.String(text.FontFamily))
	}

	if text.FontSize != 0 {
		fontNode.AddStatement(rep.FontSize, sst.Double(text.FontSize))
	}

	if text.FontStyle == "italic" {
		fontNode.AddStatement(rep.FontModifier, rep.TextModifer_italic)
	}

	if text.FontWeight == "bold" {
		fontNode.AddStatement(rep.FontModifier, rep.TextModifer_bold)
	}

	switch text.TextDecoration {
	case "underline":
		fontNode.AddStatement(rep.FontModifier, rep.TextModifer_underscore)
	case "line-through":
		fontNode.AddStatement(rep.FontModifier, rep.TextModifer_strikethrough)
	}

	return fontNode, nil
}

func createNodesForRects(graph sst.NamedGraph, rects []rect, itemUUIDs *[]sst.Term, parentTransform string, parentStyle shapeStyle) {
	for _, rect := range rects {
		combinedTransform := combineTransforms(parentTransform, rect.Transform)

		combinedStyle := combineShapeStyles(parentStyle, rect.shapeStyle)

		if combinedStyle.Fill == "#ffffff" || combinedStyle.Fill == "#FFFFFF" {
			continue
		}

		shape := transformShape(rect, combinedTransform)
		node, err := createNodeForShape(graph, shape)
		if err != nil {
			log.Panic(err)
		}
		styledNode, err := createStyledItemForShape(graph, shape, node, combinedStyle)
		if err != nil {
			log.Panic(err)
		}
		*itemUUIDs = append(*itemUUIDs, styledNode)
	}
}

func createNodeForRect(graph sst.NamedGraph, rect rect) (sst.IBNode, error) {
	position := createAxis2Placement2D(graph, rect.X, rect.Y, rect.Direction)

	node := graph.CreateIRINode(uuid.New().String())

	if rect.RX > 0 || rect.RY > 0 {
		node.AddStatement(rdf.Type, rep.RoundedRectangle)

		if rect.RX > 0 {
			node.AddStatement(rep.RadiusX, sst.Double(rect.RX))
		}
		if rect.RY > 0 {
			node.AddStatement(rep.RadiusY, sst.Double(rect.RY))
		}
	} else {
		node.AddStatement(rdf.Type, rep.Rectangle)
	}

	node.AddStatement(rep.Position, position)

	node.AddStatement(rep.XLength, sst.Double(rect.XLength))

	node.AddStatement(rep.YLength, sst.Double(rect.YLength))
	return node, nil
}

func createNodesForCircles(graph sst.NamedGraph, circles []circle, itemUUIDs *[]sst.Term, parentTransform string, parentStyle shapeStyle) {
	for _, circle := range circles {
		combinedTransform := combineTransforms(parentTransform, circle.Transform)

		combinedStyle := combineShapeStyles(parentStyle, circle.shapeStyle)

		if combinedStyle.Fill == "#ffffff" || combinedStyle.Fill == "#FFFFFF" {
			continue
		}

		shape := transformShape(circle, combinedTransform)
		node, err := createNodeForShape(graph, shape)
		if err != nil {
			log.Panic(err)
		}
		styledNode, err := createStyledItemForShape(graph, shape, node, combinedStyle)
		if err != nil {
			log.Panic(err)
		}
		*itemUUIDs = append(*itemUUIDs, styledNode)
	}
}

func createNodeForCircle(graph sst.NamedGraph, circle circle) (sst.IBNode, error) {
	position := createAxis2Placement2D(graph, circle.CX, circle.CY, circle.GetDirection())

	node := graph.CreateIRINode(uuid.New().String())
	node.AddStatement(rdf.Type, rep.Circle)
	node.AddStatement(rep.Position, position)
	node.AddStatement(rep.Radius, sst.Double(circle.Radius))
	return node, nil
}

func createNodesForEllipses(graph sst.NamedGraph, ellipses []ellipse, itemUUIDs *[]sst.Term, parentTransform string, parentStyle shapeStyle) {
	for _, ellipse := range ellipses {
		combinedTransform := combineTransforms(parentTransform, ellipse.Transform)

		combinedStyle := combineShapeStyles(parentStyle, ellipse.shapeStyle)

		if combinedStyle.Fill == "#ffffff" || combinedStyle.Fill == "#FFFFFF" {
			continue
		}

		shape := transformShape(ellipse, combinedTransform)
		node, err := createNodeForShape(graph, shape)
		if err != nil {
			log.Panic(err)
		}
		styledNode, err := createStyledItemForShape(graph, shape, node, combinedStyle)
		if err != nil {
			log.Panic(err)
		}
		*itemUUIDs = append(*itemUUIDs, styledNode)
	}
}

func createNodeForEllipse(graph sst.NamedGraph, ellipse ellipse) (sst.IBNode, error) {
	position := createAxis2Placement2D(graph, ellipse.CX, ellipse.CY, ellipse.GetDirection())

	node := graph.CreateIRINode(uuid.New().String())
	node.AddStatement(rdf.Type, rep.Ellipse)
	node.AddStatement(rep.Position, position)
	node.AddStatement(rep.SemiAxis1, sst.Double(ellipse.RX))
	node.AddStatement(rep.SemiAxis2, sst.Double(ellipse.RY))
	return node, nil
}

func createNodesForPolygons(graph sst.NamedGraph, polygons []polygon, itemUUIDs *[]sst.Term, parentTransform string, parentStyle shapeStyle) {
	for _, polygon := range polygons {
		combinedTransform := combineTransforms(parentTransform, polygon.Transform)

		combinedStyle := combineShapeStyles(parentStyle, polygon.shapeStyle)

		if combinedStyle.Fill == "#ffffff" || combinedStyle.Fill == "#FFFFFF" {
			continue
		}

		shape := transformShape(polygon, combinedTransform)
		node, err := createNodeForShape(graph, shape)
		if err != nil {
			log.Panic(err)
		}
		styledNode, err := createStyledItemForShape(graph, shape, node, combinedStyle)
		if err != nil {
			log.Panic(err)
		}
		*itemUUIDs = append(*itemUUIDs, styledNode)
	}
}

func createNodeForPolygon(graph sst.NamedGraph, polygon polygon) (sst.IBNode, error) {
	node := graph.CreateIRINode(uuid.New().String())
	node.AddStatement(rdf.Type, rep.Polygon)
	points, err := parsePoints(string(polygon.Points))
	if err != nil {
		log.Panic(err)
	}
	cartesianPoints, err := createCartesianPoints(graph, points...)
	if err != nil {
		log.Panic(err)
	}
	objectPoints := make([]sst.Term, len(cartesianPoints))
	for i, point := range cartesianPoints {
		objectPoints[i] = point
	}
	collection := graph.CreateCollection(objectPoints...)
	node.AddStatement(rep.Points, collection)
	return node, nil
}

func createNodesForPolylines(graph sst.NamedGraph, polylines []polyline, itemUUIDs *[]sst.Term, parentTransform string, parentStyle shapeStyle) {
	for _, polyline := range polylines {
		combinedTransform := combineTransforms(parentTransform, polyline.Transform)

		combinedStyle := combineShapeStyles(parentStyle, polyline.shapeStyle)

		if combinedStyle.Fill == "#ffffff" || combinedStyle.Fill == "#FFFFFF" {
			continue
		}

		shape := transformShape(polyline, combinedTransform)
		node, err := createNodeForShape(graph, shape)
		if err != nil {
			log.Panic(err)
		}
		styledNode, err := createStyledItemForShape(graph, shape, node, combinedStyle)
		if err != nil {
			log.Panic(err)
		}
		*itemUUIDs = append(*itemUUIDs, styledNode)
	}
}

func createNodeForPolyline(graph sst.NamedGraph, polyline polyline) (sst.IBNode, error) {
	node := graph.CreateIRINode(uuid.New().String())
	node.AddStatement(rdf.Type, rep.Polyline)
	points, err := parsePoints(string(polyline.Points))
	if err != nil {
		log.Panic(err)
	}
	cartesianPoints, err := createCartesianPoints(graph, points...)
	if err != nil {
		log.Panic(err)
	}
	objectPoints := make([]sst.Term, len(cartesianPoints))
	for i, point := range cartesianPoints {
		objectPoints[i] = point
	}
	collection := graph.CreateCollection(objectPoints...)
	node.AddStatement(rep.Points, collection)
	return node, nil
}

func createNodesForLines(graph sst.NamedGraph, lines []line, itemUUIDs *[]sst.Term, parentTransform string, parentStyle shapeStyle) {
	for _, line := range lines {
		combinedTransform := combineTransforms(parentTransform, line.Transform)

		combinedStyle := combineShapeStyles(parentStyle, line.shapeStyle)

		if combinedStyle.Fill == "#ffffff" || combinedStyle.Fill == "#FFFFFF" {
			continue
		}

		shape := transformShape(line, combinedTransform)
		node, err := createNodeForShape(graph, shape)
		if err != nil {
			log.Panic(err)
		}
		styledNode, err := createStyledItemForShape(graph, shape, node, combinedStyle)
		if err != nil {
			log.Panic(err)
		}
		*itemUUIDs = append(*itemUUIDs, styledNode)
	}
}

func createNodeForLine(graph sst.NamedGraph, line line) (sst.IBNode, error) {
	node := graph.CreateIRINode(uuid.New().String())
	node.AddStatement(rdf.Type, rep.Polyline)

	points := []point{
		{X: line.X1, Y: line.Y1},
		{X: line.X2, Y: line.Y2},
	}
	cartesianPoints, err := createCartesianPoints(graph, points...)
	if err != nil {
		log.Panic(err)
	}
	objectPoints := make([]sst.Term, len(cartesianPoints))
	for i, point := range cartesianPoints {
		objectPoints[i] = point
	}
	collection := graph.CreateCollection(objectPoints...)

	node.AddStatement(rep.Points, collection)

	return node, nil
}

func createNodesForPaths(graph sst.NamedGraph, paths []path, itemUUIDs *[]sst.Term, parentTransform string, parentStyle shapeStyle) {
	for _, path := range paths {
		combinedTransform := combineTransforms(parentTransform, path.Transform)

		combinedStyle := combineShapeStyles(parentStyle, path.shapeStyle)

		if combinedStyle.Fill == "#ffffff" || combinedStyle.Fill == "#FFFFFF" {
			continue
		}

		shape := transformShape(path, combinedTransform)
		node, err := createNodeForShape(graph, shape)
		if err != nil {
			log.Panic(err)
		}

		styledNode, err := createStyledItemForShape(graph, shape, node, combinedStyle)
		if err != nil {
			log.Panic(err)
		}
		*itemUUIDs = append(*itemUUIDs, styledNode)
	}
}
