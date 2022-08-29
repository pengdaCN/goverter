package generator

import "golang.org/x/tools/go/packages"

var parseExtendCtx = parseExtendContext{
	pkgCache: make(map[string][]*packages.Package),
}
