package handlers

import "testing"

func TestJournalIsUnbalancedUsesAccountingTolerance(t *testing.T) {
	if journalIsUnbalanced(100, 100.004) {
		t.Fatal("sub-cent rounding difference should be accepted")
	}
	if !journalIsUnbalanced(100, 99.99) {
		t.Fatal("one-cent difference must be reported")
	}
}
