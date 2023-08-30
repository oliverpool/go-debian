package ar_test

import (
	"io"
	"os"
	"strings"
	"testing"

	"pault.ag/go/debian/deb/ar"
)

func assertEqual[T comparable](t *testing.T, expected T, actual T) bool {
	t.Helper()
	if expected != actual {
		t.Errorf("expected %v, got %v", expected, actual)
		return false
	}
	return true
}

// As discussed in testdata/README.md, `multi_archive.a` is taken from Blake
// Smith's ar project. Some of the data below follows.
func TestReaderAt(t *testing.T) {
	file, err := os.Open("../testdata/multi_archive.a")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { file.Close() })

	ar := ar.NewReader(file)

	firstEntry, err := ar.Next()
	if err != nil {
		t.Fatal(err)
	}

	assertEqual(t, `hello.txt`, firstEntry.Name)
	assertEqual(t, 1361157466, firstEntry.ModTime.Unix())
	assertEqual(t, 501, firstEntry.Uid)
	assertEqual(t, 20, firstEntry.Gid)
	assertEqual(t, 13, firstEntry.Size)
	assertEqual(t, 0o100644, firstEntry.Mode)

	if firstEntry.SectionReader == nil {
		t.Fatal("firstEntry.SectionReader should not be nil")
	}

	hello := make([]byte, 5)
	n, err := ar.Read(hello)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, 5, n)
	assertEqual(t, "hello", string(hello))

	firstContent, err := io.ReadAll(firstEntry.SectionReader)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, 13, int64(len(firstContent)))
	assertEqual(t, "Hello world!\n", string(firstContent))

	{
		secondEntry, err := ar.Next()
		if err != nil {
			t.Fatal(err)
		}
		assertEqual(t, `lamp.txt`, secondEntry.Name)
		assertEqual(t, 1361248906, secondEntry.ModTime.Unix())
		assertEqual(t, 501, secondEntry.Uid)
		assertEqual(t, 20, secondEntry.Gid)
		assertEqual(t, 13, secondEntry.Size)
		assertEqual(t, 0o100644, secondEntry.Mode)
		secondContent, err := io.ReadAll(secondEntry.SectionReader)
		if err != nil {
			t.Fatal(err)
		}
		assertEqual(t, secondEntry.Size, int64(len(secondContent)))
		assertEqual(t, "I love lamp.\n", string(secondContent))
	}

	// Now, test that we can rewind and reread the first file even after
	// reading the second one.
	_, err = firstEntry.SectionReader.Seek(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	firstRereadContent, err := io.ReadAll(firstEntry.SectionReader)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, "Hello world!\n", string(firstRereadContent))

	lastEntry, err := ar.Next()
	if err != io.EOF {
		t.Fatal(err)
	}
	assertEqual(t, nil, lastEntry)

	lastEntry, err = ar.Next()
	if err != io.EOF {
		t.Fatal(err)
	}
	assertEqual(t, nil, lastEntry)
}

func stringReader(s string) io.Reader {
	// add LimitReader to prevent implementing ReaderAt
	return io.LimitReader(strings.NewReader(s), int64(len(s)))
}

func TestReader(t *testing.T) {
	r := stringReader("!<arch>\n" +
		"debian-binary   1342943816  0     0     100644  4         `\n2.0\n")
	if _, ok := r.(io.ReaderAt); ok {
		t.Fatal("should not be an io.ReaderAt")
	}
	ar := ar.NewReader(r)

	firstEntry, err := ar.Next()
	if err != nil {
		t.Fatal(err)
	}

	assertEqual(t, `debian-binary`, firstEntry.Name)
	assertEqual(t, 1342943816, firstEntry.ModTime.Unix())
	assertEqual(t, 0, firstEntry.Uid)
	assertEqual(t, 0, firstEntry.Gid)
	assertEqual(t, 4, firstEntry.Size)
	assertEqual(t, 0o100644, firstEntry.Mode)

	if firstEntry.SectionReader != nil {
		t.Fatal("firstEntry.SectionReader should be nil")
	}

	two := make([]byte, 3)
	n, err := ar.Read(two)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, 3, n)
	assertEqual(t, "2.0", string(two))

	lastEntry, err := ar.Next()
	if err != io.EOF {
		t.Fatal(err)
	}
	assertEqual(t, nil, lastEntry)

	lastEntry, err = ar.Next()
	if err != io.EOF {
		t.Fatal(err)
	}
	assertEqual(t, nil, lastEntry)
}
