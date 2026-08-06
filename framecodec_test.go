package protocol

import (
	"bytes"
	"fmt"
	"testing"
)

func TestFrameCodecRoundTrip(t *testing.T) {
	dict := []byte(`{"push":{"id":,"pub":{"data":{"offset":`)
	c := NewDeflateFrameCodec("v1", dict)
	msg := []byte(`{"push":{"id":7,"pub":{"data":{"price":123.45},"offset":42}}}`)
	fr := c.Compress(nil, msg)
	out, err := c.Decompress(nil, fr, 1<<20)
	if err != nil || !bytes.Equal(out, msg) {
		t.Fatalf("round trip failed: err=%v", err)
	}
	fmt.Printf("raw=%d framed=%d marker=%#x\n", len(msg), len(fr), fr[0])

	// incompressible payload must fall back to raw, not grow
	rnd := make([]byte, 64)
	for i := range rnd {
		rnd[i] = byte(i*7 + i*i*13)
	}
	fr2 := c.Compress(nil, rnd)
	out2, err := c.Decompress(nil, fr2, 1<<20)
	if err != nil || !bytes.Equal(out2, rnd) {
		t.Fatalf("raw fallback round trip failed: err=%v", err)
	}
	fmt.Printf("incompressible: raw=%d framed=%d marker=%#x (overhead %d)\n", len(rnd), len(fr2), fr2[0], len(fr2)-len(rnd))

	// bomb guard
	big := bytes.Repeat([]byte("A"), 100000)
	fr3 := c.Compress(nil, big)
	if _, err := c.Decompress(nil, fr3, 1000); err != ErrFrameTooLarge {
		t.Fatalf("expected ErrFrameTooLarge, got %v", err)
	}
	fmt.Printf("bomb guard OK (%d B compressed -> refused at 1000 B limit)\n", len(fr3))
}

func TestFrameCodecConcurrent(t *testing.T) {
	dict := []byte(`{"push":{"id":,"pub":{"data":`)
	c := NewDeflateFrameCodec("v1", dict)
	done := make(chan bool, 8)
	for g := 0; g < 8; g++ {
		go func(g int) {
			for i := 0; i < 200; i++ {
				m := []byte(fmt.Sprintf(`{"push":{"id":%d,"pub":{"data":{"v":%d}}}}`, g, i))
				out, err := c.Decompress(nil, c.Compress(nil, m), 1<<20)
				if err != nil || !bytes.Equal(out, m) {
					t.Errorf("mismatch: %v", err)
					break
				}
			}
			done <- true
		}(g)
	}
	for i := 0; i < 8; i++ {
		<-done
	}
}

// TestFrameCodecDictionaryActuallyApplies fails if the configured compression
// level silently stops honouring the preset dictionary.
//
// This is not hypothetical: both DEFLATE implementations select fast encoders at
// lower levels which ignore the dictionary entirely while still accepting it, so
// the feature degrades to plain per-message compression with no error anywhere.
// The symptom is a quiet loss of most of the compression ratio, which is easy to
// miss without an explicit assertion.
func TestFrameCodecDictionaryActuallyApplies(t *testing.T) {
	var d bytes.Buffer
	for d.Len() < 4096 {
		d.WriteString(`{"push":{"id":3,"pub":{"data":{"price":100.00},"offset":1}}}`)
	}
	msg := []byte(`{"push":{"id":7,"pub":{"data":{"price":123.45},"offset":42}}}`)

	withDict := NewDeflateFrameCodec("v", d.Bytes()[:4096])
	noDict := NewDeflateFrameCodec("v", nil)

	got := len(withDict.Compress(nil, msg))
	baseline := len(noDict.Compress(nil, msg))
	if got >= baseline {
		t.Fatalf("preset dictionary had no effect: %d B with dictionary vs %d B without "+
			"- the compression level is almost certainly using an encoder that ignores it", got, baseline)
	}
	// The dictionary should be worth a lot, not a rounding error.
	if float64(got) > 0.6*float64(baseline) {
		t.Fatalf("preset dictionary barely helped: %d B vs %d B without", got, baseline)
	}
	t.Logf("dictionary effect: %d B with vs %d B without (%.2fx)", got, baseline, float64(baseline)/float64(got))
}
