package ar

import (
	"fmt"
	"io"
)

// NewReader reads an ar archive.
// If r is also an [io.ReaderAt], the [Header] will have the field SectionReader populated.
func NewWriter(w io.Writer) *Writer {
	aw := Writer{
		w: w,
	}
	return &aw
}

type Writer struct {
	w         io.Writer
	offset    int64
	unwritten int64
}

// WriteHeader will write the Header.
// If Header.SectionReader is not nil, it will be Seeked to the start and written as file.
func (aw *Writer) WriteHeader(hdr *Header) (int64, error) {
	if aw.unwritten > 0 {
		return 0, fmt.Errorf("previous file was not written completely: %d bytes missing", aw.unwritten)
	}
	if aw.offset == 0 {
		_, err := aw.w.Write([]byte(Signature))
		if err != nil {
			return 0, err
		}
		aw.offset += int64(len(Signature))
	} else if aw.offset%2 == 1 {
		_, err := aw.w.Write([]byte{'\n'})
		if err != nil {
			return 0, err
		}
	}

	head, err := hdr.marshal()
	if err != nil {
		return 0, err
	}
	n, err := aw.w.Write(head)
	if err != nil {
		return 0, err
	}
	aw.offset += int64(n)
	aw.unwritten = hdr.Size

	if hdr.SectionReader != nil {
		if hdr.SectionReader.Size() != hdr.Size {
			return 0, fmt.Errorf("incoherent header (%d) and SectionReader (%d)", hdr.Size, hdr.SectionReader.Size())
		}
		_, err := hdr.SectionReader.Seek(0, io.SeekStart)
		if err != nil {
			return 0, err
		}
		n, err := io.Copy(aw.w, hdr.SectionReader)
		aw.offset += n
		aw.unwritten -= n
		return n, err
	}
	return 0, nil
}

func (aw *Writer) Write(b []byte) (int, error) {
	n, err := aw.w.Write(b)
	aw.offset += int64(n)
	aw.unwritten -= int64(n)
	return n, err
}
