package data

import (
	"fmt"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/jadenj13/wallet-backend/internal/api/handler"
)

func Open(path string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := db.AutoMigrate(&handler.User{}, &handler.Device{}, &handler.Session{}); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

type SQLiteUserStore struct{ db *gorm.DB }

func NewUserStore(db *gorm.DB) *SQLiteUserStore { return &SQLiteUserStore{db: db} }

func (s *SQLiteUserStore) Create(u *handler.User) error {
	return s.db.Create(u).Error
}

func (s *SQLiteUserStore) GetByEmail(email string) (*handler.User, error) {
	var u handler.User
	if err := s.db.Where("email = ?", email).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

type SQLiteDeviceStore struct{ db *gorm.DB }

func NewDeviceStore(db *gorm.DB) *SQLiteDeviceStore { return &SQLiteDeviceStore{db: db} }

func (s *SQLiteDeviceStore) Create(d *handler.Device) error {
	return s.db.Create(d).Error
}

func (s *SQLiteDeviceStore) GetByFingerPrint(userID uint, fingerprint string) (*handler.Device, error) {
	var d handler.Device
	if err := s.db.Where("user_id = ? AND fingerprint = ?", userID, fingerprint).First(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *SQLiteDeviceStore) Update(d *handler.Device) error {
	return s.db.Save(d).Error
}

type SQLiteSessionStore struct{ db *gorm.DB }

func NewSessionStore(db *gorm.DB) *SQLiteSessionStore { return &SQLiteSessionStore{db: db} }

func (s *SQLiteSessionStore) Create(sess *handler.Session) error {
	return s.db.Create(sess).Error
}

func (s *SQLiteSessionStore) Get(sessionID uint) (*handler.Session, error) {
	var sess handler.Session
	if err := s.db.First(&sess, sessionID).Error; err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *SQLiteSessionStore) Update(sess *handler.Session) error {
	return s.db.Save(sess).Error
}

type SQLiteTOTPStore struct{ db *gorm.DB }

func NewTOTPStore(db *gorm.DB) *SQLiteTOTPStore { return &SQLiteTOTPStore{db: db} }

func (s *SQLiteTOTPStore) Verify(userID uint, code string) error {
	// TODO: implement real TOTP verification
	return fmt.Errorf("TOTP not implemented")
}
