//go:build ignore

// Shape checks of the Go API that grep cannot see (go/parser based). Run by scripts/check-architecture.sh as
//
//	go run scripts/check-go-shape.go [repo root]
//
//	usecase-shape      every method of an operation interface in api/internal/usecase/*.go is
//	                   Method(ctx context.Context, in input.X[, callback func(...)]) (output.X, error) or ... error,
//	                   where X is the interface name, and input.X (and output.X when returned) is declared in
//	                   usecase/input and usecase/output. usecase/helper is exempt.
//	output-own-fields  a struct in usecase/output never has a field typed with a domain/model type.
//	gateway-params     a domain/gateway method has at most 3 params after ctx (more: one model.<Action><Model>Params).
//	business-imports   domain, usecase and service import only the standard library and api/internal/{domain,usecase}.
//
// Output matches the shell checks (FAIL / why / fix); the exit code is 1 on a violation.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const modPrefix = "github.com/vuhn/fga-demo/api/internal/"

var failed bool

func violation(what, why, fix string) {
	fmt.Printf("\nFAIL: %s\n  why: %s\n  fix: %s\n", what, why, fix)
	failed = true
}

func parseDir(dir string, withTests bool) (*token.FileSet, map[string]*ast.File) {
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || (!withTests && strings.HasSuffix(n, "_test.go")) {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, n), nil, parser.SkipObjectResolution)
		if err != nil {
			violation(fmt.Sprintf("%s does not parse: %v", filepath.Join(dir, n), err), "the checks read the syntax tree", "fix the syntax error")
			continue
		}
		files[n] = f
	}
	return fset, files
}

func typeNames(files map[string]*ast.File) map[string]bool {
	out := map[string]bool{}
	for _, f := range files {
		for _, d := range f.Decls {
			if g, ok := d.(*ast.GenDecl); ok {
				for _, s := range g.Specs {
					if t, ok := s.(*ast.TypeSpec); ok {
						out[t.Name.Name] = true
					}
				}
			}
		}
	}
	return out
}

func isSel(e ast.Expr, pkg, name string) bool {
	s, ok := e.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	x, ok := s.X.(*ast.Ident)
	return ok && x.Name == pkg && (name == "" || s.Sel.Name == name)
}

// fields flattens a param list to one type expression per parameter.
func fields(l *ast.FieldList) []ast.Expr {
	var out []ast.Expr
	if l == nil {
		return out
	}
	for _, f := range l.List {
		n := len(f.Names)
		if n == 0 {
			n = 1
		}
		for i := 0; i < n; i++ {
			out = append(out, f.Type)
		}
	}
	return out
}

func interfaces(files map[string]*ast.File) map[string][]*ast.TypeSpec {
	out := map[string][]*ast.TypeSpec{}
	for name, f := range files {
		for _, d := range f.Decls {
			if g, ok := d.(*ast.GenDecl); ok {
				for _, s := range g.Specs {
					if t, ok := s.(*ast.TypeSpec); ok {
						if _, ok := t.Type.(*ast.InterfaceType); ok {
							out[name] = append(out[name], t)
						}
					}
				}
			}
		}
	}
	return out
}

func checkUsecases(root string) {
	dir := filepath.Join(root, "api/internal/usecase")
	_, files := parseDir(dir, false)
	_, inFiles := parseDir(filepath.Join(dir, "input"), false)
	_, outFiles := parseDir(filepath.Join(dir, "output"), false)
	inputs, outputs := typeNames(inFiles), typeNames(outFiles)
	for file, specs := range interfaces(files) {
		for _, t := range specs {
			x := t.Name.Name
			for _, m := range t.Type.(*ast.InterfaceType).Methods.List {
				ft, ok := m.Type.(*ast.FuncType)
				if !ok || len(m.Names) == 0 {
					continue // an embedded interface
				}
				where := fmt.Sprintf("usecase/%s: %s.%s", file, x, m.Names[0].Name)
				ps, rs := fields(ft.Params), fields(ft.Results)
				ok = len(ps) >= 2 && len(ps) <= 3 && isSel(ps[0], "context", "Context") && isSel(ps[1], "input", x)
				if ok && len(ps) == 3 {
					_, ok = ps[2].(*ast.FuncType) // a streaming callback is the only extra param
				}
				if ok {
					switch {
					case len(rs) == 1:
						ok = isIdent(rs[0], "error")
					case len(rs) == 2:
						ok = isSel(rs[0], "output", x) && isIdent(rs[1], "error")
					default:
						ok = false
					}
				}
				if !ok {
					violation(where+" does not have the operation shape",
						"every usecase method is Method(ctx context.Context, in input."+x+"[, callback]) (output."+x+", error) or only error; actor, tenant and ids live in the input struct",
						"move the params into usecase/input/<domain>.go (type "+x+" struct{...}) and the result into usecase/output/<domain>.go")
					continue
				}
				if !inputs[x] {
					violation(where+": input."+x+" is not declared in usecase/input", "the input struct is named after the interface", "add `type "+x+" struct{...}` in usecase/input/<domain>.go")
				}
				if len(rs) == 2 && !outputs[x] {
					violation(where+": output."+x+" is not declared in usecase/output", "the output struct is named after the interface", "add `type "+x+" ...` in usecase/output/<domain>.go")
				}
			}
		}
	}
}

func isIdent(e ast.Expr, name string) bool {
	i, ok := e.(*ast.Ident)
	return ok && i.Name == name
}

func hasModel(e ast.Expr) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if s, ok := n.(*ast.SelectorExpr); ok && isSel(s, "model", "") {
			found = true
		}
		return !found
	})
	return found
}

func checkOutputs(root string) {
	fset, files := parseDir(filepath.Join(root, "api/internal/usecase/output"), false)
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		for _, d := range files[n].Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, s := range g.Specs {
				t, ok := s.(*ast.TypeSpec)
				if !ok || !hasModel(t.Type) {
					continue
				}
				violation(fmt.Sprintf("usecase/output/%s: %s uses a domain/model type (%s)", n, t.Name.Name, fset.Position(t.Pos())),
					"an output is its own struct with copied fields, so a handler never sees a model through a usecase",
					"copy the fields into the output struct and map model -> output in the service (service/<model>_output.go)")
			}
		}
	}
}

func checkGateways(root string) {
	fset, files := parseDir(filepath.Join(root, "api/internal/domain/gateway"), false)
	for n, specs := range interfaces(files) {
		for _, t := range specs {
			for _, m := range t.Type.(*ast.InterfaceType).Methods.List {
				ft, ok := m.Type.(*ast.FuncType)
				if !ok || len(m.Names) == 0 {
					continue
				}
				ps := fields(ft.Params)
				if len(ps) > 0 && isSel(ps[0], "context", "Context") {
					ps = ps[1:]
				}
				if len(ps) > 3 {
					violation(fmt.Sprintf("gateway/%s: %s.%s has %d params after ctx (%s)", n, t.Name.Name, m.Names[0].Name, len(ps), fset.Position(m.Pos())),
						"a gateway method takes at most 3 params after ctx; more travel as one model.<Action><Model>Params struct",
						"add the struct in api/internal/domain/model/<model>_params.go (or reuse a fitting one) and take it as the single param")
				}
			}
		}
	}
}

func checkImports(root string) {
	for _, layer := range []string{"domain", "usecase", "service"} {
		base := filepath.Join(root, "api/internal", layer)
		_ = filepath.WalkDir(base, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			f, err := parser.ParseFile(token.NewFileSet(), p, nil, parser.ImportsOnly)
			if err != nil {
				return nil
			}
			for _, im := range f.Imports {
				path := strings.Trim(im.Path.Value, `"`)
				first := strings.SplitN(path, "/", 2)[0]
				std := !strings.Contains(first, ".")
				business := strings.HasPrefix(path, modPrefix+"domain") || strings.HasPrefix(path, modPrefix+"usecase")
				if layer == "service" && strings.HasPrefix(path, modPrefix+"service") {
					business = true
				}
				if !std && !business {
					rel, _ := filepath.Rel(root, p)
					violation(fmt.Sprintf("%s imports %s", rel, path),
						"domain, usecase and service are the business layer: they import only the standard library and each other, never infrastructure/, handler/, cmd/ or a third-party package",
						"put the dependency behind a gateway interface in domain/gateway and implement it under infrastructure/")
				}
			}
			return nil
		})
	}
}

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	checkUsecases(root)
	checkOutputs(root)
	checkGateways(root)
	checkImports(root)
	if failed {
		os.Exit(1)
	}
}
