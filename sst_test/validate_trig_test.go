// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"fmt"
	"testing"

	"github.com/semanticstep/sst-core/sst_test/testutil"
	"github.com/semanticstep/sst-core/tools/validate"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateTrigWithRefStage(t *testing.T) {
	// Load the TriG file to be validated.
	validationStage := testutil.LoadTriG(t, "testdata/AS1-IN-203.trig")

	// Load exported TTL files as reference data.
	refStage := testutil.LoadAllTTLFromDir(t, "exported_ttl")

	// Validate the TriG data against the reference data.
	report, err := validate.ValidateAll(validationStage, refStage)
	require.NoError(t, err)
	require.NotNil(t, report)

	// Count errors and warnings.
	var errCount, warnCount int
	for _, findings := range report.Findings {
		for _, f := range findings {
			if f.Level == "error" {
				errCount++
			} else if f.Level == "warning" {
				warnCount++
			}
		}
	}

	fmt.Printf("Validation Report:\n")
	fmt.Printf("  Passed: %v\n", report.Passed)
	fmt.Printf("  Total Errors: %d\n", errCount)
	fmt.Printf("  Total Warnings: %d\n", warnCount)

	for graphIRI, findings := range report.Findings {
		if len(findings) == 0 {
			continue
		}
		t.Logf("Graph: %s (%d findings)", graphIRI, len(findings))
		for _, f := range findings {
			t.Logf("  [%s/%s] %s", f.Kind, f.Level, f.Message)
		}
	}

	// Basic assertions.
	assert.NotNil(t, report.Findings)
}
