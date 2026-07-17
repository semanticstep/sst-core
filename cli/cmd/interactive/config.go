// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package interactive

import (
	"context"

	"github.com/semanticstep/sst-core/sst"
)

type InteractiveConfig struct {
	// --- Repositories ---
	Repositories        map[string]sst.Repository // repoAlias -> Repository
	RepositoryLocations map[string]string         // repoAlias -> local path or remote URL
	RepositoryTypes     map[string]string         // repoAlias -> "local" | "localflat" | "remote"
	RepositoryAliases   []string

	// --- SuperRepositories ---
	SuperRepositories        map[string]sst.SuperRepository // superRepoAlias -> SuperRepository
	SuperRepositoryLocations map[string]string              // superRepoAlias -> local path or remote URL
	SuperRepositoryTypes     map[string]string              // superRepoAlias -> "local" | "remote"
	SuperRepositoryAliases   []string

	// --- Datasets ---
	Datasets       map[string]sst.Dataset
	DatasetAliases []string

	// --- Stages ---
	Stages        map[string]sst.Stage // stageAlias -> Stage
	StageBranches  map[string]string    // stageAlias -> branch name (e.g. "master")
	StageCommits   map[string]sst.Hash  // stageAlias -> commit hash
	StageRevisions map[string]sst.Hash  // stageAlias -> dataset revision hash
	StageSources   map[string]string    // stageAlias -> source file path (for rdfread/imported stages)
	StageAliases  []string

	// --- NamedGraphs ---
	NamedGraphs       map[string]sst.NamedGraph // namedgraphAlias -> NamedGraph
	NamedGraphAliases []string

	// --- IBNodes ---
	IBNodes       map[string]sst.IBNode // ibNodeAlias -> IBNode
	IBNodeAliases []string

	// --- Auth ---
	SuperAuthContexts map[string]context.Context        // superAlias -> AuthContext
	AuthContexts map[sst.Repository]context.Context // repo -> AuthContext
}

var interactiveConfig = &InteractiveConfig{
	Repositories:        make(map[string]sst.Repository),
	RepositoryLocations: make(map[string]string),
	RepositoryTypes:     make(map[string]string),

	SuperRepositories:        make(map[string]sst.SuperRepository),
	SuperRepositoryLocations: make(map[string]string),
	SuperRepositoryTypes:     make(map[string]string),

	Datasets: make(map[string]sst.Dataset),

	Stages:        make(map[string]sst.Stage),
	StageBranches:  make(map[string]string),
	StageCommits:   make(map[string]sst.Hash),
	StageRevisions: make(map[string]sst.Hash),
	StageSources:   make(map[string]string),

	NamedGraphs: make(map[string]sst.NamedGraph),

	IBNodes: make(map[string]sst.IBNode),

	SuperAuthContexts: make(map[string]context.Context),
	AuthContexts: make(map[sst.Repository]context.Context),
}
