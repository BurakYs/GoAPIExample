package middleware

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"

	"github.com/BurakYs/GoAPIExample/internal/models"
)

const (
	BindingLocationBody   = "body"
	BindingLocationQuery  = "query"
	BindingLocationParams = "params"
	BindingLocationForm   = "form"
)

func ValidateBody[T any]() fiber.Handler {
	return validate[T](BindingLocationBody)
}

func ValidateQuery[T any]() fiber.Handler {
	return validate[T](BindingLocationQuery)
}

func ValidateParams[T any]() fiber.Handler {
	return validate[T](BindingLocationParams)
}

func ValidateForm[T any]() fiber.Handler {
	return validate[T](BindingLocationForm)
}

type transformable interface {
	Transform()
}

func validate[T any](location string) fiber.Handler {
	return func(c fiber.Ctx) error {
		var data T
		var err error

		switch location {
		case BindingLocationBody:
			err = c.Bind().Body(&data)
		case BindingLocationQuery:
			err = c.Bind().Query(&data)
		case BindingLocationParams:
			err = c.Bind().URI(&data)
		case BindingLocationForm:
			err = c.Bind().Form(&data)
		}

		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(formatValidationError(err, location, data))
		}

		if t, ok := any(&data).(transformable); ok {
			t.Transform()
		}

		c.Locals(location, data)
		return c.Next()
	}
}

func formatValidationError(err error, location string, data any) any {
	var ve validator.ValidationErrors

	switch {
	case errors.As(err, &ve):
		failures := make([]models.ValidationFailure, 0, len(ve))

		for _, fe := range ve {
			field := getFieldName(fe.StructField(), data)
			failures = append(failures, formatFieldError(fe, location, field))
		}

		return models.ValidationError{
			Message:            "Invalid parameters provided",
			ValidationFailures: failures,
		}
	default:
		return models.APIError{
			Message: "Invalid parameters provided",
		}
	}
}

func formatFieldError(fe validator.FieldError, location, field string) models.ValidationFailure {
	msg := fmt.Sprintf("This field is invalid for tag: %s", fe.Tag())

	switch fe.Tag() {
	case "required":
		msg = "This field is required"
	case "email":
		msg = "This field must be a valid email address"
	case "uuid":
		msg = "This field must be a valid UUID"
	case "min":
		switch fe.Kind() {
		case reflect.String:
			msg = fmt.Sprintf("This field must be at least %s characters long", fe.Param())
		case reflect.Slice, reflect.Array:
			msg = fmt.Sprintf("This field must contain at least %s items", fe.Param())
		default:
			msg = fmt.Sprintf("The value must be at least %s", fe.Param())
		}
	case "max":
		switch fe.Kind() {
		case reflect.String:
			msg = fmt.Sprintf("This field must be at most %s characters long", fe.Param())
		case reflect.Slice, reflect.Array:
			msg = fmt.Sprintf("This field must contain at most %s items", fe.Param())
		default:
			msg = fmt.Sprintf("The value must be at most %s", fe.Param())
		}
	}

	return models.ValidationFailure{
		Location: location,
		Field:    field,
		Message:  msg,
	}
}

func getFieldName(structField string, obj any) string {
	t := reflect.TypeOf(obj)

	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if f, ok := t.FieldByName(structField); ok {
		tags := []string{"json", "query", "uri", "form", "header", "cookie", "cbor", "respHeader", "xml"}

		for _, tag := range tags {
			if tagValue := f.Tag.Get(tag); tagValue != "" && tagValue != "-" {
				return strings.Split(tagValue, ",")[0]
			}
		}
	}

	return ""
}
