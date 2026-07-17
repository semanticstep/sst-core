// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/semanticstep/sst-core/sst"
	_ "github.com/semanticstep/sst-core/vocabularies/dict" // Register vocabularies for pretty TTL output
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/owl"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/google/uuid"
)

func Test_ExampleReadTTLAndWrite(t *testing.T) {
	tmpDir := t.TempDir()
	ttlFile := filepath.Join(tmpDir, "example.ttl")
	sstFile := filepath.Join(tmpDir, "example.sst")
	writeAfterReadFile := filepath.Join(tmpDir, "examplewriteafterread.sst")

	t.Run("write", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph("")
		ng2 := st.CreateNamedGraph("")
		ng.AddImport(ng2)

		NgNode := ng.GetIRINodeByFragment("")
		NgNode.AddStatement(owl.VersionInfo, sst.LangString{Val: "final", LangTag: "en"})

		myIndividual := ng.CreateIRINode(string("main"))
		myIndividual.AddStatement(rdf.Type, lci.Individual)
		myIndividual.AddStatement(rdfs.Label, sst.String("hello world"))

		second := ng.CreateIRINode("", lci.Individual)
		myIndividual.AddStatement(lci.HasPart, second)
		f, err := os.Create(ttlFile)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		err = ng.RdfWrite(f, sst.RdfFormatTurtle)

		if err != nil {
			log.Panic(err)
		}
		out, err := os.Create(sstFile)
		if err != nil {
			log.Panic(err)
		}
		defer func() {
			cErr := out.Close()
			if err == nil {
				err = cErr
			}
		}()
		err = ng.SstWrite(out)
		if err != nil {
			log.Panic(err)
		}

	})
	t.Run("readThenWriteAgain", func(t *testing.T) {
		file, err := os.Open(ttlFile)
		defer func() {
			e := file.Close()
			if err == nil {
				err = e
			}
		}()

		st, err := sst.RdfRead(bufio.NewReader(file), sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
		if err != nil {
			log.Panic(err)
		}
		graph := st.NamedGraphs()[0]
		fmt.Printf("Named graph nodes: %d\n", graph.IRINodeCount())
		// Uncomment the following line if you want to print loaded graph id
		// fmt.Fprintf(os.Stderr, "Named graph: %s\n", graph.ID())
		graph.ForIRINodes(func(t sst.IBNode) error {
			if t.Fragment() == "" {
				fmt.Printf("  IBNode: empty string\n")
			} else {
				fmt.Printf("  IBNode: %s\n", t.Fragment())
			}
			t.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
				if s != t { // skip inverses
					return nil
				}
				switch o.TermKind() {
				case sst.TermKindIBNode, sst.TermKindTermCollection:
					o := o.(sst.IBNode)
					fmt.Printf("    %s %s\n", p.IRI(), o.IRI())
				case sst.TermKindLiteral:
					o := o.(sst.Literal)
					fmt.Printf("    %s %q^^%s\n", p.IRI(), o, o.DataType().IRI())
				case sst.TermKindLiteralCollection:
					o := o.(sst.LiteralCollection)
					fmt.Printf("    %s %q^^%s\n", p.IRI(), o.Values(), o.Member(0).DataType().IRI())
				}
				return nil
			})
			return nil
		})
		out, err := os.Create(writeAfterReadFile)
		if err != nil {
			log.Panic(err)
		}
		defer func() {
			cErr := out.Close()
			if err == nil {
				err = cErr
			}
		}()
		err = graph.SstWrite(out)
		if err != nil {
			log.Panic(err)
		}

		//   Named graph nodes: 2
		//   IBNode: empty string
		//     http://www.w3.org/1999/02/22-rdf-syntax-ns#type http://www.w3.org/2002/07/owl#Ontology
		//   IBNode: main
		//     http://www.w3.org/1999/02/22-rdf-syntax-ns#type http://ontology.semanticstep.net/lci#Individual
		//     http://www.w3.org/2000/01/rdf-schema#label "hello world"^^http://www.w3.org/2001/XMLSchema#string
	})

	t.Run("readsstagain", func(t *testing.T) {
		in, err := os.Open(writeAfterReadFile)
		if err != nil {
			log.Panic(err)
		}
		defer in.Close()

		graph, err := sst.SstRead(bufio.NewReader(in), sst.DefaultTriplexMode)
		if err != nil {
			log.Panic(err)
		}

		NgNode := graph.GetIRINodeByFragment("")
		fmt.Println("Ng Node Fragment: ", NgNode.Fragment())
		fmt.Println("NgNode IRI: ", NgNode.IRI())

		fmt.Printf("Named graph nodes: %d\n", graph.IRINodeCount())
		// Uncomment the following line if you want to print loaded graph id
		// fmt.Fprintf(os.Stderr, "Named graph: %s\n", graph.ID())
		graph.ForIRINodes(func(t sst.IBNode) error {
			fmt.Printf("  IBNode: %s\n", t.Fragment())
			t.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
				if s != t { // skip inverses
					return nil
				}
				switch o.TermKind() {
				case sst.TermKindIBNode, sst.TermKindTermCollection:
					o := o.(sst.IBNode)
					fmt.Printf("    %s %s\n", p.IRI(), o.IRI())
				case sst.TermKindLiteral:
					o := o.(sst.Literal)
					fmt.Printf("    %s %q^^%s\n", p.IRI(), o, o.DataType().IRI())
				case sst.TermKindLiteralCollection:
					o := o.(sst.LiteralCollection)
					fmt.Printf("    %s %q^^%s\n", p.IRI(), o.Values(), o.Member(0).DataType().IRI())
				}
				return nil
			})
			return nil
		})
	})
}

func Test_NGNodeReadTTL(t *testing.T) {
	t.Run("write", func(t *testing.T) {
		file, err := os.Open("./testdata/descriptionNode.ttl")
		defer func() {
			e := file.Close()
			if err == nil {
				err = e
			}
		}()

		st, err := sst.RdfRead(bufio.NewReader(file), sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
		if err != nil {
			log.Panic(err)
		}
		graph := st.NamedGraphs()[0]
		fmt.Printf("Named graph nodes: %d\n", graph.IRINodeCount())
		// Uncomment the following line if you want to print loaded graph id
		// fmt.Fprintf(os.Stderr, "Named graph: %s\n", graph.ID())
		graph.ForIRINodes(func(t sst.IBNode) error {
			fmt.Printf("  IBNode: %s\n", t.Fragment())
			t.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
				if s != t { // skip inverses
					return nil
				}
				switch o.TermKind() {
				case sst.TermKindIBNode, sst.TermKindTermCollection:
					o := o.(sst.IBNode)
					fmt.Printf("    %s %s\n", p.IRI(), o.IRI())
				case sst.TermKindLiteral:
					o := o.(sst.Literal)
					fmt.Printf("    %s %q^^%s\n", p.IRI(), o, o.DataType().IRI())
				case sst.TermKindLiteralCollection:
					o := o.(sst.LiteralCollection)
					fmt.Printf("    %s %q^^%s\n", p.IRI(), o.Values(), o.Member(0).DataType().IRI())
				}
				return nil
			})
			return nil
		})

		ng2 := graph.Stage().CreateNamedGraph(sst.IRI(uuid.MustParse("8428fce0-2855-4ad0-ac05-d72960a0ad2c").URN()))
		if ng2 != nil {
			graph.AddImport(ng2)
		}

		f, err := os.Create(filepath.Join(t.TempDir(), "descriptionNodeOut.ttl"))
		if err != nil {
			panic(err)
		}
		defer f.Close()
		err = graph.RdfWrite(f, sst.RdfFormatTurtle)

		if err != nil {
			log.Panic(err)
		}
		// Output:
		// Named graph nodes: 2
		//   IBNode: .
		//   IBNode: main
		//     http://www.w3.org/1999/02/22-rdf-syntax-ns#type http://ontology.semanticstep.net/lci#Individual
		//     http://www.w3.org/2000/01/rdf-schema#label "hello world"^^http://www.w3.org/2001/XMLSchema#string
		//     http://www.w3.org/2000/01/rdf-schema#comment ["comment line 1" "comment line 2"]^^http://www.w3.org/2001/XMLSchema#string
	})

}

func Test_NGNode(t *testing.T) {

	t.Run("write", func(t *testing.T) {
		p1 := sst.Element{
			Vocabulary: sst.Vocabulary{BaseIRI: "http://ontology.semanticstep.net/stuff#"},
			Name:       "p1",
		}
		graph := sst.OpenStage(sst.DefaultTriplexMode).CreateNamedGraph(sst.IRI(uuid.MustParse("50643e1b-a652-4a7a-9b52-a91c0425ab1a").URN()))
		n1 := graph.CreateIRINode("node1")
		n2 := graph.CreateIRINode("node2")
		col := graph.CreateCollection(n2)

		n1.AddStatement(p1, col)

		f, err := os.Create(filepath.Join(t.TempDir(), "NGNode.ttl"))
		if err != nil {
			panic(err)
		}
		defer f.Close()
		err = graph.RdfWrite(f, sst.RdfFormatTurtle)

		if err != nil {
			log.Panic(err)
		}
	})

}

func TestUnixSeconds_RoundTrip_UTC(t *testing.T) {
	// Repeat a few times to reduce the chance of crossing a second boundary at an unlucky moment.
	for i := 0; i < 10; i++ {
		timeNow := time.Now()
		unixSec := timeNow.Unix() // seconds since epoch (POSIX)
		utc := timeNow.UTC()
		unixBack := utc.Unix()

		if unixBack != unixSec {
			t.Fatalf("round-trip mismatch: unixSec=%d utc=%s unixBack=%d",
				unixSec, utc.Format(time.RFC3339Nano), unixBack)
		}
	}
}

func TestUnixSeconds_KnownPoints(t *testing.T) {
	cases := []struct {
		name string
		unix int64
	}{
		{"epoch", 0},
		{"one_second_after_epoch", 1},
		{"leap_day_2000_UTC", time.Date(2000, 2, 29, 0, 0, 0, 0, time.UTC).Unix()},
		{"recent_fixed", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Unix()},
		{"2038_boundary", 2147483647}, // 2038-01-19T03:14:07Z
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			utc := time.Unix(c.unix, 0).UTC()
			back := utc.Unix()
			if back != c.unix {
				t.Fatalf("mismatch: unix=%d utc=%s back=%d",
					c.unix, utc.Format(time.RFC3339Nano), back)
			}
			// Optional: also verify that formatting keeps Z and doesn't introduce local time offset
			if utc.Location() != time.UTC {
				t.Fatalf("expected UTC location, got %v", utc.Location())
			}
		})
	}
}
