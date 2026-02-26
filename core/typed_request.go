package core

import (
	"context"
	"io"
)

type TypedRequest[T any] struct {
	builder *RequestBuilder
}

func NewTypedRequest[T any](client *Client) *TypedRequest[T] {
	return &TypedRequest[T]{builder: newRequestBuilder(client)}
}

func (r *TypedRequest[T]) Path(path string) *TypedRequest[T] {
	r.builder.Path(path)
	return r
}

func (r *TypedRequest[T]) Query(key, value string) *TypedRequest[T] {
	r.builder.Query(key, value)
	return r
}

func (r *TypedRequest[T]) QueryMap(query map[string]string) *TypedRequest[T] {
	r.builder.QueryMap(query)
	return r
}

func (r *TypedRequest[T]) Body(body any) *TypedRequest[T] {
	r.builder.Body(body)
	return r
}

func (r *TypedRequest[T]) WithoutToken() *TypedRequest[T] {
	r.builder.WithoutToken()
	return r
}

func (r *TypedRequest[T]) UploadFile(field, fileName string, reader io.Reader) *TypedRequest[T] {
	r.builder.UploadFile(field, fileName, reader)
	return r
}

func (r *TypedRequest[T]) UploadField(key, value string) *TypedRequest[T] {
	r.builder.UploadField(key, value)
	return r
}

func (r *TypedRequest[T]) Get(ctx context.Context) (T, error) {
	resp, err := r.builder.Get(ctx)
	if err != nil {
		var zero T
		return zero, err
	}
	return DecodeWechat[T](resp.StatusCode, resp.Body)
}

func (r *TypedRequest[T]) Post(ctx context.Context) (T, error) {
	resp, err := r.builder.Post(ctx)
	if err != nil {
		var zero T
		return zero, err
	}
	return DecodeWechat[T](resp.StatusCode, resp.Body)
}
