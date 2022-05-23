package builder

import (
	"github.com/dave/jennifer/jen"
	"github.com/jmattheis/goverter/xtype"
)

// Pointer handles pointer types.
type Pointer struct{}

// Matches returns true, if the builder can create handle the given types.
func (*Pointer) Matches(_ *MethodContext, source, target *xtype.Type) bool {
	return source.Pointer && target.Pointer
}

// Build creates conversion source code for the given source and target type.
func (*Pointer) Build(gen Generator, ctx *MethodContext, sourceID *xtype.JenID, source, target *xtype.Type) ([]jen.Code, *xtype.JenID, *Error) {
	ctx.PointerChange = true

	var (
		outerVar     = ctx.Name(target.ID())
		innerVar     = ctx.Name(target.PointerInner.ID())
		nextSourceId *xtype.JenID
	)

	switch {
	case ctx.ZeroCopyStruct:
		ctx.TargetID = xtype.OtherID(jen.Op("&").Add(jen.Id(innerVar)))
		nextSourceId = xtype.OtherID(sourceID.Code.Clone())
	default:
		nextSourceId = xtype.OtherID(jen.Op("*").Add(sourceID.Code.Clone()))
	}

	nextBlock, id, err := gen.Build(ctx, nextSourceId, source.PointerInner, target.PointerInner)
	if err != nil {
		return nil, nil, err.Lift(&Path{
			SourceID:   "*",
			SourceType: source.PointerInner.T.String(),
			TargetID:   "*",
			TargetType: target.PointerInner.T.String(),
		})
	}

	mdef, ok := gen.Lookup(source.PointerInner, target.PointerInner)
	if !ok {
		return nil, nil, NewError("not found MethodDefinition").Lift(&Path{
			SourceID:   "*",
			SourceType: source.PointerInner.T.String(),
			TargetID:   "*",
			TargetType: target.PointerInner.T.String(),
		})
	}

	var (
		ifBlock []jen.Code
	)

	switch {
	case mdef.ZeroCopyStruct:
		ifBlock = append(ifBlock, jen.Var().Id(innerVar).Add(target.PointerInner.TypeAsJen()))
	default:
	}

	ifBlock = append(ifBlock, nextBlock...)

	switch {
	case mdef.ZeroCopyStruct:
		ifBlock = append(ifBlock, jen.Id(outerVar).Op("=").Op("&").Add(jen.Id(innerVar)))
	default:
		if id.Variable {
			ifBlock = append(ifBlock, jen.Id(outerVar).Op("=").Op("&").Add(id.Code.Clone()))
		} else {
			tempName := ctx.Name(target.PointerInner.ID())
			ifBlock = append(ifBlock, jen.Id(tempName).Op(":=").Add(id.Code.Clone()))
			ifBlock = append(ifBlock, jen.Id(outerVar).Op("=").Op("&").Id(tempName))
		}
	}

	stmt := []jen.Code{
		jen.Var().Id(outerVar).Add(target.TypeAsJen()),
		jen.If(sourceID.Code.Clone().Op("!=").Nil()).Block(ifBlock...),
	}

	return stmt, xtype.VariableID(jen.Id(outerVar)), err
}

// TargetPointer handles type were only the target is a pointer.
type TargetPointer struct{}

// Matches returns true, if the builder can create handle the given types.
func (*TargetPointer) Matches(_ *MethodContext, source, target *xtype.Type) bool {
	return !source.Pointer && target.Pointer
}

// Build creates conversion source code for the given source and target type.
func (*TargetPointer) Build(gen Generator, ctx *MethodContext, sourceID *xtype.JenID, source, target *xtype.Type) ([]jen.Code, *xtype.JenID, *Error) {
	ctx.PointerChange = true

	var (
		nextSourceId *xtype.JenID
		innerVar     = ctx.Name(target.PointerInner.ID())
	)
	switch {
	case ctx.ZeroCopyStruct:
		// TODO 移除该处理
		nextSourceId = xtype.OtherID(jen.Op("&").Add(sourceID.Code))

		ctx.TargetID = xtype.OtherID(jen.Op("&").Add(jen.Id(innerVar)))
	default:
		nextSourceId = sourceID
	}

	stmt, id, err := gen.Build(ctx, nextSourceId, source, target.PointerInner)
	if err != nil {
		return nil, nil, err.Lift(&Path{
			SourceID:   "*",
			SourceType: source.T.String(),
			TargetID:   "*",
			TargetType: target.PointerInner.T.String(),
		})
	}

	mdef, ok := gen.Lookup(source, target.PointerInner)
	if !ok {
		return nil, nil, NewError("not found MethodDefinition").Lift(&Path{
			SourceID:   "*",
			SourceType: source.PointerInner.T.String(),
			TargetID:   "*",
			TargetType: target.PointerInner.T.String(),
		})
	}

	switch {
	case mdef.ZeroCopyStruct:
		_stmt := make([]jen.Code, len(stmt)+1)
		_stmt[0] = jen.Var().Id(innerVar).Add(target.PointerInner.TypeAsJen())
		copy(_stmt[1:], stmt)

		stmt = _stmt
	default:
		if id.Variable {
			return stmt, xtype.OtherID(jen.Op("&").Add(id.Code)), nil
		}
		stmt = append(stmt, jen.Id(innerVar).Op(":=").Add(id.Code))
	}

	return stmt, xtype.OtherID(jen.Op("&").Id(innerVar)), nil
}
