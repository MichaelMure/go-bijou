<div align="center">
  <h1 align="center">go-bijou</h1>

  <p>
    <a href="https://github.com/MichaelMure/go-bijou/tags">
        <img alt="GitHub Tag" src="https://img.shields.io/github/v/tag/MichaelMure/go-bijou">
    </a>
    <a href="https://github.com/MichaelMure/go-bijou/actions?query=">
      <img src="https://github.com/MichaelMure/go-bijou/actions/workflows/gotest.yml/badge.svg" alt="Build Status">
    </a>
    <a href="https://github.com/MichaelMure/go-bijou/blob/master/LICENSE">
        <img alt="MIT License" src="https://img.shields.io/badge/License-MIT-green">
    </a>
  </p>
</div>

Go implementation of [bijou64](https://github.com/inkandswitch/bijou/blob/main/bijou64/SPEC.md) — a bijective variable-length encoding for unsigned 64-bit integers. Each value has exactly one valid encoding (canonical by construction), so no runtime canonicality check is ever needed. The first byte alone determines the encoded length, enabling O(1) skipping and streaming without continuation-bit scanning.

Developed by [Ink & Switch](https://www.inkandswitch.com/tangents/bijou64/) for their Subduction CRDT sync protocol. Original Rust implementation: [inkandswitch/bijou](https://github.com/inkandswitch/bijou).

## Encoding size

| Value range                                        | Bytes |
|----------------------------------------------------|-------|
| 0 – 247                                            | 1     |
| 248 – 503                                          | 2     |
| 504 – 66,039                                       | 3     |
| 66,040 – 16,843,255                                | 4     |
| 16,843,256 – 4,311,810,551                         | 5     |
| 4,311,810,552 – 1,103,823,438,327                  | 6     |
| 1,103,823,438,328 – 282,578,800,148,983            | 7     |
| 282,578,800,148,984 – 72,340,172,838,076,919       | 8     |
| 72,340,172,838,076,920 – 18,446,744,073,709,551,615| 9     |

## Install

```
go get github.com/MichaelMure/go-bijou
```

## Usage

```go
import bijou "github.com/MichaelMure/go-bijou"

// Encode
encoded := bijou.EncodeU64(12345)
buf = bijou.AppendU64(buf, 12345)   // append to existing slice
n := bijou.EncodedLen(12345)        // bytes needed, without allocating

// Decode from a []byte
v, consumed, err := bijou.DecodeBytes(buf)

// Decode from an io.ByteReader
v, err := bijou.DecodeU64(r)
```

Errors: `bijou.ErrBufferTooShort`, `bijou.ErrOverflow`.

## License

MIT