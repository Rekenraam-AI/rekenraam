// #284 slice 6: provenance of testify equality assertions in Go tests.
// Each side is classified by where it comes from:
// "literal-or-test" (no production call), or "production" (a production call
// directly, or through a local identifier assigned from one in the same
// function). Both sides production = differential/agreement assertion.
package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var skipQual = map[string]bool{"fmt": true, "strings": true, "strconv": true, "time": true, "json": true, "require": true, "assert": true, "errors": true, "sort": true, "slices": true, "maps": true, "big": true, "math": true, "sql": true, "context": true, "bytes": true, "http": true, "httptest": true, "url": true, "os": true, "filepath": true, "base64": true, "hex": true, "sha256": true, "regexp": true, "utf8": true, "io": true, "t": true, "exact": true}

// exact.* numeric leaves only reshape literals, so the skip list excludes them
// from production result producers along with stdlib and testify selectors.
func main() {
	root := os.Args[1]
	prod := map[string]bool{}
	testFuncs := map[string]bool{}
	filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), p, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil
		}
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				if strings.HasSuffix(p, "_test.go") {
					testFuncs[fd.Name.Name] = true
				} else {
					prod[fd.Name.Name] = true
				}
			}
		}
		return nil
	})
	isProdCall := func(c *ast.CallExpr) string {
		switch fn := c.Fun.(type) {
		case *ast.Ident:
			if prod[fn.Name] && !testFuncs[fn.Name] {
				return fn.Name
			}
		case *ast.SelectorExpr:
			q := ""
			if x, ok := fn.X.(*ast.Ident); ok {
				q = x.Name
			}
			if skipQual[q] {
				return ""
			}
			if prod[fn.Sel.Name] && !testFuncs[fn.Sel.Name] {
				return fn.Sel.Name
			}
		}
		return ""
	}
	callsIn := func(e ast.Node) []string {
		var out []string
		ast.Inspect(e, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok {
				if s := isProdCall(c); s != "" {
					out = append(out, s)
				}
			}
			return true
		})
		return out
	}
	w := csv.NewWriter(os.Stdout)
	w.Write([]string{"file", "line", "func", "assertion", "expected_source", "actual_source", "expected_calls", "actual_calls", "expected_expr"})
	filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, "_test.go") {
			return nil
		}
		fs := token.NewFileSet()
		f, err := parser.ParseFile(fs, p, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			origin := map[string][]string{}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				if a, ok := n.(*ast.AssignStmt); ok {
					var calls []string
					for _, r := range a.Rhs {
						calls = append(calls, callsIn(r)...)
						if id, ok := r.(*ast.Ident); ok {
							calls = append(calls, origin[id.Name]...)
						}
						if sel, ok := r.(*ast.SelectorExpr); ok {
							if id, ok := baseIdent(sel); ok {
								calls = append(calls, origin[id]...)
							}
						}
					}
					for _, l := range a.Lhs {
						if id, ok := l.(*ast.Ident); ok && id.Name != "_" && id.Name != "err" {
							if len(calls) > 0 {
								origin[id.Name] = calls
							} else if a.Tok == token.DEFINE {
								delete(origin, id.Name)
							}
						}
					}
				}
				return true
			})
			src := func(e ast.Expr) []string {
				calls := callsIn(e)
				ast.Inspect(e, func(n ast.Node) bool {
					if id, ok := n.(*ast.Ident); ok {
						calls = append(calls, origin[id.Name]...)
					}
					return true
				})
				return uniq(calls)
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				c, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := c.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				q, _ := sel.X.(*ast.Ident)
				if q == nil || (q.Name != "require" && q.Name != "assert") {
					return true
				}
				switch sel.Sel.Name {
				case "Equal", "EqualValues", "Exactly", "JSONEq", "ElementsMatch", "InDelta":
				default:
					return true
				}
				if len(c.Args) < 3 {
					return true
				}
				ex, ac := src(c.Args[1]), src(c.Args[2])
				kind := func(s []string) string {
					if len(s) == 0 {
						return "literal-or-test"
					}
					return "production"
				}
				var b bytes.Buffer
				printer.Fprint(&b, fs, c.Args[1])
				expr := strings.Join(strings.Fields(b.String()), " ")
				if len(expr) > 120 {
					expr = expr[:120] + "…"
				}
				w.Write([]string{rel, fmt.Sprint(fs.Position(c.Pos()).Line), fd.Name.Name, sel.Sel.Name, kind(ex), kind(ac), strings.Join(ex, " "), strings.Join(ac, " "), expr})
				return true
			})
		}
		return nil
	})
	w.Flush()
}

func baseIdent(e ast.Expr) (string, bool) {
	for {
		switch x := e.(type) {
		case *ast.Ident:
			return x.Name, true
		case *ast.SelectorExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		default:
			return "", false
		}
	}
}

func uniq(s []string) []string {
	m := map[string]bool{}
	var o []string
	for _, x := range s {
		if !m[x] {
			m[x] = true
			o = append(o, x)
		}
	}
	sort.Strings(o)
	return o
}
