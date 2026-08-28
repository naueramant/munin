package config

import (
	"fmt"
	"strings"

	"gopkg.in/go-playground/validator.v9"
)

var syntaxes = []string{"v1"}

func Validate(t Configuration) error {
	validate := validator.New()

	if err := validate.RegisterValidation("syntax", validateSyntax); err != nil {
		return err
	}

	err := validate.Struct(t)
	if err != nil {
		if valErrors, ok := err.(validator.ValidationErrors); ok {
			for _, valErr := range valErrors {
				switch valErr.Field() {
				case "Syntax":
					return fmt.Errorf("malformed config, %s: %v is not a valid syntax version", valErr.StructNamespace(), valErr.Value())
				default:
					return fmt.Errorf("malformed config, %s: %s is required", valErr.StructNamespace(), strings.ToLower(valErr.StructField()))
				}
			}
		}
		return err
	}

	return nil
}

func validateSyntax(fl validator.FieldLevel) bool {
	return contains(syntaxes, fl.Field().String())
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
