// SPDX-License-Identifier: AGPL-3.0-only

package finance

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// ErrInvalidCurrency reports a currency that is not an ISO 4217 code with minor units.
var ErrInvalidCurrency = errors.New("finance: invalid currency")

// Currency is an ISO 4217 alphabetic code, such as JPY or USD (ADR-0067 §6).
type Currency string

// minorUnits are the digits after the decimal point of the ISO 4217 currencies (list one, the
// current currency and funds codes). The codes without minor units (N.A.: the precious metals,
// the SDR, the bond market units, XSU, XUA, XTS, XXX) cannot price a cost and are left out.
var minorUnits = map[Currency]int{
	"AED": 2, "AFN": 2, "ALL": 2, "AMD": 2, "ANG": 2, "AOA": 2, "ARS": 2, "AUD": 2, "AWG": 2, "AZN": 2,
	"BAM": 2, "BBD": 2, "BDT": 2, "BGN": 2, "BHD": 3, "BIF": 0, "BMD": 2, "BND": 2, "BOB": 2, "BOV": 2,
	"BRL": 2, "BSD": 2, "BTN": 2, "BWP": 2, "BYN": 2, "BZD": 2,
	"CAD": 2, "CDF": 2, "CHE": 2, "CHF": 2, "CHW": 2, "CLF": 4, "CLP": 0, "CNY": 2, "COP": 2, "COU": 2,
	"CRC": 2, "CUP": 2, "CVE": 2, "CZK": 2,
	"DJF": 0, "DKK": 2, "DOP": 2, "DZD": 2,
	"EGP": 2, "ERN": 2, "ETB": 2, "EUR": 2,
	"FJD": 2, "FKP": 2,
	"GBP": 2, "GEL": 2, "GHS": 2, "GIP": 2, "GMD": 2, "GNF": 0, "GTQ": 2, "GYD": 2,
	"HKD": 2, "HNL": 2, "HTG": 2, "HUF": 2,
	"IDR": 2, "ILS": 2, "INR": 2, "IQD": 3, "IRR": 2, "ISK": 0,
	"JMD": 2, "JOD": 3, "JPY": 0,
	"KES": 2, "KGS": 2, "KHR": 2, "KMF": 0, "KPW": 2, "KRW": 0, "KWD": 3, "KYD": 2, "KZT": 2,
	"LAK": 2, "LBP": 2, "LKR": 2, "LRD": 2, "LSL": 2, "LYD": 3,
	"MAD": 2, "MDL": 2, "MGA": 2, "MKD": 2, "MMK": 2, "MNT": 2, "MOP": 2, "MRU": 2, "MUR": 2, "MVR": 2,
	"MWK": 2, "MXN": 2, "MXV": 2, "MYR": 2, "MZN": 2,
	"NAD": 2, "NGN": 2, "NIO": 2, "NOK": 2, "NPR": 2, "NZD": 2,
	"OMR": 3,
	"PAB": 2, "PEN": 2, "PGK": 2, "PHP": 2, "PKR": 2, "PLN": 2, "PYG": 0,
	"QAR": 2,
	"RON": 2, "RSD": 2, "RUB": 2, "RWF": 0,
	"SAR": 2, "SBD": 2, "SCR": 2, "SDG": 2, "SEK": 2, "SGD": 2, "SHP": 2, "SLE": 2, "SOS": 2, "SRD": 2,
	"SSP": 2, "STN": 2, "SVC": 2, "SYP": 2, "SZL": 2,
	"THB": 2, "TJS": 2, "TMT": 2, "TND": 3, "TOP": 2, "TRY": 2, "TTD": 2, "TWD": 2, "TZS": 2,
	"UAH": 2, "UGX": 0, "USD": 2, "USN": 2, "UYI": 0, "UYU": 2, "UYW": 4, "UZS": 2,
	"VED": 2, "VES": 2, "VND": 0, "VUV": 0,
	"WST": 2,
	"XAF": 0, "XCD": 2, "XCG": 2, "XOF": 0, "XPF": 0,
	"YER": 2,
	"ZAR": 2, "ZMW": 2, "ZWG": 2,
}

// ParseCurrency returns the currency of code; it fails with ErrInvalidCurrency for a code that
// is not in the table (codes are upper case).
func ParseCurrency(code string) (Currency, error) {
	c := Currency(code)
	if _, ok := minorUnits[c]; !ok {
		return "", fmt.Errorf("%w: %q", ErrInvalidCurrency, code)
	}
	return c, nil
}

// Valid reports whether c is an ISO 4217 code with minor units.
func (c Currency) Valid() bool {
	_, ok := minorUnits[c]
	return ok
}

// MinorUnits returns the digits after the decimal point of c (0 for JPY, 2 for USD), and false
// for an unknown currency.
func (c Currency) MinorUnits() (int, bool) {
	n, ok := minorUnits[c]
	return n, ok
}

// Currencies returns the codes of the table, sorted.
func Currencies() []Currency {
	out := make([]Currency, 0, len(minorUnits))
	for c := range minorUnits {
		out = append(out, c)
	}
	slices.Sort(out)
	return out
}

// Money is an amount in the minor unit of its currency (1 for 1 yen, 1 for 1 cent; ADR-0067 §6).
type Money struct {
	Minor    int64
	Currency Currency
}

// ErrTooManyDigits reports an amount with more digits after the decimal point than its currency
// has (ADR-0067 §9).
var ErrTooManyDigits = errors.New("finance: more digits than the currency has")

// ParseMoney parses an amount that a person enters: a decimal in major units with at most the
// digits of its currency ("12.34" USD, "1650" JPY), never rounded. It fails with
// ErrInvalidCurrency, ErrInvalidDecimal, and ErrTooManyDigits.
func ParseMoney(amount, currency string) (Money, error) {
	c, err := ParseCurrency(currency)
	if err != nil {
		return Money{}, err
	}
	if _, err := ParseDecimal(amount); err != nil {
		return Money{}, err
	}
	digits, _ := c.MinorUnits()
	if _, frac, ok := strings.Cut(amount, "."); ok && len(frac) > digits {
		return Money{}, fmt.Errorf("%w: %q in %s", ErrTooManyDigits, amount, c)
	}
	minor, err := MinorUnitsOf(amount, c)
	if err != nil {
		return Money{}, err
	}
	return Money{Minor: minor, Currency: c}, nil
}
