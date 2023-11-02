package sdk

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

type ProfileMetadata struct {
	PubKey string       `json:"-"`
	Event  *nostr.Event `json:"-"`

	Name        string `json:"name,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	About       string `json:"about,omitempty"`
	Website     string `json:"website,omitempty"`
	Picture     string `json:"picture,omitempty"`
	Banner      string `json:"banner,omitempty"`
	NIP05       string `json:"nip05,omitempty"`
	LUD16       string `json:"lud16,omitempty"`
}

func (p ProfileMetadata) Npub() string {
	v, _ := nip19.EncodePublicKey(p.PubKey)
	return v
}

func (p ProfileMetadata) Nprofile(ctx context.Context, sys *System, nrelays int) string {
	v, _ := nip19.EncodeProfile(p.PubKey, sys.FetchOutboxRelays(ctx, p.PubKey))
	return v
}

func (p ProfileMetadata) ShortName() string {
	if p.Name != "" {
		return p.Name
	}
	if p.DisplayName != "" {
		return p.DisplayName
	}
	npub := p.Npub()
	return npub[0:7] + "…" + npub[58:]
}

func FetchProfileMetadata(ctx context.Context, pool *nostr.SimplePool, pubkey string, relays ...string) ProfileMetadata {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ch := pool.SubManyEose(ctx, relays, nostr.Filters{
		{
			Kinds:   []int{nostr.KindProfileMetadata},
			Authors: []string{pubkey},
			Limit:   1,
		},
	})

	for ie := range ch {
		if m, err := ParseMetadata(ie.Event); err == nil {
			m.PubKey = pubkey
			m.Event = ie.Event
			return *m
		}
	}

	return ProfileMetadata{PubKey: pubkey}
}

func ParseMetadata(event *nostr.Event) (*ProfileMetadata, error) {
	if event.Kind != 0 {
		return nil, fmt.Errorf("event %s is kind %d, not 0", event.ID, event.Kind)
	}

	var meta ProfileMetadata
	if err := json.Unmarshal([]byte(event.Content), &meta); err != nil {
		cont := event.Content
		if len(cont) > 100 {
			cont = cont[0:99]
		}
		return nil, fmt.Errorf("failed to parse metadata (%s) from event %s: %w", cont, event.ID, err)
	}

	return &meta, nil
}
