package signer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip05"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/nbd-wtf/go-nostr/nip46"
	"github.com/nbd-wtf/go-nostr/nip49"
)

type Signer interface {
	GetPublicKey() string
	SignEvent(*nostr.Event) error
}

type SignerOptions struct {
	BunkerClientSecretKey string
	BunkerSignTimeout     time.Duration
	BunkerAuthHandler     func(string)
	Password              string
	PasswordHandler       func() string
}

func New(ctx context.Context, pool *nostr.SimplePool, input string, opts *SignerOptions) (Signer, error) {
	if opts == nil {
		opts = &SignerOptions{}
	}

	if strings.HasPrefix(input, "ncryptsec") {
		if opts.PasswordHandler != nil {
			return &EncryptedKeySigner{input, "", opts.PasswordHandler}, nil
		}
		sec, err := nip49.Decrypt(input, opts.Password)
		if err != nil {
			if opts.Password == "" {
				return nil, fmt.Errorf("failed to decrypt with blank password: %w", err)
			}
			return nil, fmt.Errorf("failed to decrypt with given password: %w", err)
		}
		pk, _ := nostr.GetPublicKey(sec)
		return KeySigner{sec, pk}, nil
	} else if nip46.IsValidBunkerURL(input) || nip05.IsValidIdentifier(input) {
		bcsk := nostr.GeneratePrivateKey()
		oa := func(url string) { println("auth_url received but not handled") }

		if opts.BunkerClientSecretKey != "" {
			bcsk = opts.BunkerClientSecretKey
		}
		if opts.BunkerAuthHandler != nil {
			oa = opts.BunkerAuthHandler
		}

		bunker, err := nip46.ConnectBunker(ctx, bcsk, input, pool, oa)
		if err != nil {
			return nil, err
		}
		return BunkerSigner{ctx, bunker}, nil
	} else if prefix, parsed, err := nip19.Decode(input); err == nil && prefix == "nsec" {
		sec := parsed.(string)
		pk, _ := nostr.GetPublicKey(sec)
		return KeySigner{sec, pk}, nil
	} else if nostr.IsValid32ByteHex(input) {
		pk, _ := nostr.GetPublicKey(input)
		return KeySigner{input, pk}, nil
	}

	return nil, fmt.Errorf("unsupported input '%s'", input)
}

type KeySigner struct {
	sk string
	pk string
}

func (ks KeySigner) SignEvent(evt *nostr.Event) error { return evt.Sign(ks.sk) }
func (ks KeySigner) GetPublicKey() string             { return ks.pk }

type EncryptedKeySigner struct {
	ncryptsec string
	pk        string
	callback  func() string
}

func (es *EncryptedKeySigner) GetPublicKey() string {
	if es.pk != "" {
		return es.pk
	}
	password := es.callback()
	key, err := nip49.Decrypt(es.ncryptsec, password)
	if err != nil {
		return ""
	}
	pk, _ := nostr.GetPublicKey(key)
	es.pk = pk
	return pk
}

func (es *EncryptedKeySigner) SignEvent(evt *nostr.Event) error {
	password := es.callback()
	key, err := nip49.Decrypt(es.ncryptsec, password)
	if err != nil {
		return fmt.Errorf("invalid password: %w", err)
	}
	es.pk = evt.PubKey
	return evt.Sign(key)
}

type BunkerSigner struct {
	ctx    context.Context
	bunker *nip46.BunkerClient
}

func (bs BunkerSigner) GetPublicKey() string {
	ctx, cancel := context.WithTimeout(bs.ctx, time.Second*30)
	defer cancel()
	pk, _ := bs.bunker.GetPublicKey(ctx)
	return pk
}

func (bs BunkerSigner) SignEvent(evt *nostr.Event) error {
	ctx, cancel := context.WithTimeout(bs.ctx, time.Second*30)
	defer cancel()
	return bs.bunker.SignEvent(ctx, evt)
}
