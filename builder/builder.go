package builder

import (
	"github.com/dave/jennifer/jen"
	"github.com/jmattheis/goverter/namer"
	"github.com/jmattheis/goverter/xtype"
)

// Builder builds converter implementations, and can decide if it can handle the given type.
type Builder interface {
	// Matches returns true, if the builder can create handle the given types
	Matches(source, target *xtype.Type, kind xtype.MethodKind) bool
	// Build creates conversion source code for the given source and target type.
	Build(gen Generator, ctx *MethodContext, sourceID *xtype.JenID, source, target *xtype.Type) ([]jen.Code, *xtype.JenID, *Error)
}

// Generator checks all existing builders if they can create a conversion implementations for the given source and target type
// If no one Builder#Matches then, an error is returned.
type Generator interface {
	Build(ctx *MethodContext, sourceID *xtype.JenID, source, target *xtype.Type) ([]jen.Code, *xtype.JenID, *Error)
	// TODO delete
	Lookup(ctx *MethodContext, source, target *xtype.Type) (*MethodDefinition, bool)
	Name() string
}

// MethodContext exposes information for the current method.
type MethodContext struct {
	*namer.Namer
	Mapping         map[string]string
	IgnoredFields   map[string]struct{}
	IdentityMapping map[string]struct{}
	GlobalExtend    map[xtype.Signature]*MethodDefinition
	MethodExtend    map[xtype.Signature]*MethodDefinition
	Signature       xtype.Signature
	TargetType      *xtype.Type
	PointerChange   bool
	MatchIgnoreCase bool
	NoStrict        bool
	ZeroCopyStruct  bool
	TargetID        *xtype.JenID
	ID              string
}

func (m *MethodContext) Enter() *MethodContext {
	return &MethodContext{
		Namer:           namer.New(),
		Mapping:         m.Mapping,
		IgnoredFields:   m.IgnoredFields,
		IdentityMapping: m.IdentityMapping,
		GlobalExtend:    m.GlobalExtend,
		MethodExtend:    m.MethodExtend,
		MatchIgnoreCase: m.MatchIgnoreCase,
		PointerChange:   m.PointerChange,
		NoStrict:        m.NoStrict,
		ZeroCopyStruct:  m.ZeroCopyStruct,
		ID:              m.ID,
	}
}
