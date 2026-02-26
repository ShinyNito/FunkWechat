package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"mime/multipart"
	"net/http"
)

type RawResponse struct {
	StatusCode int
	Body       []byte
}

type RequestBuilder struct {
	client     *Client
	path       string
	query      map[string]string
	body       any
	withToken  bool
	uploadFile io.Reader
	uploadName string
	uploadPart string
	formFields map[string]string
}

func newRequestBuilder(client *Client) *RequestBuilder {
	return &RequestBuilder{
		client:    client,
		withToken: true,
	}
}

func (b *RequestBuilder) Path(path string) *RequestBuilder {
	b.path = path
	return b
}

func (b *RequestBuilder) Query(key, value string) *RequestBuilder {
	if b.query == nil {
		b.query = make(map[string]string)
	}
	b.query[key] = value
	return b
}

func (b *RequestBuilder) QueryMap(query map[string]string) *RequestBuilder {
	if len(query) == 0 {
		return b
	}
	if b.query == nil {
		b.query = make(map[string]string, len(query))
	}
	maps.Copy(b.query, query)
	return b
}

func (b *RequestBuilder) Body(body any) *RequestBuilder {
	b.body = body
	return b
}

func (b *RequestBuilder) WithoutToken() *RequestBuilder {
	b.withToken = false
	return b
}

func (b *RequestBuilder) UploadFile(field, fileName string, r io.Reader) *RequestBuilder {
	b.uploadPart = field
	b.uploadName = fileName
	b.uploadFile = r
	return b
}

func (b *RequestBuilder) UploadField(key, value string) *RequestBuilder {
	if b.formFields == nil {
		b.formFields = make(map[string]string)
	}
	b.formFields[key] = value
	return b
}

func (b *RequestBuilder) Get(ctx context.Context) (RawResponse, error) {
	return b.execute(ctx, http.MethodGet)
}

func (b *RequestBuilder) Post(ctx context.Context) (RawResponse, error) {
	if b.uploadFile != nil {
		return b.executeUpload(ctx)
	}
	return b.execute(ctx, http.MethodPost)
}

func (b *RequestBuilder) buildQuery(ctx context.Context) (map[string]string, error) {
	if !b.withToken || b.client.tokenProvider == nil {
		return b.query, nil
	}

	token, err := b.client.tokenProvider.GetToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}

	params := make(map[string]string, len(b.query)+1)
	if len(b.query) > 0 {
		maps.Copy(params, b.query)
	}
	params["access_token"] = token
	return params, nil
}

func (b *RequestBuilder) execute(ctx context.Context, method string) (RawResponse, error) {
	var zero RawResponse

	query, err := b.buildQuery(ctx)
	if err != nil {
		return zero, err
	}

	rawURL, err := b.client.buildURL(b.path, query)
	if err != nil {
		return zero, fmt.Errorf("build url: %w", err)
	}

	var reqBody []byte
	var bodyReader io.Reader
	if b.body != nil {
		reqBody, err = json.Marshal(b.body)
		if err != nil {
			return zero, fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(reqBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, bodyReader)
	if err != nil {
		return zero, fmt.Errorf("create request: %w", err)
	}
	if len(reqBody) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	b.client.logRequest(ctx, method, rawURL, reqBody)

	resp, err := b.client.httpClient.Do(req)
	if err != nil {
		return zero, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return zero, fmt.Errorf("read response: %w", err)
	}

	b.client.logResponse(ctx, resp.StatusCode, respBody)
	return RawResponse{StatusCode: resp.StatusCode, Body: respBody}, nil
}

func (b *RequestBuilder) executeUpload(ctx context.Context) (RawResponse, error) {
	var zero RawResponse

	query, err := b.buildQuery(ctx)
	if err != nil {
		return zero, err
	}

	rawURL, err := b.client.buildURL(b.path, query)
	if err != nil {
		return zero, fmt.Errorf("build url: %w", err)
	}

	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)

	part, err := writer.CreateFormFile(b.uploadPart, b.uploadName)
	if err != nil {
		return zero, fmt.Errorf("create form file: %w", err)
	}
	if _, err = io.Copy(part, b.uploadFile); err != nil {
		return zero, fmt.Errorf("copy file: %w", err)
	}

	for key, value := range b.formFields {
		if err = writer.WriteField(key, value); err != nil {
			return zero, fmt.Errorf("write field %s: %w", key, err)
		}
	}

	if err = writer.Close(); err != nil {
		return zero, fmt.Errorf("close writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, payload)
	if err != nil {
		return zero, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	b.client.logRequest(ctx, http.MethodPost, rawURL, nil)

	resp, err := b.client.httpClient.Do(req)
	if err != nil {
		return zero, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return zero, fmt.Errorf("read response: %w", err)
	}

	b.client.logResponse(ctx, resp.StatusCode, respBody)
	return RawResponse{StatusCode: resp.StatusCode, Body: respBody}, nil
}
