# Changelog

All notable changes to this project will be documented in this file.

The format is based on Keep a Changelog and this project follows Semantic Versioning.

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
