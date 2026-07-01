// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"fmt"
	"strings"
)

type inlineBlankNodeTerm struct {
	rdfTypeBlank
	node          IBNode
	writercontext *writerContext
}

func termFromInlineBlankNode(node IBNode, writercontext *writerContext) (inlineBlankNodeTerm, error) {
	b, _, err := toIRIOrBlankNode(node, writercontext)
	if err != nil {
		return inlineBlankNodeTerm{}, err
	}
	if !writercontext.allowCollectionTerms {
		panic(writercontext)
	}
	return inlineBlankNodeTerm{
		rdfTypeBlank:  b.(rdfTypeBlank),
		node:          node,
		writercontext: writercontext,
	}, nil
}

func (c inlineBlankNodeTerm) rdfSerialize(format RdfFormat) string {
	if format != RdfFormatTurtle && format != RdfFormatTriG {
		panic(fmt.Sprintf("unexpected format %v", format))
	}
	return c.turtleSerialize()
}

func (c inlineBlankNodeTerm) String() string {
	return c.turtleSerialize()
}

func (c inlineBlankNodeTerm) turtleSerialize() string {
	var serialized strings.Builder
	var lastNL int
	serialized.WriteRune('[')
	var prevP rdfTypePredicate
	err := c.node.ForAll(func(index int, sub, pred IBNode, obj Term) error {
		if c.node == sub {
			triples, err := nodeTripleToRdfTriples(index, sub, pred, obj, c.writercontext, make([]rdfTriple, 0, 1))
			if err != nil {
				return err
			}
			for _, triple := range triples {
				if triple.Pred != prevP {
					if serialized.Len() > 1 {
						serialized.WriteString(" ;\n  ")
					} else {
						serialized.WriteString("\n  ")
					}
					lastNL = serialized.Len() - 1
					prevP = triple.Pred
					serialized.WriteString(serializeTerm(triple.Pred, c.writercontext.writer))
					serialized.WriteString(" ")
					serialized.WriteString(serializeTerm(triple.Obj, c.writercontext.writer))
				} else {
					serializedObj := serializeTerm(triple.Obj, c.writercontext.writer)
					if serialized.Len()+len(serializedObj)-lastNL <= lineLen {
						serialized.WriteString(", ")
					} else {
						serialized.WriteString(",\n    ")
						lastNL = serialized.Len() - 2
					}
					serialized.WriteString(serializeTerm(triple.Obj, c.writercontext.writer))
				}
			}
		}
		return nil
	})
	if err != nil {
		return err.Error()
	}
	if serialized.Len() == 1 {
		serialized.WriteRune(']')
	} else {
		serialized.WriteString("\n]")
	}
	return serialized.String()
}

func (c inlineBlankNodeTerm) rdfTermType() rdfTermType {
	return termInlineBlankNode
}
