package domain

import "testing"

func TestPasswordRoundTrip(t *testing.T) {
	h, err := HashPassword("s3cret")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword("s3cret", h) || CheckPassword("wrong", h) {
		t.Fatal("password check failed")
	}
}
