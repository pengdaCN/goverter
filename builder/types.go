package builder

import (
	"github.com/dave/jennifer/jen"
	"github.com/jmattheis/goverter/xtype"
)

type MethodDefinition struct {
	ID             string
	Explicit       bool
	Name           string
	Call           *jen.Statement
	Source         *xtype.Type
	Target         *xtype.Type
	ZeroCopyStruct bool

	Jen jen.Code

	SelfAsFirstParam bool
	ReturnError      bool
	ReturnTypeOrigin string
	Dirty            bool
}
