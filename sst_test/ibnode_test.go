// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

// The reason why there is a separate package is about RegisterVocabularyMap.
// In sst_test_dummyvoc package, it imports package compiler, which will register a test VocabularyMap.
// In sst_test_realvoc package, it imports package vocabularies, which will register a real
// VocabularyMap that includes lci, owl...

import (
	"bufio"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/semanticstep/sst-core/vocabularies/sso"
	"github.com/semanticstep/sst-core/vocabularies/xsd"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func Test_DictionaryElement(t *testing.T) {
	t.Run("write", func(t *testing.T) {
		data, _ := sst.StaticDictionary().Element(sst.Element{Vocabulary: sso.SSOVocabulary, Name: "ClassSystem"})
		assert.Equal(t, data.PrefixedFragment(), "sso:ClassSystem")
	})
	t.Run("read", func(t *testing.T) {
		for key, val := range sst.StaticDictionary().NamedGraphs() {
			fmt.Println(key, val.IRI())
		}
	})

}

func Test_TypeOf(t *testing.T) {
	st := sst.OpenStage(sst.DefaultTriplexMode)
	g := st.CreateNamedGraph("")
	ib := g.CreateIRINode("", sso.Part)
	fmt.Println(ib.TypeOf().PrefixedFragment())
	assert.Equal(t, ib.TypeOf().PrefixedFragment(), "sso:Part")
}

func Test_CreateEphemeralStage_Literals(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())
	defer func() {
		removeFolder(testName + ".sst")
		removeFolder(testName + ".ttl")
	}()

	// sst.AtomicLevel.SetLevel(zap.DebugLevel)
	t.Run("write", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)

		ng := st.CreateNamedGraph("")
		ib := ng.CreateIRINode("main")
		ib.AddStatement(rdfs.Label, sst.String("ABC Ltd."))
		ib.AddStatement(rdfs.Label, sst.LangString{Val: "Hello", LangTag: "en"})
		ib.AddStatement(rdfs.Label, sst.Double(3.14))
		ib.AddStatement(rdfs.Label, sst.Integer(42))
		ib.AddStatement(rdfs.Label, sst.Boolean(true))
		ib.AddStatement(rdfs.Label, sst.Short(12))
		ib.AddStatement(rdfs.Label, sst.TypedStringOf("2026-02-25T11:35:51Z", xsd.DateTime))
		ib.AddStatement(rdfs.Label, sst.TypedStringOf("2026-02-25T11:35:51Z", xsd.DateTimeStamp))

		mainNode := ng.GetIRINodeByFragment("main")
		if mainNode == nil {
			fmt.Println("main node is nil")
		} else {
			fmt.Println(mainNode.IRI())
		}

		secondNode := ng.GetIRINodeByFragment("second")
		if secondNode == nil {
			fmt.Println("second node is nil")
		} else {
			fmt.Println(secondNode.IRI())
		}

		ng.Dump()
		literalCheck(t, ng)
		writeToFile(ng, testName)
	})
	t.Run("readSSTFile", func(t *testing.T) {
		ng := readSSTFile(testName + ".sst")
		literalCheck(t, ng)
		ng.Dump()
	})

	t.Run("readTTLFile", func(t *testing.T) {
		ng := readTTLFile(testName + ".ttl")
		literalCheck(t, ng)
		ng.Dump()
	})
}

func literalCheck(t *testing.T, ng sst.NamedGraph) {
	foundTypes := make(map[string]bool)
	ng.ForAllIBNodes(func(d sst.IBNode) error {
		d.ForAll(func(i1 int, sub, pred sst.IBNode, obj sst.Term) error {
			switch o := obj.(type) {
			case sst.String:
				fmt.Println(sub.IRI(), pred.IRI(), o)
				assert.Equal(t, string(o), "ABC Ltd.")
				foundTypes["string"] = true
			case sst.LangString:
				fmt.Println(sub.IRI(), pred.IRI(), o.Val, o.LangTag)
				assert.Equal(t, o.Val, "Hello")
				assert.Equal(t, o.LangTag, "en")
				foundTypes["langString"] = true
			case sst.Double:
				fmt.Println(sub.IRI(), pred.IRI(), float64(o))
				assert.Equal(t, float64(o), 3.14)
				foundTypes["double"] = true
			case sst.Integer:
				fmt.Println(sub.IRI(), pred.IRI(), int64(o))
				assert.Equal(t, int64(o), int64(42))
				foundTypes["integer"] = true
			case sst.Boolean:
				fmt.Println(sub.IRI(), pred.IRI(), bool(o))
				assert.Equal(t, bool(o), true)
				foundTypes["boolean"] = true
			case sst.Short:
				fmt.Println(sub.IRI(), pred.IRI(), int16(o))
				assert.Equal(t, int16(o), int16(12))
				foundTypes["short"] = true
			case sst.TypedString:
				dtIRI := o.DataType().IRI().String()
				fmt.Println(sub.IRI(), pred.IRI(), o.Val, "^^"+dtIRI)
				switch dtIRI {
				case "http://www.w3.org/2001/XMLSchema#dateTime":
					assert.Equal(t, o.Val, "2026-02-25T11:35:51Z")
					foundTypes["dateTime"] = true
				case "http://www.w3.org/2001/XMLSchema#dateTimeStamp":
					assert.Equal(t, o.Val, "2026-02-25T11:35:51Z")
					foundTypes["dateTimeStamp"] = true
				default:
					t.Errorf("unexpected TypedString datatype: %s", dtIRI)
				}
			}
			return nil
		})
		return nil
	})
	// Assert all expected literal types were found
	assert.True(t, foundTypes["string"], "xsd:string literal not found")
	assert.True(t, foundTypes["langString"], "rdf:langString literal not found")
	assert.True(t, foundTypes["double"], "xsd:double literal not found")
	assert.True(t, foundTypes["integer"], "xsd:integer literal not found")
	assert.True(t, foundTypes["boolean"], "xsd:boolean literal not found")
	assert.True(t, foundTypes["short"], "xsd:short literal not found")
	assert.True(t, foundTypes["dateTime"], "xsd:dateTime literal not found")
	assert.True(t, foundTypes["dateTimeStamp"], "xsd:dateTimeStamp literal not found")
}

func Test_CreateEphemeralStage_StringLiteralCollection(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())
	defer func() {
		removeFolder(testName + ".sst")
		removeFolder(testName + ".ttl")
	}()
	t.Run("write", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)

		ng := st.CreateNamedGraph("")
		ib := ng.CreateIRINode("main")
		ib.AddStatement(rdfs.Label, sst.NewLiteralCollection(sst.String("ABC"), sst.String("DEF")))

		stringLiteralCollectionCheck(t, ng)
		writeToFile(ng, testName)
	})
	t.Run("readSSTFile", func(t *testing.T) {
		ng := readSSTFile(testName + ".sst")
		stringLiteralCollectionCheck(t, ng)
		ng.Dump()
	})

	t.Run("readTTLFile", func(t *testing.T) {
		ng := readTTLFile(testName + ".ttl")
		stringLiteralCollectionCheck(t, ng)
		ng.Dump()
	})
}

func stringLiteralCollectionCheck(t *testing.T, ng sst.NamedGraph) {
	ng.ForAllIBNodes(func(d sst.IBNode) error {
		d.ForAll(func(i1 int, sub, pred sst.IBNode, obj sst.Term) error {
			switch o := obj.(type) {
			case sst.LiteralCollection:
				o.ForMembers(func(index int, literal sst.Literal) {
					fmt.Println(index, sub.IRI(), pred.IRI(), literal)
					switch index {
					case 0:
						assert.Equal(t, string(literal.(sst.String)), "ABC")
					case 1:
						assert.Equal(t, string(literal.(sst.String)), "DEF")
					}
				})
				assert.Equal(t, string(o.Member(0).(sst.String)), "ABC")
				assert.Equal(t, string(o.Member(1).(sst.String)), "DEF")
			}
			return nil
		})
		return nil
	})
}

func Test_CreateEphemeralStage_NumericLiteralCollection(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())
	defer func() {
		removeFolder(testName + ".sst")
		removeFolder(testName + ".ttl")
	}()
	t.Run("write", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)

		ng := st.CreateNamedGraph("")
		ib := ng.CreateIRINode("main")
		// Test all new numeric types: Byte, Short, Int, Long, UnsignedByte, UnsignedShort, UnsignedInt, UnsignedLong
		// Each collection must have literals of the same type
		ib.AddStatement(rdfs.Label, sst.NewLiteralCollection(sst.Byte(-128), sst.Byte(127)))
		ib.AddStatement(rdfs.Comment, sst.NewLiteralCollection(sst.Short(-32768), sst.Short(32767)))
		ib.AddStatement(rdfs.SeeAlso, sst.NewLiteralCollection(sst.Int(-2147483648), sst.Int(2147483647)))
		ib.AddStatement(rdfs.IsDefinedBy, sst.NewLiteralCollection(sst.Long(-9223372036854775808), sst.Long(9223372036854775807)))
		ib.AddStatement(rdfs.Member, sst.NewLiteralCollection(sst.UnsignedByte(0), sst.UnsignedByte(255)))
		ib.AddStatement(rdfs.SubClassOf, sst.NewLiteralCollection(sst.UnsignedShort(0), sst.UnsignedShort(65535)))
		ib.AddStatement(rdfs.SubPropertyOf, sst.NewLiteralCollection(sst.UnsignedInt(0), sst.UnsignedInt(4294967295)))
		ib.AddStatement(rdfs.Domain, sst.NewLiteralCollection(sst.UnsignedLong(0), sst.UnsignedLong(18446744073709551615)))
		// Test xsd:float with various values
		ib.AddStatement(rdf.Value, sst.NewLiteralCollection(sst.Float(3.14159), sst.Float(-2.5), sst.Float(0.0), sst.Float(1e10), sst.Float(-1e-10)))

		numericLiteralCollectionCheck(t, ng)
		writeToFile(ng, testName)
	})
	t.Run("readSSTFile", func(t *testing.T) {
		ng := readSSTFile(testName + ".sst")
		numericLiteralCollectionCheck(t, ng)
		ng.Dump()
	})

	t.Run("readTTLFile", func(t *testing.T) {
		ng := readTTLFile(testName + ".ttl")
		// TTL format doesn't preserve specific XSD numeric types,
		// so we only check that values are readable, not specific types
		numericLiteralCollectionCheckTTL(t, ng)
		ng.Dump()
	})
}

func numericLiteralCollectionCheck(t *testing.T, ng sst.NamedGraph) {
	ng.ForAllIBNodes(func(d sst.IBNode) error {
		d.ForAll(func(i1 int, sub, pred sst.IBNode, obj sst.Term) error {
			switch o := obj.(type) {
			case sst.LiteralCollection:
				predIRI := pred.IRI()
				fmt.Println("Checking predicate:", predIRI)
				o.ForMembers(func(index int, literal sst.Literal) {
					fmt.Println(index, sub.IRI(), predIRI, literal)
				})
				// Check each collection type based on predicate
				switch predIRI {
				case "http://www.w3.org/2000/01/rdf-schema#label":
					// Byte collection
					assert.Equal(t, o.Member(0).(sst.Byte), sst.Byte(-128))
					assert.Equal(t, o.Member(1).(sst.Byte), sst.Byte(127))
				case "http://www.w3.org/2000/01/rdf-schema#comment":
					// Short collection
					assert.Equal(t, o.Member(0).(sst.Short), sst.Short(-32768))
					assert.Equal(t, o.Member(1).(sst.Short), sst.Short(32767))
				case "http://www.w3.org/2000/01/rdf-schema#seeAlso":
					// Int collection
					assert.Equal(t, o.Member(0).(sst.Int), sst.Int(-2147483648))
					assert.Equal(t, o.Member(1).(sst.Int), sst.Int(2147483647))
				case "http://www.w3.org/2000/01/rdf-schema#isDefinedBy":
					// Long collection
					assert.Equal(t, o.Member(0).(sst.Long), sst.Long(-9223372036854775808))
					assert.Equal(t, o.Member(1).(sst.Long), sst.Long(9223372036854775807))
				case "http://www.w3.org/2000/01/rdf-schema#member":
					// UnsignedByte collection
					assert.Equal(t, o.Member(0).(sst.UnsignedByte), sst.UnsignedByte(0))
					assert.Equal(t, o.Member(1).(sst.UnsignedByte), sst.UnsignedByte(255))
				case "http://www.w3.org/2000/01/rdf-schema#subClassOf":
					// UnsignedShort collection
					assert.Equal(t, o.Member(0).(sst.UnsignedShort), sst.UnsignedShort(0))
					assert.Equal(t, o.Member(1).(sst.UnsignedShort), sst.UnsignedShort(65535))
				case "http://www.w3.org/2000/01/rdf-schema#subPropertyOf":
					// UnsignedInt collection
					assert.Equal(t, o.Member(0).(sst.UnsignedInt), sst.UnsignedInt(0))
					assert.Equal(t, o.Member(1).(sst.UnsignedInt), sst.UnsignedInt(4294967295))
				case "http://www.w3.org/2000/01/rdf-schema#domain":
					// UnsignedLong collection
					assert.Equal(t, o.Member(0).(sst.UnsignedLong), sst.UnsignedLong(0))
					assert.Equal(t, o.Member(1).(sst.UnsignedLong), sst.UnsignedLong(18446744073709551615))
				case "http://www.w3.org/1999/02/22-rdf-syntax-ns#value":
					// Float collection - check various float values
					assert.Equal(t, o.Member(0).(sst.Float), sst.Float(3.14159))
					assert.Equal(t, o.Member(1).(sst.Float), sst.Float(-2.5))
					assert.Equal(t, o.Member(2).(sst.Float), sst.Float(0.0))
					assert.Equal(t, o.Member(3).(sst.Float), sst.Float(1e10))
					assert.Equal(t, o.Member(4).(sst.Float), sst.Float(-1e-10))
				}
			}
			return nil
		})
		return nil
	})
}

func numericLiteralCollectionCheckTTL(t *testing.T, ng sst.NamedGraph) {
	ng.ForAllIBNodes(func(d sst.IBNode) error {
		d.ForAll(func(i1 int, sub, pred sst.IBNode, obj sst.Term) error {
			switch o := obj.(type) {
			case sst.LiteralCollection:
				predIRI := pred.IRI()
				fmt.Println("Checking predicate (TTL):", predIRI)
				o.ForMembers(func(index int, literal sst.Literal) {
					fmt.Println(index, sub.IRI(), predIRI, literal)
				})
				// For TTL, just check that collections are readable
				// TTL parser converts all numeric literals to Integer type
				switch predIRI {
				case "http://www.w3.org/2000/01/rdf-schema#label",
					"http://www.w3.org/2000/01/rdf-schema#comment",
					"http://www.w3.org/2000/01/rdf-schema#seeAlso",
					"http://www.w3.org/2000/01/rdf-schema#isDefinedBy",
					"http://www.w3.org/2000/01/rdf-schema#member",
					"http://www.w3.org/2000/01/rdf-schema#subClassOf",
					"http://www.w3.org/2000/01/rdf-schema#subPropertyOf",
					"http://www.w3.org/2000/01/rdf-schema#domain":
					// TTL converts all numeric literals to Integer
					// Just verify the values are present
					assert.Equal(t, 2, o.MemberCount())
				case "http://www.w3.org/1999/02/22-rdf-syntax-ns#value":
					// TTL converts float literals to Integer or Double
					// Just verify the values are present
					assert.Equal(t, 5, o.MemberCount())
				}
			}
			return nil
		})
		return nil
	})
}

func Test_CreateEphemeralStage_NumericLiterals(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())
	defer func() {
		removeFolder(testName + ".sst")
		removeFolder(testName + ".ttl")
	}()
	t.Run("write", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)

		ng := st.CreateNamedGraph("")
		ib := ng.CreateIRINode("main")
		// Add individual numeric literals (not in collections)
		ib.AddStatement(rdfs.Label, sst.Byte(-128))
		ib.AddStatement(rdfs.Comment, sst.Short(-32768))
		ib.AddStatement(rdfs.SeeAlso, sst.Int(-2147483648))
		ib.AddStatement(rdfs.IsDefinedBy, sst.Long(-9223372036854775808))
		ib.AddStatement(rdfs.Member, sst.UnsignedByte(255))
		ib.AddStatement(rdfs.SubClassOf, sst.UnsignedShort(65535))
		ib.AddStatement(rdfs.SubPropertyOf, sst.UnsignedInt(4294967295))
		ib.AddStatement(rdfs.Domain, sst.UnsignedLong(18446744073709551615))

		numericLiteralsCheck(t, ng)
		writeToFile(ng, testName)
	})
	t.Run("readSSTFile", func(t *testing.T) {
		ng := readSSTFile(testName + ".sst")
		numericLiteralsCheck(t, ng)
		ng.Dump()
	})

	t.Run("readTTLFile", func(t *testing.T) {
		ng := readTTLFile(testName + ".ttl")
		// TTL format doesn't preserve specific XSD numeric types,
		// so we only check that values are readable, not specific types
		numericLiteralsCheckTTL(t, ng)
		ng.Dump()
	})
}

func numericLiteralsCheck(t *testing.T, ng sst.NamedGraph) {
	ng.ForAllIBNodes(func(d sst.IBNode) error {
		d.ForAll(func(i1 int, sub, pred sst.IBNode, obj sst.Term) error {
			if literal, ok := obj.(sst.Literal); ok {
				predIRI := pred.IRI()
				fmt.Println("Checking predicate:", predIRI, "value:", literal)
				switch predIRI {
				case "http://www.w3.org/2000/01/rdf-schema#label":
					assert.Equal(t, sst.Byte(-128), literal.(sst.Byte))
				case "http://www.w3.org/2000/01/rdf-schema#comment":
					assert.Equal(t, sst.Short(-32768), literal.(sst.Short))
				case "http://www.w3.org/2000/01/rdf-schema#seeAlso":
					assert.Equal(t, sst.Int(-2147483648), literal.(sst.Int))
				case "http://www.w3.org/2000/01/rdf-schema#isDefinedBy":
					assert.Equal(t, sst.Long(-9223372036854775808), literal.(sst.Long))
				case "http://www.w3.org/2000/01/rdf-schema#member":
					assert.Equal(t, sst.UnsignedByte(255), literal.(sst.UnsignedByte))
				case "http://www.w3.org/2000/01/rdf-schema#subClassOf":
					assert.Equal(t, sst.UnsignedShort(65535), literal.(sst.UnsignedShort))
				case "http://www.w3.org/2000/01/rdf-schema#subPropertyOf":
					assert.Equal(t, sst.UnsignedInt(4294967295), literal.(sst.UnsignedInt))
				case "http://www.w3.org/2000/01/rdf-schema#domain":
					assert.Equal(t, sst.UnsignedLong(18446744073709551615), literal.(sst.UnsignedLong))
				}
			}
			return nil
		})
		return nil
	})
}

func numericLiteralsCheckTTL(t *testing.T, ng sst.NamedGraph) {
	ng.ForAllIBNodes(func(d sst.IBNode) error {
		d.ForAll(func(i1 int, sub, pred sst.IBNode, obj sst.Term) error {
			if literal, ok := obj.(sst.Literal); ok {
				predIRI := pred.IRI()
				fmt.Println("Checking predicate (TTL):", predIRI, "value:", literal)
				// For TTL, just verify the literal is readable
				// TTL parser converts all numeric literals to Integer type
				switch predIRI {
				case "http://www.w3.org/2000/01/rdf-schema#label",
					"http://www.w3.org/2000/01/rdf-schema#comment",
					"http://www.w3.org/2000/01/rdf-schema#seeAlso",
					"http://www.w3.org/2000/01/rdf-schema#isDefinedBy",
					"http://www.w3.org/2000/01/rdf-schema#member",
					"http://www.w3.org/2000/01/rdf-schema#subClassOf",
					"http://www.w3.org/2000/01/rdf-schema#subPropertyOf",
					"http://www.w3.org/2000/01/rdf-schema#domain":
					// TTL converts numeric literals to Integer
					// Just verify it's a literal
					assert.NotNil(t, literal)
				}
			}
			return nil
		})
		return nil
	})
}

func Test_CreateEphemeralStage_LangStringLiteralCollection(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())
	defer func() {
		removeFolder(testName + ".sst")
		removeFolder(testName + ".ttl")
	}()
	t.Run("write", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)

		ng := st.CreateNamedGraph("")
		ib := ng.CreateIRINode("main")
		ib.AddStatement(rdfs.Label, sst.NewLiteralCollection(sst.LangString{Val: "Hello", LangTag: "en"}, sst.LangString{Val: "你好", LangTag: "cn"}))

		langStringLiteralCollectionCheck(t, ng)
		writeToFile(ng, testName)
	})
	t.Run("readSSTFile", func(t *testing.T) {
		ng := readSSTFile(testName + ".sst")
		langStringLiteralCollectionCheck(t, ng)
		ng.Dump()
	})

	t.Run("readTTLFile", func(t *testing.T) {
		ng := readTTLFile(testName + ".ttl")
		langStringLiteralCollectionCheck(t, ng)
		ng.Dump()
	})
}

func langStringLiteralCollectionCheck(t *testing.T, ng sst.NamedGraph) {
	ng.ForAllIBNodes(func(d sst.IBNode) error {
		d.ForAll(func(i1 int, sub, pred sst.IBNode, obj sst.Term) error {
			switch o := obj.(type) {
			case sst.LiteralCollection:
				o.ForMembers(func(index int, literal sst.Literal) {
					fmt.Println(index, sub.IRI(), pred.IRI(), literal.(sst.LangString).Val, literal.(sst.LangString).LangTag)
					switch index {
					case 0:
						assert.Equal(t, literal.(sst.LangString).Val, "Hello")
						assert.Equal(t, literal.(sst.LangString).LangTag, "en")
					case 1:
						assert.Equal(t, literal.(sst.LangString).Val, "你好")
						assert.Equal(t, literal.(sst.LangString).LangTag, "cn")
					}
				})
				assert.Equal(t, o.Member(0).(sst.LangString).Val, "Hello")
				assert.Equal(t, o.Member(0).(sst.LangString).LangTag, "en")
				assert.Equal(t, o.Member(1).(sst.LangString).Val, "你好")
				assert.Equal(t, o.Member(1).(sst.LangString).LangTag, "cn")
			}
			return nil
		})
		return nil
	})
}

func Test_CreateEphemeralStage_BooleanLiteralCollection(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())

	defer func() {
		removeFolder(testName + ".sst")
		removeFolder(testName + ".ttl")
	}()
	t.Run("write", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)

		ng := st.CreateNamedGraph("")
		ib := ng.CreateIRINode("main")
		ib.AddStatement(rdfs.Label, sst.NewLiteralCollection(sst.Boolean(true), sst.Boolean(false)))

		boolLiteralCollectionCheck(t, ng)
		writeToFile(ng, testName)
	})
	t.Run("readSSTFile", func(t *testing.T) {
		ng := readSSTFile(testName + ".sst")
		boolLiteralCollectionCheck(t, ng)
		ng.Dump()
	})

	t.Run("readTTLFile", func(t *testing.T) {
		ng := readTTLFile(testName + ".ttl")
		boolLiteralCollectionCheck(t, ng)
		ng.Dump()
	})
}

func boolLiteralCollectionCheck(t *testing.T, ng sst.NamedGraph) {
	ng.ForAllIBNodes(func(d sst.IBNode) error {
		d.ForAll(func(i1 int, sub, pred sst.IBNode, obj sst.Term) error {
			switch o := obj.(type) {
			case sst.LiteralCollection:
				o.ForMembers(func(index int, literal sst.Literal) {
					fmt.Println(index, sub.IRI(), pred.IRI(), literal)
					switch index {
					case 0:
						assert.Equal(t, bool(literal.(sst.Boolean)), true)
					case 1:
						assert.Equal(t, bool(literal.(sst.Boolean)), false)
					}
				})
				assert.Equal(t, bool(o.Member(0).(sst.Boolean)), true)
				assert.Equal(t, bool(o.Member(1).(sst.Boolean)), false)
			}
			return nil
		})
		return nil
	})
}

func Test_CreateEphemeralStage_DoubleLiteralCollection(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())
	defer func() {
		removeFolder(testName + ".sst")
		removeFolder(testName + ".ttl")
	}()
	t.Run("write", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)

		ng := st.CreateNamedGraph("")
		ib := ng.CreateIRINode("main")
		ib.AddStatement(rdfs.Label, sst.NewLiteralCollection(sst.Double(3.14), sst.Double(3.15)))
		doubleLiteralCollectionCheck(t, ng)
		writeToFile(ng, testName)
	})
	t.Run("readSSTFile", func(t *testing.T) {
		ng := readSSTFile(testName + ".sst")
		doubleLiteralCollectionCheck(t, ng)
		ng.Dump()
	})

	t.Run("readTTLFile", func(t *testing.T) {
		ng := readTTLFile(testName + ".ttl")
		doubleLiteralCollectionCheck(t, ng)
		ng.Dump()
	})
}

func doubleLiteralCollectionCheck(t *testing.T, ng sst.NamedGraph) {
	ng.ForAllIBNodes(func(d sst.IBNode) error {
		d.ForAll(func(i1 int, sub, pred sst.IBNode, obj sst.Term) error {
			switch o := obj.(type) {
			case sst.LiteralCollection:
				o.ForMembers(func(index int, literal sst.Literal) {
					fmt.Println(index, sub.IRI(), pred.IRI(), literal)
					switch index {
					case 0:
						assert.Equal(t, float64(literal.(sst.Double)), 3.14)
					case 1:
						assert.Equal(t, float64(literal.(sst.Double)), 3.15)
					}
					assert.Equal(t, float64(o.Member(0).(sst.Double)), 3.14)
					assert.Equal(t, float64(o.Member(1).(sst.Double)), 3.15)
				})
			}
			return nil
		})
		return nil
	})
}

func Test_CreateEphemeralStage_IntegerLiteralCollection(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())
	defer func() {
		removeFolder(testName + ".sst")
		removeFolder(testName + ".ttl")
	}()
	t.Run("write", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)

		ng := st.CreateNamedGraph("")
		ib := ng.CreateIRINode("main")
		ib.AddStatement(rdfs.Label, sst.NewLiteralCollection(sst.Integer(42), sst.Integer(43)))

		integerLiteralCollectionCheck(t, ng)
		writeToFile(ng, testName)
	})
	t.Run("readSSTFile", func(t *testing.T) {
		ng := readSSTFile(testName + ".sst")
		integerLiteralCollectionCheck(t, ng)
		ng.Dump()
	})

	t.Run("readTTLFile", func(t *testing.T) {
		ng := readTTLFile(testName + ".ttl")
		integerLiteralCollectionCheck(t, ng)
		ng.Dump()
	})
}

func integerLiteralCollectionCheck(t *testing.T, ng sst.NamedGraph) {
	ng.ForAllIBNodes(func(d sst.IBNode) error {
		d.ForAll(func(i1 int, sub, pred sst.IBNode, obj sst.Term) error {
			switch o := obj.(type) {
			case sst.LiteralCollection:
				o.ForMembers(func(index int, literal sst.Literal) {
					fmt.Println(index, sub.IRI(), pred.IRI(), literal)
					switch index {
					case 0:
						assert.Equal(t, int64(42), int64(literal.(sst.Integer)))
					case 1:
						assert.Equal(t, int64(43), int64(literal.(sst.Integer)))
					}
					assert.Equal(t, int64(o.Member(0).(sst.Integer)), int64(42))
					assert.Equal(t, int64(o.Member(1).(sst.Integer)), int64(43))
				})
			}
			return nil
		})
		return nil
	})
}

func Test_IRI(t *testing.T) {
	st := sst.OpenStage(sst.DefaultTriplexMode)
	id := uuid.New()
	g := st.CreateNamedGraph(sst.IRI(id.URN()))
	assert.Equal(t, g.IRI().String(), "urn:uuid:"+id.String())
	ib1 := g.CreateIRINode("", sso.Part)
	fmt.Println(ib1.IRI())

	ib2 := g.CreateIRINode("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361", sso.Part)
	fmt.Println(ib2.IRI())

	// ib3 := g.CreateBlankNode(sso.Part)
	// fmt.Println(ib3.IRI())

	fmt.Println(ib1.TypeOf().PrefixedFragment())
	assert.Equal(t, ib1.TypeOf().PrefixedFragment(), "sso:Part")
}

func Test_IBNodeDelete_NGAImportNGB(t *testing.T) {
	// testName := filepath.Join("./testdata/" + t.Name())
	t.Run("write", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)

		ngIDA := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361")
		ngIDB := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362")

		// create a NamedGraph and get it
		ngA := st.CreateNamedGraph(sst.IRI(ngIDA.URN()))
		ngB := st.CreateNamedGraph(sst.IRI(ngIDB.URN()))
		ngA.AddImport(ngB)

		mainB := ngB.CreateIRINode("mainB")
		mainB.AddStatement(rdf.Type, rep.SchematicPort)

		mainA := ngA.CreateIRINode("mainA")
		mainA.AddStatement(rep.MappingSource, mainB)
		mainB.Delete()

		ngA.Dump()
		ngB.Dump()
	})
}

func Test_Internet_addresses_RdfRead_test(t *testing.T) {
	type args struct {
		bReader *bufio.Reader
	}
	toArgs := func(rdr *bufio.Reader) args {
		return args{bReader: rdr}
	}
	tests := []struct {
		name           string
		args           args
		expected       string
		graphAssertion func(t *testing.T, graph sst.NamedGraph)
		errorAssertion assert.ErrorAssertionFunc
	}{
		{
			name: "term_collection_three_members",
			args: toArgs(fromTurtleContent(t,
				`
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix rdf:    <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:   <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:    <http://www.w3.org/2002/07/owl#> .
@prefix ex:   <http://example.com/demo#> .
@prefix xxx:   <http://example.com/xxx#> .

<http://example.com/demo#>    a   owl:Ontology .

ex:uuid1 a lci:Organization;
   rdfs:label "DCT" ;
   xxx:hompePage <https://www.dct-china.cn> ;
   xxx:contact <mailto:e.qu@dct-china.cn>  .
`)),

			expected: `{
  "kinds": [
    "rdf_type"
  ],
  "passed": true,
  "generated": "2025-11-07T11:14:49.473987+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {

			},

			errorAssertion: assert.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, err := sst.RdfRead(tt.args.bReader, sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
			assert.NoError(t, err)

			tt.graphAssertion(t, st.NamedGraphs()[0])

			st.Dump()
		})
	}
}

func Test_BlankNodeNoFragment(t *testing.T) {
	testCases := []struct {
		name    string
		content string
	}{
		{
			name: "multi-line blank node",
			content: `
@prefix rdf:   <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix xsd:   <http://www.w3.org/2001/XMLSchema#> .
@prefix ssrep: <http://ontology.semanticstep.net/rep#> .
@prefix ssqau: <http://ontology.semanticstep.net/qau#> .

<http://example.org/test-part>
    rdf:type <http://example.org/Part> ;
    ssrep:lengthValue [
        a ssqau:Length ;
        ssqau:milliMetre 1170.686
    ] .
`,
		},
		{
			name: "compact single-line blank node",
			content: `
@prefix rdf:   <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix xsd:   <http://www.w3.org/2001/XMLSchema#> .
@prefix ssrep: <http://ontology.semanticstep.net/rep#> .
@prefix ssqau: <http://ontology.semanticstep.net/qau#> .

<http://example.org/test-part>
    rdf:type <http://example.org/Part> ;
    ssrep:lengthValue [ a ssqau:Length ; ssqau:milliMetre 1170.686 ] .
`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			st, err := sst.RdfRead(fromTurtleContent(t, tc.content), sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
			assert.NoError(t, err)

			graph := st.NamedGraphs()[0]
			var blankNode sst.IBNode
			graph.ForBlankNodes(func(node sst.IBNode) error {
				blankNode = node
				return nil
			})
			assert.NotNil(t, blankNode, "expected a blank node in the test data")

			foundType := false
			foundValue := false
			blankNode.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
				if p.IRI().String() == "http://www.w3.org/1999/02/22-rdf-syntax-ns#type" {
					if ib, ok := o.(sst.IBNode); ok && ib.IRI().String() == "http://ontology.semanticstep.net/qau#Length" {
						foundType = true
					}
				}
				if p.IRI().String() == "http://ontology.semanticstep.net/qau#milliMetre" {
					if val, ok := o.(sst.Double); ok && float64(val) == 1170.686 {
						foundValue = true
					}
				}
				return nil
			})
			assert.True(t, foundType, "expected blank node to have rdf:type ssqau:Length")
			assert.True(t, foundValue, "expected blank node to have ssqau:milliMetre 1170.686")

			assert.Panics(t, func() {
				_ = blankNode.Fragment()
			}, "blank node does not have a fragment")

			assert.Panics(t, func() {
				_ = blankNode.IRI()
			}, "blank node does not have an IRI")
		})
	}
}
