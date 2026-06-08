package utils

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func FormatValidationErrors(err error) []FieldError {
	var errors []FieldError

	for _, fe := range err.(validator.ValidationErrors) {
		errors = append(errors, FieldError{
			Field:   toSnakeCase(fe.Field()),
			Message: msgForTag(fe),
		})
	}

	return errors
}

func msgForTag(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("%s is required.", fe.Field())
	case "min":
		return fmt.Sprintf("%s must be at least %s.", fe.Field(), fe.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s.", fe.Field(), fe.Param())
	case "email":
		return fmt.Sprintf("%s must be a valid email.", fe.Field())
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s.", fe.Field(), fe.Param())
	default:
		return fmt.Sprintf("%s failed %s validation.", fe.Field(), fe.Tag())
	}
}

func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}
