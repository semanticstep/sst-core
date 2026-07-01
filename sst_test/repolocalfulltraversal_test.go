// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"testing"
	"time"

	"github.com/semanticstep/sst-core/sst"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"go.etcd.io/bbolt"
)

func TestTraversal(t *testing.T) {
	// t.Skip("for manually using")
	hashNil := sst.HashNil()
	fmt.Printf("HashNil %s\n", hashNil)
	db, err := bbolt.Open(filepath.Join("testdata/Test_RemoteRepository_CheckoutRevision_BasicServer/bbolt.db"), 0600, &bbolt.Options{ReadOnly: true})
	if err != nil {
		fmt.Println(err)
		return
	}
	defer db.Close()

	err = db.View(func(tx *bbolt.Tx) error {
		return tx.ForEach(func(name []byte, b *bbolt.Bucket) error {
			fmt.Printf("Bucket: %s\n", name)
			return printBucket(b, 0, name)
		})
	})

	if err != nil {
		log.Fatal(err)
	}
}

// printBucket print bucket recursively, and for different bucket,
// it can print in different format.
func printBucket(b *bbolt.Bucket, level int, bucketName []byte) error {
	indent := ""
	for i := 0; i < level; i++ {
		indent += "  "
	}

	err := b.ForEach(func(k, v []byte) error {
		if v == nil {
			// when v is nil, k is a sub-bucket
			if bytes.Equal(bucketName, []byte("c")) {
				fmt.Printf("%scommit SHA:[%s] (subbucket)\n", indent, sst.Hash(k))
			} else if bytes.Equal(bucketName, []byte("dsr")) {
				fmt.Printf("%sDS-SHA:[%s] (subbucket)\n", indent, sst.Hash(k))
			} else {
				fmt.Printf("%s[%x] (subbucket:%s)\n", indent, k, bucketName)
			}
			subBucket := b.Bucket(k)
			if subBucket == nil {
				return fmt.Errorf("expected bucket %s not found", k)
			}
			return printBucket(subBucket, level+1, bucketName)
		} else {
			if string(bucketName) == "dsr" {
				if len(k) == 1 && k[0] == '\x00' {
					fmt.Printf("%sdefault NG-SHA: %s\n", indent, sst.Hash(v))
				} else if len(k) > 0 && k[0] == '\x00' {
					fmt.Printf("%simported DS-UUID:%x\n", indent, k[1:])
					fmt.Printf("%simported DS-SHA :%s\n", indent, sst.Hash(v))
				} else if len(k) > 0 && k[0] == '\x01' {
					fmt.Printf("%sNG-UUID:%x\n", indent, k[1:])
					fmt.Printf("%sNG-SHA :%s\n", indent, sst.Hash(v))
				} else if len(k) == 1 && k[0] == '\x02' {
					fmt.Printf("%screated commitHash :%s\n", indent, sst.Hash(v))
				} else {
					fmt.Printf("%sKey: %x, Value: %x\n", indent, k[1:], v)
				}
			} else if string(bucketName) == "ngr" {
				fmt.Printf("%sNG-SHA: %s\n", indent, sst.Hash(k))
				bufferV := bytes.NewBuffer(v)
				reader := bufio.NewReader(bufferV)
				graph, err := sst.SstRead(reader, sst.DefaultTriplexMode)
				if err == nil {
					var bufferW bytes.Buffer
					ioWriter := io.Writer(&bufferW)
					// writer := newTripleWriter(ioWriter, RdfFormatTurtle)
					// err = toWriter(graph, writer)
					// if err != nil {
					// 	log.Panic("ToWriter failed。")
					// }
					err = graph.RdfWrite(ioWriter, sst.RdfFormatTurtle)
					if err != nil {
						log.Panic("RdfWrite failed.")
					}
					data := bufferW.Bytes()
					fmt.Printf("%sValue: %s\n", indent, data)
				} else {
					log.Panic("SST Read failed.")
				}

			} else if string(bucketName) == "c" {
				if len(k) > 0 && k[0] == '\x00' {
					if len(v[32:]) != 0 {
						fmt.Printf("%scommitted _DS-UUID_: %x \n%scommitted _DS-SHA_: %s \n%sparent _Commit-SHA_: %s\n", indent, k[1:], indent+indent, sst.Hash(v[:32]), indent+indent, sst.Hash(v[32:]))
					} else {
						fmt.Printf("%scommitted _DS-UUID_: %x \n%scommitted _DS-SHA_: %s \n%sparent _Commit-SHA_: %x\n", indent, k[1:], indent+indent, sst.Hash(v[:32]), indent+indent, v[32:])
					}
				} else if bytes.Equal(k, []byte("author")) {
					var unixSeconds int64
					buf := bytes.NewBuffer(v[len(v)-8:])
					err := binary.Read(buf, binary.BigEndian, &unixSeconds)
					timeResult := time.Unix(unixSeconds, 0).UTC().Format(time.RFC3339)
					if err != nil {
						fmt.Println("binary.Read failed:", err)
					}
					fmt.Printf("%sKey: %s, Value: %s %s\n", indent, k, v[:len(v)-8], timeResult)
				} else if bytes.Equal(k, []byte("message")) {
					fmt.Printf("%sKey: %s, Value: %s\n", indent, k, v)
				} else if bytes.Equal(k, []byte("reason")) {
					fmt.Printf("%sKey: %s, Value: %s\n", indent, k, v)
				} else {
					fmt.Printf("%sKey: %s, Value: %x\n", indent, k, v)
				}
			} else if string(bucketName) == "ds" {
				if len(k) > 0 && k[0] == '\x00' {
					fmt.Printf("%sbranch: %s latest _Commit_SHA: %s\n", indent, k[1:], sst.Hash(v))
				} else if len(k) > 0 && k[0] == '\x01' {
					fmt.Printf("%stag name: %s latest _Commit_SHA: %s\n", indent, k[1:], sst.Hash(v))
				} else if len(k) > 0 && k[0] == '\x02' {
					fmt.Printf("%s_Commit_SHA_: %s latest _Commit_SHA: %s\n", indent, sst.Hash(k[1:]), sst.Hash(v))
				} else {
					fmt.Printf("%sKey: %s, Value: %x\n", indent, k, v)
				}
			} else {
				// log bucket stores just strings, so print directly
				fmt.Printf("%sKey: %s, Value: %s\n", indent, k, v)
			}
		}
		return nil
	})
	return err
}
