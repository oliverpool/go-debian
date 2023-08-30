package ar

import "io"

// NewReader reads an ar archive.
// If r is also an [io.ReaderAt], the [Header] will have the field SectionReader populated.
func NewWriter(w io.Writer) *Writer {
	aw := Writer{
		w: w,
	}
	return &aw
}

type Writer struct {
	w      io.Writer
	offset int64
	// remainingRead int64
	// current       io.Reader

	// err      error
}

func (aw *Writer) WriteHeader(hdr *Header) (int64, error) {
	if aw.offset == 0 {
		_, err := aw.w.Write([]byte(Signature))
		if err != nil {
			return 0, err
		}
		aw.offset += int64(len(Signature))
	} else if aw.offset%2 == 1 {
		_, err := aw.Write([]byte{'\n'})
		if err != nil {
			return 0, err
		}
	}

	head, err := hdr.marshal()
	if err != nil {
		return 0, err
	}
	n, err := aw.Write(head)
	if err != nil {
		return 0, err
	}
	aw.offset += int64(n)

	if hdr.SectionReader != nil {
		n, err := io.Copy(aw.w, hdr.SectionReader)
		aw.offset += n
		return n, err
	}
	return 0, nil
}

func (aw *Writer) Write(b []byte) (int, error) {
	n, err := aw.w.Write(b)
	aw.offset += int64(n)
	return n, err
}
