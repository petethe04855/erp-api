package live

import "errors"

var (
	ErrSessionNotFound         = errors.New("live session not found")
	ErrSessionOverlap          = errors.New("live session time overlaps with another session for this staff")
	ErrSessionNotEditable      = errors.New("only pending live sessions can be edited")
	ErrRejectionReasonRequired = errors.New("rejection reason is required")
	ErrInvalidSessionTimes     = errors.New("invalid session start or end time")
	ErrContentItemNotFound     = errors.New("content item not found")
)
