package sdk

import (
	"context"

	"github.com/nbd-wtf/go-nostr"
)

type RelayList = GenericList[Relay]

type Relay struct {
	URL    string
	Inbox  bool
	Outbox bool
}

func (sys *system) FetchRelays(ctx context.Context, pubkey string) RelayList {
	rl, _ := fetchGenericList[Relay](sys, ctx, pubkey, 3, parseRelayFromKind10002, sys.RelayListCache, false)
	return rl
}

func (sys *system) FetchOutboxRelays(ctx context.Context, pubkey string) []string {
	rl := sys.FetchRelays(ctx, pubkey)
	result := make([]string, 0, len(rl.Items))
	for _, relay := range rl.Items {
		if relay.Outbox {
			result = append(result, relay.URL)
		}
	}
	return result
}

func parseRelayFromKind10002(tag nostr.Tag) (rl Relay, ok bool) {
	if u := tag.Value(); u != "" && tag[0] == "r" {
		if !nostr.IsValidRelayURL(u) {
			return rl, false
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

		return relay, true
	}

	return rl, false
}
