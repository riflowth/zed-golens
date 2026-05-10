package lsp

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"slices"

	"golang.org/x/tools/go/packages"
)

func ListInterfaceLenses(fileURI string) ([]CodeLens, error) {
	filePath := URIToPath(fileURI)
	fileDir := filepath.Dir(filePath)

	// Find the module root so package loading with "./..." covers the entrie module
	moduleRoot, err := findModuleRoot(fileDir)
	if err != nil {
		moduleRoot = fileDir
	}

	// Load all packages in the module
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		Dir:  moduleRoot,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("load packages: %w", err)
	}

	// Find the package that contains the file being edited
	var targetPkg *packages.Package
	for _, pkg := range pkgs {
		if slices.Contains(pkg.GoFiles, filePath) {
			targetPkg = pkg
			break
		}
	}
	if targetPkg == nil {
		return nil, fmt.Errorf("no target package for file: %s", filePath)
	}

	// Find the AST file for the target file
	var targetFile *ast.File
	for _, f := range targetPkg.Syntax {
		pos := targetPkg.Fset.File(f.Pos())
		if pos != nil && pos.Name() == filePath {
			targetFile = f
			break
		}
	}
	if targetFile == nil {
		return nil, fmt.Errorf("AST not found for file: %s", filePath)
	}

	// Collect all named types across all packages for implementation count
	type namedWithLocation struct {
		named *types.Named
		loc   Location
	}
	allTypes := make(map[string]namedWithLocation)
	for _, pkg := range pkgs {
		if pkg.Types == nil {
			continue
		}
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			obj := scope.Lookup(name)
			tn, ok := obj.(*types.TypeName)
			if !ok {
				continue
			}
			named, ok := tn.Type().(*types.Named)
			if !ok {
				continue
			}

			pos := pkg.Fset.Position(obj.Pos())
			key := typeObjectKey(obj)

			allTypes[key] = namedWithLocation{
				named: named,
				loc: Location{
					URI: PathToURI(pos.Filename),
					Range: Range{
						Start: Position{
							Line:      uint(pos.Line - 1),
							Character: uint(pos.Column - 1),
						},
						End: Position{
							Line:      uint(pos.Line - 1),
							Character: uint(pos.Column - 1 + len(named.Obj().Name())),
						},
					},
				},
			}
		}
	}

	// Collect all references across all packages for reference count
	refMap := make(map[string][]Location)
	for _, pkg := range pkgs {
		if pkg.TypesInfo == nil {
			continue
		}
		for ident, obj := range pkg.TypesInfo.Uses {
			tn, ok := obj.(*types.TypeName)
			if !ok {
				continue
			}
			named, ok := tn.Type().(*types.Named)
			if !ok {
				continue
			}
			if _, ok := named.Underlying().(*types.Interface); !ok {
				continue
			}

			pos := pkg.Fset.Position(ident.Pos())
			end := pkg.Fset.Position(ident.End())
			key := typeObjectKey(obj)

			refMap[key] = append(refMap[key], Location{
				URI: PathToURI(pos.Filename),
				Range: Range{
					Start: Position{
						Line:      uint(pos.Line - 1),
						Character: uint(pos.Column - 1),
					},
					End: Position{
						Line:      uint(end.Line - 1),
						Character: uint(end.Column - 1),
					},
				},
			})
		}
	}

	// Walk the AST to find interface type declaration and build CodeLens
	var lenses []CodeLens
	ast.Inspect(targetFile, func(n ast.Node) bool {
		typeSpec, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}

		// Only process non-empty interfaces
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

		// Collect locations of all types that implement this interface
		ifaceKey := typeObjectKey(obj)
		var implLocs []Location
		for _, t := range allTypes {
			// Skip the interface itself
			if typeObjectKey(t.named.Obj()) == ifaceKey {
				continue
			}
			if types.Implements(t.named, iface) || types.Implements(types.NewPointer(t.named), iface) {
				implLocs = append(implLocs, t.loc)
			}
		}

		// Build implementation label
		implCount := len(implLocs)
		implLabel := fmt.Sprintf("%d implementation", implCount)
		if implCount != 1 {
			implLabel += "s"
		}

		// Build reference label
		refLocs := refMap[typeObjectKey(obj)]
		refCount := len(refLocs)
		refLabel := fmt.Sprintf("%d reference", refCount)
		if refCount != 1 {
			refLabel += "s"
		}

		// Build CodeLens at the line of the interface keyword
		startPos := targetPkg.Fset.Position(typeSpec.Name.Pos())
		endPos := targetPkg.Fset.Position(typeSpec.Name.End())
		line := startPos.Line - 1
		codeLensRange := Range{
			Start: Position{Line: uint(line), Character: uint(startPos.Column - 1)},
			End:   Position{Line: uint(line), Character: uint(endPos.Column - 1)},
		}

		lenses = append(lenses,
			CodeLens{
				Range: codeLensRange,
				Command: &Command{
					Title:   implLabel,
					Command: "editor.action.peekLocations",
					Args:    []any{fileURI, codeLensRange.Start, implLocs},
				},
			},
			CodeLens{
				Range: codeLensRange,
				Command: &Command{
					Title:   refLabel,
					Command: "editor.action.peekLocations",
					Args:    []any{fileURI, codeLensRange.Start, refLocs},
				},
			},
		)

		return true
	})

	return lenses, nil
}

func typeObjectKey(obj types.Object) string {
	if obj.Pkg() == nil {
		return obj.Name()
	}
	return obj.Pkg().Path() + "." + obj.Name()
}

func findModuleRoot(dir string) (string, error) {
	current := dir
	for {
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return dir, fmt.Errorf("go.mod not found, falling back to dir")
		}
		current = parent
	}
}
