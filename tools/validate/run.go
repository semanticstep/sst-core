// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package validate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/semanticstep/sst-core/sst"
	"go.uber.org/zap"

	// _ "github.com/semanticstep/sst-core/vocabularies/dict" // include vocabularies.
	flag "github.com/spf13/pflag"
)

type stepEnum byte

const (
	stepRdfType = stepEnum(iota)
	stepDomainRange
	experimentalStepDefinitions
	experimentalStepAcyclic
	experimentalStepConnected
)

var (
	flags = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	steps = ValuesOf[stepEnum](
		"rdf-type", "domain-range", "experimental-definitions", "experimental-acyclic", "experimental-connected",
	)
	_       = flags.VarPF(&steps, "step", "s", steps.Help("run specified `step`s only, may be set more than once\n   "))
	verbose = flags.BoolP("verbose", "v", false, "verbose output")
	quiet   = flags.BoolP("quiet", "q", false, "make validation quiet and silence normal output")

	usage = func() {
		fmt.Fprint(os.Stderr, "SST Data Validation\n\n")
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "    %s [options] ttl-file-name\n", filepath.Base(os.Args[0]))
		if flags.FlagUsages() != "" {
			fmt.Fprintln(os.Stderr, "Options:")
			flags.PrintDefaults()
		}
	}
	errorsEncountered int
)

type ExitStatusError int

func (e ExitStatusError) Error() string {
	return fmt.Sprintf("exit code %d", e)
}

func (e ExitStatusError) Status() int {
	return int(e)
}

type outputLog struct{ g sst.NamedGraph }

func (l outputLog) Log(level LogLevel, t sst.IBNode, v ...any) error {
	var err error
	switch level {
	case InfoEnterLevel:
		if *verbose {
			sst.GlobalLogger.Debug("entering" + valuesToLogString(l.g, v...))
		}
	case InfoLeaveLevel:
		if *verbose {
			sst.GlobalLogger.Debug("leaving" + valuesToLogString(l.g, v...))
		}
	case InfoLevel:
		if *verbose {
			sst.GlobalLogger.Debug("", zap.String("IBNode", ibNodeValuesToLogString(l.g, sst.IBNode(t), v...)))
		}
	case WarnLevel:
		if !*quiet {
			sst.GlobalLogger.Debug("", zap.String("IBNode", ibNodeValuesToLogString(l.g, sst.IBNode(t), v...)))
		}
	case ErrorLevel:
		errorsEncountered++
		if !*quiet {
			sst.GlobalLogger.Debug("", zap.String("IBNode", ibNodeValuesToLogString(l.g, sst.IBNode(t), v...)))
		}
	}
	return err
}

func (l outputLog) LogForGraph(level LogLevel, t sst.NamedGraph, v ...any) error {
	var err error
	switch level {
	case InfoEnterLevel:
		if !*quiet {
			sst.GlobalLogger.Debug("starting" + valuesToLogString(l.g, v...))
		}
	case InfoLeaveLevel:
		if !*quiet {
			sst.GlobalLogger.Debug("finished" + valuesToLogString(l.g, v...))
		}
	case InfoLevel:
		if !*quiet {
			sst.GlobalLogger.Debug("", zap.String("NamedGraph", sst.NamedGraph(t).IRI().String()+valuesToLogString(l.g, v...)))
		}
	case WarnLevel:
		if !*quiet {
			sst.GlobalLogger.Debug("", zap.String("WARN", sst.NamedGraph(t).IRI().String()+valuesToLogString(l.g, v...)))
		}
	case ErrorLevel:
		errorsEncountered++
		if !*quiet {
			sst.GlobalLogger.Debug("", zap.String("ERROR", sst.NamedGraph(t).IRI().String()+valuesToLogString(l.g, v...)))
		}
	}
	return err
}

func ibNodeValuesToLogString(graph sst.NamedGraph, s sst.IBNode, vv ...any) string {
	if len(vv) == 3 {
		if err, ok := vv[0].(error); ok {
			var f TripleStringFormatter
			if errors.As(err, &f) {
				if p, ok := vv[1].(sst.IBNode); ok {
					if o, ok := vv[2].(sst.Term); ok {
						return f.FormatTripleString(
							graphPrefixedFragment(graph, s),
							graphPrefixedFragment(graph, p),
							valuesToLogString(graph, o)[1:])
					}
				}
			}
		}
	}
	return fmt.Sprintf("%s%s", graphPrefixedFragment(graph, s), valuesToLogString(graph, vv...))
}

func literalToString(l sst.Literal) string {
	switch o := l.(type) {
	case sst.String:
		return string(o)
	case sst.LangString:
		return string(o.Val) + "@" + string(o.LangTag)
	case sst.Double:
		return strconv.FormatFloat(float64(o), 'f', 2, 64)
	case sst.Integer:
		return strconv.FormatInt(int64(o), 10)
	case sst.Boolean:
		return strconv.FormatBool(bool(o))
	default:
		return fmt.Errorf("unknown literal type %T", o).Error()
	}
}

func valuesToLogString(graph sst.NamedGraph, vv ...any) string {
	var b strings.Builder
	for _, v := range vv {
		// b.WriteRune(' ')
		switch v := v.(type) {
		case sst.IBNode:
			b.WriteString(graphPrefixedFragment(graph, v))
		case sst.Literal:
			b.WriteString(literalToString(v))

		case sst.LiteralCollection:
			tempS := ""
			tempS += "( "
			v.ForMembers(func(index int, li sst.Literal) {
				tempS += literalToString(li) + " "
			})
			tempS += ")"
			b.WriteString(tempS)
		default:
			b.WriteString(fmt.Sprintf("%v", v))
		}
	}
	return b.String()
}

func graphPrefixedFragment(graph sst.NamedGraph, d sst.IBNode) string {
	if d.OwningGraph() == graph {
		if d.IsIRINode() {
			_, f := d.IRI().Split()
			return ":" + f
		} else if d.IsBlankNode() {
			return ":" + d.ID().String()
		} else {
			panic("unexpected IBNode that is neither IRI nor Blank Node")
		}
	}

	if d.IsIRINode() {
		return d.PrefixedFragment()
	} else {
		return ""
	}
}

func (l outputLog) Logf(level LogLevel, t sst.IBNode, format string, v ...any) error {
	return l.Log(level, t, fmt.Sprintf(format, v...))
}

func (l outputLog) LogfForGraph(level LogLevel, t sst.NamedGraph, format string, v ...any) error {
	return l.LogForGraph(level, t, fmt.Sprintf(format, v...))
}

type ValidationKind string

const (
	KindRdfType            ValidationKind = "rdf_type"
	KindDomainRange        ValidationKind = "domain_range"
	KindFunctionalProperty ValidationKind = "functional_property"
)

type Rule string

const (
	RuleRdfTypeMissing Rule = "RdfType_missing"
	RuleRdfTypeWrong   Rule = "RdfType_wrong"

	RuleDomainMismatch       Rule = "domain_mismatch"
	RuleRangeMismatch        Rule = "range_mismatch"
	RulePredicateNotProperty Rule = "predicate_not_property"
	RulePredicateNotKnown    Rule = "predicate_not_known"

	RulePredicateFunctionalProperty        Rule = "predicate_is_functional_property"
	RulePredicateInverseFunctionalProperty Rule = "predicate_is_inverse_functional_property"

	// RuleRangeMismatch  Rule = "range_mismatch".
)

type ValidateReport struct {
	Kinds     []ValidationKind     `json:"kinds"`
	Passed    bool                 `json:"passed"`   // if there is any error, then false
	Findings  map[string][]Finding `json:"findings"` // warnings and errors
	Generated time.Time            `json:"generated"`
}

type CheckedSummary struct {
	Triples    int `json:"triples"`
	Subjects   int `json:"subjects"`
	Properties int `json:"properties"`
}

// single finding.
type Finding struct {
	Kind    ValidationKind `json:"kind"`
	Rule    Rule           `json:"rule"`
	Level   string         `json:"level"` // "error" or "warning"
	Message string         `json:"message"`

	// related triple
	S string `json:"subject"`
	P string `json:"predicate"`
	O string `json:"object"`

	Expect string `json:"expect,omitempty"` // "range in {sso:Occurrence | sso:Terminal}"
	Actual string `json:"actual,omitempty"` // "object is literal xsd:string"
}

func NewReport(kinds ...ValidationKind) ValidateReport {
	return ValidateReport{
		Kinds:     kinds,
		Passed:    true,
		Findings:  map[string][]Finding{},
		Generated: time.Now(),
	}
}

func (r *ValidateReport) add(level string, ngIRI string, f Finding) {
	f.Level = level
	r.Findings[ngIRI] = append(r.Findings[ngIRI], f)
	if level == "error" {
		r.Passed = false
	} else {
	}
}

func (r *ValidateReport) Error(ngIRI string, f Finding) { r.add("error", ngIRI, f) }
func (r *ValidateReport) Warn(ngIRI string, f Finding)  { r.add("warning", ngIRI, f) }

func (r *ValidateReport) String() string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) //
	enc.SetIndent("", "  ")  //
	if err := enc.Encode(r); err != nil {
		return fmt.Sprintf(`{"error": "%v"}`, err)
	}
	return buf.String()
}

// FormatHumanReadable returns a human-readable formatted validation report.
func (r *ValidateReport) FormatHumanReadable() string {
	var buf bytes.Buffer

	formatValue := func(s string) string {
		if strings.TrimSpace(s) == "" {
			return "-"
		}
		return s
	}

	// Header
	buf.WriteString(strings.Repeat("=", 80) + "\n")
	buf.WriteString("SST Stage Validation Report\n")
	buf.WriteString(strings.Repeat("=", 80) + "\n")

	// Status
	statusText := "VALIDATION PASSED"
	if !r.Passed {
		statusText = "VALIDATION FAILED"
	}
	buf.WriteString(fmt.Sprintf("Status: %s\n", statusText))
	buf.WriteString(fmt.Sprintf("Generated: %s\n", r.Generated.UTC().Format(time.RFC3339)))

	// Validation kinds
	kindNames := make([]string, len(r.Kinds))
	for i, k := range r.Kinds {
		kindStr := string(k)
		switch k {
		case KindRdfType:
			kindStr = "RdfType"
		case KindDomainRange:
			kindStr = "DomainRange"
		default:
			parts := strings.Split(kindStr, "_")
			for j, part := range parts {
				if len(part) > 0 {
					parts[j] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
				}
			}
			kindStr = strings.Join(parts, "")
		}
		kindNames[i] = kindStr
	}
	buf.WriteString(fmt.Sprintf("Validation Kinds: %s\n", strings.Join(kindNames, ", ")))

	// Count errors and warnings
	totalErrors := 0
	totalWarnings := 0
	for _, findings := range r.Findings {
		for _, f := range findings {
			if f.Level == "error" {
				totalErrors++
			} else if f.Level == "warning" {
				totalWarnings++
			}
		}
	}

	// Summary
	buf.WriteString("Summary:\n")
	buf.WriteString(fmt.Sprintf("  - Total Errors:   %d\n", totalErrors))
	buf.WriteString(fmt.Sprintf("  - Total Warnings: %d\n", totalWarnings))
	buf.WriteString(fmt.Sprintf("  - NamedGraphs:    %d\n", len(r.Findings)))

	// If passed with no findings
	if r.Passed && totalErrors == 0 && totalWarnings == 0 && len(r.Findings) == 0 {
		buf.WriteString("All validations passed successfully!\n")
		buf.WriteString(strings.Repeat("=", 80) + "\n")
		return buf.String()
	}

	// Findings by NamedGraph
	type entry struct {
		iri      string
		findings []Finding
	}
	ordered := make([]entry, 0, len(r.Findings))
	for ngIRI, findings := range r.Findings {
		ordered = append(ordered, entry{iri: ngIRI, findings: findings})
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].iri < ordered[j].iri })

	for _, e := range ordered {
		buf.WriteString(strings.Repeat("=", 80) + "\n")
		buf.WriteString(fmt.Sprintf("NamedGraph: %s\n", e.iri))
		buf.WriteString(strings.Repeat("-", 80) + "\n")

		var errors, warnings []Finding
		for _, f := range e.findings {
			if f.Level == "error" {
				errors = append(errors, f)
			} else if f.Level == "warning" {
				warnings = append(warnings, f)
			}
		}

		buf.WriteString(fmt.Sprintf("ERRORS (%d)\n", len(errors)))
		if len(errors) == 0 {
			buf.WriteString("  (none)\n")
		} else {
			for idx, f := range errors {
				buf.WriteString(fmt.Sprintf("[%d] Rule: %s (Kind: %s)\n", idx+1, f.Rule, f.Kind))
				buf.WriteString(fmt.Sprintf("    Message: %s\n", f.Message))
				buf.WriteString(fmt.Sprintf("    Triple: Subject=%s | Predicate=%s | Object=%s\n",
					formatValue(f.S), formatValue(f.P), formatValue(f.O)))
				if f.Expect != "" {
					buf.WriteString(fmt.Sprintf("    Expected: %s\n", f.Expect))
				}
				if f.Actual != "" {
					buf.WriteString(fmt.Sprintf("    Actual: %s\n", f.Actual))
				}
			}
		}

		if len(errors) > 0 {
			buf.WriteString("\n")
		}
		buf.WriteString(fmt.Sprintf("WARNINGS (%d)\n", len(warnings)))
		if len(warnings) == 0 {
			buf.WriteString("  (none)\n")
		} else {
			for idx, f := range warnings {
				if idx > 0 {
					buf.WriteString("\n")
				}
				buf.WriteString(fmt.Sprintf("[%d] Rule: %s (Kind: %s)\n", idx+1, f.Rule, f.Kind))
				buf.WriteString(fmt.Sprintf("    Message: %s\n", f.Message))
				buf.WriteString(fmt.Sprintf("    Triple: Subject=%s | Predicate=%s | Object=%s\n",
					formatValue(f.S), formatValue(f.P), formatValue(f.O)))
				if f.Expect != "" {
					buf.WriteString(fmt.Sprintf("    Expected: %s\n", f.Expect))
				}
				if f.Actual != "" {
					buf.WriteString(fmt.Sprintf("    Actual: %s\n", f.Actual))
				}
			}
		}
	}

	buf.WriteString(strings.Repeat("=", 80) + "\n")
	return buf.String()
}

// Validate runs the specified validation kinds on the given stage and returns a report.
// If no kinds are specified, all validation kinds are performed by default.
func Validate(stage sst.Stage, kinds ...ValidationKind) (*ValidateReport, error) {
	if len(kinds) == 0 {
		kinds = []ValidationKind{KindRdfType, KindDomainRange, KindFunctionalProperty}
	}
	report := NewReport(kinds...)
	for _, graph := range stage.NamedGraphs() {
		for _, s := range kinds {
			switch s {
			case KindRdfType:
				err := RdfType(graph, &report, outputLog{graph})
				if err != nil {
					return &report, err
				}
			case KindDomainRange:
				err := DomainAndRange(graph, &report, outputLog{graph})
				if err != nil {
					return &report, err
				}

			case KindFunctionalProperty:
				err := FunctionalProperty(graph, &report, outputLog{graph})
				if err != nil {
					return &report, err
				}

			}
		}
	}
	return &report, nil
}
