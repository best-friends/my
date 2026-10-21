// SPDX-License-Identifier: AGPL-3.0-only

// Package aws is the finance.BillingSource adapter of AWS (ADR-0068 §6): it reads the usage
// charges of the billing months from Cost Explorer GetCostAndUsage, one MONTHLY request grouped
// by SERVICE and RECORD_TYPE, page by page against the request cap.
package aws

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math/big"
	"net"
	"regexp"
	"slices"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	cetypes "github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	appfinance "github.com/best-friends/my/internal/app/finance"
	"github.com/best-friends/my/internal/domain/finance"
)

// Region is the region of the Cost Explorer endpoint, ce.us-east-1.amazonaws.com.
const Region = "us-east-1"

// The metrics of billing_aws_metric.
const (
	// MetricNetUnblendedCost is the cost after the discounts, the closest to the invoice (the
	// default, ADR-0068 §6).
	MetricNetUnblendedCost = "NetUnblendedCost"
	// MetricUnblendedCost is the cost before the discounts.
	MetricUnblendedCost = "UnblendedCost"
)

// The filters of billing_aws_filter.
const (
	FilterNone          = "none"
	FilterLinkedAccount = "linked_account"
	FilterTag           = "tag"
)

// Client is the part of *costexplorer.Client that the Source uses.
type Client interface {
	GetCostAndUsage(ctx context.Context, in *costexplorer.GetCostAndUsageInput, optFns ...func(*costexplorer.Options)) (*costexplorer.GetCostAndUsageOutput, error)
}

// Options configure a Source.
type Options struct {
	// Metric is MetricNetUnblendedCost (the default when empty) or MetricUnblendedCost.
	Metric string
	// Filter is FilterNone (the default when empty), FilterLinkedAccount, or FilterTag.
	Filter string
	// LinkedAccounts are the 12-digit account IDs of FilterLinkedAccount.
	LinkedAccounts []string
	// Tag is the key=value of an activated cost allocation tag of FilterTag.
	Tag string
	// Now returns the current time; time.Now when nil.
	Now func() time.Time
}

// Source is the BillingSource of AWS. Its billing months are UTC calendar months.
type Source struct {
	client Client
	metric string
	filter *cetypes.Expression
	now    func() time.Time
}

var _ appfinance.BillingSource = (*Source)(nil)

// New returns the Source of o on client.
func New(client Client, o Options) (*Source, error) {
	if client == nil {
		return nil, errors.New("billing aws: no Cost Explorer client")
	}
	if err := CheckMetric(o.Metric); o.Metric != "" && err != nil {
		return nil, err
	}
	filter, err := filterExpression(o)
	if err != nil {
		return nil, err
	}
	s := &Source{client: client, metric: cmp.Or(o.Metric, MetricNetUnblendedCost), filter: filter, now: o.Now}
	if s.now == nil {
		s.now = time.Now
	}
	return s, nil
}

// NewFromConfig returns the Source of o with the Cost Explorer client of cfg (credentials, HTTP
// client, endpoint) in Region. The SDK does not retry: a request that fails ends the fetch, and
// the job's retry (ADR-0004), which the request cap counts, tries again.
func NewFromConfig(cfg awssdk.Config, o Options) (*Source, error) {
	return New(costexplorer.NewFromConfig(cfg, func(co *costexplorer.Options) {
		co.Region = Region
		co.Retryer = awssdk.NopRetryer{}
	}), o)
}

// CheckMetric accepts the metrics of billing_aws_metric.
func CheckMetric(v string) error {
	if v != MetricNetUnblendedCost && v != MetricUnblendedCost {
		return fmt.Errorf("unknown metric %q; want %s or %s", v, MetricNetUnblendedCost, MetricUnblendedCost)
	}
	return nil
}

var accountIDRe = regexp.MustCompile(`^[0-9]{12}$`)

// CheckLinkedAccounts accepts comma-separated 12-digit AWS account IDs, each once.
func CheckLinkedAccounts(v string) error {
	ids := SplitLinkedAccounts(v)
	if len(ids) == 0 {
		return errors.New("must list at least one 12-digit AWS account ID")
	}
	for i, id := range ids {
		if !accountIDRe.MatchString(id) {
			return fmt.Errorf("%q is not a 12-digit AWS account ID", id)
		}
		if slices.Contains(ids[:i], id) {
			return fmt.Errorf("%q is listed twice", id)
		}
	}
	return nil
}

// SplitLinkedAccounts splits comma-separated account IDs, dropping the blanks.
func SplitLinkedAccounts(v string) []string {
	var ids []string
	for s := range strings.SplitSeq(v, ",") {
		if s = strings.TrimSpace(s); s != "" {
			ids = append(ids, s)
		}
	}
	return ids
}

// CheckTag accepts a key=value with a non-blank key.
func CheckTag(v string) error {
	_, _, err := splitTag(v)
	return err
}

func splitTag(v string) (key, value string, err error) {
	key, value, ok := strings.Cut(v, "=")
	if !ok || strings.TrimSpace(key) == "" {
		return "", "", fmt.Errorf("%q is not key=value, such as app=my", v)
	}
	return strings.TrimSpace(key), strings.TrimSpace(value), nil
}

var roleARNRe = regexp.MustCompile(`^arn:aws[a-z-]*:iam::[0-9]{12}:role/[\w+=,.@/-]+$`)

// CheckRoleARN accepts the ARN of an IAM role, arn:aws:iam::123456789012:role/name.
func CheckRoleARN(v string) error {
	if !roleARNRe.MatchString(v) {
		return fmt.Errorf("%q is not the ARN of an IAM role, such as arn:aws:iam::123456789012:role/my-billing", v)
	}
	return nil
}

func filterExpression(o Options) (*cetypes.Expression, error) {
	switch o.Filter {
	case "", FilterNone:
		return nil, nil
	case FilterLinkedAccount:
		ids := slices.Clone(o.LinkedAccounts)
		if err := CheckLinkedAccounts(strings.Join(ids, ",")); err != nil {
			return nil, fmt.Errorf("billing aws: linked accounts: %w", err)
		}
		return &cetypes.Expression{Dimensions: &cetypes.DimensionValues{
			Key: cetypes.DimensionLinkedAccount, Values: ids,
		}}, nil
	case FilterTag:
		key, value, err := splitTag(o.Tag)
		if err != nil {
			return nil, fmt.Errorf("billing aws: tag: %w", err)
		}
		return &cetypes.Expression{Tags: &cetypes.TagValues{
			Key: awssdk.String(key), Values: []string{value}, MatchOptions: []cetypes.MatchOption{cetypes.MatchOptionEquals},
		}}, nil
	default:
		return nil, fmt.Errorf("billing aws: unknown filter %q; want %s, %s, or %s", o.Filter, FilterNone, FilterLinkedAccount, FilterTag)
	}
}

// Name implements appfinance.BillingSource.
func (s *Source) Name() string { return "aws" }

// Location implements appfinance.BillingSource: Cost Explorer's periods are UTC calendar dates.
func (s *Source) Location() *time.Location { return time.UTC }

// Fetch implements appfinance.BillingSource. It asks for the months from the first day of the
// oldest through today (End is tomorrow, exclusive), counts each page as a billed request, and
// stops with ErrRequestCap rather than take a page beyond req.MaxRequests, keeping no line.
func (s *Source) Fetch(ctx context.Context, req appfinance.FetchRequest) (appfinance.Snapshot, error) {
	var snap appfinance.Snapshot
	if err := ctx.Err(); err != nil {
		return snap, err
	}
	if req.MaxRequests <= 0 {
		return snap, fmt.Errorf("%w: no billed request is left", appfinance.ErrRequestCap)
	}
	if len(req.Months) == 0 {
		return snap, nil
	}
	months := slices.Clone(req.Months)
	slices.SortFunc(months, finance.YearMonth.Compare)
	today := finance.DateOf(s.now().UTC())
	start := months[0].Range().Start
	end := months[len(months)-1].Range().End.AddDays(1)
	if tomorrow := today.AddDays(1); tomorrow.Before(end) {
		end = tomorrow
	}
	if !start.Before(end) {
		// Every month asked for is in the future: it has no cost yet.
		return snap, nil
	}
	in := &costexplorer.GetCostAndUsageInput{
		TimePeriod:  &cetypes.DateInterval{Start: awssdk.String(start.String()), End: awssdk.String(end.String())},
		Granularity: cetypes.GranularityMonthly,
		Metrics:     []string{s.metric},
		GroupBy: []cetypes.GroupDefinition{
			{Type: cetypes.GroupDefinitionTypeDimension, Key: awssdk.String(string(cetypes.DimensionService))},
			{Type: cetypes.GroupDefinitionTypeDimension, Key: awssdk.String(string(cetypes.DimensionRecordType))},
		},
		Filter: s.filter,
	}
	acc := newAccumulator(months, today)
	for {
		if snap.Requests >= req.MaxRequests {
			return appfinance.Snapshot{Requests: snap.Requests}, fmt.Errorf("%w: %d pages taken, more remain", appfinance.ErrRequestCap, snap.Requests)
		}
		snap.Requests++
		out, err := s.client.GetCostAndUsage(ctx, in)
		if err != nil {
			return appfinance.Snapshot{Requests: snap.Requests}, classify(ctx, err)
		}
		for _, r := range out.ResultsByTime {
			if err := acc.add(r, s.metric); err != nil {
				return appfinance.Snapshot{Requests: snap.Requests}, err
			}
		}
		if awssdk.ToString(out.NextPageToken) == "" {
			break
		}
		in.NextPageToken = out.NextPageToken
	}
	snap.Lines = acc.lines()
	return snap, nil
}

// classify wraps an error of GetCostAndUsage in the error of its kind (ADR-0068 §9).
func classify(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
		return fmt.Errorf("billing aws: %w", ctxErr)
	}
	var (
		unavailable *cetypes.DataUnavailableException
		limit       *cetypes.LimitExceededException
		expired     *cetypes.BillExpirationException
		badToken    *cetypes.InvalidNextTokenException
		changed     *cetypes.RequestChangedException
	)
	switch {
	case errors.As(err, &unavailable):
		return fmt.Errorf("%w: Cost Explorer is not enabled for the account, or Linked Account Access is off: %w", appfinance.ErrBillingNotEnabled, err)
	case errors.As(err, &limit):
		return fmt.Errorf("%w: %w", appfinance.ErrBillingThrottled, err)
	case errors.As(err, &expired):
		return fmt.Errorf("%w: the bill of the period has expired: %w", appfinance.ErrBillingNotFound, err)
	case errors.As(err, &badToken), errors.As(err, &changed):
		return fmt.Errorf("%w: %w", appfinance.ErrInvalidBillingResponse, err)
	}
	var api smithy.APIError
	if errors.As(err, &api) {
		switch api.ErrorCode() {
		case "AccessDeniedException", "AccessDenied", "UnrecognizedClientException", "InvalidClientTokenId",
			"ExpiredTokenException", "ExpiredToken", "SignatureDoesNotMatch", "InvalidSignatureException",
			"MissingAuthenticationToken", "IncompleteSignature":
			return fmt.Errorf("%w: %w", appfinance.ErrBillingPermission, err)
		case "ThrottlingException", "Throttling", "TooManyRequestsException":
			return fmt.Errorf("%w: %w", appfinance.ErrBillingThrottled, err)
		}
		if api.ErrorFault() == smithy.FaultServer || serverError(err) {
			return fmt.Errorf("%w: %w", appfinance.ErrBillingUnavailable, err)
		}
		return fmt.Errorf("%w: %w", appfinance.ErrInvalidBillingResponse, err)
	}
	if serverError(err) {
		return fmt.Errorf("%w: %w", appfinance.ErrBillingUnavailable, err)
	}
	var resp *smithyhttp.ResponseError
	if errors.As(err, &resp) {
		// A response that the SDK could not read as an answer or as an error of the API.
		return fmt.Errorf("%w: %w", appfinance.ErrInvalidBillingResponse, err)
	}
	var deser *smithy.DeserializationError
	if errors.As(err, &deser) {
		return fmt.Errorf("%w: %w", appfinance.ErrInvalidBillingResponse, err)
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return fmt.Errorf("%w: %w", appfinance.ErrBillingUnavailable, err)
	}
	// The credentials could not be retrieved, or the request could not be sent.
	return fmt.Errorf("%w: %w", appfinance.ErrBillingUnavailable, err)
}

// serverError reports whether err comes from an HTTP response of status 5xx.
func serverError(err error) bool {
	var resp *smithyhttp.ResponseError
	return errors.As(err, &resp) && resp.HTTPStatusCode() >= 500
}

// ChargeTypeOf maps a RECORD_TYPE of Cost Explorer to its charge type (ADR-0068 §6).
func ChargeTypeOf(recordType string) finance.ChargeType {
	switch strings.ToLower(strings.TrimSpace(recordType)) {
	case "usage", "discountedusage", "savingsplancoveredusage":
		return finance.ChargeUsage
	case "credit":
		return finance.ChargeCredit
	case "refund":
		return finance.ChargeRefund
	case "tax":
		return finance.ChargeTax
	case "support", "support fee":
		return finance.ChargeSupport
	case "rifee", "fee", "recurring reservation fee", "upfront reservation fee",
		"savingsplanrecurringfee", "savingsplanupfrontfee":
		return finance.ChargeFee
	default:
		return finance.ChargeOther
	}
}

// serviceCategories is the table of the known services of Cost Explorer's SERVICE dimension;
// the others are finance.CategoryOther.
var serviceCategories = map[string]finance.Category{
	"Amazon Elastic Compute Cloud - Compute":          finance.CategoryCompute,
	"EC2 - Other":                                     finance.CategoryCompute,
	"AWS Lambda":                                      finance.CategoryCompute,
	"Amazon Elastic Container Service":                finance.CategoryCompute,
	"Amazon Elastic Container Service for Kubernetes": finance.CategoryCompute,
	"AWS Fargate":                                     finance.CategoryCompute,
	"AWS App Runner":                                  finance.CategoryCompute,
	"Amazon Lightsail":                                finance.CategoryCompute,
	"Amazon Simple Queue Service":                     finance.CategoryCompute,
	"Amazon Simple Notification Service":              finance.CategoryCompute,
	"AWS Key Management Service":                      finance.CategorySoftware,
	"AWS Secrets Manager":                             finance.CategorySoftware,
	"AmazonCloudWatch":                                finance.CategorySoftware,
	"AWS X-Ray":                                       finance.CategorySoftware,
	"Amazon Relational Database Service":              finance.CategoryDatabase,
	"Amazon Aurora DSQL":                              finance.CategoryDatabase,
	"Amazon DynamoDB":                                 finance.CategoryDatabase,
	"Amazon ElastiCache":                              finance.CategoryDatabase,
	"Amazon MemoryDB":                                 finance.CategoryDatabase,
	"Amazon Simple Storage Service":                   finance.CategoryStorage,
	"Amazon Elastic File System":                      finance.CategoryStorage,
	"Amazon Elastic Block Store":                      finance.CategoryStorage,
	"Amazon EC2 Container Registry (ECR)":             finance.CategoryStorage,
	"AWS Backup":                                      finance.CategoryStorage,
	"Amazon CloudFront":                               finance.CategoryNetwork,
	"Amazon Virtual Private Cloud":                    finance.CategoryNetwork,
	"Elastic Load Balancing":                          finance.CategoryNetwork,
	"Amazon API Gateway":                              finance.CategoryNetwork,
	"AWS Data Transfer":                               finance.CategoryNetwork,
	"AWS WAF":                                         finance.CategoryNetwork,
	"Amazon Route 53":                                 finance.CategoryDomain,
	"Amazon Registrar":                                finance.CategoryDomain,
	"Amazon Simple Email Service":                     finance.CategoryEmail,
}

// CategoryOf returns the ADR-0067 category of a service and charge type: the table's for the
// known services, finance.CategoryOther for the unknown ones and the taxes.
func CategoryOf(service string, charge finance.ChargeType) finance.Category {
	if c, ok := serviceCategories[service]; ok && charge != finance.ChargeTax {
		return c
	}
	return finance.CategoryOther
}

// accumulator sums the groups of the pages by (month, service, charge type): several record
// types map to one charge type, and a group may come in more than one page.
type accumulator struct {
	months []finance.YearMonth
	today  finance.Date
	order  []lineKey
	sums   map[lineKey]*lineSum
	final  map[finance.YearMonth]bool
}

type lineKey struct {
	month   finance.YearMonth
	service string
	charge  finance.ChargeType
}

type lineSum struct {
	amount   *big.Rat
	raw      string // the amount as answered while one group makes the line
	groups   int
	scale    int
	currency string
}

func newAccumulator(months []finance.YearMonth, today finance.Date) *accumulator {
	return &accumulator{months: months, today: today, sums: map[lineKey]*lineSum{}, final: map[finance.YearMonth]bool{}}
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", appfinance.ErrInvalidBillingResponse, fmt.Sprintf(format, args...))
}

func (a *accumulator) add(r cetypes.ResultByTime, metric string) error {
	if r.TimePeriod == nil {
		return invalid("a result without a time period")
	}
	start, err := finance.ParseDate(awssdk.ToString(r.TimePeriod.Start))
	if err != nil {
		return invalid("time period start %q", awssdk.ToString(r.TimePeriod.Start))
	}
	month := finance.YearMonthOf(start)
	if start != month.Range().Start {
		return invalid("time period %s does not start a month", start)
	}
	if !slices.Contains(a.months, month) {
		return nil
	}
	a.final[month] = !r.Estimated
	for _, g := range r.Groups {
		if len(g.Keys) != 2 || strings.TrimSpace(g.Keys[0]) == "" {
			return invalid("group keys %q, want SERVICE and RECORD_TYPE", g.Keys)
		}
		m, ok := g.Metrics[metric]
		if !ok || m.Amount == nil {
			return invalid("group %q has no %s", g.Keys, metric)
		}
		raw := awssdk.ToString(m.Amount)
		amount, err := finance.ParseDecimal(raw)
		if err != nil {
			return invalid("group %q: amount %q", g.Keys, raw)
		}
		currency := cmp.Or(awssdk.ToString(m.Unit), "USD")
		if _, err := finance.ParseCurrency(currency); err != nil {
			return invalid("group %q: unit %q", g.Keys, currency)
		}
		charge := ChargeTypeOf(g.Keys[1])
		k := lineKey{month: month, service: g.Keys[0], charge: charge}
		sum, ok := a.sums[k]
		if !ok {
			sum = &lineSum{amount: new(big.Rat), currency: currency}
			a.sums[k] = sum
			a.order = append(a.order, k)
		} else if sum.currency != currency {
			return invalid("%s %s %s mixes %s and %s", month, k.service, charge, sum.currency, currency)
		}
		sum.amount.Add(sum.amount, amount)
		sum.raw = raw
		sum.groups++
		if _, frac, ok := strings.Cut(raw, "."); ok {
			sum.scale = max(sum.scale, len(frac))
		}
	}
	return nil
}

// lines returns the summed lines: a final month covers its whole range, an estimated one its
// first day through yesterday (UTC), when Cost Explorer has its data.
func (a *accumulator) lines() []appfinance.CostLine {
	out := make([]appfinance.CostLine, 0, len(a.order))
	for _, k := range a.order {
		sum := a.sums[k]
		final := a.final[k.month]
		covered := k.month.Range()
		if !final {
			if through := a.today.AddDays(-1); through.Before(covered.End) {
				covered.End = through
			}
			if covered.End.Before(covered.Start) {
				covered.End = covered.Start
			}
		}
		amount := sum.raw
		if sum.groups > 1 {
			// The sum of decimals of at most n fraction digits has at most n: exact.
			amount = sum.amount.FloatString(sum.scale)
		}
		out = append(out, appfinance.CostLine{
			Month: k.month, Covered: covered, Service: k.service, ChargeType: k.charge,
			Category: CategoryOf(k.service, k.charge), Amount: amount, Currency: sum.currency, Final: final,
		})
	}
	slices.SortFunc(out, func(x, y appfinance.CostLine) int {
		return cmp.Or(x.Month.Compare(y.Month), cmp.Compare(x.Service, y.Service), cmp.Compare(x.ChargeType, y.ChargeType))
	})
	return out
}
