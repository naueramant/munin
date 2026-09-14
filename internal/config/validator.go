package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	cronparser "github.com/robfig/cron/v3"
	"gopkg.in/go-playground/validator.v9"
)

var syntaxes = []string{"v1"}

var scheduleWhenParser = cronparser.NewParser(
	cronparser.Minute | cronparser.Hour | cronparser.Dom | cronparser.Month | cronparser.Dow | cronparser.Descriptor,
)

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

// validScheduleWhen reports whether a schedule "when" value is a valid cron
// expression, an "HH:MM" time-of-day, or a Go duration string.
func validScheduleWhen(expr string) bool {
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" {
		return false
	}

	if strings.Contains(trimmed, ":") && len(strings.Fields(trimmed)) == 1 {
		parts := strings.Split(trimmed, ":")
		if len(parts) == 2 {
			hour, errH := strconv.Atoi(parts[0])
			min, errM := strconv.Atoi(parts[1])
			if errH == nil && errM == nil && hour >= 0 && hour < 24 && min >= 0 && min < 60 {
				return true
			}
		}
	}

	if _, err := scheduleWhenParser.Parse(trimmed); err == nil {
		return true
	}

	if d, err := time.ParseDuration(trimmed); err == nil && d > 0 {
		return true
	}

	return false
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
