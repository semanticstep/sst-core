// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package utils // replace with your actual package name

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/semanticstep/sst-core/sst"
)

// Read the raw SST bytes of a NamedGraphRevision (NGR)
func SstRead(ctx context.Context, repo sst.Repository, h sst.Hash) ([]byte, error) {
	var buf bytes.Buffer
	if err := repo.ExtractSstFile(ctx, h, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Compute diff triples between two NGRs
func SstDiffTriples(ctx context.Context, repo sst.Repository, oldH, newH sst.Hash, normalize bool) ([]sst.DiffTriple, error) {
	oldBytes, err := SstRead(ctx, repo, oldH)
	if err != nil {
		return nil, fmt.Errorf("load old SST: %w", err)
	}
	newBytes, err := SstRead(ctx, repo, newH)
	if err != nil {
		return nil, fmt.Errorf("load new SST: %w", err)
	}
	tris, err := sst.SstWriteDiff(
		bufio.NewReader(bytes.NewReader(oldBytes)),
		bufio.NewReader(bytes.NewReader(newBytes)),
		io.Discard,
		normalize,
	)
	if err != nil {
		return nil, fmt.Errorf("compute diff: %w", err)
	}
	return tris, nil
}

// Print a list of diff triples
func PrintDiffTriples(tris []sst.DiffTriple) {
	if len(tris) == 0 {
		fmt.Println("  - No differences found.")
		return
	}
	for _, dt := range tris {
		fmt.Printf("  - %s %s %s %s\n",
			FlagToSymbol(dt.Flag), dt.Sub, dt.Pred, dt.Obj,
		)
	}
}

// flagToSymbol returns a visual symbol for a DiffFlag
func FlagToSymbol(flag sst.DiffTripleFlag) string {
	switch flag {
	case sst.TripleAdded:
		return "+"
	case sst.TripleRemoved:
		return "-"
	default:
		return "?"
	}
}
