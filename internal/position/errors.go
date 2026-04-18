package position

import "errors"

var (
	ErrNotAssigned = errors.New("position not assigned")
	ErrUnknown     = errors.New("position unknown")
)
