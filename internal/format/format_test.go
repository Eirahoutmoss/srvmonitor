package format

import "testing"

func TestBytes(t *testing.T) {
	cases := map[uint64]string{
		0:          "0 B",
		512:        "512 B",
		1024:       "1.0 KiB",
		1536:       "1.5 KiB",
		1048576:    "1.0 MiB",
		1073741824: "1.0 GiB",
	}
	for in, want := range cases {
		if got := Bytes(in); got != want {
			t.Errorf("Bytes(%d)=%q want %q", in, got, want)
		}
	}
}

func TestBytesPerSecNonNegative(t *testing.T) {
	if got := BytesPerSec(-5); got != "0 B/s" {
		t.Errorf("negative throughput = %q", got)
	}
}

func TestDuration(t *testing.T) {
	cases := map[uint64]string{
		30:    "0dk",
		90:    "1dk",
		3600:  "1sa 0dk",
		90000: "1g 1sa 0dk",
	}
	for in, want := range cases {
		if got := Duration(in); got != want {
			t.Errorf("Duration(%d)=%q want %q", in, got, want)
		}
	}
}
