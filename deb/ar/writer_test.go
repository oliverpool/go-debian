package ar_test

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"pault.ag/go/debian/deb/ar"
)

func TestFileInfoHeader(t *testing.T) {
	f, err := os.Open("../testdata/multi_archive.a")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })

	fi, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	hdr := ar.FileInfoHeader(fi)
	assertEqual(t, "multi_archive.a", hdr.Name)
	assertEqual(t, 1693419806, hdr.ModTime.Unix())
	assertEqual(t, 0, hdr.Uid)
	assertEqual(t, 0, hdr.Gid)
	assertEqual(t, "-rw-r--r--", hdr.Mode.String())
	assertEqual(t, 156, hdr.Size)

}

func TestWriter(t *testing.T) {
	var buf bytes.Buffer
	w := ar.NewWriter(&buf)
	n, err := w.WriteHeader(&ar.Header{
		Name:          "debian-binary",
		ModTime:       time.Unix(1342943816, 0),
		Uid:           0,
		Gid:           1,
		Mode:          0o100644,
		Size:          4,
		SectionReader: io.NewSectionReader(strings.NewReader("2.0\n"), 0, 4),
	})
	assertEqual(t, 4, n)
	if err != nil {
		t.Fatal(err)
	}

	assertEqual(t, "!<arch>\n"+
		"debian-binary   1342943816  0     1     100644  4         `\n2.0\n",
		buf.String())

	n, err = w.WriteHeader(&ar.Header{
		Name:    "hello",
		ModTime: time.Unix(1342943816, 0),
		Uid:     0,
		Gid:     1,
		Mode:    0o100644,
		Size:    3,
	})
	assertEqual(t, 0, n)
	if err != nil {
		t.Fatal(err)
	}
	n32, err := w.Write([]byte("123"))
	assertEqual(t, 3, n32)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, "!<arch>\n"+
		"debian-binary   1342943816  0     1     100644  4         `\n2.0\n"+
		"hello           1342943816  0     1     100644  3         `\n123",
		buf.String())

	// padding should be added
	n, err = w.WriteHeader(&ar.Header{
		Name:    "hello",
		ModTime: time.Unix(1342943816, 0),
		Uid:     0,
		Gid:     1,
		Mode:    0o100644,
		Size:    40,
	})
	assertEqual(t, 0, n)
	if err != nil {
		t.Fatal(err)
	}
	n32, err = w.Write([]byte("123"))
	assertEqual(t, 3, n32)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, "!<arch>\n"+
		"debian-binary   1342943816  0     1     100644  4         `\n2.0\n"+
		"hello           1342943816  0     1     100644  3         `\n123\n"+
		"hello           1342943816  0     1     100644  40        `\n123",
		buf.String())

}
