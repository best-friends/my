// SPDX-License-Identifier: AGPL-3.0-only

// Package gcp is the finance.BillingSource adapter of Google Cloud (ADR-0068 §7). Its billing
// months are the invoice months of Cloud Billing, US Pacific calendar months, and a month is final
// by the rule of finance.ClosedByRule, since Google Cloud has no mark of its own.
//
// The method of billing_gcp_method = bigquery is BigQuerySource: it sums the standard usage cost
// export of Cloud Billing to BigQuery with one jobs.query of the BigQuery REST API. The method of
// billing_gcp_method = budget is BudgetSource: it pulls the notifications of a monthly budget from
// a Pub/Sub subscription and reports the estimated total of each month. Both have the same Name,
// Location, and final rule (the functions of this file).
package gcp

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	appfinance "github.com/mycroblog/my/internal/app/finance"
	"github.com/mycroblog/my/internal/domain/finance"
)

// The methods of billing_gcp_method.
const (
	// MethodBigQuery sums the Cloud Billing export to BigQuery (the default, ADR-0068 §7).
	MethodBigQuery = "bigquery"
	// MethodBudget reads the budget notifications of a Pub/Sub subscription (ADR-0068 §7).
	MethodBudget = "budget"
)

// Name is the source's identifier of billing_sources.
const Name = "gcp"

// Pacific is the time zone of the invoice months of Cloud Billing.
const Pacific = "America/Los_Angeles"

// Location returns the time zone of the invoice months: US Pacific time, with daylight saving.
func Location() (*time.Location, error) {
	loc, err := time.LoadLocation(Pacific)
	if err != nil {
		return nil, fmt.Errorf("billing gcp: load the time zone %s: %w", Pacific, err)
	}
	return loc, nil
}

// serviceCategories is the table of the known services of the export's service.description;
// the others are finance.CategoryOther.
var serviceCategories = map[string]finance.Category{
	"Compute Engine":                     finance.CategoryCompute,
	"Cloud Run":                          finance.CategoryCompute,
	"Cloud Run Functions":                finance.CategoryCompute,
	"Cloud Functions":                    finance.CategoryCompute,
	"Kubernetes Engine":                  finance.CategoryCompute,
	"App Engine":                         finance.CategoryCompute,
	"Cloud Tasks":                        finance.CategoryCompute,
	"Cloud Scheduler":                    finance.CategoryCompute,
	"Cloud Pub/Sub":                      finance.CategoryCompute,
	"Cloud Build":                        finance.CategoryCompute,
	"Cloud SQL":                          finance.CategoryDatabase,
	"AlloyDB":                            finance.CategoryDatabase,
	"Cloud Spanner":                      finance.CategoryDatabase,
	"Cloud Firestore":                    finance.CategoryDatabase,
	"Cloud Memorystore for Redis":        finance.CategoryDatabase,
	"Memorystore for Valkey":             finance.CategoryDatabase,
	"Cloud Storage":                      finance.CategoryStorage,
	"Artifact Registry":                  finance.CategoryStorage,
	"Cloud Filestore":                    finance.CategoryStorage,
	"Networking":                         finance.CategoryNetwork,
	"Cloud CDN":                          finance.CategoryNetwork,
	"Cloud Load Balancing":               finance.CategoryNetwork,
	"Cloud DNS":                          finance.CategoryDomain,
	"Cloud Domains":                      finance.CategoryDomain,
	"Secret Manager":                     finance.CategorySoftware,
	"Cloud Key Management Service (KMS)": finance.CategorySoftware,
	"Cloud Logging":                      finance.CategorySoftware,
	"Cloud Monitoring":                   finance.CategorySoftware,
	"Cloud Trace":                        finance.CategorySoftware,
}

// CategoryOf returns the ADR-0067 category of a service and charge type: the table's for the
// known services, finance.CategoryOther for the unknown ones and the taxes.
func CategoryOf(service string, charge finance.ChargeType) finance.Category {
	if c, ok := serviceCategories[service]; ok && charge != finance.ChargeTax {
		return c
	}
	return finance.CategoryOther
}

// ChargeTypeOf maps a cost_type of the export to its charge type (ADR-0068 §7). The credits are
// not a cost_type: they are the credits[] of the rows, finance.ChargeCredit.
func ChargeTypeOf(costType string) finance.ChargeType {
	switch strings.ToLower(strings.TrimSpace(costType)) {
	case "regular", "":
		return finance.ChargeUsage
	case "tax":
		return finance.ChargeTax
	case "adjustment", "rounding error":
		return finance.ChargeAdjustment
	default:
		return finance.ChargeOther
	}
}

// Final reports whether the invoice month is final on today (in Pacific time): from the
// ClosingDay of the next month on (ADR-0068 §3).
func Final(month finance.YearMonth, today finance.Date) bool {
	return appfinance.ClosedByRule(month, today)
}

var (
	projectIDRe = regexp.MustCompile(`^([a-z][a-z0-9.-]*[a-z0-9]:)?[a-z][a-z0-9-]{4,28}[a-z0-9]$`)
	datasetRe   = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
	tableRe     = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
)

// Table is a BigQuery table, project.dataset.table.
type Table struct {
	Project string
	Dataset string
	Table   string
}

// String returns project.dataset.table.
func (t Table) String() string { return t.Project + "." + t.Dataset + "." + t.Table }

// ParseTable parses project.dataset.table; the project may be domain-scoped (example.com:project).
func ParseTable(v string) (Table, error) {
	v = strings.TrimSpace(v)
	i := strings.LastIndex(v, ".")
	if i < 0 {
		return Table{}, fmt.Errorf("%q is not project.dataset.table, such as my-project.billing_export.gcp_billing_export_v1_012345_6789AB_CDEF01", v)
	}
	rest, table := v[:i], v[i+1:]
	j := strings.LastIndex(rest, ".")
	if j < 0 {
		return Table{}, fmt.Errorf("%q is not project.dataset.table, such as my-project.billing_export.gcp_billing_export_v1_012345_6789AB_CDEF01", v)
	}
	t := Table{Project: rest[:j], Dataset: rest[j+1:], Table: table}
	if !projectIDRe.MatchString(t.Project) {
		return Table{}, fmt.Errorf("%q in %q is not a Google Cloud project ID", t.Project, v)
	}
	if !datasetRe.MatchString(t.Dataset) {
		return Table{}, fmt.Errorf("%q in %q is not a BigQuery dataset name", t.Dataset, v)
	}
	if !tableRe.MatchString(t.Table) {
		return Table{}, fmt.Errorf("%q in %q is not a BigQuery table name", t.Table, v)
	}
	return t, nil
}

// CheckTable accepts project.dataset.table.
func CheckTable(v string) error {
	_, err := ParseTable(v)
	return err
}

// CheckProject accepts a Google Cloud project ID.
func CheckProject(v string) error {
	if !projectIDRe.MatchString(strings.TrimSpace(v)) {
		return fmt.Errorf("%q is not a Google Cloud project ID, such as my-project", v)
	}
	return nil
}

// CheckProjects accepts comma-separated Google Cloud project IDs, each once.
func CheckProjects(v string) error {
	ids := SplitProjects(v)
	if len(ids) == 0 {
		return errors.New("must list at least one Google Cloud project ID")
	}
	for i, id := range ids {
		if err := CheckProject(id); err != nil {
			return err
		}
		if slices.Contains(ids[:i], id) {
			return fmt.Errorf("%q is listed twice", id)
		}
	}
	return nil
}

// SplitProjects splits comma-separated project IDs, dropping the blanks.
func SplitProjects(v string) []string {
	var ids []string
	for s := range strings.SplitSeq(v, ",") {
		if s = strings.TrimSpace(s); s != "" {
			ids = append(ids, s)
		}
	}
	return ids
}

// CheckLabel accepts a key=value with a non-blank key.
func CheckLabel(v string) error {
	_, _, err := splitLabel(v)
	return err
}

func splitLabel(v string) (key, value string, err error) {
	key, value, ok := strings.Cut(v, "=")
	if !ok || strings.TrimSpace(key) == "" {
		return "", "", fmt.Errorf("%q is not key=value, such as app=my", v)
	}
	return strings.TrimSpace(key), strings.TrimSpace(value), nil
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", appfinance.ErrInvalidBillingResponse, fmt.Sprintf(format, args...))
}
