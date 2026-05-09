package lsp

import (
	"fmt"
	"go/ast"
	"go/types"
	"path/filepath"

	"golang.org/x/tools/go/packages"
)

func ListInterfaceLenses(fileURI string) ([]CodeLens, error) {
	filePath := URIToPath(fileURI)
	dir := filepath.Dir(filePath)

	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo,
		Dir: dir,
	}

	// Load all packages
	// TODO: check if there is any better ways
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("load packages: %w", err)
	}

	var targetPkg *packages.Package
	for _, pkg := range pkgs {
		for _, f := range pkg.GoFiles {
			if f == filePath {
				targetPkg = pkg
				break
			}
		}
	}
	if targetPkg == nil {
		return nil, fmt.Errorf("no target package for file")
	}

	// Collect all named types across all packages
	var allTypes []*types.Named
	for _, pkg := range pkgs {
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			obj := scope.Lookup(name)
			if tn, ok := obj.(*types.TypeName); ok {
				if named, ok := tn.Type().(*types.Named); ok {
					allTypes = append(allTypes, named)
				}
			}
		}
	}

	fset := targetPkg.Fset

	// Parse the file for AST position info
	var targetFile *ast.File
	for _, f := range targetPkg.Syntax {
		pos := targetPkg.Fset.File(f.Pos())
		if pos != nil && pos.Name() == filePath {
			targetFile = f
			break
		}
	}
	if targetFile == nil {
		return nil, fmt.Errorf("AST not found for file")
	}

	// Find interface type declaration by walking into the AST
	var lenses []CodeLens
	ast.Inspect(targetFile, func(n ast.Node) bool {
		typeSpec, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}

		ifaceType, ok := typeSpec.Type.(*ast.InterfaceType)
		if !ok || ifaceType.Methods == nil || len(ifaceType.Methods.List) == 0 {
			return true
		}

		obj := targetPkg.TypesInfo.Defs[typeSpec.Name]
		if obj == nil {
			return true
		}

		named, ok := obj.Type().(*types.Named)
		if !ok {
			return true
		}

		iface, ok := named.Underlying().(*types.Interface)
		if !ok {
			return true
		}

		// Count implementors across all types
		count := 0
		for _, t := range allTypes {
			if types.Implements(t, iface) || types.Implements(types.NewPointer(t), iface) {
				count++
			}
		}

		// Build CodeLens at the line of the interface keyword
		startPos := fset.Position(typeSpec.Name.Pos())
		line := startPos.Line - 1

		label := fmt.Sprintf("%d implementation", count)
		if count != 1 {
			label += "s"
		}

		lenses = append(lenses, CodeLens{
			Range: Range{
				Start: Position{Line: uint(line), Character: 0},
				End:   Position{Line: uint(line), Character: 0},
			},
			Command: &Command{
				Title:   label,
				Command: "editor::GoToImplementation",
			},
		})

		return true
	})

	return lenses, nil
}
