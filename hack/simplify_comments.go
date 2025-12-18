/*
Copyright 2025 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: go run hack/simplify_comments.go <directory>\n")
		os.Exit(1)
	}

	root := os.Args[1]
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if strings.Contains(path, "generated") {
			return nil
		}

		return processFile(path)
	})

	if err != nil {
		panic(err)
	}
}

func processFile(path string) error {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return err
	}

	modified := false

	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			if genDecl.Lparen == token.NoPos && genDecl.Doc != nil {
				if simplifyDoc(genDecl.Doc, typeSpec.Name.Name) {
					modified = true
				}
			} else if typeSpec.Doc != nil {
				if simplifyDoc(typeSpec.Doc, typeSpec.Name.Name) {
					modified = true
				}
			}

			if structType, ok := typeSpec.Type.(*ast.StructType); ok {
				for _, field := range structType.Fields.List {
					name := ""
					if len(field.Names) > 0 {
						name = field.Names[0].Name
					} else {
						if ident, ok := field.Type.(*ast.Ident); ok {
							name = ident.Name
						} else if star, ok := field.Type.(*ast.StarExpr); ok {
							if ident, ok := star.X.(*ast.Ident); ok {
								name = ident.Name
							}
						}
					}

					if name != "" {
						if simplifyDoc(field.Doc, name) {
							modified = true
						}
					}
				}
			}
		}
	}

	if modified {
		file, err := os.Create(path)
		if err != nil {
			return err
		}
		defer file.Close()
		if err := format.Node(file, fset, f); err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, path)
	}
	return nil
}

func simplifyDoc(doc *ast.CommentGroup, name string) bool {
	if doc == nil || len(doc.List) == 0 {
		return false
	}

	var descIndices []int
	var markerIndices []int

	for i, c := range doc.List {
		text := strings.TrimPrefix(c.Text, "//")
		trimmed := strings.TrimSpace(text)
		if strings.HasPrefix(trimmed, "+") || strings.HasPrefix(trimmed, "Deprecated:") {
			markerIndices = append(markerIndices, i)
		} else {
			descIndices = append(descIndices, i)
		}
	}

	var newComments []*ast.Comment
	changed := false

	if len(descIndices) > 0 {
		// Existing description found. Reuse the LAST description line to maintain adjacency.
		targetIdx := descIndices[len(descIndices)-1]

		for i, c := range doc.List {
			switch {
			case i == targetIdx:
				newText := "// " + name
				if c.Text != newText {
					c.Text = newText
					changed = true
				}
				newComments = append(newComments, c)
			case contains(markerIndices, i):
				newComments = append(newComments, c)
			default:
				// Drop other description lines
				changed = true
			}
		}
	} else {
		// Only markers found. Hijack the FIRST marker to insert the name.
		targetIdx := markerIndices[0]

		for i, c := range doc.List {
			if i == targetIdx {
				originalText := c.Text
				newText := "// " + name
				if c.Text != newText {
					c.Text = newText
				}
				newComments = append(newComments, c)

				// Re-insert the original marker as a new comment
				newComments = append(newComments, &ast.Comment{Text: originalText})
				changed = true
			} else {
				newComments = append(newComments, c)
			}
		}
	}

	if changed {
		doc.List = newComments
	}
	return changed
}

func contains(indices []int, target int) bool {
	for _, i := range indices {
		if i == target {
			return true
		}
	}
	return false
}
