package builder

import (
	"github.com/dave/jennifer/jen"
	"github.com/jmattheis/goverter/xtype"
)

// Pointer handles pointer types.
type Pointer struct{}

// Matches returns true, if the builder can create handle the given types.
func (*Pointer) Matches(source, target *xtype.Type, kind xtype.MethodKind) bool {
	return source.Pointer && target.Pointer && kind == xtype.InSourceOutTarget
}

// Build creates conversion source code for the given source and target type.
func (*Pointer) Build(gen Generator, ctx *MethodContext, sourceID *xtype.JenID, source, target *xtype.Type) ([]jen.Code, *xtype.JenID, *Error) {
	var (
		outerVar        = ctx.Name(target.ID())
		innerVar        = ctx.Name(target.PointerInner.ID())
		nextSourceID    *xtype.JenID
		nextSource      = source
		nextTarget      = target
		enabledZeroCopy = source.PointerInner.Struct && target.PointerInner.Struct
	)

	if enabledZeroCopy {
		nextSourceID = xtype.OtherID(sourceID.Code.Clone())
		ctx.TargetID = xtype.OtherID(jen.Op("&").Add(jen.Id(innerVar)))
		ctx.WantMethodKind = xtype.InSourceIn2Target
	} else {
		nextSourceID = xtype.OtherID(jen.Op("*").Add(sourceID.Code.Clone()))
		nextSource = source.PointerInner
		nextTarget = target.PointerInner
	}

	nextBlock, id, err := gen.Build(ctx, nextSourceID, nextSource, nextTarget)
	if err != nil {
		return nil, nil, err.Lift(&Path{
			SourceID:   "*",
			SourceType: source.PointerInner.T.String(),
			TargetID:   "*",
			TargetType: target.PointerInner.T.String(),
		})
	}

	var (
		ifBlock []jen.Code
	)

	if enabledZeroCopy {
		ifBlock = append(ifBlock, jen.Var().Id(innerVar).Add(target.PointerInner.TypeAsJen()))
	}

	ifBlock = append(ifBlock, nextBlock...)

	if enabledZeroCopy {
		ifBlock = append(ifBlock, jen.Id(outerVar).Op("=").Op("&").Add(jen.Id(innerVar)))
	} else {
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
func (*TargetPointer) Matches(source, target *xtype.Type, kind xtype.MethodKind) bool {
	return !source.Pointer && target.Pointer && kind == xtype.InSourceOutTarget
}

// Build creates conversion source code for the given source and target type.
func (*TargetPointer) Build(gen Generator, ctx *MethodContext, sourceID *xtype.JenID, source, target *xtype.Type) ([]jen.Code, *xtype.JenID, *Error) {
	var (
		innerVar        = ctx.Name(target.PointerInner.ID())
		nextSource      = source
		nextTarget      = target
		nextSourceID    = sourceID
		enabledZeroCopy = source.PointerInner.Struct && target.PointerInner.Struct
	)

	ctx.TargetID = xtype.OtherID(jen.Id(innerVar))
	if enabledZeroCopy {
		nextSource = xtype.WrapWithPtr(source)
		nextSourceID = xtype.OtherID(jen.Op("&").Add(sourceID.Code.Clone()))
		ctx.WantMethodKind = xtype.InSourceIn2Target
	}

	stmt, id, err := gen.Build(ctx, nextSourceID, nextSource, nextTarget)
	if err != nil {
		return nil, nil, err.Lift(&Path{
			SourceID:   "*",
			SourceType: source.T.String(),
			TargetID:   "*",
			TargetType: target.PointerInner.T.String(),
		})
	}

	if enabledZeroCopy {
		_stmt := make([]jen.Code, len(stmt)+1)
		_stmt[0] = jen.Var().Id(innerVar).Add(target.PointerInner.TypeAsJen())
		copy(_stmt[1:], stmt)

		stmt = _stmt
	} else {
		if id.Variable {
			return stmt, xtype.OtherID(jen.Op("&").Add(id.Code)), nil
		}
		stmt = append(stmt, jen.Id(innerVar).Op(":=").Add(id.Code))
	}

	return stmt, xtype.OtherID(jen.Op("&").Id(innerVar)), nil
}
