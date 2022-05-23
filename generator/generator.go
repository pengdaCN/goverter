package generator

import (
	"fmt"
	"github.com/dave/jennifer/jen"
	"github.com/jmattheis/goverter/builder"
	"github.com/jmattheis/goverter/comments"
	"github.com/jmattheis/goverter/namer"
	"github.com/jmattheis/goverter/xtype"
	"go/types"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/tools/go/packages"
	"sort"
	"strings"
)

type generator struct {
	namer  *namer.Namer
	name   string
	file   *jen.File
	lookup map[xtype.Signature]*builder.MethodDefinition
	extend map[xtype.Signature]*builder.MethodDefinition
	// pkgCache caches the extend packages, saving load time
	pkgCache map[string][]*packages.Package
	// workingDir is a working directory, can be empty
	workingDir string
}

func (g *generator) registerMethod(methodType *types.Func) error {
	signature, ok := methodType.Type().(*types.Signature)
	if !ok {
		return fmt.Errorf("expected signature %#v", methodType.Type())
	}
	params := signature.Params()
	if params.Len() != 1 {
		return fmt.Errorf("expected signature to have only one parameter")
	}
	result := signature.Results()
	if result.Len() < 1 || result.Len() > 2 {
		return fmt.Errorf("return has no or too many parameters")
	}
	source := params.At(0).Type()
	target := result.At(0).Type()
	returnError := false
	if result.Len() == 2 {
		if i, ok := result.At(1).Type().(*types.Named); ok && i.Obj().Name() == "error" && i.Obj().Pkg() == nil {
			returnError = true
		} else {
			return fmt.Errorf("second return parameter must have type error but had: %s", result.At(1).Type())
		}
	}

	m := &builder.MethodDefinition{
		Call:             jen.Id(xtype.ThisVar).Dot(methodType.Name()),
		ID:               methodType.String(),
		Explicit:         true,
		Name:             methodType.Name(),
		Source:           xtype.TypeOf(source),
		Target:           xtype.TypeOf(target),
		ReturnError:      returnError,
		ReturnTypeOrigin: methodType.FullName(),
	}

	g.lookup[xtype.Signature{
		Source: source.String(),
		Target: target.String(),
	}] = m
	g.namer.Register(m.Name)
	return nil
}

func (g *generator) createMethods(doc *comments.Converter) error {
	var methods []*builder.MethodDefinition
	for _, method := range g.lookup {
		methods = append(methods, method)
	}
	sort.Slice(methods, func(i, j int) bool {
		return methods[i].Name < methods[j].Name
	})
	for _, method := range methods {
		if method.Jen != nil && !method.Dirty {
			continue
		}
		method.Dirty = false

		ctx := doc.BuildCtx(method.Name)

		err := g.buildMethod(ctx.Enter(), method)
		if err != nil {
			err = err.Lift(&builder.Path{
				SourceID:   "source",
				TargetID:   "target",
				SourceType: method.Source.T.String(),
				TargetType: method.Target.T.String(),
			})
			return fmt.Errorf("Error while creating converter method:\n    %s\n\n%s", method.ID, builder.ToString(err))
		}
	}
	for _, method := range g.lookup {
		if method.Dirty {
			return g.createMethods(doc)
		}
	}
	g.appendToFile()
	return nil
}

func (g *generator) appendToFile() {
	var methods []*builder.MethodDefinition
	for _, method := range g.lookup {
		methods = append(methods, method)
	}
	sort.Slice(methods, func(i, j int) bool {
		return methods[i].Name < methods[j].Name
	})
	for _, method := range methods {
		g.file.Add(method.Jen)
	}
}

func (g *generator) buildMethod(ctx *builder.MethodContext, method *builder.MethodDefinition) *builder.Error {
	sourceID := jen.Id(xtype.In)
	source := method.Source

	target := method.Target

	var (
		returns = make([]jen.Code, 2)
	)

	if method.ReturnError {
		returns[1] = jen.Id("error")
	}

	ctx.TargetType = target
	ctx.Signature = xtype.Signature{Source: method.Source.T.String(), Target: method.Target.T.String()}

	stmt, newID, err := g.buildNoLookup(ctx, xtype.VariableID(sourceID.Clone()), source, target)
	if err != nil {
		return err
	}

	ret := []jen.Code{newID.Code}
	if method.ReturnError {
		ret = append(ret, jen.Nil())
	}

	stmt = append(stmt, jen.Return(ret...))

	var (
		params []jen.Code
	)
	switch {
	case method.ZeroCopyStruct:
		params = append(params, jen.Id(xtype.In).Add(jen.Op("*").Add(source.TypeAsJen())), jen.Id(xtype.Out).Add(jen.Op("*").Add(target.TypeAsJen())))
		returns[0] = jen.Op("*").Add(target.TypeAsJen())
	default:
		params = append(params, jen.Id(xtype.In).Add(source.TypeAsJen()))
		returns[0] = target.TypeAsJen()
	}

	method.Jen = jen.Func().
		Params(
			jen.Id(xtype.ThisVar).
				Op("*").
				Id(g.name),
		).
		Id(method.Name).
		Params(params...).
		Params(returns...).
		Block(stmt...)

	return nil
}

func (g *generator) buildNoLookup(ctx *builder.MethodContext, sourceID *xtype.JenID, source, target *xtype.Type) ([]jen.Code, *xtype.JenID, *builder.Error) {
	for _, rule := range BuildSteps {
		if rule.Matches(ctx, source, target) {
			return rule.Build(g, ctx, sourceID, source, target)
		}
	}
	return nil, nil, builder.NewError(fmt.Sprintf("TypeMismatch: Cannot convert %s to %s", source.T, target.T))
}

// Build builds an implementation for the given source and target type, or uses an existing method for it.
func (g *generator) Build(ctx *builder.MethodContext, sourceID *xtype.JenID, source, target *xtype.Type) ([]jen.Code, *xtype.JenID, *builder.Error) {
	var (
		_source   = source
		_target   = target
		_sourceID = sourceID
		_targetID *xtype.JenID
		method    *builder.MethodDefinition
		ok        bool
	)

	_sourceID, method, ok = g._lookupExtend(source, target, sourceID)
	if !ok {
		_source, _target, _sourceID, _targetID = OptimizeParams(ctx, source, target, sourceID, ctx.TargetID)
		method, ok = g._lookup(_source, _target)
	}

	if ok {
		var (
			params []jen.Code
		)
		if method.SelfAsFirstParam {
			params = append(params, jen.Id(xtype.ThisVar))
		}
		params = append(params, _sourceID.Code.Clone())

		switch {
		case method.ZeroCopyStruct:
			params = append(params, _targetID.Code.Clone())
		default:
		}

		if method.ReturnError {
			current := g.lookup[ctx.Signature]
			if !current.ReturnError {
				if current.Explicit {
					return nil, nil, builder.NewError(fmt.Sprintf("ReturnTypeMismatch: Cannot use\n\n    %s\n\nin\n\n    %s\n\nbecause no error is returned as second parameter", method.ReturnTypeOrigin, current.ID))
				}
				current.ReturnError = true
				current.ReturnTypeOrigin = method.ID
				current.Dirty = true
			}

			switch {
			case method.ZeroCopyStruct:
				stmt := []jen.Code{
					jen.List(jen.Id("err")).Op(":=").Add(
						method.Call.Clone().Call(params...),
					),
					jen.If(jen.Id("err").Op("!=").Nil()).Block(
						jen.Return(jen.Id("err")),
					),
				}

				return stmt, nil, nil
			default:
				name := ctx.Name(target.ID())
				innerName := ctx.Name("errValue")
				stmt := []jen.Code{
					jen.List(jen.Id(name), jen.Id("err")).Op(":=").Add(method.Call.Clone().Call(params...)),
					jen.If(jen.Id("err").Op("!=").Nil()).Block(
						jen.Var().Id(innerName).Add(ctx.TargetType.TypeAsJen()),
						jen.Return(jen.Id(innerName), jen.Id("err")),
					),
				}
				return stmt, xtype.VariableID(jen.Id(name)), nil
			}
		}

		stmt := method.Call.Clone().Call(params...)

		switch {
		case method.ZeroCopyStruct:
			return []jen.Code{stmt}, nil, nil
		default:
			return nil, xtype.OtherID(stmt), nil
		}
	}

	if (_source.Named && !_source.Basic) || (_target.Named && !_target.Basic) {
		var (
			name string
		)

		method := &builder.MethodDefinition{
			Source: xtype.TypeOf(_source.T),
			Target: xtype.TypeOf(_target.T),
		}

		if _source.Struct && _target.Struct && ctx.ZeroCopyStruct {
			method.ZeroCopyStruct = true
		}
		if !method.ZeroCopyStruct {
			name = g.namer.Name(source.UnescapedID() + "To" + cases.Title(language.English).String(target.UnescapedID()))
		} else {
			name = g.namer.Name(source.UnescapedID() + "Mapping" + cases.Title(language.English).String(target.UnescapedID()))
		}
		method.ID = name
		method.Name = name
		method.Call = jen.Id(xtype.ThisVar).Dot(name)

		if ctx.PointerChange {
			ctx.PointerChange = false
		}

		g.lookup[xtype.Signature{Source: _source.T.String(), Target: _target.T.String()}] = method
		g.namer.Register(method.Name)
		if err := g.buildMethod(ctx.Enter(), method); err != nil {
			return nil, nil, err
		}
		// try again to trigger the found method thingy above
		return g.Build(ctx, sourceID, source, target)
	}

	for _, rule := range BuildSteps {
		if rule.Matches(ctx, source, target) {
			return rule.Build(g, ctx, sourceID, source, target)
		}
	}

	return nil, nil, builder.NewError(fmt.Sprintf("TypeMismatch: Cannot convert %s to %s", source.T, target.T))
}

func (g *generator) Lookup(ctx *builder.MethodContext, source, target *xtype.Type) (*builder.MethodDefinition, bool) {
	_source, _target, _, _ := OptimizeParams(ctx, source, target, nil, nil)

	return g._lookup(_source, _target)
}

func (g *generator) _lookup(source, target *xtype.Type) (*builder.MethodDefinition, bool) {
	method, ok := g.lookup[xtype.Signature{Source: source.T.String(), Target: target.T.String()}]

	return method, ok
}

func (g *generator) _lookupExtend(source, target *xtype.Type, sourceID *xtype.JenID) (
	nextSourceID *xtype.JenID,
	method *builder.MethodDefinition,
	ok bool,
) {
	method, ok = g.extend[xtype.Signature{Source: source.T.String(), Target: target.T.String()}]
	if !ok {
		if source.Pointer {
			nextSourceID = xtype.OtherID(jen.Op("*").Add(sourceID.Code.Clone()))
			method, ok = g.extend[xtype.Signature{
				Source: strings.TrimLeft(source.T.String(), "*"),
				Target: target.T.String(),
			}]
		} else {
			nextSourceID = xtype.OtherID(jen.Op("&").Add(sourceID.Code.Clone()))
			method, ok = g.extend[xtype.Signature{
				Source: "*" + source.T.String(),
				Target: target.T.String(),
			}]
		}

		return
	}

	nextSourceID = sourceID

	return
}

func OptimizeParams(ctx *builder.MethodContext, source, target *xtype.Type, sourceID, targetID *xtype.JenID) (
	finalSource, finalTarget *xtype.Type,
	nextSourceID, nextTargetID *xtype.JenID,
) {
	switch {
	case ctx.ZeroCopyStruct:
		finalSource, nextSourceID = optimizeParam(source, sourceID)
		finalTarget, nextTargetID = optimizeParam(target, targetID)

	default:
		finalSource, nextSourceID = source, sourceID
		finalTarget, nextTargetID = target, targetID
	}

	return
}

func optimizeParam(param *xtype.Type, id *xtype.JenID) (
	finalParam *xtype.Type,
	nextID *xtype.JenID,
) {
	switch {
	case param.Struct:
		finalParam = param
		if id != nil {
			nextID = xtype.OtherID(jen.Op("&").Add(id.Code))
		}

	case param.Pointer && param.PointerInner.Struct:
		finalParam = param.PointerInner
		nextID = id

	default:
		finalParam = param
		nextID = id
	}

	return
}
