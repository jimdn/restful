package restful

import (
	"github.com/globalsign/mgo/bson"
	"github.com/jimdn/objectid"
	"github.com/nu7hatch/gouuid"
	"math/rand"
)

// RandString is an function to gen a rand string
func RandString(n int) string {
	letter := []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]byte, n)
	for i := range b {
		b[i] = letter[rand.Intn(len(letter))]
	}
	return string(b)
}

// GenUniqueID is an function to gen a unique id with STRING type
// support objectid or uuid
func GenUniqueID() string {
	if gCfg.DefaultIdGenerator == "objectid" {
		return objectid.New().String()
	}
	u, _ := uuid.NewV4()
	return u.String()
}

// GetStringD check s type
// if s is String, return its value
// if s is not STRING, return default d
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

// GetString check s type
// if s is String, return its value
// if s is not STRING, return empty string
func GetString(s interface{}) string {
	return GetStringD(s, "")
}

// CheckBool check v type
// if v is BOOL, return v
// if v is not BOOL, return nil
func CheckBool(v interface{}) interface{} {
	if b, ok := v.(bool); ok {
		return b
	}
	return nil
}

// CheckInt check value type
// if value is any type represent INT, return INT64 value
// if value is not any type represent INT, return nil
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
		return v
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

// CheckUint check value type
// if value is any type represent UINT, return UINT64 value
// if value is not any type represent UINT, return nil
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
		return v
	case float32:
		return uint64(v)
	case float64:
		return uint64(v)
	}
	return nil
}

// CheckFloat check value type
// if value is any type represent FLOAT, return FLOAT64 value
// if value is not any type represent FLOAT, return nil
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
		return v
	}
	return nil
}

// CheckString check value type
// if value is any type represent STRING, return STRING value
// if value is not any type represent STRING, return nil
func CheckString(value interface{}) interface{} {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	}
	return nil
}

// CheckObject check value type
// if value is OBJECT, return its value
// if value is not OBJECT, return nil
func CheckObject(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		return v
	}
	return nil
}

// IsEmptyBool check whether value is empty
// if value is nil or default value of bool, return true
func IsEmptyBool(value interface{}) bool {
	if value == nil {
		return true
	}
	v, ok := value.(bool)
	return ok && v == false
}

// IsEmptyNumber check whether value is empty
// if value is nil or default value of float64, return true
func IsEmptyNumber(value interface{}) bool {
	if value == nil {
		return true
	}
	v, ok := value.(float64)
	return ok && v == 0
}

// IsEmptyString check whether value is empty
// if value is nil or default value of string, return true
func IsEmptyString(value interface{}) bool {
	if value == nil {
		return true
	}
	v, ok := value.(string)
	return ok && v == ""
}

// IsEmptyArray check whether value is empty
// if value is nil or empty array, return true
func IsEmptyArray(value interface{}) bool {
	if value == nil {
		return true
	}
	v, ok := value.([]interface{})
	return ok && len(v) == 0
}

// IsEmptyObject check whether value is empty
// if value is nil or empty object, return true
func IsEmptyObject(value interface{}) bool {
	if value == nil {
		return true
	}
	switch v := value.(type) {
	case map[string]interface{}:
		return len(v) == 0
	case bson.M:
		return len(v) == 0
	}
	return false
}

// RemoveDupArray remove duplicate elements
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
