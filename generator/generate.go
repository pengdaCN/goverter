package generator

import (
	"fmt"
	"go/types"

	"github.com/dave/jennifer/jen"
	"github.com/pengdaCN/goverter/builder"
	"github.com/pengdaCN/goverter/comments"
	"github.com/pengdaCN/goverter/namer"
	"github.com/pengdaCN/goverter/xtype"
)

// Config the generate config.
type Config struct {
	Name          string
	PackagePath   string
	ExtendMethods []string
	WorkingDir    string
}

// BuildSteps that'll used for generation.
var BuildSteps = []builder.Builder{
	&builder.ZeroCopyStruct{},
	&builder.BasicTargetPointerRule{},
	&builder.Struct{},
	&builder.TargetStruct{},
	&builder.Pointer{},
	&builder.TargetPointer{},
	&builder.Basic{},
	&builder.List{},
	&builder.Map{},
}

// Generate generates a jen.File containing converters.
func Generate(pattern string, mapping []comments.Converter, config Config) (*jen.File, error) {
	file := jen.NewFilePathName(config.PackagePath, config.Name)
	file.HeaderComment("// Code generated by https://github.com/pengdaCN/goverter, DO NOT EDIT.")

	for _, converter := range mapping {
		obj := converter.Scope.Lookup(converter.Name)
		if obj == nil {
			return nil, fmt.Errorf("%s: could not find %s", pattern, converter.Name)
		}

		// create the converter struct
		file.Add(jen.Comment("nolint"))
		file.Type().Id(converter.Config.Name).Struct()

		gen := generator{
			namer:  namer.New(),
			file:   file,
			name:   converter.Config.Name,
			lookup: make(map[xtype.Signature]*builder.MethodDefinition),
		}
		interf := obj.Type().Underlying().(*types.Interface)

		extendMethods := make([]string, 0, len(config.ExtendMethods)+len(converter.Config.ExtendMethods))
		// Order is important: converter methods are keyed using their in and out type pairs; newly
		// discovered methods override existing ones. To enable fine-tuning per converter, extends
		// declared on the converter inteface should override extends provided globally.
		extendMethods = append(extendMethods, config.ExtendMethods...)
		extendMethods = append(extendMethods, converter.Config.ExtendMethods...)

		parseExtendCtx.workingDir = config.WorkingDir
		extend, err := parseExtendCtx.parseExtend(obj.Type(), converter.Scope, extendMethods)
		if err != nil {
			return nil, fmt.Errorf("Error while parsing extend in\n    %s\n\n%s", obj.Type().String(), err)
		}
		converter.RegGlobalExtend(extend)

		// we checked in comments, that it is an interface
		for i := 0; i < interf.NumMethods(); i++ {
			method := interf.Method(i)

			m, ok := converter.Methods[method.Name()]
			if !ok {
				return nil, fmt.Errorf("Error not found method:\n    %s", method.Name())
			}

			if len(m.ExtendMethods) != 0 {
				localExtend, err := parseExtendCtx.parseExtend(obj.Type(), converter.Scope, m.ExtendMethods)
				if err != nil {
					return nil, fmt.Errorf("Error while parsing extend in\n    %s\n\n%s", method.Name(), err)
				}

				converter.RegSpecificExtend(method.Name(), localExtend)
			}

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
