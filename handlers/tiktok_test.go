package handlers

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"chawy-erp-api/models"
)

func TestShouldSkipOnlyZeroValueUnmappedTiktokItems(t *testing.T) {
	unmapped := fmt.Errorf("%w: missing", errTiktokSKUUnmapped)
	if !shouldSkipUnmappedTiktokItem(models.TiktokOrderItem{Amount: 0}, unmapped) {
		t.Fatal("zero-value unmapped item should be treated as non-stock")
	}
	if shouldSkipUnmappedTiktokItem(models.TiktokOrderItem{Amount: 10}, unmapped) {
		t.Fatal("paid unmapped item must block stock deduction")
	}
	if shouldSkipUnmappedTiktokItem(models.TiktokOrderItem{Amount: 0}, errors.New("database unavailable")) {
		t.Fatal("database errors must not be skipped")
	}
}

func TestResolveTiktokBundleComponentSKUMatchesProduct(t *testing.T) {
	component := models.BundleComponent{ComponentProductID: 24, ComponentSku: " fd-rl-80-10 "}
	sku, err := resolveTiktokBundleComponentSKU("FD-MX-C1T2-03", component, "FD-RL-80-10")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sku != "FD-RL-80-10" {
		t.Fatalf("resolved SKU = %s, want FD-RL-80-10", sku)
	}
}

func TestResolveTiktokBundleComponentSKURejectsMismatch(t *testing.T) {
	component := models.BundleComponent{ComponentProductID: 24, ComponentSku: "FD-CK-80-01"}
	_, err := resolveTiktokBundleComponentSKU("FD-MX-C1T2-03", component, "FD-RL-80-10")
	if err == nil || !strings.Contains(err.Error(), "component data mismatch") {
		t.Fatalf("expected component mismatch error, got %v", err)
	}
}

func TestResolveTiktokBundleComponentSKUFallsBackWithoutProductID(t *testing.T) {
	component := models.BundleComponent{ComponentSku: " fd-tn-80-01 "}
	sku, err := resolveTiktokBundleComponentSKU("FD-MX-C1T2-03", component, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sku != "FD-TN-80-01" {
		t.Fatalf("resolved SKU = %s, want FD-TN-80-01", sku)
	}
}

func TestTiktokOrderNeedsStockDeduction(t *testing.T) {
	for _, status := range []string{"SHIPPED", "IN_TRANSIT", "DELIVERED", "COMPLETED", " completed "} {
		if !tiktokOrderNeedsStockDeduction(status) {
			t.Errorf("status %q should require deduction", status)
		}
	}
	for _, status := range []string{"UNPAID", "ON_HOLD", "AWAITING_SHIPMENT", "CANCELLED"} {
		if tiktokOrderNeedsStockDeduction(status) {
			t.Errorf("status %q should not require deduction", status)
		}
	}
}
