package messages

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
	"sort"
	"strings"
)

// From https://golang.org/src/mime/multipart/writer.go?s=2140:2215#L76
func writeHeader(w io.Writer, header textproto.MIMEHeader) error {
	keys := make([]string, 0, len(header))
	for k := range header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range header[k] {
			if _, err := fmt.Fprintf(w, "%s: %s\r\n", k, v); err != nil {
				return err
			}
		}
	}
	_, err := fmt.Fprintf(w, "\r\n")
	return err
}

// A writer formats entities.
type Writer struct {
	w  io.Writer
	c  io.Closer
	mw *multipart.Writer
}

func newWriter(w io.Writer, header textproto.MIMEHeader) (textproto.MIMEHeader, *Writer) {
	ww := &Writer{w: w}

	mediaType, mediaParams, _ := mime.ParseMediaType(header.Get("Content-Type"))
	if strings.HasPrefix(mediaType, "multipart/") {
		ww.mw = multipart.NewWriter(ww.w)
		ww.c = ww.mw

		if mediaParams["boundary"] != "" {
			ww.mw.SetBoundary(mediaParams["boundary"])
		} else {
			mediaParams["boundary"] = ww.mw.Boundary()
			header.Set("Content-Type", mime.FormatMediaType(mediaType, mediaParams))
		}

		header.Del("Content-Transfer-Encoding")
	} else {
		wc := encode(header.Get("Content-Transfer-Encoding"), ww.w)
		ww.w = wc
		ww.c = wc
	}

	return header, ww
}

// CreateWriter creates a new Writer writing to w.
func CreateWriter(w io.Writer, header textproto.MIMEHeader) (*Writer, error) {
	header, ww := newWriter(w, header)

	if err := writeHeader(w, header); err != nil {
		return nil, err
	}

	return ww, nil
}

// Write implements io.Writer.
func (w *Writer) Write(b []byte) (int, error) {
	return w.w.Write(b)
}

// Close implements io.Closer.
func (w *Writer) Close() error {
	return w.c.Close()
}

// CreatePart returns a Writer to a new part in this multipart entity. If this
// entity is not multipart, it fails.
func (w *Writer) CreatePart(header textproto.MIMEHeader) (*Writer, error) {
	if w.mw == nil {
		return nil, errors.New("cannot create a part in a non-multipart message")
	}

	// cw -> ww -> pw -> w.mw -> w.w

	ww := &struct{ io.Writer }{nil}
	header, cw := newWriter(ww, header)

	pw, err := w.mw.CreatePart(header)
	if err != nil {
		return nil, err
	}

	ww.Writer = pw
	return cw, nil
}