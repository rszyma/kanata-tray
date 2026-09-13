package stripansi

import (
	"bytes"
	"testing"
)

func writeChunks(t *testing.T, chunks []string) string {
	t.Helper()
	var buf bytes.Buffer
	w := StripANSIColorWriter(&buf)
	for _, c := range chunks {
		n, err := w.Write([]byte(c))
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
		if n != len(c) {
			t.Fatalf("short write: got %d want %d", n, len(c))
		}
	}
	return buf.String()
}

func TestStripANSIWhole(t *testing.T) {
	got := writeChunks(t, []string{"\x1b[32mINFO\x1b[0m server started on :8080"})
	want := "INFO server started on :8080"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestStripANSISplitCSI(t *testing.T) {
	got := writeChunks(t, []string{"time=12:00 level=\x1b[3", "4mDEBUG\x1b[0m msg=hi"})
	want := "time=12:00 level=DEBUG msg=hi"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestStripANSISplitESC(t *testing.T) {
	got := writeChunks(t, []string{"time=12:00 level=\x1b", "[31mERROR\x1b[0m msg=boom"})
	want := "time=12:00 level=ERROR msg=boom"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestStripANSIByteByByte(t *testing.T) {
	in := "\x1b[38;2;255;120;0mWARN\x1b[0m disk 90% full"
	var chunks []string
	for i := 0; i < len(in); i++ {
		chunks = append(chunks, in[i:i+1])
	}
	want := "WARN disk 90% full"
	if got := writeChunks(t, chunks); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestStripANSIStacked(t *testing.T) {
	got := writeChunks(t, []string{"\x1b[1m\x1b[31mBold Red\x1b[0m plain"})
	want := "Bold Red plain"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestStripANSINonSGRPasses(t *testing.T) {
	// The writer strips SGR color codes only, so erase-line
	// passes through intact.
	got := writeChunks(t, []string{"progress:\x1b[2K done"})
	want := "progress:\x1b[2K done"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
