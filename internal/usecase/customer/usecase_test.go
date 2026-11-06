package customer

import (
	"context"
	"testing"

	domainCustomer "chawy-erp-api/internal/domain/customer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRepo struct{ last *domainCustomer.Customer }

func (f *fakeRepo) Create(_ context.Context, c *domainCustomer.Customer) error {
	f.last = c
	return nil
}
func (f *fakeRepo) FindByID(context.Context, uint) (*domainCustomer.Customer, error) {
	return f.last, nil
}
func (f *fakeRepo) FindByCode(context.Context, string) (*domainCustomer.Customer, error) {
	return nil, nil
}
func (f *fakeRepo) FindAll(context.Context, domainCustomer.Query) ([]domainCustomer.Customer, int64, error) {
	return nil, 0, nil
}
func (f *fakeRepo) Update(_ context.Context, c *domainCustomer.Customer) error {
	f.last = c
	return nil
}
func (f *fakeRepo) Delete(context.Context, uint) error { return nil }

func TestCreateProvince(t *testing.T) {
	tests := []struct {
		name     string
		province string
		address  string
		want     string
	}{
		{"explicit province wins", "ชลบุรี", "123 ถ.สุขุมวิท กรุงเทพ", "ชลบุรี"},
		{"derived from address", "", "123 ถ.สุขุมวิท จ.ชลบุรี 20130", "ชลบุรี"},
		{"derived from english alias", "", "Bangkok, Thailand", "กรุงเทพมหานคร"},
		{"unknown stays empty", "", "12/34 ซอยอะไรบางอย่าง", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{}
			uc := NewCustomerUsecase(repo)
			c, err := uc.Create(context.Background(), CreateInput{
				Name:     "Test",
				Address:  tt.address,
				Province: tt.province,
			})
			require.NoError(t, err)
			assert.Equal(t, tt.want, c.Province)
		})
	}
}

func TestUpdateProvinceReDerivesWhenAddressChanges(t *testing.T) {
	repo := &fakeRepo{}
	uc := NewCustomerUsecase(repo)
	_, err := uc.Create(context.Background(), CreateInput{Name: "Test", Address: "กรุงเทพ"})
	require.NoError(t, err)
	require.Equal(t, "กรุงเทพมหานคร", repo.last.Province)

	// Address changed, no explicit province -> re-derive from new address.
	_, err = uc.Update(context.Background(), repo.last.ID, UpdateInput{Address: "100 ถ.มีชัย เชียงใหม่"})
	require.NoError(t, err)
	assert.Equal(t, "เชียงใหม่", repo.last.Province)

	// Explicit province wins over address.
	_, err = uc.Update(context.Background(), repo.last.ID, UpdateInput{Province: "ภูเก็ต"})
	require.NoError(t, err)
	assert.Equal(t, "ภูเก็ต", repo.last.Province)
}
