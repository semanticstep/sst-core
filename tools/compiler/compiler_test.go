// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package main

import (
	"io"
	"io/fs"
	"log"
	"testing"

	wrfs "github.com/relab/wrfs"
	"github.com/semanticstep/sst-core/sst"
	ontologies "github.com/semanticstep/sst-ontologies"
	"github.com/stretchr/testify/assert"
)

func TestLoadDictOntologies(t *testing.T) {
	type args struct {
		fsys    fs.FS
		baseDir string
		errs    ErrorReporter
	}
	tests := []struct {
		name          string
		args          args
		wantAssertion func(testing.TB, sst.Stage)
		assertion     assert.ErrorAssertionFunc
	}{
		{
			name: "load_write_read",
			args: args{
				fsys:    ontologies.FS,
				baseDir: "ontologies",
				errs:    log.New(io.Discard, "", 0),
			},
			wantAssertion: func(t testing.TB, stage sst.Stage) {
				dictDir := t.TempDir()
				assert.NoError(t, stage.WriteToSstFilesWithBaseURL(wrfs.DirFS(dictDir)))
				st, err := sst.ReadStageFromSstFiles(wrfs.DirFS(dictDir), sst.DefaultTriplexMode)
				if !assert.NoError(t, err) {
					return
				}

				var ngIRIs []sst.IRI
				for _, ng := range stage.NamedGraphs() {
					ngIRIs = append(ngIRIs, ng.IRI())
				}

				var IRIs []sst.IRI
				for _, ng := range st.NamedGraphs() {
					IRIs = append(IRIs, ng.IRI())
				}

				assert.ElementsMatch(t, ngIRIs, IRIs)
			},
			assertion: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// for i := 0; i < 50; i++ {
			got, err := LoadDictOntologies(tt.args.fsys, tt.args.baseDir, tt.args.errs)
			tt.assertion(t, err)
			tt.wantAssertion(t, got)
			// }
		})
	}
}
