// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package testutil

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	"github.com/stretchr/testify/require"
)

// LoadAllTTLFromDir reads every .ttl file in the given directory,
// parses each into a temporary Stage, and merges them into a single
// reference Stage that is returned to the caller.
// Carriage-return characters are normalised to plain LF so that files
// with Windows line endings do not confuse the Turtle parser.
func LoadAllTTLFromDir(t *testing.T, dir string) sst.Stage {
	t.Helper()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	refStage := sst.OpenStage(sst.DefaultTriplexMode)

	var loaded int
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".ttl") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
		data = bytes.ReplaceAll(data, []byte("\r"), []byte("\n"))

		tempStage, err := sst.RdfRead(
			bufio.NewReader(bytes.NewReader(data)),
			sst.RdfFormatTurtle,
			sst.StrictHandler,
			sst.DefaultTriplexMode,
		)
		require.NoError(t, err, "failed to parse %s", path)

		_, err = refStage.MoveAndMerge(context.Background(), tempStage)
		require.NoError(t, err, "failed to merge %s", path)
		loaded++
	}

	t.Logf("Loaded %d TTL files into reference stage", loaded)
	return refStage
}

// LoadTriG reads a TriG file from the given path and returns the Stage.
func LoadTriG(t *testing.T, path string) sst.Stage {
	t.Helper()

	f, err := os.Open(path)
	require.NoError(t, err, "open file failed: %v", err)
	defer f.Close()

	data, err := os.ReadFile(path)
	require.NoError(t, err, "read trig file failed: %v", err)
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	data = bytes.ReplaceAll(data, []byte("\r"), []byte("\n"))

	stage, err := sst.RdfRead(bufio.NewReader(bytes.NewReader(data)), sst.RdfFormatTriG, sst.StrictHandler, sst.DefaultTriplexMode)
	require.NoError(t, err, "read trig failed: %v", err)

	return stage
}
