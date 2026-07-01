// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

// This source file provides means to set sst literal values.
// Notice that it should never be assumed that literals of types in this package
// are returned from sst.IBNode triple accessing methods. The interface sst.Literal
// and it's specializations should be used instead.

package sst

import "fmt"

// String is a wrapper for the build-in string type that implements the Literal interface.
// It represents the xsd:string data type.
type String string

var (
	_ Term    = (*String)(nil)
	_ Literal = (*String)(nil)
)

// TermKind implements sst.Object.TermKind().
func (String) TermKind() TermKind {
	return TermKindLiteral
}

// DataType implements Literal.DataType() that returns an internal const IBNode for the String data type.
// e.g.
// String.DataType().IRI() will return http://www.w3.org/2001/XMLSchema#string, or in short xsd:string
func (String) DataType() IBNode {
	return &literalTypeString.ibNode
}

// apiValue implements Literal.apiValue().
func (s String) apiValue() interface{} {
	return s
}

// Byte is a wrapper for the built-in int8 type that implements the Literal interface.
// It represents the xsd:byte data type.
type Byte int8

var (
	_ Term    = (*Byte)(nil)
	_ Literal = (*Byte)(nil)
)

// TermKind implements sst.Object.TermKind().
func (Byte) TermKind() TermKind {
	return TermKindLiteral
}

// DataType implements Literal.DataType() that returns an internal const IBNode for the Byte data type.
// e.g.
// Byte.DataType().IRI() will return http://www.w3.org/2001/XMLSchema#byte, or in short xsd:byte
func (Byte) DataType() IBNode {
	return &literalTypeByte.ibNode
}

// apiValue implements Literal.apiValue().
func (b Byte) apiValue() interface{} {
	return b
}

// Short is a wrapper for the built-in int16 type that implements the Literal interface.
// It represents the xsd:short data type.
type Short int16

var (
	_ Term    = (*Short)(nil)
	_ Literal = (*Short)(nil)
)

// TermKind implements sst.Object.TermKind().
func (Short) TermKind() TermKind {
	return TermKindLiteral
}

// DataType implements Literal.DataType() that returns an internal const IBNode for the Short data type.
// e.g.
// Short.DataType().IRI() will return http://www.w3.org/2001/XMLSchema#short, or in short xsd:short
func (Short) DataType() IBNode {
	return &literalTypeShort.ibNode
}

// apiValue implements Literal.apiValue().
func (s Short) apiValue() interface{} {
	return s
}

// Int is a wrapper for the built-in int32 type that implements the Literal interface.
// It represents the xsd:int data type.
type Int int32

var (
	_ Term    = (*Int)(nil)
	_ Literal = (*Int)(nil)
)

// TermKind implements sst.Object.TermKind().
func (Int) TermKind() TermKind {
	return TermKindLiteral
}

// DataType implements Literal.DataType() that returns an internal const IBNode for the Int data type.
// e.g.
// Int.DataType().IRI() will return http://www.w3.org/2001/XMLSchema#int, or in short xsd:int
func (Int) DataType() IBNode {
	return &literalTypeInt.ibNode
}

// apiValue implements Literal.apiValue().
func (i Int) apiValue() interface{} {
	return i
}

// Long is a wrapper for the built-in int64 type that implements the Literal interface.
// It represents the xsd:long data type.
type Long int64

var (
	_ Term    = (*Long)(nil)
	_ Literal = (*Long)(nil)
)

// TermKind implements sst.Object.TermKind().
func (Long) TermKind() TermKind {
	return TermKindLiteral
}

// DataType implements Literal.DataType() that returns an internal const IBNode for the Long data type.
// e.g.
// Long.DataType().IRI() will return http://www.w3.org/2001/XMLSchema#long, or in short xsd:long
func (Long) DataType() IBNode {
	return &literalTypeLong.ibNode
}

// apiValue implements Literal.apiValue().
func (l Long) apiValue() interface{} {
	return l
}

// UnsignedByte is a wrapper for the built-in uint8 type that implements the Literal interface.
// It represents the xsd:unsignedByte data type.
type UnsignedByte uint8

var (
	_ Term    = (*UnsignedByte)(nil)
	_ Literal = (*UnsignedByte)(nil)
)

// TermKind implements sst.Object.TermKind().
func (UnsignedByte) TermKind() TermKind {
	return TermKindLiteral
}

// DataType implements Literal.DataType() that returns an internal const IBNode for the UnsignedByte data type.
// e.g.
// UnsignedByte.DataType().IRI() will return http://www.w3.org/2001/XMLSchema#unsignedByte, or in short xsd:unsignedByte
func (UnsignedByte) DataType() IBNode {
	return &literalTypeUnsignedByte.ibNode
}

// apiValue implements Literal.apiValue().
func (ub UnsignedByte) apiValue() interface{} {
	return ub
}

// UnsignedShort is a wrapper for the built-in uint16 type that implements the Literal interface.
// It represents the xsd:unsignedShort data type.
type UnsignedShort uint16

var (
	_ Term    = (*UnsignedShort)(nil)
	_ Literal = (*UnsignedShort)(nil)
)

// TermKind implements sst.Object.TermKind().
func (UnsignedShort) TermKind() TermKind {
	return TermKindLiteral
}

// DataType implements Literal.DataType() that returns an internal const IBNode for the UnsignedShort data type.
// e.g.
// UnsignedShort.DataType().IRI() will return http://www.w3.org/2001/XMLSchema#unsignedShort, or in short xsd:unsignedShort
func (UnsignedShort) DataType() IBNode {
	return &literalTypeUnsignedShort.ibNode
}

// apiValue implements Literal.apiValue().
func (us UnsignedShort) apiValue() interface{} {
	return us
}

// UnsignedInt is a wrapper for the built-in uint32 type that implements the Literal interface.
// It represents the xsd:unsignedInt data type.
type UnsignedInt uint32

var (
	_ Term    = (*UnsignedInt)(nil)
	_ Literal = (*UnsignedInt)(nil)
)

// TermKind implements sst.Object.TermKind().
func (UnsignedInt) TermKind() TermKind {
	return TermKindLiteral
}

// DataType implements Literal.DataType() that returns an internal const IBNode for the UnsignedInt data type.
// e.g.
// UnsignedInt.DataType().IRI() will return http://www.w3.org/2001/XMLSchema#unsignedInt, or in short xsd:unsignedInt
func (UnsignedInt) DataType() IBNode {
	return &literalTypeUnsignedInt.ibNode
}

// apiValue implements Literal.apiValue().
func (ui UnsignedInt) apiValue() interface{} {
	return ui
}

// UnsignedLong is a wrapper for the built-in uint64 type that implements the Literal interface.
// It represents the xsd:unsignedLong data type.
type UnsignedLong uint64

var (
	_ Term    = (*UnsignedLong)(nil)
	_ Literal = (*UnsignedLong)(nil)
)

// TermKind implements sst.Object.TermKind().
func (UnsignedLong) TermKind() TermKind {
	return TermKindLiteral
}

// DataType implements Literal.DataType() that returns an internal const IBNode for the UnsignedLong data type.
// e.g.
// UnsignedLong.DataType().IRI() will return http://www.w3.org/2001/XMLSchema#unsignedLong, or in short xsd:unsignedLong
func (UnsignedLong) DataType() IBNode {
	return &literalTypeUnsignedLong.ibNode
}

// apiValue implements Literal.apiValue().
func (ul UnsignedLong) apiValue() interface{} {
	return ul
}

// LangString is a struct for rdf:langString that implements the Literal interface.
// It represents the rdf:langString data type.
type LangString struct {
	Val     string
	LangTag string
}

// LangStringOf constructs LangString from given value and language.
func LangStringOf(val, langTag string) LangString {
	return LangString{
		Val:     val,
		LangTag: langTag,
	}
}

var (
	_ Term    = (*LangString)(nil)
	_ Literal = (*LangString)(nil)
)

// TermKind implements sst.Object.TermKind().
func (LangString) TermKind() TermKind {
	return TermKindLiteral
}

// DataType implements Literal.DataType() that returns an internal const IBNode for the LangString data type.
// e.g.
// LangString.DataType().IRI() will return http://www.w3.org/1999/02/22-rdf-syntax-ns#langString, or in short rdf:langString
func (LangString) DataType() IBNode {
	return &literalTypeLangString.ibNode
}

// apiValue implements Literal.apiValue().
func (s LangString) apiValue() interface{} {
	return s
}

// TypedString is a struct that holds a string value together with an explicit data type Node.
// It implements the Literal interface.
type TypedString struct {
	Val  string
	Type Node
}

// TypedStringOf constructs LangString from given value and language.
func TypedStringOf(val string, t Node) TypedString {
	return TypedString{
		Val:  val,
		Type: t,
	}
}

var (
	_ Term    = (*TypedString)(nil)
	_ Literal = (*TypedString)(nil)
)

// TermKind implements sst.Object.TermKind().
func (TypedString) TermKind() TermKind {
	return TermKindLiteral
}
func (ts TypedString) DataType() IBNode {
	var tp IBNode

	switch p := ts.Type.(type) {
	case IBNode:
		tp = p.(*ibNode)
	case Elementer:
		tp = literalElementToIBNode(p.VocabularyElement())
	default:
		panic(fmt.Sprintf("unsupported type for TypedString: %T", ts.Type))
	}
	return tp
}

func literalElementToIBNode(e Element) IBNode {
	switch e.IRI() {
	case literalTypeDateTime.IRI():
		return &literalTypeDateTime.ibNode
	case literalTypeDateTimeStamp.IRI():
		return &literalTypeDateTimeStamp.ibNode
	default:
		return nil
	}
}

// apiValue implements Literal.apiValue().
func (s TypedString) apiValue() interface{} {
	return s
}

// Double is a wrapper for the build-in float64 type that implements the Literal interface.
// It represents the xsd:double data type.
type Double float64

var (
	_ Term    = (*Double)(nil)
	_ Literal = (*Double)(nil)
)

// TermKind implements sst.Object.TermKind().
func (Double) TermKind() TermKind {
	return TermKindLiteral
}

// DataType implements Literal.DataType() that returns an internal const IBNode for the Double data type.
// e.g.
// Double.DataType().IRI() will return http://www.w3.org/2001/XMLSchema#double, or in short xsd:double
func (Double) DataType() IBNode {
	return &literalTypeDouble.ibNode
}

// apiValue implements Literal.apiValue().
func (d Double) apiValue() interface{} {
	return d
}

// Float is a wrapper for the build-in float32 type that implements the Literal interface.
// It represents the xsd:float data type.
type Float float32

var (
	_ Term    = (*Float)(nil)
	_ Literal = (*Float)(nil)
)

// TermKind implements sst.Object.TermKind().
func (Float) TermKind() TermKind {
	return TermKindLiteral
}

// DataType implements Literal.DataType() that returns an internal const IBNode for the Float data type.
// e.g.
// Float.DataType().IRI() will return http://www.w3.org/2001/XMLSchema#float, or in short xsd:float
func (Float) DataType() IBNode {
	return &literalTypeFloat.ibNode
}

// apiValue implements Literal.apiValue().
func (f Float) apiValue() interface{} {
	return f
}

// Integer is a wrapper for the build-in int64 type that implements the Literal interface.
// It represents the xsd:integer data type.
type Integer int64

var (
	_ Term    = (*Integer)(nil)
	_ Literal = (*Integer)(nil)
)

// TermKind implements sst.Object.TermKind().
func (Integer) TermKind() TermKind {
	return TermKindLiteral
}

// DataType implements Literal.DataType() that returns an internal const IBNode for the Integer data type.
// e.g.
// Integer.DataType().IRI() will return http://www.w3.org/2001/XMLSchema#integer, or in short xsd:integer
func (Integer) DataType() IBNode {
	return &literalTypeInteger.ibNode
}

// apiValue implements Literal.apiValue().
func (i Integer) apiValue() interface{} {
	return i
}

// Boolean is a wrapper for the build-in bool type that implements the Literal interface.
// It represents the xsd:boolean data type.
type Boolean bool

var (
	_ Term    = (*Boolean)(nil)
	_ Literal = (*Boolean)(nil)
)

// TermKind implements sst.Object.TermKind().
func (Boolean) TermKind() TermKind {
	return TermKindLiteral
}

// DataType implements Literal.DataType() that returns an internal const IBNode for the Boolean data type.
// e.g.
// Boolean.DataType().IRI() will return http://www.w3.org/2001/XMLSchema#boolean, or in short xsd:boolean
// question: can we use in go switch statement:
// case xsd.Boolean
func (Boolean) DataType() IBNode {
	return &literalTypeBoolean.ibNode
}

// apiValue implements Literal.apiValue().
func (i Boolean) apiValue() interface{} {
	return i
}
