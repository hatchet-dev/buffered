package buffered

import "github.com/go-playground/validator/v10"

func Validate(s any) error {
	validate := validator.New()

	return validate.Struct(s)
}
