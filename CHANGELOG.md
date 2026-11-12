# Changelog

All notable changes to this project will be documented in this file.

The format is based on Keep a Changelog and this project follows Semantic Versioning.

## [3.1.0] - 2026-11-12

### Changed
- JSON request bodies now use `encoding/json/v2.Marshal` with its default semantics. Object keys are no longer sorted, nil maps encode as `{}`, nil slices encode as `[]`, and `omitempty` follows JSON-value emptiness. Code that depends on v1-specific request serialization should account for these changes.

### Tests
- Added a request-encoding benchmark with a five-field subscribe message.
- Added coverage for JSON v2 defaults, including nil collections, zero-valued numeric fields, omitted empty strings, and literal HTML characters.

## [3.0.0] - 2026-11-10

The module path is now `github.com/ShinyNito/FunkWechat/v3`. This release reshapes the API without compatibility shims.

### Changed
- Requests use generic terminal methods: `client.Request(path).Body(...).Post[T](ctx)`. `TypedRequest`, `core.NewTypedRequest`, and the `Request[T]` helpers in `miniprogram` and `officialaccount` are removed.
- All requests share one execution path. JSON and multipart bodies are buffered, so uploads are also retried once when the access token is rejected. Multipart requests use `File(field, name, content []byte)` and `Field(key, value)`.
- Response types must embed `core.Status`, which carries `errcode` and `errmsg`. `Get[T]` and `Post[T]` decode the body once into `T` and read the errcode from the embedded field; a type without `core.Status` does not compile. Use `Post[core.Status]` for endpoints with no payload. `Code2SessionResponse` and `GetPhoneNumberResponse` now embed `core.Status`.
- Added `GetRaw` and `PostRaw` for endpoints that return binary content on success, such as mini program codes. They still return `*core.WechatError` when the response carries a non-zero `errcode`.
- `miniprogram.New` and `officialaccount.New` take a `core.AppConfig`. Both clients embed `*core.App`, which provides `Request`, `Config`, `Logger`, and `AccessToken`. `core.Client`, `core.NewClient`, `core.NewAppClient`, and the per-package `Config` types are removed.
- Access tokens and tickets are managed by an internal credential type that keeps the value in process memory. An unexpired credential is returned with one atomic read. The fetch runs with `context.WithoutCancel` and a 30-second timeout, and a canceled caller no longer affects other waiters. `core.TokenManager` and `core.AccessTokenProvider` are removed.
- `AppConfig.Cache` is optional and is no longer read on every request: it is read at startup and when a credential expires, and values read from it are rechecked at most once a minute. `Cache.Get` now returns `(value, ok, err)`, and `Cache.Delete` is removed. A failing cache is logged at warn level and the credential is fetched from WeChat.
- `officialaccount.Client.GetTicket` takes a `TicketType` directly and returns an error for unsupported types. `RefreshTicket` always fetches a new ticket.
- Cache keys are now `funkwechat:access_token:<appid>` and `funkwechat:ticket:<appid>:<type>`.
- `core/utils` is renamed to `wxcrypto`. `RandomString` no longer returns an error, `SHA1Sign` and `SHA256Sign` no longer sort the caller's slice, and the JS-SDK signature is available as `wxcrypto.SignJSSDK`.
- `*core.WechatError` includes the request `Path` and matches by errcode with `errors.Is`, for example `errors.Is(err, core.ErrUserRefused)`. The `core.ErrCode*` integer constants are replaced by `core.Err*` values, and `NewWechatError` is removed.
- Non-2xx responses without an errcode return `*core.HTTPError`. Validation failures wrap `core.ErrInvalidArgument`.
- Added `core.IsRetryable(ctx, err)` for errcode `-1`, HTTP 429 and 5xx, and network timeouts. It returns false once the caller's context is canceled or past its deadline, since a caller deadline and an HTTP client timeout produce the same error. `core.IsTokenError` also covers `40014`.
- The default HTTP client keeps up to 32 idle connections per host. Responses larger than 10 MB are rejected.

### Removed
- `core.RedactQueryMap`, `core.RedactURLQuery`, `core.RedactJSONBody`, `core.DecodeWechat`, `core.StableTokenFetcher`, `core.MemoryCache`, and `core.NewMemoryCache`.
- The testify test dependency. The module has no third-party dependencies.

### Security
- Network errors no longer include the request URL's `access_token`, `secret`, or `js_code`.
- Debug logs also redact personal data: `openid`, `unionid`, `touser`, `phoneNumber`, `purePhoneNumber`, `encryptedData`, `iv`, and `ticket`.

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
