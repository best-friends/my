// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"fmt"
	"reflect"
	"strings"
)

// nulPath returns the path, starting at path, of the first string under v that has a NUL, or "".
// It follows pointers, interfaces, structs (unexported fields included), slices, arrays, and maps.
func nulPath(v reflect.Value, path string) string {
	switch v.Kind() {
	case reflect.String:
		if strings.Contains(v.String(), "\x00") {
			return path
		}
	case reflect.Pointer, reflect.Interface:
		if !v.IsNil() {
			return nulPath(v.Elem(), path)
		}
	case reflect.Struct:
		for i := range v.NumField() {
			if p := nulPath(v.Field(i), path+"."+v.Type().Field(i).Name); p != "" {
				return p
			}
		}
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			if p := nulPath(v.Index(i), fmt.Sprintf("%s[%d]", path, i)); p != "" {
				return p
			}
		}
	case reflect.Map:
		for it := v.MapRange(); it.Next(); {
			if p := nulPath(it.Value(), fmt.Sprintf("%s[%v]", path, it.Key())); p != "" {
				return p
			}
		}
	}
	return ""
}
