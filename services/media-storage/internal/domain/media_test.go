package domain

import (
	"strings"
	"testing"
)

func TestCheckType(t *testing.T) {
	if ext, err := CheckType("image/jpeg"); err != nil || ext != ".jpg" {
		t.Fatalf("jpeg: %v %v", ext, err)
	}
	if _, err := CheckType("application/x-msdownload"); err == nil {
		t.Fatal("executables must be rejected")
	}
	if _, err := CheckType("text/html; charset=utf-8"); err == nil {
		t.Fatal("html must be rejected")
	}
	if !strings.HasSuffix(NewKey(".png"), ".png") {
		t.Fatal("key extension")
	}
}
