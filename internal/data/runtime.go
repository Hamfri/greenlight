package data

import (
	"fmt"
	"strconv"
)

type Runtime int

// here we deliberately use a value receiver instead of a pointer receiver
// because value methods can be invoked on pointers and values.
// if we were modifying the data a pointer reeceiver would make more sense. see learning Go
func (r Runtime) MarshalJSON() ([]byte, error) {
	// format data accordingly
	jsonValue := fmt.Sprintf("%d mins", r)

	// wrap the jsonValue in double quotes
	quotedJSONValue := strconv.Quote(jsonValue)

	// convert quoted string into a byte slice
	return []byte(quotedJSONValue), nil
}
