// SPDX-License-Identifier: AGPL-3.0-only

package stripe

import (
	"strconv"
	"strings"
)

// exponents are the decimal places of the amounts that Stripe's API takes in the currencies
// that do not have two (https://docs.stripe.com/currencies#zero-decimal and #three-decimal).
// Stripe sends ISK, HUF, and TWD with two decimal places, as the default.
var exponents = map[string]int{
	"BIF": 0, "CLP": 0, "DJF": 0, "GNF": 0, "JPY": 0, "KMF": 0, "KRW": 0, "MGA": 0, "PYG": 0,
	"RWF": 0, "UGX": 0, "VND": 0, "VUV": 0, "XAF": 0, "XOF": 0, "XPF": 0,
	"BHD": 3, "JOD": 3, "KWD": 3, "OMR": 3, "TND": 3,
}

// exponent returns the decimal places of Stripe's amounts in currency (ISO 4217, any case).
func exponent(currency string) int {
	if e, ok := exponents[strings.ToUpper(currency)]; ok {
		return e
	}
	return 2
}

// minimumAmounts are Stripe's smallest charges in the currencies of its settlement, in minor
// units (https://docs.stripe.com/currencies#minimum-and-maximum-charge-amounts). A currency
// outside the table may still be charged, converted to the account's currency, with a minimum
// that depends on the rate; MinimumAmount reports it as not taken.
var minimumAmounts = map[string]int64{
	"USD": 50, "AED": 200, "AUD": 50, "BGN": 100, "BRL": 50, "CAD": 50, "CHF": 50, "CZK": 1500,
	"DKK": 250, "EUR": 50, "GBP": 30, "HKD": 400, "HUF": 17500, "INR": 50, "JPY": 50, "MXN": 1000,
	"MYR": 200, "NOK": 300, "NZD": 50, "PLN": 200, "RON": 200, "SEK": 300, "SGD": 50, "THB": 1000,
}

// MinimumAmount implements supporters.PaymentProvider.
func (p *Provider) MinimumAmount(currency string) (int64, bool) {
	n, ok := minimumAmounts[strings.ToUpper(currency)]
	return n, ok
}

// decimal formats an amount in Stripe's minor units of currency as a decimal in major units:
// -1250 USD is "-12.50", 1000 JPY is "1000".
func decimal(minor int64, currency string) string {
	exp := exponent(currency)
	s := strconv.FormatInt(minor, 10)
	sign := ""
	if rest, ok := strings.CutPrefix(s, "-"); ok {
		sign, s = "-", rest
	}
	if exp > 0 {
		for len(s) <= exp {
			s = "0" + s
		}
		s = s[:len(s)-exp] + "." + s[len(s)-exp:]
	}
	return sign + s
}
