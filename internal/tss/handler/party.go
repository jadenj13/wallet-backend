package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/bnb-chain/tss-lib/v2/common"
	"github.com/bnb-chain/tss-lib/v2/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/v2/ecdsa/signing"
	tsslib "github.com/bnb-chain/tss-lib/v2/tss"
)

type PartyStore interface {
	SaveKeyData(ctx context.Context, partyID int, data *keygen.LocalPartySaveData) error
	LoadKeyData(ctx context.Context, partyID int) (*keygen.LocalPartySaveData, error)
}

type Party struct {
	ID        int
	Peers     []string
	threshold int
	parties   int
	keyData   *keygen.LocalPartySaveData
	store     PartyStore
	activeParty    tsslib.Party
	activePartyIDs []*tsslib.PartyID
	keygenStarted  bool
	mu             sync.RWMutex
}

type Message struct {
	From    int    `json:"from"`
	To      []int  `json:"to"`
	IsBcast bool   `json:"is_broadcast"`
	Payload []byte `json:"payload"`
}

func NewParty(id int, peers []string, threshold, parties int, store PartyStore) *Party {
	return &Party{
		ID:        id,
		Peers:     peers,
		threshold: threshold,
		parties:   parties,
		store:     store,
	}
}

func (p *Party) loadKeyData(ctx context.Context) {
	p.mu.RLock()
	if p.keyData != nil {
		p.mu.RUnlock()
		return
	}
	p.mu.RUnlock()

	keyData, err := p.store.LoadKeyData(ctx, p.ID)
	if err != nil {
		log.Printf("Party %d: Failed to load key data from store: %v", p.ID, err)
		return
	}
	if keyData != nil {
		p.mu.Lock()
		if p.keyData == nil { // re-check under write lock
			p.keyData = keyData
			log.Printf("Party %d: Loaded key data from store", p.ID)
		}
		p.mu.Unlock()
	}
}

func (p *Party) broadcastMessage(msg tsslib.Message) {
	wireBytes, _, err := msg.WireBytes()
	if err != nil {
		log.Printf("Party %d: Failed to get wire bytes: %v", p.ID, err)
		return
	}

	message := Message{
		From:    p.ID,
		IsBcast: msg.IsBroadcast(),
		Payload: wireBytes,
	}

	if msg.IsBroadcast() {
		message.To = make([]int, 0, p.parties-1)
		for i := 0; i < p.parties; i++ {
			if i != p.ID {
				message.To = append(message.To, i)
			}
		}
	} else {
		routing := msg.GetTo()
		message.To = make([]int, len(routing))
		for i, party := range routing {
			message.To[i] = party.Index
		}
	}

	msgBytes, err := json.Marshal(message)
	if err != nil {
		log.Printf("Party %d: Failed to marshal message: %v", p.ID, err)
		return
	}

	for _, peerURL := range p.Peers {
		go func(url string) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			req, err := http.NewRequestWithContext(ctx, "POST", url+"/message",
				bytes.NewReader(msgBytes))
			if err != nil {
				log.Printf("Party %d: Failed to create request to %s: %v", p.ID, url, err)
				return
			}
			req.Header.Set("Content-Type", "application/json")
			http.DefaultClient.Do(req) //nolint:errcheck
		}(peerURL)
	}
}

func (p *Party) runKeygen() {
	unsorted := make(tsslib.UnSortedPartyIDs, p.parties)
	for i := 0; i < p.parties; i++ {
		unsorted[i] = tsslib.NewPartyID(fmt.Sprintf("%d", i), "", big.NewInt(int64(i+1)))
	}
	partyIDs := tsslib.SortPartyIDs(unsorted)

	var myPartyID *tsslib.PartyID
	for _, pid := range partyIDs {
		if pid.Id == fmt.Sprintf("%d", p.ID) {
			myPartyID = pid
			break
		}
	}

	ctx := tsslib.NewPeerContext(partyIDs)
	params := tsslib.NewParameters(tsslib.S256(), ctx, myPartyID, p.parties, p.threshold)

	outCh := make(chan tsslib.Message, p.parties*p.parties)
	endCh := make(chan *keygen.LocalPartySaveData, 1)

	party := keygen.NewLocalParty(params, outCh, endCh)

	p.mu.Lock()
	p.activeParty = party
	p.activePartyIDs = partyIDs
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		p.activeParty = nil
		p.activePartyIDs = nil
		p.keygenStarted = false // allow retry if keygen failed without producing key data
		p.mu.Unlock()
	}()

	go func() {
		if err := party.Start(); err != nil {
			log.Printf("Party %d: Keygen error: %v", p.ID, err)
		}
	}()

	keygenTimeout := time.After(60 * time.Second)
	for {
		select {
		case msg := <-outCh:
			p.broadcastMessage(msg)
		case save := <-endCh:
			p.mu.Lock()
			p.keyData = save
			p.mu.Unlock()

			if err := p.store.SaveKeyData(context.Background(), p.ID, save); err != nil {
				log.Printf("Party %d: Failed to save key data: %v", p.ID, err)
			} else {
				log.Printf("Party %d: Key data saved to store", p.ID)
			}

			log.Printf("Party %d: Keygen completed successfully", p.ID)
			return
		case <-keygenTimeout:
			log.Printf("Party %d: Keygen timeout", p.ID)
			return
		}
	}
}

func (p *Party) InitKeygen(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	if p.keygenStarted || p.keyData != nil {
		p.mu.Unlock()
		http.Error(w, "Keygen already started or completed", http.StatusConflict)
		return
	}
	p.keygenStarted = true
	p.mu.Unlock()

	go p.runKeygen()
	json.NewEncoder(w).Encode(map[string]string{"status": "keygen_started"})
}

func (p *Party) HandleSign(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	p.mu.RLock()
	keyData := p.keyData
	p.mu.RUnlock()

	if keyData == nil {
		p.loadKeyData(r.Context())
		p.mu.RLock()
		keyData = p.keyData
		p.mu.RUnlock()
	}

	if keyData == nil {
		http.Error(w, "No key data available", http.StatusBadRequest)
		return
	}

	sig, err := p.runSigning([]byte(req.Message), *keyData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"r":         fmt.Sprintf("%x", sig.R),
		"s":         fmt.Sprintf("%x", sig.S),
		"signature": fmt.Sprintf("%x", sig.Signature),
	})
}

func (p *Party) runSigning(msg []byte, keyData keygen.LocalPartySaveData) (*common.SignatureData, error) {
	log.Printf("Party %d: Starting signing", p.ID)

	unsorted := make(tsslib.UnSortedPartyIDs, p.parties)
	for i := 0; i < p.parties; i++ {
		unsorted[i] = tsslib.NewPartyID(fmt.Sprintf("%d", i), "", big.NewInt(int64(i+1)))
	}
	partyIDs := tsslib.SortPartyIDs(unsorted)

	var myPartyID *tsslib.PartyID
	for _, pid := range partyIDs {
		if pid.Id == fmt.Sprintf("%d", p.ID) {
			myPartyID = pid
			break
		}
	}

	ctx := tsslib.NewPeerContext(partyIDs)
	params := tsslib.NewParameters(tsslib.S256(), ctx, myPartyID, p.parties, p.threshold)

	outCh := make(chan tsslib.Message, p.parties*p.parties)
	endCh := make(chan *common.SignatureData, 1)

	msgHash := new(big.Int).SetBytes(common.SHA512_256(msg))
	party := signing.NewLocalParty(msgHash, params, keyData, outCh, endCh)

	p.mu.Lock()
	p.activeParty = party
	p.activePartyIDs = partyIDs
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		p.activeParty = nil
		p.activePartyIDs = nil
		p.mu.Unlock()
	}()

	go func() {
		if err := party.Start(); err != nil {
			log.Printf("Party %d: Signing error: %v", p.ID, err)
		}
	}()

	signingTimeout := time.After(60 * time.Second)
	for {
		select {
		case msg := <-outCh:
			p.broadcastMessage(msg)
		case sig := <-endCh:
			if sig != nil {
				log.Printf("Party %d: Signing completed", p.ID)
				return sig, nil
			}
		case <-signingTimeout:
			log.Printf("Party %d: Signing timeout", p.ID)
			return nil, fmt.Errorf("signing timeout")
		}
	}
}

func (p *Party) HandleMessage(w http.ResponseWriter, r *http.Request) {
	var msg Message
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// For unicast messages, drop if this party is not the intended recipient.
	if !msg.IsBcast {
		isRecipient := false
		for _, to := range msg.To {
			if to == p.ID {
				isRecipient = true
				break
			}
		}
		if !isRecipient {
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	p.mu.RLock()
	activeParty := p.activeParty
	var fromPartyID *tsslib.PartyID
	if msg.From >= 0 && msg.From < len(p.activePartyIDs) {
		fromPartyID = p.activePartyIDs[msg.From]
	}
	p.mu.RUnlock()

	if activeParty == nil {
		http.Error(w, "No active protocol session", http.StatusConflict)
		return
	}
	if fromPartyID == nil {
		http.Error(w, "Invalid sender index", http.StatusBadRequest)
		return
	}

	ok, tssErr := activeParty.UpdateFromBytes(msg.Payload, fromPartyID, msg.IsBcast)
	if !ok || tssErr != nil {
		log.Printf("Party %d: Failed to process message from party %d: %v", p.ID, msg.From, tssErr)
		http.Error(w, "Failed to process message", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (p *Party) HandleGetPubKey(w http.ResponseWriter, r *http.Request) {
	p.mu.RLock()
	keyData := p.keyData
	p.mu.RUnlock()

	if keyData == nil {
		p.loadKeyData(r.Context())
		p.mu.RLock()
		keyData = p.keyData
		p.mu.RUnlock()
	}

	if keyData == nil {
		http.Error(w, "No key data available", http.StatusNotFound)
		return
	}

	pubKey := keyData.ECDSAPub
	json.NewEncoder(w).Encode(map[string]any{
		"x": pubKey.X().String(),
		"y": pubKey.Y().String(),
	})
}
