package api

type MessageResponse struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	// Receipt identifies this particular delivery of the message and is
	// required by the Forq server on ack/nack. Opaque - do not parse it.
	Receipt string `json:"receipt"`
}

type ErrorResponse struct {
	Code string `json:"code,omitempty"`
}

func (e *ErrorResponse) Error() string {
	return e.Code
}
