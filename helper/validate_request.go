package helper

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

func ValidateRequest(ctx *fiber.Ctx, v *validator.Validate, request any) error {
	err := ctx.BodyParser(request)
	if err != nil {
		return err
	}

	if err := v.Struct(request); err != nil {
		if errs, ok := err.(validator.ValidationErrors); ok {
			var messages []string
			for _, fe := range errs {
				fieldPath := getJSONField(request, fe)
				messages = append(messages, fmt.Sprintf("%s is %s", fieldPath, fe.ActualTag()))
			}
			return fmt.Errorf("%s", strings.Join(messages, ", "))
		}
	}

	return nil
}

func ValidateQueryParams(ctx *fiber.Ctx, c *validator.Validate, request any) error {
	err := ctx.QueryParser(request)
	if err != nil {
		return err
	}

	if err := c.Struct(request); err != nil {
		var errMessage string
		for _, err := range err.(validator.ValidationErrors) {
			fieldName := err.Field()
			errMessage = strings.ToLower(fieldName) + " is " + err.ActualTag()
		}

		return fmt.Errorf("%v", errMessage)
	}

	return nil
}

func getJSONField(obj interface{}, fe validator.FieldError) string {
	namespaceParts := strings.Split(fe.StructNamespace(), ".")[1:]

	typ := reflect.TypeOf(obj)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	var pathParts []string
	for _, part := range namespaceParts {
		fieldName := part
		indexSuffix := ""

		if idx := strings.Index(part, "["); idx != -1 {
			fieldName = part[:idx]
			indexSuffix = part[idx:]
		}

		for typ.Kind() == reflect.Ptr || typ.Kind() == reflect.Slice {
			typ = typ.Elem()
		}

		field, found := typ.FieldByName(fieldName)
		if !found {
			pathParts = append(pathParts, fieldName+indexSuffix)
			continue
		}

		jsonTag := field.Tag.Get("json")
		if commaIdx := strings.Index(jsonTag, ","); commaIdx != -1 {
			jsonTag = jsonTag[:commaIdx]
		}
		if jsonTag == "" {
			jsonTag = strings.ToLower(field.Name)
		}

		pathParts = append(pathParts, jsonTag+indexSuffix)

		typ = field.Type
	}

	return strings.Join(pathParts, ".")
}
