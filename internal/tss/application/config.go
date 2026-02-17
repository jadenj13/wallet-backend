package application

import (
	"encoding/json"
	"log"
	"os"
	"strconv"
)

type Config struct {
	ServerPort   uint16
	PartyID      uint16
	Peers        []string
	Threshold    uint16
	Parties      uint16
	RedisAddress string
}

func LoadConfig() Config {
	partyIDEnv := os.Getenv("PARTY_ID")
	peersEnv := os.Getenv("PEERS")

	if partyIDEnv == "" || peersEnv == "" {
		log.Fatal("PARTY_ID and PEERS must be set")
	}

	partyID, err := strconv.ParseUint(partyIDEnv, 10, 16)
	if err != nil {
		log.Fatalf("Invalid PARTY_ID %q: %v", partyIDEnv, err)
	}

	var peers []string
	if err := json.Unmarshal([]byte(peersEnv), &peers); err != nil {
		log.Fatalf("Invalid PEERS %q: %v", peersEnv, err)
	}

	cfg := Config{
		ServerPort:   3000,
		PartyID:      uint16(partyID),
		Peers:        peers,
		Threshold:    1, // t+1 parties needed (2/3)
		Parties:      3,
		RedisAddress: "localhost:6379",
	}

	if serverPort, exists := os.LookupEnv("SERVER_PORT"); exists {
		if port, err := strconv.ParseUint(serverPort, 10, 16); err == nil {
			cfg.ServerPort = uint16(port)
		}
	}

	if redisAddr, exists := os.LookupEnv("REDIS_ADDR"); exists {
		cfg.RedisAddress = redisAddr
	}

	return cfg
}
