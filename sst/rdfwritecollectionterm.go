// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"fmt"
	"strings"
)

type literalCollectionTerm struct {
	rdfTypeLiteral
	collection literalCollection
	writer     *tripleWriter
}

func termFromLiteralCollection(collection literalCollection, writer *tripleWriter) (literalCollectionTerm, error) {
	m0Term, err := termFromLiteral(collection.Member(0))
	if err != nil {
		return literalCollectionTerm{}, err
	}
	return literalCollectionTerm{
		rdfTypeLiteral: m0Term,
		collection:     collection,
		writer:         writer,
	}, nil
}

func (c literalCollectionTerm) rdfSerialize(format RdfFormat) string {
	if format != RdfFormatTurtle && format != RdfFormatTriG {
		panic(fmt.Sprintf("unexpected format %v", format))
	}
	return c.turtleSerialize()
}

func (c literalCollectionTerm) String() string {
	return c.turtleSerialize()
}

func (c literalCollectionTerm) turtleSerialize() string {
	var serialized strings.Builder
	var lastNL int
	serialized.WriteRune('(')
	var err error
	c.collection.forInternalMembers(func(index int, l Literal) {
		mTerm, e := termFromLiteral(l)
		if e != nil {
			err = e
		}
		lastNL = serializeCollectionTerm(index, mTerm, c.writer, &serialized, lastNL)
	})
	if err != nil {
		return err.Error()
	}
	serialized.WriteString(" )")
	return serialized.String()
}

func (c literalCollectionTerm) rdfTermType() rdfTermType {
	return termLiteralCollection
}

type termCollectionTerm struct {
	rdfTypeBlank
	collection TermCollection
	skip       int
	wc         *writerContext
}

func termFromTermCollection(
	collection TermCollection,
	skip int,
	wc *writerContext,
) (termCollectionTerm, error) {
	b, _, err := toIRIOrBlankNode(collection.(IBNode), wc)
	if err != nil {
		return termCollectionTerm{}, err
	}
	if !wc.allowCollectionTerms {
		panic(wc)
	}
	return termCollectionTerm{
		rdfTypeBlank: b.(rdfTypeBlank),
		collection:   collection,
		skip:         skip,
		wc:           wc,
	}, nil
}

func (c termCollectionTerm) rdfSerialize(format RdfFormat) string {
	if format != RdfFormatTurtle && format != RdfFormatTriG {
		panic(fmt.Sprintf("unexpected format %v", format))
	}
	return c.turtleSerialize()
}

func (c termCollectionTerm) String() string {
	return c.turtleSerialize()
}

func (c termCollectionTerm) turtleSerialize() string {
	var serialized strings.Builder
	var lastNL int
	serialized.WriteRune('(')
	var err error
	c.collection.ForMembers(func(index int, o Term) {
		if index < c.skip {
			return
		}
		triples, e := toRdfObject(o, c.wc, make([]rdfTriple, 0, 1))
		if e != nil {
			err = e
		}
		mTerm := triples[0].Obj
		lastNL = serializeCollectionTerm(index-c.skip, mTerm, c.wc.writer, &serialized, lastNL)
	})
	if err != nil {
		return err.Error()
	}
	if serialized.Len() == 1 {
		serialized.WriteRune(')')
	} else {
		serialized.WriteString(" )")
	}
	return serialized.String()
}

func (c termCollectionTerm) rdfTermType() rdfTermType {
	return termCollection
}

func serializeCollectionTerm(
	index int,
	mTerm rdfTerm,
	twr *tripleWriter,
	serialized *strings.Builder,
	inLastNL int,
) (lastNL int) {
	lastNL = inLastNL
	serializedTerm := serializeTerm(mTerm, twr)
	if index == 0 || serialized.Len()+len(serializedTerm)-lastNL <= lineLen {
		serialized.WriteRune(' ')
	} else {
		serialized.WriteString("\n  ")
		lastNL = serialized.Len() - 1
	}
	serialized.WriteString(serializedTerm)
	return
}
