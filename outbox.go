package sdk

import (
	"context"
	"fmt"
	"sync"

	"github.com/nbd-wtf/go-nostr"
)

var fetchOutboxLocks [5]sync.Mutex

func (sys *System) FetchOutboxRelays(ctx context.Context, pubkey string, n int) []string {
	idx := pubkey[0] % 5
	fetchOutboxLocks[idx].Lock()
	defer fetchOutboxLocks[idx].Unlock()

	if _, ok := sys.RelayListCache.Get(pubkey); !ok {
		fetchGenericList(sys, ctx, pubkey, 10002, parseRelayFromKind10002, sys.RelayListCache, false)
	}

	relays := sys.Hints.TopN(pubkey, n)
	return relays
}

func (sys *System) ExpandQueriesByAuthorAndRelays(
	ctx context.Context,
	filter nostr.Filter,
) (map[string]nostr.Filter, error) {
	n := len(filter.Authors)
	if n == 0 {
		return nil, fmt.Errorf("no authors in filter")
	}

	relaysForPubkey := make(map[string][]string, n)
	mu := sync.Mutex{}

	wg := sync.WaitGroup{}
	wg.Add(n)
	for _, pubkey := range filter.Authors {
		go func(pubkey string) {
			defer wg.Done()
			relayURLs := sys.FetchOutboxRelays(ctx, pubkey, 3)
			c := 0
			for _, r := range relayURLs {
				relay, err := sys.Pool.EnsureRelay(r)
				if err != nil {
					continue
				}
				mu.Lock()
				relaysForPubkey[pubkey] = append(relaysForPubkey[pubkey], relay.URL)
				mu.Unlock()
				c++
				if c == 3 {
					return
				}
			}
		}(pubkey)
	}
	wg.Wait()

	filterForRelay := make(map[string]nostr.Filter, n) // { [relay]: filter }
	for pubkey, relays := range relaysForPubkey {
		for _, relay := range relays {
			flt, ok := filterForRelay[relay]
			if !ok {
				flt = filter.Clone()
				filterForRelay[relay] = flt
			}
			flt.Authors = append(flt.Authors, pubkey)
		}
	}

	return filterForRelay, nil
}
