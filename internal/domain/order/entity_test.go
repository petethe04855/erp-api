package order_test

import (
	"testing"

	domainOrder "chawy-erp-api/internal/domain/order"
)

func TestOrderCanTransition(t *testing.T) {
	tests := []struct {
		name     string
		from     domainOrder.Status
		to       domainOrder.Status
		expected bool
	}{
		// From Pending
		{"Pending to Pending (idempotent)", domainOrder.StatusPending, domainOrder.StatusPending, true},
		{"Pending to Confirmed", domainOrder.StatusPending, domainOrder.StatusConfirmed, true},
		{"Pending to Cancelled", domainOrder.StatusPending, domainOrder.StatusCancelled, true},
		{"Pending to Shipped (forbidden skip)", domainOrder.StatusPending, domainOrder.StatusShipped, false},

		// From Confirmed
		{"Confirmed to Confirmed (idempotent)", domainOrder.StatusConfirmed, domainOrder.StatusConfirmed, true},
		{"Confirmed to Shipped", domainOrder.StatusConfirmed, domainOrder.StatusShipped, true},
		{"Confirmed to Cancelled", domainOrder.StatusConfirmed, domainOrder.StatusCancelled, true},
		{"Confirmed to Pending (revert forbidden)", domainOrder.StatusConfirmed, domainOrder.StatusPending, false},

		// From Shipped
		{"Shipped to Shipped (idempotent)", domainOrder.StatusShipped, domainOrder.StatusShipped, true},
		{"Shipped to Cancelled (forbidden)", domainOrder.StatusShipped, domainOrder.StatusCancelled, false},
		{"Shipped to Pending (forbidden)", domainOrder.StatusShipped, domainOrder.StatusPending, false},
		{"Shipped to Confirmed (forbidden)", domainOrder.StatusShipped, domainOrder.StatusConfirmed, false},

		// From Cancelled
		{"Cancelled to Cancelled (idempotent)", domainOrder.StatusCancelled, domainOrder.StatusCancelled, true},
		{"Cancelled to Pending (forbidden)", domainOrder.StatusCancelled, domainOrder.StatusPending, false},
		{"Cancelled to Confirmed (forbidden)", domainOrder.StatusCancelled, domainOrder.StatusConfirmed, false},
		{"Cancelled to Shipped (forbidden)", domainOrder.StatusCancelled, domainOrder.StatusShipped, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := domainOrder.CanTransition(tt.from, tt.to)
			if actual != tt.expected {
				t.Errorf("CanTransition(%s, %s) = %v; want %v", tt.from, tt.to, actual, tt.expected)
			}
		})
	}
}
