package contract

import "reflect"

func GoTypeName(t reflect.Type) string {
	if t.Name() == "" {
		if t.Kind() == reflect.Struct && t.NumField() == 0 {
			return "struct{}"
		}
		return t.String()
	}
	if t.PkgPath() == "" {
		return t.Name()
	}
	return t.PkgPath() + "." + t.Name()
}
