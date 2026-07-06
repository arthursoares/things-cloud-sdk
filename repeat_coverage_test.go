package thingscloud

import (
	"testing"
	"time"
)

// FrequencyUnit 0 matches none of the daily/weekly/monthly/yearly constants, so
// both entry points fall through to their zero-time default branch.
func TestComputeFirstScheduledAt_UnknownFrequency(t *testing.T) {
	t.Parallel()
	c := RepeaterConfiguration{FrequencyUnit: FrequencyUnit(0)}
	if got := c.ComputeFirstScheduledAt(time.Now()); !got.IsZero() {
		t.Errorf("ComputeFirstScheduledAt = %v, want zero time", got)
	}
}

func TestNextScheduledAt_UnknownFrequency(t *testing.T) {
	t.Parallel()
	c := RepeaterConfiguration{FrequencyUnit: FrequencyUnit(0)}
	if got := c.NextScheduledAt(0); !got.IsZero() {
		t.Errorf("NextScheduledAt = %v, want zero time", got)
	}
}

// A daily repeater bounded by RepeatCount (no LastScheduledAt) must stop once
// the requested occurrence reaches the count.
func TestNextDailyScheduledAt_RepeatCountExhausted(t *testing.T) {
	t.Parallel()
	first := Timestamp(time.Date(2017, 9, 3, 0, 0, 0, 0, time.UTC))
	count := int64(3)
	c := RepeaterConfiguration{
		FrequencyUnit:      FrequencyUnitDaily,
		FrequencyAmplitude: 1,
		FirstScheduledAt:   &first,
		RepeatCount:        &count,
	}
	// Occurrence within the count is a real date.
	if got := c.NextScheduledAt(1); got.IsZero() {
		t.Error("NextScheduledAt(1) = zero, want a real date within RepeatCount")
	}
	// Occurrence at/after the count is exhausted -> zero time.
	if got := c.NextScheduledAt(3); !got.IsZero() {
		t.Errorf("NextScheduledAt(3) = %v, want zero time (RepeatCount exhausted)", got)
	}
}
