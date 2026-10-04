package domain

import "testing"

func TestRiverStatus(t *testing.T) {
	cases := []struct {
		g    string
		lvl  float64
		want string
	}{{"ganga-rishikesh", 4.9, "below"}, {"ganga-rishikesh", 5.0, "warning"}, {"ganga-rishikesh", 6.2, "danger"}, {"unknown", 3.5, "warning"}}
	for _, c := range cases {
		if got, _ := RiverStatus(c.g, c.lvl); got != c.want {
			t.Errorf("%s %.1f: got %s want %s", c.g, c.lvl, got, c.want)
		}
	}
}

func TestClampStage(t *testing.T) {
	if ClampStage(-5) != 0 || ClampStage(120) != 100 || ClampStage(40) != 40 {
		t.Fatal("clamp")
	}
}
