package thingscloud

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTime_ZeroValueReturnsNil(t *testing.T) {
	t.Parallel()

	if got := Time(time.Time{}); got != nil {
		bs, _ := json.Marshal(got)
		t.Errorf("Time(zero) = %s, want nil — the zero time marshals to a garbage 1754 epoch instead of being omitted", bs)
	}
}

func TestTime_RealValueRoundTrips(t *testing.T) {
	t.Parallel()

	now := time.Now()
	got := Time(now)
	if got == nil {
		t.Fatal("Time(now) = nil, want pointer")
	}
	if !time.Time(*got).Equal(now) {
		t.Errorf("Time(now) = %v, want %v", time.Time(*got), now)
	}
}
