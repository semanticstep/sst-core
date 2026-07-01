// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

// P21Converter is an example SST application that converts a STEP Part 21 file
// to Turtle and can also write the raw parser graph before SST conversion.
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/step/p21"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
)

func main() {
	inputPath := flag.String("in", "", "input STEP Part 21 file")
	outputPath := flag.String("out", "", "converted Turtle output file")
	rawOutputPath := flag.String("raw", "", "raw pre-conversion Turtle output file")
	flag.Parse()

	if *inputPath == "" && flag.NArg() > 0 {
		*inputPath = flag.Arg(0)
	}
	if *inputPath == "" {
		log.Fatal("missing input file; usage: go run ./examples/p21 -in input.stp [-out output.ttl] [-raw raw.ttl]")
	}
	if *outputPath == "" {
		*outputPath = defaultOutputPath(*inputPath)
	}
	if *rawOutputPath == "" {
		*rawOutputPath = defaultRawOutputPath(*inputPath)
	}

	data, err := os.ReadFile(*inputPath)
	if err != nil {
		log.Fatal(err)
	}

	rawGraph, err := p21.ParseRaw(bufio.NewReader(bytes.NewReader(data)), log.Default())
	if err != nil {
		log.Fatal(err)
	}
	if err := writeTurtle(*rawOutputPath, rawGraph); err != nil {
		log.Fatal(err)
	}

	convertedGraph, err := p21.Parse(bufio.NewReader(bytes.NewReader(data)), log.Default())
	if err != nil {
		log.Fatal(err)
	}
	if err := writeTurtle(*outputPath, convertedGraph); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("raw: %s\nconverted: %s\n", *rawOutputPath, *outputPath)
}

func defaultOutputPath(inputPath string) string {
	return filepath.Join(defaultOutputDirectory(), inputBaseName(inputPath)+".ttl")
}

func defaultRawOutputPath(inputPath string) string {
	base := inputBaseName(inputPath)
	return filepath.Join(defaultOutputDirectory(), "RAW-"+base+".ttl")
}

func defaultOutputDirectory() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	return filepath.Dir(file)
}

func inputBaseName(inputPath string) string {
	base := filepath.Base(inputPath)
	ext := filepath.Ext(base)
	if ext != "" {
		base = base[:len(base)-len(ext)]
	}
	return base
}

func writeTurtle(path string, graph sst.NamedGraph) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	return graph.RdfWrite(file, sst.RdfFormatTurtle)
}
