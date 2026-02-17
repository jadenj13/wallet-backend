package handler

import (
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

type Party struct{
	ID        int
	Peers     []string
	threshold int
	parties   int
	keyData  *keygen.LocalPartySaveData
	outCh    chan tsslib.Message
	msgStore map[string][]tsslib.Message
	mu        sync.RWMutex
}

type Message struct {
	From    int             `json:"from"`
	To      []int           `json:"to"`
	IsBcast bool            `json:"is_broadcast"`
	Payload json.RawMessage `json:"payload"`
}

func NewParty(id int, peers []string, threshold, parties int) *Party {
	return &Party{
		ID:        id,
		Peers:     peers,
		threshold: threshold,
		parties:   parties,
		msgStore:  make(map[string][]tsslib.Message),
	}
}

func (p *Party) broadcastMessage(msg tsslib.Message) {
	msgBytes, _ := json.Marshal(msg)

	message := Message{
		From:    p.ID,
		IsBcast: msg.IsBroadcast(),
		Payload: msgBytes,
	}

	if msg.IsBroadcast() {
		message.To = make([]int, 0)
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

	for _, peerURL := range p.Peers {
		go func(url string) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			req, _ := http.NewRequestWithContext(ctx, "POST", url+"/message",
				http.NoBody)
			http.DefaultClient.Do(req)
		}(peerURL)
	}
}

func (p *Party) runKeygen() {
	partyIDs := make([]*tsslib.PartyID, p.ID)
	for i := 0; i < p.parties; i++ {
		partyIDs[i] = tsslib.NewPartyID(fmt.Sprintf("%d", i), "", big.NewInt(int64(i)))
	}

	ctx := tsslib.NewPeerContext(partyIDs)
	params := tsslib.NewParameters(tsslib.S256(), ctx, partyIDs[p.ID], p.parties, p.threshold)

	p.outCh = make(chan tsslib.Message, p.parties*p.parties)
	endCh := make(chan *keygen.LocalPartySaveData, 1)

	party := keygen.NewLocalParty(params, p.outCh, endCh)

		go func() {
		if err := party.Start(); err != nil {
			log.Printf("Party %d: Keygen error: %v", p.ID, err)
		}
	}()

	keygenTimeout := time.After(60 * time.Second)
	for {
		select {
		case msg := <-p.outCh:
			p.broadcastMessage(msg)
		case save := <-endCh:
			p.mu.Lock()
			p.keyData = save
			p.mu.Unlock()
			log.Printf("Party %d: Keygen completed successfully", p.ID)
			return
		case <-keygenTimeout:
			log.Printf("Party %d: Keygen timeout", p.ID)
			return
		}
	}
}

func (p *Party) InitKeygen(w http.ResponseWriter, r *http.Request) {
	go p.runKeygen()
	json.NewEncoder(w).Encode(map[string]string{"status": "keygen_started"})
}


func (p *Party) HandleSign(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if p.keyData == nil {
		http.Error(w, "No key data available", http.StatusBadRequest)
		return
	}

	go p.runSigning([]byte(req.Message))
	json.NewEncoder(w).Encode(map[string]string{"status": "signing_started"})
}

func (p *Party) runSigning(msg []byte) {
	log.Printf("Party %d: Starting signing", p.ID)

	partyIDs := make([]*tsslib.PartyID, p.parties)
	for i := 0; i < p.parties; i++ {
		partyIDs[i] = tsslib.NewPartyID(fmt.Sprintf("%d", i), "", big.NewInt(int64(i)))
	}

	ctx := tsslib.NewPeerContext(partyIDs)
	params := tsslib.NewParameters(tsslib.S256(), ctx, partyIDs[p.ID], p.parties, p.threshold)

	p.outCh = make(chan tsslib.Message, p.parties*p.parties)
	endCh := make(chan *common.SignatureData, 1)

	msgHash := new(big.Int).SetBytes(common.SHA512_256(msg))
	party := signing.NewLocalParty(msgHash, params, *p.keyData, p.outCh, endCh)

	go func() {
		if err := party.Start(); err != nil {
			log.Printf("Party %d: Signing error: %v", p.ID, err)
		}
	}()

	signingTimeout := time.After(60 * time.Second)
	for {
		select {
		case msg := <-p.outCh:
			p.broadcastMessage(msg)
		case sig := <-endCh:
			if sig != nil {
				log.Printf("Party %d: Signing completed", p.ID)
				return
			}
		case <-signingTimeout:
			log.Printf("Party %d: Signing timeout", p.ID)
			return
		}
	}
}

func (p *Party) HandleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var msg Message
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("Party %d: Received message from party %d", p.ID, msg.From)
	w.WriteHeader(http.StatusOK)
}

func (p *Party) HandleGetPubKey(w http.ResponseWriter, r *http.Request) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.keyData == nil {
		http.Error(w, "No key data available", http.StatusNotFound)
		return
	}

	pubKey := p.keyData.ECDSAPub
	json.NewEncoder(w).Encode(map[string]interface{}{
		"x": pubKey.X().String(),
		"y": pubKey.Y().String(),
	})
}
