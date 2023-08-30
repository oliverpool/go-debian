package ar

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"time"
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
	r        io.Reader
	offset   int64
	fileSize int64

	err      error
	readerAt io.ReaderAt
}

const Signature = "!<arch>\n"

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

	toDiscard := ar.fileSize
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

	h, err := newHeader(headerBuf)
	if err != nil {
		ar.err = fmt.Errorf("could not parse header: %w", err)
		return nil, ar.err
	}
	ar.fileSize = h.size

	if ar.readerAt != nil {
		h.SectionReader = io.NewSectionReader(ar.readerAt, ar.offset, h.size)
	}

	return &h, nil
}

type Header struct {
	name    string
	modTime time.Time

	Uid int
	Gid int

	mode os.FileMode
	size int64

	// SectionReader will be non-nil if the underlying reader is an [io.ReaderAt]
	SectionReader *io.SectionReader
}

var _ fs.FileInfo = Header{}

// IsDir implements fs.FileInfo.
func (h Header) IsDir() bool {
	return h.mode.IsDir()
}

// ModTime implements fs.FileInfo.
func (h Header) ModTime() time.Time {
	return h.modTime
}

// Mode implements fs.FileInfo.
func (h Header) Mode() fs.FileMode {
	return h.mode
}

// Name implements fs.FileInfo.
func (h Header) Name() string {
	return h.name
}

// Size implements fs.FileInfo.
func (h Header) Size() int64 {
	return h.size
}

// Sys implements fs.FileInfo.
func (h Header) Sys() any {
	return h.SectionReader
}

const headerEndChars = "`\n"

func newHeader(buf []byte) (h Header, err error) {
	parseInt := func(name string, input []byte, base, bitSize int) int64 {
		n, serr := strconv.ParseInt(strings.TrimRight(string(input), " "), base, bitSize)
		if serr != nil {
			err = errors.Join(err, fmt.Errorf("%s: %w", name, serr))
		}
		return n
	}

	h.name = strings.TrimSuffix(strings.TrimSpace(string(buf[0:16])), "/")

	unixTime := parseInt("modification timestamp", buf[16:28], 10, 64)
	h.modTime = time.Unix(unixTime, 0)

	h.Uid = int(parseInt("owner ID", buf[28:34], 10, 32))
	h.Gid = int(parseInt("group ID", buf[34:40], 10, 32))

	h.mode = fs.FileMode(parseInt("file mode", buf[40:48], 8, 64))

	h.size = parseInt("file size", buf[48:58], 10, 64)

	if string(buf[58:]) != headerEndChars {
		err = errors.Join(err, fmt.Errorf("expected end chars %q, got %q", headerEndChars, string(buf[58:])))
	}

	return h, err
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
