package builder

import (
	"github.com/dave/jennifer/jen"
	"github.com/pengdaCN/goverter/xtype"
)

// List handles array / slice types.
type List struct{}

// Matches returns true, if the builder can create handle the given types.
func (*List) Matches(source, target *xtype.Type, kind xtype.MethodKind) bool {
	return source.List && target.List && !target.ListFixed && kind == xtype.InSourceOutTarget
}

// Build creates conversion source code for the given source and target type.
func (*List) Build(gen Generator, ctx *MethodContext, sourceID *xtype.JenID, source, target *xtype.Type) ([]jen.Code, *xtype.JenID, *Error) {
	var (
		targetSlice                             = ctx.Name(target.ID())
		index                                   = ctx.Index()
		nextSource, nextTarget, enabledZeroCopy = optimizeZeroCopy(source.ListInner, target.ListInner)
		nextSourceID                            = xtype.VariableID(sourceID.Code.Clone().Index(jen.Id(index)))
	)
	ctx.TargetID = xtype.OtherID(jen.Id(targetSlice).Index(jen.Id(index)))
	if enabledZeroCopy {
		ctx.WantMethodKind = xtype.InSourceIn2Target

		if source.ListInner.Struct {
			nextSourceID = xtype.OtherID(jen.Op("&").Add(nextSourceID.Code.Clone()))
		}

		if target.ListInner.Struct {
			ctx.TargetID = xtype.OtherID(jen.Op("&").Add(ctx.TargetID.Code.Clone()))
		}
	} else {
		nextSource = source.ListInner
		nextTarget = target.ListInner
	}

	newStmt, newID, err := gen.Build(ctx, nextSourceID, nextSource, nextTarget)
	if err != nil {
		return nil, nil, err.Lift(&Path{
			SourceID:   "[]",
			SourceType: source.ListInner.T.String(),
			TargetID:   "[]",
			TargetType: target.ListInner.T.String(),
		})
	}

	if enabledZeroCopy {
		if target.ListInner.Pointer {
			_newStmt := make([]jen.Code, len(newStmt)+1)
			_newStmt[0] = jen.Id(targetSlice).Index(jen.Id(index)).Op("=").Add(jen.New(target.ListInner.PointerInner.TypeAsJen()))
			copy(_newStmt[1:], newStmt)

			newStmt = _newStmt
		}
	} else {
		newStmt = append(newStmt, jen.Id(targetSlice).Index(jen.Id(index)).Op("=").Add(newID.Code))
	}

	stmt := []jen.Code{
		jen.Id(targetSlice).Op(":=").Make(target.TypeAsJen(), jen.Len(sourceID.Code.Clone())),
		jen.For(jen.Id(index).Op(":=").Lit(0), jen.Id(index).Op("<").Len(sourceID.Code.Clone()), jen.Id(index).Op("++")).
			Block(newStmt...),
	}

	return stmt, xtype.VariableID(jen.Id(targetSlice)), nil
}
