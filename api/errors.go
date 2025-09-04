package api

const (
	ErrCodeBadRequestContentExceedsLimit = "bad_request.body.content.exceeds_limit"
	ErrCodeBadRequestProcessAfterInPast  = "bad_request.body.processAfter.in_past"
	ErrCodeBadRequestProcessAfterTooFar  = "bad_request.body.processAfter.too_far"
	ErrCodeBadRequestInvalidBody         = "bad_request.body.invalid"
	ErrCodeBadRequestDlqOnlyOp           = "bad_request.dlq_only_operation"
	ErrCodeUnauthorized                  = "unauthorized"
	ErrCodeNotFoundMessage               = "not_found.message"
	ErrCodeInternal                      = "internal"
)
