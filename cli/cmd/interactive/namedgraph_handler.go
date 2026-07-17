// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package interactive

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/semanticstep/sst-core/cli/cmd/utils"
	"github.com/semanticstep/sst-core/sst"
	ssttosvg "github.com/semanticstep/sst-core/tools/sst_to_svg"
	"github.com/google/uuid"
)

func handleListForIRINode(graphAlias string) {
	graph, exists := interactiveConfig.NamedGraphs[graphAlias]
	if !exists {
		fmt.Printf("Error: NamedGraph alias '%s' not found.\n", graphAlias)
		return
	}

	var lines []string
	err := graph.ForIRINodes(func(node sst.IBNode) error {
		lines = append(lines, fmt.Sprintf("- IRI: %s", node.IRI()))
		return nil
	})

	if err != nil {
		utils.PrintCLIProblem("list IRI nodes", err)
		return
	}

	if len(lines) == 0 {
		fmt.Printf("No IRINodes found in NamedGraph '%s'.\n", graphAlias)
		return
	}

	fmt.Printf("- IRINodes in NamedGraph '%s':\n", graphAlias)
	utils.PaginateOutput(lines, 20)
}

func handleListForAllIBNodes(graphAlias string) {
	graph, exists := interactiveConfig.NamedGraphs[graphAlias]
	if !exists {
		fmt.Printf("Error: NamedGraph alias '%s' not found.\n", graphAlias)
		return
	}

	var lines []string
	err := graph.ForAllIBNodes(func(node sst.IBNode) error {
		if node.IsBlankNode() {
			lines = append(lines, fmt.Sprintf("- ID: %s", node.ID()))
		} else {
			lines = append(lines, fmt.Sprintf("- IRI: %s", node.IRI()))
		}
		return nil
	})

	if err != nil {
		utils.PrintCLIProblem("list IB nodes", err)
		return
	}

	if len(lines) == 0 {
		fmt.Printf("No IBNodes found in NamedGraph '%s'.\n", graphAlias)
		return
	}

	fmt.Printf("- IBNodes in NamedGraph '%s':\n", graphAlias)
	utils.PaginateOutput(lines, 20)
}

func handleListForBlankNode(graphAlias string) {
	graph, exists := interactiveConfig.NamedGraphs[graphAlias]
	if !exists {
		fmt.Printf("Error: NamedGraph alias '%s' not found.\n", graphAlias)
		return
	}

	var lines []string
	err := graph.ForBlankNodes(func(node sst.IBNode) error {
		lines = append(lines, fmt.Sprintf("- ID: %s", node.ID()))
		return nil
	})

	if err != nil {
		utils.PrintCLIProblem("list blank nodes", err)
		return
	}

	if len(lines) == 0 {
		fmt.Printf("No BlankNodes found in NamedGraph '%s'.\n", graphAlias)
		return
	}

	fmt.Printf("- BlankNodes in NamedGraph '%s':\n", graphAlias)
	utils.PaginateOutput(lines, 20)
}

func handleGetIRINodeByFragment(graphAlias string, args []string) {
	graph, exists := interactiveConfig.NamedGraphs[graphAlias]
	if !exists {
		fmt.Printf("Error: NamedGraph alias '%s' not found.\n", graphAlias)
		return
	}

	if len(args) == 0 {
		fmt.Println("No fragment provided.")
		return
	}

	fragment := args[0]

	aliasResult, err := utils.GetAlias(args, "ibnode")
	if err != nil {
		fmt.Println(err)
		return
	}
	ibNodeAlias := aliasResult.Alias

	// Use defer to confirm alias generation only on success
	success := false
	defer func() {
		if success {
			aliasResult.Confirm()
		}
	}()

	if _, exists := interactiveConfig.IBNodes[ibNodeAlias]; exists {
		fmt.Printf("Error: IBNodes alias '%s' already exists.\n", ibNodeAlias)
		return
	}

	ibNode := graph.GetIRINodeByFragment(fragment)

	if ibNode == nil {
		fmt.Printf("No IRINode found in NamedGraph '%s' with fragment '%s'.\n", graphAlias, fragment)
		return
	}

	// Success! Set flag so defer will confirm
	success = true

	interactiveConfig.IBNodes[ibNodeAlias] = ibNode
	interactiveConfig.IBNodeAliases = append(interactiveConfig.IBNodeAliases, ibNodeAlias)

	utils.PrintIBNodeDetails(ibNodeAlias, ibNode)
}

func handleGetBlankNodeByFragment(graphAlias string, args []string) {
	graph, exists := interactiveConfig.NamedGraphs[graphAlias]
	if !exists {
		fmt.Printf("Error: NamedGraph alias '%s' not found.\n", graphAlias)
		return
	}

	if len(args) == 0 {
		fmt.Println("No fragment provided.")
		return
	}

	uuidStr := args[0]

	aliasResult, err := utils.GetAlias(args, "ibnode")
	if err != nil {
		fmt.Println(err)
		return
	}
	ibNodeAlias := aliasResult.Alias

	// Use defer to confirm alias generation only on success
	success := false
	defer func() {
		if success {
			aliasResult.Confirm()
		}
	}()

	if _, exists := interactiveConfig.IBNodes[ibNodeAlias]; exists {
		fmt.Printf("Error: IBNodes alias '%s' already exists.\n", ibNodeAlias)
		return
	}

	nodeUUID, err := uuid.Parse(uuidStr)
	if err != nil {
		fmt.Printf("Error: Invalid UUID format: %s\n", uuidStr)
		return
	}

	ibNode := graph.GetBlankNodeByID(nodeUUID)

	if ibNode == nil {
		fmt.Printf("Error: IBNode with UUID '%s' not found in NamedGraph '%s'.\n", uuidStr, graphAlias)
		return
	}

	// Success! Set flag so defer will confirm
	success = true

	interactiveConfig.IBNodes[ibNodeAlias] = ibNode
	interactiveConfig.IBNodeAliases = append(interactiveConfig.IBNodeAliases, ibNodeAlias)

	utils.PrintIBNodeDetails(ibNodeAlias, ibNode)
}

func handleRdfWrite(graphAlias string, args []string) {
	graph, exists := interactiveConfig.NamedGraphs[graphAlias]
	if !exists {
		fmt.Printf("Error: NamedGraph alias '%s' not found.\n", graphAlias)
		return
	}

	if len(args) == 0 {
		fmt.Println("No file path provided. Usage: graph.rdfwrite <filename>")
		return
	}

	fileName := args[0]
	fileName = utils.EnsureOutputExt(fileName, ".ttl")

	// Check if file exists
	if _, err := os.Stat(fileName); err == nil {
		// File exists, ask user whether to overwrite
		fmt.Printf("File '%s' already exists. Overwrite? (y/N): ", fileName)
		var input string
		fmt.Scanln(&input)
		if strings.ToLower(strings.TrimSpace(input)) != "y" {
			fmt.Println("Aborted.")
			return
		}
	} else if !os.IsNotExist(err) {
		// Other error while checking file
		fmt.Fprintf(os.Stderr, "Error checking file '%s': %v\n", fileName, err)
		return
	}

	// Create the output file
	f, err := os.Create(fileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create file '%s': %v\n", fileName, err)
		return
	}
	defer f.Close()

	var writeErr error
	utils.ShowLoadingIndicator(fmt.Sprintf("Writing RDF for '%s'", graphAlias), func() {
		writeErr = graph.RdfWrite(f, sst.RdfFormatTurtle)
	})

	if writeErr != nil {
		fmt.Fprintf(os.Stderr, "\nError writing RDF: %v\n", writeErr)
		return
	}

	fmt.Printf("RDF successfully written to %s\n", fileName)
}

func handleSstWrite(graphAlias string, args []string) {
	graph, exists := interactiveConfig.NamedGraphs[graphAlias]
	if !exists {
		fmt.Printf("Error: NamedGraph alias '%s' not found.\n", graphAlias)
		return
	}

	if len(args) == 0 {
		fmt.Println("No file path provided. Usage: <namedgraph>.sstwrite <filename>")
		return
	}

	fileName := args[0]
	fileName = utils.EnsureOutputExt(fileName, ".sst")

	if _, err := os.Stat(fileName); err == nil {
		fmt.Printf("File '%s' already exists. Overwrite? (y/N): ", fileName)
		var input string
		fmt.Scanln(&input)
		if strings.ToLower(strings.TrimSpace(input)) != "y" {
			fmt.Println("Aborted.")
			return
		}
	} else if !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error checking file '%s': %v\n", fileName, err)
		return
	}

	f, err := os.Create(fileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create file '%s': %v\n", fileName, err)
		return
	}
	defer f.Close()

	var writeErr error
	utils.ShowLoadingIndicator(fmt.Sprintf("Writing SST for '%s'", graphAlias), func() {
		writeErr = graph.SstWrite(f)
	})
	if writeErr != nil {
		fmt.Fprintf(os.Stderr, "\nError writing SST: %v\n", writeErr)
		return
	}

	fmt.Printf("SST successfully written to %s\n", fileName)
}

func handleTtl(alias string) {
	if _, ok := interactiveConfig.IBNodes[alias]; ok {
		handleIBNodettl(alias)
		return
	}

	if _, ok := interactiveConfig.NamedGraphs[alias]; ok {
		handleNamedgraphTtl(alias)
		return
	}

	fmt.Printf("Error: Alias '%s' not found in IBNodes or NamedGraphs.\n", alias)
}

func handleNamedgraphTtl(graphAlias string) {
	graph, exists := interactiveConfig.NamedGraphs[graphAlias]
	if !exists {
		fmt.Printf("Error: NamedGraph alias '%s' not found.\n", graphAlias)
		return
	}

	fmt.Printf("--- RDF Output for NamedGraph '%s' ---\n", graphAlias)

	if err := graph.RdfWrite(os.Stdout, sst.RdfFormatTurtle); err != nil {
		fmt.Fprintf(os.Stderr, "\nError writing RDF to console: %v\n", err)
		return
	}

	fmt.Println("\n--- End of RDF Output ---")
}

func handleNamedGraphExportSVG(graphAlias string, args []string) {
	graph, exists := interactiveConfig.NamedGraphs[graphAlias]
	if !exists {
		fmt.Printf("Error: NamedGraph alias '%s' not found.\n", graphAlias)
		return
	}
	if len(args) < 1 {
		fmt.Printf("Error: Usage: %s.exportsvg <file>\n", graphAlias)
		return
	}
	outPath := filepath.Clean(strings.TrimSpace(args[0]))
	if outPath == "" || outPath == "." {
		fmt.Println("Error: output path must be a non-empty file path.")
		return
	}
	outPath = utils.EnsureOutputExt(outPath, ".svg")

	if _, err := os.Stat(outPath); err == nil {
		fmt.Printf("File '%s' already exists. Overwrite? (y/N): ", outPath)
		var input string
		fmt.Scanln(&input)
		if strings.ToLower(strings.TrimSpace(input)) != "y" {
			fmt.Println("Aborted.")
			return
		}
	} else if !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error checking file '%s': %v\n", outPath, err)
		return
	}

	outDir := filepath.Dir(outPath)
	if outDir != "." && outDir != "" {
		if err := os.MkdirAll(outDir, os.ModePerm); err != nil {
			fmt.Fprintf(os.Stderr, "exportsvg: failed to create directory %q: %v\n", outDir, err)
			return
		}
	}

	f, err := os.Create(outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "exportsvg: failed to create %q: %v\n", outPath, err)
		return
	}
	defer func() { _ = f.Close() }()

	w := bufio.NewWriter(f)
	if err := ssttosvg.ConvertGraphToSVG(graph, w); err != nil {
		fmt.Fprintf(os.Stderr, "exportsvg: conversion failed: %v\n", err)
		return
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "exportsvg: failed to flush %q: %v\n", outPath, err)
		return
	}

	fmt.Printf("SVG successfully written to %s\n", outPath)
}
