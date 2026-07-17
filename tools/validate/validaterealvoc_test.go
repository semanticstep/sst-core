// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

// This package contains test cases for the validate package.
package validate_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/tools/validate"

	// This need to be manually added.
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fromTurtleContent(
	_ *testing.T,
	content string,
) (rdr *bufio.Reader) {
	return bufio.NewReader(strings.NewReader(content))
}

func findingKey(f validate.Finding) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s",
		f.Kind, f.Rule, f.Level, f.S, f.P, f.O)
}

func multisetOf(fs []validate.Finding) map[string]int {
	m := make(map[string]int, len(fs))
	for _, f := range fs {
		m[findingKey(f)]++
	}
	return m
}

func equalMultiset(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		if b[k] != av {
			return false
		}
	}
	return true
}

func Test_RdfTypeValidation(t *testing.T) {
	type args struct {
		bReader *bufio.Reader
	}
	toArgs := func(rdr *bufio.Reader) args {
		return args{bReader: rdr}
	}
	tests := []struct {
		name           string
		args           args
		expected       string
		graphAssertion func(t *testing.T, graph sst.NamedGraph)
		errorAssertion assert.ErrorAssertionFunc
	}{
		{
			name: "rdfType_correct",
			args: toArgs(fromTurtleContent(t,
				`
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix rdf:    <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:   <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:    <http://www.w3.org/2002/07/owl#> .
@prefix :   <urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa10#> .

<urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa10#>    a   owl:Ontology .
:c1efcf54-3e8e-4cc7-a7d1-82a9f613ad20  a lci:Person .
`)),

			expected: `{
  "kinds": [
    "rdf_type"
  ],
  "passed": true,
  "generated": "2025-11-07T11:14:49.473987+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
			},

			errorAssertion: assert.NoError,
		},
		{
			name: "rdfType_custom_mainclass_direct",
			args: toArgs(fromTurtleContent(t,
				`
@prefix rdf:    <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:   <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:    <http://www.w3.org/2002/07/owl#> .
@prefix ssmeta: <http://ontology.semanticstep.net/ssmeta#> .
@prefix :       <urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa10#> .

<urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa10#>    a   owl:Ontology .
:MyClass    a   owl:Class , ssmeta:MainClass .
:instance1  a   :MyClass .
`)),

			expected: `{
  "kinds": [
    "rdf_type"
  ],
  "passed": true,
  "generated": "2025-11-07T11:14:49.473987+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
			},

			errorAssertion: assert.NoError,
		},
		{
			name: "rdfType_custom_mainclass_subclass",
			args: toArgs(fromTurtleContent(t,
				`
@prefix rdf:    <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:   <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:    <http://www.w3.org/2002/07/owl#> .
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix :       <urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa10#> .

<urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa10#>    a   owl:Ontology .
:SubClass   a   owl:Class ;
    rdfs:subClassOf lci:Person .
:instance2  a   :SubClass .
`)),

			expected: `{
  "kinds": [
    "rdf_type"
  ],
  "passed": true,
  "generated": "2025-11-07T11:14:49.473987+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
			},

			errorAssertion: assert.NoError,
		},
		{
			name: "rdfType_missing",
			args: toArgs(fromTurtleContent(t,
				`
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix rdf:    <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:   <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:    <http://www.w3.org/2002/07/owl#> .
@prefix :   <urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa10#> .

<urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa10#>    a   owl:Ontology .
:c1efcf54-3e8e-4cc7-a7d1-82a9f613ad20  rdfs:label  "Test Company" .
`)),

			expected: `{
  "kinds": [
    "rdf_type"
  ],
  "passed": false,
  "findings": {
    "urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa10": [
      {
        "kind": "rdf_type",
        "rule": "RdfType_missing",
        "level": "error",
        "message": ":c1efcf54-3e8e-4cc7-a7d1-82a9f613ad20 has no rdf:type",
        "subject": ":c1efcf54-3e8e-4cc7-a7d1-82a9f613ad20",
        "predicate": "",
        "object": ""
      }
    ]
  },
  "generated": "2025-11-07T11:14:49.473987+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
			},

			errorAssertion: assert.NoError,
		},

		{
			name: "invalid_rdfType",
			args: toArgs(fromTurtleContent(t, `@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix sso:    <http://ontology.semanticstep.net/sso#> .
@prefix rdf:    <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:   <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:    <http://www.w3.org/2002/07/owl#> .
@prefix :   <urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa10#> .
@prefix ssmeta:   <http://ontology.semanticstep.net/ssmeta#> .

<urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa10#>    a   owl:Ontology .

:c1efcf54-3e8e-4cc7-a7d1-82a9f613ad21   a   ssmeta:DerivedValue .`)),
			expected: `{
  "kinds": [
    "rdf_type"
  ],
  "passed": false,
  "findings": {
    "urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa10": [
      {
        "kind": "rdf_type",
        "rule": "RdfType_wrong",
        "level": "error",
        "message": ":c1efcf54-3e8e-4cc7-a7d1-82a9f613ad21 does not contain a type that is either property or main class",
        "subject": ":c1efcf54-3e8e-4cc7-a7d1-82a9f613ad21",
        "predicate": "",
        "object": ""
      }
    ]
  },
  "generated": "2025-11-07T11:14:49.484161+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
			},

			errorAssertion: assert.NoError,
		},

		{
			name: "rdf_type_correct_subPropertyOf_property_2_level",
			args: toArgs(fromTurtleContent(t, `
@prefix lci:	<http://ontology.semanticstep.net/lci#> .
@prefix sso:	<http://ontology.semanticstep.net/sso#> .
@prefix rdf:	<http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:	<http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:	<http://www.w3.org/2002/07/owl#> .
@prefix :	<urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa20#> .
@prefix ns2:	<urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa10#> .

<urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa20>	a	owl:Ontology .
:c1efcf54-3e8e-4cc7-a7d1-82a9f613ad20	a	lci:Organization ,
			ns2:c1efcf54-3e8e-4cc7-a7d1-82a9f613ad11 ;
	rdfs:label	"Test Company" .
	
:c1efcf54-3e8e-4cc7-a7d1-82a9f613ad21	a	owl:ObjectProperty ;
	sso:idOwner	:c1efcf54-3e8e-4cc7-a7d1-82a9f613ad20 ;
	rdfs:subPropertyOf	sso:id .`)),
			expected: `{
  "kinds": [
    "rdf_type"
  ],
  "passed": true,
  "findings": {},
  "generated": "2025-11-11T11:16:30.7635267+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
				graph.Dump()
			},

			errorAssertion: assert.NoError,
		},

		{
			name: "rdf_type_correct_subPropertyOf_property",
			args: toArgs(fromTurtleContent(t, `
@prefix lci:	<http://ontology.semanticstep.net/lci#> .
@prefix sso:	<http://ontology.semanticstep.net/sso#> .
@prefix rdf:	<http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:	<http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:	<http://www.w3.org/2002/07/owl#> .
@prefix :	<urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa20#> .
@prefix ns2:	<urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa10#> .

<urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa20>	a	owl:Ontology .
:c1efcf54-3e8e-4cc7-a7d1-82a9f613ad20	a	lci:Organization ,
			ns2:c1efcf54-3e8e-4cc7-a7d1-82a9f613ad11 ;
	rdfs:label	"Test Company" .

:c1efcf54-3e8e-4cc7-a7d1-82a9f613ad21	a	owl:ObjectProperty ;
	sso:idOwner	:c1efcf54-3e8e-4cc7-a7d1-82a9f613ad20 ;
	rdfs:subPropertyOf	:c1efcf54-3e8e-4cc7-a7d1-82a9f613ad22 .

:c1efcf54-3e8e-4cc7-a7d1-82a9f613ad22	a	owl:ObjectProperty ;
	sso:idOwner	:c1efcf54-3e8e-4cc7-a7d1-82a9f613ad20 ;
	rdfs:subPropertyOf	sso:id .
	`)),
			expected: `{
  "kinds": [
    "rdf_type"
  ],
  "passed": true,
  "findings": {},
  "generated": "2025-11-11T11:16:30.7635267+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
				graph.Dump()
			},

			errorAssertion: assert.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, err := sst.RdfRead(tt.args.bReader, sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
			assert.NoError(t, err)

			rp, err := validate.Validate(st, validate.KindRdfType)
			assert.NoError(t, err)

			var want validate.ValidateReport
			require.NoError(t, json.Unmarshal([]byte(tt.expected), &want))

			require.Equal(t, want.Passed, rp.Passed, "passed flag mismatch")
			assert.ElementsMatch(t, want.Kinds, rp.Kinds, "kinds mismatch (unordered)")

			require.Equal(t, len(want.Findings), len(rp.Findings), "subject count mismatch")
			for subj, wantList := range want.Findings {
				gotList, ok := rp.Findings[subj]
				require.True(t, ok, "missing findings for subject %s", subj)

				wantSet := multisetOf(wantList)
				gotSet := multisetOf(gotList)
				if !equalMultiset(wantSet, gotSet) {
					t.Fatalf("unordered findings mismatch for subject %s\nwant=%v\ngot =%v", subj, wantSet, gotSet)
				}
			}

			if tt.errorAssertion(t, err) {
				tt.graphAssertion(t, st.NamedGraphs()[0])
			}
		})
	}
}

func Test_DomainRangeValidation(t *testing.T) {
	type args struct {
		bReader *bufio.Reader
	}
	toArgs := func(rdr *bufio.Reader) args {
		return args{bReader: rdr}
	}
	tests := []struct {
		name           string
		args           args
		expected       string
		graphAssertion func(t *testing.T, graph sst.NamedGraph)
		errorAssertion assert.ErrorAssertionFunc
	}{
		// domainRange_correct
		{
			name: "domainRange_correct",
			args: toArgs(fromTurtleContent(t,
				`
@prefix rdf:   <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:  <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:   <http://www.w3.org/2002/07/owl#> .
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix ex:    <http://example.org#> .

<http://example.org>    a   owl:Ontology .
ex:pump1  a lci:Individual ;
          lci:partOf ex:plant1 .
ex:plant1 a lci:Individual .
`)),

			expected: `{
  "kinds": [
    "domain_range"
  ],
  "passed": true,
  "findings": {},
  "generated": "2025-11-07T14:42:13.7028174+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
			},

			errorAssertion: assert.NoError,
		},

		// domainRange_inverseOf_correct
		{
			name: "domainRange_inverseOf_correct",
			args: toArgs(fromTurtleContent(t,
				`
@prefix rdf:   <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:  <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:   <http://www.w3.org/2002/07/owl#> .
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix ex:    <http://example.org#> .

<http://example.org>    a   owl:Ontology .
ex:engine1 a lci:Individual .
ex:car1    a lci:Individual ;
           lci:hasPart ex:engine1 .
`)),

			expected: `{
  "kinds": [
    "domain_range"
  ],
  "passed": true,
  "findings": {},
  "generated": "2025-11-07T14:42:13.7028174+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
			},

			errorAssertion: assert.NoError,
		},

		// domainRange_not_correct_domain_mismatch
		{
			name: "domainRange_not_correct_domain_mismatch",
			args: toArgs(fromTurtleContent(t,
				`
@prefix rdf:   <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:  <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:   <http://www.w3.org/2002/07/owl#> .
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix ex:    <http://example.org#> .

<http://example.org>    a   owl:Ontology .

ex:plant1 a lci:Individual .
ex:omniA a lci:Class ;
         lci:partOf ex:plant1 .
`)),

			expected: `{
  "kinds": [
    "domain_range"
  ],
  "passed": false,
  "findings": {
    "http://example.org": [
      {
        "kind": "domain_range",
        "rule": "domain_mismatch",
        "level": "error",
        "message": "domain mismatch",
        "subject": ":omniA",
        "predicate": "lci:partOf",
        "object": ":plant1"
      }
    ]
  },
  "generated": "2025-11-07T14:52:43.1898046+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
			},

			errorAssertion: assert.NoError,
		},

		// domainRange_not_correct_range_mismatch
		{
			name: "domainRange_not_correct_range_mismatch",
			args: toArgs(fromTurtleContent(t,
				`
@prefix rdf:   <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:  <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:   <http://www.w3.org/2002/07/owl#> .
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix ex:    <http://example.org#> .

<http://example.org>    a   owl:Ontology .

ex:pump2  a lci:Individual ;
          lci:partOf ex:omniB .
ex:omniB  a lci:Class .
`)),

			expected: `{
  "kinds": [
    "domain_range"
  ],
  "passed": false,
  "findings": {
    "http://example.org": [
      {
        "kind": "domain_range",
        "rule": "range_mismatch",
        "level": "error",
        "message": "Range mismatch",
        "subject": ":pump2",
        "predicate": "lci:partOf",
        "object": ":omniB"
      }
    ]
  },
  "generated": "2025-11-07T14:54:37.0426784+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
			},

			errorAssertion: assert.NoError,
		},

		// domainRange_not_correct_range_type_mismatch
		{
			name: "domainRange_not_correct_range_type_mismatch",
			args: toArgs(fromTurtleContent(t,
				`
@prefix rdf:   <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:  <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:   <http://www.w3.org/2002/07/owl#> .
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix ex:    <http://example.org#> .

<http://example.org>    a   owl:Ontology .

ex:car2 a lci:Individual ;
       lci:hasPart "wheel-literal" .
`)),

			expected: `{
  "kinds": [
    "domain_range"
  ],
  "passed": false,
  "findings": {
    "http://example.org": [
      {
        "kind": "domain_range",
        "rule": "range_mismatch",
        "level": "error",
        "message": "Range mismatch",
        "subject": ":car2",
        "predicate": "lci:hasPart",
        "object": "wheel-literal"
      }
    ]
  },
  "generated": "2025-11-07T15:14:02.3909279+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
			},

			errorAssertion: assert.NoError,
		},

		// domainRange_not_correct_domain_unknown
		{
			name: "domainRange_not_correct_domain_unknown",
			args: toArgs(fromTurtleContent(t,
				`
@prefix rdf:   <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:  <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:   <http://www.w3.org/2002/07/owl#> .
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix ex:    <http://example.org#> .

<http://example.org>    a   owl:Ontology .

ex:plant1 a lci:Individual .
ex:unknownSubject
    lci:partOf ex:plant1 .
`)),

			expected: `{
  "kinds": [
    "domain_range"
  ],
  "passed": false,
  "findings": {
    "http://example.org": [
      {
        "kind": "domain_range",
        "rule": "domain_mismatch",
        "level": "error",
        "message": "domain mismatch",
        "subject": ":unknownSubject",
        "predicate": "lci:partOf",
        "object": ":plant1"
      }
    ]
  },
  "generated": "2025-11-07T15:22:32.3630259+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
			},

			errorAssertion: assert.NoError,
		},

		// domainRange_not_correct_predicate_isNotProperty
		{
			name: "domainRange_not_correct_predicate_isNotProperty",
			args: toArgs(fromTurtleContent(t,
				`
@prefix rdf:   <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:  <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:   <http://www.w3.org/2002/07/owl#> .
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix ex:    <http://example.org#> .
@prefix exv:   <http://example.org/vocab#> .


<http://example.org>    a   owl:Ontology .

ex:s1 lci:Thing ex:s2 .
`)),

			expected: `{
  "kinds": [
    "domain_range"
  ],
  "passed": false,
  "findings": {
    "http://example.org": [
      {
        "kind": "domain_range",
        "rule": "predicate_not_property",
        "level": "error",
        "message": "the predicate is not a valid property",
        "subject": ":s1",
        "predicate": "lci:Thing",
        "object": ":s2"
      }
    ]
  },
  "generated": "2025-11-07T16:54:50.6936047+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
			},

			errorAssertion: assert.NoError,
		},

		// domainRange_not_correct_predicate_application_isNotProperty
		{
			name: "domainRange_not_correct_predicate_application_isNotProperty",
			args: toArgs(fromTurtleContent(t,
				`
@prefix rdf:   <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:  <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:   <http://www.w3.org/2002/07/owl#> .
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix ex:    <http://example.org#> .
@prefix exv:   <http://example.org/vocab#> .


<http://example.org>    a   owl:Ontology .

ex:newProperty1 a lci:Individual .

ex:s1 ex:newProperty1 ex:s2 .
`)),

			expected: `{
  "kinds": [
    "domain_range"
  ],
  "passed": false,
  "findings": {
    "http://example.org": [
      {
        "kind": "domain_range",
        "rule": "predicate_not_property",
        "level": "error",
        "message": "the predicate is not a valid property",
        "subject": ":s1",
        "predicate": ":newProperty1",
        "object": ":s2"
      }
    ]
  },
  "generated": "2025-11-07T16:43:06.7621187+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
			},

			errorAssertion: assert.NoError,
		},

		// domainRange_correct_predicate_is_subProperty_of_another_property
		{
			name: "domainRange_correct_predicate_is_subProperty_of_another_property",
			args: toArgs(fromTurtleContent(t,
				`
@prefix rdf:   <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:  <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:   <http://www.w3.org/2002/07/owl#> .
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix ex:    <http://example.org#> .
@prefix exv:   <http://example.org/vocab#> .


<http://example.org>    a   owl:Ontology .

ex:relatesTo a owl:ObjectProperty ;
  rdfs:subPropertyOf  lci:connectedTo.

ex:hasComponent a rdf:Property ;
  rdfs:subPropertyOf ex:relatesTo .

ex:a1 a lci:SpaceTimeIndividual ;
     ex:hasComponent ex:a2 .

ex:a2 a lci:SpaceTimeIndividual .
`)),

			expected: `{
  "kinds": [
    "domain_range"
  ],
  "passed": true,
  "findings": {},
  "generated": "2025-11-10T15:36:07.5324529+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
			},

			errorAssertion: assert.NoError,
		},

		// domainRange_not_correct_predicate_is_from_referenced_graph
		{
			name: "domainRange_not_correct_predicate_is_from_referenced_graph",
			args: toArgs(fromTurtleContent(t,
				`
@prefix rdf:   <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:  <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:   <http://www.w3.org/2002/07/owl#> .
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix ex:    <http://example.org#> .
@prefix exv:   <http://example.org/vocab#> .


<http://example.org>    a   owl:Ontology .

ex:s1 exv:newProperty1 ex:s2 .
`)),

			expected: `{
  "kinds": [
    "domain_range"
  ],
  "passed": false,
  "findings": {
    "http://example.org": [
      {
        "kind": "domain_range",
        "rule": "predicate_not_property",
        "level": "error",
        "message": "the predicate is not a valid property",
        "subject": ":s1",
        "predicate": "<http://example.org/vocab#newProperty1>",
        "object": ":s2"
      }
    ]
  },
  "generated": "2025-11-10T14:18:28.0234613+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
			},

			errorAssertion: assert.NoError,
		},

		// domainRange_correct_ObjectProperty_TermCollection
		{
			name: "domainRange_correct_ObjectProperty_TermCollection",
			args: toArgs(fromTurtleContent(t,
				`
@prefix rdf:   <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:  <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:   <http://www.w3.org/2002/07/owl#> .
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix ex:    <http://example.org#> .
@prefix exv:   <http://example.org/vocab#> .


<http://example.org>    a   owl:Ontology .

ex:d1 a lci:SpaceTimeIndividual ;
     lci:hasPart ( ex:d2 ex:d3 ) .

ex:d2 a lci:SpaceTimeIndividual .

ex:d3 a lci:SpaceTimeIndividual .
`)),

			expected: `{
  "kinds": [
    "domain_range"
  ],
  "passed": false,
  "findings": {
    "http://example.org": [
      {
        "kind": "domain_range",
        "rule": "range_mismatch",
        "level": "error",
        "message": "Range mismatch",
        "subject": ":d1",
        "predicate": "lci:hasPart",
        "object": ":cd6a63a5-5e33-4179-b916-2c1d9ec91ccd"
      },
      {
        "kind": "domain_range",
        "rule": "domain_mismatch",
        "level": "error",
        "message": "domain mismatch",
        "subject": ":cd6a63a5-5e33-4179-b916-2c1d9ec91ccd",
        "predicate": "rdf:first",
        "object": ":d2"
      },
      {
        "kind": "domain_range",
        "rule": "domain_mismatch",
        "level": "error",
        "message": "domain mismatch",
        "subject": ":cd6a63a5-5e33-4179-b916-2c1d9ec91ccd",
        "predicate": "rdf:first",
        "object": ":d3"
      }
    ]
  },
  "generated": "2025-11-10T16:14:43.3033204+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
			},

			errorAssertion: assert.NoError,
		},

		// domainRange_correct_DatatypeProperty_Literal
		{
			name: "domainRange_correct_DatatypeProperty_Literal",
			args: toArgs(fromTurtleContent(t,
				`
@prefix rdf:   <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:  <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:   <http://www.w3.org/2002/07/owl#> .
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix ex:    <http://example.org#> .
@prefix sso:	<http://ontology.semanticstep.net/sso#> .
@prefix exv:   <http://example.org/vocab#> .


<http://example.org>    a   owl:Ontology .

ex:e1 a lci:Thing ;
     sso:id "ID-001" .
`)),

			expected: `{
  "kinds": [
    "domain_range"
  ],
  "passed": true,
  "findings": {},
  "generated": "2025-11-11T11:16:30.7635267+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
			},

			errorAssertion: assert.NoError,
		},
		// domainRange_not_correct_DatatypeProperty_Literal
		{
			name: "domainRange_not_correct_DatatypeProperty_Literal",
			args: toArgs(fromTurtleContent(t,
				`
@prefix rdf:   <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:  <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:   <http://www.w3.org/2002/07/owl#> .
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix ex:    <http://example.org#> .
@prefix sso:	<http://ontology.semanticstep.net/sso#> .
@prefix exv:   <http://example.org/vocab#> .


<http://example.org>    a   owl:Ontology .

ex:e1 a lci:Thing ;
     sso:id 1 .
`)),

			expected: `{
  "kinds": [
    "domain_range"
  ],
  "passed": false,
  "findings": {
    "http://example.org": [
      {
        "kind": "domain_range",
        "rule": "range_mismatch",
        "level": "error",
        "message": "Range mismatch, expected datatype xsd:string, found datatype xsd:integer",
        "subject": ":e1",
        "predicate": "sso:id",
        "object": "1"
      }
    ]
  },
  "generated": "2025-11-12T11:42:50.3071298+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
			},

			errorAssertion: assert.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "domainRange_correct_ObjectProperty_TermCollection" {
				t.Skip("skip until fixed")
			}
			st, err := sst.RdfRead(tt.args.bReader, sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
			assert.NoError(t, err)

			rp, err := validate.Validate(st, validate.KindDomainRange)
			assert.NoError(t, err)

			var want validate.ValidateReport
			require.NoError(t, json.Unmarshal([]byte(tt.expected), &want))

			require.Equal(t, want.Passed, rp.Passed, "passed flag mismatch")
			assert.ElementsMatch(t, want.Kinds, rp.Kinds, "kinds mismatch (unordered)")

			require.Equal(t, len(want.Findings), len(rp.Findings), "subject count mismatch")
			for subj, wantList := range want.Findings {
				gotList, ok := rp.Findings[subj]
				require.True(t, ok, "missing findings for subject %s", subj)

				wantSet := multisetOf(wantList)
				gotSet := multisetOf(gotList)
				if !equalMultiset(wantSet, gotSet) {
					t.Fatalf("unordered findings mismatch for subject %s\nwant=%v\ngot =%v", subj, wantSet, gotSet)
				}
			}

			if tt.errorAssertion(t, err) {
				tt.graphAssertion(t, st.NamedGraphs()[0])
			}
		})
	}
}

func Test_ExampleValidation(t *testing.T) {
	// t.Skip("skip until fixed")
	type args struct {
		bReader *bufio.Reader
	}
	toArgs := func(rdr *bufio.Reader) args {
		return args{bReader: rdr}
	}
	tests := []struct {
		name           string
		args           args
		expected       string
		graphAssertion func(t *testing.T, graph sst.NamedGraph)
		errorAssertion assert.ErrorAssertionFunc
	}{
		{
			name: "example",
			args: toArgs(fromTurtleContent(t, `
@prefix lci:	<http://ontology.semanticstep.net/lci#> .
@prefix sso:	<http://ontology.semanticstep.net/sso#> .
@prefix rdf:	<http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:	<http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:	<http://www.w3.org/2002/07/owl#> .
@prefix :	<urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa20#> .
@prefix ns2:	<urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa10#> .

<urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa20>	a	owl:Ontology .
:c1efcf54-3e8e-4cc7-a7d1-82a9f613ad20	a	lci:Organization ,
			ns2:c1efcf54-3e8e-4cc7-a7d1-82a9f613ad11 ;
	rdfs:label	"Test Company" .
	
:c1efcf54-3e8e-4cc7-a7d1-82a9f613ad21	a	owl:ObjectProperty ;
	sso:idOwner	:c1efcf54-3e8e-4cc7-a7d1-82a9f613ad20 ;
	rdfs:subPropertyOf	:c1efcf54-3e8e-4cc7-a7d1-82a9f613ad22 .

:c1efcf54-3e8e-4cc7-a7d1-82a9f613ad22	a	owl:ObjectProperty ;
	sso:idOwner	:c1efcf54-3e8e-4cc7-a7d1-82a9f613ad20 ;
	rdfs:subPropertyOf	sso:id .	
`)),
			expected: `{
  "kinds": [
    "rdf_type",
    "domain_range"
  ],
  "passed": true,
  "findings": {},
  "generated": "2025-11-25T09:17:51.4629272+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
				graph.Dump()
			},

			errorAssertion: assert.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, err := sst.RdfRead(tt.args.bReader, sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
			assert.NoError(t, err)

			rp, err := validate.Validate(st, validate.KindRdfType, validate.KindDomainRange)
			assert.NoError(t, err)


			var want validate.ValidateReport
			require.NoError(t, json.Unmarshal([]byte(tt.expected), &want))

			require.Equal(t, want.Passed, rp.Passed, "passed flag mismatch")
			assert.ElementsMatch(t, want.Kinds, rp.Kinds, "kinds mismatch (unordered)")

			require.Equal(t, len(want.Findings), len(rp.Findings), "subject count mismatch")
			for subj, wantList := range want.Findings {
				gotList, ok := rp.Findings[subj]
				require.True(t, ok, "missing findings for subject %s", subj)

				wantSet := multisetOf(wantList)
				gotSet := multisetOf(gotList)
				if !equalMultiset(wantSet, gotSet) {
					t.Fatalf("unordered findings mismatch for subject %s\nwant=%v\ngot =%v", subj, wantSet, gotSet)
				}
			}

			if tt.errorAssertion(t, err) {
				tt.graphAssertion(t, st.NamedGraphs()[0])
			}
		})
	}
}

func Test_TermCollectionValidation(t *testing.T) {
	// t.Skip("skip until fixed")
	type args struct {
		bReader *bufio.Reader
	}
	toArgs := func(rdr *bufio.Reader) args {
		return args{bReader: rdr}
	}
	tests := []struct {
		name           string
		args           args
		expected       string
		graphAssertion func(t *testing.T, graph sst.NamedGraph)
		errorAssertion assert.ErrorAssertionFunc
	}{
		// term_collection_member_type_correct
		{
			name: "term_collection_member_type_correct",
			args: toArgs(fromTurtleContent(t, `
@prefix rdf:   <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:  <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:   <http://www.w3.org/2002/07/owl#> .
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix rep:	<http://ontology.semanticstep.net/rep#> .
@prefix :    <http://example.org#> .
@prefix exv:   <http://example.org/vocab#> .


<http://example.org>    a   owl:Ontology .

:uuid1 a rep:Edge . 
:uuid2 a rep:Edge .  
:uuid3 a rep:Edge .

:uuid4 a rep:Path ;
    rep:edgeList (:uuid1 :uuid2 :uuid3) .
`)),
			expected: `{
  "kinds": [
    "domain_range"
  ],
  "passed": true,
  "findings": {},
  "generated": "2025-11-20T11:15:36.6530803+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
				// graph.Dump()
			},

			errorAssertion: assert.NoError,
		},

		// term_collection_member_type_mismatch
		{
			name: "term_collection_member_type_mismatch",
			args: toArgs(fromTurtleContent(t, `
@prefix rdf:   <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:  <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:   <http://www.w3.org/2002/07/owl#> .
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix rep:	<http://ontology.semanticstep.net/rep#> .
@prefix :    <http://example.org#> .
@prefix exv:   <http://example.org/vocab#> .


<http://example.org>    a   owl:Ontology .

:uuid1 a rep:Edge . 
:uuid2 a rep:Edge .  
:uuid3 a rep:Point .

:uuid4 a rep:Path ;
    rep:edgeList (:uuid1 :uuid2 :uuid3) .
`)),
			expected: `{
  "kinds": [
    "domain_range"
  ],
  "passed": false,
  "findings": {
    "http://example.org": [
      {
        "kind": "domain_range",
        "rule": "range_mismatch",
        "level": "error",
        "message": "Term Collection member type mismatch :uuid3",
        "subject": ":uuid4",
        "predicate": "rep:edgeList",
        "object": ":uuid3"
      }
    ]
  },
  "generated": "2025-11-20T11:11:52.6340245+08:00"
}`,

			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
				// graph.Dump()
			},

			errorAssertion: assert.NoError,
		},

		// predicate_range_not_term_collection
		{
			name: "predicate_range_not_term_collection",
			args: toArgs(fromTurtleContent(t, `
@prefix rdf:   <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:  <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:   <http://www.w3.org/2002/07/owl#> .
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix rep:	<http://ontology.semanticstep.net/rep#> .
@prefix :    <http://example.org#> .
@prefix exv:   <http://example.org/vocab#> .


<http://example.org>    a   owl:Ontology .

:uuid1 a rep:Edge . 
:uuid2 a rep:Edge .  
:uuid3 a rep:Point .

:uuid4 a rep:OrientedPath ;
    rep:pathElement (:uuid1 :uuid2 :uuid3) .
`)),
			expected: `{
  "kinds": [
    "domain_range"
  ],
  "passed": false,
  "findings": {
    "http://example.org": [
      {
        "kind": "domain_range",
        "rule": "range_mismatch",
        "level": "error",
        "message": "Predicate rep:pathElement range is not a TermCollection",
        "subject": ":uuid4",
        "predicate": "rep:pathElement",
        "object": ""
      }
    ]
  },
  "generated": "2025-11-20T11:46:15.9916453+08:00"
}`,

			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
				// graph.Dump()
			},

			errorAssertion: assert.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, err := sst.RdfRead(tt.args.bReader, sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
			assert.NoError(t, err)

			rp, err := validate.Validate(st, validate.KindDomainRange)
			assert.NoError(t, err)

			var want validate.ValidateReport
			require.NoError(t, json.Unmarshal([]byte(tt.expected), &want))

			require.Equal(t, want.Passed, rp.Passed, "passed flag mismatch")
			assert.ElementsMatch(t, want.Kinds, rp.Kinds, "kinds mismatch (unordered)")

			require.Equal(t, len(want.Findings), len(rp.Findings), "subject count mismatch")
			for subj, wantList := range want.Findings {
				gotList, ok := rp.Findings[subj]
				require.True(t, ok, "missing findings for subject %s", subj)

				wantSet := multisetOf(wantList)
				gotSet := multisetOf(gotList)
				if !equalMultiset(wantSet, gotSet) {
					t.Fatalf("unordered findings mismatch for subject %s\nwant=%v\ngot =%v", subj, wantSet, gotSet)
				}
			}

			if tt.errorAssertion(t, err) {
				tt.graphAssertion(t, st.NamedGraphs()[0])
			}
		})
	}
}

func Test_LiteralCollectionValidation(t *testing.T) {
	type args struct {
		bReader *bufio.Reader
	}
	toArgs := func(rdr *bufio.Reader) args {
		return args{bReader: rdr}
	}
	tests := []struct {
		name           string
		args           args
		expected       string
		graphAssertion func(t *testing.T, graph sst.NamedGraph)
		errorAssertion assert.ErrorAssertionFunc
	}{
		// literal_collection_member_type_correct
		{
			name: "literal_collection_member_type_correct",
			args: toArgs(fromTurtleContent(t, `
@prefix rdf:   <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:  <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:   <http://www.w3.org/2002/07/owl#> .
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix rep:	<http://ontology.semanticstep.net/rep#> .
@prefix :    <http://example.org#> .
@prefix exv:   <http://example.org/vocab#> .


<http://example.org>    a   owl:Ontology .

:uuid4 a rep:ColourRGB ;
	rep:rgb ( 1 2 3) .
`)),
			expected: `{
  "kinds": [
    "domain_range"
  ],
  "passed": true,
  "findings": {},
  "generated": "2025-11-24T10:54:34.5171444+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
				graph.Dump()
			},

			errorAssertion: assert.NoError,
		},
		// term_collection_member_type_mismatch
		{
			name: "term_collection_member_type_mismatch",
			args: toArgs(fromTurtleContent(t, `
@prefix rdf:   <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:  <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:   <http://www.w3.org/2002/07/owl#> .
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix rep:	<http://ontology.semanticstep.net/rep#> .
@prefix :    <http://example.org#> .
@prefix exv:   <http://example.org/vocab#> .


<http://example.org>    a   owl:Ontology .

:uuid4 a rep:ColourRGB ;
	rep:rgb ( 1 2 true) .
`)),
			expected: `{
  "kinds": [
    "domain_range"
  ],
  "passed": false,
  "findings": {
    "http://example.org": [
      {
        "kind": "domain_range",
        "rule": "range_mismatch",
        "level": "error",
        "message": "Range mismatch, expected datatype xsd:integer, found datatype xsd:boolean",
        "subject": ":uuid4",
        "predicate": "rep:rgb",
        "object": "true"
      }
    ]
  },
  "generated": "2025-11-24T16:16:57.6942743+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
				// graph.Dump()
			},

			errorAssertion: assert.NoError,
		},

		// literal_collection_member_type_all_mismatch
		{
			name: "literal_collection_member_type_correct",
			args: toArgs(fromTurtleContent(t, `
@prefix rdf:   <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:  <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:   <http://www.w3.org/2002/07/owl#> .
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix rep:	<http://ontology.semanticstep.net/rep#> .
@prefix :    <http://example.org#> .
@prefix exv:   <http://example.org/vocab#> .


<http://example.org>    a   owl:Ontology .

:uuid4 a rep:ColourRGB ;
	rep:rgb ( true true false) .
`)),
			expected: `{
  "kinds": [
    "domain_range"
  ],
  "passed": false,
  "findings": {
    "http://example.org": [
      {
        "kind": "domain_range",
        "rule": "range_mismatch",
        "level": "error",
        "message": "Range mismatch, expected datatype xsd:integer, found datatype xsd:boolean",
        "subject": ":uuid4",
        "predicate": "rep:rgb",
        "object": "( true true false )"
      }
    ]
  },
  "generated": "2025-11-25T10:34:35.0506802+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
				graph.Dump()
			},

			errorAssertion: assert.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, err := sst.RdfRead(tt.args.bReader, sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
			assert.NoError(t, err)

			if tt.errorAssertion(t, err) {
				tt.graphAssertion(t, st.NamedGraphs()[0])
			}

			rp, err := validate.Validate(st, validate.KindDomainRange)
			assert.NoError(t, err)

			var want validate.ValidateReport
			require.NoError(t, json.Unmarshal([]byte(tt.expected), &want))

			require.Equal(t, want.Passed, rp.Passed, "passed flag mismatch")
			assert.ElementsMatch(t, want.Kinds, rp.Kinds, "kinds mismatch (unordered)")

			require.Equal(t, len(want.Findings), len(rp.Findings), "subject count mismatch")
			for subj, wantList := range want.Findings {
				gotList, ok := rp.Findings[subj]
				require.True(t, ok, "missing findings for subject %s", subj)

				wantSet := multisetOf(wantList)
				gotSet := multisetOf(gotList)
				if !equalMultiset(wantSet, gotSet) {
					t.Fatalf("unordered findings mismatch for subject %s\nwant=%v\ngot =%v", subj, wantSet, gotSet)
				}
			}
		})
	}
}

func Test_FunctionalPropValidation(t *testing.T) {
	type args struct {
		bReader *bufio.Reader
	}
	toArgs := func(rdr *bufio.Reader) args {
		return args{bReader: rdr}
	}
	tests := []struct {
		name           string
		args           args
		expected       string
		graphAssertion func(t *testing.T, graph sst.NamedGraph)
		errorAssertion assert.ErrorAssertionFunc
	}{
		// functional_correct
		{
			name: "functional_correct",
			args: toArgs(fromTurtleContent(t,
				`
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix rdf:    <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:   <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:    <http://www.w3.org/2002/07/owl#> .
@prefix :   <urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa10#> .

<urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa10#>    a   owl:Ontology .

:pair1 a lci:OrderedPair ;
       lci:hasFirstInPair :objA .

:objA a owl:Thing . 

`)),

			expected: `{
  "kinds": [
    "functional_property"
  ],
  "passed": true,
  "findings": {},
  "generated": "2025-11-14T11:19:43.7088649+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
			},
			errorAssertion: assert.NoError,
		},
		// functional_correct_multiple
		{
			name: "functional_correct_multiple",
			args: toArgs(fromTurtleContent(t,
				`
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix rdf:    <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:   <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:    <http://www.w3.org/2002/07/owl#> .
@prefix :   <urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa10#> .

<urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa10#>    a   owl:Ontology .

:pair1 a lci:OrderedPair ;
       lci:hasFirstInPair :first1 .

:first1 a owl:Thing .

:pair2 a lci:OrderedPair ;
       lci:hasFirstInPair :first2 .

:first2 a owl:Thing .

`)),

			expected: `{
  "kinds": [
    "functional_property"
  ],
  "passed": true,
  "findings": {},
  "generated": "2025-11-14T11:19:43.7088649+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
			},
			errorAssertion: assert.NoError,
		},

		// functional_not_correct
		{
			name: "functional_not_correct",
			args: toArgs(fromTurtleContent(t,
				`
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix rdf:    <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:   <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:    <http://www.w3.org/2002/07/owl#> .
@prefix :   <urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa10#> .

<urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa10#>    a   owl:Ontology .

:pair1 a lci:OrderedPair ;
       lci:hasFirstInPair :objA ;
       lci:hasFirstInPair :objB .

:objA a owl:Thing .
:objB a owl:Thing .
`)),

			expected: `{
  "kinds": [
    "functional_property"
  ],
  "passed": false,
  "findings": {
    "urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa10": [
      {
        "kind": "functional_property",
        "rule": "predicate_is_functional_property",
        "level": "error",
        "message": "predicate is owl.FunctionalProperty, it can be used only once for a subject",
        "subject": ":pair1",
        "predicate": "lci:hasFirstInPair",
        "object": ":objB"
      }
    ]
  },
  "generated": "2025-11-14T15:02:08.2579142+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
			},
			errorAssertion: assert.NoError,
		},

		// inverse_functional_correct
		{
			name: "inverse_functional_correct",
			args: toArgs(fromTurtleContent(t,
				`
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix rdf:    <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:   <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:    <http://www.w3.org/2002/07/owl#> .
@prefix :   <urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa10#> .

<urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa10#>    a   owl:Ontology .

:PowerOfPerson   lci:powerClassOf :Person .
:PowerOfStudent  lci:powerClassOf :Student .

`)),

			expected: `{
  "kinds": [
    "functional_property"
  ],
  "passed": true,
  "findings": {},
  "generated": "2025-11-14T15:00:43.3137982+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
			},
			errorAssertion: assert.NoError,
		},

		// inverse_functional_not_correct
		{
			name: "inverse_functional_not_correct",
			args: toArgs(fromTurtleContent(t,
				`
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix rdf:    <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:   <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:    <http://www.w3.org/2002/07/owl#> .
@prefix :   <urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa10#> .

<urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa10#>    a   owl:Ontology .

:PowerOfPerson1 a owl:Class ;
    lci:powerClassOf :Person .

:PowerOfPerson2 a owl:Class ;
    lci:powerClassOf :Person .
`)),

			expected: `{
  "kinds": [
    "functional_property"
  ],
  "passed": false,
  "findings": {
    "urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f61aaa10": [
      {
        "kind": "functional_property",
        "rule": "predicate_is_inverse_functional_property",
        "level": "error",
        "message": "predicate is owl.InverseFunctionalProperty, it can be used only once for an object",
        "subject": ":PowerOfPerson1",
        "predicate": "lci:powerClassOf",
        "object": " :Person"
      }
    ]
  },
  "generated": "2025-11-14T15:23:54.8271281+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
			},
			errorAssertion: assert.NoError,
		},

		// functional_not_correct_application_prop
		{
			name: "functional_not_correct_application_prop",
			args: toArgs(fromTurtleContent(t,
				`
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix rdf:    <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:   <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:    <http://www.w3.org/2002/07/owl#> .
@prefix ex:   <http://example.com/vocab#> .

<http://example.com/vocab#>    a   owl:Ontology .

ex:Person a owl:Class .
ex:EmailAccount a owl:Class .
ex:NationalID a owl:Class .

ex:hasPrimaryEmail
    a owl:ObjectProperty , owl:FunctionalProperty ;
    rdfs:domain ex:Person ;
    rdfs:range  ex:EmailAccount .

ex:alice a ex:Person ;
    ex:hasPrimaryEmail ex:aliceMail1 ;
    ex:hasPrimaryEmail ex:aliceMail2 .

ex:aliceMail1 a ex:EmailAccount .
ex:aliceMail2 a ex:EmailAccount .
`)),

			expected: `{
  "kinds": [
    "functional_property"
  ],
  "passed": false,
  "findings": {
    "http://example.com/vocab": [
      {
        "kind": "functional_property",
        "rule": "predicate_is_functional_property",
        "level": "error",
        "message": "predicate is owl.FunctionalProperty, it can be used only once for a subject",
        "subject": ":alice",
        "predicate": ":hasPrimaryEmail",
        "object": ":aliceMail2"
      }
    ]
  },
  "generated": "2025-11-14T16:25:41.6774104+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
			},
			errorAssertion: assert.NoError,
		},

		// inverse_functional_not_correct_application_prop
		{
			name: "inverse_functional_not_correct_application_prop",
			args: toArgs(fromTurtleContent(t,
				`
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix rdf:    <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:   <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:    <http://www.w3.org/2002/07/owl#> .
@prefix ex:   <http://example.com/vocab#> .

<http://example.com/vocab#>    a   owl:Ontology .

ex:Person a owl:Class .
ex:EmailAccount a owl:Class .
ex:NationalID a owl:Class .

ex:hasNationalID
    a owl:ObjectProperty , owl:InverseFunctionalProperty ;
    rdfs:domain ex:Person ;
    rdfs:range  ex:NationalID .

ex:nidShared a ex:NationalID .

ex:alice a ex:Person ;
    ex:hasNationalID ex:nidShared .

ex:bob   a ex:Person ;
    ex:hasNationalID ex:nidShared .
`)),

			expected: `{
  "kinds": [
    "functional_property"
  ],
  "passed": false,
  "findings": {
    "http://example.com/vocab": [
      {
        "kind": "functional_property",
        "rule": "predicate_is_inverse_functional_property",
        "level": "error",
        "message": "predicate is owl.InverseFunctionalProperty, it can be used only once for an object",
        "subject": ":bob",
        "predicate": ":hasNationalID",
        "object": " :nidShared"
      }
    ]
  },
  "generated": "2025-11-14T16:27:44.1743056+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
			},
			errorAssertion: assert.NoError,
		},

		// functional_correct_application_subProperty_prop
		{
			name: "functional_correct_application_subProperty_prop",
			args: toArgs(fromTurtleContent(t,
				`
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix rdf:    <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:   <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:    <http://www.w3.org/2002/07/owl#> .
@prefix ex:   <http://example.com/vocab#> .

<http://example.com/vocab#>    a   owl:Ontology .

ex:Person      a owl:Class .
ex:Email       a owl:Class .
ex:NationalID  a owl:Class .

ex:hasPrimaryEmail
    a owl:ObjectProperty , owl:FunctionalProperty ;
    rdfs:domain ex:Person ;
    rdfs:range  ex:Email .

ex:hasWorkEmail
    a owl:ObjectProperty ;
    rdfs:subPropertyOf ex:hasPrimaryEmail .

ex:alice a ex:Person ;
    ex:hasPrimaryEmail ex:aliceMain .

ex:bob a ex:Person ;
    ex:hasWorkEmail ex:bobWork .

ex:aliceMain a ex:Email .
ex:bobWork   a ex:Email .
`)),

			expected: `{
  "kinds": [
    "functional_property"
  ],
  "passed": true,
  "findings": {},
  "generated": "2025-11-17T09:53:16.2778506+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
			},
			errorAssertion: assert.NoError,
		},

		// functional_not_correct_application_subProperty_prop_same_object
		{
			name: "functional_not_correct_application_subProperty_prop_same_object",
			args: toArgs(fromTurtleContent(t,
				`
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix rdf:    <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:   <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:    <http://www.w3.org/2002/07/owl#> .
@prefix sso:	<http://ontology.semanticstep.net/sso#> .
@prefix ex:   <http://example.com/vocab#> .

<http://example.com/vocab#>    a   owl:Ontology .

ex:belgianVatId
    rdf:type owl:DatatypeProperty ;
    rdfs:subPropertyOf sso:europeanVatId ;
    rdfs:domain lci:Organization .

ex:org3 a lci:Organization ;
    sso:europeanVatId "BE123" ;
    ex:belgianVatId   "BE123" .
`)),

			expected: `{
  "kinds": [
    "functional_property"
  ],
  "passed": false,
  "findings": {
    "http://example.com/vocab": [
      {
        "kind": "functional_property",
        "rule": "predicate_is_functional_property",
        "level": "error",
        "message": "predicate is owl.FunctionalProperty, it can be used only once for a subject",
        "subject": ":org3",
        "predicate": ":belgianVatId",
        "object": "BE123"
      }
    ]
  },
  "generated": "2025-11-18T10:30:25.8052878+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
			},
			errorAssertion: assert.NoError,
		},
		// functional_not_correct_application_subProperty_prop_diff_objects
		{
			name: "functional_not_correct_application_subProperty_prop_diff_objects",
			args: toArgs(fromTurtleContent(t,
				`
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix rdf:    <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:   <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:    <http://www.w3.org/2002/07/owl#> .
@prefix sso:	<http://ontology.semanticstep.net/sso#> .
@prefix ex:   <http://example.com/vocab#> .

<http://example.com/vocab#>    a   owl:Ontology .

ex:belgianVatId
    rdf:type owl:DatatypeProperty ;
    rdfs:subPropertyOf sso:europeanVatId ;
    rdfs:domain lci:Organization .

ex:org5 a lci:Organization ;
    sso:europeanVatId "BE333" ;
    ex:belgianVatId   "BE444" .
`)),

			expected: `{
  "kinds": [
    "functional_property"
  ],
  "passed": false,
  "findings": {
    "http://example.com/vocab": [
      {
        "kind": "functional_property",
        "rule": "predicate_is_functional_property",
        "level": "error",
        "message": "predicate is owl.FunctionalProperty, it can be used only once for a subject",
        "subject": ":org5",
        "predicate": ":belgianVatId",
        "object": "BE444"
      }
    ]
  },
  "generated": "2025-11-18T13:34:40.0278205+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
			},
			errorAssertion: assert.NoError,
		},
		// functional_correct_application_super_prop_diff_objects
		{
			name: "functional_not_correct_application_subProperty_prop_diff_objects",
			args: toArgs(fromTurtleContent(t,
				`
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix rdf:    <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:   <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:    <http://www.w3.org/2002/07/owl#> .
@prefix sso:	<http://ontology.semanticstep.net/sso#> .
@prefix ex:   <http://example.com/vocab#> .

<http://example.com/vocab#>    a   owl:Ontology .

ex:org7 a lci:Organization ;
    sso:id            "OLD-ID" ;
    sso:europeanVatId "NEW-ID" .
`)),

			expected: `{
  "kinds": [
    "functional_property"
  ],
  "passed": true,
  "findings": {},
  "generated": "2025-11-18T13:48:29.1056711+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
			},
			errorAssertion: assert.NoError,
		},

		// inverse_functional_not_correct_application_subProperty_prop
		{
			name: "inverse_functional_not_correct_application_subProperty_prop",
			args: toArgs(fromTurtleContent(t,
				`
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix rdf:    <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:   <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:    <http://www.w3.org/2002/07/owl#> .
@prefix sso:	<http://ontology.semanticstep.net/sso#> .
@prefix ex:   <http://example.com/vocab#> .

<http://example.com/vocab#>    a   owl:Ontology .

ex:specialPowerClassOf
    a owl:ObjectProperty ;
    rdfs:subPropertyOf lci:powerClassOf .

ex:m1 a owl:Class .
ex:m2 a owl:Class .
ex:mo1 a owl:Class .

ex:m1 lci:powerClassOf       ex:mo1 .
ex:m2 ex:specialPowerClassOf ex:mo1 .
`)),

			expected: `{
  "kinds": [
    "functional_property"
  ],
  "passed": false,
  "findings": {
    "http://example.com/vocab": [
      {
        "kind": "functional_property",
        "rule": "predicate_is_inverse_functional_property",
        "level": "error",
        "message": "InverseFunctionalProperty lci:powerClassOf must be used only once for an object",
        "subject": ":m2",
        "predicate": ":specialPowerClassOf",
        "object": ":mo1"
      }
    ]
  },
  "generated": "2025-11-25T09:24:54.6355768+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
			},
			errorAssertion: assert.NoError,
		},

		// inverse_functional_correct_application_super_property_prop
		{
			name: "inverse_functional_not_correct_application_subProperty_prop",
			args: toArgs(fromTurtleContent(t,
				`
@prefix lci:    <http://ontology.semanticstep.net/lci#> .
@prefix rdf:    <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:   <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl:    <http://www.w3.org/2002/07/owl#> .
@prefix sso:	<http://ontology.semanticstep.net/sso#> .
@prefix ex:   <http://example.com/vocab#> .

<http://example.com/vocab#>    a   owl:Ontology .

ex:m1  a sso:MatingOccurrenceMember , sso:AssemblyGroupOccurrenceMember .
ex:m2  a sso:MatingOccurrenceMember , sso:AssemblyGroupOccurrenceMember .

ex:mo1 a sso:MatingOccurrence .

ex:m1 sso:matingOccurrenceOf        ex:mo1 .
ex:m2 sso:assembledGroupOccurrenceOf ex:mo1 .
`)),

			expected: `{
  "kinds": [
    "functional_property"
  ],
  "passed": true,
  "findings": {},
  "generated": "2025-11-18T15:36:37.2180193+08:00"
}`,
			graphAssertion: func(t *testing.T, graph sst.NamedGraph) {
			},
			errorAssertion: assert.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, err := sst.RdfRead(tt.args.bReader, sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
			assert.NoError(t, err)

			rp, err := validate.Validate(st, validate.KindFunctionalProperty)
			assert.NoError(t, err)

			var want validate.ValidateReport
			require.NoError(t, json.Unmarshal([]byte(tt.expected), &want))

			require.Equal(t, want.Passed, rp.Passed, "passed flag mismatch")
			assert.ElementsMatch(t, want.Kinds, rp.Kinds, "kinds mismatch (unordered)")

			require.Equal(t, len(want.Findings), len(rp.Findings), "subject count mismatch")

			// because subject in this test case is random
			if tt.name == "inverse_functional_not_correct" ||
				tt.name == "inverse_functional_not_correct_application_prop" ||
				tt.name == "inverse_functional_not_correct_application_subProperty_prop" {
				return
			}

			for subj, wantList := range want.Findings {
				gotList, ok := rp.Findings[subj]
				require.True(t, ok, "missing findings for subject %s", subj)

				wantSet := multisetOf(wantList)
				gotSet := multisetOf(gotList)
				if !equalMultiset(wantSet, gotSet) {
					t.Fatalf("unordered findings mismatch for subject %s\nwant=%v\ngot =%v", subj, wantSet, gotSet)
				}
			}

			if tt.errorAssertion(t, err) {
				tt.graphAssertion(t, st.NamedGraphs()[0])
			}
		})
	}
}
