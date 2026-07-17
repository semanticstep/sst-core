// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package separatedkeyword

import (
	separated "github.com/semanticstep/sst-core/defaultderive/tokenizerseparated"
	"github.com/blevesearch/bleve/v2/analysis"
	"github.com/blevesearch/bleve/v2/analysis/token/lowercase"
	"github.com/blevesearch/bleve/v2/registry"
)

const Name = "sepratedkeyword"

func AnalyzerConstructor(config map[string]interface{}, cache *registry.Cache) (*analysis.Analyzer, error) {
	separatedKeywordTokenizer, err := cache.TokenizerNamed(separated.Name)
	if err != nil {
		return nil, err
	}
	toLowerFilter, err := cache.TokenFilterNamed(lowercase.Name)
	if err != nil {
		return nil, err
	}
	return &analysis.Analyzer{
		Tokenizer:    separatedKeywordTokenizer,
		TokenFilters: []analysis.TokenFilter{toLowerFilter},
	}, nil
}

func init() {
	registry.RegisterAnalyzer(Name, AnalyzerConstructor)
}
