package web

import (
	"bytes"
	"net/http"
)

// Flushing or hijacking would return a response before commit
type transactionResponse struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (r *transactionResponse) Header() http.Header {
	return r.header
}

func (r *transactionResponse) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
}

func (r *transactionResponse) Write(body []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.body.Write(body)
}

func (r *transactionResponse) publish(response *Response) error {
	for key := range response.Header() {
		delete(response.Header(), key)
	}
	for key, values := range r.header {
		response.Header()[key] = values
	}
	if r.status == 0 {
		return nil
	}
	response.WriteHeader(r.status)
	_, err := response.Write(r.body.Bytes())
	return err
}
