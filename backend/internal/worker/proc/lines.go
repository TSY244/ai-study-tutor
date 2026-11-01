package proc

// lineWriter bounds incomplete records independently of diagnostic capture.
type lineWriter struct {
	buf      []byte
	dropping bool
	onLine   func([]byte)
}

func (w *lineWriter) Write(p []byte) (int, error) {
	for _, b := range p {
		if b == '\n' {
			if !w.dropping {
				w.onLine(append([]byte(nil), w.buf...))
			}
			w.buf = w.buf[:0]
			w.dropping = false
		} else if !w.dropping {
			if len(w.buf) >= 1024 {
				w.buf = w.buf[:0]
				w.dropping = true
			} else {
				w.buf = append(w.buf, b)
			}
		}
	}
	return len(p), nil
}
