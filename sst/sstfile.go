// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// SST File Structure
// ├── [8? bytes]  Magic Header "SST...."
// ├── [varint+str] Graph URI
// │
// ├── IMPORTS
// │   ├── [varint] Import Count
// │   └── For each import:
// │       └── [varint+str] Import Graph Base IRI
// │
// ├── REFERENCES
// │   ├── [varint] Reference Count
// │   └── For each reference:
// │       └── [varint+str] Reference Graph IRI
// │
// ├── DICTIONARY
// │   ├── [varint] IRI Node Count
// │   ├── [varint] Blank Node Count
// │   └── For each node (sorted):
// │       ├── [varint+str] Fragment (blank="" )
// │       └── [varint] Total Reference Count (tc)
// │
// │   ├── For each import graph (sorted by UUID):
// │   │   ├── [varint] Ref Node Count
// │   │   └── For each node (sorted by fragment):
// │   │       ├── [varint+str] Fragment
// │   │       └── [varint] Reference Count
// │   │
// │   └── For each reference graph (sorted by IRI):
// │       └── (same structure as import)
// │
// └── CONTENT
//     └── For each node (sorted):
//         ├── [varint] Non-Literal Triple Count
//         ├── Non-Literal Triples[]
//         │   └── [varint] Pred Index, [varint] Obj Index
//         │
//         ├── [varint] Literal Triple Count
//         └── Literal Triples[]
//             ├── [varint] Pred Index
//             └── Literal Data (type-specific encoding)
//                 or [varint] Pred Index, [varint] Collection Flag,
//                    [varint] Member Count, Literal Data[]

// defaultEncoderConfig returns the default JSON encoder configuration for the sst package.
func defaultEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		TimeKey:        "t",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
}

// GlobalLogger defaults to a no-op logger and is used for all internal logging.
// Callers can inject a custom logger via SetLogger or enable the default
// stdout JSON logger via EnableDefaultLogger.
var GlobalLogger *zap.Logger = zap.NewNop()

// AtomicLevel controls the log level for the default logger. It can also be
// used as a reference when adjusting logging verbosity at runtime.
var AtomicLevel = zap.NewAtomicLevel()

// SetLogger allows the caller (cli, server, or consumer application) to
// inject a custom logger. Example: sst.SetLogger(zap.NewProduction()).
func SetLogger(l *zap.Logger) {
	if l != nil {
		GlobalLogger = l
	}
}

// EnableDefaultLogger explicitly enables the sst package default stdout JSON
// logger. The default level is Info and can be changed at runtime through
// AtomicLevel.SetLevel.
func EnableDefaultLogger() {
	encoder := zapcore.NewJSONEncoder(defaultEncoderConfig())
	core := zapcore.NewCore(encoder, os.Stdout, AtomicLevel)
	GlobalLogger = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zap.ErrorLevel))
}

// SetLogLevel is a convenience helper to change the AtomicLevel.
func SetLogLevel(l zapcore.Level) {
	AtomicLevel.SetLevel(l)
}

var sstFileHeader = [8]byte{'S', 'S', 'T', '-', '1', '.', '0', 0}

type writtenLiteralKind uint8

const (
	// 0: xsd:string
	// Public API: sst.String (alias for string)
	// Internal storage: string (raw string value)
	writtenLiteralString = writtenLiteralKind(iota)

	// 1: rdf:langString
	// Public API: sst.LangString struct { Val string; LangTag string }
	// Internal storage: string (value + 2-byte language tag suffix, no separator)
	writtenLiteralLangString

	// 2: xsd:boolean
	// Public API: sst.Boolean (alias for bool)
	// Internal storage: string ("t" for true, empty string for false)
	writtenLiteralBoolean

	// 3: xsd:decimal (TBD - currently uses string representation)
	// Public API: sst.String
	// Internal storage: string (arbitrary-precision decimal number)
	_

	// 4: xsd:integer
	// Public API: sst.Integer (alias for int64)
	// Internal storage: string (arbitrary-size integer number as string)
	writtenLiteralInteger

	// 5: xsd:double (64-bit IEEE 754 floating point)
	// Public API: sst.Double (alias for float64)
	// Internal storage: 8 bytes (big-endian IEEE 754 binary64)
	writtenLiteralDouble

	// 6: xsd:float (32-bit IEEE 754 floating point)
	// Public API: sst.Float (alias for float32)
	// Internal storage: 4 bytes (big-endian IEEE 754 binary32), supports ±Inf, ±0, NaN
	writtenLiteralFloat

	// 7: xsd:date (SKIPPED - not supported)
	// Would be: string storage for date (yyyy-mm-dd) with/without timezone
	_

	// 8: xsd:time (SKIPPED - not supported)
	// Would be: string storage for time (hh:mm:ss.sss…) with/without timezone
	_

	// 9: xsd:dateTime
	// Public API: sst.TypedString { Val string; Type Node }
	// Internal storage: string (date and time with/without timezone)
	writtenLiteralDateTime

	// 10: xsd:dateTimeStamp
	// Public API: sst.TypedString { Val string; Type Node }
	// Internal storage: string (date and time with required timezone)
	writtenLiteralDateTimeStamp

	// 11-15: Gregorian calendar types (SKIPPED - not supported)
	_ // xsd:gYear
	_ // xsd:gMonth
	_ // xsd:gDay
	_ // xsd:gYearMonth
	_ // xsd:gMonthDay

	// 16-18: Duration types (SKIPPED - not supported)
	_ // xsd:duration
	_ // xsd:yearMonthDuration
	_ // xsd:dayTimeDuration

	// 19: xsd:byte (signed 8-bit integer)
	// Public API: sst.Byte (alias for int8)
	// Internal storage: 1 byte (raw byte value)
	writtenLiteralByte

	// 20: xsd:short (signed 16-bit integer)
	// Public API: sst.Short (alias for int16)
	// Internal storage: 2 bytes (big-endian int16)
	writtenLiteralShort

	// 21: xsd:int (signed 32-bit integer)
	// Public API: sst.Int (alias for int32)
	// Internal storage: 4 bytes (big-endian int32)
	writtenLiteralInt

	// 22: xsd:long (signed 64-bit integer)
	// Public API: sst.Long (alias for int64)
	// Internal storage: 8 bytes (big-endian int64)
	writtenLiteralLong

	// 23: xsd:unsignedByte (unsigned 8-bit integer)
	// Public API: sst.UnsignedByte (alias for uint8)
	// Internal storage: 1 byte (raw byte value)
	writtenLiteralUnsignedByte

	// 24: xsd:unsignedShort (unsigned 16-bit integer)
	// Public API: sst.UnsignedShort (alias for uint16)
	// Internal storage: 2 bytes (big-endian uint16)
	writtenLiteralUnsignedShort

	// 25: xsd:unsignedInt (unsigned 32-bit integer)
	// Public API: sst.UnsignedInt (alias for uint32)
	// Internal storage: 4 bytes (big-endian uint32)
	writtenLiteralUnsignedInt

	// 26: xsd:unsignedLong (unsigned 64-bit integer)
	// Public API: sst.UnsignedLong (alias for uint64)
	// Internal storage: 8 bytes (big-endian uint64)
	writtenLiteralUnsignedLong

	// 27-30: Derived integer types (SKIPPED - not supported, use xsd:integer instead)
	_ // xsd:positiveInteger (>0)
	_ // xsd:nonNegativeInteger (≥0)
	_ // xsd:negativeInteger (<0)
	_ // xsd:nonPositiveInteger (≤0)

	// 31-32: Binary types (SKIPPED - not supported)
	_ // xsd:hexBinary
	_ // xsd:base64Binary

	// 127: rdf:List (LiteralCollection)
	// Public API: sst.LiteralCollection interface
	// Internal storage: Special format - number of members followed by encoded literals
	writtenLiteralCollection = writtenLiteralKind(127)
)

func literalKindUintToString(k writtenLiteralKind) string {
	switch k {
	case writtenLiteralString:
		return "String"
	case writtenLiteralLangString:
		return "LangString"
	case writtenLiteralBoolean:
		return "Bool"
	case writtenLiteralInteger:
		return "Integer"
	case writtenLiteralDouble:
		return "Double"
	case writtenLiteralFloat:
		return "Float"
	case writtenLiteralByte:
		return "Byte"
	case writtenLiteralShort:
		return "Short"
	case writtenLiteralInt:
		return "Int"
	case writtenLiteralLong:
		return "Long"
	case writtenLiteralUnsignedByte:
		return "UnsignedByte"
	case writtenLiteralUnsignedShort:
		return "UnsignedShort"
	case writtenLiteralUnsignedInt:
		return "UnsignedInt"
	case writtenLiteralUnsignedLong:
		return "UnsignedLong"
	case writtenLiteralDateTime:
		return "DateTime"
	case writtenLiteralDateTimeStamp:
		return "DateTimeStamp"
	case writtenLiteralCollection:
		return "Literal Collection"
	default:
		GlobalLogger.Error("Literal Kind not recognized", zap.Uint8("kind", uint8(k)))
	}
	GlobalLogger.Panic("literalKindUintToString error")
	return ""
}
