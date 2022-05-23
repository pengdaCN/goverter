package builder

import (
	"github.com/dave/jennifer/jen"
	"github.com/jmattheis/goverter/xtype"
)

// List handles array / slice types.
type List struct{}

// Matches returns true, if the builder can create handle the given types.
func (*List) Matches(_ *MethodContext, source, target *xtype.Type) bool {
	return source.List && target.List && !target.ListFixed
}

// Build creates conversion source code for the given source and target type.
func (*List) Build(gen Generator, ctx *MethodContext, sourceID *xtype.JenID, source, target *xtype.Type) ([]jen.Code, *xtype.JenID, *Error) {
	var (
		targetSlice = ctx.Name(target.ID())
		index       = ctx.Index()
	)

	switch {
	case ctx.ZeroCopyStruct:
		ctx.TargetID = xtype.OtherID(jen.Id(targetSlice).Index(jen.Id(index)))
	}

	indexedSource := xtype.VariableID(sourceID.Code.Clone().Index(jen.Id(index)))

	newStmt, newID, err := gen.Build(ctx, indexedSource, source.ListInner, target.ListInner)
	if err != nil {
		return nil, nil, err.Lift(&Path{
			SourceID:   "[]",
			SourceType: source.ListInner.T.String(),
			TargetID:   "[]",
			TargetType: target.ListInner.T.String(),
		})
	}

	mdef, ok := gen.Lookup(ctx, source.ListInner, target.ListInner)
	switch {
	case ok && mdef.ZeroCopyStruct:
		if target.ListInner.Pointer {
			_newStmt := make([]jen.Code, len(newStmt)+1)
			_newStmt[0] = jen.Id(targetSlice).Index(jen.Id(index)).Op("=").Add(jen.New(target.ListInner.PointerInner.TypeAsJen()))
			copy(_newStmt[1:], newStmt)

			newStmt = _newStmt
		}
	default:
		newStmt = append(newStmt, jen.Id(targetSlice).Index(jen.Id(index)).Op("=").Add(newID.Code))
	}

	stmt := []jen.Code{
		jen.Id(targetSlice).Op(":=").Make(target.TypeAsJen(), jen.Len(sourceID.Code.Clone())),
		jen.For(jen.Id(index).Op(":=").Lit(0), jen.Id(index).Op("<").Len(sourceID.Code.Clone()), jen.Id(index).Op("++")).
			Block(newStmt...),
	}

	return stmt, xtype.VariableID(jen.Id(targetSlice)), nil
}
