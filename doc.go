// Package protocol contains client-server protocol definitions of the Centrifugal
// ecosystem ([Centrifugo] server and [Centrifuge] library) together with fast
// encoders and decoders for them.
//
// The protocol is described in [client.proto] and consists of three top-level
// messages:
//
//   - [Command] – sent from a client to a server. It carries exactly one request,
//     for example [ConnectRequest] or [SubscribeRequest].
//   - [Reply] – sent from a server to a client. It either answers a Command with a
//     result (such as [ConnectResult]) or an [Error], or wraps an asynchronous Push.
//   - [Push] – asynchronous server-to-client message, for example [Publication],
//     [Join] or [Leave] in a channel.
//
// # Serialization formats
//
// Every message may be serialized either as JSON or as Protobuf, see [Type].
// Both formats are generated from the same client.proto, so the two are always
// in sync. Which one is used is negotiated by the transport – JSON is the default,
// Protobuf is used by clients which need a more compact binary representation.
//
// Messages of both formats can be streamed one after another inside a single
// transport frame. In JSON messages are separated by a `\n` delimiter, in Protobuf
// every message is prefixed with its length encoded as a varint.
//
// # Encoders and decoders
//
// The package does not expose a single generic Marshal/Unmarshal pair. Instead it
// provides narrow interfaces for the parts of the protocol a server or a client
// needs to touch, each with a JSON and a Protobuf implementation:
//
//   - [CommandEncoder] and [CommandDecoder] (plus [StreamCommandDecoder] for
//     decoding commands from an [io.Reader] with an optional message size limit).
//   - [ReplyEncoder] and [ReplyDecoder].
//   - [PushEncoder] and [ResultEncoder] to encode parts of a Reply separately, so
//     that the same encoded payload can be reused for many connections.
//   - [DataEncoder] to concatenate several already encoded messages into a single
//     transport frame using the framing described above.
//
// Implementations are chosen by protocol [Type], usually through the pooled
// helpers – [GetCommandDecoder]/[PutCommandDecoder], [GetDataEncoder]/[PutDataEncoder],
// [GetStreamCommandDecoderLimited]/[PutStreamCommandDecoder] and [ReplyPool]. These
// helpers reuse objects between messages and let a server avoid allocations on
// hot paths.
//
// # Payloads
//
// Application-specific payloads (such as [Publication] Data) use the [Raw] type.
// Raw is a []byte which is passed through encoding as is, so the payload a
// publisher sent is the payload a subscriber decodes – see [Raw.MarshalJSON] for
// the single exception required by the JSON framing.
//
// # Stability
//
// The protocol itself is backwards compatible: fields are only added, never
// repurposed. The Go API of this package is shaped by the needs of Centrifugo and
// Centrifuge, and while it rarely changes it does not provide the compatibility
// guarantees of a v1 module – pin a version when depending on it directly.
//
// [Centrifugo]: https://github.com/centrifugal/centrifugo
// [Centrifuge]: https://github.com/centrifugal/centrifuge
// [client.proto]: https://github.com/centrifugal/protocol/blob/master/client.proto
package protocol
