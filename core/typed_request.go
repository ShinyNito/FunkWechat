package core

import (
	"context"
	"errors"
	"fmt"
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
	return r.do(ctx, r.builder.Get)
}

func (r *TypedRequest[T]) Post(ctx context.Context) (T, error) {
	return r.do(ctx, r.builder.Post)
}

// do 发送请求并解码。access_token 失效（40001/42001）时刷新 token 并重试一次，
// 用于缓存中的 token 已被其他服务作废的情况。上传请求的文件流无法重放，不重试。
func (r *TypedRequest[T]) do(ctx context.Context, send func(context.Context) (RawResponse, error)) (T, error) {
	out, err := decode[T](send(ctx))
	if !IsTokenError(err) || !r.builder.canRetryWithNewToken() {
		return out, err
	}
	if _, refreshErr := r.builder.client.tokenProvider.RefreshToken(ctx); refreshErr != nil {
		return out, errors.Join(err, fmt.Errorf("refresh access token: %w", refreshErr))
	}
	return decode[T](send(ctx))
}

func decode[T any](resp RawResponse, err error) (T, error) {
	if err != nil {
		var zero T
		return zero, err
	}
	return DecodeWechat[T](resp.StatusCode, resp.Body)
}
