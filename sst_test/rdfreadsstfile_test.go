// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"bufio"
	"os"
	"path/filepath"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromFile_sstfileWrite(t *testing.T) {
	file_44e, err := os.Open("testdata/24f24afe-e30c-401f-bccd-46f2a125344e.ttl")
	defer func() {
		e := file_44e.Close()
		if err == nil {
			err = e
		}
	}()

	tests := []struct {
		reader        *bufio.Reader
		name          string
		path          string
		wantAssertion func(t assert.TestingT, fi os.FileInfo)
		assertion     assert.ErrorAssertionFunc
	}{
		{
			reader: bufio.NewReader(file_44e),
			name:   "24f24afe_e30c_401f_bccd_46f2a125344e",
			path:   "testdata/24f24afe-e30c-401f-bccd-46f2a125344e.ttl",
			wantAssertion: func(t assert.TestingT, fi os.FileInfo) {
				assert.Greater(t, fi.Size(), int64(0))
			},
			assertion: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, err := sst.RdfRead(tt.reader, sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
			graph := st.NamedGraphs()[0]
			require.NoError(t, err)
			outFilePath := filepath.Join(t.TempDir(), "out.sst")
			file, err := os.Create(outFilePath)
			require.NoError(t, err)
			writtenSST := assert.NotPanics(t, func() { tt.assertion(t, graph.SstWrite(file)) })
			assert.NoError(t, file.Close())
			defer os.Remove(outFilePath)
			if writtenSST {
				file, err = os.Open(outFilePath)
				assert.NoError(t, err)
				loadedGraph, err := sst.SstRead(bufio.NewReader(file), sst.DefaultTriplexMode)
				assert.NoError(t, err)
				assert.Equal(t, graph.IRINodeCount(), loadedGraph.IRINodeCount())
				defer file.Close()
			}
			fi, err := os.Stat(outFilePath)
			assert.NoError(t, err)
			tt.wantAssertion(t, fi)
		})
	}
}

type dummyWriter struct{}

func (w dummyWriter) Write(p []byte) (n int, err error) {
	return len(p), err
}
