// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/stretchr/testify/assert"
)

// example 1: read a ttl and write it to an SST file
func TestReadTTLWriteToSST(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())
	ttlFilePath := filepath.Join(testName)

	defer os.Remove(ttlFilePath + ".sst")
	t.Run("read then write", func(t *testing.T) {
		file, err := os.Open(ttlFilePath + ".ttl")
		if err != nil {
			log.Panicf("Error opening file %s: %v", ttlFilePath, err)
		}
		defer file.Close()

		ng, err := sst.RdfRead(bufio.NewReader(file), sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
		if err != nil {
			t.Fatal(err)
		}

		out, err := os.Create(ttlFilePath + ".sst")
		if err != nil {
			panic(err)
		}
		defer out.Close()

		err = ng.NamedGraphs()[0].SstWrite(out)
		if err != nil {
			log.Panic(err)
		}
	})
}

// example 2: read a ttl without NG, expect a panic
func TestReadTTLWriteToSSTWithOutNG(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())
	ttlFilePath := filepath.Join(testName)
	defer os.Remove(ttlFilePath + ".sst")
	t.Run("read then write", func(t *testing.T) {
		file, err := os.Open(ttlFilePath + ".ttl")
		if err != nil {
			log.Panicf("Error opening file %s: %v", ttlFilePath, err)
		}
		defer file.Close()

		assert.Panics(t, func() {
			sst.RdfRead(bufio.NewReader(file), sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
		})
	})
}

// example 3: read ttls from a folder and put them to SST files in another folder, also check the typeof of nodes
func TestReadTTLsWriteToSSTs(t *testing.T) {
	ttlPath := filepath.Join("./testdata/" + t.Name())
	sstPath := filepath.Join("./testdata/" + t.Name() + "SSTs")
	defer os.RemoveAll(sstPath)

	t.Run("read then write", func(t *testing.T) {
		// read dir
		removeFolder(sstPath)
		err := os.Mkdir(sstPath, os.ModePerm)
		if err != nil {
			log.Panicf("Error creating directory %s: %v", sstPath, err)
		}

		entries, err := os.ReadDir(ttlPath)
		if err != nil {
			fmt.Printf("unable to read %s: %v\n", ttlPath, err)
			return
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue // skip subdirectories
			}

			fullPath := filepath.Join(ttlPath, entry.Name())
			file, err := os.Open(fullPath)
			if err != nil {
				log.Panicf("Error opening file %s: %v", fullPath, err)
			}
			defer file.Close()

			ng, err := sst.RdfRead(bufio.NewReader(file), sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
			if err != nil {
				t.Fatal(err)
			}

			out, err := os.Create(filepath.Join(sstPath, strings.TrimSuffix(entry.Name(), ".ttl")+".sst"))
			if err != nil {
				panic(err)
			}
			defer out.Close()

			err = ng.NamedGraphs()[0].SstWrite(out)
			if err != nil {
				log.Panic(err)
			}
		}
	})

	// read SSTs from a folder then MoveAndMerge and check its typeof
	t.Run("read then write", func(t *testing.T) {
		// read dir
		entries, err := os.ReadDir(sstPath)
		if err != nil {
			fmt.Printf("unable to read %s: %v\n", sstPath, err)
			return
		}

		newStorageStage := sst.OpenStage(sst.DefaultTriplexMode)
		for _, entry := range entries {
			if entry.IsDir() {
				continue // skip subdirectories
			}

			fullPath := filepath.Join(sstPath, entry.Name())

			file, err := os.Open(fullPath)
			if err != nil {
				log.Panicf("Error opening file %s: %v", fullPath, err)
			}
			defer file.Close()

			ng, err := sst.SstRead(bufio.NewReader(file), sst.DefaultTriplexMode)
			if err != nil {
				panic(err)
			}

			ng.ForIRINodes(func(d sst.IBNode) error {
				if d.TypeOf() != nil {
					fmt.Println(d.IRI(), d.TypeOf().IRI())
				} else {
					fmt.Println(d.IRI(), d.TypeOf())
				}
				return nil
			})
			_, err = newStorageStage.MoveAndMerge(context.TODO(), ng.Stage())
			if err != nil {
				panic(err)
			}
			fmt.Println("------------------------------------------------------------------------")
		}

		for _, val := range newStorageStage.NamedGraphs() {
			val.ForIRINodes(func(d sst.IBNode) error {
				if d.TypeOf() != nil {
					fmt.Println(d.IRI(), d.TypeOf().IRI())
				} else {
					fmt.Println(d.IRI(), d.TypeOf())
				}
				return nil
			})
		}
	})
}
