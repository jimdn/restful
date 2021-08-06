package restful

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/globalsign/mgo/bson"
)

// A set of supported Field Kind
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

// Field definition
type Field struct {
	Kind       uint // field's kind
	CreateOnly bool // field can only be written when creating by POST or PUT
	ReadOnly   bool // field can not be written or update, data should be loaded into DB by other ways
}

// FieldSet is a structure to store DataStruct fields parsing result
type FieldSet struct {
	FMap map[string]Field // fields map
	FSli []string         // fields ordered
}

// BuildFieldSet is a function to parsing the DataStruct
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

// CheckObject check obj is valid or not
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
		path := append(prefix, k)
		full := strings.Join(path, ".")
		kind := KindInvalid
		ok := false

		// field contains dot
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
			// map field
			kind, ok = fs.IsMapMember(k)
			if ok {
				// check map field type
				v := ParseKindValue(value, kind-KindMapBase)
				if v == nil {
					invalidFields[k] = "type mismatch"
					delete(obj, k)
					continue
				}
				// check read only or create only
				if fs.IsFieldReadOnly(k) {
					invalidFields[k] = "read only"
					delete(obj, k)
					continue
				}
				if fs.IsFieldCreateOnly(k) {
					invalidFields[k] = "create only"
					delete(obj, k)
					continue
				}
				continue
			}
		}

		// ordinary field
		kind, ok = fs.IsFieldMember(full)
		if !ok {
			invalidFields[full] = "unknown"
			delete(obj, full)
			continue
		}
		// check read only or create only
		if dotOk {
			// PATCH method, update
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
			if fs.IsFieldReadOnly(full) {
				invalidFields[full] = "read only"
				delete(obj, full)
				continue
			}
		}
		// check field type
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

// IsFieldMember check field is a member of Struct or not
func (fs *FieldSet) IsFieldMember(field string) (uint, bool) {
	if _, ok := fs.FMap[field]; !ok {
		return fs.IsMapMember(field)
	}
	kind := fs.FMap[field].Kind
	return kind, true
}

// IsMapMember check field is a member of Struct map field or not
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

// IsFieldCreateOnly check field is create only or not
func (fs *FieldSet) IsFieldCreateOnly(field string) bool {
	if _, ok := fs.FMap[field]; ok {
		return fs.FMap[field].CreateOnly
	}
	return false
}

// IsFieldReadOnly check field is read only or not
func (fs *FieldSet) IsFieldReadOnly(field string) bool {
	if _, ok := fs.FMap[field]; ok {
		return fs.FMap[field].ReadOnly
	}
	return false
}

// SetCreateOnlyFields set the fields create only
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

// SetReadOnlyFields set the fields read only
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

// InReplace adapted MongoDB '_id' field
func (fs *FieldSet) InReplace(value *map[string]interface{}) {
	// id --> _id
	if v, ok := (*value)["id"]; ok {
		(*value)["_id"] = v
		delete(*value, "id")
	}
	if v, ok := (*value)["$or"]; ok {
		switch sli := v.(type) {
		case []interface{}:
			newOr := make([]map[string]interface{}, 0)
			for _, elem := range sli {
				switch m := elem.(type) {
				case map[string]interface{}:
					if vv, ok := m["id"]; ok {
						m["_id"] = vv
						delete(m, "id")
					}
					newOr = append(newOr, m)
				}
			}
			(*value)["$or"] = newOr
		}
	}
}

// OutReplace adapted MongoDB '_id' field
func (fs *FieldSet) OutReplace(value *map[string]interface{}) {
	// _id --> id
	if v, ok := (*value)["_id"]; ok {
		(*value)["id"] = v
		delete(*value, "_id")
	}
}

// OutReplaceArray adapted MongoDB '_id' field for ARRAY
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

// InSort sort data
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

// BuildFilterObj build the condition like `WHERE f1 = xxx AND ...` in SQL
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

// BuildRangeObj build the condition of `range` filter
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

// BuildInObj build the condition of `in` filter
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

// BuildNinObj build the condition of `nin` filter
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

// BuildAllObj build the condition of `all` filter
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

// BuildOrObj build the condition of `or` filter
func (fs *FieldSet) BuildOrObj(or []interface{}, cond map[string]interface{}) error {
	if _, exist := cond["$or"]; exist {
		return fmt.Errorf("or field condition conflict")
	}
	var err error
	orCond := make([]interface{}, 0)
	for _, obj := range or {
		switch m := obj.(type) {
		case map[string]interface{}:
			condition := make(map[string]interface{})
			for k, value := range m {
				switch k {
				case "filter":
					switch filter := value.(type) {
					case map[string]interface{}:
						err = fs.BuildFilterObj(filter, condition)
						if err != nil {
							return err
						}
					default:
						return fmt.Errorf("or field %v filter type not map", obj)
					}
				case "range":
					switch rang := value.(type) {
					case map[string]interface{}:
						err = fs.BuildRangeObj(rang, condition)
						if err != nil {
							return err
						}
					default:
						return fmt.Errorf("or field %v range type not map", obj)
					}
				case "in":
					switch in := value.(type) {
					case map[string]interface{}:
						err = fs.BuildInObj(in, condition)
						if err != nil {
							return err
						}
					default:
						return fmt.Errorf("or field %v in type not map", obj)
					}
				case "nin":
					switch nin := value.(type) {
					case map[string]interface{}:
						err = fs.BuildNinObj(nin, condition)
						if err != nil {
							return err
						}
					default:
						return fmt.Errorf("or field %v nin type not map", obj)
					}
				case "all":
					switch all := value.(type) {
					case map[string]interface{}:
						err = fs.BuildAllObj(all, condition)
						if err != nil {
							return err
						}
					default:
						return fmt.Errorf("or field %v all type not map", obj)
					}
				default:
					return fmt.Errorf("or field %v condition %v unknown", obj, k)
				}
			}
			orCond = append(orCond, condition)
		default:
			return fmt.Errorf("or field %v not map", obj)
		}
	}
	if len(orCond) > 0 {
		cond["$or"] = orCond
	}
	return nil
}

// BuildRegexSearchObj build the condition of `regex search` filter
func (fs *FieldSet) BuildRegexSearchObj(search string, regexSearchFields []string, cond map[string]interface{}) error {
	if _, exist := cond["$or"]; exist {
		return fmt.Errorf("or field condition conflict")
	}
	orCond := make([]interface{}, 0)
	for _, field := range regexSearchFields {
		condition := make(map[string]interface{})
		condition[field] = map[string]interface{}{"$regex": search}
		orCond = append(orCond, condition)
	}
	if len(orCond) > 0 {
		cond["$or"] = orCond
	}
	return nil
}

// BuildOrderArray build sort
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

// OrderArray2Slice convert the sort array to string slice
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

// BuildSelectObj build the select fields
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

// CheckSearchFields check the search fields in the config of Processor valid or not
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

// CheckRegexSearchFields check the search fields in the config of Processor valid or not
func (fs *FieldSet) CheckRegexSearchFields(fields []string) error {
	fields = RemoveDupArray(fields)
	for _, field := range fields {
		if len(field) <= 0 {
			return fmt.Errorf("regex search field %s invalid", field)
		}
		kind, ok := fs.IsFieldMember(field)
		if !ok {
			return fmt.Errorf("regex search field %s unknown", field)
		}
		if kind != KindString {
			return fmt.Errorf("regex search field %s not string", field)
		}
	}
	return nil
}

// BuildSearchContent concat the text according to the search fields
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

// CheckIndexFields check the index in the config of Processor valid or not
func (fs *FieldSet) CheckIndexFields(fields []string) ([]string, error) {
	if fields == nil || len(fields) == 0 {
		return nil, fmt.Errorf("index fields empty")
	}
	if len(fields) != len(RemoveDupArray(fields)) {
		return nil, fmt.Errorf("index fields dup")
	}
	formatFields := make([]string, 0)
	for i, field := range fields {
		if len(field) <= 1 {
			return nil, fmt.Errorf("index fields[%d]=%s invalid", i, field)
		}
		r, k := field[0], field[1:]
		if r != '+' && r != '-' {
			return nil, fmt.Errorf("index fields[%d]=%s should start with +/- ", i, field)
		}
		if k == "id" {
			return nil, fmt.Errorf("index fields should not contains id field")
		}
		if _, ok := fs.IsFieldMember(k); !ok {
			return nil, fmt.Errorf("index fields[%d]=%s unknown", i, field)
		}
		if r == '+' {
			formatFields = append(formatFields, k)
		} else {
			formatFields = append(formatFields, field)
		}
	}
	return formatFields, nil
}

// ParseSimpleValue parse a simple kind of value
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

// ParseSimpleArray parse a simple array kind of value
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

// ParseKindValue parse all kind of value
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

// ParseKindArray parse all array kind of value
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

// ParseKindMap parse map kind of value
func ParseKindMap(value map[string]interface{}, kind uint) interface{} {
	for _, v := range value {
		if ParseKindValue(v, kind-KindMapBase) == nil {
			return nil
		}
	}
	return value
}

// IsEmpty check value is nin or default value of its kind
func IsEmpty(value interface{}, kind uint) bool {
	k := kind
	switch {
	case k == KindBool:
		return IsEmptyBool(value)
	case KindInt <= k && k < KindFloat:
		return IsEmptyNumber(value)
	case k == KindString:
		return IsEmptyString(value)
	case k == KindObject:
		return IsEmptyObject(value)
	case KindArrayBase < k && k < KindArrayEnd:
		return IsEmptyArray(value)
	case KindMapBase < k && k < KindMapEnd:
		return IsEmptyObject(value)
	}
	return false
}

// EmptyValue get the default value of kind
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
