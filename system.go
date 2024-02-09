package sdk

import (
	"github.com/fiatjaf/eventstore"
	"github.com/fiatjaf/eventstore/nullstore"
	"github.com/graph-gophers/dataloader/v7"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/nostr-sdk/cache"
)

type System struct {
	RelaysCache      cache.Cache32[[]Relay]
	FollowsCache     cache.Cache32[[]Follow]
	MetadataCache    cache.Cache32[ProfileMetadata]
	Pool             *nostr.SimplePool
	RelayListRelays  []string
	FollowListRelays []string
	MetadataRelays   []string
	Store            eventstore.Store

	relayStore         nostr.RelayStore
	replaceableLoaders map[int]*dataloader.Loader[string, *nostr.Event]
}

func (sys *System) Init() {
	if sys.Store == nil {
		sys.Store = nullstore.NullStore{}
	}
	sys.relayStore = eventstore.RelayWrapper{Store: sys.Store}

	sys.initializeDataloaders()
}

func (sys System) Close() {}

func (sys System) StoreRelay() nostr.RelayStore {
	return sys.relayStore
}
