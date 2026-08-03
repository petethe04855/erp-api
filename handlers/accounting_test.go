package handlers

import "testing"

func TestValidatePostingLines(t *testing.T) {
	tests := []struct {
		name    string
		lines   []postingLine
		wantErr bool
	}{
		{"balanced", []postingLine{{AccountCode: "1200", Debit: 100}, {AccountCode: "4000", Credit: 100}}, false},
		{"unbalanced", []postingLine{{AccountCode: "1200", Debit: 100}, {AccountCode: "4000", Credit: 90}}, true},
		{"both sides", []postingLine{{AccountCode: "1200", Debit: 100, Credit: 100}, {AccountCode: "4000", Credit: 100}}, true},
		{"zero line", []postingLine{{AccountCode: "1200"}, {AccountCode: "4000"}}, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validatePostingLines(test.lines)
			if (err != nil) != test.wantErr {
				t.Fatalf("validatePostingLines() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}
