package api

// Response 正常响应
type Response struct {
	Tracks []string `json:"tracks"`
	Sdp64  string   `json:"sdp64"`
}

// ResponseError 错误响应
type ResponseError struct {
	Error string `json:"error"`
}
