
package errs

import "errors"

var (
	ErrDB              = errors.New("database error")
	ErrRecordingWNC    = errors.New("recording wasn't created")
	ErrRecordingWND    = errors.New("recording wasn't deleted")
	ErrRecordingWNF    = errors.New("recording wasn't found")
	ErrUserExists      = errors.New("user already exists")
	ErrInvalidToken    = errors.New("invalid or expired token")
	ErrSessionNotFound = errors.New("session not found")
)
