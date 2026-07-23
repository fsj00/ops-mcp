package netutil

import (
	"testing"
	"time"
)

func TestDecodeDataHex(t *testing.T) {
	b, err := DecodeData("0102030a")
	if err != nil {
		t.Fatal(err)
	}
	if BytesToHex(b) != "0102030a" {
		t.Fatalf("hex=%s", BytesToHex(b))
	}
	b, err = DecodeData("01:02 03-0A")
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 4 || b[3] != 0x0a {
		t.Fatalf("got %v", b)
	}
}

func TestDecodeDataBytesArray(t *testing.T) {
	b, err := DecodeData([]interface{}{1.0, 2.0, 3.0, 10.0})
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{1, 2, 3, 10}
	if BytesToHex(b) != BytesToHex(want) {
		t.Fatalf("got %v want %v", b, want)
	}
}

func TestDecodeDataRejects(t *testing.T) {
	cases := []interface{}{nil, "", "abc", "010", []interface{}{}, []interface{}{256}}
	for _, c := range cases {
		if _, err := DecodeData(c); err == nil {
			t.Fatalf("expected error for %#v", c)
		}
	}
}

func TestValidateTarget(t *testing.T) {
	if _, _, err := ValidateTarget("", 80); err == nil {
		t.Fatal("empty ip")
	}
	if _, _, err := ValidateTarget("127.0.0.1", 0); err == nil {
		t.Fatal("bad port")
	}
	h, p, err := ValidateTarget("  example.com ", 443)
	if err != nil || h != "example.com" || p != 443 {
		t.Fatalf("got %q %d %v", h, p, err)
	}
}

func TestResolveTimeout(t *testing.T) {
	d, err := ResolveTimeout(time.Time{}, "2s", DefaultTimeout)
	if err != nil || d != 2*time.Second {
		t.Fatalf("got %v %v", d, err)
	}
	deadline := time.Now().Add(500 * time.Millisecond)
	d, err = ResolveTimeout(deadline, "5s", DefaultTimeout)
	if err != nil || d > 500*time.Millisecond {
		t.Fatalf("expected clamp, got %v %v", d, err)
	}
}

func TestResolveMaxResponseBytes(t *testing.T) {
	n, err := ResolveMaxResponseBytes(0)
	if err != nil || n != DefaultMaxResponseBytes {
		t.Fatalf("got %d %v", n, err)
	}
	if _, err := ResolveMaxResponseBytes(AbsoluteMaxResponseBytes + 1); err == nil {
		t.Fatal("expected hard cap error")
	}
}
