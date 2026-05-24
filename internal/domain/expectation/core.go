package expectation

import "errors"

// ErrNotVerified — аккаунт не верифицирован
var (
	ErrNotVerified = errors.New("account is not verified")
	ErrNotFound    = errors.New("moderator_account not found")
)
