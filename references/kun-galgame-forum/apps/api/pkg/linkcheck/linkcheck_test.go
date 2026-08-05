package linkcheck

import "testing"

func TestAggregate(t *testing.T) {
	d := Result{Status: StatusDead}
	a := Result{Status: StatusAlive}
	u := Result{Status: StatusUnknown}

	cases := []struct {
		name string
		in   []Result
		want Status
	}{
		{"empty → unknown (never a default-dead wipe)", nil, StatusUnknown},
		{"single dead → dead", []Result{d}, StatusDead},
		{"all dead → dead", []Result{d, d}, StatusDead},
		{"any alive wins over dead → alive", []Result{d, a}, StatusAlive},
		{"any alive wins over unknown → alive", []Result{u, a}, StatusAlive},
		{"dead + unknown (not all dead, no alive) → unknown", []Result{d, u}, StatusUnknown},
		{"all unknown → unknown", []Result{u, u}, StatusUnknown},
	}
	for _, tc := range cases {
		if got := aggregate(tc.in); got != tc.want {
			t.Errorf("%s: aggregate=%q want %q", tc.name, got, tc.want)
		}
	}
}
