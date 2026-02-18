package application

import (
	"encoding/json"
	"log"
	"os"
	"strconv"
)

type Config struct {
	ServerPort   uint16
	PartyURLs    []string
	DBPath       string
	JWTSecret    string
	DeviceSecret string
}

func LoadConfig() Config {
	cfg := Config{
		ServerPort:   3000,
		DBPath:       "wallet.db",
		JWTSecret:    "change-me-in-production",
		DeviceSecret: "change-me-in-production",
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

	if dbPath, exists := os.LookupEnv("DB_PATH"); exists {
		cfg.DBPath = dbPath
	}

	if jwtSecret, exists := os.LookupEnv("JWT_SECRET"); exists {
		cfg.JWTSecret = jwtSecret
	}

	if deviceSecret, exists := os.LookupEnv("DEVICE_SECRET"); exists {
		cfg.DeviceSecret = deviceSecret
	}

	return cfg
}
