package generator

import (
	"fmt"
	"go/types"
	"regexp"
	"strings"

	"github.com/pengdaCN/goverter/builder"

	"golang.org/x/tools/go/packages"

	"github.com/pengdaCN/goverter/xtype"
	"github.com/pkg/errors"
)

const (
	// packageNameSep separates between package path and name pattern
	// in goverter:extend input with package path.
	packageNameSep = ":"
)

type parseExtendContext struct {
	// extend map[xtype.Signature]*builder.MethodDefinition
	// pkgCache caches the extend packages, saving load time
	pkgCache map[string][]*packages.Package
	// workingDir is a working directory, can be empty
	workingDir string
}

// ParseExtendOptions holds extend method options.
type ParseExtendOptions struct {
	// PkgPath where the extend methods are located. If it is empty, the package is same as the
	// ConverterInterface package and ConverterScope should be used for the lookup.
	PkgPath string
	// Scope of the ConverterInterface.
	ConverterScope *types.Scope
	// ConverterInterface to use - can be nil if its use is not allowed.
	ConverterInterface types.Type
	// NamePattern is the regexp pattern to search for within the PkgPath above or
	// (if PkgPath is empty) within the Scope.
	NamePattern *regexp.Regexp
}

// parseExtendPackage parses the goverter:extend inputs with or without packages (local or external).
//
// extend statement can be one of the following:
// 1) local scope with a name: "ConvertAToB", it is also equivalent to ":ConvertAToB"
// 2) package with a name: "github.com/google/uuid:FromBytes"
// 3) either (1) or (2) with the above with a regexp pattern instead of a name
//
// To scan the whole package for candidate methods, use "package/path:.*".
// Note: if regexp pattern is used, only the methods matching the conversion signature can be used.
// Those are methods that have exactly one input (to convert from) and either one output (to covert to)
// or two outputs: type to convert to and an error object.
func (g *parseExtendContext) parseExtendPackage(opts *ParseExtendOptions, extend map[xtype.Signature]*builder.MethodDefinition) error {
	if opts.PkgPath == "" {
		// search in the converter's scope
		loaded, err := g.searchExtendsInScope(opts.ConverterScope, opts, extend)
		if err == nil && loaded == 0 {
			// no failure, but also nothing found (this can happen if pattern is used yet no matches found)
			err = fmt.Errorf("local package does not have methods with names that match "+
				"the golang regexp pattern %q and a convert signature", opts.NamePattern)
		}
		return err
	}

	return g.searchExtendsInPackages(opts, extend)
}

// searchExtendsInPackages searches for extend conversion methods that match an input regexp pattern
// within a given package path.
// Note: if this method finds no candidates, it will report an error. Two reasons for that:
// scanning packages takes time and it is very likely a human error.
func (g *parseExtendContext) searchExtendsInPackages(opts *ParseExtendOptions, extend map[xtype.Signature]*builder.MethodDefinition) error {
	// load a package by its path, loadPackages uses cache
	pkgs, err := g.loadPackages(opts.PkgPath)
	if err != nil {
		return err
	}

	var loaded int
	for _, pkg := range pkgs {
		// search in the scope of each package, first package is going to be the root one
		pkgLoaded, pkgErr := g.searchExtendsInScope(pkg.Types.Scope(), opts, extend)
		if pkgErr != nil {
			if err == nil {
				// remember the first err only - it is likely the most relevant if name is exact
				err = pkgErr
			}
		} else {
			loaded += pkgLoaded
		}
	}

	if loaded == 0 {
		if err == nil {
			return fmt.Errorf(`package %s does not have methods with names that match
the golang regexp pattern %q and a convert signature`, opts.PkgPath, opts.NamePattern.String())
		}
		return errors.Wrap(err, "could not extend")
	}

	return nil
}

// searchExtendsInScope searches the given package scope (either local or external) for
// the conversion method candidates. See parseExtendPackage for more details.
// If the input scope is not local, always pass converterInterface as a nil.
func (g *parseExtendContext) searchExtendsInScope(scope *types.Scope, opts *ParseExtendOptions, extend map[xtype.Signature]*builder.MethodDefinition) (int, error) {
	if prefix, complete := opts.NamePattern.LiteralPrefix(); complete {
		// this is not a regexp, use regular lookup and report error as is
		// we expect only one function to match
		return 1, g.parseExtendScopeMethod(scope, prefix, opts, extend)
	}

	// this is regexp, scan thru the package methods to find funcs that match the pattern
	var loaded int
	for _, name := range scope.Names() {
		loc := opts.NamePattern.FindStringIndex(name)
		if len(loc) != 2 {
			continue
		}
		if loc[0] != 0 || loc[1] != len(name) {
			// we want full match only: e.g. CopyAbc.* won't match OtherCopyAbc
			continue
		}

		// must be a func
		obj := scope.Lookup(name)
		fn, ok := obj.(*types.Func)
		if !ok {
			// obj == nil also won't type cast
			continue
		}

		err := g.parseExtendFunc(fn, opts, extend)
		if err == nil {
			loaded++
		}
	}
	return loaded, nil
}

// parseExtend prepares a list of extend methods for use.
func (g *parseExtendContext) parseExtend(converterInterface types.Type, converterScope *types.Scope, methods []string) (map[xtype.Signature]*builder.MethodDefinition, error) {
	extend := make(map[xtype.Signature]*builder.MethodDefinition)
	for _, methodName := range methods {
		parts := strings.SplitN(methodName, packageNameSep, 2)
		var pkgPath, namePattern string
		switch len(parts) {
		case 0:
			continue
		case 1:
			// name only, ignore empty inputs
			namePattern = parts[0]
			if namePattern == "" {
				continue
			}
		case 2:
			pkgPath = parts[0]
			if pkgPath == "" {
				// example: goverter:extend :MyLocalConvert
				// the purpose of the ':' in this case is confusing, do not allow such case
				return nil, fmt.Errorf(`package path must not be empty in the extend statement "%s".
See https://github.com/jmattheis/goverter#extend-with-custom-implementation`, methodName)
			}
			namePattern = parts[1]
			if namePattern == "" {
				return nil, fmt.Errorf(`method name pattern is required in the extend statement "%s".
See https://github.com/jmattheis/goverter#extend-with-custom-implementation`, methodName)
			}
		}

		pattern, err := regexp.Compile(namePattern)
		if err != nil {
			return nil, errors.Wrapf(err, "could not parse name as regexp %q", namePattern)
		}

		opts := &ParseExtendOptions{
			ConverterScope:     converterScope,
			PkgPath:            pkgPath,
			NamePattern:        pattern,
			ConverterInterface: converterInterface,
		}

		err = g.parseExtendPackage(opts, extend)
		if err != nil {
			return nil, err
		}
	}
	return extend, nil
}

// parseExtend prepares an extend conversion method using its name and a scope to search.
func (g *parseExtendContext) parseExtendScopeMethod(scope *types.Scope, methodName string, opts *ParseExtendOptions, extend map[xtype.Signature]*builder.MethodDefinition) error {
	obj := scope.Lookup(methodName)
	if obj == nil {
		return fmt.Errorf("%s does not exist in scope", methodName)
	}

	fn, ok := obj.(*types.Func)
	if !ok {
		return fmt.Errorf("%s is not a function", methodName)
	}

	return g.parseExtendFunc(fn, opts, extend)
}

// parseExtend prepares an extend conversion method using its func pointer.
func (g *parseExtendContext) parseExtendFunc(fn *types.Func, opts *ParseExtendOptions, extend map[xtype.Signature]*builder.MethodDefinition) error {
	if !fn.Exported() {
		return fmt.Errorf("method %s is unexported", fn.Name())
	}

	m, err := ParseMethod(fn, UseConverterInter(opts.ConverterInterface), UseExplicit(true), UseQual(fn.Pkg().Path()))
	if err != nil {
		return err
	}
	xsig := xtype.Signature{
		Source: m.Source.T.String(),
		Target: m.Target.T.String(),
		Kind:   m.Kind,
	}

	extend[xsig] = m
	return nil
}
