package generator

import (
	"fmt"
	"go/types"
	"sort"

	"github.com/dave/jennifer/jen"
	"github.com/pengdaCN/goverter/builder"
	"github.com/pengdaCN/goverter/comments"
	"github.com/pengdaCN/goverter/namer"
	"github.com/pengdaCN/goverter/xtype"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type generator struct {
	namer  *namer.Namer
	name   string
	file   *jen.File
	lookup map[xtype.Signature]*builder.MethodDefinition
}

func (g *generator) registerMethod(methodType *types.Func) error {
	m, err := ParseMethod(methodType)
	if err != nil {
		return err
	}
	m.Explicit = true

	g.lookup[xtype.Signature{
		Source: m.Source.T.String(),
		Target: m.Target.T.String(),
		Kind:   m.Kind,
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
	var (
		sourceID = jen.Id(xtype.In)
		targetID = jen.Id(xtype.Out)
		source   = method.Source
		target   = method.Target
	)

	returns := make([]jen.Code, 2)

	if method.ReturnError {
		switch method.Kind {
		case xtype.InSourceOutTarget:
			returns[1] = jen.Id("error")
		case xtype.InSourceIn2Target:
			returns[0] = jen.Id("err").Add(jen.Id("error"))
		}
	}

	ctx.TargetType = target
	if method.Kind == xtype.InSourceIn2Target {
		ctx.TargetID = xtype.VariableID(targetID.Clone())
	}
	ctx.Signature = xtype.Signature{Source: method.Source.T.String(), Target: method.Target.T.String(), Kind: method.Kind}
	ctx.WantMethodKind = ctx.Signature.Kind

	stmt, newID, err := g.buildNoLookup(ctx, xtype.VariableID(sourceID.Clone()), source, target)
	if err != nil {
		return err
	}

	var ret []jen.Code

	switch method.Kind {
	case xtype.InSourceOutTarget:
		ret = []jen.Code{newID.Code}
	case xtype.InSourceIn2Target:
	}

	if method.ReturnError {
		ret = append(ret, jen.Nil())
	}

	stmt = append(stmt, jen.Return(ret...))

	var params []jen.Code
	switch method.Kind {
	case xtype.InSourceOutTarget:
		params = append(params, jen.Id(xtype.In).Add(source.TypeAsJen()))
		returns[0] = target.TypeAsJen()
	case xtype.InSourceIn2Target:
		params = append(params, jen.Id(xtype.In).Add(source.TypeAsJen()), jen.Id(xtype.Out).Add(target.TypeAsJen()))
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
		if rule.Matches(source, target, ctx.WantMethodKind) {
			return rule.Build(g, ctx, sourceID, source, target)
		}
	}
	return nil, nil, builder.NewError(fmt.Sprintf("TypeMismatch: Cannot convert %s to %s", source.T, target.T))
}

// Build builds an implementation for the given source and target type, or uses an existing method for it.
func (g *generator) Build(ctx *builder.MethodContext, sourceID *xtype.JenID, source, target *xtype.Type) ([]jen.Code, *xtype.JenID, *builder.Error) {
	var (
		_sourceID = sourceID
		_targetID *xtype.JenID
		method    *builder.MethodDefinition
		ok        bool
	)

	_sourceID, _targetID, method, ok = _lookupExtend(ctx, source, target, sourceID)
	if !ok {
		_sourceID = sourceID
		_targetID = ctx.TargetID
		method, ok = g._lookup(source, target, ctx.WantMethodKind)
	}

	if ok {
		var params []jen.Code
		if method.SelfAsFirstParam {
			params = append(params, jen.Id(xtype.ThisVar))
		}
		params = append(params, _sourceID.Code.Clone())

		switch method.Kind {
		case xtype.InSourceIn2Target:
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

			switch method.Kind {
			case xtype.InSourceIn2Target:
				stmt := []jen.Code{
					jen.List(jen.Id("err")).Op(":=").Add(
						method.Call.Clone().Call(params...),
					),
					jen.If(jen.Id("err").Op("!=").Nil()).Block(
						jen.Return(jen.Id("err")),
					),
				}

				return stmt, nil, nil
			case xtype.InSourceOutTarget:
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

		switch method.Kind {
		case xtype.InSourceOutTarget:
			return nil, xtype.OtherID(stmt), nil
		case xtype.InSourceIn2Target:
			return []jen.Code{stmt}, nil, nil
		}
	}

	if (source.Named && !source.Basic) || (target.Named && !target.Basic) || (source.Pointer && target.Pointer && source.PointerInner.Struct && target.PointerInner.Struct) {
		var name string

		m := &builder.MethodDefinition{
			Source: xtype.TypeOf(source.T),
			Target: xtype.TypeOf(target.T),
		}

		if source.Pointer && target.Pointer && source.PointerInner.Struct && target.PointerInner.Struct {
			m.Kind = xtype.InSourceIn2Target
		}

		switch m.Kind {
		case xtype.InSourceOutTarget:
			name = g.namer.Name(source.UnescapedID() + "To" + cases.Title(language.English).String(target.UnescapedID()))
		case xtype.InSourceIn2Target:
			name = g.namer.Name(source.UnescapedID() + "Mapping" + cases.Title(language.English).String(target.UnescapedID()))
		}

		m.ID = name
		m.Name = name
		m.Call = jen.Id(xtype.ThisVar).Dot(name)

		g.lookup[xtype.Signature{Source: source.T.String(), Target: target.T.String(), Kind: m.Kind}] = m

		g.namer.Register(m.Name)
		if err := g.buildMethod(ctx.Enter(), m); err != nil {
			return nil, nil, err
		}
		// try again to trigger the found method thingy above
		return g.Build(ctx, sourceID, source, target)
	}

	for _, rule := range BuildSteps {
		if rule.Matches(source, target, ctx.WantMethodKind) {
			return rule.Build(g, ctx, sourceID, source, target)
		}
	}

	return nil, nil, builder.NewError(fmt.Sprintf("TypeMismatch: Cannot convert %s to %s", source.T, target.T))
}

func (g *generator) Name() string {
	return g.name
}

func (g *generator) _lookup(source, target *xtype.Type, kind xtype.MethodKind) (*builder.MethodDefinition, bool) {
	sign := xtype.Signature{
		Source: source.T.String(),
		Target: target.T.String(),
		Kind:   kind,
	}

	method, ok := g.lookup[sign]
	return method, ok
}

func _lookupExtend(ctx *builder.MethodContext, source, target *xtype.Type, sourceID *xtype.JenID) (
	nextSourceID *xtype.JenID,
	nextTargetID *xtype.JenID,
	method *builder.MethodDefinition,
	ok bool,
) {
	const (
		raw byte = iota + 1
		ref
		deref
	)

	var (
		// source 类型能取引用
		sourceRef = !source.Pointer && !source.List && !source.Map
		// target 类型能取引用
		targetRef = !target.Pointer && !target.List && !target.Map
		// source 类型能解引用
		sourceDeref = source.Pointer
		sourceVerb  = []byte{raw}
		targetVerb  = []byte{raw}
	)
	{
		if sourceRef {
			sourceVerb = append(sourceVerb, ref)
		}
		if targetRef {
			targetVerb = append(targetVerb, ref)
		}

		if sourceDeref {
			sourceVerb = append(sourceVerb, deref)
		}
	}
	for _, sVerb := range sourceVerb {
		var sourceTy string
		switch sVerb {
		case raw:
			sourceTy = source.T.String()
			nextSourceID = xtype.OtherID(sourceID.Code.Clone())
		case ref:
			sourceTy = "*" + source.T.String()
			nextSourceID = xtype.OtherID(jen.Op("&").Add(sourceID.Code.Clone()))
		case deref:
			sourceTy = source.PointerInner.T.String()
			nextSourceID = xtype.OtherID(jen.Op("*").Add(sourceID.Code.Clone()))
		}

		for _, tVerb := range targetVerb {
			var targetTy string
			switch tVerb {
			case raw:
				targetTy = target.T.String()

				if ctx.TargetID != nil {
					nextTargetID = xtype.OtherID(ctx.TargetID.Code.Clone())
				}
			case ref:
				if ctx.TargetID == nil {
					continue
				}

				targetTy = "*" + target.T.String()
				nextTargetID = xtype.OtherID(jen.Op("&").Add(ctx.TargetID.Code.Clone()))
			}

			sign := xtype.Signature{
				Source: sourceTy,
				Target: targetTy,
				Kind:   xtype.InSourceOutTarget,
			}

			for _, extends := range []map[xtype.Signature]*builder.MethodDefinition{
				ctx.MethodExtend,
				ctx.GlobalExtend,
			} {
				method, ok = extends[sign]
				if ok {
					return
				}

				var needSearchInSourceIn2Target bool
				// 判断是否需要InSourceIn2Target类型函数查询
				switch tVerb {
				case raw:
					needSearchInSourceIn2Target = target.Pointer
				case ref:
					needSearchInSourceIn2Target = true
				}

				if needSearchInSourceIn2Target {
					sign.Kind = xtype.InSourceIn2Target
					method, ok = extends[sign]
					if ok {
						return
					}
				}
			}
		}
	}

	return
}
