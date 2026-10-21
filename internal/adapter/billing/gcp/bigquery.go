// SPDX-License-Identifier: AGPL-3.0-only

package gcp

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"google.golang.org/api/bigquery/v2"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"

	appfinance "github.com/best-friends/my/internal/app/finance"
	"github.com/best-friends/my/internal/domain/finance"
)

// DefaultMaxBytesBilled is the default of billing_gcp_max_bytes_billed: 1 GiB (ADR-0068 §4).
const DefaultMaxBytesBilled int64 = 1 << 30

// queryTimeout is how long jobs.query and jobs.getQueryResults wait for the job, and maxPolls
// how many times the Source asks for the results of a job that has not completed.
const (
	queryTimeout = 20 * time.Second
	maxPolls     = 10
)

// BigQueryOptions configure a BigQuerySource.
type BigQueryOptions struct {
	// Table is the standard usage cost export, project.dataset.table
	// (gcp_billing_export_v1_<BILLING_ACCOUNT_ID>).
	Table string
	// QueryProject is the project that runs (and pays for) the query; the table's project when
	// empty.
	QueryProject string
	// Projects, when not empty, keeps the rows of these project IDs only.
	Projects []string
	// Label, when not empty, is a key=value that keeps the rows carrying that label only.
	Label string
	// MaxBytesBilled is the maximumBytesBilled of the query; DefaultMaxBytesBilled when 0.
	MaxBytesBilled int64
	// Now returns the current time; time.Now when nil.
	Now func() time.Time
}

// BigQuerySource is the BillingSource of Google Cloud by the Cloud Billing export to BigQuery
// (billing_gcp_method = bigquery, ADR-0068 §7). One fetch is one jobs.query, with the calls to
// the pages of its results: each call counts as a request against FetchRequest.MaxRequests.
type BigQuerySource struct {
	jobs         *bigquery.JobsService
	table        Table
	queryProject string
	projects     []string
	labelKey     string
	labelValue   string
	maxBytes     int64
	loc          *time.Location
	now          func() time.Time
	sql          string
}

var _ appfinance.BillingSource = (*BigQuerySource)(nil)

// NewBigQuery returns the BigQuerySource of o on the BigQuery REST API of opts: the Application
// Default Credentials and Google's endpoint when opts is empty. The BigQuery API is the operator's
// own cloud, so its client does not go through the SSRF-guarded kernel.HTTPClient. The client
// does not retry: a call that fails ends the fetch, and the job's retry (ADR-0004) tries again.
func NewBigQuery(ctx context.Context, o BigQueryOptions, opts ...option.ClientOption) (*BigQuerySource, error) {
	s, err := newBigQuerySource(o)
	if err != nil {
		return nil, err
	}
	opts = append([]option.ClientOption{option.WithScopes(bigquery.BigqueryScope)}, opts...)
	svc, err := bigquery.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("billing gcp: the BigQuery client: %w", err)
	}
	s.jobs = svc.Jobs
	return s, nil
}

func newBigQuerySource(o BigQueryOptions) (*BigQuerySource, error) {
	table, err := ParseTable(o.Table)
	if err != nil {
		return nil, fmt.Errorf("billing gcp: table: %w", err)
	}
	s := &BigQuerySource{table: table, queryProject: cmp.Or(strings.TrimSpace(o.QueryProject), table.Project), maxBytes: o.MaxBytesBilled, now: o.Now}
	if err := CheckProject(s.queryProject); err != nil {
		return nil, fmt.Errorf("billing gcp: query project: %w", err)
	}
	if len(o.Projects) > 0 {
		if err := CheckProjects(strings.Join(o.Projects, ",")); err != nil {
			return nil, fmt.Errorf("billing gcp: projects: %w", err)
		}
		s.projects = slices.Clone(o.Projects)
	}
	if o.Label != "" {
		if s.labelKey, s.labelValue, err = splitLabel(o.Label); err != nil {
			return nil, fmt.Errorf("billing gcp: label: %w", err)
		}
	}
	if s.maxBytes < 0 {
		return nil, fmt.Errorf("billing gcp: max bytes billed %d is negative", s.maxBytes)
	}
	if s.maxBytes == 0 {
		s.maxBytes = DefaultMaxBytesBilled
	}
	if s.now == nil {
		s.now = time.Now
	}
	if s.loc, err = Location(); err != nil {
		return nil, err
	}
	s.sql = s.query()
	return s, nil
}

// query is the standard SQL of the fetch: per invoice month, service, cost type, and currency,
// the sums of the costs and of the credits, as NUMERIC to avoid the error of FLOAT64 and as
// STRING to read them without a float, and the end of the last usage, in microseconds. The
// months, projects, and label are query parameters; the table is checked by ParseTable.
func (s *BigQuerySource) query() string {
	var b strings.Builder
	b.WriteString("SELECT invoice.month AS invoice_month, service.description AS service, cost_type, currency,\n")
	b.WriteString("  CAST(SUM(CAST(cost AS NUMERIC)) AS STRING) AS cost,\n")
	b.WriteString("  CAST(SUM(IFNULL((SELECT SUM(CAST(c.amount AS NUMERIC)) FROM UNNEST(credits) AS c), 0)) AS STRING) AS credits,\n")
	b.WriteString("  CAST(UNIX_MICROS(MAX(usage_end_time)) AS STRING) AS usage_end\n")
	b.WriteString("FROM `" + s.table.String() + "`\n")
	b.WriteString("WHERE invoice.month IN UNNEST(@months)\n")
	if len(s.projects) > 0 {
		b.WriteString("  AND project.id IN UNNEST(@projects)\n")
	}
	if s.labelKey != "" {
		b.WriteString("  AND EXISTS (SELECT 1 FROM UNNEST(labels) AS l WHERE l.key = @label_key AND l.value = @label_value)\n")
	}
	b.WriteString("GROUP BY invoice_month, service, cost_type, currency\n")
	b.WriteString("ORDER BY invoice_month, service, cost_type, currency")
	return b.String()
}

// Name implements appfinance.BillingSource.
func (s *BigQuerySource) Name() string { return Name }

// Location implements appfinance.BillingSource: the invoice months are US Pacific months.
func (s *BigQuerySource) Location() *time.Location { return s.loc }

// Fetch implements appfinance.BillingSource. It runs one query of the months that have begun,
// with maximumBytesBilled, and reads the pages of its results, counting each call as a request
// and stopping with ErrRequestCap rather than make one beyond req.MaxRequests, keeping no line.
func (s *BigQuerySource) Fetch(ctx context.Context, req appfinance.FetchRequest) (appfinance.Snapshot, error) {
	var snap appfinance.Snapshot
	if err := ctx.Err(); err != nil {
		return snap, err
	}
	if req.MaxRequests <= 0 {
		return snap, fmt.Errorf("%w: no billed request is left", appfinance.ErrRequestCap)
	}
	today := finance.DateOf(s.now().In(s.loc))
	var months []finance.YearMonth
	for _, m := range req.Months {
		if !today.Before(m.Range().Start) && !slices.Contains(months, m) {
			months = append(months, m)
		}
	}
	if len(months) == 0 {
		// Every month asked for is in the future: it has no cost yet.
		return snap, nil
	}
	slices.SortFunc(months, finance.YearMonth.Compare)
	acc := newBigQueryAccumulator(months, today, s.loc)
	call := func(do func() error) error {
		if snap.Requests >= req.MaxRequests {
			return fmt.Errorf("%w: %d calls made, more remain", appfinance.ErrRequestCap, snap.Requests)
		}
		snap.Requests++
		if err := do(); err != nil {
			return classify(ctx, err)
		}
		return nil
	}
	fail := func(err error) (appfinance.Snapshot, error) {
		return appfinance.Snapshot{Requests: snap.Requests, BytesBilled: snap.BytesBilled}, err
	}

	var res *bigquery.QueryResponse
	if err := call(func() (err error) {
		res, err = s.jobs.Query(s.queryProject, s.request(months)).Context(ctx).Do()
		return err
	}); err != nil {
		return fail(err)
	}
	snap.BytesBilled = res.TotalBytesBilled
	rows, token, complete, job := res.Rows, res.PageToken, res.JobComplete, res.JobReference
	for polls := 0; !complete; polls++ {
		if job == nil || job.JobId == "" {
			return fail(invalid("an incomplete query without a job reference"))
		}
		if polls >= maxPolls {
			return fail(fmt.Errorf("%w: the query job %s did not complete", appfinance.ErrBillingUnavailable, job.JobId))
		}
		var page *bigquery.GetQueryResultsResponse
		if err := call(func() (err error) {
			page, err = s.results(job, "").Context(ctx).Do()
			return err
		}); err != nil {
			return fail(err)
		}
		rows, token, complete = page.Rows, page.PageToken, page.JobComplete
		if complete {
			// The bytes billed of a job that jobs.query left running are in its statistics.
			var j *bigquery.Job
			if err := call(func() (err error) {
				j, err = s.jobs.Get(job.ProjectId, job.JobId).Location(job.Location).Context(ctx).Do()
				return err
			}); err != nil {
				return fail(err)
			}
			if j.Statistics != nil && j.Statistics.Query != nil {
				snap.BytesBilled = j.Statistics.Query.TotalBytesBilled
			}
		}
	}
	for {
		if err := acc.add(rows); err != nil {
			return fail(err)
		}
		if token == "" {
			break
		}
		if job == nil || job.JobId == "" {
			return fail(invalid("a page token without a job reference"))
		}
		var page *bigquery.GetQueryResultsResponse
		if err := call(func() (err error) {
			page, err = s.results(job, token).Context(ctx).Do()
			return err
		}); err != nil {
			return fail(err)
		}
		rows, token = page.Rows, page.PageToken
	}
	snap.Lines = acc.lines()
	return snap, nil
}

func (s *BigQuerySource) results(job *bigquery.JobReference, token string) *bigquery.JobsGetQueryResultsCall {
	call := s.jobs.GetQueryResults(cmp.Or(job.ProjectId, s.queryProject), job.JobId).
		TimeoutMs(queryTimeout.Milliseconds())
	if job.Location != "" {
		call = call.Location(job.Location)
	}
	if token != "" {
		call = call.PageToken(token)
	}
	return call
}

func (s *BigQuerySource) request(months []finance.YearMonth) *bigquery.QueryRequest {
	invoiceMonths := make([]*bigquery.QueryParameterValue, 0, len(months))
	for _, m := range months {
		invoiceMonths = append(invoiceMonths, &bigquery.QueryParameterValue{Value: InvoiceMonth(m)})
	}
	params := []*bigquery.QueryParameter{stringArray("months", invoiceMonths)}
	if len(s.projects) > 0 {
		projects := make([]*bigquery.QueryParameterValue, 0, len(s.projects))
		for _, p := range s.projects {
			projects = append(projects, &bigquery.QueryParameterValue{Value: p})
		}
		params = append(params, stringArray("projects", projects))
	}
	if s.labelKey != "" {
		params = append(params, stringParam("label_key", s.labelKey), stringParam("label_value", s.labelValue))
	}
	return &bigquery.QueryRequest{
		Query:              s.sql,
		UseLegacySql:       new(bool),
		ParameterMode:      "NAMED",
		QueryParameters:    params,
		MaximumBytesBilled: s.maxBytes,
		TimeoutMs:          queryTimeout.Milliseconds(),
	}
}

func stringArray(name string, values []*bigquery.QueryParameterValue) *bigquery.QueryParameter {
	return &bigquery.QueryParameter{
		Name:           name,
		ParameterType:  &bigquery.QueryParameterType{Type: "ARRAY", ArrayType: &bigquery.QueryParameterType{Type: "STRING"}},
		ParameterValue: &bigquery.QueryParameterValue{ArrayValues: values},
	}
}

func stringParam(name, value string) *bigquery.QueryParameter {
	return &bigquery.QueryParameter{
		Name:           name,
		ParameterType:  &bigquery.QueryParameterType{Type: "STRING"},
		ParameterValue: &bigquery.QueryParameterValue{Value: value, ForceSendFields: []string{"Value"}},
	}
}

// InvoiceMonth returns the invoice.month of the export of m: YYYYMM.
func InvoiceMonth(m finance.YearMonth) string { return fmt.Sprintf("%04d%02d", m.Year, int(m.Month)) }

// parseInvoiceMonth parses an invoice.month, YYYYMM.
func parseInvoiceMonth(v string) (finance.YearMonth, error) {
	if len(v) != 6 {
		return finance.YearMonth{}, fmt.Errorf("invoice month %q is not YYYYMM", v)
	}
	return finance.ParseYearMonth(v[:4] + "-" + v[4:])
}

// classify wraps an error of the BigQuery API in the error of its kind (ADR-0068 §9).
func classify(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
		return fmt.Errorf("billing gcp: %w", ctxErr)
	}
	var api *googleapi.Error
	if errors.As(err, &api) {
		reasons := make([]string, 0, len(api.Errors))
		for _, e := range api.Errors {
			reasons = append(reasons, e.Reason)
		}
		has := func(rs ...string) bool {
			for _, r := range rs {
				if slices.Contains(reasons, r) {
					return true
				}
			}
			return false
		}
		switch {
		case has("bytesBilledLimitExceeded"):
			return fmt.Errorf("%w: the query scans more than billing_gcp_max_bytes_billed: %w", appfinance.ErrBytesCap, err)
		case has("rateLimitExceeded", "quotaExceeded") || api.Code == http.StatusTooManyRequests:
			return fmt.Errorf("%w: %w", appfinance.ErrBillingThrottled, err)
		case has("accessNotConfigured", "billingNotEnabled", "projectDisabled"):
			return fmt.Errorf("%w: the BigQuery API is not enabled for the query project: %w", appfinance.ErrBillingNotEnabled, err)
		case api.Code == http.StatusUnauthorized || api.Code == http.StatusForbidden:
			return fmt.Errorf("%w: %w", appfinance.ErrBillingPermission, err)
		case api.Code == http.StatusNotFound || has("notFound"):
			return fmt.Errorf("%w: the dataset or table of billing_gcp_table does not exist: %w", appfinance.ErrBillingNotFound, err)
		case api.Code >= 500 || has("backendError", "internalError"):
			return fmt.Errorf("%w: %w", appfinance.ErrBillingUnavailable, err)
		}
		return fmt.Errorf("%w: %w", appfinance.ErrInvalidBillingResponse, err)
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return fmt.Errorf("%w: %w", appfinance.ErrBillingUnavailable, err)
	}
	// The credentials could not be retrieved, the request could not be sent, or the answer could
	// not be read.
	return fmt.Errorf("%w: %w", appfinance.ErrBillingUnavailable, err)
}

// bigQueryAccumulator sums the result rows by (month, service, charge type): several cost
// types map to one charge type, and the credits of a service's rows make one credit line.
type bigQueryAccumulator struct {
	months   []finance.YearMonth
	today    finance.Date
	loc      *time.Location
	sums     map[lineKey]*lineSum
	currency map[finance.YearMonth]string
	usageEnd map[finance.YearMonth]int64
}

type lineKey struct {
	month   finance.YearMonth
	service string
	charge  finance.ChargeType
}

type lineSum struct {
	amount *big.Rat
	raw    string // the amount as answered while one row makes the line
	rows   int
	scale  int
}

func newBigQueryAccumulator(months []finance.YearMonth, today finance.Date, loc *time.Location) *bigQueryAccumulator {
	return &bigQueryAccumulator{
		months: months, today: today, loc: loc, sums: map[lineKey]*lineSum{},
		currency: map[finance.YearMonth]string{}, usageEnd: map[finance.YearMonth]int64{},
	}
}

// columns are the result columns of the query, in order.
const columns = 7

func cell(row *bigquery.TableRow, i int) (string, bool, error) {
	if row.F[i] == nil {
		return "", false, nil
	}
	switch v := row.F[i].V.(type) {
	case nil:
		return "", false, nil
	case string:
		return v, true, nil
	default:
		return "", false, invalid("column %d is %T, want a string", i, v)
	}
}

func (a *bigQueryAccumulator) add(rows []*bigquery.TableRow) error {
	for _, row := range rows {
		if row == nil || len(row.F) != columns {
			return invalid("a row that is not of %d columns", columns)
		}
		var vals [columns]string
		var ok [columns]bool
		for i := range columns {
			v, present, err := cell(row, i)
			if err != nil {
				return err
			}
			vals[i], ok[i] = v, present
		}
		month, err := parseInvoiceMonth(vals[0])
		if err != nil {
			return invalid("%v", err)
		}
		if !slices.Contains(a.months, month) {
			// The query asks for the months only; a row of another is not usable.
			return invalid("a row of invoice month %s, which was not asked for", month)
		}
		service := strings.TrimSpace(vals[1])
		if service == "" {
			return invalid("a row of %s without a service", month)
		}
		currency := vals[3]
		if _, err := finance.ParseCurrency(currency); err != nil {
			return invalid("%s %s: currency %q", month, service, currency)
		}
		if c, seen := a.currency[month]; seen && c != currency {
			return invalid("%s mixes the currencies %s and %s", month, c, currency)
		}
		a.currency[month] = currency
		if ok[6] {
			micros, err := strconv.ParseInt(vals[6], 10, 64)
			if err != nil {
				return invalid("%s %s: usage end %q", month, service, vals[6])
			}
			a.usageEnd[month] = max(a.usageEnd[month], micros)
		}
		if err := a.sum(lineKey{month, service, ChargeTypeOf(vals[2])}, cmp.Or(vals[4], "0")); err != nil {
			return err
		}
		if err := a.sum(lineKey{month, service, finance.ChargeCredit}, cmp.Or(vals[5], "0")); err != nil {
			return err
		}
	}
	return nil
}

func (a *bigQueryAccumulator) sum(k lineKey, raw string) error {
	amount, err := finance.ParseDecimal(raw)
	if err != nil {
		return invalid("%s %s %s: amount %q", k.month, k.service, k.charge, raw)
	}
	if amount.Sign() == 0 {
		return nil
	}
	s, ok := a.sums[k]
	if !ok {
		s = &lineSum{amount: new(big.Rat)}
		a.sums[k] = s
	}
	s.amount.Add(s.amount, amount)
	s.raw = raw
	s.rows++
	if _, frac, ok := strings.Cut(raw, "."); ok {
		s.scale = max(s.scale, len(frac))
	}
	return nil
}

// lines returns the summed lines whose sum is not 0. A final month covers its whole range; an
// open one its first day through the day of its last usage (in Pacific time).
func (a *bigQueryAccumulator) lines() []appfinance.CostLine {
	out := make([]appfinance.CostLine, 0, len(a.sums))
	for k, sum := range a.sums {
		if sum.amount.Sign() == 0 {
			continue
		}
		final := Final(k.month, a.today)
		covered := k.month.Range()
		if !final {
			covered.End = a.through(k.month, covered)
		}
		amount := sum.raw
		if sum.rows > 1 {
			// The sum of decimals of at most n fraction digits has at most n: exact.
			amount = sum.amount.FloatString(sum.scale)
		}
		out = append(out, appfinance.CostLine{
			Month: k.month, Covered: covered, Service: k.service, ChargeType: k.charge,
			Category: CategoryOf(k.service, k.charge), Amount: amount, Currency: a.currency[k.month], Final: final,
		})
	}
	slices.SortFunc(out, func(x, y appfinance.CostLine) int {
		return cmp.Or(x.Month.Compare(y.Month), cmp.Compare(x.Service, y.Service), cmp.Compare(x.ChargeType, y.ChargeType))
	})
	return out
}

// through is the last day of an open month with data: the Pacific day of the end of its last
// usage, an end at midnight belonging to the day before, within the month and not after today.
func (a *bigQueryAccumulator) through(m finance.YearMonth, month finance.DateRange) finance.Date {
	end := month.End
	if !a.today.After(end) {
		end = a.today
	}
	micros, ok := a.usageEnd[m]
	if !ok {
		return end
	}
	last := finance.DateOf(time.UnixMicro(micros - 1).In(a.loc))
	switch {
	case last.Before(month.Start):
		return month.Start
	case last.After(end):
		return end
	}
	return last
}
