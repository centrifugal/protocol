module github.com/centrifugal/protocol/internal/cfjsoncmp

go 1.26.0

require (
	github.com/centrifugal/protocol v0.0.0
	github.com/mailru/easyjson v0.7.7
	github.com/segmentio/encoding v0.5.4
	github.com/valyala/bytebufferpool v1.0.0
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/josharian/intern v1.0.0 // indirect
	github.com/segmentio/asm v1.1.4 // indirect
	golang.org/x/sys v0.21.0 // indirect
)

replace github.com/centrifugal/protocol => ../..
