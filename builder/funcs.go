package builder

import "github.com/jmattheis/goverter/xtype"

func optimizeZeroCopy(source *xtype.Type, target *xtype.Type) (
	nextSource *xtype.Type,
	nextTarget *xtype.Type,
	zeroCopy bool,
) {

	for origin, next := range map[*xtype.Type]**xtype.Type{
		source: &nextSource,
		target: &nextTarget,
	} {
		switch {
		case origin.Pointer && origin.PointerInner.Struct:
			*next = source
		case origin.Struct:
			*next = xtype.WrapWithPtr(source)
		default:
			return
		}
	}

	zeroCopy = true

	return
}
