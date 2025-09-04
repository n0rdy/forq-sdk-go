package api

type MessageResponse struct {
	Id      string `json:"id"`
	Content string `json:"content"`
}

type ErrorResponse struct {
	Code string `json:"code,omitempty"`
}

func (e *ErrorResponse) Error() string {
	return e.Code
}
