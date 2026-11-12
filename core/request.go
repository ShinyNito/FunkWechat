package core

import (
	"bytes"
	"context"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Request 一次微信 API 请求。用 App.Request 创建，链式设置参数后
// 用 Get、Post 解码 JSON 响应，或用 GetRaw、PostRaw 取原始响应。
//
// Get、Post 的类型参数 T 是嵌入了 Status 的结构体，例如：
//
//	type UserInfo struct {
//		core.Status
//		Nickname string `json:"nickname"`
//	}
//
// 接口没有业务字段时直接用 Post[core.Status]。这个要求由编译器检查。
//
// 所有终结方法共用同一条执行路径：附加 access_token、解析 errcode、
// token 失效时刷新并重试一次、限制响应体大小、记录日志。
type Request struct {
	client       *client
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

// Get 发送 GET 请求并把响应解码为 T。T 必须是嵌入了 Status 的结构体。
func (r *Request) Get[T any, PT Response[T]](ctx context.Context) (T, error) {
	return doJSON[T, PT](ctx, r, http.MethodGet)
}

// Post 发送 POST 请求并把响应解码为 T。T 必须是嵌入了 Status 的结构体。
func (r *Request) Post[T any, PT Response[T]](ctx context.Context) (T, error) {
	return doJSON[T, PT](ctx, r, http.MethodPost)
}

// GetRaw 返回原始响应，用于成功时不返回 JSON 的接口。
func (r *Request) GetRaw(ctx context.Context) (RawResponse, error) {
	resp, err := r.do(ctx, http.MethodGet, r.checkRaw)
	return RawResponse{ContentType: resp.contentType, Body: resp.body}, err
}

// PostRaw 返回原始响应，用于小程序码等成功时返回图片、失败时返回 JSON 的接口。
func (r *Request) PostRaw(ctx context.Context) (RawResponse, error) {
	resp, err := r.do(ctx, http.MethodPost, r.checkRaw)
	return RawResponse{ContentType: resp.contentType, Body: resp.body}, err
}

type response struct {
	status      int
	contentType string
	body        []byte
}

func (resp response) ok() bool {
	return resp.status >= 200 && resp.status < 300
}

func (resp response) httpError(path string) *HTTPError {
	return &HTTPError{Path: path, StatusCode: resp.status, Body: truncate(resp.body, 256)}
}

func doJSON[T any, PT Response[T]](ctx context.Context, r *Request, method string) (T, error) {
	var out T
	_, err := r.do(ctx, method, func(resp response) error {
		out = *new(T) // 重试时丢弃上一次的结果
		return decodeResponse(r.path, resp, PT(&out))
	})
	if err != nil {
		return *new(T), err
	}
	return out, nil
}

// decodeResponse 把响应体解码到 out，只解析一遍：errcode、errmsg 落在 out 嵌入的 Status 里。
// 依次检查 errcode、HTTP 状态码和解码错误。
func decodeResponse[T any, PT Response[T]](path string, resp response, out PT) error {
	var decodeErr error
	if len(bytes.TrimSpace(resp.body)) > 0 {
		decodeErr = json.Unmarshal(resp.body, out)
	}
	// 字段类型不匹配时解码器会继续处理其余成员，所以即使有解码错误，errcode 也已经取到。
	if status := out.wechatStatus(); status != nil && status.ErrCode != 0 {
		return &WechatError{Path: path, ErrCode: status.ErrCode, ErrMsg: status.ErrMsg}
	}
	if !resp.ok() {
		return resp.httpError(path)
	}
	if decodeErr != nil {
		return fmt.Errorf("decode %s response: %w", path, decodeErr)
	}
	return nil
}

func (r *Request) usesToken() bool {
	return !r.withoutToken && r.client.token != nil
}

// do 执行请求。access_token 失效（40001/40014/42001）时刷新并重试一次，
// 用于缓存中的 token 已被其他服务作废的情况。请求体已缓冲在内存中，上传也可以重试。
//
// check 检查并解码响应，每次收到响应都会调用一次。
func (r *Request) do(ctx context.Context, method string, check func(response) error) (response, error) {
	body, contentType, err := r.encode()
	if err != nil {
		return response{}, fmt.Errorf("encode %s request: %w", r.path, err)
	}

	resp, err := r.send(ctx, method, body, contentType, check)
	if !IsTokenError(err) || !r.usesToken() {
		return resp, err
	}

	r.client.logger.LogAttrs(ctx, slog.LevelWarn, "wechat access token rejected, refreshing",
		slog.String("path", r.path), slog.Any("error", err))
	if _, refreshErr := r.client.token.Refresh(ctx); refreshErr != nil {
		return resp, errors.Join(err, fmt.Errorf("refresh access token: %w", refreshErr))
	}
	return r.send(ctx, method, body, contentType, check)
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
	body, err := jsonv2.Marshal(r.jsonBody)
	return body, "application/json", err
}

func (r *Request) send(ctx context.Context, method string, body []byte, contentType string, check func(response) error) (response, error) {
	query := maps.Clone(r.query)
	if r.usesToken() {
		token, err := r.client.token.Get(ctx)
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
		err = check(resp)
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

// checkRaw 用于不解码的原始响应：优先返回微信的 errcode，其次检查 HTTP 状态码。
func (r *Request) checkRaw(resp response) error {
	if wechatErr := parseWechatError(r.path, resp.body); wechatErr != nil {
		return wechatErr
	}
	if !resp.ok() {
		return resp.httpError(r.path)
	}
	return nil
}

var errcodeKey = []byte(`"errcode"`)

// parseWechatError 只解析带 errcode 字段的 JSON 对象，图片等二进制响应直接跳过。
func parseWechatError(path string, body []byte) *WechatError {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' || !bytes.Contains(trimmed, errcodeKey) {
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
	// 截断可能落在多字节字符中间，去掉不完整的部分。
	return strings.ToValidUTF8(string(body[:limit]), "") + "..."
}
