package xtype

import (
	"go/types"
	"reflect"
)

var (
	ptrElemOffset uintptr
)

func init() {
	ptrTy := reflect.TypeOf((*types.Pointer)(nil)).Elem()
	for i := 0; i < ptrTy.NumField(); i++ {
		fld := ptrTy.Field(i)
		switch fld.Name {
		case "base":
			ptrElemOffset = fld.Offset
			break
		}
	}
}
