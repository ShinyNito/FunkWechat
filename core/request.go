package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"
)

// Request 一次微信 API 请求。用 Client.Request 创建，链式设置参数后
// 用 Get、Post 解码 JSON 响应，或用 GetRaw、PostRaw 取原始响应。
//
// 所有终结方法共用同一条执行路径：附加 access_token、解析 errcode、
// token 失效时刷新并重试一次、限制响应体大小、记录日志。
type Request struct {
	client       *Client
	path         string
	query        url.Values
	jsonBody     any
	hasJSONBody  bool
	files        []formFile
	fields       []formField
	withoutToken bool
}

type formFile struct {
	field, name string
	content     []byte
}

type formField struct {
	key, value string
}

// RawResponse 未解码的成功响应。errcode 非 0 时终结方法返回 *WechatError，不会返回 RawResponse。
type RawResponse struct {
	ContentType string
	Body        []byte
}

func (r *Request) Query(key, value string) *Request {
	if r.query == nil {
		r.query = url.Values{}
	}
	r.query.Set(key, value)
	return r
}

// Body 设置 JSON 请求体。
func (r *Request) Body(v any) *Request {
	r.jsonBody = v
	r.hasJSONBody = true
	return r
}

// File 添加 multipart 文件。与 Body 互斥。
func (r *Request) File(field, fileName string, content []byte) *Request {
	r.files = append(r.files, formFile{field: field, name: fileName, content: content})
	return r
}

// Field 添加 multipart 普通字段。与 Body 互斥。
func (r *Request) Field(key, value string) *Request {
	r.fields = append(r.fields, formField{key: key, value: value})
	return r
}

// WithoutToken 不附加 access_token，用于获取 token、code2session 等接口。
func (r *Request) WithoutToken() *Request {
	r.withoutToken = true
	return r
}

func (r *Request) Get[T any](ctx context.Context) (T, error) {
	resp, err := r.do(ctx, http.MethodGet)
	return decodeJSON[T](r.path, resp, err)
}

func (r *Request) Post[T any](ctx context.Context) (T, error) {
	resp, err := r.do(ctx, http.MethodPost)
	return decodeJSON[T](r.path, resp, err)
}

// GetRaw 返回原始响应，用于成功时不返回 JSON 的接口。
func (r *Request) GetRaw(ctx context.Context) (RawResponse, error) {
	resp, err := r.do(ctx, http.MethodGet)
	return RawResponse{ContentType: resp.contentType, Body: resp.body}, err
}

// PostRaw 返回原始响应，用于小程序码等成功时返回图片、失败时返回 JSON 的接口。
func (r *Request) PostRaw(ctx context.Context) (RawResponse, error) {
	resp, err := r.do(ctx, http.MethodPost)
	return RawResponse{ContentType: resp.contentType, Body: resp.body}, err
}

type response struct {
	status      int
	contentType string
	body        []byte
}

func decodeJSON[T any](path string, resp response, err error) (T, error) {
	var out T
	if err != nil {
		return out, err
	}
	if len(bytes.TrimSpace(resp.body)) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(resp.body, &out); err != nil {
		return out, fmt.Errorf("decode %s response: %w", path, err)
	}
	return out, nil
}

func (r *Request) usesToken() bool {
	return !r.withoutToken && r.client.token != nil
}

// do 执行请求。access_token 失效（40001/42001）时刷新并重试一次，
// 用于缓存中的 token 已被其他服务作废的情况。请求体已缓冲在内存中，上传也可以重试。
func (r *Request) do(ctx context.Context, method string) (response, error) {
	body, contentType, err := r.encode()
	if err != nil {
		return response{}, fmt.Errorf("encode %s request: %w", r.path, err)
	}

	resp, err := r.send(ctx, method, body, contentType)
	if !IsTokenError(err) || !r.usesToken() {
		return resp, err
	}

	r.client.logger.LogAttrs(ctx, slog.LevelWarn, "wechat access token rejected, refreshing",
		slog.String("path", r.path), slog.Any("error", err))
	if _, refreshErr := r.client.token.RefreshToken(ctx); refreshErr != nil {
		return resp, errors.Join(err, fmt.Errorf("refresh access token: %w", refreshErr))
	}
	return r.send(ctx, method, body, contentType)
}

func (r *Request) encode() ([]byte, string, error) {
	if len(r.files) > 0 || len(r.fields) > 0 {
		if r.hasJSONBody {
			return nil, "", errors.New("json body and multipart form are mutually exclusive")
		}
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		for _, file := range r.files {
			part, err := writer.CreateFormFile(file.field, file.name)
			if err != nil {
				return nil, "", err
			}
			if _, err := part.Write(file.content); err != nil {
				return nil, "", err
			}
		}
		for _, field := range r.fields {
			if err := writer.WriteField(field.key, field.value); err != nil {
				return nil, "", err
			}
		}
		if err := writer.Close(); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), writer.FormDataContentType(), nil
	}

	if !r.hasJSONBody {
		return nil, "", nil
	}
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	// 微信接口按原样展示消息内容，不转义 <、>、&。
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(r.jsonBody); err != nil {
		return nil, "", err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), "application/json", nil
}

func (r *Request) send(ctx context.Context, method string, body []byte, contentType string) (response, error) {
	query := maps.Clone(r.query)
	if r.usesToken() {
		token, err := r.client.token.GetToken(ctx)
		if err != nil {
			return response{}, fmt.Errorf("get access token: %w", err)
		}
		if query == nil {
			query = url.Values{}
		}
		query.Set("access_token", token)
	}
	rawURL := r.client.baseURL + r.path
	if len(query) > 0 {
		rawURL += "?" + query.Encode()
	}

	var bodyReader io.Reader = http.NoBody
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, bodyReader)
	if err != nil {
		return response{}, fmt.Errorf("create %s request: %w", r.path, redactURLError(err))
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	start := time.Now()
	resp, err := r.receive(req)
	if err == nil {
		err = resp.check(r.path)
	}
	r.client.logAttempt(ctx, method, r.path, body, resp, time.Since(start), err)
	return resp, err
}

func (r *Request) receive(req *http.Request) (response, error) {
	httpResp, err := r.client.http.Do(req)
	if err != nil {
		// *url.Error 带完整 URL，其中可能有 access_token、secret 等参数。
		return response{}, redactURLError(err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(httpResp.Body, maxResponseBytes+1))
	if err != nil {
		return response{}, fmt.Errorf("read %s response: %w", r.path, err)
	}
	if len(body) > maxResponseBytes {
		return response{}, fmt.Errorf("%s response exceeds %d bytes", r.path, maxResponseBytes)
	}
	return response{
		status:      httpResp.StatusCode,
		contentType: httpResp.Header.Get("Content-Type"),
		body:        body,
	}, nil
}

// check 优先返回微信的 errcode，其次检查 HTTP 状态码。
func (resp response) check(path string) error {
	if wechatErr := parseWechatError(path, resp.body); wechatErr != nil {
		return wechatErr
	}
	if resp.status < 200 || resp.status >= 300 {
		return fmt.Errorf("%s: http status %d: %s", path, resp.status, truncate(resp.body, 256))
	}
	return nil
}

// parseWechatError 只解析 JSON 对象，图片等二进制响应直接跳过。
func parseWechatError(path string, body []byte) *WechatError {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil
	}
	var envelope struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(trimmed, &envelope); err != nil || envelope.ErrCode == 0 {
		return nil
	}
	return &WechatError{Path: path, ErrCode: envelope.ErrCode, ErrMsg: envelope.ErrMsg}
}

func truncate(body []byte, limit int) string {
	if len(body) <= limit {
		return string(body)
	}
	return string(body[:limit]) + "..."
}
