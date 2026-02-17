package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"

	"github.com/jadenj13/wallet-backend/internal/api/address"
	"github.com/jadenj13/wallet-backend/internal/api/client"
)

type Wallet struct {
	parties []*client.PartyClient
}

func NewWallet(parties []*client.PartyClient) *Wallet {
	return &Wallet{parties: parties}
}

func (w *Wallet) fanOut(_ context.Context, fn func(*client.PartyClient) error) error {
	type result struct{ err error }

	ch := make(chan result, len(w.parties))
	for _, p := range w.parties {
		go func(party *client.PartyClient) {
			ch <- result{err: fn(party)}
		}(p)
	}

	var errs []string
	for range w.parties {
		if res := <-ch; res.err != nil {
			errs = append(errs, res.err.Error())
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

func (w *Wallet) Init(rw http.ResponseWriter, r *http.Request) {
	if len(w.parties) == 0 {
		http.Error(rw, "no party nodes configured", http.StatusServiceUnavailable)
		return
	}

	if err := w.fanOut(r.Context(), func(p *client.PartyClient) error {
		return p.InitKeygen(r.Context())
	}); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(map[string]string{"status": "keygen_started"})
}

func (w *Wallet) Sign(rw http.ResponseWriter, r *http.Request) {
	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Message == "" {
		http.Error(rw, "message is required", http.StatusBadRequest)
		return
	}

	if len(w.parties) == 0 {
		http.Error(rw, "no party nodes configured", http.StatusServiceUnavailable)
		return
	}

	type result struct {
		sig *client.SignatureData
		err error
	}

	ch := make(chan result, len(w.parties))
	for _, p := range w.parties {
		go func(party *client.PartyClient) {
			sig, err := party.Sign(r.Context(), req.Message)
			ch <- result{sig, err}
		}(p)
	}

	var firstSig *client.SignatureData
	var errs []string
	for range w.parties {
		res := <-ch
		if res.err != nil {
			errs = append(errs, res.err.Error())
		} else if firstSig == nil {
			firstSig = res.sig
		}
	}

	if firstSig == nil {
		http.Error(rw, strings.Join(errs, "; "), http.StatusBadGateway)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(firstSig)
}

func (w *Wallet) GetAddress(rw http.ResponseWriter, r *http.Request) {
	if len(w.parties) == 0 {
		http.Error(rw, "no party nodes configured", http.StatusServiceUnavailable)
		return
	}

	var pubKey *client.PubKey
	for _, p := range w.parties {
		pk, err := p.GetPubKey(r.Context())
		if err != nil {
			continue
		}
		pubKey = pk
		break
	}

	if pubKey == nil {
		http.Error(rw, "no party could provide public key", http.StatusBadGateway)
		return
	}

	x, ok := new(big.Int).SetString(pubKey.X, 10)
	if !ok {
		http.Error(rw, "invalid public key X coordinate", http.StatusInternalServerError)
		return
	}
	y, ok := new(big.Int).SetString(pubKey.Y, 10)
	if !ok {
		http.Error(rw, "invalid public key Y coordinate", http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "application/json")

	if chain := r.URL.Query().Get("chain"); chain != "" {
		addr, supported := address.Derive(chain, x, y)
		if !supported {
			http.Error(rw, fmt.Sprintf("unsupported chain %q, supported: ethereum, bitcoin", chain), http.StatusBadRequest)
			return
		}
		json.NewEncoder(rw).Encode(map[string]string{"chain": chain, "address": addr})
		return
	}

	json.NewEncoder(rw).Encode(address.All(x, y))
}

func (w *Wallet) GetPubKey(rw http.ResponseWriter, r *http.Request) {
	if len(w.parties) == 0 {
		http.Error(rw, "no party nodes configured", http.StatusServiceUnavailable)
		return
	}

	for _, p := range w.parties {
		pubKey, err := p.GetPubKey(r.Context())
		if err != nil {
			continue
		}
		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(pubKey)
		return
	}

	http.Error(rw, "no party could provide public key", http.StatusBadGateway)
}
