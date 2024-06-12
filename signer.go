package sdk

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

func (sys *system) InitSigner(ctx context.Context, input string, opts *SignerOptions) error {
	if opts == nil {
		opts = &SignerOptions{}
	}

	if strings.HasPrefix(input, "ncryptsec") {
		if opts.PasswordHandler != nil {
			sys.Signer = &EncryptedKeySigner{input, "", opts.PasswordHandler}
			return nil
		}
		sec, err := nip49.Decrypt(input, opts.Password)
		if err != nil {
			if opts.Password == "" {
				return fmt.Errorf("failed to decrypt with blank password: %w", err)
			}
			return fmt.Errorf("failed to decrypt with given password: %w", err)
		}
		pk, _ := nostr.GetPublicKey(sec)
		sys.Signer = KeySigner{sec, pk}
		return nil
	} else if nip46.IsValidBunkerURL(input) || nip05.IsValidIdentifier(input) {
		bcsk := nostr.GeneratePrivateKey()
		oa := func(url string) { println("auth_url received but not handled") }

		if opts.BunkerClientSecretKey != "" {
			bcsk = opts.BunkerClientSecretKey
		}
		if opts.BunkerAuthHandler != nil {
			oa = opts.BunkerAuthHandler
		}

		bunker, err := nip46.ConnectBunker(ctx, bcsk, input, sys.Pool, oa)
		if err != nil {
			return err
		}
		sys.Signer = BunkerSigner{bunker}
	} else if prefix, parsed, err := nip19.Decode(input); err == nil && prefix == "nsec" {
		sec := parsed.(string)
		pk, _ := nostr.GetPublicKey(sec)
		sys.Signer = KeySigner{sec, pk}
		return nil
	} else if nostr.IsValid32ByteHex(input) {
		pk, _ := nostr.GetPublicKey(input)
		sys.Signer = KeySigner{input, pk}
		return nil
	}

	return fmt.Errorf("unsupported input '%s'", input)
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
	bunker *nip46.BunkerClient
}

func (bs BunkerSigner) GetPublicKey() string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	pk, _ := bs.bunker.GetPublicKey(ctx)
	return pk
}

func (bs BunkerSigner) SignEvent(evt *nostr.Event) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	return bs.bunker.SignEvent(ctx, evt)
}
