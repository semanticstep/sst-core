// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/semanticstep/sst-core/sst"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
)

func Test_StaticDictionary_Repository(t *testing.T) {
	dict := sst.StaticDictionary()
	require.NotNil(t, dict)

	repo := dict.Repository()
	require.NotNil(t, repo, "dictionary stage should be linked to a repository")

	info, err := repo.Info(context.TODO(), "")
	require.NoError(t, err)

	assert.NotEmpty(t, info.VersionHash, "dictionary repository should have a version hash")
	t.Logf("Dictionary VersionHash: %s", info.VersionHash)
}
