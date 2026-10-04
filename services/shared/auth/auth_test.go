package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func serve(h http.HandlerFunc, token, key string) int {
	r := httptest.NewRequest("GET", "/", nil)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if key != "" {
		r.Header.Set("X-Service-Key", key)
	}
	w := httptest.NewRecorder()
	h(w, r)
	return w.Code
}

func TestRolesAndKeys(t *testing.T) {
	ok := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }
	ctl, _, _ := Sign("s3", "u1", "c@x", Controller, time.Hour)
	res, _, _ := Sign("s3", "u2", "r@x", Responder, time.Hour)
	cit, _, _ := Sign("s3", "u3", "z@x", Citizen, time.Hour)
	other, _, _ := Sign("other-secret", "u4", "o@x", Controller, time.Hour)
	expired, _, _ := Sign("s3", "u5", "e@x", Controller, -time.Minute)

	ctlOnly := Require("s3", Controller)(ok)
	field := Require("s3", Controller, Responder)(ok)
	anyone := Require("s3")(ok)
	for _, c := range []struct {
		name string
		h    http.HandlerFunc
		tok  string
		want int
	}{
		{"controller passes controller-only", ctlOnly, ctl, 200},
		{"responder refused controller-only", ctlOnly, res, 403},
		{"citizen refused field endpoint", field, cit, 403},
		{"controller passes a responder endpoint", Require("s3", Responder)(ok), ctl, 200},
		{"anyone with a token", anyone, cit, 200},
		{"no token", anyone, "", 401},
		{"wrong signing secret", anyone, other, 401},
		{"expired token", anyone, expired, 401},
	} {
		if got := serve(c.h, c.tok, ""); got != c.want {
			t.Errorf("%s: %d want %d", c.name, got, c.want)
		}
	}
	either := RequireOrKey("s3", "k1", Controller)(ok)
	if serve(either, "", "k1") != 200 || serve(either, "", "bad") != 401 || serve(either, ctl, "") != 200 || serve(either, res, "") != 403 {
		t.Error("RequireOrKey: key or role")
	}
	if serve(ServiceKey("k1")(ok), "", "k1") != 200 || serve(ServiceKey("k1")(ok), ctl, "") != 401 {
		t.Error("ServiceKey")
	}
}
