// Command validate loads the mtgo-bot-api Bot API schema and cross-checks it
// against the live implementation (the methods registered in internal/client),
// reporting coverage gaps and inconsistencies.
//
// It prints:
//   - official methods present in the schema that are NOT registered (missing),
//   - methods registered in the implementation that are NOT in the schema
//     (untracked), unless they are known extensions,
//   - methods whose parameter set is marked incomplete (params_complete=false),
//   - methods flagged as stubs/partial/unsupported in status.json.
//
// The command exits with status 1 if any official method declared in the schema
// is missing from the implementation (a real parity gap), and 0 otherwise.
//
// Usage:
//
//	go run ./schema/cmd/validate
//	go run ./schema/cmd/validate -schema ./schema
//	go run ./schema/cmd/validate -fail-on-incomplete   # also fail on params gaps
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mtgo-labs/mtgo-bot-api/internal/client"
	"github.com/mtgo-labs/mtgo-bot-api/internal/version"
	"github.com/mtgo-labs/mtgo-bot-api/schema"
)

func main() {
	schemaDir := flag.String("schema", defaultSchemaDir(), "path to the schema/ directory")
	failOnIncomplete := flag.Bool("fail-on-incomplete", false, "exit non-zero if any method has params_complete=false")
	jsonOut := flag.Bool("json", false, "emit the report as JSON instead of human-readable text")
	flag.Parse()

	sc, err := schema.Load(*schemaDir)
	if err != nil {
		fatalf("load schema: %v", err)
	}

	registered := client.RegisteredMethods()
	sort.Strings(registered)
	regSet := toSet(registered)

	report := buildReport(sc, registered, regSet)

	exit := 0
	if len(report.MissingOfficial) > 0 {
		exit = 1
	}
	if *failOnIncomplete && len(report.IncompleteParams) > 0 {
		exit = 1
	}

	if *jsonOut {
		printJSON(report)
	} else {
		printText(sc, report)
	}
	os.Exit(exit)
}

type report struct {
	APISchema      string
	APIImpl        string
	TotalSchema    int
	TotalOfficial  int
	TotalImpl      int
	Implemented    int
	MissingOfficial []string // in schema (official) but NOT registered
	Untracked      []string // registered but NOT in schema (potential new/extension methods)
	Extensions     []string // registered, in schema as non-official
	IncompleteParams []string // schema methods with params_complete=false
	Stubs          []stub   // status.json stub/partial/unsupported
}

type stub struct {
	Name  string
	State string
	Note  string
}

func buildReport(sc *schema.Schema, registered []string, regSet map[string]bool) report {
	r := report{
		APISchema: sc.APIVersion,
		APIImpl:   version.BotAPIVersion,
		TotalImpl: len(registered),
	}

	schemaSet := map[string]bool{}
	for i := range sc.Methods {
		m := &sc.Methods[i]
		schemaSet[m.Name] = true
		r.TotalSchema++
		if m.Official {
			r.TotalOfficial++
			if !regSet[m.Name] {
				r.MissingOfficial = append(r.MissingOfficial, m.Name)
				continue
			}
		}
		if regSet[m.Name] {
			r.Implemented++
		}
		if !m.ParamsComplete {
			r.IncompleteParams = append(r.IncompleteParams, m.Name)
		}
		if !m.Official {
			r.Extensions = append(r.Extensions, m.Name)
		}
	}

	for _, name := range registered {
		if !schemaSet[name] {
			r.Untracked = append(r.Untracked, name)
		}
	}

	// Status overrides (stubs/partial/unsupported only; "implemented" is the default).
	for _, m := range sc.Methods {
		st, ok := sc.Status[m.Name]
		if !ok {
			continue
		}
		switch st.State {
		case "stub", "partial", "unsupported":
			r.Stubs = append(r.Stubs, stub{Name: m.Name, State: st.State, Note: st.Note})
		}
	}
	sort.Strings(r.MissingOfficial)
	sort.Strings(r.Untracked)
	sort.Strings(r.Extensions)
	sort.Strings(r.IncompleteParams)
	return r
}

func printText(sc *schema.Schema, r report) {
	versionMatch := r.APISchema == r.APIImpl
	fmt.Printf("mtgo-bot-api Bot API schema coverage\n")
	fmt.Printf("------------------------------------\n")
	fmt.Printf("schema version : %s\n", r.APISchema)
	fmt.Printf("impl  version  : %s", r.APIImpl)
	if !versionMatch {
		fmt.Printf("  (MISMATCH)")
	}
	fmt.Println()
	fmt.Printf("schema methods : %d (official %d)\n", r.TotalSchema, r.TotalOfficial)
	fmt.Printf("impl  methods  : %d\n", r.TotalImpl)
	fmt.Printf("coverage       : %d/%d official implemented (%.1f%%)\n",
		r.TotalOfficial-len(r.MissingOfficial), r.TotalOfficial,
		percent(r.TotalOfficial-len(r.MissingOfficial), r.TotalOfficial))
	fmt.Println()

	section("MISSING official methods (in schema, not registered)", r.MissingOfficial)
	section("UNTRACKED methods (registered, not in schema)", r.Untracked)
	section("Extension methods (non-official)", r.Extensions)
	sectionf("INCOMPLETE parameter sets (%d)", []string{}, r.IncompleteParams)

	if len(r.Stubs) > 0 {
		fmt.Printf("STUBBED / partial methods (%d):\n", len(r.Stubs))
		for _, s := range r.Stubs {
			fmt.Printf("  - %-32s [%s] %s\n", s.Name, s.State, s.Note)
		}
		fmt.Println()
	}

	if len(r.MissingOfficial) > 0 {
		fmt.Printf("RESULT: FAIL — %d official method(s) missing from implementation\n", len(r.MissingOfficial))
	} else {
		fmt.Printf("RESULT: PASS — all %d official schema methods are registered\n", r.TotalOfficial)
	}
}

func section(title string, items []string) {
	sectionf(title, nil, items)
}

func sectionf(title string, _ []string, items []string) {
	if len(items) == 0 {
		fmt.Printf("%s: none\n", title)
		return
	}
	fmt.Printf("%s (%d):\n", title, len(items))
	for _, it := range items {
		fmt.Printf("  - %s\n", it)
	}
	fmt.Println()
}

func percent(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d) * 100
}

func toSet(items []string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, it := range items {
		m[it] = true
	}
	return m
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "validate: "+format+"\n", args...)
	os.Exit(2)
}

// defaultSchemaDir resolves the schema/ directory relative to the module root.
func defaultSchemaDir() string {
	// The binary runs from the module root in the common case. Fall back to a
	// path computed from this source file when running via `go run` from a
	// different working directory.
	if _, err := os.Stat("schema/methods.json"); err == nil {
		return "schema"
	}
	if abs, err := filepath.Abs(filepath.Join(schemaSrcDir(), ".")); err == nil {
		return abs
	}
	return "schema"
}

// schemaSrcDir is the directory of this file at build time, used to locate the
// schema data files when the tool is invoked from an arbitrary cwd.
func schemaSrcDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "schema"
	}
	// `go run` builds a temp binary; the source dir is not recoverable from it,
	// so default to the conventional module-relative path.
	_ = exe
	return filepath.Join("..", "..")
}

func printJSON(r report) {
	var b strings.Builder
	fmt.Fprintf(&b, "{\n")
	fmt.Fprintf(&b, "  \"api_schema_version\": %q,\n", r.APISchema)
	fmt.Fprintf(&b, "  \"api_impl_version\": %q,\n", r.APIImpl)
	fmt.Fprintf(&b, "  \"total_schema_methods\": %d,\n", r.TotalSchema)
	fmt.Fprintf(&b, "  \"total_official_methods\": %d,\n", r.TotalOfficial)
	fmt.Fprintf(&b, "  \"total_impl_methods\": %d,\n", r.TotalImpl)
	fmt.Fprintf(&b, "  \"official_implemented\": %d,\n", r.TotalOfficial-len(r.MissingOfficial))
	fmt.Fprintf(&b, "  \"coverage_percent\": %.1f,\n", percent(r.TotalOfficial-len(r.MissingOfficial), r.TotalOfficial))
	fmt.Fprintf(&b, "  \"missing_official\": %s,\n", jsonList(r.MissingOfficial))
	fmt.Fprintf(&b, "  \"untracked\": %s,\n", jsonList(r.Untracked))
	fmt.Fprintf(&b, "  \"extensions\": %s,\n", jsonList(r.Extensions))
	fmt.Fprintf(&b, "  \"incomplete_params\": %s\n", jsonList(r.IncompleteParams))
	fmt.Fprintf(&b, "}\n")
	fmt.Print(b.String())
}

func jsonList(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	quoted := make([]string, len(items))
	for i, it := range items {
		quoted[i] = fmt.Sprintf("%q", it)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}
