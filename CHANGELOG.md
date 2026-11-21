# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/).

## [0.1.0] - 2026-11-21

### Added

- `hello.NewClient` with `WithAPIKey` (default `HELLO_API_KEY`), `WithBaseURL`, `WithHTTPClient`, `WithTimeout`, `WithMaxRetries`.
- `Ping`, `PhoneNumbers` (`List`, `ListForWABA`), `Messages` (`Send`, `SendText`, `SendTemplate`, `SendMedia`, `MarkAsRead`, `Get`), `Media` (`Upload`, `Get`, `Download`), `Templates` (`List`, `Get`, `Create`, `CreateBulk`, `Edit`, `Sync`, `Delete`, `DeleteByName`) and `WebhookOverride` (`Get`, `Set`, `Remove`, `Reapply`).
- Typed `*APIError` parsing both Meta-style and Hello-style error envelopes, with `IsAuthentication`, `IsNotFound`, `IsRateLimit`.
- Retries with exponential backoff, jitter and `Retry-After` support; sends are retried only with an idempotency key.
- Runnable examples: `send-text`, `send-template`, `upload-and-send-media`.

[0.1.0]: https://github.com/lakshyapocketlab/hello-go/releases/tag/v0.1.0
