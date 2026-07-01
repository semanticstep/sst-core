// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package svgtosst

import "encoding/xml"

type svg struct {
	XMLName   xml.Name   `xml:"svg"`
	Width     string     `xml:"width,attr"`
	Height    string     `xml:"height,attr"`
	ViewBox   string     `xml:"viewBox,attr"`
	XMLNS     string     `xml:"xmlns,attr"`
	Version   string     `xml:"version,attr"`
	Title     string     `xml:"title"`
	Desc      string     `xml:"desc"`
	Rects     []rect     `xml:"rect"`
	Texts     []text     `xml:"text"`
	Circles   []circle   `xml:"circle"`
	Ellipses  []ellipse  `xml:"ellipse"`
	Lines     []line     `xml:"line"`
	Paths     []path     `xml:"path"`
	Polygons  []polygon  `xml:"polygon"`
	Polylines []polyline `xml:"polyline"`
	Groups    []group    `xml:"g"`
}

type group struct {
	XMLName xml.Name `xml:"g"`
	Style   string   `xml:"style,attr,omitempty"`
	shapeStyle
	Transform string     `xml:"transform,attr,omitempty"`
	Rects     []rect     `xml:"rect"`
	Texts     []text     `xml:"text"`
	Circles   []circle   `xml:"circle"`
	Ellipses  []ellipse  `xml:"ellipse"`
	Lines     []line     `xml:"line"`
	Paths     []path     `xml:"path"`
	Polygons  []polygon  `xml:"polygon"`
	Polylines []polyline `xml:"polyline"`
	Groups    []group    `xml:"g"`
}

type shapeStyle struct {
	Fill        string `xml:"fill,attr"`
	Stroke      string `xml:"stroke,attr"`
	StrokeWidth string `xml:"stroke-width,attr"`
}

type shape interface {
	GetStyle() shapeStyle
	GetType() string
	GetDirection() float64
}

type styledShape interface {
	shape
}

type rect struct {
	XMLName xml.Name `xml:"rect"`
	Style   string   `xml:"style,attr"`
	XLength float64  `xml:"width,attr"`
	YLength float64  `xml:"height,attr"`
	X       float64  `xml:"x,attr"`
	Y       float64  `xml:"y,attr"`
	RX      float64  `xml:"rx,attr"`
	RY      float64  `xml:"ry,attr"`
	shapeStyle
	Transform string `xml:"transform,attr,omitempty"`
	Direction float64
}

func (r rect) GetStyle() shapeStyle {
	return r.shapeStyle
}

func (r rect) GetType() string {
	return "Rect"
}

func (r rect) GetDirection() float64 {
	return r.Direction
}

type text struct {
	XMLName        xml.Name `xml:"text"`
	Style          string   `xml:"style,attr"`
	X              float64  `xml:"x,attr"`
	Y              float64  `xml:"y,attr"`
	Content        string   `xml:",chardata"`
	FontFamily     string   `xml:"font-family,attr,omitempty"`
	FontSize       float64  `xml:"font-size,attr,omitempty"`
	FontStyle      string   `xml:"font-style,attr,omitempty"`
	FontWeight     string   `xml:"font-weight,attr,omitempty"`
	TextDecoration string   `xml:"text-decoration,attr,omitempty"`
	shapeStyle
	Transform string `xml:"transform,attr,omitempty"`
	Direction float64
}

func (t text) GetStyle() shapeStyle {
	return t.shapeStyle
}

func (t text) GetType() string {
	return "Text"
}

func (t text) GetDirection() float64 {
	return t.Direction
}

type circle struct {
	CX     float64 `xml:"cx,attr"`
	CY     float64 `xml:"cy,attr"`
	Radius float64 `xml:"r,attr"`
	shapeStyle
	Transform string `xml:"transform,attr,omitempty"`
	Direction float64
}

func (c circle) GetStyle() shapeStyle {
	return c.shapeStyle
}

func (c circle) GetType() string {
	return "Circle"
}

func (c circle) GetDirection() float64 {
	return c.Direction
}

type ellipse struct {
	CX float64 `xml:"cx,attr"`
	CY float64 `xml:"cy,attr"`
	RX float64 `xml:"rx,attr"`
	RY float64 `xml:"ry,attr"`
	shapeStyle
	Transform string `xml:"transform,attr,omitempty"`
	Direction float64
}

func (e ellipse) GetStyle() shapeStyle {
	return e.shapeStyle
}

func (e ellipse) GetType() string {
	return "Ellipse"
}

func (e ellipse) GetDirection() float64 {
	return e.Direction
}

type line struct {
	XMLName xml.Name `xml:"line"`
	X1      float64  `xml:"x1,attr"`
	Y1      float64  `xml:"y1,attr"`
	X2      float64  `xml:"x2,attr"`
	Y2      float64  `xml:"y2,attr"`
	shapeStyle
	Transform string `xml:"transform,attr,omitempty"`
	Direction float64
}

func (l line) GetStyle() shapeStyle {
	return l.shapeStyle
}

func (l line) GetType() string {
	return "Line"
}

func (l line) GetDirection() float64 {
	return l.Direction
}

type polygon struct {
	XMLName xml.Name `xml:"polygon"`
	Points  string   `xml:"points,attr"`
	shapeStyle
	Transform string `xml:"transform,attr,omitempty"`
	Direction float64
}

func (p polygon) GetStyle() shapeStyle {
	return p.shapeStyle
}

func (p polygon) GetType() string {
	return "Polygon"
}

func (p polygon) GetDirection() float64 {
	return p.Direction
}

type polyline struct {
	XMLName xml.Name `xml:"polyline"`
	Points  string   `xml:"points,attr"`
	shapeStyle
	Transform string `xml:"transform,attr,omitempty"`
	Direction float64
}

func (p polyline) GetStyle() shapeStyle {
	return p.shapeStyle
}

func (p polyline) GetType() string {
	return "Polyline"
}

func (p polyline) GetDirection() float64 {
	return p.Direction
}

type point struct {
	X float64
	Y float64
}

type path struct {
	XMLName xml.Name `xml:"path"`
	D       string   `xml:"d,attr"`
	shapeStyle
	Transform string `xml:"transform,attr,omitempty"`
	Direction float64
}

func (p path) GetStyle() shapeStyle {
	return p.shapeStyle
}

func (p path) GetType() string {
	return "Path"
}

func (p path) GetDirection() float64 {
	return p.Direction
}

type pathCommand struct {
	Command string
	Params  []float64
}

type ellipticalArc struct {
	RX            float64
	RY            float64
	StartX        float64
	StartY        float64
	EndX          float64
	EndY          float64
	XAxisRotation float64
	LargeArcFlag  bool
	SweepFlag     bool
}

type quadraticBezier struct {
	StartX   float64
	StartY   float64
	ControlX float64
	ControlY float64
	EndX     float64
	EndY     float64
}

type cubicBezier struct {
	StartX    float64
	StartY    float64
	ControlX1 float64
	ControlY1 float64
	ControlX2 float64
	ControlY2 float64
	EndX      float64
	EndY      float64
}

