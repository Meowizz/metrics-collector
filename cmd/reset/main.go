package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
)

func main() {

	fset := token.NewFileSet()
	err := filepath.WalkDir("../..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			fmt.Printf("Ошибка парсинга файла %s: %v\n", path, err) // TODO: Поправить на логгер?
			return nil
		}

		ast.Inspect(file, func(n ast.Node) bool {
			genDecl, ok := n.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				return true
			}

			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, isStruct := typeSpec.Type.(*ast.StructType); !isStruct {
					continue
				}
				doc := genDecl.Doc
				if doc == nil {
					doc = typeSpec.Doc
				}

				if doc != nil {
					if strings.Contains(doc.Text(), "generate:reset") {
						fmt.Printf(" найдена структура: %-20s (файл: %s)\n", typeSpec.Name.Name, path) // TODO: Прихранить это в мапе или чем-то таком
					}
				}
			}

			return true
		})

		return nil
	})

	if err != nil {
		fmt.Printf("ошибка при сканировании директории: %v\n", err) // TODO: Поправить на логгер?
	}
}
