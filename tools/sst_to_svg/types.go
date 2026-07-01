// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package ssttosvg

import "encoding/xml"

type polyline struct {
	XMLName     xml.Name `xml:"polyline"`
	Points      string   `xml:"points,attr"`
	Fill        string   `xml:"fill,attr,omitempty"`
	Stroke      string   `xml:"stroke,attr,omitempty"`
	StrokeWidth string   `xml:"stroke-width,attr,omitempty"`
	Transform   string   `xml:"transform,attr,omitempty"`
}

type polygon struct {
	XMLName     xml.Name `xml:"polygon"`
	Points      string   `xml:"points,attr"`
	Fill        string   `xml:"fill,attr,omitempty"`
	Stroke      string   `xml:"stroke,attr,omitempty"`
	StrokeWidth string   `xml:"stroke-width,attr,omitempty"`
	Transform   string   `xml:"transform,attr,omitempty"`
}

type path struct {
	XMLName     xml.Name `xml:"path"`
	D           string   `xml:"d,attr"`
	Fill        string   `xml:"fill,attr,omitempty"`
	Stroke      string   `xml:"stroke,attr,omitempty"`
	StrokeWidth string   `xml:"stroke-width,attr,omitempty"`
	Transform   string   `xml:"transform,attr,omitempty"`
}

type circle struct {
	XMLName     xml.Name `xml:"circle"`
	Cx          string   `xml:"cx,attr,omitempty"`
	Cy          string   `xml:"cy,attr,omitempty"`
	R           string   `xml:"r,attr"`
	Fill        string   `xml:"fill,attr,omitempty"`
	Stroke      string   `xml:"stroke,attr,omitempty"`
	StrokeWidth string   `xml:"stroke-width,attr,omitempty"`
	Transform   string   `xml:"transform,attr,omitempty"`
}

type ellipse struct {
	XMLName     xml.Name `xml:"ellipse"`
	Cx          string   `xml:"cx,attr,omitempty"`
	Cy          string   `xml:"cy,attr,omitempty"`
	Rx          string   `xml:"rx,attr"`
	Ry          string   `xml:"ry,attr"`
	Fill        string   `xml:"fill,attr,omitempty"`
	Stroke      string   `xml:"stroke,attr,omitempty"`
	StrokeWidth string   `xml:"stroke-width,attr,omitempty"`
	Transform   string   `xml:"transform,attr,omitempty"`
}

type rectangle struct {
	XMLName     xml.Name `xml:"rect"`
	X           string   `xml:"x,attr,omitempty"`
	Y           string   `xml:"y,attr,omitempty"`
	Width       string   `xml:"width,attr"`
	Height      string   `xml:"height,attr"`
	Rx          string   `xml:"rx,attr,omitempty"`
	Ry          string   `xml:"ry,attr,omitempty"`
	Fill        string   `xml:"fill,attr,omitempty"`
	Stroke      string   `xml:"stroke,attr,omitempty"`
	StrokeWidth string   `xml:"stroke-width,attr,omitempty"`
	Transform   string   `xml:"transform,attr,omitempty"`
}

type text struct {
	XMLName     xml.Name `xml:"text"`
	X           string   `xml:"x,attr,omitempty"`
	Y           string   `xml:"y,attr,omitempty"`
	Text        string   `xml:",chardata"`
	FontFamily  string   `xml:"font-family,attr,omitempty"`
	FontSize    string   `xml:"font-size,attr,omitempty"`
	Fill        string   `xml:"fill,attr,omitempty"`
	Stroke      string   `xml:"stroke,attr,omitempty"`
	StrokeWidth string   `xml:"stroke-width,attr,omitempty"`
	Bold        string   `xml:"font-weight,attr,omitempty"`
	Underline   string   `xml:"text-decoration,attr,omitempty"`
	Transform   string   `xml:"transform,attr,omitempty"`
}

type xmlResult struct {
	XMLName xml.Name `xml:"http://www.w3.org/2000/svg svg"`
	Version string   `xml:"version,attr"`
	// Height    string   `xml:"height,attr"`
	// Width     string   `xml:"width,attr"`
	Polyline  []polyline
	Polygon   []polygon
	Path      []path
	Circle    []circle
	Rectangle []rectangle
	Ellipse   []ellipse
	Text      []text
}

