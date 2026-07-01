// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

// The reason why there is a separate package is about RegisterVocabularyMap.
// In sst_test_dummyvoc package, it imports package compiler, which will register a test VocabularyMap.
// In sst_test_realvoc package, it imports package vocabularies, which will register a real
// VocabularyMap that includes lci, owl...

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// check typeof field after MoveAndMerge
func Test_Stage_MergeWithReport(t *testing.T) {
	// AtomicLevel.SetLevel(zap.DebugLevel)
	fileName1 := filepath.Join("./testdata/" + "Test_Stage_Merge1")
	fileName2 := filepath.Join("./testdata/" + "Test_Stage_Merge2")
	outFile2 := filepath.Join(t.TempDir(), "Test_Stage_Merge2")

	t.Run("write", func(t *testing.T) {
		ng := readTTLFile(fileName1 + ".ttl")

		ng2 := readTTLFile(fileName2 + ".ttl")

		out, err := os.Create(outFile2 + ".sst")
		if err != nil {
			log.Panic(err)
		}
		// ... ensure that the file will be closed at the end ...
		defer out.Close()

		// ... and write the binary graph content out into the file.
		err = ng2.SstWrite(out)
		if err != nil {
			log.Panic(err)
		}

		john := ng2.GetIRINodeByFragment("John")
		// inside sst package, typeof field cannot be assigned
		assert.Nil(t, john.TypeOf())

		ng2.PrintTriples()

		report, err := ng.Stage().MoveAndMerge(context.TODO(), ng2.Stage())
		if err != nil {
			panic(err)
		}

		fmt.Println(report)

		john2 := ng.Stage().NamedGraph(IRI(ng2.IRI())).GetIRINodeByFragment("John")
		assert.Nil(t, john2.TypeOf())
		ng.PrintTriples()
	})
	t.Run("write", func(t *testing.T) {
		in, err := os.Open(outFile2 + ".sst")
		if err != nil {
			log.Panic(err)
		}
		defer in.Close()
		graph22, err := SstRead(bufio.NewReader(in), DefaultTriplexMode)
		if err != nil {
			log.Panic(err)
		}
		john := graph22.GetIRINodeByFragment("John")

		assert.Nil(t, john.TypeOf())
	})
}

// test rep ttl file RdfRead and sst file SstRead
func Test_RdfRead_SstWrite_RdfFirst(t *testing.T) {
	AtomicLevel.SetLevel(zap.DebugLevel)
	inFile := filepath.Join("./testdata/" + "rep")
	outFile := filepath.Join(t.TempDir(), "rep")
	t.Run("RdfRead", func(t *testing.T) {
		file, err := os.Open(inFile + ".ttl")
		if err != nil {
			panic(err)
		}
		defer file.Close()

		// convert the content of the turtle file "helloworldwrite.ttl", which is RDF, to a default NamedGraph.
		st, err := RdfRead(bufio.NewReader(file), RdfFormatTurtle, StrictHandler, DefaultTriplexMode)
		if err != nil {
			panic(err)
		}
		graph := st.NamedGraphs()[0]

		graph.PrintTriples()

		out, err := os.Create(outFile + ".sst")
		if err != nil {
			panic(err)
		}
		defer out.Close()

		err = graph.SstWrite(out)
		if err != nil {
			log.Panic(err)
		}

	})
	t.Run("SstRead", func(t *testing.T) {
		in, err := os.Open(outFile + ".sst")
		if err != nil {
			log.Panic(err)
		}
		defer in.Close()
		graph22, err := SstRead(bufio.NewReader(in), DefaultTriplexMode)
		if err != nil {
			log.Panic(err)
		}
		graph22.Dump()
	})

}

// test RdfRead with StrictHandler
func Test_RdfRead_SstWrite_StrictHandler(t *testing.T) {
	AtomicLevel.SetLevel(zap.DebugLevel)
	fileName1 := filepath.Join("./testdata/" + "format-test")

	t.Run("write", func(t *testing.T) {
		assert.Panics(t, func() {
			file, err := os.Open(fileName1 + ".ttl")
			if err != nil {
				panic(err)
			}
			defer file.Close()

			// convert the content of the turtle file "helloworldwrite.ttl", which is RDF, to a default NamedGraph.
			st, err := RdfRead(bufio.NewReader(file), RdfFormatTurtle, StrictHandler, DefaultTriplexMode)
			if err != nil {
				panic(err)
			}
			graph := st.NamedGraphs()[0]
			_ = graph
		})
	})
}

func Test_RdfRead_SstWrite_RecoverHandler(t *testing.T) {
	AtomicLevel.SetLevel(zap.DebugLevel)
	fileName1 := filepath.Join("./testdata/" + "format-test")

	defer os.Remove(fileName1 + ".sst")
	t.Run("write", func(t *testing.T) {
		assert.NotPanics(t, func() {
			file, err := os.Open((fileName1 + ".ttl"))
			if err != nil {
				panic(err)
			}
			defer file.Close()

			// convert the content of the turtle file "helloworldwrite.ttl", which is RDF, to a default NamedGraph.
			st, err := RdfRead(bufio.NewReader(file), RdfFormatTurtle, RecoverHandler, DefaultTriplexMode)
			if err != nil {
				panic(err)
			}
			graph := st.NamedGraphs()[0]
			_ = graph
		})
	})
}

func Test_RdfRead_SstWrite_UserDefinedErrorHandler(t *testing.T) {
	AtomicLevel.SetLevel(zap.DebugLevel)
	fileName1 := filepath.Join("./testdata/" + "format-test")

	defer os.Remove(fileName1 + ".sst")
	t.Run("write", func(t *testing.T) {
		assert.NotPanics(t, func() {
			file, err := os.Open((fileName1 + ".ttl"))
			if err != nil {
				panic(err)
			}
			defer file.Close()

			// convert the content of the turtle file "helloworldwrite.ttl", which is RDF, to a default NamedGraph.
			st, err := RdfRead(bufio.NewReader(file), RdfFormatTurtle,
				func(err error) error {
					fmt.Println(err)
					return nil
				}, DefaultTriplexMode)
			if err != nil {
				panic(err)
			}
			graph := st.NamedGraphs()[0]
			_ = graph
		})
	})
}
