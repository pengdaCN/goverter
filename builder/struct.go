package builder

import (
	"fmt"
	"go/types"
	"log"
	"strings"

	"github.com/samber/lo"

	"github.com/dave/jennifer/jen"
	"github.com/pengdaCN/goverter/xtype"
)

// Struct handles struct types.
type Struct struct{}

// Matches returns true, if the builder can create handle the given types.
func (p *Struct) Matches(source, target *xtype.Type, _ xtype.MethodKind) bool {
	return source.Struct && target.Struct
}

// Build creates conversion source code for the given source and target type.
func (*Struct) Build(gen Generator, ctx *MethodContext, sourceID *xtype.JenID, source, target *xtype.Type) ([]jen.Code, *xtype.JenID, *Error) {
	var (
		name = ctx.Name(target.ID())
		stmt = []jen.Code{
			jen.Var().Id(name).Add(target.TypeAsJen()),
		}
	)

	ctx.TargetID = xtype.OtherID(jen.Op("&").Add(jen.Id(name)))
	ctx.WantMethodKind = xtype.InSourceIn2Target

	alloc, _, err := gen.Build(
		ctx,
		xtype.OtherID(jen.Op("&").Add(sourceID.Code.Clone())),
		xtype.WrapWithPtr(source),
		xtype.WrapWithPtr(target),
	)
	if err != nil {
		return nil, nil, err
	}

	stmt = append(stmt, alloc...)

	return stmt, xtype.VariableID(jen.Id(name)), nil
}

type ZeroCopyStruct struct{}

func (z *ZeroCopyStruct) Matches(source, target *xtype.Type, kind xtype.MethodKind) bool {
	return source.Pointer && target.Pointer && source.PointerInner.Struct && target.PointerInner.Struct && kind == xtype.InSourceIn2Target
}

func (z *ZeroCopyStruct) Build(gen Generator, ctx *MethodContext, sourceID *xtype.JenID, source, target *xtype.Type) ([]jen.Code, *xtype.JenID, *Error) {
	var (
		name = xtype.Out
		stmt = []jen.Code{
			jen.If(
				jen.Id(xtype.In).Op("==").Nil().
					Op("||").
					Id(xtype.Out).Op("==").Nil(),
			).
				Block(
					jen.Return(),
				),
		}

		innerSource = source.PointerInner
		innerTarget = target.PointerInner
	)

	for i := 0; i < innerTarget.StructType.NumFields(); i++ {
		targetField := innerTarget.StructType.Field(i)
		targetFieldTag := innerTarget.StructType.Tag(i)
		targetFieldType := xtype.TypeOf(targetField.Type())
		targetFieldRef := jen.Id(name).Dot(targetField.Name())
		nextTarget := targetFieldType
		nextSourceID := sourceID
		nextSource := source

		if _, ignore := ctx.IgnoredFields[targetField.Name()]; ignore {
			continue
		}
		if !targetField.Exported() {
			if ctx.NoStrict {
				if !ctx.IgnoreUnexported {
					log.Printf("(%s.%s) warn: Cannot set value for unexported field: %s\n", gen.Name(), ctx.ID, strings.Join([]string{target.T.String(), targetField.Name()}, "."))
				}
				continue
			}

			cause := unexportedStructError(targetField.Name(), source.T.String(), target.T.String())
			return nil, nil, NewError(cause).Lift(&Path{
				Prefix:     ".",
				SourceID:   "???",
				TargetID:   targetField.Name(),
				TargetType: targetField.Type().String(),
			})
		}

		// 对于targetField是匿名嵌入类型，自动进行IdentityMapping操作
		if _, ok := ctx.IdentityMapping[targetField.Name()]; ok || targetField.Embedded() {
			goto assign
		}

		{
			var (
				mapStmt []jen.Code
				// lift    []*Path
				nextID *jen.Statement
				err    *Error
			)

			findCtx := ctx.EnterWithNamer()
			findCtx.Signature.Source = innerSource.T.String()
			findCtx.Signature.Target = innerTarget.T.String()

			nextID, nextSource, mapStmt, _, err = mapField(findCtx, targetField, targetFieldTag, sourceID, innerSource, innerTarget)
			if err != nil {
				if ctx.NoStrict {
					log.Printf("(%s.%s)warn: Cannot match the target field with the source entry %s\n", gen.Name(), ctx.ID, strings.Join([]string{target.T.String(), targetField.Name()}, "."))
					continue
				}

				return nil, nil, err
			}
			nextSourceID = xtype.VariableID(nextID)
			stmt = append(stmt, mapStmt...)
		}

	assign:
		var (
			fieldStmt       []jen.Code
			fieldID         *xtype.JenID
			err             *Error
			ok              bool
			sourceIsPtr     bool
			_nextSource     *xtype.Type
			_nextTarget     *xtype.Type
			enabledZeroCopy bool
		)
		// 开始尝试extend
		ok, fieldStmt, fieldID, err = gen.BuildWithExtend(ctx, nextSourceID, nextSource, nextTarget)
		if ok {
			if err != nil {
				return nil, nil, err
			}

			if nextSource.Pointer {
				sourceIsPtr = true
			}

			goto assignStmt
		}

		_nextSource, _nextTarget, enabledZeroCopy = optimizeZeroCopy(nextSource, nextTarget)
		if enabledZeroCopy {
			ctx.WantMethodKind = xtype.InSourceIn2Target
			ctx.TargetID = xtype.OtherID(targetFieldRef.Clone())

			if nextSource.Pointer {
				sourceIsPtr = true
			} else {
				nextSourceID = xtype.OtherID(jen.Op("&").Add(nextSourceID.Code.Clone()))
			}

			if nextTarget.Pointer {
				stmt = append(stmt, targetFieldRef.Clone().Op("=").Add(jen.New(_nextTarget.PointerInner.TypeAsJen())))
			} else {
				ctx.TargetID = xtype.OtherID(jen.Op("&").Add(ctx.TargetID.Code.Clone()))
			}
		} else {
			ctx.WantMethodKind = xtype.InSourceOutTarget
			_nextSource = nextSource
			_nextTarget = nextTarget
		}

		fieldStmt, fieldID, err = gen.Build(ctx, nextSourceID, _nextSource, _nextTarget)
		if err != nil {
			return nil, nil, err.Lift(&Path{
				Prefix:     ".",
				SourceID:   "???",
				SourceType: nextSource.T.String(),
				TargetID:   targetField.Name(),
				TargetType: targetField.Type().String(),
			})
		}
		ctx.WantMethodKind = xtype.InSourceIn2Target

	assignStmt:
		if sourceIsPtr {
			if fieldID != nil {
				fieldStmt = append(fieldStmt, targetFieldRef.Clone().Op("=").Add(fieldID.Code))
			}

			stmt = append(stmt, jen.
				If(
					nextSourceID.Code.Clone().Op("!=").Nil(),
				).
				Block(
					fieldStmt...,
				),
			)
		} else {
			stmt = append(stmt, fieldStmt...)
			if fieldID != nil {
				stmt = append(stmt, targetFieldRef.Clone().Op("=").Add(fieldID.Code))
			}
		}
	}

	return stmt, nil, nil
}

type TargetStruct struct{}

func (t *TargetStruct) Matches(source, target *xtype.Type, kind xtype.MethodKind) bool {
	return source.Pointer && source.PointerInner.Struct && target.Struct && kind == xtype.InSourceOutTarget
}

func (t *TargetStruct) Build(gen Generator, ctx *MethodContext, sourceID *xtype.JenID, source, target *xtype.Type) ([]jen.Code, *xtype.JenID, *Error) {
	var (
		name = ctx.Name(target.ID())
		stmt = []jen.Code{
			jen.Var().Id(name).Add(target.TypeAsJen()),
		}
	)

	ctx.TargetID = xtype.OtherID(jen.Op("&").Add(jen.Id(name)))
	ctx.WantMethodKind = xtype.InSourceIn2Target

	alloc, _, err := gen.Build(
		ctx,
		sourceID,
		source,
		xtype.WrapWithPtr(target),
	)
	if err != nil {
		return nil, nil, err
	}

	stmt = append(stmt, alloc...)

	return stmt, xtype.VariableID(jen.Id(name)), nil
}

func mapField(
	ctx *MethodContext,
	targetField *types.Var,
	targetFiledTag string,
	sourceID *xtype.JenID,
	source, target *xtype.Type,
) (
	*jen.Statement,
	*xtype.Type,
	[]jen.Code,
	[]*Path, *Error,
) {
	var lift []*Path

	mappedName, hasOverride := searchRefPathWithMapping(source, ctx, targetField.Name(), targetFiledTag, ctx.SearchTag)
	if ctx.Signature.Target != target.T.String() || !hasOverride {
		sourceMatch, err := source.StructField(targetField.Name(), targetFiledTag, ctx.MatchIgnoreCase, ctx.IgnoredFields, ctx.SearchTag)
		if err == nil {
			nextID := sourceID.Code.Clone().Dot(sourceMatch.Name)
			lift = append(lift, &Path{
				Prefix:     ".",
				SourceID:   sourceMatch.Name,
				SourceType: sourceMatch.Type.T.String(),
				TargetID:   targetField.Name(),
				TargetType: targetField.Type().String(),
			})
			return nextID, sourceMatch.Type, []jen.Code{}, lift, nil
		}
		// field lookup either did not find anything or failed due to ambiquous match with case ignored
		cause := fmt.Sprintf("Cannot match the target field with the source entry: %s.", err.Error())
		return nil, nil, nil, nil, NewError(cause).Lift(&Path{
			Prefix:     ".",
			SourceID:   "???",
			TargetID:   targetField.Name(),
			TargetType: targetField.Type().String(),
		})
	}

	path := strings.Split(mappedName, ".")
	var condition *jen.Statement

	var stmt []jen.Code
	nextID := sourceID.Code
	nextSource := source
	for i := 0; i < len(path); i++ {
		if nextSource.Pointer {
			addCondition := nextID.Clone().Op("!=").Nil()
			if condition == nil {
				condition = addCondition
			} else {
				condition = condition.Clone().Op("&&").Add(addCondition)
			}
			nextSource = nextSource.PointerInner
		}
		if !nextSource.Struct {
			cause := fmt.Sprintf("Cannot access '%s' on %s.", path[i], nextSource.T)
			return nil, nil, nil, nil, NewError(cause).Lift(&Path{
				Prefix:     ".",
				SourceID:   path[i],
				SourceType: "???",
			}).Lift(lift...)
		}
		// since we are searching for a mapped name, search for exact match, explicit field map does not ignore case
		sourceMatch, err := nextSource.StructField(path[i], "", false, ctx.IgnoredFields, nil)
		if err == nil {
			nextSource = sourceMatch.Type
			nextID = nextID.Clone().Dot(sourceMatch.Name)
			liftPath := &Path{
				Prefix:     ".",
				SourceID:   sourceMatch.Name,
				SourceType: nextSource.T.String(),
			}

			if i == len(path)-1 {
				liftPath.TargetID = targetField.Name()
				liftPath.TargetType = targetField.Type().String()
				if condition != nil && !nextSource.Pointer {
					liftPath.SourceType = fmt.Sprintf("*%s (It is a pointer because the nested property in the goverter:map was a pointer)", liftPath.SourceType)
				}
			}
			lift = append(lift, liftPath)
			continue
		}

		cause := fmt.Sprintf("Cannot find the mapped field on the source entry: %s.", err.Error())
		return nil, nil, []jen.Code{}, nil, NewError(cause).Lift(&Path{
			Prefix:     ".",
			SourceID:   path[i],
			SourceType: "???",
		}).Lift(lift...)
	}

	if condition != nil {
		var (
			isCopyable = true
			wrapType   = nextSource
		)

		{
			next := nextSource
			for next.Named {
				next = xtype.TypeOf(next.NamedType.Underlying())
			}
			if next.Struct {
				wrapType = xtype.TypeOf(types.NewPointer(next.T))
				isCopyable = false
			}
		}

		tempName := ctx.Name(wrapType.ID())
		stmt = append(stmt, jen.Var().Id(tempName).Add(wrapType.TypeAsJen()))
		if isCopyable {
			stmt = append(stmt, jen.If(condition).Block(
				jen.Id(tempName).Op("=").Add(nextID.Clone()),
			))
		} else {
			stmt = append(stmt, jen.If(condition).Block(
				jen.Id(tempName).Op("=").Op("&").Add(nextID.Clone()),
			))
		}
		nextSource = wrapType
		nextID = jen.Id(tempName)
	}

	return nextID, nextSource, stmt, lift, nil
}

func searchRefPathWithMapping(source *xtype.Type, ctx *MethodContext, field string, tag string, tags []string) (string, bool) {
	const (
		maxFindTimes = 1000
	)

	mapping, ok := ctx.Mapping[field]
	if ok {
		return mapping, ok
	}

	searchTypes := []lo.Tuple2[string, *xtype.Type]{
		{
			A: "",
			B: source,
		},
	}
	var path strings.Builder
	for searchStep := 0; len(searchTypes) > searchStep && searchStep < maxFindTimes; searchStep++ {
		prefix := searchTypes[searchStep].A
		nextSource := searchTypes[searchStep].B
		sourceMatch, err := nextSource.StructField(field, tag, ctx.MatchIgnoreCase, ctx.IgnoredFields, tags)
		if err == nil {
			if prefix != "" {
				path.WriteString(prefix)
				path.WriteString(".")
			}

			path.WriteString(sourceMatch.Name)

			return path.String(), true
		}

		embedFields := nextSource.EmbedField()
		if len(embedFields) != 0 {
			searchTypes = append(searchTypes, lo.Map(embedFields, func(v lo.Tuple2[string, *xtype.Type], _ int) lo.Tuple2[string, *xtype.Type] {
				for v.B.Pointer {
					v.B = v.B.PointerInner
				}

				if prefix == "" {
					return v
				}

				v.A = strings.Join([]string{
					prefix,
					v.A,
				}, ".")

				return v
			})...)
		}
	}

	return "", false
}

func unexportedStructError(targetField, sourceType, targetType string) string {
	return fmt.Sprintf(`Cannot set value for unexported field "%s".

Possible solutions:

* Ignore the given field with:

      // goverter:ignore %s

* Convert the struct yourself and use goverter for converting nested structs / maps / lists.

* Create a custom converter function (only works, if the struct with unexported fields is nested inside another struct)

      func CustomConvert(source %s) %s {
          // implement me
      }

      // goverter:extend CustomConvert
      type MyConverter interface {
          // ...
      }

See https://github.com/jmattheis/goverter#extend-with-custom-implementation`, targetField, targetField, sourceType, targetType)
}
