// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRead(t *testing.T) {
	type args struct {
		r *bufio.Reader
	}
	tests := []struct {
		name           string
		args           args
		errAssertion   assert.ErrorAssertionFunc
		graphAssertion func(*testing.T, sst.NamedGraph)
	}{
		{
			name: "emptyDefaultGraph",
			args: args{
				r: bb("SST-1.0\x00\x2durn:uuid:e38c8811-1028-4442-af79-2cce833feb4e\x00\x00\x00\x00"),
			},
			errAssertion: assert.NoError,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
				assert.Equal(t, "urn:uuid:e38c8811-1028-4442-af79-2cce833feb4e", graph.IRI().String())
			},
		},
		{
			name: "emptyURIGraph",
			args: args{
				r: bb("SST-1.0\x00\x2bhttp://www.w3.org/1999/02/22-rdf-syntax-ns#\x00\x00\x00\x00"),
			},
			errAssertion: assert.NoError,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
				assert.Equal(t, "http://www.w3.org/1999/02/22-rdf-syntax-ns", graph.IRI().String())
			},
		},
		{
			name: "defaultGraphWithNodesNoImports",
			args: args{
				r: bb("SST-1.0\x00\x2durn:uuid:546a46db-4273-4a6c-8031-1726e869b3d2\x00\x01" +
					"\x1fhttp://semanticstep.net/schema#\x02\x01\x02s1\x01\x02s2\x02\x01\x02\x02p1\x01\x02p2\x01" +
					"\x01\x03\x01\x00\x01\x04\x02\x00\x00\x00"),
			},
			errAssertion: assert.NoError,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
				assert.Equal(t, "urn:uuid:546a46db-4273-4a6c-8031-1726e869b3d2", graph.IRI().String())
			},
		},
		{
			name: "defaultWithNodesAndImport",
			args: args{
				r: bb("SST-1.0\x00\x2durn:uuid:39c59163-2526-4fc4-8c56-644d1b3b81d4" +
					"\x01\x2durn:uuid:91bedd7d-8f59-48e8-98fa-c06ec328e4a1\x01\x1fhttp://semanticstep.net/schema#" +
					"\x02\x01\x02s1\x01\x02s2\x02\x01\x00\x02\x02p1\x01\x02p2\x01" +
					"\x01\x03\x01\x00\x01\x04\x02\x00\x00\x00"),
			},
			errAssertion: assert.NoError,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
				assert.Equal(t, 3, graph.IRINodeCount())
				// assert.Len(t, graph.Imports().Direct(), 1)
			},
		},
		{
			name: "defaultWithNodesAndImports",
			args: args{
				r: bb("SST-1.0\x00\x2durn:uuid:39c59163-2526-4fc4-8c56-644d1b3b81d4" +
					"\x02\x2durn:uuid:91bedd7d-8f59-48e8-98fa-c06ec328e4a1" +
					"\x2durn:uuid:c9137479-b4bf-4870-8e16-be6936513d58\x01\x1fhttp://semanticstep.net/schema#" +
					"\x02\x01\x02s1\x01\x02s2\x02\x01\x00\x00\x02\x02p1\x01\x02p2\x01" +
					"\x01\x03\x01\x00\x01\x04\x02\x00\x00\x00"),
			},
			errAssertion: assert.NoError,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
				assert.Equal(t, 3, graph.IRINodeCount())
				assert.Len(t, graph.DirectImports(), 2)
			},
		},
		{
			name: "uriGraphWithImport",
			args: args{
				r: bb("SST-1.0\x00\x1ehttp://www.w3.org/2002/07/owl#" +
					"\x01\x25http://www.w3.org/2000/01/rdf-schema#\x00\x00\x00\x00"),
			},
			errAssertion: assert.NoError,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
				assert.Equal(t, "http://www.w3.org/2002/07/owl", graph.IRI().String())
				assert.Equal(t, 1, graph.IRINodeCount())
				if assert.Len(t, graph.DirectImports(), 1) {
					assert.Equal(
						t,
						"http://www.w3.org/2000/01/rdf-schema",
						graph.DirectImports()[0].IRI().String(),
					)
				}
			},
		},
		{
			name: "defaultAndImportedGraphs",
			args: args{
				r: bb("SST-1.0\x00" +
					"\x2durn:uuid:43e199ee-ac39-46c6-852f-50704fdccaef" +
					"\x02\x2durn:uuid:5184e8b3-0649-493d-8b61-a2a2b42c4f24\x2durn:uuid:d7bb18e3-b830-42dc-97cc-3f3a14317caf" +
					"\x01\x1fhttp://semanticstep.net/schema#" +
					"\x02\x00\x03sA1\x03\x03sA2\x01\x01\x03sB1\x01\x01\x03sC1\x01" +
					"\x04\x02p1\x02\x02p2\x02\x02t1\x01\x02t2\x01" +
					"\x03\x04\x06\x05\x02\x05\x03\x00\x01\x04\x07\x00"),
			},
			errAssertion: assert.NoError,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
				fragments := map[string]struct{}{}
				err := graph.ForIRINodes(func(t sst.IBNode) error {
					if t.Fragment() != "" {
						fragments[t.Fragment()] = struct{}{}
					}
					return nil
				})
				assert.NoError(t, err)
				assert.Equal(t, map[string]struct{}{
					"sA1": {},
					"sA2": {},
				}, fragments)
				assert.Equal(t, "urn:uuid:43e199ee-ac39-46c6-852f-50704fdccaef", graph.IRI().String())
			},
		},
		{
			name: "graphWithLiterals",
			args: args{
				r: bb("SST-1.0\x00-urn:uuid:d270203d-2598-4a71-80b1-50576a1fda84\x00\x01" +
					"\x1ehttp://semanticstep.net/schema\x02\x00\x00\x00\x02s1\x03\x03\x02p1\x01\x02p2\x01\x02p3\x01\x00" +
					"\x00\x00\x03\x02\x00\x04str1\x03\x04\x85\xd2\xa9\x9a!\x04\x05?\xf3333333"),
			},
			errAssertion: assert.NoError,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
				values := map[interface{}]struct{}{}
				err := graph.ForIRINodes(func(s sst.IBNode) error {
					if s.Fragment() == "s1" {
						assert.Equal(t, "s1", string(s.Fragment()))
						err := s.ForAll(func(_ int, _, p sst.IBNode, o sst.Term) error {
							values[o.(sst.Literal)] = struct{}{}
							return nil
						})
						assert.NoError(t, err)
					}
					return nil
				})
				assert.NoError(t, err)
				assert.Equal(t, map[interface{}]struct{}{
					sst.Double(1.2):          {},
					sst.Integer(-4456789123): {},
					sst.String("str1"):       {},
				}, values)
				assert.Equal(t, "urn:uuid:d270203d-2598-4a71-80b1-50576a1fda84", graph.IRI().String())
			},
		},
		{
			name: "graphWithLiteralAndLiteralCollection",
			args: args{
				r: bb("SST-1.0\x00\x2durn:uuid:2a49b2d9-ddbd-49bc-ba58-2fbea7cf6792\x00\x01" +
					"\x1fhttp://semanticstep.net/schema#\x01\x00\x02s1\x02\x02\x02p1\x01\x02p2\x01\x00" +
					"\x02\x01\x04\x14\x02\x7f\x02\x00\x03cl1\x00\x03cl2"),
			},
			errAssertion: assert.NoError,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
				values := map[interface{}]struct{}{}
				err := graph.ForIRINodes(func(s sst.IBNode) error {
					if s.Fragment() == "s1" {
						assert.Equal(t, "s1", string(s.Fragment()))
						err := s.ForAll(func(_ int, _, p sst.IBNode, o sst.Term) error {
							switch o.TermKind() {
							case sst.TermKindLiteral:
								values[o.(sst.Literal)] = struct{}{}
							case sst.TermKindLiteralCollection:
								o.(sst.LiteralCollection).ForMembers(func(_ int, l sst.Literal) {
									values[l] = struct{}{}
								})
							default:
								assert.Fail(t, "unexpected kind")
							}
							return nil
						})
						assert.NoError(t, err)
					}
					return nil
				})
				assert.NoError(t, err)
				assert.Equal(t, map[interface{}]struct{}{
					sst.Integer(10):   {},
					sst.String("cl1"): {},
					sst.String("cl2"): {},
				}, values)
				assert.Equal(t, "urn:uuid:2a49b2d9-ddbd-49bc-ba58-2fbea7cf6792", graph.IRI().String())
			},
		},
		{
			name: "readGraphWithTripleLessTermCollection",
			args: args{
				r: bb("SST-1.0\x00-urn:uuid:0c20d026-3fda-4e35-ab7d-88ba6cc0c5fc\x00\x02" +
					"\x1fhttp://semanticstep.net/schema#*http://www.w3.org/1999/02/22-rdf-syntax-ns\x02\x01\x02s1\x01\x02s2\x01\x02\x01\x02p1\x01\x01" +
					"\x05first\x01\x01\x03\x02\x00\x00\x00\x01\x04\x01\x00"),
			},
			errAssertion: assert.NoError,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
				s1 := graph.GetIRINodeByFragment("s1")
				assert.NotNil(t, s1)
				s2 := graph.GetIRINodeByFragment("s2")
				assert.NotNil(t, s2)
				var col sst.TermCollection
				assert.NoError(t, s1.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
					if s == s1 && p.Fragment() == "p1" && (o.TermKind() == sst.TermKindIBNode || o.TermKind() == sst.TermKindTermCollection) {
						o := o.(sst.IBNode)
						col, _ = o.AsCollection()
					}
					return nil
				}))
				if assert.NotEqual(t, nil, col) && assert.Equal(t, 1, col.MemberCount()) {
					assert.Equal(t, s2, col.Member(0))
				}
				assert.Equal(t, "urn:uuid:0c20d026-3fda-4e35-ab7d-88ba6cc0c5fc", graph.IRI().String())
			},
		},
		{
			name: "readGraphWithTermCollectionAndTriple",
			args: args{
				r: bb("SST-1.0\x00\x2durn:uuid:0c20d026-3fda-4e35-ab7d-88ba6cc0c5fc\x00\x02" +
					"\x1fhttp://semanticstep.net/schema#*http://www.w3.org/1999/02/22-rdf-syntax-ns\x02\x01\x02s1\x01\x02s2\x01\x03\x01\x02p1\x02\x01" +
					"\x05first\x01\x01\x03\x02\x00\x00\x00\x01\x04\x01\x01\x03\x00\x04val1"),
			},
			errAssertion: assert.NoError,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
				s1 := graph.GetIRINodeByFragment("s1")
				assert.NotNil(t, s1)
				s2 := graph.GetIRINodeByFragment("s2")
				assert.NotNil(t, s2)
				graph.Dump()

				var col sst.TermCollection
				assert.NoError(t, s1.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
					if s == s1 && p.Fragment() == "p1" && (o.TermKind() == sst.TermKindIBNode || o.TermKind() == sst.TermKindTermCollection) {
						o := o.(sst.IBNode)
						col, _ = o.AsCollection()
					}
					return nil
				}))
				if assert.NotEqual(t, nil, col) && assert.Equal(t, 1, col.MemberCount()) {
					assert.Equal(t, s2, col.Member(0))
					var tripleCnt int
					assert.NoError(t, col.(sst.IBNode).ForAll(func(index int, s, p sst.IBNode, o sst.Term) error {
						if s == col.(sst.IBNode) && p.Fragment() == string("p1") {
							tripleCnt++
							// assert.Equal(t, string("p1"), p.Fragment())
							assert.Equal(t, sst.TermKindLiteral, o.TermKind())
						}
						return nil
					}))
					assert.Equal(t, 1, tripleCnt)
				}
				assert.Equal(t, "urn:uuid:0c20d026-3fda-4e35-ab7d-88ba6cc0c5fc", graph.IRI().String())
			},
		},
		{
			name: "readGraphWithTermCollectionMembers",
			args: args{
				r: bb("SST-1.0\x00\x2durn:uuid:0c20d026-3fda-4e35-ab7d-88ba6cc0c5fc\x00\x02" +
					"\x1fhttp://semanticstep.net/schema#*http://www.w3.org/1999/02/22-rdf-syntax-ns\x02\x01\x02s1\x01\x02s2\x02\x04\x01\x02p1\x01\x01" +
					"\x05first\x03\x01\x03\x02\x00\x00\x00" +
					"\x03\x04\x01\x04\x04\x04\x01\x01\x04\x00\x04val2"),
			},
			errAssertion: assert.NoError,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
				s1 := graph.GetIRINodeByFragment("s1")
				assert.NotNil(t, s1)
				s2 := graph.GetIRINodeByFragment("s2")
				assert.NotNil(t, s2)
				var col sst.TermCollection
				assert.NoError(t, s1.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
					if s == s1 && p.Fragment() == "p1" && (o.TermKind() == sst.TermKindIBNode || o.TermKind() == sst.TermKindTermCollection) {
						o := o.(sst.IBNode)
						col, _ = o.AsCollection()
					}
					return nil
				}))
				if assert.NotEqual(t, nil, col) && assert.Equal(t, 3, col.MemberCount()) {
					assert.Equal(t, s2, col.Member(0))
					assert.Equal(t, sst.TermKindLiteral, col.Member(1).TermKind())
					assert.Equal(t, s2, col.Member(2))
				}
				assert.Equal(t, "urn:uuid:0c20d026-3fda-4e35-ab7d-88ba6cc0c5fc", graph.IRI().String())
			},
		},
		{
			name: "readGraphWithMemberCollection",
			args: args{
				r: bb("SST-1.0\x00\x2durn:uuid:0c20d026-3fda-4e35-ab7d-88ba6cc0c5fc\x00\x02" +
					"\x1fhttp://semanticstep.net/schema#*http://www.w3.org/1999/02/22-rdf-syntax-ns\x02\x02\x02s1\x01\x02s2\x02\x03\x02\x01\x02p1\x01\x01" +
					"\x05first\x03\x01\x04\x02\x00\x00\x00" +
					"\x02\x05\x01\x05\x03\x00\x01\x05\x01\x00"),
			},
			errAssertion: assert.NoError,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
				s1 := graph.GetIRINodeByFragment("s1")
				assert.NotNil(t, s1)
				s2 := graph.GetIRINodeByFragment("s2")
				assert.NotNil(t, s2)
				var col1 sst.TermCollection
				assert.NoError(t, s1.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
					if s == s1 && p.Fragment() == "p1" && (o.TermKind() == sst.TermKindIBNode || o.TermKind() == sst.TermKindTermCollection) {
						o := o.(sst.IBNode)
						col1, _ = o.AsCollection()
					}
					return nil
				}))
				if assert.NotEqual(t, nil, col1) && assert.Equal(t, 2, col1.MemberCount()) {
					assert.Equal(t, s2, col1.Member(0))
					col2Obj := col1.Member(1)
					if assert.Equal(t, sst.TermKindTermCollection, col2Obj.TermKind()) {
						col2, ok := col2Obj.(sst.IBNode).AsCollection()
						if assert.True(t, ok) {
							assert.Equal(t, 1, col2.MemberCount())
							var mc, ic int
							assert.NoError(t, col2.(sst.IBNode).ForAll(func(index int, s, p sst.IBNode, o sst.Term) error {
								// assert.Equal(t, sst.RdfFirstProperty, p)
								if s == col2.(sst.IBNode) {
									assert.Equal(t, s2, o)
									mc++
								} else {
									assert.Equal(t, col1, s)
									ic++
								}
								return nil
							}))
							assert.Equal(t, 1, mc)
							assert.Equal(t, 1, ic)
						}
					}
				}
				assert.Equal(t, "urn:uuid:0c20d026-3fda-4e35-ab7d-88ba6cc0c5fc", graph.IRI().String())
			},
		},
		{
			name: "incorrectIRILen",
			args: args{
				r: bb("SST-1.0\x00\x3furn:uuid:b7cb7c31-d5a6-4459-a08d-72bea1672fac\x00\x00"),
			},
			errAssertion:   assert.Error,
			graphAssertion: nil,
		},
		{
			name: "unexpectedEOF",
			args: args{
				r: bb("SST-1.0\x00\x2durn:uuid:b7cb7c31-d5a6-4459-a08d-72bea1672fac\x00\x00\x00\x00\x00"),
			},
			errAssertion: func(t assert.TestingT, err error, _ ...interface{}) bool {
				return assert.ErrorIs(t, err, sst.ErrEOFExpected)
			},
			graphAssertion: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "incorrectIRILen" {
				assert.Panics(t, func() { sst.SstRead(tt.args.r, sst.DefaultTriplexMode) })
			} else {
				got, err := sst.SstRead(tt.args.r, sst.DefaultTriplexMode)
				if tt.errAssertion(t, err) {
					if tt.graphAssertion != nil {
						tt.graphAssertion(t, got)
					}
				}
			}
		})
	}
}

func bb(str string) *bufio.Reader {
	return bufio.NewReader(bytes.NewBuffer([]byte(str)))
}

// Test_SstRead_TermCollectionWithLiteralCollectionMember verifies that a term
// collection whose member is a LiteralCollection can survive an SST write/read
// round-trip. This exercises fillRdfFirstPlaceholder, which is invoked when
// SstRead encounters an rdf:first triple whose object is a LiteralCollection.
func Test_SstRead_TermCollectionWithLiteralCollectionMember(t *testing.T) {
	turtle := `
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix : <http://example.org#> .

:s1 :p1 ( ( true ) ) .
`
	st, err := sst.RdfRead(bufio.NewReader(strings.NewReader(turtle)), sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
	require.NoError(t, err)

	ng := st.NamedGraphs()[0]
	s1 := ng.GetIRINodeByFragment("s1")
	require.NotNil(t, s1)

	var col sst.TermCollection
	require.NoError(t, s1.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if s == s1 && p.Fragment() == "p1" {
			if o.TermKind() == sst.TermKindIBNode || o.TermKind() == sst.TermKindTermCollection {
				col, _ = o.(sst.IBNode).AsCollection()
			}
		}
		return nil
	}))
	require.NotNil(t, col)
	require.Equal(t, 1, col.MemberCount())

	member := col.Member(0)
	require.Equal(t, sst.TermKindLiteralCollection, member.TermKind())
	lc := member.(sst.LiteralCollection)
	require.Equal(t, 1, lc.MemberCount())
	require.Equal(t, sst.Boolean(true), lc.Member(0))

	// Round-trip through SST binary format. This exercises fillRdfFirstPlaceholder.
	var buf bytes.Buffer
	require.NoError(t, ng.SstWrite(&buf))

	ng2, err := sst.SstRead(bufio.NewReader(&buf), sst.DefaultTriplexMode)
	require.NoError(t, err)

	s1b := ng2.GetIRINodeByFragment("s1")
	require.NotNil(t, s1b)

	var col2 sst.TermCollection
	require.NoError(t, s1b.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if s == s1b && p.Fragment() == "p1" {
			if o.TermKind() == sst.TermKindIBNode || o.TermKind() == sst.TermKindTermCollection {
				col2, _ = o.(sst.IBNode).AsCollection()
			}
		}
		return nil
	}))
	require.NotNil(t, col2)
	require.Equal(t, 1, col2.MemberCount())

	member2 := col2.Member(0)
	require.Equal(t, sst.TermKindLiteralCollection, member2.TermKind())
	lc2 := member2.(sst.LiteralCollection)
	require.Equal(t, 1, lc2.MemberCount())
	require.Equal(t, sst.Boolean(true), lc2.Member(0))
}

// Test_SstRead_TermCollectionWithMixedLiteralCollectionMembers verifies a term
// collection containing both IBNode members and LiteralCollection members.
func Test_SstRead_TermCollectionWithMixedLiteralCollectionMembers(t *testing.T) {
	turtle := `
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix : <http://example.org#> .

:s1 :p1 ( :s2 ( false ) :s3 ( true "hello" ) ) .
:s2 a :MyType .
:s3 a :MyType .
`
	st, err := sst.RdfRead(bufio.NewReader(strings.NewReader(turtle)), sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
	require.NoError(t, err)

	ng := st.NamedGraphs()[0]
	s1 := ng.GetIRINodeByFragment("s1")
	require.NotNil(t, s1)

	var col sst.TermCollection
	require.NoError(t, s1.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if s == s1 && p.Fragment() == "p1" {
			if o.TermKind() == sst.TermKindIBNode || o.TermKind() == sst.TermKindTermCollection {
				col, _ = o.(sst.IBNode).AsCollection()
			}
		}
		return nil
	}))
	require.NotNil(t, col)
	require.Equal(t, 4, col.MemberCount())

	require.Equal(t, sst.TermKindIBNode, col.Member(0).TermKind())
	require.Equal(t, sst.TermKindLiteralCollection, col.Member(1).TermKind())
	require.Equal(t, sst.TermKindIBNode, col.Member(2).TermKind())
	// ( true "hello" ) is NOT optimized to LiteralCollection because the
	// members have different datatypes (boolean vs string).
	require.Equal(t, sst.TermKindTermCollection, col.Member(3).TermKind())

	lc1 := col.Member(1).(sst.LiteralCollection)
	require.Equal(t, 1, lc1.MemberCount())
	require.Equal(t, sst.Boolean(false), lc1.Member(0))

	innerCol, ok := col.Member(3).(sst.IBNode).AsCollection()
	require.True(t, ok)
	require.Equal(t, 2, innerCol.MemberCount())
	require.Equal(t, sst.Boolean(true), innerCol.Member(0))
	require.Equal(t, sst.String("hello"), innerCol.Member(1))

	// Round-trip through SST binary format.
	var buf bytes.Buffer
	require.NoError(t, ng.SstWrite(&buf))

	ng2, err := sst.SstRead(bufio.NewReader(&buf), sst.DefaultTriplexMode)
	require.NoError(t, err)

	s1b := ng2.GetIRINodeByFragment("s1")
	require.NotNil(t, s1b)

	var col2 sst.TermCollection
	require.NoError(t, s1b.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if s == s1b && p.Fragment() == "p1" {
			if o.TermKind() == sst.TermKindIBNode || o.TermKind() == sst.TermKindTermCollection {
				col2, _ = o.(sst.IBNode).AsCollection()
			}
		}
		return nil
	}))
	require.NotNil(t, col2)
	require.Equal(t, 4, col2.MemberCount())
	require.Equal(t, sst.TermKindLiteralCollection, col2.Member(1).TermKind())
	// Same as above: different datatypes prevent LiteralCollection optimization.
	require.Equal(t, sst.TermKindTermCollection, col2.Member(3).TermKind())
}
