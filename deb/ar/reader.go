package ar

import (
	"errors"
	"fmt"
	"io"
)

// NewReader reads an ar archive.
// If r is also an [io.ReaderAt], the [Header] will have the field SectionReader populated.
func NewReader(r io.Reader) *Reader {
	ar := Reader{
		r: r,
	}
	if readerAt, ok := r.(io.ReaderAt); ok {
		ar.readerAt = readerAt
	}
	return &ar
}

type Reader struct {
	r             io.Reader
	offset        int64
	remainingRead int64
	current       io.Reader

	err      error
	readerAt io.ReaderAt
}

var _ io.Reader = &Reader{}

func (ar *Reader) Next() (*Header, error) {
	if ar.err != nil {
		return nil, ar.err
	}
	if ar.offset == 0 {
		buf := make([]byte, len(Signature))
		_, err := io.ReadFull(ar.r, buf)
		if err != nil {
			ar.err = fmt.Errorf("could not read signature: %w", err)
			return nil, ar.err
		}
		ar.offset += int64(len(Signature))

		if string(buf) != Signature {
			ar.err = fmt.Errorf("expected signature %q, got %q", Signature, string(buf))
			return nil, ar.err
		}
	}

	toDiscard := ar.remainingRead
	if (ar.offset+toDiscard)%2 == 1 {
		toDiscard += 1
	}

	headerBuf := make([]byte, 60)

	if ar.readerAt != nil {
		ar.offset += toDiscard
		n, err := ar.readerAt.ReadAt(headerBuf, ar.offset)
		if err != nil {
			ar.err = err
			if err == io.EOF && n > 0 {
				err = io.ErrUnexpectedEOF
			}
			if err != io.EOF {
				ar.err = fmt.Errorf("could not read header: %w", err)
			}
			return nil, ar.err
		}
	} else {
		if toDiscard > 0 {
			err := discard(ar.r, toDiscard)
			if err != nil {
				ar.err = err
				return nil, err
			}
			ar.offset += toDiscard
		}

		_, err := io.ReadFull(ar.r, headerBuf)
		if err != nil {
			if err == io.EOF {
				ar.err = io.EOF
			} else {
				ar.err = fmt.Errorf("could not read header: %w", err)
			}
			return nil, ar.err
		}
	}

	ar.offset += 60

	h, err := unmarshalHeader(headerBuf)
	if err != nil {
		ar.err = fmt.Errorf("could not parse header: %w", err)
		return nil, ar.err
	}
	ar.remainingRead = h.Size
	ar.current = io.LimitReader(ar.r, h.Size)

	if ar.readerAt != nil {
		h.SectionReader = io.NewSectionReader(ar.readerAt, ar.offset, h.Size)
	}

	return &h, nil
}

// Read the current entry
func (ar *Reader) Read(p []byte) (n int, err error) {
	if ar.err != nil {
		return 0, ar.err
	}
	if ar.current == nil {
		if ar.offset == 0 {
			return 0, errors.New("you must first call Next")
		}
		return 0, errors.New("no more entries")
	}
	n, err = ar.current.Read(p)
	ar.offset += int64(n)
	ar.remainingRead -= int64(n)
	return n, err
}

// copied from stdlib: archive/tar
//
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license.
//
// discard skips n bytes in r, reporting an error if unable to do so.
func discard(r io.Reader, n int64) error {
	// If possible, Seek to the last byte before the end of the data section.
	// Do this because Seek is often lazy about reporting errors; this will mask
	// the fact that the stream may be truncated. We can rely on the
	// io.CopyN done shortly afterwards to trigger any IO errors.
	var seekSkipped int64 // Number of bytes skipped via Seek
	if sr, ok := r.(io.Seeker); ok && n > 1 {
		// Not all io.Seeker can actually Seek. For example, os.Stdin implements
		// io.Seeker, but calling Seek always returns an error and performs
		// no action. Thus, we try an innocent seek to the current position
		// to see if Seek is really supported.
		pos1, err := sr.Seek(0, io.SeekCurrent)
		if pos1 >= 0 && err == nil {
			// Seek seems supported, so perform the real Seek.
			pos2, err := sr.Seek(n-1, io.SeekCurrent)
			if pos2 < 0 || err != nil {
				return err
			}
			seekSkipped = pos2 - pos1
		}
	}

	copySkipped, err := io.CopyN(io.Discard, r, n-seekSkipped)
	if err == io.EOF && seekSkipped+copySkipped < n {
		err = io.ErrUnexpectedEOF
	}
	return err
}
