package builder

import (
	"github.com/dave/jennifer/jen"
	"github.com/pengdaCN/goverter/xtype"
)

type MethodDefinition struct {
	ID       string
	Explicit bool
	Name     string
	Call     *jen.Statement
	Source   *xtype.Type
	Target   *xtype.Type
	Kind     xtype.MethodKind

	Jen jen.Code

	SelfAsFirstParam bool
	ReturnError      bool
	ReturnTypeOrigin string
	Dirty            bool
}
