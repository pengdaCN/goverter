package builder

import "github.com/pengdaCN/goverter/xtype"

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
			*next = origin
		case origin.Struct:
			*next = xtype.WrapWithPtr(origin)
		default:
			return
		}
	}

	zeroCopy = true

	return
}
