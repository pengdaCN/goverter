package builder

import (
	"github.com/dave/jennifer/jen"
	"github.com/jmattheis/goverter/xtype"
)

type MethodDefinition struct {
	ID       string
	Explicit bool
	Name     string
	Call     *jen.Statement
	Source   *xtype.Type
	Target   *xtype.Type
	// TODO delete
	ZeroCopyStruct bool
	Kind           xtype.MethodKind

	Jen jen.Code

	SelfAsFirstParam bool
	ReturnError      bool
	ReturnTypeOrigin string
	Dirty            bool
}
