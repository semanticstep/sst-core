// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package validate_test

import (
	"testing"

	"github.com/semanticstep/sst-core/sst_test/testutil"
	"github.com/semanticstep/sst-core/tools/validate"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAll(t *testing.T) {
	t.Run("validate data against reference stage", func(t *testing.T) {
		// Load all exported TTL files as reference data.
		refStage := testutil.LoadAllTTLFromDir(t, "../../sst_test/exported_ttl")

		// Load all TTL files to be validated.
		validationStage := testutil.LoadAllTTLFromDir(t, "../../sst_test/datatobevalidate")

		// Validate the data stage against the reference data.
		report, err := validate.ValidateAll(validationStage, refStage)
		require.NoError(t, err)
		require.NotNil(t, report)

		// Print the full report for inspection.
		t.Logf("ValidateAll report:\n%s", report.String())

		// TODO: adjust assertions based on actual validation results.
		assert.NotNil(t, report.Findings)
	})

	t.Run("valid data returns no findings", func(t *testing.T) {
		// TODO: create a stage with valid RDF data and validate it
	})

	t.Run("invalid data returns findings", func(t *testing.T) {
		// TODO: create a stage with invalid RDF data and validate it
	})

	t.Run("reference stages are merged correctly", func(t *testing.T) {
		// TODO: create a validation stage and reference stages, then validate
	})

	t.Run("nil reference stage is skipped", func(t *testing.T) {
		// TODO: pass nil reference stages and ensure no panic
	})

	t.Run("does not mutate input stages", func(t *testing.T) {
		refStage := testutil.LoadAllTTLFromDir(t, "../../sst_test/exported_ttl")
		validationStage := testutil.LoadAllTTLFromDir(t, "../../sst_test/datatobevalidate")

		// Record original local graph counts and IRIs.
		origValGraphs := validationStage.NamedGraphs()
		origRefGraphs := refStage.NamedGraphs()

		_, err := validate.ValidateAll(validationStage, refStage)
		require.NoError(t, err)

		// After validation the input stages must still contain exactly the
		// same local NamedGraphs as before.
		assert.Len(t, validationStage.NamedGraphs(), len(origValGraphs), "validationStage was mutated")
		assert.Len(t, refStage.NamedGraphs(), len(origRefGraphs), "referenceStage was mutated")

		valIRIs := make(map[string]struct{}, len(origValGraphs))
		for _, ng := range origValGraphs {
			valIRIs[string(ng.IRI())] = struct{}{}
		}
		for _, ng := range validationStage.NamedGraphs() {
			assert.Contains(t, valIRIs, string(ng.IRI()), "unexpected graph in validationStage")
		}

		refIRIs := make(map[string]struct{}, len(origRefGraphs))
		for _, ng := range origRefGraphs {
			refIRIs[string(ng.IRI())] = struct{}{}
		}
		for _, ng := range refStage.NamedGraphs() {
			assert.Contains(t, refIRIs, string(ng.IRI()), "unexpected graph in refStage")
		}
	})
}
