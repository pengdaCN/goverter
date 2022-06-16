package builder

import (
	"github.com/dave/jennifer/jen"
	"github.com/jmattheis/goverter/xtype"
)

type MethodKind byte

const (
	InSourceOutTarget MethodKind = iota + 1
	InSourceIn2Target
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

	Kind MethodKind

	Jen jen.Code

	SelfAsFirstParam bool
	ReturnError      bool
	ReturnTypeOrigin string
	Dirty            bool
}
