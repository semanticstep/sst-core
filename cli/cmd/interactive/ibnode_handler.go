// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package interactive

import (
	"fmt"

	"github.com/semanticstep/sst-core/cli/cmd/utils"
	"github.com/semanticstep/sst-core/sst"
)

func handleForAllTriples(alias string) {
	ibNode, exists := interactiveConfig.IBNodes[alias]
	if !exists {
		fmt.Printf("Error: IBNode alias '%s' not found.\n", alias)
		return
	}

	err := ibNode.ForAll(func(index int, subject sst.IBNode, predicate sst.IBNode, object sst.Term) error {
		switch object.TermKind() {
		case sst.TermKindIBNode, sst.TermKindTermCollection:
			ibObj := object.(sst.IBNode)
			fmt.Printf("- %s, %s, %s\n", subject.IRI(), predicate.IRI(), ibObj.IRI())
		case sst.TermKindLiteral:
			fmt.Printf("- %s \"%v\"^^%s\n", predicate.IRI(), object.(sst.Literal), object.(sst.Literal).DataType().IRI())
		case sst.TermKindLiteralCollection:
			fmt.Printf("- %s, %s, %v\n", subject.IRI(), predicate.IRI(), object.(sst.LiteralCollection))
		default:
			fmt.Printf("- %s, %s, %v\n", subject.IRI(), predicate.IRI(), object)
		}
		return nil
	})

	if err != nil {
		fmt.Println(utils.ExplainCLIError("iterate triples", err))
	}
}

func handleIBNodettl(alias string) {
	ibNode, ok := interactiveConfig.IBNodes[alias]
	if !ok {
		fmt.Printf("Error: IBNode alias '%s' not found.\n", alias)
		return
	}

	triples := ibNode.DumpTriples()
	for _, t := range triples {
		fmt.Printf("%s %s %s .\n", t[0], t[1], t[2])
	}
}
