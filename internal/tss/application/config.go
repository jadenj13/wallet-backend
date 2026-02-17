package application

import (
	"os"
	"strconv"
	"fmt"
	"log"
	"encoding/json"
)

type Config struct {
	ServerPort   uint16
	PartyID uint16
	Peers []string
	Threshold uint16
	Parties uint16
}

func LoadConfig() Config {
	partyIDEnv := os.Getenv("PARTY_ID")
	peersEnv := os.Getenv("PEERS")

	if partyIDEnv == "" || peersEnv == "" {
		log.Fatal("PARTY_ID and PEERS must be set")
	}

	var partyID uint16 = 0
	fmt.Sscanf(partyIDEnv, "%d", &partyID)

	var peers []string
	if peersEnv != "" {
		json.Unmarshal([]byte(peersEnv), &peers)
	}

	cfg := Config{
		ServerPort:   3000,
		PartyID: partyID,
		Peers: peers,
		Threshold: 1, // t+1 parties needed (2/3)
		Parties: 3,
	}

	if serverPort, exists := os.LookupEnv("SERVER_PORT"); exists {
		if port, err := strconv.ParseUint(serverPort, 10, 16); err == nil {
			cfg.ServerPort = uint16(port)
		}
	}

	return cfg
}
