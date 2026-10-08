package tiktok_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"chawy-erp-api/pkg/tiktok"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetOrdersDetail(t *testing.T) {
	mockResponse := tiktok.OrderDetailResponse{
		Code:    0,
		Message: "success",
	}
	mockResponse.Data.Orders = []tiktok.OrderDetailItem{
		{
			ID:     "57668123555",
			Status: "DELIVERED",
			RecipientAddress: tiktok.RecipientAddress{
				PostalCode: "10110",
				RegionCode: "TH",
				DistrictInfo: []tiktok.DistrictInfo{
					{
						AddressLevel:     "L1",
						AddressLevelName: "Province",
						AddressName:      "Bangkok",
					},
					{
						AddressLevel:     "L2",
						AddressLevelName: "District",
						AddressName:      "Watthana",
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/order/202309/orders", r.URL.Path)
		assert.Equal(t, "57668123555", r.URL.Query().Get("ids"))
		assert.Equal(t, "mock-token", r.Header.Get("x-tts-access-token"))
		assert.NotEmpty(t, r.URL.Query().Get("sign"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	// Set test custom base URL via client
	client := tiktok.NewClientWithBaseURL(server.URL)
	resp, err := client.GetOrdersDetail("mock-token", "mock-shop", "app-key", "app-secret", []string{"57668123555"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 0, resp.Code)
	require.Len(t, resp.Data.Orders, 1)
	assert.Equal(t, "57668123555", resp.Data.Orders[0].ID)
	assert.Equal(t, "10110", resp.Data.Orders[0].RecipientAddress.PostalCode)
	require.Len(t, resp.Data.Orders[0].RecipientAddress.DistrictInfo, 2)
	assert.Equal(t, "Bangkok", resp.Data.Orders[0].RecipientAddress.DistrictInfo[0].AddressName)
}
