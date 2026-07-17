// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"testing"

	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestLangStringLiteral(t *testing.T) {
	type args struct {
		val     string
		langTag string
	}
	tests := []struct {
		name            string
		args            args
		wantVal         string
		wantLang        string
		objectAssertion assert.ValueAssertionFunc
	}{
		{
			name: "en_string_with_value",
			args: args{
				val:     "string-with-language",
				langTag: "en",
			},
			wantVal:  "string-with-language",
			wantLang: "en",
		},
		{
			name: "string_value",
			args: args{
				val:     "value",
				langTag: "de",
			},
			wantVal:  "value",
			wantLang: "de",
			objectAssertion: func(t assert.TestingT, v interface{}, _ ...interface{}) bool {
				return assert.Equal(t, "value", v.(sst.LangString).Val)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stage := sst.OpenStage(sst.DefaultTriplexMode)
			id, _ := uuid.NewRandom()
			graph := stage.CreateNamedGraph(sst.IRI(id.URN()))
			d := graph.CreateBlankNode()
			d.AddStatement(rdfs.Comment, sst.LangStringOf(tt.args.val, tt.args.langTag))
			assert.NoError(t, d.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
				if d == s && p.Is(rdfs.Comment) && o.TermKind() == sst.TermKindLiteral {
					assert.Equal(t, tt.wantVal, o.(sst.LangString).Val)
					assert.Equal(t, tt.wantLang, o.(sst.LangString).LangTag)
					if tt.objectAssertion != nil {
						tt.objectAssertion(t, o)
					}
				}
				return nil
			}))
		})
	}
}

func Test_literals(t *testing.T) {
	tests := []struct {
		name    string
		literal interface {
			sst.Term
			sst.Literal
		}
		wantValue       interface{}
		objectAssertion assert.ValueAssertionFunc
	}{
		{
			name:      "boolean_true_literal",
			literal:   sst.Boolean(true),
			wantValue: true,
			objectAssertion: func(t assert.TestingT, l interface{}, _ ...interface{}) bool {
				return assert.Equal(t, true, bool(l.(sst.Boolean)))
			},
		},
		{
			name:      "boolean_false_literal",
			literal:   sst.Boolean(false),
			wantValue: false,
			objectAssertion: func(t assert.TestingT, l interface{}, _ ...interface{}) bool {
				return assert.Equal(t, false, bool(l.(sst.Boolean)))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stage := sst.OpenStage(sst.DefaultTriplexMode)
			id, _ := uuid.NewRandom()
			graph := stage.CreateNamedGraph(sst.IRI(id.URN()))
			d := graph.CreateBlankNode()
			d.AddStatement(rdfs.Comment, tt.literal)
			assert.NoError(t, d.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
				if d == s && p.Is(rdfs.Comment) && o.TermKind() == sst.TermKindLiteral {
					assert.Equal(t, sst.Boolean(tt.wantValue.(bool)), o.(sst.Literal))
					tt.objectAssertion(t, o)
				}
				return nil
			}))
		})
	}
}
