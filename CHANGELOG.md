# Changelog

All notable changes to this project will be documented in this file.

The format is based on Keep a Changelog and this project follows Semantic Versioning.

## [3.0.0] - 2026-11-10

The module path is now `github.com/ShinyNito/FunkWechat/v3`. This release reshapes the API without compatibility shims.

### Changed
- Requests use generic terminal methods: `client.Request(path).Body(...).Post[T](ctx)`. `TypedRequest`, `core.NewTypedRequest`, and the `Request[T]` helpers in `miniprogram` and `officialaccount` are removed.
- All requests share one execution path. JSON and multipart bodies are buffered, so uploads are also retried once when the access token is rejected. Multipart requests use `File(field, name, content []byte)` and `Field(key, value)`.
- Added `GetRaw` and `PostRaw` for endpoints that return binary content on success, such as mini program codes. They still return `*core.WechatError` when the response carries a non-zero `errcode`.
- `miniprogram.New` and `officialaccount.New` take a `core.AppConfig`. Both clients embed `*core.App`, which provides `Request`, `Config`, `Logger`, and `AccessToken`. `core.Client`, `core.NewClient`, `core.NewAppClient`, and the per-package `Config` types are removed.
- Access tokens and tickets are managed by an internal credential type. The cache is checked at most twice without holding a lock, the fetch runs with `context.WithoutCancel` and a 30-second timeout, and a canceled caller no longer affects other waiters. `core.TokenManager` and `core.AccessTokenProvider` are removed.
- `officialaccount.Client.GetTicket` takes a `TicketType` directly and returns an error for unsupported types. `RefreshTicket` always fetches a new ticket.
- Cache keys are now `funkwechat:access_token:<appid>` and `funkwechat:ticket:<appid>:<type>`.
- `core/utils` is renamed to `wxcrypto`. `RandomString` no longer returns an error, `SHA1Sign` and `SHA256Sign` no longer sort the caller's slice, and the JS-SDK signature is available as `wxcrypto.SignJSSDK`.
- `*core.WechatError` includes the request `Path`, and `NewWechatError` is removed.
- The default HTTP client keeps up to 32 idle connections per host. Responses larger than 10 MB are rejected.

### Removed
- `core.RedactQueryMap`, `core.RedactURLQuery`, `core.RedactJSONBody`, `core.DecodeWechat`, `core.StableTokenFetcher`, and `MemoryCache.Cleanup`.
- The testify test dependency. The module has no third-party dependencies.

### Security
- Network errors no longer include the request URL's `access_token`, `secret`, or `js_code`.

### Logging
- Each API call is logged as one debug line with `appid`, method, path, status, elapsed time, and redacted bodies. A rejected access token is logged at warn level before the retry.

## [2.2.0] - 2026-11-10

### Changed
- Mini program and official account clients now fetch access tokens from `/cgi-bin/stable_token` in normal mode instead of `/cgi-bin/token`. Tokens fetched by other services or instances are no longer invalidated, and the app secret is sent in the request body instead of the URL.
- Typed requests that fail with `40001` or `42001` refresh the access token and retry once. Uploads and requests built with `WithoutToken` are not retried.
- Requires Go 1.27.

### Added
- `core.StableTokenFetcher` and `core.StableTokenPath`.
- `miniprogram.Client.SendSubscribeMessage` with `SubscribeMessageRequest`, `SubscribeMessageValue`, and `MiniProgramState`.
- `core.ErrCodeUserRefused` (`43101`).
- `core.RedactJSONBody`.

### Security
- Debug logs now redact sensitive fields (`secret`, `access_token`, `session_key`, etc.) in JSON request and response bodies. Non-JSON bodies are logged as their length only.

## [2.1.0] - 2026-02-27

### Changed
- `officialaccount.Client.GetTicket` now returns `string` instead of a response struct.
- `officialaccount.Client.RefreshTicket` now returns `string` instead of a response struct.
- Ticket cache format changed from raw string to JSON payload with `ticket` and `expires_at`.
- Invalid or expired ticket cache payloads are treated as cache miss and will trigger refresh.

### Security
- Signature verification now uses constant-time comparison in:
  - `core/utils.VerifySignature`
  - `core/utils.VerifyMsgSignature`

### Tests
- Updated official account tests for new ticket API behavior.
- Added test to ensure invalid legacy cached ticket values trigger refresh.

### Docs
- Updated README official account example to print ticket string directly.

### Breaking Changes
- The `GetTicket` and `RefreshTicket` method return types changed. Call sites must be updated.
