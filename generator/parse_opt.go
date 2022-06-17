package generator

import "go/types"

func UseConverterInter(inter types.Type) ParseOpt {
	return func(opt *ParseOption) {
		opt.ConverterInterface = inter
	}
}

func UseQual(q string) ParseOpt {
	return func(opt *ParseOption) {
		opt.Qual = q
	}
}

func UseExplicit(e bool) ParseOpt {
	return func(opt *ParseOption) {
		opt.Explicit = e
	}
}
