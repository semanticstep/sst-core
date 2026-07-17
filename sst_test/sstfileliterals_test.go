// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestWrite_literals(t *testing.T) {
	graphWithTripleCreator := func(t *testing.T, l sst.Term) sst.NamedGraph {
		st := sst.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(sst.IRI(uuid.MustParse("d6bf18ac-5729-499d-b010-661b1b79db5d").URN()))
		d := ng.CreateIRINode("main")
		d.AddStatement(rdfs.Comment, l)
		return ng
	}
	tests := []struct {
		name         string
		graphCreator func(*testing.T) sst.NamedGraph
		wantOutput   string
		assertion    assert.ErrorAssertionFunc
	}{
		{
			name: "simple_string",
			graphCreator: func(t *testing.T) sst.NamedGraph {
				return graphWithTripleCreator(t, sst.String("string in two\n\tlines"))
			},
			wantOutput: "SST-1.0\x00-urn:uuid:d6bf18ac-5729-499d-b010-661b1b79db5d\x00\x01$http://www.w3.org/2000/01/rdf-schema\x02\x00\x00\x00\x04main\x01\x01\acomment\x01\x00\x00\x00\x01\x02\x00\x14string in two\n\tlines",
			assertion:  assert.NoError,
		},
		{
			name: "en_string",
			graphCreator: func(t *testing.T) sst.NamedGraph {
				return graphWithTripleCreator(t, sst.LangStringOf("english string", "en"))
			},
			wantOutput: "SST-1.0\x00-urn:uuid:d6bf18ac-5729-499d-b010-661b1b79db5d\x00\x01$http://www.w3.org/2000/01/rdf-schema\x02\x00\x00\x00\x04main\x01\x01\acomment\x01\x00\x00\x00\x01\x02\x01\x0eenglish string\x02en",
			assertion:  assert.NoError,
		},
		{
			name: "boolean_true",
			graphCreator: func(t *testing.T) sst.NamedGraph {
				return graphWithTripleCreator(t, sst.Boolean(true))
			},
			wantOutput: "SST-1.0\x00-urn:uuid:d6bf18ac-5729-499d-b010-661b1b79db5d\x00\x01$http://www.w3.org/2000/01/rdf-schema\x02\x00\x00\x00\x04main\x01\x01\acomment\x01\x00\x00\x00\x01\x02\x02\x01",
			assertion:  assert.NoError,
		},
		{
			name: "boolean_false",
			graphCreator: func(t *testing.T) sst.NamedGraph {
				return graphWithTripleCreator(t, sst.Boolean(false))
			},
			wantOutput: "SST-1.0\x00-urn:uuid:d6bf18ac-5729-499d-b010-661b1b79db5d\x00\x01$http://www.w3.org/2000/01/rdf-schema\x02\x00\x00\x00\x04main\x01\x01\acomment\x01\x00\x00\x00\x01\x02\x02\x00",
			assertion:  assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			graph := tt.graphCreator(t)
			var output bytes.Buffer
			tt.assertion(t, graph.SstWrite(&output))
			assert.Equal(t, tt.wantOutput, output.String())
		})
	}
}

func TestRead_literals(t *testing.T) {
	tests := []struct {
		name           string
		r              *bufio.Reader
		errAssertion   assert.ErrorAssertionFunc
		graphAssertion assert.ValueAssertionFunc
	}{
		{
			name: "simple_string",
			r: bb("SST-1.0\x00\x2durn:uuid:5cdecbe6-8136-497f-a6c9-b3f6669fa811\x00" +
				"\x01\x25http://www.w3.org/2000/01/rdf-schema#\x01\x00\x04main\x01\x01\x07comment\x01\x00\x01\x01\x00" +
				"\x14string in two\n\tlines"),
			errAssertion:   assert.NoError,
			graphAssertion: assertSimpleStringGraph,
		},
		{
			name: "en_string",
			r: bb("SST-1.0\x00\x2durn:uuid:5cdecbe6-8136-497f-a6c9-b3f6669fa811\x00" +
				"\x01\x25http://www.w3.org/2000/01/rdf-schema#\x01\x00\x04main\x01\x01\x07comment\x01\x00\x01\x01\x01" +
				"\x0eenglish string\x02en"),
			errAssertion:   assert.NoError,
			graphAssertion: assertEnStringGraph,
		},
		{
			name: "boolean_true",
			r: bb("SST-1.0\x00\x2durn:uuid:5cdecbe6-8136-497f-a6c9-b3f6669fa811\x00" +
				"\x01\x25http://www.w3.org/2000/01/rdf-schema#\x01\x00\x04main\x01\x01\x07comment\x01\x00\x01\x01\x02\x01"),
			errAssertion:   assert.NoError,
			graphAssertion: assertBoolGraph(true),
		},
		{
			name: "boolean_false",
			r: bb("SST-1.0\x00\x2durn:uuid:5cdecbe6-8136-497f-a6c9-b3f6669fa811\x00" +
				"\x01\x25http://www.w3.org/2000/01/rdf-schema#\x01\x00\x04main\x01\x01\x07comment\x01\x00\x01\x01\x02\x00"),
			errAssertion:   assert.NoError,
			graphAssertion: assertBoolGraph(false),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sst.SstRead(tt.r, sst.DefaultTriplexMode)
			if tt.errAssertion(t, err) {
				if tt.graphAssertion != nil {
					tt.graphAssertion(t, got)
				}
			}
		})
	}
}

func TestWriteDiff_literal(t *testing.T) {
	type args struct {
		rFrom *bufio.Reader
		rTo   *bufio.Reader
	}
	tests := []struct {
		name      string
		args      args
		wantW     string
		assertion assert.ErrorAssertionFunc
	}{
		{
			name: "empty_graph_to_graph_with_simple_string",
			args: args{
				rFrom: bb("SST-1.0\x00\x2durn:uuid:5cdecbe6-8136-497f-a6c9-b3f6669fa811\x00\x00\x00\x00"),
				rTo: bb("SST-1.0\x00\x2durn:uuid:5cdecbe6-8136-497f-a6c9-b3f6669fa811\x00" +
					"\x01\x25http://www.w3.org/2000/01/rdf-schema#\x01\x00\x04main\x01\x01\x07comment\x01\x00\x01\x01\x00" +
					"\x14string in two\n\tlines"),
			},
			wantW: "\x00\x00\x00\x01\x02\x25http://www.w3.org/2000/01/rdf-schema#\x00\x01\x00\x00" +
				"\x02\x04main\x02\x00\x01\x02\x07comment\x02\x01\x00\x01\x01\x06\x00\x14string in two\n\tlines",
			assertion: assert.NoError,
		},
		{
			name: "empty_graph_to_graph_with_en_string",
			args: args{
				rFrom: bb("SST-1.0\x00\x2durn:uuid:5cdecbe6-8136-497f-a6c9-b3f6669fa811\x00\x00\x00\x00"),
				rTo: bb("SST-1.0\x00\x2durn:uuid:5cdecbe6-8136-497f-a6c9-b3f6669fa811\x00" +
					"\x01\x25http://www.w3.org/2000/01/rdf-schema#\x01\x00\x04main\x01\x01\x07comment\x01\x00\x01\x01\x01" +
					"\x0eenglish string\x02en"),
			},
			wantW: "\x00\x00\x00\x01\x02\x25http://www.w3.org/2000/01/rdf-schema#\x00\x01\x00\x00" +
				"\x02\x04main\x02\x00\x01\x02\x07comment\x02\x01\x00\x01\x01\x06\x01\x0eenglish string\x02en",
			assertion: assert.NoError,
		},
		{
			name: "empty_graph_to_graph_with_boolean_true",
			args: args{
				rFrom: bb("SST-1.0\x00\x2durn:uuid:5cdecbe6-8136-497f-a6c9-b3f6669fa811\x00\x00\x00\x00"),
				rTo: bb("SST-1.0\x00\x2durn:uuid:5cdecbe6-8136-497f-a6c9-b3f6669fa811\x00" +
					"\x01\x25http://www.w3.org/2000/01/rdf-schema#\x01\x00\x04main\x01\x01\x07comment\x01\x00\x01\x01\x02\x01"),
			},
			wantW: "\x00\x00\x00\x01\x02\x25http://www.w3.org/2000/01/rdf-schema#\x00\x01\x00\x00" +
				"\x02\x04main\x02\x00\x01\x02\x07comment\x02\x01\x00\x01\x01\x06\x02\x01",
			assertion: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &bytes.Buffer{}
			_, err := sst.SstWriteDiff(tt.args.rFrom, tt.args.rTo, w, true)
			tt.assertion(t, err)
			assert.Equal(t, tt.wantW, w.String())
		})
	}
}

func TestReadDiff_literal(t *testing.T) {
	type args struct {
		rBase *bufio.Reader
		rDiff *bufio.Reader
	}
	tests := []struct {
		name           string
		args           args
		want           sst.NamedGraph
		errAssertion   assert.ErrorAssertionFunc
		graphAssertion assert.ValueAssertionFunc
	}{
		{
			name: "empty_graph_to_graph_with_simple_string",
			args: args{
				rBase: bb("SST-1.0\x00\x2durn:uuid:5cdecbe6-8136-497f-a6c9-b3f6669fa811\x00\x00\x00\x00"),
				rDiff: bb("\x00\x00\x00\x01\x02\x25http://www.w3.org/2000/01/rdf-schema#\x00\x01\x00\x00" +
					"\x02\x04main\x02\x00\x01\x02\x07comment\x02\x01\x00\x01\x01\x06\x00\x14string in two\n\tlines"),
			},
			errAssertion:   assert.NoError,
			graphAssertion: assertSimpleStringGraph,
		},
		{
			name: "en_string",
			args: args{
				rBase: bb("SST-1.0\x00\x2durn:uuid:5cdecbe6-8136-497f-a6c9-b3f6669fa811\x00\x00\x00\x00"),
				rDiff: bb("\x00\x00\x00\x01\x02\x25http://www.w3.org/2000/01/rdf-schema#\x00\x01\x00\x00" +
					"\x02\x04main\x02\x00\x01\x02\x07comment\x02\x01\x00\x01\x01\x06\x01\x0eenglish string\x02en"),
			},
			errAssertion:   assert.NoError,
			graphAssertion: assertEnStringGraph,
		},
		{
			name: "boolean_true",
			args: args{
				rBase: bb("SST-1.0\x00\x2durn:uuid:5cdecbe6-8136-497f-a6c9-b3f6669fa811\x00\x00\x00\x00"),
				rDiff: bb("\x00\x00\x00\x01\x02\x25http://www.w3.org/2000/01/rdf-schema#\x00\x01\x00\x00" +
					"\x02\x04main\x02\x00\x01\x02\x07comment\x02\x01\x00\x01\x01\x06\x02\x01"),
			},
			errAssertion:   assert.NoError,
			graphAssertion: assertBoolGraph(true),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sst.SstReadDiff(sst.DefaultTriplexMode, tt.args.rBase, tt.args.rDiff)
			if tt.errAssertion(t, err) {
				if tt.graphAssertion != nil {
					tt.graphAssertion(t, got)
				}
			}
		})
	}
}

func assertSimpleStringGraph(t assert.TestingT, g interface{}, _ ...interface{}) bool {
	graph := g.(sst.NamedGraph)
	assert.Equal(t, uuid.MustParse("5cdecbe6-8136-497f-a6c9-b3f6669fa811").URN(), graph.IRI().String())
	assert.Equal(t, 2, graph.IRINodeCount())
	d := graph.GetIRINodeByFragment("main")
	assert.NotNil(t, d)

	return assert.NoError(t, d.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if d == s {
			assert.Equal(t, "string in two\n\tlines", string(o.(sst.String)))

		}
		return nil
	}))
}

func assertEnStringGraph(t assert.TestingT, g interface{}, _ ...interface{}) bool {
	graph := g.(sst.NamedGraph)
	assert.Equal(t, uuid.MustParse("5cdecbe6-8136-497f-a6c9-b3f6669fa811").URN(), graph.IRI().String())
	assert.Equal(t, 2, graph.IRINodeCount())
	d := graph.GetIRINodeByFragment("main")
	assert.NotNil(t, d)
	return assert.NoError(t, d.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if d == s {
			l := o.(sst.LangString)
			assert.Equal(t, "english string", l.Val)
			assert.Equal(t, "en", l.LangTag)
		}
		return nil
	}))
}

func assertBoolGraph(expect bool) assert.ValueAssertionFunc {
	return func(t assert.TestingT, g interface{}, _ ...interface{}) bool {
		graph := g.(sst.NamedGraph)
		assert.Equal(t, uuid.MustParse("5cdecbe6-8136-497f-a6c9-b3f6669fa811").URN(), graph.IRI().String())
		assert.Equal(t, 2, graph.IRINodeCount())
		d := graph.GetIRINodeByFragment("main")
		assert.NotNil(t, d)
		return assert.NoError(t, d.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
			if d == s {
				assert.Equal(t, expect, bool(o.(sst.Boolean)))

			}
			return nil
		}))
	}
}
