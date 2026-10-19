// SPDX-License-Identifier: AGPL-3.0-only

package sns

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// processIDRe is the form of a process ID (stream.NewProcessID).
var processIDRe = regexp.MustCompile(`^[0-9a-f]{32}$`)

// processOf returns the process ID in the queue name, or "" when name is not the name of a queue
// of the Bus.
func (b *Bus) processOf(name string) string {
	rest, ok := strings.CutPrefix(name, b.queuePrefix+"-")
	if !ok {
		return ""
	}
	id, n, ok := strings.Cut(rest, "-")
	if !ok || !processIDRe.MatchString(id) {
		return ""
	}
	if _, err := strconv.ParseUint(n, 10, 64); err != nil {
		return ""
	}
	return id
}

// lastSegment returns what follows the last sep in s: the name in a queue URL or ARN.
func lastSegment(s string, sep byte) string {
	return s[strings.LastIndexByte(s, sep)+1:]
}

// Sweep deletes the queues under the prefix created at least minAge ago whose process is not
// alive, with their subscriptions to the topic, and the subscriptions of the topic to queues
// under the prefix that no longer exist, of processes that are not alive. It counts each queue
// and subscription it deletes. A process is asked once; a resource whose process cannot be told
// is kept, and the errors are returned once the rest is swept.
func (b *Bus) Sweep(ctx context.Context, minAge time.Duration, alive func(ctx context.Context, processID string) (bool, error)) (int, error) {
	queues, err := b.listQueues(ctx)
	if err != nil {
		return 0, err
	}
	subs, err := b.listSubscriptions(ctx)
	if err != nil {
		return 0, err
	}

	var errs []error
	verdicts := map[string]bool{}
	dead := func(processID string) bool {
		if v, ok := verdicts[processID]; ok {
			return v
		}
		ok, err := alive(ctx, processID)
		if err != nil {
			errs = append(errs, fmt.Errorf("stream/sns: liveness of process %s: %w", processID, err))
			ok = true // kept
		}
		verdicts[processID] = !ok
		return !ok
	}

	deleted := 0
	now := b.clock.Now()
	for name, url := range queues {
		if !dead(b.processOf(name)) {
			continue
		}
		created, err := b.queueCreated(ctx, url)
		if err != nil {
			if !queueGone(err) {
				errs = append(errs, err)
			}
			continue
		}
		if now.Sub(created) < minAge {
			continue
		}
		for _, sub := range subs[name] {
			deleted += b.unsubscribe(ctx, sub, &errs)
		}
		delete(subs, name)
		_, err = b.sqs.DeleteQueue(ctx, &sqs.DeleteQueueInput{QueueUrl: aws.String(url)})
		switch {
		case err == nil:
			deleted++
		case !queueGone(err):
			errs = append(errs, fmt.Errorf("stream/sns: delete queue %s: %w", name, err))
		}
	}
	for name, arns := range subs {
		if _, ok := queues[name]; ok || !dead(b.processOf(name)) {
			continue
		}
		for _, sub := range arns {
			deleted += b.unsubscribe(ctx, sub, &errs)
		}
	}
	return deleted, errors.Join(errs...)
}

// listQueues returns the URLs of the queues of the Bus by name.
func (b *Bus) listQueues(ctx context.Context) (map[string]string, error) {
	queues := map[string]string{}
	in := &sqs.ListQueuesInput{QueueNamePrefix: aws.String(b.queuePrefix + "-"), MaxResults: aws.Int32(1000)}
	for {
		out, err := b.sqs.ListQueues(ctx, in)
		if err != nil {
			return nil, fmt.Errorf("stream/sns: list queues: %w", err)
		}
		for _, url := range out.QueueUrls {
			if name := lastSegment(url, '/'); b.processOf(name) != "" {
				queues[name] = url
			}
		}
		if out.NextToken == nil {
			return queues, nil
		}
		in.NextToken = out.NextToken
	}
}

// listSubscriptions returns the ARNs of the topic's subscriptions to queues of the Bus, by queue
// name.
func (b *Bus) listSubscriptions(ctx context.Context) (map[string][]string, error) {
	subs := map[string][]string{}
	in := &sns.ListSubscriptionsByTopicInput{TopicArn: aws.String(b.topicARN)}
	for {
		out, err := b.sns.ListSubscriptionsByTopic(ctx, in)
		if err != nil {
			return nil, fmt.Errorf("stream/sns: list subscriptions: %w", err)
		}
		for _, s := range out.Subscriptions {
			endpoint := aws.ToString(s.Endpoint)
			if aws.ToString(s.Protocol) != "sqs" || !strings.HasPrefix(endpoint, b.queueARN) {
				continue
			}
			if name := lastSegment(endpoint, ':'); b.processOf(name) != "" {
				subs[name] = append(subs[name], aws.ToString(s.SubscriptionArn))
			}
		}
		if out.NextToken == nil {
			return subs, nil
		}
		in.NextToken = out.NextToken
	}
}

// queueCreated returns when the queue of url was created.
func (b *Bus) queueCreated(ctx context.Context, url string) (time.Time, error) {
	out, err := b.sqs.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl: aws.String(url), AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameCreatedTimestamp},
	})
	if err != nil {
		return time.Time{}, fmt.Errorf("stream/sns: attributes of queue %s: %w", lastSegment(url, '/'), err)
	}
	s, err := strconv.ParseInt(out.Attributes[string(sqstypes.QueueAttributeNameCreatedTimestamp)], 10, 64)
	if err != nil || s <= 0 {
		return time.Time{}, fmt.Errorf("stream/sns: queue %s has no CreatedTimestamp", lastSegment(url, '/'))
	}
	return time.Unix(s, 0), nil
}

// unsubscribe deletes a subscription and returns 1, or 0 when it was gone already or could not
// be deleted (the error is added to errs).
func (b *Bus) unsubscribe(ctx context.Context, subscriptionARN string, errs *[]error) int {
	_, err := b.sns.Unsubscribe(ctx, &sns.UnsubscribeInput{SubscriptionArn: aws.String(subscriptionARN)})
	switch {
	case err == nil:
		return 1
	case !subscriptionGone(err):
		*errs = append(*errs, fmt.Errorf("stream/sns: unsubscribe %s: %w", subscriptionARN, err))
	}
	return 0
}
