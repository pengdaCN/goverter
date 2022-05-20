package generator

import (
	"fmt"
	"go/types"

	"github.com/dave/jennifer/jen"
	"github.com/jmattheis/goverter/builder"
	"github.com/jmattheis/goverter/comments"
	"github.com/jmattheis/goverter/namer"
	"github.com/jmattheis/goverter/xtype"
)

// Config the generate config.
type Config struct {
	Name          string
	ExtendMethods []string
	WorkingDir    string
}

// BuildSteps that'll used for generation.
var BuildSteps = []builder.Builder{
	&builder.BasicTargetPointerRule{},
	&builder.Struct{},
	&builder.Pointer{},
	&builder.TargetPointer{},
	&builder.Basic{},
	&builder.List{},
	&builder.Map{},
}

// Generate generates a jen.File containing converters.
func Generate(pattern string, mapping []comments.Converter, config Config) (*jen.File, error) {
	file := jen.NewFile(config.Name)
	file.HeaderComment("// Code generated by github.com/jmattheis/goverter, DO NOT EDIT.")

	for _, converter := range mapping {
		obj := converter.Scope.Lookup(converter.Name)
		if obj == nil {
			return nil, fmt.Errorf("%s: could not find %s", pattern, converter.Name)
		}

		// create the converter struct
		file.Type().Id(converter.Config.Name).Struct()

		gen := generator{
			namer:      namer.New(),
			file:       file,
			name:       converter.Config.Name,
			lookup:     map[xtype.Signature]*builder.MethodDefinition{},
			extend:     map[xtype.Signature]*builder.MethodDefinition{},
			workingDir: config.WorkingDir,
		}
		interf := obj.Type().Underlying().(*types.Interface)

		extendMethods := make([]string, 0, len(config.ExtendMethods)+len(converter.Config.ExtendMethods))
		// Order is important: converter methods are keyed using their in and out type pairs; newly
		// discovered methods override existing ones. To enable fine-tuning per converter, extends
		// declared on the converter inteface should override extends provided globally.
		extendMethods = append(extendMethods, config.ExtendMethods...)
		extendMethods = append(extendMethods, converter.Config.ExtendMethods...)

		if err := gen.parseExtend(obj.Type(), converter.Scope, extendMethods); err != nil {
			return nil, fmt.Errorf("Error while parsing extend in\n    %s\n\n%s", obj.Type().String(), err)
		}

		// we checked in comments, that it is an interface
		for i := 0; i < interf.NumMethods(); i++ {
			method := interf.Method(i)
			//var converterMethod comments.Method
			//
			//if m, ok := converter.Methods[method.Name()]; ok {
			//	converterMethod = m
			//}
			if err := gen.registerMethod(method); err != nil {
				return nil, fmt.Errorf("Error while creating converter method:\n    %s\n\n%s", method.String(), err)
			}
		}
		if err := gen.createMethods(&converter); err != nil {
			return nil, err
		}
	}
	return file, nil
}
