package stripansi

import "io"

type ANSIColorStripWriter struct {
	target io.Writer
	tail   []byte
}

func StripANSIColorWriter(target io.Writer) io.Writer {
	return &ANSIColorStripWriter{target: target}
}

func (w *ANSIColorStripWriter) Write(p []byte) (int, error) {
	data := p
	if len(w.tail) > 0 {
		buf := make([]byte, 0, len(w.tail)+len(p))
		buf = append(buf, w.tail...)
		buf = append(buf, p...)
		data = buf
		w.tail = w.tail[:0]
	}
	out := make([]byte, 0, len(data))
	i := 0
	for i < len(data) {
		if data[i] == 0x1b && i+1 < len(data) && data[i+1] == '[' {
			j := i + 2
			for j < len(data) && ((data[j] >= '0' && data[j] <= '9') || data[j] == ';') {
				j++
			}
			if j < len(data) && data[j] == 'm' {
				i = j + 1
				continue
			}
			if j == len(data) {
				break
			}
		}
		if data[i] == 0x1b && i == len(data)-1 {
			break
		}
		out = append(out, data[i])
		i++
	}
	if i < len(data) {
		w.tail = append(w.tail, data[i:]...)
	}
	if len(out) > 0 {
		if _, err := w.target.Write(out); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}
