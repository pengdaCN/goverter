package generator

import (
	"fmt"
	"github.com/dave/jennifer/jen"
	"github.com/jmattheis/goverter/builder"
	"github.com/jmattheis/goverter/xtype"
	"go/types"
)

type ParseOption struct {
	ConverterInterface types.Type
}

type ParseOpt func(opt *ParseOption)

func ParseMethod(method *types.Func, opts ...ParseOpt) (*builder.MethodDefinition, error) {
	var opt ParseOption
	for _, parseOpt := range opts {
		parseOpt(&opt)
	}

	signature, ok := method.Type().(*types.Signature)
	if !ok {
		return nil, fmt.Errorf("expected signature %#v", method.Type())
	}
	params := tupleToVars(signature.Params())
	result := tupleToVars(signature.Results())

	var (
		source               types.Type
		target               types.Type
		advTarget            *xtype.Type
		maybeErr             types.Type
		kind                 xtype.MethodKind
		selfAsFirstParameter bool
		returnError          bool
	)

	// 处理第一个参数可能是ConverterInterface的情况
	if opt.ConverterInterface != nil {
		if len(params) >= 2 {
			if params[0].Type().String() == opt.ConverterInterface.String() {
				selfAsFirstParameter = true
				params = params[1:]
			}
		}
	}

	switch {
	case len(params) == 1 && len(result) <= 2:
		kind = xtype.InSourceOutTarget
		source = params[0].Type()
		target = result[0].Type()
		if len(result) == 2 {
			maybeErr = result[1].Type()
		}
	case len(params) == 2 && len(result) <= 1:
		kind = xtype.InSourceIn2Target
		source = params[0].Type()
		target = params[1].Type()
		if len(result) == 1 {
			maybeErr = result[0].Type()
		}
	default:
		return nil, fmt.Errorf("invalid function singature format")
	}

	// 判读最后一个返回参数是否时error
	if maybeErr != nil {
		if i, ok := maybeErr.(*types.Named); ok && i.Obj().Name() == "error" && i.Obj().Pkg() == nil {
			returnError = true
		} else {
			return nil, fmt.Errorf("the fast return parameter must have type error but had: %s", maybeErr.String())
		}
	}

	advTarget = xtype.TypeOf(target)
	if kind == xtype.InSourceIn2Target {
		if !advTarget.Pointer {
			return nil, fmt.Errorf("the second parameter must be pointer type but had: %s", target.String())
		}
	}

	return &builder.MethodDefinition{
		Call:             jen.Id(xtype.ThisVar).Dot(method.Name()),
		ID:               method.String(),
		Name:             method.Name(),
		SelfAsFirstParam: selfAsFirstParameter,
		Kind:             kind,
		Source:           xtype.TypeOf(source),
		Target:           advTarget,
		ReturnError:      returnError,
		ReturnTypeOrigin: method.FullName(),
	}, nil
}

func tupleToVars(in *types.Tuple) []*types.Var {
	if in.Len() == 0 {
		return nil
	}

	r := make([]*types.Var, in.Len())
	for i := 0; i < in.Len(); i++ {
		r[i] = in.At(i)
	}

	return r
}
