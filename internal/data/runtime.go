package data

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var ErrInvalidRuntimeFormat = errors.New("invalid runtime format")

type Runtime int32

// here we deliberately use a value receiver instead of a pointer receiver
// because value methods can be invoked on pointers and values.
// if we were modifying data a pointer receiver would make more sense. see learning Go
// since we satisfy `Marshaler interface` this method is called automatically when we invoke json.MarshalIndent in helpers/writeJSON
func (r Runtime) MarshalJSON() ([]byte, error) {
	// format data accordingly
	jsonValue := fmt.Sprintf("%d mins", r)

	// wrap the jsonValue in double quotes
	quotedJSONValue := strconv.Quote(jsonValue)

	// convert quoted string into a byte slice
	return []byte(quotedJSONValue), nil
}

// to persist the changes to runtime we must use a pointer
// otherwise the changes will only be visible within this function
// since we satisfy `Unmarshaler interface` this method is called automatically when we invoke json.NewDecoder in helpers/readJSON
func (r *Runtime) UnmarshalJSON(jsonValue []byte) error {
	// %q jsonValue equals >>> "\"107 mins\""
	unquotedJSONValue, err := strconv.Unquote(string(jsonValue))
	if err != nil {
		return ErrInvalidRuntimeFormat
	}

	parts := strings.Split(unquotedJSONValue, " ")
	if len(parts) != 2 || parts[1] != "mins" {
		return ErrInvalidRuntimeFormat
	}

	// parse string to an int
	i, err := strconv.Atoi(parts[0])
	if err != nil {
		return ErrInvalidRuntimeFormat
	}

	// convert int to Runtime type and assign it to the receiver.
	// * is an indirection operator used to return pointed-to value i.e dereference
	*r = Runtime(i)

	return nil
}
