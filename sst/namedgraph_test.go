// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNewLocalNamedGraph(t *testing.T) {
	baseIRI := "http://test.example.com/"
	newUUID := uuid.NewSHA1(uuid.NameSpaceURL, ([]byte)(baseIRI))
	exampleGraph := namedGraph{
		baseIRI: baseIRI,
		flags: namedGraphFlags{
			isReferenced: false,
			modified:     true,
		},
		stage:                nil,
		id:                   newUUID,
		stringNodes:          map[string]*ibNodeString{},
		uuidNodes:            map[uuid.UUID]*ibNodeUuid{},
		triplexStorage:       []triplex{},
		triplexKinds:         []uint{},
		directImports:        map[uuid.UUID]*namedGraph{},
		isImportedBy:         map[uuid.UUID]*namedGraph{},
		checkedOutCommits:    []Hash{},
		checkedOutNGRevisions: []Hash{},
		checkedOutDSRevisions: []Hash{},
	}
	type args struct {
		stage   *stage
		baseURI string
	}
	tests := []struct {
		name      string
		args      args
		want      *namedGraph
		assertion assert.ErrorAssertionFunc
	}{
		{
			name: "sample NamedGraph",
			args: args{
				stage:   nil,
				baseURI: "http://test.example.com/",
			},
			want:      &exampleGraph,
			assertion: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newNamedGraphIRI(tt.args.stage, IRI(tt.args.baseURI), false, false)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewReferencedGraph(t *testing.T) {
	baseIRI := "http://test.example.com/"
	newUUID := uuid.NewSHA1(uuid.NameSpaceURL, ([]byte)(baseIRI))
	exampleGraph := namedGraph{
		baseIRI: baseIRI,
		flags: namedGraphFlags{
			isReferenced: true,
			modified:     false,
		},
		stage:                nil,
		id:                   newUUID,
		stringNodes:          map[string]*ibNodeString{},
		uuidNodes:            map[uuid.UUID]*ibNodeUuid{},
		triplexStorage:       []triplex{},
		triplexKinds:         []uint{},
		directImports:        map[uuid.UUID]*namedGraph{},
		isImportedBy:         map[uuid.UUID]*namedGraph{},
		checkedOutCommits:    []Hash{},
		checkedOutNGRevisions: []Hash{},
		checkedOutDSRevisions: []Hash{},
	}
	type args struct {
		stage   *stage
		baseURI string
	}
	tests := []struct {
		name      string
		args      args
		want      *namedGraph
		assertion assert.ErrorAssertionFunc
	}{
		{
			name: "sample NamedGraph",
			args: args{
				stage:   nil,
				baseURI: "http://test.example.com/",
			},
			want:      &exampleGraph,
			assertion: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newNamedGraphIRI(tt.args.stage, IRI(tt.args.baseURI), true, false)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReferencedGraph_Base(t *testing.T) {
	type fields struct {
		ng       namedGraph
		stage    *stage
		subjects map[string]*ibNodeString
	}
	baseIRI, err := NewIRI("http://test.example.com/")
	assert.NoError(t, err)
	tests := []struct {
		name   string
		fields fields
		want   IRI
	}{
		{
			name: "sample NamedGraph",
			fields: fields{
				ng: namedGraph{
					baseIRI: "http://test.example.com/",
					flags: namedGraphFlags{
						isReferenced: true,
					},
				},
				stage:    nil,
				subjects: map[string]*ibNodeString{},
			},
			want: baseIRI,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &tt.fields.ng
			assert.Equal(t, tt.want, g.IRI())
		})
	}
}

func TestReferencedGraph_subjectWithFragment(t *testing.T) {
	type fields struct {
		stage    *stage
		subjects map[string]*ibNodeString
	}
	type args struct {
		fragment string
	}
	tests := []struct {
		name      string
		fields    fields
		args      args
		want      *ibNode
		assertion assert.ErrorAssertionFunc
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &namedGraph{
				stage:       tt.fields.stage,
				stringNodes: tt.fields.subjects,
			}
			got := g.GetIRINodeByFragment(tt.args.fragment)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReferencedGraph_forSubjects(t *testing.T) {
	type fields struct {
		stage    *stage
		subjects map[string]*ibNodeString
	}
	type args struct {
		c func(*ibNode) error
	}
	tests := []struct {
		name      string
		fields    fields
		args      args
		assertion assert.ErrorAssertionFunc
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &namedGraph{
				stage:       tt.fields.stage,
				stringNodes: tt.fields.subjects,
			}
			tt.assertion(t, g.forIRINodes(tt.args.c))
		})
	}
}

func TestReferencedGraph_createSubject(t *testing.T) {
	type fields struct {
		subjects map[string]*ibNodeString
	}
	type args struct {
		fragment string
	}
	tests := []struct {
		name      string
		fields    fields
		args      args
		want      *ibNode
		assertion assert.ErrorAssertionFunc
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &namedGraph{
				stringNodes: tt.fields.subjects,
			}
			got, err := g.createIRIStringNode(tt.args.fragment)
			tt.assertion(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
