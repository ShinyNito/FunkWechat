# Changelog

All notable changes to this project will be documented in this file.

The format is based on Keep a Changelog and this project follows Semantic Versioning.

## [Unreleased]

### Changed
- `miniprogram.Config` and `officialaccount.Config` are now aliases of the new `core.AppConfig`, and both clients are built with the new `core.NewAppClient`. Existing struct literals keep working.
- Official account tickets now use `core.TokenManager`: each ticket type is cached and refreshed independently, concurrent refreshes are merged, and callers waiting on a refresh honor context cancellation.
- Ticket cache entries now store the raw ticket under `officialaccount:ticket:<appid>:<type>`. Entries written by 2.2.0 and earlier under `officialaccount:jsapi_ticket:` are ignored and expire on their own.
- Removed the testify test dependency; the module now has no third-party dependencies.

### Fixed
- `officialaccount.Client.RefreshTicket` now always fetches a new ticket. It previously returned the cached ticket when one was present.

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
