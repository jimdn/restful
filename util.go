package restful

import (
	"github.com/globalsign/mgo/bson"
	"github.com/nu7hatch/gouuid"
)


func UUID() string {
	u, _ := uuid.NewV4()
	return u.String()
}

func GetStringD(s interface{}, d string) string {
	if s == nil {
		return d
	}
	switch v := s.(type) {
	case string:
		return v
	case *interface{}:
		return GetStringD(*v, d)
	}
	return d
}

func GetString(s interface{}) string {
	return GetStringD(s, "")
}

func CheckBool(value interface{}) interface{} {
	if v, ok := value.(bool); ok {
		return bool(v)
	}
	return nil
}

func CheckInt(value interface{}) interface{} {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int8:
		return int64(v)
	case int16:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return int64(v)
	case uint:
		return int64(v)
	case uint8:
		return int64(v)
	case uint16:
		return int64(v)
	case uint32:
		return int64(v)
	case uint64:
		return int64(v)
	case float32:
		return int64(v)
	case float64:
		return int64(v)
	}

	return nil
}

func CheckUint(value interface{}) interface{} {
	switch v := value.(type) {
	case int:
		return uint64(v)
	case int8:
		return uint64(v)
	case int16:
		return uint64(v)
	case int32:
		return uint64(v)
	case int64:
		return uint64(v)
	case uint:
		return uint64(v)
	case uint8:
		return uint64(v)
	case uint16:
		return uint64(v)
	case uint32:
		return uint64(v)
	case uint64:
		return uint64(v)
	case float32:
		return uint64(v)
	case float64:
		return uint64(v)
	}
	return nil
}

func CheckFloat(value interface{}) interface{} {
	switch v := value.(type) {
	case int:
		return float64(v)
	case int8:
		return float64(v)
	case int16:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case uint:
		return float64(v)
	case uint8:
		return float64(v)
	case uint16:
		return float64(v)
	case uint32:
		return float64(v)
	case uint64:
		return float64(v)
	case float32:
		return float64(v)
	case float64:
		return float64(v)
	}
	return nil
}

func CheckString(value interface{}) interface{} {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	}
	return nil
}

func CheckObject(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		return v
	}
	return nil
}

func IsEmptyBool(d interface{}) bool {
	if d == nil {
		return true
	}
	v, ok := d.(bool)
	return ok && v == false
}

func IsEmptyNumber(d interface{}) bool {
	if d == nil {
		return true
	}
	v, ok := d.(float64)
	return ok && v == 0
}

func IsEmptyString(d interface{}) bool {
	if d == nil {
		return true
	}
	v, ok := d.(string)
	return ok && v == ""
}

func IsEmptyArray(d interface{}) bool {
	if d == nil {
		return true
	}
	v, ok := d.([]interface{})
	return ok && len(v) == 0
}

func IsEmptyObject(d interface{}) bool {
	if d == nil {
		return true
	}
	switch v := d.(type) {
	case map[string]interface{}:
		return len(v) == 0
	case bson.M:
		return len(v) == 0
	}
	return false
}

func RemoveDupArray(s []string) []string {
	m := make(map[string]bool)
	for i := range s {
		m[s[i]] = true
	}
	o := make([]string, 0, len(m))
	for k := range m {
		o = append(o, k)
	}
	return o
}
