// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/semanticstep/sst-core/vocabularies/sso"
	"github.com/google/uuid"
	fs "github.com/relab/wrfs"
	"github.com/stretchr/testify/assert"
)

func TestSstWriteStage(t *testing.T) {
	tests := []struct {
		name         string
		stageCreator func(*testing.T) sst.Stage
		assertion    assert.ErrorAssertionFunc
	}{
		{
			name: "example1",
			stageCreator: func(t *testing.T) sst.Stage {
				st := sst.OpenStage(sst.DefaultTriplexMode)
				g3 := st.CreateNamedGraph(sst.IRI(uuid.New().URN()))
				g2 := st.CreateNamedGraph(sst.IRI(uuid.New().URN()))
				g3.AddImport(g2)
				g1 := st.CreateNamedGraph(sst.IRI(uuid.New().URN()))
				g2.AddImport(g1)

				// g3i, _, err := g3.Imports().AddOrUpgradeByID(uuid.New(), sst.ImportFilterNew)
				// assert.NoError(t, err)
				// g2, err := g3i.Graph()
				// assert.NoError(t, err)
				// g2i, _, err := g2.Imports().AddOrUpgradeByID(uuid.New(), sst.ImportFilterNew)
				// assert.NoError(t, err)
				// g1, err := g2i.Graph()
				// assert.NoError(t, err)
				mainG1 := g1.CreateIRINode("main")
				mainG1.AddStatement(rdf.Type, lci.Organization)
				mainG1.AddStatement(rdfs.Label, sst.String("ABC Ltd."))
				uuid1 := g2.CreateIRINode(string(uuid.New().String()))
				uuid1.AddStatement(rdfs.SubPropertyOf, sso.ID)
				uuid1.AddStatement(sso.IDOwner, mainG1)
				mainG2 := g2.CreateIRINode("main")
				mainG2.AddStatement(rdf.Type, sso.Part)
				mainG2.AddStatement(rdfs.Label, sst.String("my fancy part"))
				mainG2.AddStatement(uuid1, sst.String("123"))
				mainG3 := g3.CreateIRINode("main")
				mainG3.AddStatement(rdf.Type, sso.AssemblyDesign)
				mainG3.AddStatement(rdfs.Label, sst.String("my first Assembly"))
				mainG3.AddStatement(uuid1, sst.String("4711"))
				return st
			},
			assertion: func(t assert.TestingT, err error, i ...interface{}) bool {
				if !assert.NoError(t, err) {
					return false
				}
				outDir := i[0].(string)
				contentInfo, err := os.ReadDir(outDir)
				return assert.NoError(t, err) && assert.Len(t, contentInfo, 3) &&
					assert.Condition(t, func() bool {
						for _, i := range contentInfo {
							if !assert.False(t, i.IsDir()) {
								return false
							}
						}
						return true
					}, "stage directory should contain files only")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outDir := filepath.Join(t.TempDir(), tt.name)
			assert.NoError(t, os.Mkdir(outDir, os.ModePerm))
			tt.assertion(t, tt.stageCreator(t).WriteSstFilesDirectory(fs.DirFS(outDir)), outDir)
		})
	}
}
