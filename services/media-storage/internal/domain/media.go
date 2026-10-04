// Package domain holds media-storage's upload rules.
package domain

import (
	"fmt"
	"mime"
	"strings"

	"github.com/google/uuid"
)

const MaxBytes = 10 << 20 // 10 MiB per attachment

var allowed = map[string]string{
	"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp", "image/gif": ".gif",
	"audio/mpeg": ".mp3", "audio/ogg": ".ogg", "audio/webm": ".weba", "audio/wav": ".wav",
	"video/mp4": ".mp4", "video/webm": ".webm", "application/pdf": ".pdf",
}

// CheckType accepts only attachments a field report can reasonably carry.
func CheckType(contentType string) (string, error) {
	mt, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return "", fmt.Errorf("unreadable content type %q", contentType)
	}
	ext, ok := allowed[strings.ToLower(mt)]
	if !ok {
		return "", fmt.Errorf("content type %s is not allowed (images, audio, video and PDF only)", mt)
	}
	return ext, nil
}

// NewKey is the object key for an upload: a fresh uuid plus the extension for its type.
func NewKey(ext string) string { return uuid.NewString() + ext }
