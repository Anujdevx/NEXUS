// Package http holds media-storage's REST handlers: attachment upload and download (MinIO, S3-compatible).
package http

import (
	"context"
	"io"
	"net/http"
	"strings"

	"github.com/minio/minio-go/v7"

	"nexus/shared/auth"
	"nexus/shared/envelope"
	"nexus/shared/httpx"

	"github.com/nexus/media-storage/internal/domain"
)

type Publisher interface {
	Publish(ctx context.Context, env envelope.Envelope) error
}

type API struct {
	MC     *minio.Client
	Bucket string
	Pub    Publisher
	Secret string
}

func (a *API) Register(app *httpx.App) {
	app.Handle("POST /api/v1/media", auth.Require(a.Secret)(a.upload))
	app.Handle("GET /api/v1/media/{id}", auth.Require(a.Secret)(a.download))
}

// upload takes a multipart form with a "file" field and stores it in the bucket.
func (a *API) upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, domain.MaxBytes+1<<20)
	if err := r.ParseMultipartForm(domain.MaxBytes); err != nil {
		httpx.Error(w, http.StatusRequestEntityTooLarge, "too_large", "send multipart form data with a file under 10 MiB")
		return
	}
	f, hdr, err := r.FormFile("file")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "missing form file field \"file\"")
		return
	}
	defer f.Close()
	ct := hdr.Header.Get("Content-Type")
	ext, err := domain.CheckType(ct)
	if err != nil {
		httpx.Error(w, http.StatusUnsupportedMediaType, "bad_type", err.Error())
		return
	}
	key := domain.NewKey(ext)
	if _, err := a.MC.PutObject(r.Context(), a.Bucket, key, f, hdr.Size, minio.PutObjectOptions{ContentType: ct}); err != nil {
		httpx.Error(w, http.StatusBadGateway, "storage_unavailable", err.Error())
		return
	}
	user := ""
	if c := auth.FromContext(r.Context()); c != nil {
		user = c.Role
	}
	_ = a.Pub.Publish(r.Context(), envelope.New("media", "uploaded", "ahvana", "media-storage").WithStatus(ct).
		WithPayload(map[string]any{"id": key, "bytes": hdr.Size, "type": ct, "by": user}))
	httpx.JSON(w, http.StatusCreated, map[string]any{"id": key, "url": "/api/v1/media/" + key, "bytes": hdr.Size, "type": ct})
}

func (a *API) download(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if strings.ContainsAny(id, "/\\") {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "bad id")
		return
	}
	obj, err := a.MC.GetObject(r.Context(), a.Bucket, id, minio.GetObjectOptions{})
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, "storage_unavailable", err.Error())
		return
	}
	defer obj.Close()
	st, err := obj.Stat()
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", "no such attachment")
		return
	}
	w.Header().Set("Content-Type", st.ContentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", "inline")
	_, _ = io.Copy(w, obj)
}
