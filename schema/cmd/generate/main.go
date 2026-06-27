// Command generate emits human-readable artifacts from the schema: a Markdown
// coverage/coverage report and a per-category method index. It complements
// validate (which cross-checks the live implementation) by producing docs that
// contributors and SDK authors can read directly.
//
// Outputs:
//
//   -report schema/COVERAGE.md    coverage table + missing/untracked lists
//   -index   schema/METHODS.md    grouped method reference
//
// When the live implementation is available (internal/client can be imported),
// the report annotates each method with its implementation status.
//
// Usage:
//
//	go run ./schema/cmd/generate                       # writes both into ./schema
//	go run ./schema/cmd/generate -report COVERAGE.md   # only the coverage report
//	go run ./schema/cmd/generate -index   METHODS.md   # only the method index
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mtgo-labs/mtgo-bot-api/internal/client"
	"github.com/mtgo-labs/mtgo-bot-api/internal/version"
	"github.com/mtgo-labs/mtgo-bot-api/schema"
)

func main() {
	schemaDir := flag.String("schema", "schema", "path to the schema/ directory")
	reportPath := flag.String("report", "schema/COVERAGE.md", "output coverage report (- to print to stdout, empty to skip)")
	indexPath := flag.String("index", "schema/METHODS.md", "output method index (- to print to stdout, empty to skip)")
	flag.Parse()

	sc, err := schema.Load(*schemaDir)
	check(err, "load schema")

	registered := toSet(client.RegisteredMethods())

	if *reportPath != "" {
		check(writeTarget(*reportPath, coverageReport(sc, registered)), "write report")
	}
	if *indexPath != "" {
		check(writeTarget(*indexPath, methodIndex(sc, registered)), "write index")
	}
	fmt.Println("generate: wrote", *reportPath, "and", *indexPath)
}

// coverageReport builds a Markdown summary of schema vs implementation parity.
func coverageReport(sc *schema.Schema, registered map[string]bool) string {
	var missing, untracked, implemented []string
	for i := range sc.Methods {
		m := &sc.Methods[i]
		if registered[m.Name] {
			implemented = append(implemented, m.Name)
			continue
		}
		if m.Official {
			missing = append(missing, m.Name)
		}
	}
	for name := range registered {
		if sc.MethodByName(name) == nil {
			untracked = append(untracked, name)
		}
	}
	sort.Strings(missing)
	sort.Strings(untracked)
	sort.Strings(implemented)

	var b strings.Builder
	fmt.Fprintf(&b, "# Bot API Coverage Report\n\n")
	fmt.Fprintf(&b, "- **Schema version:** %s\n", sc.APIVersion)
	fmt.Fprintf(&b, "- **Implementation version:** %s\n", version.BotAPIVersion)
	fmt.Fprintf(&b, "- **Schema methods:** %d (official %d)\n", len(sc.Methods), countOfficial(sc))
	fmt.Fprintf(&b, "- **Implemented:** %d\n", len(implemented))
	official := countOfficial(sc)
	fmt.Fprintf(&b, "- **Official coverage:** %d/%d (%.1f%%)\n\n", official-len(missing), official, pct(official-len(missing), official))

	fmt.Fprintf(&b, "## Missing official methods (%d)\n\n", len(missing))
	if len(missing) == 0 {
		b.WriteString("_None — all official schema methods are registered._\n")
	} else {
		for _, n := range missing {
			fmt.Fprintf(&b, "- `%s`\n", n)
		}
	}

	fmt.Fprintf(&b, "\n## Untracked methods (registered, not in schema) (%d)\n\n", len(untracked))
	if len(untracked) == 0 {
		b.WriteString("_None._\n")
	} else {
		for _, n := range untracked {
			fmt.Fprintf(&b, "- `%s`\n", n)
		}
	}

	fmt.Fprintf(&b, "\n## Incomplete parameter sets (%d)\n\n", 0)
	inc := incompleteMethods(sc)
	if len(inc) == 0 {
		b.WriteString("_None._\n")
	} else {
		for _, n := range inc {
			fmt.Fprintf(&b, "- `%s`\n", n)
		}
	}

	fmt.Fprintf(&b, "\n## Status overrides\n\n")
	if len(sc.Status) == 0 {
		b.WriteString("_None._\n")
	} else {
		names := make([]string, 0, len(sc.Status))
		for k := range sc.Status {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, n := range names {
			fmt.Fprintf(&b, "- `%s` — **%s**: %s\n", n, sc.Status[n].State, sc.Status[n].Note)
		}
	}
	return b.String()
}

// methodIndex builds a Markdown reference grouped by category, each entry
// annotated with implementation status.
func methodIndex(sc *schema.Schema, registered map[string]bool) string {
	byCat := map[string][]schema.Method{}
	for i := range sc.Methods {
		cat := sc.Methods[i].Category
		if cat == "" {
			cat = "Uncategorized"
		}
		byCat[cat] = append(byCat[cat], sc.Methods[i])
	}
	cats := make([]string, 0, len(byCat))
	for c := range byCat {
		cats = append(cats, c)
	}
	sort.Strings(cats)

	var b strings.Builder
	fmt.Fprintf(&b, "# Bot API Method Index\n\n")
	fmt.Fprintf(&b, "Generated from schema version **%s**.\n\n", sc.APIVersion)
	fmt.Fprintf(&b, "`+` = implemented, `~` = deprecated/extension, `-` = missing.\n\n")

	for _, cat := range cats {
		ms := byCat[cat]
		sort.Slice(ms, func(i, j int) bool { return ms[i].Name < ms[j].Name })
		fmt.Fprintf(&b, "## %s (%d)\n\n", cat, len(ms))
		b.WriteString("| Method | Returns | Status |\n|--------|---------|--------|\n")
		for _, m := range ms {
			st := "-"
			switch {
			case registered[m.Name]:
				if m.Official {
					st = "+"
				} else {
					st = "~"
				}
			case !m.Official:
				st = "~"
			}
			fmt.Fprintf(&b, "| `%s` | `%s` | %s |\n", m.Title, orDash(m.Returns), st)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func incompleteMethods(sc *schema.Schema) []string {
	var out []string
	for i := range sc.Methods {
		if !sc.Methods[i].ParamsComplete {
			out = append(out, sc.Methods[i].Name)
		}
	}
	sort.Strings(out)
	return out
}

func countOfficial(sc *schema.Schema) int {
	n := 0
	for i := range sc.Methods {
		if sc.Methods[i].Official {
			n++
		}
	}
	return n
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func pct(n, d int) float64 {
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

func writeTarget(path, content string) error {
	if path == "-" {
		fmt.Print(content)
		return nil
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func check(err error, what string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate: %s: %v\n", what, err)
		os.Exit(1)
	}
}
