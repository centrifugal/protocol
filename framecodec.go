package protocol

import (
	"bytes"
	"compress/flate"
	"errors"
	"fmt"
	"io"
	"math"
	"sync"
)

// Once a connection has negotiated frame compression, every frame starts with
// one of these markers.
//
// The marker deliberately does not name a codec. Which codec a connection uses
// is settled once, at connect, by the capability flags the client advertised, so
// repeating it per frame would be redundant - and would make every new codec a
// protocol change. The marker only answers the one question that genuinely
// varies frame to frame: was this frame compressed at all.
const (
	// FrameCodecRaw marks a frame whose payload follows verbatim. Every receiver
	// must support it: a sender falls back to it whenever compression would not
	// shrink a frame, or has been measured not to pay off for a connection.
	FrameCodecRaw byte = 0x00
	// FrameCodecCompressed marks a frame compressed with whatever codec this
	// connection negotiated, against its active dictionary.
	FrameCodecCompressed byte = 0x01
)

// MinDictionaryCompressionLevel is the lowest level NewDeflateFrameCodec
// accepts.
//
// DEFLATE implementations switch to specialised fast encoders below this which
// ignore the preset dictionary completely while still accepting it. The
// standard library does this at level 1: measured on a 61 byte frame against a
// 4KB dictionary it emits 66 bytes, versus 24 bytes at level 2 and above and 60
// bytes with no dictionary at all. A level below this does not trade ratio for
// speed, it silently disables the entire feature while still paying to carry
// the dictionary around - so NewDeflateFrameCodec panics rather than accept
// one.
//
// TestFrameCodecDictionaryActuallyApplies guards this.
const MinDictionaryCompressionLevel = 2

var (
	// ErrFrameTooLarge is returned when a compressed frame expands beyond the
	// caller supplied limit. Without it a small frame could be inflated into an
	// arbitrary amount of memory.
	ErrFrameTooLarge = errors.New("centrifugal: decompressed frame too large")
	// ErrUnknownFrameCodec is returned for a frame marker the codec does not know.
	ErrUnknownFrameCodec = errors.New("centrifugal: unknown frame codec")
	// ErrEmptyFrame is returned for a frame with no codec marker at all.
	ErrEmptyFrame = errors.New("centrifugal: empty frame")
)

// DeflateFrameCodec compresses and decompresses whole transport frames with
// DEFLATE against a shared preset dictionary, applying the frame marker
// convention above.
//
// It lives in this package because a server and a client have to agree on it
// byte for byte, and this package is the only thing they both import - putting
// it anywhere else would let the two sides drift. It is not an abstraction over
// codecs: it is the one codec the protocol ships with. A second algorithm would
// be a sibling type emitting the same markers, not an implementation of an
// interface here, since which codec a connection uses is settled by capability
// flags at connect rather than by anything on the wire.
//
// One codec is meant to be shared by every connection using the same dictionary,
// and it is safe for concurrent use. This matters: a DEFLATE writer retains
// several hundred kilobytes of window and hash state, so giving each connection
// its own would dwarf the rest of a connection's footprint. Pooling them behind
// a shared codec keeps that cost proportional to the number of concurrent writes
// instead of the number of connections.
type DeflateFrameCodec struct {
	id    string
	dict  []byte
	level int
	wPool sync.Pool
	rPool sync.Pool
}

// NewDeflateFrameCodec builds a codec for the given dictionary. id identifies
// the dictionary content so both sides can tell which one a frame was built
// with.
//
// level is a DEFLATE compression level from MinDictionaryCompressionLevel to
// 9. It affects only how hard this side's encoder works to shrink a frame,
// and never needs to match what the other side of a connection uses: DEFLATE's
// format is self-describing per block, so a decoder never depends on which
// level produced its input, only on sharing the same dictionary.
// NewDeflateFrameCodec panics if level is below MinDictionaryCompressionLevel,
// since that silently disables the dictionary rather than trading ratio for
// speed.
//
// This package does not recommend one level for every caller, because the
// right choice depends on traffic and dictionary shapes only the caller sees,
// and it has already moved once as compress/flate itself changed. Measured on
// go1.26.5 and go1.27.0, frames drawn from the vocabulary a dictionary is
// built from, against a few KB dictionary:
//
//   - go1.26: levels 2-9 are byte-identical in output and cost, because a
//     per-frame compression there is dominated by loading the dictionary on
//     Reset, not by encoding the frame - the level only governs the smaller
//     half of the work.
//   - go1.27 rewrote compress/flate's levels 2-6 into new encoders optimised
//     for large-payload throughput (golang/go#75532), at the cost of ratio for
//     a small payload against a large preset dictionary - exactly this
//     codec's shape. A 40 byte frame against a 2.4KB dictionary compresses to
//     47 bytes at level 6 on go1.27 (worse than sending it raw) versus 34
//     bytes on go1.26. Levels 7-9 kept the old algorithm, so they are
//     unaffected: 31 bytes on either Go version, at a CPU cost within a few
//     percent of what level 6 cost on go1.26 - i.e. level 7 on go1.27 tracks
//     the historical level-6 cost/ratio, where level 6 on go1.27 does not.
//   - Levels 7, 8 and 9 produce byte-identical output for this shape, so 9
//     buys nothing over 7 and only costs more CPU for a caller with this shape
//     of traffic.
func NewDeflateFrameCodec(id string, dict []byte, level int) *DeflateFrameCodec {
	if level < MinDictionaryCompressionLevel {
		panic(fmt.Sprintf("centrifugal/protocol: dictionary compression level %d is below MinDictionaryCompressionLevel (%d): "+
			"levels below this silently ignore the preset dictionary instead of trading ratio for speed",
			level, MinDictionaryCompressionLevel))
	}
	c := &DeflateFrameCodec{id: id, dict: dict, level: level}
	c.wPool.New = func() any {
		w, _ := flate.NewWriterDict(io.Discard, c.level, c.dict)
		return w
	}
	c.rPool.New = func() any {
		return flate.NewReaderDict(bytes.NewReader(nil), c.dict)
	}
	return c
}

// ID returns the dictionary identifier this codec was built for.
func (c *DeflateFrameCodec) ID() string { return c.id }

// Dict returns the raw dictionary bytes. The result must not be modified.
func (c *DeflateFrameCodec) Dict() []byte { return c.dict }

// Compress encodes src into a framed payload appended to dst.
//
// It falls back to FrameCodecRaw whenever compression does not actually shrink
// the frame, so an incompressible payload - already compressed data, encrypted
// blobs - costs one marker byte instead of growing.
func (c *DeflateFrameCodec) Compress(dst, src []byte) []byte {
	w := c.wPool.Get().(*flate.Writer)
	buf := bytes.NewBuffer(make([]byte, 0, len(src)/2+16))
	w.Reset(buf) // Reset keeps the dictionary passed to NewWriterDict.
	_, err := w.Write(src)
	if err == nil {
		err = w.Close()
	}
	c.wPool.Put(w)
	if err != nil || buf.Len() >= len(src) {
		dst = append(dst, FrameCodecRaw)
		return append(dst, src...)
	}
	dst = append(dst, FrameCodecCompressed)
	return append(dst, buf.Bytes()...)
}

// Decompress decodes a framed payload produced by Compress, appending the result
// to dst. maxSize bounds the decompressed output, pass 0 to leave it unbounded.
func (c *DeflateFrameCodec) Decompress(dst, frame []byte, maxSize int) ([]byte, error) {
	if len(frame) == 0 {
		return nil, ErrEmptyFrame
	}
	switch frame[0] {
	case FrameCodecRaw:
		return append(dst, frame[1:]...), nil
	case FrameCodecCompressed:
	default:
		return nil, ErrUnknownFrameCodec
	}
	r := c.rPool.Get().(io.ReadCloser)
	defer c.rPool.Put(r)
	if err := r.(flate.Resetter).Reset(bytes.NewReader(frame[1:]), c.dict); err != nil {
		return nil, err
	}
	var rd io.Reader = r
	if maxSize > 0 {
		// Read one byte past the limit so an oversized frame is detected rather
		// than silently truncated.
		rd = io.LimitReader(r, decompressBudget(maxSize))
	}
	out := bytes.NewBuffer(dst)
	n, err := out.ReadFrom(rd)
	if err != nil {
		return nil, err
	}
	if maxSize > 0 && int(n) > maxSize {
		return nil, ErrFrameTooLarge
	}
	return out.Bytes(), nil
}

// decompressBudget is how many bytes Decompress may read while decoding one
// frame: one more than maxSize, so that a frame which is over it is detected
// rather than silently truncated. maxSize equal to math.MaxInt would overflow
// int64(maxSize)+1, leaving a negative budget that stops the reader before it
// delivers anything - the same hazard readBudget guards against for stream
// decoders - so it is clamped.
func decompressBudget(maxSize int) int64 {
	if maxSize == math.MaxInt {
		return math.MaxInt64
	}
	return int64(maxSize) + 1
}

// DeflateDictionary compresses dictionary content with raw DEFLATE and no preset
// dictionary, for delivery to a client that has nothing installed yet.
//
// It carries no codec marker: this is content inside a Dictionary message rather
// than a frame, and dictionary content is always deflated, so there is nothing
// to signal.
// A dictionary is a concatenation of real message samples, so it is ordinary
// text and compresses several fold - which is the difference between a first
// dictionary a connection can afford and one it cannot.
//
// level is a plain flate.NewWriter level: unlike NewDeflateFrameCodec's level,
// there is no preset dictionary here for a low level to silently ignore, so
// any valid flate level is fine - MinDictionaryCompressionLevel does not apply.
func DeflateDictionary(dict []byte, level int) []byte {
	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, level)
	if err != nil {
		return nil
	}
	if _, err := w.Write(dict); err != nil {
		return nil
	}
	if err := w.Close(); err != nil {
		return nil
	}
	return buf.Bytes()
}

// InflateDictionary reverses DeflateDictionary, refusing anything that would
// expand past maxSize so a small crafted payload cannot be inflated into an
// unbounded allocation.
func InflateDictionary(data []byte, maxSize int) ([]byte, error) {
	r := flate.NewReader(bytes.NewReader(data))
	defer func() { _ = r.Close() }()
	// decompressBudget avoids the int64(maxSize)+1 overflow when maxSize is
	// math.MaxInt, which would otherwise silently truncate the output to
	// nothing instead of decompressing it.
	out, err := io.ReadAll(io.LimitReader(r, decompressBudget(maxSize)))
	if err != nil {
		return nil, err
	}
	if len(out) > maxSize {
		return nil, errors.New("centrifugal: dictionary too large")
	}
	return out, nil
}
