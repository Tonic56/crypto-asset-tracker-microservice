package errs

import "errors"

var ErrNotFound = errors.New("not found")

var ErrAlreadyExists = errors.New("already exists")

var ErrInternal = errors.New("internal error")

var ErrInsufficientFunds = errors.New("insufficient funds")
