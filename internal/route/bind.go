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

func Bind(ptr any, params []Param) error {
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
	for _, param := range params {
		name, raw := param.Name, param.Value
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

func BindQuery(ptr any, values map[string][]string) error {
	if len(values) == 0 {
		return nil
	}
	v := reflect.ValueOf(ptr)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return errors.New("binding query parameters: input is not a pointer")
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return errors.New("binding query parameters: input is not a struct")
	}
	for name, raw := range values {
		if len(raw) == 0 {
			continue
		}
		field, ok := FieldByWireName(v.Type(), name)
		if !ok {
			continue
		}
		target := v.FieldByIndex(field.Index)
		if target.Kind() == reflect.Pointer {
			if target.IsNil() {
				target.Set(reflect.New(target.Type().Elem()))
			}
			target = target.Elem()
		}
		if target.Kind() == reflect.Slice {
			slice := reflect.MakeSlice(target.Type(), 0, len(raw))
			for _, item := range raw {
				elem := reflect.New(target.Type().Elem()).Elem()
				if err := setScalar(elem, item, name); err != nil {
					return err
				}
				slice = reflect.Append(slice, elem)
			}
			target.Set(slice)
			continue
		}
		if err := setScalar(target, raw[len(raw)-1], name); err != nil {
			return err
		}
	}
	return nil
}

func QueryBindable(t reflect.Type) bool {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() == reflect.Slice {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.String, reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

func setScalar(target reflect.Value, raw, name string) error {
	switch target.Kind() {
	case reflect.String:
		target.SetString(raw)
	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return fmt.Errorf("query parameter %q must be true or false", name)
		}
		target.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(raw, 10, target.Type().Bits())
		if err != nil {
			return fmt.Errorf("query parameter %q must be a whole number", name)
		}
		target.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(raw, 10, target.Type().Bits())
		if err != nil {
			return fmt.Errorf("query parameter %q must be a whole number that is not negative", name)
		}
		target.SetUint(n)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(raw, target.Type().Bits())
		if err != nil {
			return fmt.Errorf("query parameter %q must be a number", name)
		}
		target.SetFloat(f)
	default:
		return fmt.Errorf("query parameter %q maps to a field of an unsupported kind", name)
	}
	return nil
}
