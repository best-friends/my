// SPDX-License-Identifier: AGPL-3.0-only

package finance_test

import (
	"errors"
	"testing"

	"github.com/best-friends/my/internal/domain/finance"
)

func TestParseMoney(t *testing.T) {
	for _, tc := range []struct {
		amount, currency string
		want             finance.Money
		err              error
	}{
		{"1650", "JPY", finance.Money{Minor: 1650, Currency: "JPY"}, nil},
		{"12.34", "USD", finance.Money{Minor: 1234, Currency: "USD"}, nil},
		{"12.3", "USD", finance.Money{Minor: 1230, Currency: "USD"}, nil},
		{"0", "BHD", finance.Money{Minor: 0, Currency: "BHD"}, nil},
		{"1.5", "JPY", finance.Money{}, finance.ErrTooManyDigits},
		{"0.001", "USD", finance.Money{}, finance.ErrTooManyDigits},
		{"1e3", "USD", finance.Money{}, finance.ErrInvalidDecimal},
		{"", "USD", finance.Money{}, finance.ErrInvalidDecimal},
		{"1", "usd", finance.Money{}, finance.ErrInvalidCurrency},
		{"1", "XXX", finance.Money{}, finance.ErrInvalidCurrency},
	} {
		got, err := finance.ParseMoney(tc.amount, tc.currency)
		if !errors.Is(err, tc.err) || got != tc.want {
			t.Errorf("ParseMoney(%q, %q) = %+v, %v; want %+v, %v", tc.amount, tc.currency, got, err, tc.want, tc.err)
		}
	}
}
