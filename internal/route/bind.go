package route

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

func Bindable(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	}
	return false
}

func FieldByWireName(t reflect.Type, name string) (reflect.StructField, bool) {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return reflect.StructField{}, false
	}
	for i := range t.NumField() {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		wire := f.Name
		if tag, ok := f.Tag.Lookup("json"); ok {
			if head, _, _ := strings.Cut(tag, ","); head != "" {
				if head == "-" {
					continue
				}
				wire = head
			}
		}
		if wire == name {
			return f, true
		}
	}
	return reflect.StructField{}, false
}

func Bind(ptr any, params map[string]string) error {
	if len(params) == 0 {
		return nil
	}
	v := reflect.ValueOf(ptr)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return errors.New("binding path parameters: input is not a pointer")
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return errors.New("binding path parameters: input is not a struct")
	}
	for name, raw := range params {
		field, ok := FieldByWireName(v.Type(), name)
		if !ok {
			return fmt.Errorf("path parameter %q has no matching input field", name)
		}
		target := v.FieldByIndex(field.Index)
		switch target.Kind() {
		case reflect.String:
			target.SetString(raw)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			n, err := strconv.ParseInt(raw, 10, target.Type().Bits())
			if err != nil {
				return fmt.Errorf("path parameter %q must be a whole number", name)
			}
			target.SetInt(n)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			n, err := strconv.ParseUint(raw, 10, target.Type().Bits())
			if err != nil {
				return fmt.Errorf("path parameter %q must be a whole number that is not negative", name)
			}
			target.SetUint(n)
		default:
			return fmt.Errorf("path parameter %q maps to a field of an unsupported kind", name)
		}
	}
	return nil
}
