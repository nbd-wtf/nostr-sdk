package sdk

import (
	"context"

	"github.com/fiatjaf/eventstore"
	"github.com/fiatjaf/eventstore/nullstore"
	"github.com/graph-gophers/dataloader/v7"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/nostr-sdk/cache"
	cache_memory "github.com/nbd-wtf/nostr-sdk/cache/memory"
)

type system struct {
	RelayListCache   cache.Cache32[RelayList]
	FollowListCache  cache.Cache32[FollowList]
	MetadataCache    cache.Cache32[ProfileMetadata]
	Pool             *nostr.SimplePool
	RelayListRelays  []string
	FollowListRelays []string
	MetadataRelays   []string
	FallbackRelays   []string
	Store            eventstore.Store
	Signer           Signer

	StoreRelay         nostr.RelayStore
	replaceableLoaders map[int]*dataloader.Loader[string, *nostr.Event]
}

type SystemModifier func(sys *system)

func System(mods ...SystemModifier) *system {
	sys := &system{
		RelayListCache:   cache_memory.New32[RelayList](1000),
		FollowListCache:  cache_memory.New32[FollowList](1000),
		MetadataCache:    cache_memory.New32[ProfileMetadata](1000),
		Pool:             nostr.NewSimplePool(context.Background()),
		RelayListRelays:  []string{"wss://purplepag.es", "wss://user.kindpag.es", "wss://relay.nos.social"},
		FollowListRelays: []string{"wss://purplepag.es", "wss://user.kindpag.es", "wss://relay.nos.social"},
		MetadataRelays:   []string{"wss://purplepag.es", "wss://user.kindpag.es", "wss://relay.nos.social"},
		FallbackRelays: []string{
			"wss://relay.primal.net",
			"wss://relay.damus.io",
			"wss://nostr.wine",
			"wss://nostr.mom",
			"wss://offchain.pub",
			"wss://nos.lol",
			"wss://mostr.pub",
			"wss://relay.nostr.band",
			"wss://nostr21.com",
		},
	}

	for _, mod := range mods {
		mod(sys)
	}

	if sys.Store == nil {
		sys.Store = nullstore.NullStore{}
	}
	sys.StoreRelay = eventstore.RelayWrapper{Store: sys.Store}

	sys.initializeDataloaders()

	return sys
}

func (sys *system) Close() {}

func WithRelayListRelays(list []string) SystemModifier {
	return func(sys *system) {
		sys.RelayListRelays = list
	}
}

func WithMetadataRelays(list []string) SystemModifier {
	return func(sys *system) {
		sys.MetadataRelays = list
	}
}

func WithFollowListRelays(list []string) SystemModifier {
	return func(sys *system) {
		sys.FollowListRelays = list
	}
}

func WithFallbackRelays(list []string) SystemModifier {
	return func(sys *system) {
		sys.FallbackRelays = list
	}
}

func WithPool(pool *nostr.SimplePool) SystemModifier {
	return func(sys *system) {
		sys.Pool = pool
	}
}

func WithStore(store eventstore.Store) SystemModifier {
	return func(sys *system) {
		sys.Store = store
	}
}

func WithRelayListCache(cache cache.Cache32[RelayList]) SystemModifier {
	return func(sys *system) {
		sys.RelayListCache = cache
	}
}

func WithFollowListCache(cache cache.Cache32[FollowList]) SystemModifier {
	return func(sys *system) {
		sys.FollowListCache = cache
	}
}

func WithMetadataCache(cache cache.Cache32[ProfileMetadata]) SystemModifier {
	return func(sys *system) {
		sys.MetadataCache = cache
	}
}
