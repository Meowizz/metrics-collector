package osexitcheck

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "osexitcheck",
	Doc:  "запрещает прямой вызов os.Exit в функции main",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	if pass.Pkg.Name() != "main" {
		return nil, nil
	}
	for _, file := range pass.Files {
		if ast.IsGenerated(file) {
			continue
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Name.Name != "main" || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				obj, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
				if ok && obj.Pkg() != nil && obj.Pkg().Path() == "os" && obj.Name() == "Exit" {
					pass.Reportf(call.Pos(), "прямой вызов os.Exit в функции main запрещён")
				}
				return true
			})
		}
	}
	return nil, nil
}
