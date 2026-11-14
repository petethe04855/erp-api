package handler

import "testing"

func TestIsProtectedUploadPath(t *testing.T) {
	cases := map[string]bool{
		"/uploads/expenses/expense-1.jpg":         true,
		"/uploads/incomes/income-1.jpg":           true,
		"/UPLOADS/Expenses/expense-1.jpg":         true,
		"/uploads//expenses/expense-1.jpg":        true,
		"/uploads/images/../expenses/a.jpg":       true,
		"/uploads/expenses":                       true,
		`/uploads\expenses\a.jpg`:                 true,
		"/uploads/images/product.jpg":             false,
		"/uploads/logos/chawy.jpg":                false,
		"/uploads/expenses-archive-public/a.jpg":  false,
	}
	for p, want := range cases {
		if got := IsProtectedUploadPath(p); got != want {
			t.Errorf("IsProtectedUploadPath(%q) = %v, want %v", p, got, want)
		}
	}
}
