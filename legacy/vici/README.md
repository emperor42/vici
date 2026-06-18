# VICI (I Conquer)

**VICI** is a lightweight, zero-dependency Go middleware for managing user context, anonymous sessions, and authentication. It is designed to work seamlessly with `net/http`'s standard `FileServer` and can be layered with other middlewares.

## Features

- **Anonymous Tracking**: Automatically generates and manages session IDs via secure cookies.
- **Authentication**: Supports token validation via HTTP Headers or URL Query Parameters.
- **Context Propagation**: Injects user identity and metadata into `context.Context` for downstream handlers.
- **Zero Dependencies**: Built entirely on the Go Standard Library (`crypto/rand`, `net/http`, `encoding/hex`).
- **Modular**: Can be used standalone or combined with VENI and VIDI.

## Installation

```bash
go get github.com/Emperor42/vici