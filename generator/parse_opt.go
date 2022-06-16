package generator

import "go/types"

func UseConverterInter(inter types.Type) ParseOpt {
	return func(opt *ParseOption) {
		opt.ConverterInterface = inter
	}
}
