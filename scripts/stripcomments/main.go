package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	dryRun := flag.Bool("dry", false, "report only")
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: stripcomments [-dry] <root>...")
		os.Exit(2)
	}
	for _, root := range flag.Args() {
		if err := walk(root, *dryRun); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

func walk(root string, dry bool) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == "vendor" || name == "node_modules" || name == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_generated.go") || strings.HasSuffix(path, ".pb.go") {
			return nil
		}
		return processFile(path, dry)
	})
}

func isDirective(text string) bool {
	t := strings.TrimSpace(text)
	if strings.HasPrefix(t, "//go:") ||
		strings.HasPrefix(t, "// +build") ||
		strings.HasPrefix(t, "//+build") ||
		strings.HasPrefix(t, "//nolint") ||
		strings.HasPrefix(t, "//lint:") ||
		strings.HasPrefix(t, "//export ") ||
		strings.HasPrefix(t, "// Code generated") {
		return true
	}
	return false
}

func processFile(path string, dry bool) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	type span struct{ start, end token.Pos }
	var bodies []span
	ast.Inspect(file, func(n ast.Node) bool {
		switch fn := n.(type) {
		case *ast.FuncDecl:
			if fn.Body != nil {
				bodies = append(bodies, span{fn.Body.Lbrace, fn.Body.Rbrace})
			}
		case *ast.FuncLit:
			if fn.Body != nil {
				bodies = append(bodies, span{fn.Body.Lbrace, fn.Body.Rbrace})
			}
		}
		return true
	})
	inBody := func(p token.Pos) bool {
		for _, b := range bodies {
			if p > b.start && p < b.end {
				return true
			}
		}
		return false
	}

	var kept []*ast.CommentGroup
	removed := 0
	for _, cg := range file.Comments {
		var newList []*ast.Comment
		for _, c := range cg.List {
			if inBody(c.Pos()) && !isDirective(c.Text) {
				removed++
				continue
			}
			newList = append(newList, c)
		}
		if len(newList) > 0 {
			cg.List = newList
			kept = append(kept, cg)
		}
	}
	if removed == 0 {
		return nil
	}
	file.Comments = kept

	if dry {
		fmt.Printf("%s: would remove %d comment(s)\n", path, removed)
		return nil
	}

	var buf bytes.Buffer
	cfg := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 8}
	if err := cfg.Fprint(&buf, fset, file); err != nil {
		return fmt.Errorf("print %s: %w", path, err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return err
	}
	fmt.Printf("%s: removed %d comment(s)\n", path, removed)
	return nil
}
