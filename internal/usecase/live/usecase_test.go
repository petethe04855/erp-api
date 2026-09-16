package live

import (
	"testing"
	"time"

	domainLive "chawy-erp-api/internal/domain/live"
)

func TestCalculateNetMinutes_SameDay(t *testing.T) {
	start := time.Date(2026, 9, 12, 19, 0, 0, 0, time.Local)
	end := time.Date(2026, 9, 12, 22, 15, 0, 0, time.Local) // 3h 15m = 195m
	breakMins := 15

	net, err := CalculateNetMinutes(start, end, breakMins)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := 180 // 195 - 15 = 180 mins (3h)
	if net != expected {
		t.Errorf("expected %d mins, got %d", expected, net)
	}
}

func TestCalculateNetMinutes_CrossMidnight(t *testing.T) {
	// 23:00 to 02:00 next day = 3 hours = 180 mins
	start := time.Date(2026, 9, 12, 23, 0, 0, 0, time.Local)
	end := time.Date(2026, 9, 12, 2, 0, 0, 0, time.Local) // End time clock is smaller than start
	breakMins := 10

	net, err := CalculateNetMinutes(start, end, breakMins)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := 170 // 180 - 10 = 170 mins
	if net != expected {
		t.Errorf("expected %d mins, got %d", expected, net)
	}
}

func TestApplyRounding(t *testing.T) {
	tests := []struct {
		minutes  int
		policy   domainLive.RoundingPolicy
		expected int
	}{
		{42, domainLive.RoundingActual, 42},
		{42, domainLive.RoundingQuarterUp, 45},
		{45, domainLive.RoundingQuarterUp, 45},
		{46, domainLive.RoundingQuarterUp, 60},
		{42, domainLive.RoundingUp10, 50},
		{42, domainLive.RoundingUp30, 60},
	}

	for _, tt := range tests {
		got := ApplyRounding(tt.minutes, tt.policy)
		if got != tt.expected {
			t.Errorf("ApplyRounding(%d, %s) = %d; want %d", tt.minutes, tt.policy, got, tt.expected)
		}
	}
}
