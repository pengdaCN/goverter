package builder

import (
	"github.com/dave/jennifer/jen"
	"github.com/jmattheis/goverter/xtype"
)

type PtrStruct struct{}

func (p *PtrStruct) Matches(ctx *MethodContext, source, target *xtype.Type) bool {
	return ctx.ZeroCopyStruct && source.Pointer && target.Pointer && source.PointerInner.Struct && target.PointerInner.Struct
}

func (p *PtrStruct) Build(gen Generator, ctx *MethodContext, sourceID *xtype.JenID, source, target *xtype.Type) ([]jen.Code, *xtype.JenID, *Error) {
	//TODO implement me
	panic("implement me")
}
