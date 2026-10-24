// Hash every Go function declaration for #284 slice 6 source reconciliation.
// Output: file,function,receiver,lines,sha256 of the printed declaration.
// Pass "tests" as the second argument to index _test.go files instead.
package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	root := os.Args[1]
	tests := len(os.Args) > 2 && os.Args[2] == "tests"
	filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") {
			return nil
		}
		if strings.HasSuffix(p, "_test.go") != tests {
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
			if !ok {
				continue
			}
			recv := ""
			if fd.Recv != nil && len(fd.Recv.List) > 0 {
				var b bytes.Buffer
				printer.Fprint(&b, fs, fd.Recv.List[0].Type)
				recv = strings.TrimPrefix(b.String(), "*")
			}
			var b bytes.Buffer
			printer.Fprint(&b, token.NewFileSet(), fd)
			start, end := fs.Position(fd.Pos()).Line, fs.Position(fd.End()).Line
			fmt.Printf("%s,%s,%s,%d,%x\n", rel, fd.Name.Name, recv, end-start+1, sha256.Sum256(b.Bytes()))
		}
		return nil
	})
}
