package domain

import "errors"

var (
	ErrLinkNotFound = errors.New("link not found")
	ErrLinkExpired  = errors.New("link expired")
	ErrInvalidSlug  = errors.New("invalid slug")
)

