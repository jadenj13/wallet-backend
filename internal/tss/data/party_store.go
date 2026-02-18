package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bnb-chain/tss-lib/v2/ecdsa/keygen"
	"github.com/redis/go-redis/v9"
)

type RedisPartyStore struct {
	rdb *redis.Client
}

func NewRedisPartyStore(rdb *redis.Client) *RedisPartyStore {
	return &RedisPartyStore{rdb: rdb}
}

func (s *RedisPartyStore) SaveKeyData(ctx context.Context, partyID int, userID string, data *keygen.LocalPartySaveData) error {
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal key data: %w", err)
	}
	return s.rdb.Set(ctx, keyDataKey(partyID, userID), b, 0).Err()
}

func (s *RedisPartyStore) LoadKeyData(ctx context.Context, partyID int, userID string) (*keygen.LocalPartySaveData, error) {
	b, err := s.rdb.Get(ctx, keyDataKey(partyID, userID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get key data: %w", err)
	}

	var data keygen.LocalPartySaveData
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, fmt.Errorf("unmarshal key data: %w", err)
	}

	if data.Xi == nil || data.ECDSAPub == nil {
		return nil, fmt.Errorf("loaded key data is invalid: missing required fields")
	}

	return &data, nil
}

func keyDataKey(partyID int, userID string) string {
	return fmt.Sprintf("tss:party:%d:user:%s:keydata", partyID, userID)
}
