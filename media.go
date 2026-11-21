package hello

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
)

// MediaService uploads and downloads WhatsApp media.
type MediaService struct{ c *Client }

// UploadedMedia is the result of Upload.
type UploadedMedia struct {
	// ID is the media ID to use as image.id, document.id, ... in a send, from
	// the same phone number. Valid for about 30 days.
	ID string `json:"id"`
}

// Upload uploads a file as multipart/form-data with messaging_product=whatsapp
// and type=mimeType (POST /v1/{phone_number_id}/media). filename is shown to the
// recipient for documents and defaults to "file". The file is read fully into
// memory first. An upload is not retried.
func (s *MediaService) Upload(ctx context.Context, phoneNumberID string, file io.Reader, filename, mimeType string) (*UploadedMedia, error) {
	id, err := pathSegment("phoneNumberID", phoneNumberID)
	if err != nil {
		return nil, err
	}
	if file == nil || mimeType == "" {
		return nil, errors.New("hello: file and mimeType are required")
	}
	if filename == "" {
		filename = "file"
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	// Writes to a bytes.Buffer cannot fail.
	_ = mw.WriteField("messaging_product", "whatsapp")
	_ = mw.WriteField("type", mimeType)
	part, _ := mw.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": {mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": filename})},
		"Content-Type":        {mimeType},
	})
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("hello: reading file: %w", err)
	}
	_ = mw.Close()

	var out UploadedMedia
	r := request{method: http.MethodPost, path: "/v1/" + id + "/media", raw: buf.Bytes(), rawType: mw.FormDataContentType()}
	if err := s.c.do(ctx, r, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// MediaInfo describes a media ID (GET /v1/{media_id}).
type MediaInfo struct {
	MessagingProduct string `json:"messaging_product"`
	// URL is a Hello URL; fetch it with the same API key (Download does).
	URL      string `json:"url"`
	MimeType string `json:"mime_type"`
	SHA256   string `json:"sha256"`
	FileSize int64  `json:"file_size"`
	ID       string `json:"id"`
}

// Get looks up a media ID, from an inbound webhook or an upload.
func (s *MediaService) Get(ctx context.Context, mediaID string) (*MediaInfo, error) {
	id, err := pathSegment("mediaID", mediaID)
	if err != nil {
		return nil, err
	}
	var out MediaInfo
	if err := s.c.do(ctx, request{method: http.MethodGet, path: "/v1/" + id, retry: true}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Download streams the file of a media ID (GET /v1/{media_id}/download). The
// caller must close the returned reader.
func (s *MediaService) Download(ctx context.Context, mediaID string) (io.ReadCloser, error) {
	id, err := pathSegment("mediaID", mediaID)
	if err != nil {
		return nil, err
	}
	resp, err := s.c.send(ctx, request{method: http.MethodGet, path: "/v1/" + id + "/download", retry: true})
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}
