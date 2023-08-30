package ar

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"time"
)

const Signature = "!<arch>\n"

const headerEndChars = "`\n"

type Header struct {
	Name    string
	ModTime time.Time

	Uid int
	Gid int

	Mode os.FileMode
	Size int64

	// SectionReader:
	// - when reading: will be non-nil if the underlying reader is an [io.ReaderAt]
	// - when writing: if not-nil the data will be read and written to the archive
	SectionReader SectionReader
}

func unmarshalHeader(buf []byte) (h Header, err error) {
	parseInt := func(name string, input []byte, base, bitSize int) int64 {
		n, serr := strconv.ParseInt(strings.TrimRight(string(input), " "), base, bitSize)
		if serr != nil {
			err = errors.Join(err, fmt.Errorf("%s: %w", name, serr))
		}
		return n
	}

	h.Name = strings.TrimSuffix(strings.TrimSpace(string(buf[0:16])), "/")

	unixTime := parseInt("modification timestamp", buf[16:28], 10, 64)
	h.ModTime = time.Unix(unixTime, 0)

	h.Uid = int(parseInt("owner ID", buf[28:34], 10, 32))
	h.Gid = int(parseInt("group ID", buf[34:40], 10, 32))

	h.Mode = fs.FileMode(parseInt("file mode", buf[40:48], 8, 64))

	h.Size = parseInt("file size", buf[48:58], 10, 64)

	if string(buf[58:]) != headerEndChars {
		err = errors.Join(err, fmt.Errorf("expected end chars %q, got %q", headerEndChars, string(buf[58:])))
	}

	return h, err
}

func (h Header) marshal() ([]byte, error) {
	buf := bytes.Repeat([]byte{' '}, 58)
	name := h.Name
	if len(name) > 16 {
		return nil, fmt.Errorf("name is to long (max 16 chars): %q", name)
	}

	copy(buf[0:16], []byte(name))
	copy(buf[16:28], []byte(strconv.FormatInt(h.ModTime.Unix(), 10)))
	copy(buf[28:34], []byte(strconv.Itoa(h.Uid)))
	copy(buf[34:40], []byte(strconv.Itoa(h.Gid)))
	copy(buf[40:48], []byte(strconv.FormatUint(uint64(h.Mode), 8)))
	copy(buf[48:58], []byte(strconv.FormatInt(h.Size, 10)))
	buf = append(buf, []byte(headerEndChars)...)

	return buf, nil
}

type SectionReader interface {
	io.Reader
	io.ReaderAt
	io.Seeker
	Size() int64
}

func FileInfoHeader(fi fs.FileInfo) *Header {
	return &Header{
		Name:          fi.Name(),
		ModTime:       fi.ModTime(),
		Uid:           0,
		Gid:           0,
		Mode:          fi.Mode(),
		Size:          fi.Size(),
		SectionReader: nil,
	}
}

func (h Header) FileInfo() fs.FileInfo {
	return fileInfo{
		name:    h.Name,
		modTime: h.ModTime,
		mode:    h.Mode,
		size:    h.Size,
	}
}

type fileInfo struct {
	name    string
	modTime time.Time

	mode os.FileMode
	size int64
}

// IsDir implements fs.FileInfo.
func (fi fileInfo) IsDir() bool {
	return fi.mode.IsDir()
}

// ModTime implements fs.FileInfo.
func (fi fileInfo) ModTime() time.Time {
	return fi.modTime
}

// Mode implements fs.FileInfo.
func (fi fileInfo) Mode() fs.FileMode {
	return fi.mode
}

// Name implements fs.FileInfo.
func (fi fileInfo) Name() string {
	return fi.name
}

// Size implements fs.FileInfo.
func (fi fileInfo) Size() int64 {
	return fi.size
}

// Sys implements fs.FileInfo.
func (fi fileInfo) Sys() any {
	return nil
}
