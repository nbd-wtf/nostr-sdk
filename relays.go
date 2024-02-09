package sdk

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

type Relay struct {
	URL    string
	Inbox  bool
	Outbox bool
}

func (sys System) FetchRelays(ctx context.Context, pubkey string) []Relay {
	if v, ok := sys.RelaysCache.Get(pubkey); ok {
		return v
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	thunk10002 := sys.replaceableLoaders[10002].Load(ctx, pubkey)
	thunk3 := sys.replaceableLoaders[3].Load(ctx, pubkey)

	result := make([]Relay, 0, 20)

	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		evt, err := thunk10002()
		if err == nil {
			result = append(result, ParseRelaysFromKind10002(evt)...)
		}
	}()
	go func() {
		defer wg.Done()
		evt, err := thunk3()
		if err == nil {
			result = append(result, ParseRelaysFromKind3(evt)...)
		}
	}()

	sys.RelaysCache.SetWithTTL(pubkey, result, time.Hour*6)
	return result
}

func (sys System) FetchOutboxRelays(ctx context.Context, pubkey string) []string {
	relays := sys.FetchRelays(ctx, pubkey)
	result := make([]string, 0, len(relays))
	for _, relay := range relays {
		if relay.Outbox {
			result = append(result, relay.URL)
		}
	}
	return result
}

func ParseRelaysFromKind10002(evt *nostr.Event) []Relay {
	result := make([]Relay, 0, len(evt.Tags))
	for _, tag := range evt.Tags {
		if u := tag.Value(); u != "" && tag[0] == "r" {
			if !nostr.IsValidRelayURL(u) {
				continue
			}
			u := nostr.NormalizeURL(u)

			relay := Relay{
				URL: u,
			}

			if len(tag) == 2 {
				relay.Inbox = true
				relay.Outbox = true
			} else if tag[2] == "write" {
				relay.Outbox = true
			} else if tag[2] == "read" {
				relay.Inbox = true
			}

			result = append(result, relay)
		}
	}

	return result
}

func ParseRelaysFromKind3(evt *nostr.Event) []Relay {
	type Item struct {
		Read  bool `json:"read"`
		Write bool `json:"write"`
	}

	items := make(map[string]Item, 20)
	json.Unmarshal([]byte(evt.Content), &items)

	results := make([]Relay, len(items))
	i := 0
	for u, item := range items {
		if !nostr.IsValidRelayURL(u) {
			continue
		}
		u := nostr.NormalizeURL(u)

		relay := Relay{
			URL: u,
		}

		if item.Read {
			relay.Inbox = true
		}
		if item.Write {
			relay.Outbox = true
		}

		results = append(results, relay)
		i++
	}

	return results
}
