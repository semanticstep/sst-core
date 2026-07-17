// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

// Package main provides a tool to read and write NamedGraphRevisions from/to a bbolt database.
// The NamedGraphRevisions bucket (key: "ngr") contains versioned NamedGraphs where:
//   - key is the Hash key (32 bytes) of the corresponding standalone binary NG-SST file
//   - value is the binary NG-SST file content
//
// Usage examples:
//
//	# List all NGRs:
//	bboltngr repo.db list
//
//	# Get, process, and put back a single NGR:
//	bboltngr repo.db process <hash>
//
//	# Export all to directory:
//	bboltngr repo.db export ./backup/
//
//	# Import all from directory:
//	bboltngr repo.db import ./backup/
//
//	# Process all NGRs in the database:
//	bboltngr repo.db process-all
//
// To add your own data handling, modify the processNGRData() function below.
package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/semanticstep/sst-core/sst"
	"github.com/google/uuid"
	"go.etcd.io/bbolt"
)

var (
	keyNamedGraphRevisions = []byte("ngr")
	keyDatasetRevisions    = []byte("dsr")
	keyCommits             = []byte("c")
	keyDatasets            = []byte("ds")
)

// NGREntry represents a single NamedGraphRevision entry
type NGREntry struct {
	Hash sst.Hash // 32-byte hash (key)
	Data []byte   // Binary SST content (value)
}

// processNGRData is where you add your custom data handling logic.
// It reads the original NGR data using SstReadOld (old format) and writes
// back using SstWrite (new format). You can modify the NamedGraph between
// reading and writing.
//
// Input: original NGR data
// Output: processed NGR data (or nil to skip/delete)
// Return error to stop processing
func processNGRData(entry NGREntry) ([]byte, error) {
	// Step 1: Read the old format SST data
	reader := bufio.NewReader(bytes.NewReader(entry.Data))
	ng, err := sst.SstReadOld(reader, sst.DefaultTriplexMode)
	if err != nil {
		return nil, fmt.Errorf("failed to read old format SST: %w", err)
	}

	// Step 2: TODO - Add your data handling/modification here
	// Examples:
	// - Modify nodes, triples, literals in the NamedGraph
	// - Filter based on content
	// - Transform the data structure
	// - etc.
	//
	// Example: Get the stage and iterate over nodes
	// stage := ng.Stage()
	// for _, node := range stage.Nodes() {
	//     // Modify node...
	// }

	// Step 3: Write the data back in new format
	var buf bytes.Buffer
	if err := ng.SstWrite(&buf); err != nil {
		return nil, fmt.Errorf("failed to write new format SST: %w", err)
	}

	return buf.Bytes(), nil

	// Alternative: Return nil to delete this entry
	// return nil, nil
}

func main() {
	if len(os.Args) < 3 {
		printUsage()
		os.Exit(1)
	}

	dbPath := os.Args[1]
	command := os.Args[2]

	switch command {
	case "list", "ls":
		if err := listNGR(dbPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "get":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Usage: %s <db_path> get <hash> [-o output_file]\n", os.Args[0])
			os.Exit(1)
		}
		hashStr := os.Args[3]
		outputFile := "-" // default to stdout
		if len(os.Args) >= 6 && os.Args[4] == "-o" {
			outputFile = os.Args[5]
		}
		if err := getNGR(dbPath, hashStr, outputFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "put":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Usage: %s <db_path> put <hash> [-i input_file]\n", os.Args[0])
			os.Exit(1)
		}
		hashStr := os.Args[3]
		inputFile := "-" // default to stdin
		if len(os.Args) >= 6 && os.Args[4] == "-i" {
			inputFile = os.Args[5]
		}
		if err := putNGR(dbPath, hashStr, inputFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "process":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Usage: %s <db_path> process <hash>\n", os.Args[0])
			os.Exit(1)
		}
		hashStr := os.Args[3]
		if err := processSingleNGR(dbPath, hashStr); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "process-all":
		if err := processAllNGRs(dbPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "delete", "del", "rm":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Usage: %s <db_path> delete <hash>\n", os.Args[0])
			os.Exit(1)
		}
		hashStr := os.Args[3]
		if err := deleteNGR(dbPath, hashStr); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "export":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Usage: %s <db_path> export <output_dir>\n", os.Args[0])
			os.Exit(1)
		}
		outputDir := os.Args[3]
		if err := exportAllNGR(dbPath, outputDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "import":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Usage: %s <db_path> import <input_dir>\n", os.Args[0])
			os.Exit(1)
		}
		inputDir := os.Args[3]
		if err := importAllNGR(dbPath, inputDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s <db_path> <command> [args...]\n\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  list, ls                          List all NamedGraphRevisions")
	fmt.Fprintln(os.Stderr, "  get <hash> [-o file]              Get NGR to stdout (default) or file")
	fmt.Fprintln(os.Stderr, "  put <hash> [-i file]              Put NGR from stdin (default) or file")
	fmt.Fprintln(os.Stderr, "  process <hash>                    Process single NGR using processNGRData()")
	fmt.Fprintln(os.Stderr, "  process-all                       Process all NGRs using processNGRData()")
	fmt.Fprintln(os.Stderr, "  delete, del, rm <hash>            Delete an NGR entry")
	fmt.Fprintln(os.Stderr, "  export <dir>                      Export all NGRs to directory")
	fmt.Fprintln(os.Stderr, "  import <dir>                      Import all NGRs from directory")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Examples:")
	fmt.Fprintf(os.Stderr, "  %s repo.db list\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s repo.db get <hash> -o data.sst\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s repo.db put <hash> -i data.sst\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s repo.db process <hash>\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s repo.db process-all\n", os.Args[0])
}

// parseHash parses a hash string (Base58 encoded) into sst.Hash
func parseHash(hashStr string) (sst.Hash, error) {
	return sst.StringToHash(hashStr)
}

// listNGR lists all entries in the NamedGraphRevisions bucket
func listNGR(dbPath string) error {
	db, err := bbolt.Open(dbPath, 0o600, nil)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	return db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(keyNamedGraphRevisions)
		if bucket == nil {
			return fmt.Errorf("NamedGraphRevisions bucket not found")
		}

		fmt.Println("NamedGraphRevisions in database:")
		fmt.Println("================================")
		fmt.Printf("%-50s | %12s\n", "Hash", "Size")
		fmt.Println(string(make([]byte, 50)) + " | " + string(make([]byte, 12)))

		count := 0
		bucket.ForEach(func(k, v []byte) error {
			hash := sst.BytesToHash(k)
			fmt.Printf("%-50s | %12d\n", hash, len(v))
			count++
			return nil
		})
		fmt.Printf("\nTotal: %d entries\n", count)
		return nil
	})
}

// getNGR retrieves a single NamedGraphRevision by hash and writes it to stdout or file
func getNGR(dbPath, hashStr, outputFile string) error {
	hash, err := parseHash(hashStr)
	if err != nil {
		return fmt.Errorf("invalid hash: %w", err)
	}

	db, err := bbolt.Open(dbPath, 0o600, nil)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	var data []byte
	err = db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(keyNamedGraphRevisions)
		if bucket == nil {
			return fmt.Errorf("NamedGraphRevisions bucket not found")
		}

		data = bucket.Get(hash[:])
		if data == nil {
			return fmt.Errorf("NamedGraphRevision with hash %s not found", hash)
		}
		// Make a copy since bbolt data is only valid within transaction
		data = append([]byte(nil), data...)
		return nil
	})
	if err != nil {
		return err
	}

	if outputFile == "-" || outputFile == "" {
		_, err = os.Stdout.Write(data)
		if err != nil {
			return fmt.Errorf("failed to write to stdout: %w", err)
		}
	} else {
		if err := os.WriteFile(outputFile, data, 0o644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Wrote %d bytes to %s\n", len(data), outputFile)
	}

	return nil
}

// putNGR stores a single NamedGraphRevision from stdin or file into the database
func putNGR(dbPath, hashStr, inputFile string) error {
	hash, err := parseHash(hashStr)
	if err != nil {
		return fmt.Errorf("invalid hash: %w", err)
	}

	var data []byte
	if inputFile == "-" || inputFile == "" {
		data, err = readStdin()
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
	} else {
		data, err = os.ReadFile(inputFile)
		if err != nil {
			return fmt.Errorf("failed to read input file: %w", err)
		}
	}

	db, err := bbolt.Open(dbPath, 0o600, nil)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	return db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(keyNamedGraphRevisions)
		if err != nil {
			return fmt.Errorf("failed to create/get NamedGraphRevisions bucket: %w", err)
		}

		if err := bucket.Put(hash[:], data); err != nil {
			return fmt.Errorf("failed to put data: %w", err)
		}

		fmt.Fprintf(os.Stderr, "Stored %d bytes with hash %s\n", len(data), hash)
		return nil
	})
}

// processSingleNGR reads a single NGR, applies processNGRData(), and writes back
func processSingleNGR(dbPath, hashStr string) error {
	hash, err := parseHash(hashStr)
	if err != nil {
		return fmt.Errorf("invalid hash: %w", err)
	}

	db, err := bbolt.Open(dbPath, 0o600, nil)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	var entry NGREntry
	err = db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(keyNamedGraphRevisions)
		if bucket == nil {
			return fmt.Errorf("NamedGraphRevisions bucket not found")
		}

		data := bucket.Get(hash[:])
		if data == nil {
			return fmt.Errorf("NamedGraphRevision with hash %s not found", hash)
		}

		entry = NGREntry{
			Hash: hash,
			Data: append([]byte(nil), data...),
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Apply custom processing
	processedData, err := processNGRData(entry)
	if err != nil {
		return fmt.Errorf("processing failed: %w", err)
	}

	// Write back (or delete if processedData is nil)
	return db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(keyNamedGraphRevisions)
		if bucket == nil {
			return fmt.Errorf("NamedGraphRevisions bucket not found")
		}

		if processedData == nil {
			// Delete the entry
			if err := bucket.Delete(hash[:]); err != nil {
				return fmt.Errorf("failed to delete: %w", err)
			}
			fmt.Fprintf(os.Stderr, "Deleted NGR %s (processNGRData returned nil)\n", hash)
		} else {
			// Update the entry
			if err := bucket.Put(hash[:], processedData); err != nil {
				return fmt.Errorf("failed to put data: %w", err)
			}
			fmt.Fprintf(os.Stderr, "Processed and updated NGR %s (%d -> %d bytes)\n",
				hash, len(entry.Data), len(processedData))
		}
		return nil
	})
}

// processAllNGRs migrates data from old bbolt to a new localFullRepository.
// Workflow:
// 1. Open source bbolt
// 2. Go to ds bucket, find each dataset's branch master commit Hash
// 3. Go to commit bucket, find each dataset's commit DS-SHA
// 4. Go to dsr bucket, find each DS-SHA's default NamedGraph-SHA
// 5. Go to ngr bucket, get each NamedGraphRevision content (by SstReadOld)
// 6. Open Stage A and MoveAndMerge each NamedGraphRevision's stage into Stage A
// 7. Create a localFullRepository
// 8. Open Stage B from the repo
// 9. MoveAndMerge Stage A to Stage B
// 10. Commit Stage B to store all data into the new repository
func processAllNGRs(sourceDbPath string) error {
	ctx := context.Background()
	startTime := time.Now()

	// Step 1: Open source bbolt
	fmt.Fprintf(os.Stderr, "[1/10] Opening source database: %s\n", sourceDbPath)
	sourceDb, err := bbolt.Open(sourceDbPath, 0o600, nil)
	if err != nil {
		return fmt.Errorf("failed to open source database: %w", err)
	}
	defer sourceDb.Close()

	// First pass: count total datasets
	fmt.Fprintf(os.Stderr, "[2/10] Counting datasets...\n")
	totalDatasets := 0
	err = sourceDb.View(func(tx *bbolt.Tx) error {
		dsBucket := tx.Bucket(keyDatasets)
		if dsBucket == nil {
			return nil
		}
		return dsBucket.ForEach(func(dsUUID, _ []byte) error {
			dsSubBucket := dsBucket.Bucket(dsUUID)
			if dsSubBucket == nil {
				return nil
			}
			// Count only datasets with master/main branch
			masterKey := append([]byte{0x00}, []byte("master")...)
			if dsSubBucket.Get(masterKey) != nil {
				totalDatasets++
				return nil
			}
			mainKey := append([]byte{0x00}, []byte("main")...)
			if dsSubBucket.Get(mainKey) != nil {
				totalDatasets++
			}
			return nil
		})
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "       Found %d datasets to migrate\n", totalDatasets)

	// Step 6: Open Stage A (ephemeral stage to collect all data)
	fmt.Fprintf(os.Stderr, "[6/10] Creating Stage A for collecting data...\n")
	stageA := sst.OpenStage(sst.DefaultTriplexMode)

	// Track unique NamedGraph hashes to avoid duplicates
	processedNGHashes := make(map[sst.Hash]struct{})

	// Process all datasets from source
	fmt.Fprintf(os.Stderr, "[2-5/10] Processing datasets and collecting NamedGraphs...\n")
	processedDatasets := 0
	totalNGs := 0

	err = sourceDb.View(func(tx *bbolt.Tx) error {
		// Step 2: Go to ds bucket, find each dataset's branch master commit Hash
		dsBucket := tx.Bucket(keyDatasets)
		if dsBucket == nil {
			return fmt.Errorf("datasets bucket not found")
		}

		commitsBucket := tx.Bucket(keyCommits)
		if commitsBucket == nil {
			return fmt.Errorf("commits bucket not found")
		}

		dsrBucket := tx.Bucket(keyDatasetRevisions)
		if dsrBucket == nil {
			return fmt.Errorf("datasetRevisions bucket not found")
		}

		ngrBucket := tx.Bucket(keyNamedGraphRevisions)
		if ngrBucket == nil {
			return fmt.Errorf("namedGraphRevisions bucket not found")
		}

		// Iterate over all datasets
		return dsBucket.ForEach(func(dsUUID, _ []byte) error {
			dsSubBucket := dsBucket.Bucket(dsUUID)
			if dsSubBucket == nil {
				return nil
			}

			// Look for "master" branch (key is '\x00' + branchName)
			masterKey := append([]byte{0x00}, []byte("master")...)
			commitHashBytes := dsSubBucket.Get(masterKey)
			if commitHashBytes == nil {
				// Try "main" branch if no master
				mainKey := append([]byte{0x00}, []byte("main")...)
				commitHashBytes = dsSubBucket.Get(mainKey)
				if commitHashBytes == nil {
					return nil // Skip datasets without master/main branch
				}
			}
			commitHash := sst.BytesToHash(commitHashBytes)
			processedDatasets++

			percent := float64(processedDatasets) * 100.0 / float64(totalDatasets)
			fmt.Fprintf(os.Stderr, "  [%3.1f%%] Dataset %d/%d: %s (commit: %s)\n",
				percent, processedDatasets, totalDatasets, uuid.UUID(dsUUID), commitHash)

			// Step 3: Go to commit bucket, find each dataset's commit DS-SHA
			commitSubBucket := commitsBucket.Bucket(commitHash[:])
			if commitSubBucket == nil {
				fmt.Fprintf(os.Stderr, "         Warning: commit %s not found, skipping\n", commitHash)
				return nil
			}

			ngCount := 0
			// Iterate over dataset revisions in this commit
			// Key: '\x00' + DS-UUID, Value: DS-Revision-Hash + parent commit hashes
			err := commitSubBucket.ForEach(func(k, v []byte) error {
				if len(k) == 0 || k[0] != 0x00 {
					return nil // Skip non-dataset entries
				}
				if len(v) < 32 {
					return nil // Invalid value
				}
				dsRevisionHash := sst.BytesToHash(v[:32])

				// Step 4: Go to dsr bucket, find each DS-SHA's default NamedGraph-SHA
				dsrSubBucket := dsrBucket.Bucket(dsRevisionHash[:])
				if dsrSubBucket == nil {
					return nil
				}

				// Get default NG hash (key is "\x00")
				ngHashBytes := dsrSubBucket.Get([]byte{0x00})
				if ngHashBytes == nil || len(ngHashBytes) != 32 {
					return nil
				}
				ngHash := sst.BytesToHash(ngHashBytes)

				// Check if we already processed this NG
				if _, ok := processedNGHashes[ngHash]; ok {
					return nil
				}
				processedNGHashes[ngHash] = struct{}{}
				ngCount++
				totalNGs++

				// Step 5: Go to ngr bucket, get each NamedGraphRevision content
				ngContent := ngrBucket.Get(ngHash[:])
				if ngContent == nil {
					return fmt.Errorf("namedGraphRevision %s not found", ngHash)
				}

				// Read old format SST
				reader := bufio.NewReader(bytes.NewReader(ngContent))
				ng, err := sst.SstReadOld(reader, sst.DefaultTriplexMode)
				if err != nil {
					return fmt.Errorf("failed to read old format SST for %s: %w", ngHash, err)
				}

				// Step 6: MoveAndMerge each NamedGraphRevision's stage into Stage A
				_, err = stageA.MoveAndMerge(ctx, ng.Stage())
				if err != nil {
					return fmt.Errorf("failed to merge stage for %s: %w", ngHash, err)
				}

				if totalNGs%10 == 0 {
					fmt.Fprintf(os.Stderr, "         Collected %d NamedGraphs so far...\n", totalNGs)
				}
				return nil
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "         Added %d NamedGraphs from this dataset (total: %d)\n", ngCount, totalNGs)
			return nil
		})
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "\n[6/10] Stage A collected %d NamedGraphs from %d datasets\n", totalNGs, processedDatasets)

	// Step 7: Create a localFullRepository
	fmt.Fprintf(os.Stderr, "[7/10] Creating new repository...\n")
	repoDir := sourceDbPath + "_migrated"
	if err := os.RemoveAll(repoDir); err != nil {
		return fmt.Errorf("failed to clean up repo directory: %w", err)
	}

	repo, err := sst.CreateLocalRepository(repoDir, "migration@example.com", "Migration", true)
	if err != nil {
		return fmt.Errorf("failed to create repository: %w", err)
	}
	fmt.Fprintf(os.Stderr, "       Created repository at: %s\n", repoDir)

	// Step 8: Open Stage B from the repo
	fmt.Fprintf(os.Stderr, "[8/10] Opening Stage B from new repository...\n")
	stageB := repo.OpenStage(sst.DefaultTriplexMode)

	// Step 9: MoveAndMerge Stage A to Stage B
	fmt.Fprintf(os.Stderr, "[9/10] Merging Stage A into Stage B...\n")
	_, err = stageB.MoveAndMerge(ctx, stageA)
	if err != nil {
		return fmt.Errorf("failed to merge Stage A into Stage B: %w", err)
	}
	fmt.Fprintf(os.Stderr, "       Successfully merged %d NamedGraphs into Stage B\n", totalNGs)

	// Step 10: Commit Stage B to store all data into the new repository
	fmt.Fprintf(os.Stderr, "[10/10] Committing to new repository...\n")
	_, _, err = stageB.Commit(ctx, "Migrated from old format", "master")
	if err != nil {
		return fmt.Errorf("failed to commit Stage B: %w", err)
	}

	elapsed := time.Since(startTime)
	fmt.Fprintf(os.Stderr, "\n✓ Migration completed successfully in %s\n", elapsed)
	fmt.Fprintf(os.Stderr, "  Source:      %s\n", sourceDbPath)
	fmt.Fprintf(os.Stderr, "  Destination: %s\n", repoDir)
	fmt.Fprintf(os.Stderr, "  Datasets:    %d\n", processedDatasets)
	fmt.Fprintf(os.Stderr, "  NamedGraphs: %d\n", totalNGs)
	return nil
}

// deleteNGR removes a single NamedGraphRevision from the database
func deleteNGR(dbPath, hashStr string) error {
	hash, err := parseHash(hashStr)
	if err != nil {
		return fmt.Errorf("invalid hash: %w", err)
	}

	db, err := bbolt.Open(dbPath, 0o600, nil)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	return db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(keyNamedGraphRevisions)
		if bucket == nil {
			return fmt.Errorf("NamedGraphRevisions bucket not found")
		}

		if bucket.Get(hash[:]) == nil {
			return fmt.Errorf("NamedGraphRevision with hash %s not found", hash)
		}

		if err := bucket.Delete(hash[:]); err != nil {
			return fmt.Errorf("failed to delete: %w", err)
		}

		fmt.Fprintf(os.Stderr, "Deleted NGR with hash %s\n", hash)
		return nil
	})
}

// exportAllNGR exports all NamedGraphRevisions to a directory
func exportAllNGR(dbPath, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	db, err := bbolt.Open(dbPath, 0o600, nil)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	return db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(keyNamedGraphRevisions)
		if bucket == nil {
			return fmt.Errorf("NamedGraphRevisions bucket not found")
		}

		count := 0
		bucket.ForEach(func(k, v []byte) error {
			hash := sst.BytesToHash(k)
			outputFile := filepath.Join(outputDir, hash.String()+".sst")

			if err := os.WriteFile(outputFile, v, 0o644); err != nil {
				return fmt.Errorf("failed to write %s: %w", outputFile, err)
			}

			count++
			if count%100 == 0 {
				fmt.Fprintf(os.Stderr, "Exported %d entries...\n", count)
			}
			return nil
		})
		fmt.Fprintf(os.Stderr, "Exported %d NamedGraphRevisions to %s\n", count, outputDir)
		return nil
	})
}

// importAllNGR imports all NamedGraphRevisions from a directory
func importAllNGR(dbPath, inputDir string) error {
	db, err := bbolt.Open(dbPath, 0o600, nil)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	entries, err := os.ReadDir(inputDir)
	if err != nil {
		return fmt.Errorf("failed to read input directory: %w", err)
	}

	return db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(keyNamedGraphRevisions)
		if err != nil {
			return fmt.Errorf("failed to create/get NamedGraphRevisions bucket: %w", err)
		}

		count := 0
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			name := entry.Name()
			// Expect files named as <hash>.sst (hash is 44 chars in Base58)
			if len(name) < 5 || name[len(name)-4:] != ".sst" {
				continue
			}

			hashStr := name[:len(name)-4]
			hash, err := sst.StringToHash(hashStr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Skipping %s: invalid hash\n", name)
				continue
			}

			inputFile := filepath.Join(inputDir, name)
			data, err := os.ReadFile(inputFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Skipping %s: %v\n", name, err)
				continue
			}

			if err := bucket.Put(hash[:], data); err != nil {
				return fmt.Errorf("failed to put %s: %w", name, err)
			}

			count++
			if count%100 == 0 {
				fmt.Fprintf(os.Stderr, "Imported %d entries...\n", count)
			}
		}

		fmt.Fprintf(os.Stderr, "Imported %d NamedGraphRevisions from %s\n", count, inputDir)
		return nil
	})
}

// Helper functions

func readStdin() ([]byte, error) {
	var result []byte
	buf := make([]byte, 4096)
	for {
		n, err := os.Stdin.Read(buf)
		if n > 0 {
			result = append(result, buf[:n]...)
		}
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, err
		}
	}
	return result, nil
}
