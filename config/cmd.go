package config

import (
	"fmt"
	"reflect"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func loadConfigFlags(cmd *cobra.Command, prefix string, configuration interface{}) error {
	val := reflect.ValueOf(configuration)
	typ := reflect.TypeOf(configuration)

	if val.Kind() == reflect.Ptr {
		val = val.Elem()
		typ = typ.Elem()
	}

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("mapstructure")
		description := field.Tag.Get("description")
		fieldValue := val.Field(i)

		if tag == "" {
			continue
		}

		fullTag := tag
		if prefix != "" {
			fullTag = prefix + "." + tag
		}

		switch fieldValue.Kind() {
		case reflect.String:
			cmd.PersistentFlags().String(fullTag, fieldValue.String(), description)
			err := viper.BindPFlag(fullTag, cmd.PersistentFlags().Lookup(fullTag))

			if err != nil {
				return fmt.Errorf("error binding flag %s: %v", fullTag, err)
			}
		case reflect.Slice:
			if fieldValue.Type().Elem().Kind() == reflect.String {
				cmd.PersistentFlags().StringSlice(fullTag, fieldValue.Interface().([]string), description)
				err := viper.BindPFlag(fullTag, cmd.PersistentFlags().Lookup(fullTag))

				if err != nil {
					return fmt.Errorf("error binding flag %s: %v", fullTag, err)
				}
			} else {
				return fmt.Errorf("unsupported slice type: %v", fieldValue.Type().Elem().Kind())
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			cmd.PersistentFlags().Int(fullTag, int(fieldValue.Int()), description)
			err := viper.BindPFlag(fullTag, cmd.PersistentFlags().Lookup(fullTag))

			if err != nil {
				return fmt.Errorf("error binding flag %s: %v", fullTag, err)
			}
		case reflect.Bool:
			cmd.PersistentFlags().Bool(fullTag, fieldValue.Bool(), description)
			err := viper.BindPFlag(fullTag, cmd.PersistentFlags().Lookup(fullTag))

			if err != nil {
				return fmt.Errorf("error binding flag %s: %v", fullTag, err)
			}
		case reflect.Struct:
			// Recursively handle nested structs.
			loadConfigFlags(cmd, fullTag, fieldValue.Interface())
		}
	}

	return nil
}

func LoadConfigFlags[T any](cmd *cobra.Command) error {
	return loadConfigFlags(cmd, "", new(T))
}
