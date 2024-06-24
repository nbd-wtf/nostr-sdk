package memory

import (
	"fmt"
	"slices"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/nostr-sdk/hints"
)

var _ hints.HintsDB = (*HintDB)(nil)

type HintDB struct {
	RelayBySerial         []string
	OrderedRelaysByPubKey map[string]RelaysForPubKey
}

func NewHintDB() *HintDB {
	return &HintDB{
		RelayBySerial:         make([]string, 0, 100),
		OrderedRelaysByPubKey: make(map[string]RelaysForPubKey, 100),
	}
}

func (db *HintDB) Save(pubkey string, relay string, key hints.HintKey, ts nostr.Timestamp) {
	now := nostr.Now()
	// this is used for calculating what counts as a usable hint
	threshold := (now - 60*60*24*180)
	if threshold < 0 {
		threshold = 0
	}

	relayIndex := slices.Index(db.RelayBySerial, relay)
	if relayIndex == -1 {
		relayIndex = len(db.RelayBySerial)
		db.RelayBySerial = append(db.RelayBySerial, relay)
	}

	// fmt.Println(" ", relay, "index", relayIndex, "--", "adding", hints.HintKey(key).String(), ts)

	rfpk, _ := db.OrderedRelaysByPubKey[pubkey]

	entries := rfpk.Entries

	if rfpk.Oldest == 0 {
		rfpk.Oldest = now
	}

	var prevV int64 = 0
	entryIndex := slices.IndexFunc(entries, func(re RelayEntry) bool { return re.Relay == relayIndex })
	if entryIndex == -1 {
		// we don't have an entry for this relay, so add one
		entryIndex = len(entries)

		entry := RelayEntry{
			Relay: relayIndex,
		}
		entry.Timestamps[key] = ts

		entries = append(entries, entry)
	} else {
		// just update this entry
		if entries[entryIndex].Timestamps[key] < ts {
			prevV = entries[entryIndex].Sum(rfpk.Oldest) // capture the value before we update it
			entries[entryIndex].Timestamps[key] = ts
		} else {
			// no need to update anything
			return
		}
	}

	// whenever something changes the oldest might not be the oldest anymore
	if ts > threshold && ts < rfpk.Oldest {
		rfpk.Oldest = ts
		// sort everything from scratch based on the new oldest base
		slices.SortFunc(rfpk.Entries, func(a, b RelayEntry) int {
			return int(b.Sum(rfpk.Oldest) - a.Sum(rfpk.Oldest))
		})
	} else {
		// if the oldest value hasn't changed we can just reposition
		// the newly added or modified entry so the thing remains ordered
		newV := entries[entryIndex].Sum(rfpk.Oldest)
		var dir int
		if newV > prevV {
			// reposition upwards
			dir = -1
		} else {
			// reposition downwards
			dir = +1
		}
		// fmt.Println("   ", "newV", newV, "prevV", prevV, "::", newV > prevV, "dir", dir)

		for {
			nextIndex := entryIndex + dir
			if nextIndex == -1 || nextIndex == len(entries) {
				break
			}

			nextV := entries[nextIndex].Sum(rfpk.Oldest)
			if (dir == -1 && nextV < newV) || (dir == +1 && nextV > newV) {
				// swap
				entries[entryIndex], entries[nextIndex] = entries[nextIndex], entries[entryIndex]
				entryIndex = nextIndex
			} else {
				break
			}
		}
	}

	rfpk.Entries = entries
	db.OrderedRelaysByPubKey[pubkey] = rfpk
}

func (db HintDB) TopN(pubkey string, n int) []string {
	urls := make([]string, 0, n)
	if rfpk, ok := db.OrderedRelaysByPubKey[pubkey]; ok {
		for i, re := range rfpk.Entries {
			urls = append(urls, db.RelayBySerial[re.Relay])
			if i+1 == n {
				break
			}
		}
	}
	return urls
}

func (db *HintDB) PrintScores() {
	for pubkey, rfpk := range db.OrderedRelaysByPubKey {
		fmt.Println("== relay scores for", pubkey)
		for i, re := range rfpk.Entries {
			fmt.Printf("  %3d :: %30s (%3d) ::> %12d\n", i, db.RelayBySerial[re.Relay], re.Relay, re.Sum(rfpk.Oldest))
		}
	}
}

type RelaysForPubKey struct {
	Oldest  nostr.Timestamp
	Entries []RelayEntry
}

type RelayEntry struct {
	Relay      int
	Timestamps [8]nostr.Timestamp
}

func (re RelayEntry) Sum(base nostr.Timestamp) int64 {
	var sum int64
	// fmt.Println("  summing", re.Relay, "with base", base)
	for i, ts := range re.Timestamps {
		if ts == 0 {
			continue
		}

		hk := hints.HintKey(i)
		multiplier := hk.BasePoints()
		var value int64
		if ts == base {
			value = multiplier // * 1
		} else if ts < base {
			value = multiplier / 2 // * 0.5
		} else {
			value = multiplier * int64(ts) / (60 * 60 * 24 * 30)
		}
		// fmt.Println("   ", i, "value:", value)
		sum += value
	}
	return sum
}
