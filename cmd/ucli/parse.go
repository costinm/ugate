package main

import (
	"errors"
	"reflect"
	"strings"

	"github.com/spf13/cobra"
)

var (
	Err = errors.New("error")
)

// ParseStruct parses structure and add flags.
func ParseStruct(cmd *cobra.Command, cfg any) (error) {
	if cfg == nil {
		return nil
	}

	v := reflect.ValueOf(cfg)

	if v.Kind() != reflect.Ptr {
		return nil
	}

	if v.IsNil() {
		return nil
	}

	switch e := v.Elem(); e.Kind() {
	case reflect.Struct:
		return parseStruct(cmd, e)
	default:
		return nil
	}
}


//func parseVal(value reflect.Value) ([]*Flag, Value, error) {
//	// value is addressable, let's check if we can parse it
//	if value.CanAddr() && value.Addr().CanInterface() {
//		valueInterface := value.Addr().Interface()
//		val := parseGenerated(valueInterface)
//
//		if val != nil {
//			return nil, val, nil
//		}
//		// check if field implements Value interface
//		if val, casted := valueInterface.(Value); casted {
//			return nil, val, nil
//		}
//	}
//
//	switch value.Kind() {
//	case reflect.Ptr:
//		if value.IsNil() {
//			value.Set(reflect.New(value.Type().Elem()))
//		}
//
//		val := parseGeneratedPtrs(value.Addr().Interface())
//
//		if val != nil {
//			return nil, val, nil
//		}
//
//		return parseVal(value.Elem(), optFuncs...)
//
//	case reflect.Struct:
//		flags, err := parseStruct(value, optFuncs...)
//
//		return flags, nil, err
//
//	case reflect.Map:
//		val := parseMap(value)
//
//		return nil, val, nil
//	}
//
//	return nil, nil, nil
//}
//
func parseStruct(cmd *cobra.Command, value reflect.Value) (error) {

	valueType := value.Type()
fields:
	for i := 0; i < value.NumField(); i++ {

		field := valueType.Field(i)
		// skip unexported and non anonymous fields
		if field.PkgPath != "" && !field.Anonymous {
			continue fields
		}

		//fieldValue := value.Field(i)
		// Tag is a string - usually  key:"value" key2:"value2"

		switch field.Type.Kind() {
		case reflect.String:
			cmd.PersistentFlags().String(strings.ToLower(field.Name), field.Tag.Get("def"), field.Tag.Get("usage"))
		}

		continue fields
	}

	return nil
}

//func parseMap(value reflect.Value) Value {
//	mapType := value.Type()
//	keyKind := value.Type().Key().Kind()
//
//	// check that map key is string or integer
//	if !anyOf(MapAllowedKinds, keyKind) {
//		return nil
//	}
//
//	if value.IsNil() {
//		value.Set(reflect.MakeMap(mapType))
//	}
//
//	valueInterface := value.Addr().Interface()
//	val := parseGeneratedMap(valueInterface)
//
//	return val
//}
