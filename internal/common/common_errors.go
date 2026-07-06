package common

import "errors"

var (
	ErrWorkspaceNotFound = errors.New("workspace not found")
	ErrItemNotFound      = errors.New("item not found")
)
