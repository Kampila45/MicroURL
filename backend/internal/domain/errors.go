package domain

import "errors"

var (
	ErrLinkNotFound     = errors.New("link not found")
	ErrLinkExpired      = errors.New("link expired")
	ErrInvalidSlug      = errors.New("invalid slug")
	ErrInvalidAlias     = errors.New("invalid custom alias")
	ErrAliasConflict    = errors.New("custom alias is already taken")
	ErrInvalidURL       = errors.New("invalid url")
	ErrExpiredTimestamp = errors.New("expires_at must be in the future")
)

