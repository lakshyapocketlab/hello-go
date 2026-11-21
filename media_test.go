package hello

import (
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
)

type failReader struct{}

func (failReader) Read([]byte) (int, error) { return 0, errors.New("disk gone") }

func TestMediaUpload(t *testing.T) {
	f, c := newFake(t, jsonReply(200, `{"id":"1099"}`))
	got, err := c.Media.Upload(context.Background(), "106", strings.NewReader("%PDF-data"), `inv "1".pdf`, "application/pdf")
	if err != nil || got.ID != "1099" {
		t.Fatalf("%+v %v", got, err)
	}
	r := f.last()
	expect(t, r, "POST", "/v1/106/media")
	mt, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != "multipart/form-data" {
		t.Fatalf("content type %q", r.Header.Get("Content-Type"))
	}
	mr := multipart.NewReader(strings.NewReader(string(r.Body)), params["boundary"])
	fields := map[string]string{}
	var fileName, fileType string
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(p)
		if p.FormName() == "file" {
			fileName, fileType = p.FileName(), p.Header.Get("Content-Type")
		}
		fields[p.FormName()] = string(data)
	}
	if fields["messaging_product"] != "whatsapp" || fields["type"] != "application/pdf" || fields["file"] != "%PDF-data" ||
		fileName != `inv "1".pdf` || fileType != "application/pdf" {
		t.Fatalf("fields=%v name=%q type=%q", fields, fileName, fileType)
	}

	// Default file name.
	_, _ = c.Media.Upload(context.Background(), "106", strings.NewReader("x"), "", "image/png")
	if !strings.Contains(string(f.last().Body), `filename=file`) {
		t.Fatal("default filename")
	}

	if _, err := c.Media.Upload(context.Background(), "", strings.NewReader("x"), "", "a/b"); err == nil {
		t.Fatal("empty id")
	}
	if _, err := c.Media.Upload(context.Background(), "1", nil, "", "a/b"); err == nil {
		t.Fatal("nil file")
	}
	if _, err := c.Media.Upload(context.Background(), "1", strings.NewReader("x"), "", ""); err == nil {
		t.Fatal("empty mime")
	}
	if _, err := c.Media.Upload(context.Background(), "1", failReader{}, "", "a/b"); err == nil || !strings.Contains(err.Error(), "disk gone") {
		t.Fatalf("err = %v", err)
	}

	_, c = newFake(t, jsonReply(400, `{"error":{"message":"bad type","code":100,"error_data":{"hello_code":"INVALID_MEDIA"}}}`))
	_, err = c.Media.Upload(context.Background(), "1", strings.NewReader("x"), "", "a/b")
	var ae *APIError
	if !errors.As(err, &ae) || ae.HelloCode != "INVALID_MEDIA" {
		t.Fatalf("err = %v", err)
	}
}

func TestMediaGetAndDownload(t *testing.T) {
	f, c := newFake(t, jsonReply(200, `{"messaging_product":"whatsapp","url":"https://x/v1/9/download","mime_type":"image/jpeg","sha256":"abc","file_size":48213,"id":"9"}`))
	got, err := c.Media.Get(context.Background(), "9")
	if err != nil || got.URL == "" || got.MimeType != "image/jpeg" || got.FileSize != 48213 || got.SHA256 != "abc" {
		t.Fatalf("%+v %v", got, err)
	}
	expect(t, f.last(), "GET", "/v1/9")
	if _, err := c.Media.Get(context.Background(), ""); err == nil {
		t.Fatal("empty id")
	}
	_, c = newFake(t, jsonReply(404, `{"error":{"message":"gone","code":100,"error_subcode":33}}`))
	if _, err := c.Media.Get(context.Background(), "9"); !IsNotFound(err) {
		t.Fatal(err)
	}

	f, c = newFake(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = io.WriteString(w, "BYTES")
	})
	rc, err := c.Media.Download(context.Background(), "9")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(rc)
	rc.Close()
	if string(data) != "BYTES" {
		t.Fatalf("data = %q", data)
	}
	expect(t, f.last(), "GET", "/v1/9/download")
	if _, err := c.Media.Download(context.Background(), ""); err == nil {
		t.Fatal("empty id")
	}
	_, c = newFake(t, jsonReply(404, `{"message":"x"}`))
	if _, err := c.Media.Download(context.Background(), "9"); !IsNotFound(err) {
		t.Fatal(err)
	}
}
