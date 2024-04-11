package sdk

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/nbd-wtf/go-nostr"
)

type EventResult dataloader.Result[*nostr.Event]

func (sys *system) initializeDataloaders() {
	sys.replaceableLoaders = make(map[int]*dataloader.Loader[string, *nostr.Event])
	for _, kind := range []int{0, 3, 10000, 10001, 10002, 10003, 10004, 10005, 10006, 10007, 10015, 10030} {
		sys.replaceableLoaders[kind] = sys.createReplaceableDataloader(kind)
	}
}

func (sys *system) createReplaceableDataloader(kind int) *dataloader.Loader[string, *nostr.Event] {
	return dataloader.NewBatchedLoader(
		func(
			ctx context.Context,
			pubkeys []string,
		) []*dataloader.Result[*nostr.Event] {
			return sys.batchLoadReplaceableEvents(ctx, kind, pubkeys)
		},
		dataloader.WithBatchCapacity[string, *nostr.Event](400),
		dataloader.WithClearCacheOnBatch[string, *nostr.Event](),
		dataloader.WithWait[string, *nostr.Event](time.Millisecond*400),
	)
}

func (sys *system) batchLoadReplaceableEvents(
	ctx context.Context,
	kind int,
	pubkeys []string,
) []*dataloader.Result[*nostr.Event] {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	batchSize := len(pubkeys)
	results := make([]*dataloader.Result[*nostr.Event], batchSize)
	keyPositions := make(map[string]int)          // { [pubkey]: slice_index }
	relayFilters := make(map[string]nostr.Filter) // { [relayUrl]: filter }

	for i, pubkey := range pubkeys {
		// if we're attempting this query with a short key (last 8 characters), stop here
		if len(pubkey) != 64 {
			results[i] = &dataloader.Result[*nostr.Event]{
				Error: fmt.Errorf("won't proceed to query relays with a shortened key (%d)", kind),
			}
			continue
		}

		// save attempts here so we don't try the same failed query over and over
		if doItNow := DoThisNotMoreThanOnceAnHour("repl:" + strconv.Itoa(kind) + pubkey); !doItNow {
			results[i] = &dataloader.Result[*nostr.Event]{
				Error: fmt.Errorf("last attempt failed, waiting more to try again"),
			}
			continue
		}

		// build batched queries for the external relays
		keyPositions[pubkey] = i // this is to help us know where to save the result later

		// gather relays we'll use for this pubkey
		relays := sys.determineRelaysToQuery(ctx, pubkey, kind)

		// by default we will return an error (this will be overwritten when we find an event)
		results[i] = &dataloader.Result[*nostr.Event]{
			Error: fmt.Errorf("couldn't find a kind %d event anywhere %v", kind, relays),
		}

		for _, relay := range relays {
			// each relay will have a custom filter
			filter, ok := relayFilters[relay]
			if !ok {
				filter = nostr.Filter{
					Kinds:   []int{kind},
					Authors: make([]string, 0, batchSize-i /* this and all pubkeys after this can be added */),
				}
			}
			filter.Authors = append(filter.Authors, pubkey)
			relayFilters[relay] = filter
		}
	}

	// query all relays with the prepared filters
	multiSubs := sys.batchReplaceableRelayQueries(ctx, relayFilters)
	for {
		select {
		case evt, more := <-multiSubs:
			if !more {
				return results
			}

			// insert this event at the desired position
			pos := keyPositions[evt.PubKey] // @unchecked: it must succeed because it must be a key we passed
			if results[pos].Data == nil || results[pos].Data.CreatedAt < evt.CreatedAt {
				results[pos] = &dataloader.Result[*nostr.Event]{Data: evt}
			}
		case <-ctx.Done():
			return results
		}
	}
}

func (sys *system) determineRelaysToQuery(ctx context.Context, pubkey string, kind int) []string {
	relays := make([]string, 0, 10)

	// search in specific relays for user
	if kind != 10002 && kind != 3 {
		// (but not for these kinds otherwise we will create an infinite loop)
		relays = sys.FetchOutboxRelays(ctx, pubkey)
	}

	// use a different set of extra relays depending on the kind
	switch kind {
	case 0:
		relays = append(relays, sys.MetadataRelays...)
	case 3:
		relays = append(relays, sys.FollowListRelays...)
	case 10002:
		relays = append(relays, sys.RelayListRelays...)
	}

	return relays
}

// batchReplaceableRelayQueries subscribes to multiple relays using a different filter for each and returns
// a single channel with all results. it closes on EOSE or when all the expected events were returned.
//
// the number of expected events is given by the number of pubkeys in the .Authors filter field.
// because of that, batchReplaceableRelayQueries is only suitable for querying replaceable events -- and
// care must be taken to not include the same pubkey more than once in the filter .Authors array.
func (sys *system) batchReplaceableRelayQueries(
	ctx context.Context,
	relayFilters map[string]nostr.Filter,
) <-chan *nostr.Event {
	all := make(chan *nostr.Event)

	wg := sync.WaitGroup{}
	wg.Add(len(relayFilters))
	for url, filter := range relayFilters {
		go func(url string, filter nostr.Filter) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(ctx, time.Second*4)
			defer cancel()

			n := len(filter.Authors)

			relay, err := sys.Pool.EnsureRelay(url)
			if err != nil {
				return
			}
			sub, _ := relay.Subscribe(ctx, nostr.Filters{filter}, nostr.WithLabel("batch-repl"))
			if sub == nil {
				return
			}

			received := 0
			for {
				select {
				case evt, more := <-sub.Events:
					if !more {
						// ctx canceled, sub.Events is closed
						return
					}

					all <- evt

					received++
					if received >= n {
						// we got all events we asked for, unless the relay is shitty and sent us two from the same
						return
					}
				case <-sub.EndOfStoredEvents:
					// close here
					return
				}
			}
		}(url, filter)
	}

	go func() {
		wg.Wait()
		close(all)
	}()

	return all
}
