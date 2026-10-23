// SPDX-License-Identifier: AGPL-3.0-only

package awstest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/mycroblog/my/internal/app/kernel"
)

// The account and region of the fake's ARNs and queue URLs.
const (
	FakeAccount = "000000000000"
	FakeRegion  = "us-east-1"
)

// The limits of Amazon SNS and SQS that the fake enforces (the API references of CreateQueue,
// ReceiveMessage, DeleteMessageBatch, ListQueues, and Publish).
const (
	maxQueueName        = 80
	minRetention        = 60
	maxRetention        = 1209600
	defaultRetention    = 345600
	maxWaitTime         = 20
	maxReceive          = 10
	maxBatch            = 10
	maxListQueues       = 1000
	defaultVisibility   = 30
	maxMessageSize      = 256 * 1024
	queueNameReuseDelay = 60 * time.Second
	subscriptionsPage   = 100
)

var queueNameRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// Fake is Amazon SNS and SQS in memory, for the tests of adapters that publish to an SNS
// topic and receive from the SQS queues subscribed to it. SNS returns its SNS client and SQS its
// SQS client; they have the method signatures of the aws-sdk-go-v2 clients for the operations
// below, so they stand in for *sns.Client and *sqs.Client behind an adapter's narrow interfaces.
//
//   - SNS: CreateTopic, DeleteTopic, Publish, Subscribe (protocol sqs), Unsubscribe,
//     ListSubscriptionsByTopic (100 per page).
//   - SQS: CreateQueue, GetQueueAttributes, ListQueues, ReceiveMessage (long polling),
//     DeleteMessageBatch, DeleteQueue.
//
// It keeps the behavior that such adapters rely on: a topic delivers a message to every queue
// subscribed to it, unchanged with RawMessageDelivery and in the JSON envelope of SNS without
// it, and only to a queue whose policy lets sns.amazonaws.com send it messages from that topic
// (aws:SourceArn); a received message is hidden for the visibility timeout until it is deleted
// and expires after the queue's MessageRetentionPeriod; a queue name cannot be used again for 60
// seconds after the queue is deleted; a long poll returns when a message arrives, when the queue
// is deleted (empty), or at its wait time. Errors are the aws-sdk-go-v2 error types
// (sqstypes.QueueDoesNotExist, snstypes.NotFoundException, ...). Time is the Clock's, except for
// the waits of long polls.
//
// It is safe for concurrent use.
type Fake struct {
	clock kernel.Clock

	mu          sync.Mutex
	topics      map[string]*fakeTopic // by ARN
	queues      map[string]*fakeQueue // by name
	deleted     map[string]time.Time  // queue name -> when it was deleted
	unreachable bool
	seq         int
}

type fakeTopic struct {
	arn  string
	subs []*fakeSubscription
}

type fakeSubscription struct {
	arn, topicARN, endpoint string
	raw                     bool
}

type fakeQueue struct {
	name, url, arn string
	created        time.Time
	attributes     map[string]string
	tags           map[string]string
	messages       []*fakeMessage
	// wake is closed and replaced when a message arrives or the queue is deleted, to end the
	// long polls that wait on it.
	wake chan struct{}
}

type fakeMessage struct {
	id, body  string
	sent      time.Time
	receipt   string
	invisible time.Time // hidden until then
}

// NewFake returns a fake without topics and queues whose time is clock's; nil means the system
// time.
func NewFake(clock kernel.Clock) *Fake {
	if clock == nil {
		clock = systemClock{}
	}
	return &Fake{
		clock:   clock,
		topics:  map[string]*fakeTopic{},
		queues:  map[string]*fakeQueue{},
		deleted: map[string]time.Time{},
	}
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// SNS returns the fake's SNS client.
func (f *Fake) SNS() *FakeSNS { return &FakeSNS{f: f} }

// SQS returns the fake's SQS client.
func (f *Fake) SQS() *FakeSQS { return &FakeSQS{f: f} }

// SetUnreachable makes the fake unreachable, or reachable again: while it is unreachable, every
// call waits for its context to end and returns its error, as with an endpoint that does not
// answer.
func (f *Fake) SetUnreachable(unreachable bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.unreachable = unreachable
}

// QueueTags returns the tags of the queue name, or nil when there is no such queue.
func (f *Fake) QueueTags(name string) map[string]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if q, ok := f.queues[name]; ok {
		return clone(q.tags)
	}
	return nil
}

// begin waits for ctx while the fake is unreachable and returns ctx's error.
func (f *Fake) begin(ctx context.Context) error {
	f.mu.Lock()
	unreachable := f.unreachable
	f.mu.Unlock()
	if unreachable {
		<-ctx.Done()
	}
	return ctx.Err()
}

func (f *Fake) newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	f.seq++
	return hex.EncodeToString(b[:]) + strconv.Itoa(f.seq)
}

func clone(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// --- SNS ----------------------------------------------------------------------------------

// FakeSNS is the SNS client of a Fake.
type FakeSNS struct{ f *Fake }

func snsInvalid(format string, args ...any) error {
	return &snstypes.InvalidParameterException{Message: aws.String(fmt.Sprintf(format, args...))}
}

func snsNotFound(what string) error {
	return &snstypes.NotFoundException{Message: aws.String(what + " does not exist")}
}

// CreateTopic creates a standard topic, or returns the ARN of the existing topic of that name.
func (c *FakeSNS) CreateTopic(ctx context.Context, in *sns.CreateTopicInput, _ ...func(*sns.Options)) (*sns.CreateTopicOutput, error) {
	f := c.f
	if err := f.begin(ctx); err != nil {
		return nil, err
	}
	name := aws.ToString(in.Name)
	if name == "" || len(name) > 256 || !queueNameRe.MatchString(name) {
		return nil, snsInvalid("invalid topic name %q", name)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	arn := "arn:aws:sns:" + FakeRegion + ":" + FakeAccount + ":" + name
	if _, ok := f.topics[arn]; !ok {
		f.topics[arn] = &fakeTopic{arn: arn}
	}
	return &sns.CreateTopicOutput{TopicArn: aws.String(arn)}, nil
}

// DeleteTopic deletes a topic and its subscriptions; a missing topic is not an error.
func (c *FakeSNS) DeleteTopic(ctx context.Context, in *sns.DeleteTopicInput, _ ...func(*sns.Options)) (*sns.DeleteTopicOutput, error) {
	f := c.f
	if err := f.begin(ctx); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.topics, aws.ToString(in.TopicArn))
	return &sns.DeleteTopicOutput{}, nil
}

// Publish delivers the message to the queues subscribed to the topic whose policy allows it.
func (c *FakeSNS) Publish(ctx context.Context, in *sns.PublishInput, _ ...func(*sns.Options)) (*sns.PublishOutput, error) {
	f := c.f
	if err := f.begin(ctx); err != nil {
		return nil, err
	}
	msg := aws.ToString(in.Message)
	if msg == "" {
		return nil, snsInvalid("empty message")
	}
	if len(msg) > maxMessageSize {
		return nil, snsInvalid("message longer than %d bytes", maxMessageSize)
	}
	if in.TargetArn != nil || in.PhoneNumber != nil {
		return nil, snsInvalid("the fake publishes to topics only")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.topics[aws.ToString(in.TopicArn)]
	if !ok {
		return nil, snsNotFound("topic")
	}
	id := f.newID()
	now := f.clock.Now()
	for _, s := range t.subs {
		q := f.queueByARN(s.endpoint)
		if q == nil || !q.allowsSNS(t.arn) {
			continue // SNS drops what it cannot deliver
		}
		body := msg
		if !s.raw {
			envelope, _ := json.Marshal(map[string]string{
				"Type": "Notification", "MessageId": id, "TopicArn": t.arn, "Message": msg,
				"Timestamp": now.UTC().Format(time.RFC3339Nano),
			})
			body = string(envelope)
		}
		q.messages = append(q.messages, &fakeMessage{id: f.newID(), body: body, sent: now})
		q.notify()
	}
	return &sns.PublishOutput{MessageId: aws.String(id)}, nil
}

// Subscribe subscribes a queue (protocol sqs, endpoint the queue's ARN) to a topic. Subscribing
// the same endpoint again returns the existing subscription.
func (c *FakeSNS) Subscribe(ctx context.Context, in *sns.SubscribeInput, _ ...func(*sns.Options)) (*sns.SubscribeOutput, error) {
	f := c.f
	if err := f.begin(ctx); err != nil {
		return nil, err
	}
	if aws.ToString(in.Protocol) != "sqs" {
		return nil, snsInvalid("the fake supports the sqs protocol only")
	}
	endpoint := aws.ToString(in.Endpoint)
	if !strings.HasPrefix(endpoint, "arn:aws:sqs:") {
		return nil, snsInvalid("invalid SQS endpoint %q", endpoint)
	}
	raw := false
	for k, v := range in.Attributes {
		switch k {
		case "RawMessageDelivery":
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, snsInvalid("invalid RawMessageDelivery %q", v)
			}
			raw = b
		default:
			return nil, snsInvalid("the fake does not support the subscription attribute %s", k)
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.topics[aws.ToString(in.TopicArn)]
	if !ok {
		return nil, snsNotFound("topic")
	}
	for _, s := range t.subs {
		if s.endpoint == endpoint {
			s.raw = raw
			return &sns.SubscribeOutput{SubscriptionArn: aws.String(s.arn)}, nil
		}
	}
	s := &fakeSubscription{arn: t.arn + ":" + f.newID(), topicARN: t.arn, endpoint: endpoint, raw: raw}
	t.subs = append(t.subs, s)
	return &sns.SubscribeOutput{SubscriptionArn: aws.String(s.arn)}, nil
}

// Unsubscribe deletes a subscription; a missing one is snstypes.NotFoundException.
func (c *FakeSNS) Unsubscribe(ctx context.Context, in *sns.UnsubscribeInput, _ ...func(*sns.Options)) (*sns.UnsubscribeOutput, error) {
	f := c.f
	if err := f.begin(ctx); err != nil {
		return nil, err
	}
	arn := aws.ToString(in.SubscriptionArn)
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, t := range f.topics {
		for i, s := range t.subs {
			if s.arn == arn {
				t.subs = slices.Delete(t.subs, i, i+1)
				return &sns.UnsubscribeOutput{}, nil
			}
		}
	}
	return nil, snsNotFound("subscription")
}

// ListSubscriptionsByTopic lists the subscriptions of a topic, 100 per page.
func (c *FakeSNS) ListSubscriptionsByTopic(ctx context.Context, in *sns.ListSubscriptionsByTopicInput, _ ...func(*sns.Options)) (*sns.ListSubscriptionsByTopicOutput, error) {
	f := c.f
	if err := f.begin(ctx); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.topics[aws.ToString(in.TopicArn)]
	if !ok {
		return nil, snsNotFound("topic")
	}
	start, err := pageStart(in.NextToken)
	if err != nil || start > len(t.subs) {
		return nil, snsInvalid("invalid NextToken")
	}
	end := min(start+subscriptionsPage, len(t.subs))
	out := &sns.ListSubscriptionsByTopicOutput{}
	for _, s := range t.subs[start:end] {
		out.Subscriptions = append(out.Subscriptions, snstypes.Subscription{
			SubscriptionArn: aws.String(s.arn), TopicArn: aws.String(s.topicARN),
			Protocol: aws.String("sqs"), Endpoint: aws.String(s.endpoint), Owner: aws.String(FakeAccount),
		})
	}
	if end < len(t.subs) {
		out.NextToken = aws.String(strconv.Itoa(end))
	}
	return out, nil
}

func pageStart(token *string) (int, error) {
	if token == nil {
		return 0, nil
	}
	n, err := strconv.Atoi(*token)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid token")
	}
	return n, nil
}

// --- SQS ----------------------------------------------------------------------------------

// FakeSQS is the SQS client of a Fake.
type FakeSQS struct{ f *Fake }

func queueDoesNotExist() error {
	return &sqstypes.QueueDoesNotExist{Message: aws.String("The specified queue does not exist.")}
}

func sqsInvalid(format string, args ...any) error {
	return &sqstypes.InvalidAttributeValue{Message: aws.String(fmt.Sprintf(format, args...))}
}

// queueByURL returns the queue of url, or nil; f.mu is held.
func (f *Fake) queueByURL(url string) *fakeQueue {
	i := strings.LastIndexByte(url, '/')
	if q, ok := f.queues[url[i+1:]]; ok && q.url == url {
		return q
	}
	return nil
}

// queueByARN returns the queue of arn, or nil; f.mu is held.
func (f *Fake) queueByARN(arn string) *fakeQueue {
	i := strings.LastIndexByte(arn, ':')
	if q, ok := f.queues[arn[i+1:]]; ok && q.arn == arn {
		return q
	}
	return nil
}

// notify ends the long polls waiting on q; f.mu is held.
func (q *fakeQueue) notify() {
	close(q.wake)
	q.wake = make(chan struct{})
}

// allowsSNS reports whether q's policy lets sns.amazonaws.com send it messages from topicARN.
func (q *fakeQueue) allowsSNS(topicARN string) bool {
	var policy struct {
		Statement []struct {
			Effect    string
			Principal json.RawMessage
			Action    json.RawMessage
			Resource  json.RawMessage
			Condition map[string]map[string]json.RawMessage
		}
	}
	if err := json.Unmarshal([]byte(q.attributes["Policy"]), &policy); err != nil {
		return false
	}
	for _, s := range policy.Statement {
		if s.Effect != "Allow" ||
			!principalIs(s.Principal, "sns.amazonaws.com") ||
			!anyOf(s.Action, "sqs:SendMessage", "sqs:*", "*") ||
			!anyOf(s.Resource, q.arn, "*") {
			continue
		}
		source := false
		for op, keys := range s.Condition {
			if (op == "ArnEquals" || op == "StringEquals" || op == "ArnLike") && anyOf(keys["aws:SourceArn"], topicARN) {
				source = true
			}
		}
		if source {
			return true
		}
	}
	return false
}

// anyOf reports whether raw, a JSON string or array of strings, holds one of want.
func anyOf(raw json.RawMessage, want ...string) bool {
	var one string
	if json.Unmarshal(raw, &one) == nil {
		return slices.Contains(want, one)
	}
	var many []string
	if json.Unmarshal(raw, &many) == nil {
		return slices.ContainsFunc(many, func(s string) bool { return slices.Contains(want, s) })
	}
	return false
}

func principalIs(raw json.RawMessage, service string) bool {
	var p struct{ Service json.RawMessage }
	if json.Unmarshal(raw, &p) != nil || p.Service == nil {
		return false
	}
	return anyOf(p.Service, service)
}

// queueAttributeLimits are the CreateQueue attributes that the fake takes, with their ranges.
var queueAttributeLimits = map[string][2]int{
	"DelaySeconds":                  {0, 900},
	"MaximumMessageSize":            {1024, 1048576},
	"MessageRetentionPeriod":        {minRetention, maxRetention},
	"ReceiveMessageWaitTimeSeconds": {0, maxWaitTime},
	"VisibilityTimeout":             {0, 43200},
}

// CreateQueue creates a standard queue, or returns the URL of the existing queue of that name
// when its attributes are the same.
func (c *FakeSQS) CreateQueue(ctx context.Context, in *sqs.CreateQueueInput, _ ...func(*sqs.Options)) (*sqs.CreateQueueOutput, error) {
	f := c.f
	if err := f.begin(ctx); err != nil {
		return nil, err
	}
	name := aws.ToString(in.QueueName)
	if name == "" || len(name) > maxQueueName || !queueNameRe.MatchString(name) {
		return nil, &sqstypes.InvalidAttributeName{Message: aws.String(fmt.Sprintf("invalid queue name %q", name))}
	}
	for k, v := range in.Attributes {
		if k == "Policy" {
			if !json.Valid([]byte(v)) {
				return nil, sqsInvalid("invalid Policy")
			}
			continue
		}
		limits, ok := queueAttributeLimits[k]
		if !ok {
			return nil, &sqstypes.InvalidAttributeName{Message: aws.String("the fake does not support the attribute " + k)}
		}
		n, err := strconv.Atoi(v)
		if err != nil || n < limits[0] || n > limits[1] {
			return nil, sqsInvalid("invalid value %q for %s", v, k)
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	now := f.clock.Now()
	if q, ok := f.queues[name]; ok {
		for k, v := range in.Attributes {
			if q.attributes[k] != v {
				return nil, &sqstypes.QueueNameExists{Message: aws.String("a queue named " + name + " exists with other attributes")}
			}
		}
		return &sqs.CreateQueueOutput{QueueUrl: aws.String(q.url)}, nil
	}
	if at, ok := f.deleted[name]; ok && now.Sub(at) < queueNameReuseDelay {
		return nil, &sqstypes.QueueDeletedRecently{Message: aws.String("wait 60 seconds after deleting a queue to create one of the same name")}
	}
	q := &fakeQueue{
		name:    name,
		url:     "https://sqs." + FakeRegion + ".amazonaws.com/" + FakeAccount + "/" + name,
		arn:     "arn:aws:sqs:" + FakeRegion + ":" + FakeAccount + ":" + name,
		created: now, attributes: clone(in.Attributes), tags: clone(in.Tags),
		wake: make(chan struct{}),
	}
	f.queues[name] = q
	return &sqs.CreateQueueOutput{QueueUrl: aws.String(q.url)}, nil
}

// GetQueueAttributes returns the attributes the queue was created with and QueueArn,
// CreatedTimestamp, and ApproximateNumberOfMessages; All returns every one.
func (c *FakeSQS) GetQueueAttributes(ctx context.Context, in *sqs.GetQueueAttributesInput, _ ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error) {
	f := c.f
	if err := f.begin(ctx); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	q := f.queueByURL(aws.ToString(in.QueueUrl))
	if q == nil {
		return nil, queueDoesNotExist()
	}
	all := clone(q.attributes)
	all["QueueArn"] = q.arn
	all["CreatedTimestamp"] = strconv.FormatInt(q.created.Unix(), 10)
	all["ApproximateNumberOfMessages"] = strconv.Itoa(len(q.messages))
	for k, v := range map[string]string{
		"MessageRetentionPeriod": strconv.Itoa(defaultRetention), "ReceiveMessageWaitTimeSeconds": "0",
		"VisibilityTimeout": strconv.Itoa(defaultVisibility),
	} {
		if _, ok := all[k]; !ok {
			all[k] = v
		}
	}
	out := map[string]string{}
	for _, name := range in.AttributeNames {
		if name == sqstypes.QueueAttributeNameAll {
			out = all
			break
		}
		if v, ok := all[string(name)]; ok {
			out[string(name)] = v
		}
	}
	return &sqs.GetQueueAttributesOutput{Attributes: out}, nil
}

// ListQueues lists the URLs of the queues whose name starts with QueueNamePrefix, by name; with
// MaxResults it pages, without it it returns at most 1,000.
func (c *FakeSQS) ListQueues(ctx context.Context, in *sqs.ListQueuesInput, _ ...func(*sqs.Options)) (*sqs.ListQueuesOutput, error) {
	f := c.f
	if err := f.begin(ctx); err != nil {
		return nil, err
	}
	limit := maxListQueues
	if in.MaxResults != nil {
		limit = int(*in.MaxResults)
		if limit < 1 || limit > maxListQueues {
			return nil, sqsInvalid("invalid MaxResults %d", limit)
		}
	} else if in.NextToken != nil {
		return nil, sqsInvalid("NextToken needs MaxResults")
	}
	start, err := pageStart(in.NextToken)
	if err != nil {
		return nil, sqsInvalid("invalid NextToken")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var names []string
	for name := range f.queues {
		if strings.HasPrefix(name, aws.ToString(in.QueueNamePrefix)) {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	start = min(start, len(names))
	end := min(start+limit, len(names))
	out := &sqs.ListQueuesOutput{}
	for _, name := range names[start:end] {
		out.QueueUrls = append(out.QueueUrls, f.queues[name].url)
	}
	if in.MaxResults != nil && end < len(names) {
		out.NextToken = aws.String(strconv.Itoa(end))
	}
	return out, nil
}

// ReceiveMessage returns up to MaxNumberOfMessages visible messages and hides them for the
// visibility timeout; with a wait time it waits for one, and returns empty when the queue is
// deleted meanwhile.
func (c *FakeSQS) ReceiveMessage(ctx context.Context, in *sqs.ReceiveMessageInput, _ ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
	f := c.f
	if err := f.begin(ctx); err != nil {
		return nil, err
	}
	n := int(in.MaxNumberOfMessages)
	if n == 0 {
		n = 1
	}
	if n < 1 || n > maxReceive || in.WaitTimeSeconds < 0 || in.WaitTimeSeconds > maxWaitTime || in.VisibilityTimeout < 0 {
		return nil, sqsInvalid("invalid ReceiveMessage parameters")
	}
	url := aws.ToString(in.QueueUrl)
	var deadline <-chan time.Time
	for {
		f.mu.Lock()
		q := f.queueByURL(url)
		if q == nil {
			f.mu.Unlock()
			return nil, queueDoesNotExist()
		}
		if msgs := f.take(q, n, in.VisibilityTimeout); len(msgs) > 0 {
			f.mu.Unlock()
			return &sqs.ReceiveMessageOutput{Messages: msgs}, nil
		}
		if deadline == nil {
			wait := int(in.WaitTimeSeconds)
			if wait == 0 {
				wait, _ = strconv.Atoi(q.attributes["ReceiveMessageWaitTimeSeconds"])
			}
			if wait == 0 {
				f.mu.Unlock()
				return &sqs.ReceiveMessageOutput{}, nil
			}
			timer := time.NewTimer(time.Duration(wait) * time.Second)
			defer timer.Stop()
			deadline = timer.C
		}
		wake := q.wake
		f.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline:
			return &sqs.ReceiveMessageOutput{}, nil
		case <-wake:
			f.mu.Lock()
			gone := f.queueByURL(url) != q
			f.mu.Unlock()
			if gone {
				return &sqs.ReceiveMessageOutput{}, nil
			}
		}
	}
}

// take hides and returns up to n visible messages of q, dropping the expired ones; f.mu is held.
func (f *Fake) take(q *fakeQueue, n int, visibility int32) []sqstypes.Message {
	now := f.clock.Now()
	retention := defaultRetention
	if v, err := strconv.Atoi(q.attributes["MessageRetentionPeriod"]); err == nil {
		retention = v
	}
	q.messages = slices.DeleteFunc(q.messages, func(m *fakeMessage) bool {
		return now.Sub(m.sent) >= time.Duration(retention)*time.Second
	})
	hide := time.Duration(visibility) * time.Second
	if visibility == 0 {
		v, err := strconv.Atoi(q.attributes["VisibilityTimeout"])
		if err != nil {
			v = defaultVisibility
		}
		hide = time.Duration(v) * time.Second
	}
	var out []sqstypes.Message
	for _, m := range q.messages {
		if len(out) == n {
			break
		}
		if now.Before(m.invisible) {
			continue
		}
		m.receipt = f.newID()
		m.invisible = now.Add(hide)
		out = append(out, sqstypes.Message{MessageId: aws.String(m.id), Body: aws.String(m.body), ReceiptHandle: aws.String(m.receipt)})
	}
	return out
}

// DeleteMessageBatch deletes up to 10 received messages by their receipt handles.
func (c *FakeSQS) DeleteMessageBatch(ctx context.Context, in *sqs.DeleteMessageBatchInput, _ ...func(*sqs.Options)) (*sqs.DeleteMessageBatchOutput, error) {
	f := c.f
	if err := f.begin(ctx); err != nil {
		return nil, err
	}
	if len(in.Entries) == 0 {
		return nil, &sqstypes.EmptyBatchRequest{Message: aws.String("no entries")}
	}
	if len(in.Entries) > maxBatch {
		return nil, &sqstypes.TooManyEntriesInBatchRequest{Message: aws.String("more than 10 entries")}
	}
	ids := map[string]bool{}
	for _, e := range in.Entries {
		id := aws.ToString(e.Id)
		if ids[id] {
			return nil, &sqstypes.BatchEntryIdsNotDistinct{Message: aws.String("duplicate entry id " + id)}
		}
		ids[id] = true
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	q := f.queueByURL(aws.ToString(in.QueueUrl))
	if q == nil {
		return nil, queueDoesNotExist()
	}
	out := &sqs.DeleteMessageBatchOutput{}
	for _, e := range in.Entries {
		i := slices.IndexFunc(q.messages, func(m *fakeMessage) bool { return m.receipt != "" && m.receipt == aws.ToString(e.ReceiptHandle) })
		if i < 0 {
			out.Failed = append(out.Failed, sqstypes.BatchResultErrorEntry{Id: e.Id, Code: aws.String("ReceiptHandleIsInvalid"), SenderFault: true})
			continue
		}
		q.messages = slices.Delete(q.messages, i, i+1)
		out.Successful = append(out.Successful, sqstypes.DeleteMessageBatchResultEntry{Id: e.Id})
	}
	return out, nil
}

// DeleteQueue deletes a queue and its messages; its name cannot be used for 60 seconds.
func (c *FakeSQS) DeleteQueue(ctx context.Context, in *sqs.DeleteQueueInput, _ ...func(*sqs.Options)) (*sqs.DeleteQueueOutput, error) {
	f := c.f
	if err := f.begin(ctx); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	q := f.queueByURL(aws.ToString(in.QueueUrl))
	if q == nil {
		return nil, queueDoesNotExist()
	}
	delete(f.queues, q.name)
	f.deleted[q.name] = f.clock.Now()
	q.notify()
	return &sqs.DeleteQueueOutput{}, nil
}
