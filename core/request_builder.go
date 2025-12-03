package core

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
)

// RequestBuilder 请求构建器
type RequestBuilder struct {
	client               *Client
	path                 string
	query                map[string]string
	body                 any
	shouldAddAccessToken bool
	method               string

	// 文件上传相关
	uploadFile        io.Reader
	uploadFieldName   string
	uploadFileName    string
	uploadExtraFields map[string]string
}

// newRequestBuilder 创建请求构建器（包内使用）
func newRequestBuilder(client *Client) *RequestBuilder {
	return &RequestBuilder{
		client:               client,
		query:                make(map[string]string),
		shouldAddAccessToken: true, // 默认添加 access_token
	}
}

// Path 设置请求路径
func (b *RequestBuilder) Path(path string) *RequestBuilder {
	b.path = path
	return b
}

// Query 添加单个查询参数
func (b *RequestBuilder) Query(key, value string) *RequestBuilder {
	if b.query == nil {
		b.query = make(map[string]string)
	}
	b.query[key] = value
	return b
}

// QueryMap 批量设置查询参数
func (b *RequestBuilder) QueryMap(query map[string]string) *RequestBuilder {
	if b.query == nil {
		b.query = make(map[string]string)
	}
	for k, v := range query {
		b.query[k] = v
	}
	return b
}

// Body 设置请求体
func (b *RequestBuilder) Body(body any) *RequestBuilder {
	b.body = body
	return b
}

// WithoutToken 不添加 access_token
func (b *RequestBuilder) WithoutToken() *RequestBuilder {
	b.shouldAddAccessToken = false
	return b
}

// WithToken 添加 access_token（默认行为）
func (b *RequestBuilder) WithToken() *RequestBuilder {
	b.shouldAddAccessToken = true
	return b
}

// UploadFile 设置文件上传参数
// fieldName: 表单字段名
// fileName: 文件名
// fileReader: 文件内容
func (b *RequestBuilder) UploadFile(fieldName, fileName string, fileReader io.Reader) *RequestBuilder {
	b.uploadFile = fileReader
	b.uploadFieldName = fieldName
	b.uploadFileName = fileName
	return b
}

// UploadExtraFields 设置上传时的额外表单字段
func (b *RequestBuilder) UploadExtraFields(fields map[string]string) *RequestBuilder {
	b.uploadExtraFields = fields
	return b
}

// Get 执行 GET 请求
func (b *RequestBuilder) Get(ctx context.Context) ([]byte, error) {
	b.method = http.MethodGet
	return b.do(ctx)
}

// Post 执行 POST 请求
func (b *RequestBuilder) Post(ctx context.Context) ([]byte, error) {
	b.method = http.MethodPost

	// 如果有上传文件，使用 multipart 上传
	if b.uploadFile != nil {
		return b.doUpload(ctx)
	}

	return b.do(ctx)
}

// do 执行普通请求
func (b *RequestBuilder) do(ctx context.Context) ([]byte, error) {
	// 构建参数
	params, err := b.client.buildParams(ctx, b.query, b.shouldAddAccessToken)
	if err != nil {
		return nil, err
	}

	// 执行请求
	return b.client.doRequest(ctx, b.method, b.path, params, b.body)
}

// doUpload 执行文件上传
func (b *RequestBuilder) doUpload(ctx context.Context) ([]byte, error) {
	// 获取 access_token
	var params map[string]string
	var err error
	if b.shouldAddAccessToken {
		params, err = b.client.buildParams(ctx, b.query, true)
		if err != nil {
			return nil, fmt.Errorf("get access token: %w", err)
		}
	} else {
		params = b.query
	}

	// 构建 URL
	reqURL, err := b.client.buildURL(b.path, params)
	if err != nil {
		return nil, fmt.Errorf("build url: %w", err)
	}

	// 构建 multipart body
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// 添加文件字段
	part, err := writer.CreateFormFile(b.uploadFieldName, b.uploadFileName)
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, b.uploadFile); err != nil {
		return nil, fmt.Errorf("copy file: %w", err)
	}

	// 添加额外字段
	for key, value := range b.uploadExtraFields {
		if err = writer.WriteField(key, value); err != nil {
			return nil, fmt.Errorf("write field %s: %w", key, err)
		}
	}

	if err = writer.Close(); err != nil {
		return nil, fmt.Errorf("close writer: %w", err)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	b.client.logger.Debug("upload request",
		slog.String("method", http.MethodPost),
		slog.String("url", reqURL),
		slog.String("filename", b.uploadFileName),
	)

	// 发送请求
	resp, err := b.client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	b.client.logger.DebugContext(ctx, "upload response",
		slog.Int("status", resp.StatusCode),
		slog.String("body", string(respBody)),
	)

	return respBody, nil
}
