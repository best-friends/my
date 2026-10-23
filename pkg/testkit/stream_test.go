// SPDX-License-Identifier: AGPL-3.0-only

package testkit_test

import (
	"context"
	"testing"

	"github.com/mycroblog/my/internal/app/stream"
	"github.com/mycroblog/my/pkg/testkit"
)

func TestStreamBusContractMemoryBroker(t *testing.T) {
	testkit.StreamBusContract(t, func(*testing.T) testkit.StreamBus {
		broker := stream.NewMemoryBroker()
		return testkit.StreamBus{
			NewBus: func(*testing.T) stream.Bus { return broker.Bus() },
			Unreachable: func(*testing.T) stream.Bus {
				unreachable := stream.NewMemoryBroker()
				unreachable.SetUnreachable(true)
				return unreachable.Bus()
			},
			Break:     func(*testing.T) { broker.Break() },
			Resources: func(*testing.T) int { return broker.Listeners() },
		}
	})
}

func TestStreamBrokerContractHub(t *testing.T) {
	testkit.StreamBrokerContract(t, func(t *testing.T, o stream.HubOptions) testkit.StreamBroker {
		h := stream.NewHub(o)
		t.Cleanup(h.Close)
		return testkit.StreamBroker{Publisher: h, Subscriber: h}
	})
}

func TestStreamBrokerContractRelay(t *testing.T) {
	testkit.StreamBrokerContract(t, func(t *testing.T, o stream.HubOptions) testkit.StreamBroker {
		broker := stream.NewMemoryBroker()
		local := stream.NewRelay(broker.Bus(), stream.RelayOptions{Hub: o})
		remote := stream.NewRelay(broker.Bus(), stream.RelayOptions{})
		for _, r := range []*stream.Relay{local, remote} {
			t.Cleanup(func() {
				if err := r.Close(context.Background()); err != nil {
					t.Errorf("Close: %v", err)
				}
			})
		}
		return testkit.StreamBroker{
			Publisher: local, Subscriber: local, Remote: remote,
			Break: func(*testing.T) { broker.Break() },
		}
	})
}
