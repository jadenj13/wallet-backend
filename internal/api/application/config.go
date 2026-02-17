package application

import (
	"encoding/json"
	"log"
	"os"
	"strconv"
)

type Config struct {
	RedisAddress string
	ServerPort   uint16
	PartyURLs    []string
}

func LoadConfig() Config {
	cfg := Config{
		RedisAddress: "localhost:6379",
		ServerPort:   3000,
	}

	if redisAddr, exists := os.LookupEnv("REDIS_ADDR"); exists {
		cfg.RedisAddress = redisAddr
	}

	if serverPort, exists := os.LookupEnv("SERVER_PORT"); exists {
		if port, err := strconv.ParseUint(serverPort, 10, 16); err == nil {
			cfg.ServerPort = uint16(port)
		}
	}

	if partyURLs, exists := os.LookupEnv("PARTY_URLS"); exists {
		if err := json.Unmarshal([]byte(partyURLs), &cfg.PartyURLs); err != nil {
			log.Fatalf("Invalid PARTY_URLS %q: %v", partyURLs, err)
		}
	}

	return cfg
}
