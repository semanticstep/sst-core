// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package p21

import (
	"bufio"
	"os"
	"strings"
	"testing"
	"unicode"

	"github.com/semanticstep/sst-core/sst"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_lexer_Lex(t *testing.T) {
	type args struct {
		src  *bufio.Reader
		lval *yySymType
	}
	tests := []struct {
		name string
		args args
		want []int
	}{
		{
			name: "string_only",
			args: args{
				src:  bufio.NewReader(strings.NewReader(`'a'`)),
				lval: &yySymType{},
			},
			want: []int{STRING},
		},
		{
			name: "lhs_eq_resource",
			args: args{
				src:  bufio.NewReader(strings.NewReader(`#10=<http://example.com/definition>;`)),
				lval: &yySymType{},
			},
			want: []int{ENTITY_INSTANCE_NAME, '=', RESOURCE, ';'},
		},
		{
			name: "string_with_apostrophe",
			args: args{
				src:  bufio.NewReader(strings.NewReader(`' '''`)),
				lval: &yySymType{},
			},
			want: []int{STRING},
		},
		{
			name: "string_with_utf8",
			args: args{
				src:  bufio.NewReader(strings.NewReader(`'.€.'`)),
				lval: &yySymType{},
			},
			want: []int{STRING},
		},
		{
			name: "string_with_nl",
			args: args{
				src: bufio.NewReader(strings.NewReader(`'a
				'`)),
				lval: &yySymType{},
			},
			want: []int{unicode.ReplacementChar, '\''},
		},
		{
			name: "string_and_illegal_char",
			args: args{
				src:  bufio.NewReader(strings.NewReader("'a'\x00")),
				lval: &yySymType{},
			},
			want: []int{STRING, unicode.ReplacementChar},
		},
		{
			name: "strings",
			args: args{
				src: bufio.NewReader(strings.NewReader(`
				'a'
				'b'`)),
				lval: &yySymType{},
			},
			want: []int{STRING, STRING},
		},
		{
			name: "string_with_comment",
			args: args{
				src: bufio.NewReader(strings.NewReader(`/* '
				'b' */
				'c'
				/*'d'*/
				`)),
				lval: &yySymType{},
			},
			want: []int{STRING},
		},
		{
			name: "string_eq_string",
			args: args{
				src:  bufio.NewReader(strings.NewReader(`'a'='b'`)),
				lval: &yySymType{},
			},
			want: []int{STRING, int('='), STRING},
		},
		{
			name: "unterminated_comment",
			args: args{
				src: bufio.NewReader(strings.NewReader(`/* '
				`)),
				lval: &yySymType{},
			},
			want: []int{unicode.ReplacementChar},
		},
		{
			name: "keyword_only",
			args: args{
				src:  bufio.NewReader(strings.NewReader(`DATA`)),
				lval: &yySymType{},
			},
			want: []int{STANDARD_KEYWORD},
		},
		{
			name: "data_keyword",
			args: args{
				src: bufio.NewReader(strings.NewReader(`
				ENDSEC;
				DATA;
				PART`)),
				lval: &yySymType{},
			},
			want: []int{ENDSEC, DATA, ';', STANDARD_KEYWORD},
		},
		{
			name: "tag_name",
			args: args{
				src: bufio.NewReader(strings.NewReader(`
				ANCHOR;
				<tool_tip_face>=#10 {TAG:#11};
				ENDSEC;
				`)),
				lval: &yySymType{},
			},
			want: []int{
				ANCHOR, ANCHOR_NAME, '=', ENTITY_INSTANCE_NAME,
				'{', TAG_NAME, ':', ENTITY_INSTANCE_NAME, '}', ';', ENDSEC,
			},
		},
		{
			name: "signature_section",
			args: args{
				src: bufio.NewReader(strings.NewReader(`
				END-ISO-10303-21;
 /*			*/	SIGNATURE
dvgLYb3tnkuyGfQJoM7N6IIb
				ENDSEC;
				`)),
				lval: &yySymType{},
			},
			want: []int{ENDISO1030321, SIGNATURE, SIGNATURE_CONTENT, ENDSEC},
		},
		{
			name: "end_iso_no_nl",
			args: args{
				src: bufio.NewReader(strings.NewReader(`
				END-ISO-10303-21;`)),
				lval: &yySymType{},
			},
			want: []int{ENDISO1030321},
		},
		{
			name: "endesc_end_iso",
			args: args{
				src: bufio.NewReader(strings.NewReader(`
				ENDSEC;
				END-ISO-10303-21;`)),
				lval: &yySymType{},
			},
			want: []int{ENDSEC, ENDISO1030321},
		},
		{
			name: "end_iso_nl",
			args: args{
				src: bufio.NewReader(strings.NewReader(`
				END-ISO-10303-21;` + "\n")),
				lval: &yySymType{},
			},
			want: []int{ENDISO1030321},
		},
		{
			name: "AP214miniPart",
			args: func() args {
				f, err := os.Open("testdata/AP214miniPart.stp.header")
				require.NoError(t, err)
				t.Cleanup(func() {
					require.NoError(t, f.Close())
				})
				return args{
					src:  bufio.NewReader(f),
					lval: &yySymType{},
				}
			}(),
			want: []int{
				ISO1030321, HEADER, STANDARD_KEYWORD, '(', '(', STRING, ')', ',', STRING, ')', ';',
				STANDARD_KEYWORD, '(', STRING, ',', STRING, ',', '(', STRING, ')', ',', '(', STRING, ')', ',',
				STRING, ',', STRING, ',', STRING, ')', ';', STANDARD_KEYWORD, '(', '(', STRING, ')', ')', ';',
				ENDSEC, DATA, ';',
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := newLexer(tt.args.src, newTestSstData(t))
			var got []int
			for {
				token := l.Lex(tt.args.lval)
				if token == 0 {
					break
				}
				got = append(got, token)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func newTestSstData(t testing.TB) *sstData {
	stage := sst.OpenStage(sst.DefaultTriplexMode)
	id, _ := uuid.NewRandom()
	graph := stage.CreateNamedGraph(sst.IRI(id.URN()))
	return newSSTData(graph)
}
