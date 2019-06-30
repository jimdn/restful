package restful

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/globalsign/mgo/bson"
)

const (
	KindInvalid     = uint(reflect.Invalid)
	KindBool        = uint(reflect.Bool)
	KindInt         = uint(reflect.Int64)
	KindUint        = uint(reflect.Uint64)
	KindFloat       = uint(reflect.Float64)
	KindString      = uint(reflect.String)
	KindObject      = uint(reflect.Struct)
	KindSimpleEnd   = uint(999)
	KindArrayBase   = uint(1000)
	KindArrayBool   = KindArrayBase + KindBool
	KindArrayInt    = KindArrayBase + KindInt
	KindArrayUint   = KindArrayBase + KindUint
	KindArrayFloat  = KindArrayBase + KindFloat
	KindArrayString = KindArrayBase + KindString
	KindArrayObject = KindArrayBase + KindObject
	KindArrayEnd    = uint(1999)
	KindMapBase     = uint(2000)
	KindMapBool     = KindMapBase + KindBool
	KindMapInt      = KindMapBase + KindInt
	KindMapUint     = KindMapBase + KindUint
	KindMapFloat    = KindMapBase + KindFloat
	KindMapString   = KindMapBase + KindString
	KindMapObject   = KindMapBase + KindObject
	KindMapEnd      = uint(2999)
)

type Field struct {
	Kind       uint // field's kind
	CreateOnly bool // field can only be written when creating by POST or PUT
	ReadOnly   bool // field can not be written or update, data should be loaded into DB by other ways
}

type FieldSet struct {
	FMap map[string]Field // fields map
	FSli []string         // fields ordered
}

func BuildFieldSet(typ reflect.Type) *FieldSet {
	p := &FieldSet{
		FMap: make(map[string]Field),
		FSli: make([]string, 0),
	}
	p.FMap[""] = Field{Kind: KindObject}
	build(typ, make([]string, 0, 0), p)
	return p
}

func build(typ reflect.Type, prefix []string, p *FieldSet) {
	t := typ
	if typ.Kind() == reflect.Ptr {
		t = typ.Elem()
	}
	path := strings.Join(prefix, ".")
	kind := parseKind(t)
	if path != "" && kind != KindInvalid {
		p.FMap[path] = Field{Kind: kind}
		p.FSli = append(p.FSli, path)
	}
	switch kind {
	case KindObject, KindArrayObject, KindMapObject:
		if t.Kind() == reflect.Array || t.Kind() == reflect.Slice || t.Kind() == reflect.Map {
			t = t.Elem()
		}
		for t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct {
			return
		}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			tag := strings.Split(f.Tag.Get("json"), ",")[0]
			prefix = append(prefix, tag)
			build(f.Type, prefix, p)
			prefix = prefix[:len(prefix)-1]
		}
	}
}

func parseKind(typ reflect.Type) uint {
	t := typ
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	kind := t.Kind()
	if kind == reflect.Array || kind == reflect.Slice {
		elemKind := parseKind(t.Elem())
		if elemKind < KindBool || elemKind > KindObject {
			return KindInvalid
		}
		return KindArrayBase + elemKind
	}
	if kind == reflect.Map {
		elemKind := parseKind(t.Elem())
		if elemKind < KindBool || elemKind > KindObject {
			return KindInvalid
		}
		return KindMapBase + elemKind
	}

	if kind == reflect.Bool {
		return KindBool
	}
	if kind >= reflect.Int && kind <= reflect.Int64 {
		return KindInt
	}
	if kind >= reflect.Uint && kind <= reflect.Uint64 {
		return KindUint
	}
	if kind == reflect.Float32 || kind == reflect.Float64 {
		return KindFloat
	}
	if kind == reflect.String {
		return KindString
	}
	if kind == reflect.Struct {
		return KindObject
	}
	return KindInvalid
}

func (fs *FieldSet) CheckObject(obj map[string]interface{}, dotOk bool) error {
	invalidFields := make(map[string]interface{})
	prefix := make([]string, 0, 0)
	fs.check(obj, prefix, dotOk, invalidFields)
	if len(invalidFields) != 0 {
		return fmt.Errorf("invalid fields %v", invalidFields)
	}
	return nil
}

func (fs *FieldSet) check(obj map[string]interface{}, prefix []string, dotOk bool, invalidFields map[string]interface{}) {
	for k, value := range obj {
		if strings.Index(k, ".") >= 0 {
			if !dotOk {
				invalidFields[k] = "dot not allow"
				delete(obj, k)
				continue
			}

			// dot field in object is invalid
			if len(prefix) > 0 {
				invalidFields[k] = "dot invalid"
				delete(obj, k)
				continue
			}

			// check map
			kind, ok := fs.IsMapMember(k)
			if !ok {
				invalidFields[k] = "unknown"
				delete(obj, k)
				continue
			}
			v := ParseKindValue(value, kind-KindMapBase)
			if v == nil {
				invalidFields[k] = "type mismatch"
				delete(obj, k)
				continue
			}
			continue
		}

		path := append(prefix, k)
		full := strings.Join(path, ".")
		kind := KindInvalid
		ok := false
		if dotOk {
			// PATCH method, update
			kind, ok = fs.IsFieldMember(full)
			if !ok {
				invalidFields[full] = "unknown"
				delete(obj, full)
				continue
			}
			if fs.IsFieldReadOnly(full) {
				invalidFields[full] = "read only"
				delete(obj, full)
				continue
			}
			if fs.IsFieldCreateOnly(full) {
				invalidFields[full] = "create only"
				delete(obj, full)
				continue
			}
		} else {
			// POST or PUT method, create
			kind, ok = fs.IsFieldMember(full)
			if !ok {
				invalidFields[full] = "unknown"
				delete(obj, full)
				continue
			}
			if fs.IsFieldReadOnly(full) {
				invalidFields[full] = "read only"
				delete(obj, full)
				continue
			}
		}
		v := ParseKindValue(value, kind)
		if v == nil {
			invalidFields[full] = "type mismatch"
			delete(obj, full)
			continue
		}
		switch kind {
		case KindObject:
			fs.check(v.(map[string]interface{}), path, dotOk, invalidFields)
		case KindArrayObject:
			for _, elem := range v.([]interface{}) {
				fs.check(elem.(map[string]interface{}), path, dotOk, invalidFields)
			}
		}
	}
}

func (fs *FieldSet) IsFieldMember(field string) (uint, bool) {
	if _, ok := fs.FMap[field]; !ok {
		return fs.IsMapMember(field)
	}
	kind := fs.FMap[field].Kind
	return kind, true
}

func (fs *FieldSet) IsMapMember(field string) (uint, bool) {
	pos := strings.LastIndex(field, ".")
	if pos == -1 {
		return KindInvalid, false
	}
	if _, ok := fs.FMap[field[:pos]]; ok {
		kind := fs.FMap[field[:pos]].Kind
		if kind > KindMapBase && kind < KindMapEnd {
			return kind, true
		}
	}
	return KindInvalid, false
}

func (fs *FieldSet) IsFieldCreateOnly(field string) bool {
	return fs.FMap[field].CreateOnly
}

func (fs *FieldSet) IsFieldReadOnly(field string) bool {
	return fs.FMap[field].ReadOnly
}

func (fs *FieldSet) SetCreateOnlyFields(fields []string) {
	fields = RemoveDupArray(fields)
	for _, field := range fields {
		for k, f := range fs.FMap {
			if strings.HasPrefix(k, field) {
				f.CreateOnly = true
				fs.FMap[k] = f
			}
		}
	}
}

func (fs *FieldSet) SetReadOnlyFields(fields []string) {
	fields = RemoveDupArray(fields)
	for _, field := range fields {
		for k, f := range fs.FMap {
			if strings.HasPrefix(k, field) {
				f.ReadOnly = true
				fs.FMap[k] = f
			}
		}
	}
}

func (fs *FieldSet) InReplace(value *map[string]interface{}) {
	// id --> _id
	if v, ok := (*value)["id"]; ok {
		(*value)["_id"] = v
		delete(*value, "id")
	}
}

func (fs *FieldSet) OutReplace(value *map[string]interface{}) {
	// _id --> id
	if v, ok := (*value)["_id"]; ok {
		(*value)["id"] = v
		delete(*value, "_id")
	}
}

func (fs *FieldSet) OutReplaceArray(values []interface{}) {
	for _, value := range values {
		switch v := value.(type) {
		case map[string]interface{}:
			fs.OutReplace(&v)
		case bson.M:
			fs.OutReplace((*map[string]interface{})(&v))
		default:
			continue
		}
	}
}

func (fs *FieldSet) InSort(data *map[string]interface{}) bson.D {
	d := make([]bson.DocElem, 0)
	for _, value := range (*fs).FSli {
		// replace id
		if value == "id" {
			value = "_id"
		}
		if strings.Index(value, ".") >= 0 {
			continue
		}
		if _, ok := (*data)[value]; ok {
			d = append(d, bson.DocElem{Name: value, Value: (*data)[value]})
		}
	}
	return d
}

func (fs *FieldSet) BuildFilterObj(filter map[string]interface{}, cond map[string]interface{}) error {
	for k, value := range filter {
		if _, exist := cond[k]; exist {
			return fmt.Errorf("filter field %s condition conflict", k)
		}
		kind, ok := fs.IsFieldMember(k)
		if !ok {
			return fmt.Errorf("filter field %s unknown", k)
		}
		// nil or empty
		if kind != KindInvalid && value == nil || IsEmpty(value, kind) {
			empty := EmptyValue(kind)
			if empty == nil {
				cond[k] = nil
			} else {
				cond[k] = bson.M{"$in": []interface{}{nil, empty}}
			}
			continue
		}
		// simple kind
		if kind >= KindBool && kind < KindSimpleEnd {
			v := fs.ParseSimpleValue(value, kind)
			if v != nil {
				cond[k] = v
			} else {
				return fmt.Errorf("filter field %s type mismatch", k)
			}
		}
		// array
		if kind > KindArrayBase && kind < KindArrayEnd {
			// filter's element is array
			switch value.(type) {
			case []interface{}:
				// field is an array and value is an array too
				cond[k] = value
				continue
			default:
				return fmt.Errorf("filter field %s type mismatch", k)
			}
		}
		// map
		if kind > KindMapBase && kind < KindMapEnd {
			v := ParseKindValue(value, kind-KindMapBase)
			if v != nil {
				cond[k] = v
			} else {
				return fmt.Errorf("filter field %s type mismatch", k)
			}
		}
	}
	return nil
}

func (fs *FieldSet) BuildRangeObj(rang map[string]interface{}, cond map[string]interface{}) error {
	for k, value := range rang {
		if _, exist := cond[k]; exist {
			return fmt.Errorf("range field %s condition conflict", k)
		}
		hasGTorGTE := false
		hasLTorLTE := false
		switch mv := value.(type) {
		case map[string]interface{}:
			obj := make(map[string]interface{})
			kind, ok := fs.IsFieldMember(k)
			if !ok {
				return fmt.Errorf("range field %s unknown", k)
			}
			if kind >= KindMapBool && kind <= KindMapString {
				kind = kind - KindMapBase
			}
			if kind >= KindBool && kind <= KindString {
				// gt or gte
				if gt, ok := mv["gt"]; ok {
					v := fs.ParseSimpleValue(gt, kind)
					if v == nil {
						return fmt.Errorf("range field %s type mismatch", k)
					}
					obj["$gt"] = v
					hasGTorGTE = true
				}
				if gte, ok := mv["gte"]; ok {
					if hasGTorGTE {
						return fmt.Errorf("range field %s gt or gte conflict", k)
					}
					v := fs.ParseSimpleValue(gte, kind)
					if v == nil {
						return fmt.Errorf("range field %s type mismatch", k)
					}
					obj["$gte"] = v
				}
				// lt or lte
				if lt, ok := mv["lt"]; ok {
					v := fs.ParseSimpleValue(lt, kind)
					if v == nil {
						return fmt.Errorf("range field %s type mismatch", k)
					}
					obj["$lt"] = v
					hasLTorLTE = true
				}
				if lte, ok := mv["lte"]; ok {
					if hasLTorLTE {
						return fmt.Errorf("range field %s lt or lte conflict", k)
					}
					v := fs.ParseSimpleValue(lte, kind)
					if v == nil {
						return fmt.Errorf("range field %s type mismatch", k)
					}
					obj["$lte"] = v
				}
				if len(obj) > 0 {
					cond[k] = obj
				} else {
					return fmt.Errorf("range field %s invalid", k)
				}
				continue
			}
			return fmt.Errorf("range field %s type not support", k)
		default:
			return fmt.Errorf("range field %s not map", k)
		}
	}
	return nil
}

func (fs *FieldSet) BuildInObj(in map[string]interface{}, cond map[string]interface{}) error {
	for k, value := range in {
		if _, exist := cond[k]; exist {
			return fmt.Errorf("in field %s condition conflict", k)
		}
		kind, ok := fs.IsFieldMember(k)
		if !ok {
			return fmt.Errorf("in field %s unknown", k)
		}
		if kind >= KindArrayBool && kind <= KindArrayString {
			kind = kind - KindArrayBase
		}
		if kind >= KindMapBool && kind <= KindMapString {
			kind = kind - KindMapBase
		}
		if kind >= KindBool && kind <= KindString {
			v := fs.ParseSimpleArray(value, kind)
			if v != nil {
				cond[k] = map[string]interface{}{"$in": v}
			} else {
				return fmt.Errorf("in field %s should be array or elem type mismatch", k)
			}
			continue
		}
		return fmt.Errorf("in field %s type not support", k)
	}
	return nil
}

func (fs *FieldSet) BuildNinObj(nin map[string]interface{}, cond map[string]interface{}) error {
	for k, value := range nin {
		if _, exist := cond[k]; exist {
			return fmt.Errorf("nin field %s condition conflict", k)
		}
		kind, ok := fs.IsFieldMember(k)
		if !ok {
			return fmt.Errorf("nin field %s unknown", k)
		}
		if kind >= KindArrayBool && kind <= KindArrayString {
			kind = kind - KindArrayBase
		}
		if kind >= KindMapBool && kind <= KindMapString {
			kind = kind - KindMapBase
		}
		if kind >= KindBool && kind <= KindString {
			v := fs.ParseSimpleArray(value, kind)
			if v != nil {
				cond[k] = map[string]interface{}{"$nin": v}
			} else {
				return fmt.Errorf("nin field %s should be array or elem type mismatch", k)
			}
			continue
		}
		return fmt.Errorf("nin field %s type not support", k)
	}
	return nil
}

func (fs *FieldSet) BuildAllObj(all map[string]interface{}, cond map[string]interface{}) error {
	for k, value := range all {
		if _, exist := cond[k]; exist {
			return fmt.Errorf("all field %s condition conflict", k)
		}
		kind, ok := fs.IsFieldMember(k)
		if !ok {
			return fmt.Errorf("all field %s unknown", k)
		}
		if kind >= KindArrayBool && kind <= KindArrayString {
			kind = kind - KindArrayBase
		}
		if kind >= KindMapBool && kind <= KindMapString {
			kind = kind - KindMapBase
		}
		if kind >= KindBool && kind <= KindString {
			v := fs.ParseSimpleArray(value, kind)
			if v != nil {
				cond[k] = map[string]interface{}{"$all": v}
			} else {
				return fmt.Errorf("all field %s should be array or elem type mismatch", k)
			}
			continue
		}
		return fmt.Errorf("all field %s type not support", k)
	}
	return nil
}

func (fs *FieldSet) BuildOrderArray(order []string, sort *bson.D) error {
	for _, value := range order {
		if len(value) <= 1 {
			return fmt.Errorf("order field %s invalid", value)
		}
		r, k := value[0], value[1:]
		v := int64(0)
		if r == '+' {
			v = 1
		} else if r == '-' {
			v = -1
		} else {
			return fmt.Errorf("order field %s should start with +/- ", value)
		}
		if _, ok := fs.IsFieldMember(k); !ok {
			return fmt.Errorf("order field %s unknown", value)
		}
		*sort = append(*sort, bson.DocElem{Name: k, Value: v})
	}
	return nil
}

func (fs *FieldSet) OrderArray2Slice(sort *bson.D) []string {
	r := make([]string, 0, 0)
	for _, elem := range *sort {
		k := elem.Name
		value := CheckInt(elem.Value)
		if value == nil {
			continue
		}
		dir := "+"
		v := value.(int64)
		if v < 0 {
			dir = "-"
		}
		// id --> _id
		if k == "id" {
			k = "_id"
		}
		r = append(r, dir+k)
	}
	return r
}

func (fs *FieldSet) BuildSelectObj(slice []string, sel map[string]interface{}) error {
	for _, value := range slice {
		if len(value) == 0 {
			return fmt.Errorf("select field invalid")
		}
		if _, ok := fs.IsFieldMember(value); !ok {
			return fmt.Errorf("select field %s unknown", value)
		}
		sel[value] = 1
	}
	return nil
}

func (fs *FieldSet) CheckSearchFields(fields []string) error {
	fields = RemoveDupArray(fields)
	for _, field := range fields {
		if len(field) <= 0 {
			return fmt.Errorf("search field %s invalid", field)
		}
		kind, ok := fs.IsFieldMember(field)
		if !ok {
			return fmt.Errorf("search field %s unknown", field)
		}
		if kind != KindString && kind != KindArrayString {
			return fmt.Errorf("search field %s not string", field)
		}
	}
	return nil
}

func (fs *FieldSet) BuildSearchContent(obj map[string]interface{}, fields []string) string {
	array := make([]string, 0)
	for _, field := range fields {
		if field == "id" {
			field = "_id"
		}
		path := strings.Split(field, ".")
		depth := len(path)
		o := obj
		i := 0
		for i = 0; i < (depth - 1); i++ {
			t := o[path[i]]
			if t == nil {
				break
			}
			o = t.(map[string]interface{})
		}
		if i == (depth - 1) {
			switch v := o[path[depth-1]].(type) {
			case string:
				array = append(array, v)
			case []interface{}:
				for _, elem := range v {
					vv := CheckString(elem)
					if vv != nil {
						array = append(array, vv.(string))
					}
				}
			}
		}
	}
	return strings.Join(array, " ")
}

func (fs *FieldSet) ParseSimpleValue(value interface{}, kind uint) interface{} {
	if value == nil {
		return nil
	}
	switch kind {
	case KindBool:
		return CheckBool(value)
	case KindInt:
		return CheckInt(value)
	case KindUint:
		return CheckUint(value)
	case KindFloat:
		return CheckFloat(value)
	case KindString:
		return CheckString(value)
	case KindObject:
		return CheckObject(value)
	}
	return nil
}

func (fs *FieldSet) ParseSimpleArray(value interface{}, kind uint) interface{} {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case []interface{}:
		r := make([]interface{}, 0, 0)
		for _, elem := range v {
			if elem == nil {
				r = append(r, nil)
				continue
			}
			simpleValue := fs.ParseSimpleValue(elem, kind)
			if simpleValue != nil {
				r = append(r, simpleValue)
			}
		}
		if len(r) > 0 {
			return r
		}
	}
	return nil
}

func ParseKindValue(value interface{}, kind uint) interface{} {
	switch kind {
	case KindBool:
		return CheckBool(value)
	case KindInt:
		return CheckInt(value)
	case KindUint:
		return CheckUint(value)
	case KindFloat:
		return CheckFloat(value)
	case KindString:
		return CheckString(value)
	case KindObject:
		return CheckObject(value)
	case KindArrayBool:
		fallthrough
	case KindArrayInt:
		fallthrough
	case KindArrayUint:
		fallthrough
	case KindArrayFloat:
		fallthrough
	case KindArrayString:
		fallthrough
	case KindArrayObject:
		switch v := value.(type) {
		case []interface{}:
			return ParseKindArray(v, kind)
		}
	case KindMapBool:
		fallthrough
	case KindMapInt:
		fallthrough
	case KindMapUint:
		fallthrough
	case KindMapFloat:
		fallthrough
	case KindMapString:
		fallthrough
	case KindMapObject:
		switch v := value.(type) {
		case map[string]interface{}:
			return ParseKindMap(v, kind)
		}
	}
	return nil
}

func ParseKindArray(value []interface{}, kind uint) interface{} {
	r := make([]interface{}, 0, len(value))
	switch kind {
	case KindBool:
		fallthrough
	case KindArrayInt:
		fallthrough
	case KindArrayUint:
		fallthrough
	case KindArrayFloat:
		fallthrough
	case KindArrayString:
		fallthrough
	case KindArrayObject:
		for _, elem := range value {
			v := ParseKindValue(elem, kind-KindArrayBase)
			if v == nil {
				return nil
			}
			r = append(r, v)
		}
	}
	return r
}

func ParseKindMap(value map[string]interface{}, kind uint) interface{} {
	for _, v := range value {
		if ParseKindValue(v, kind-KindMapBase) == nil {
			return nil
		}
	}
	return value
}

func IsEmpty(d interface{}, kind uint) bool {
	k := kind
	switch {
	case k == KindBool:
		return IsEmptyBool(d)
	case KindInt <= k && k < KindFloat:
		return IsEmptyNumber(d)
	case k == KindString:
		return IsEmptyString(d)
	case k == KindObject:
		return IsEmptyObject(d)
	case KindArrayBase < k && k < KindArrayEnd:
		return IsEmptyArray(d)
	case KindMapBase < k && k < KindMapEnd:
		return IsEmptyObject(d)
	}
	return false
}

func EmptyValue(kind uint) interface{} {
	switch kind {
	case KindBool:
		return false
	case KindInt, KindUint, KindFloat:
		return 0
	case KindString:
		return ""
	case KindObject:
		return make(map[string]interface{})
	}
	if kind > KindArrayBase && kind < KindArrayEnd {
		return make([]interface{}, 0, 0)
	}
	if kind > KindMapBase && kind < KindMapEnd {
		return make(map[string]interface{})
	}
	return nil
}
