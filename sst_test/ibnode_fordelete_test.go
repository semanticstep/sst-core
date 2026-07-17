// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"reflect"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// Total triple count is 3, after delete one "comment" triple, reserved triple count is 2.
// Check reserved triple count and if "comment" triple still there or not.
func Test_ibNode_ForDelete(t *testing.T) {
	graph := loadTestData()
	tripleCount := 0
	err := graph.ForAllIBNodes(func(s sst.IBNode) error {
		err := s.ForAll(func(_ int, ts, _ sst.IBNode, _ sst.Term) error {
			if s == ts {
				tripleCount++
			}
			return nil
		})
		return err
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Graph triple count: %d\n", tripleCount)

	err = graph.ForAllIBNodes(func(s sst.IBNode) error {
		s.ForDelete(func(_ int, s, p sst.IBNode, o sst.Term) bool {
			if p.Fragment() == "comment" {
				return true
			} else {
				return false
			}
		})
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("after deleted")

	tripleCount = 0
	foundedDeleteTriple := false
	err = graph.ForAllIBNodes(func(s sst.IBNode) error {
		err := s.ForAll(func(_ int, ts, p sst.IBNode, _ sst.Term) error {
			if s == ts {
				tripleCount++
				if p.Fragment() == "comment" {
					foundedDeleteTriple = true
				}
			}
			return nil
		})
		return err
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Graph triple count: %d\n", tripleCount)

	assert.Equal(t, 2, tripleCount)
	assert.Equal(t, false, foundedDeleteTriple)
}

// If the IBNode is invaild(deleted),
// ForDelete function should return ErrNodeDeletedOrInvalid
func Test_ibNode_ForDelete_NodeNotValid(t *testing.T) {
	// ErrNodeDeletedOrInvalid := errors.New("node deleted or invalid")
	defer func() {
		if r := recover(); r != nil {
			expected := "node deleted or invalid"
			if r.(error).Error() != expected {
				t.Errorf("expected panic with %v, but got %v", expected, r)
			} else {
				// fmt.Println(tt.name, "pass")
			}
		} else {
			t.Errorf("expected panic, but function did not panic")
		}
	}()

	graph := loadTestData()
	tripleCount := 0
	err := graph.ForAllIBNodes(func(s sst.IBNode) error {
		err := s.ForAll(func(_ int, ts, _ sst.IBNode, _ sst.Term) error {
			if s == ts {
				tripleCount++
			}
			return nil
		})
		return err
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Graph triple count: %d\n", tripleCount)

	tripleCount = 0

	err = graph.ForAllIBNodes(func(s sst.IBNode) error {
		s.Delete()
		s.ForDelete(func(_ int, s, p sst.IBNode, o sst.Term) bool {
			if p.Fragment() == "comment" {
				return true
			} else {
				return false
			}
		})
		return nil
	})

	assert.Equal(t, 0, tripleCount)
	// assert.Equal(t, err, ErrNodeDeletedOrInvalid)
}

func Test_ibNode_ForTermCollection(t *testing.T) {
	stage := sst.OpenStage(sst.DefaultTriplexMode)

	id, _ := uuid.NewRandom()
	graph := stage.CreateNamedGraph(sst.IRI(id.URN()))

	p1 := graph.CreateIRINode("", rep.Path)

	p2 := graph.CreateIRINode("", rep.Path)

	p3 := graph.CreateIRINode("", rep.Path)

	graph.CreateBlankNode()

	e1 := graph.CreateIRINode("", rep.Edge)

	e2 := graph.CreateIRINode("", rep.Edge)

	e3 := graph.CreateIRINode("", rep.Edge)

	co := graph.CreateCollection(e1, e2)

	co1 := graph.CreateCollection(e2, e3)

	p1.AddStatement(rep.ListOfEdges, co)
	p2.AddStatement(rep.ListOfEdges, co)
	p3.AddStatement(rep.ListOfEdges, co)
	p3.AddStatement(rep.ListOfEdges, co1)

	// print all TermCollections of graph
	termCollectionCount := 0
	assert.Equal(t, graph.TermCollectionCount(), 2)
	err := graph.ForTermCollection(func(t sst.TermCollection) error {
		fmt.Printf("ForTermCollections IBNode: %s\n", t.ID())
		termCollectionCount++
		t.ForMembers(func(e int, o sst.Term) {
			switch o.TermKind() {
			case sst.TermKindIBNode, sst.TermKindTermCollection:
				fmt.Printf("KindIBNode:   %s\n", o.(sst.IBNode).IRI())
			case sst.TermKindLiteral:
				fmt.Printf("KindLiteral:  %q^^%s\n", o.(sst.Literal), o.(sst.Literal).DataType().IRI())
			case sst.TermKindLiteralCollection:
				fmt.Printf("KindLiteralCollection:   %s\n", reflect.TypeOf(o))
			default:
				fmt.Printf("default:    %s\n", o)
			}
		})
		return nil
	})
	if err != nil {
		log.Panic(err)
	}

	assert.Equal(t, termCollectionCount, 2)
}

// load test data
func loadTestData() sst.NamedGraph {
	var errorCount int
	tooManyErrors := errors.New("too many errors")
	file, err := os.Open("testdata/new_ibnode_test_data.ttl")
	defer func() {
		e := file.Close()
		if err == nil {
			err = e
		}
	}()

	st, err := sst.RdfRead(bufio.NewReader(file), sst.RdfFormatTurtle, func(err error) error {
		log.Println(err)
		if errorCount > 50 {
			return tooManyErrors
		}
		errorCount++
		return nil
	}, sst.DefaultTriplexMode)
	if err != nil {
		log.Fatal(err)
	}
	return st.NamedGraph("http://example.org/test#")
}
