package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"context"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthLevel int

const (
	AuthLevelNone AuthLevel = iota
	AuthLevelDevice
	AuthLevelPassword
	AuthLevelBiometric
	AuthLevel2FA
	AuthLevelFull
)

type UserStore interface {
	Create(user *User) error
	GetByEmail(email string) (*User, error)
}

type DeviceStore interface {
	Create(device *Device) error
	GetByFingerPrint(userID uint, fingerprint string) (*Device, error)
	Update(device *Device) error
}

type SessionStore interface {
	Create(session *Session) error
	Get(sessionID uint) (*Session, error)
	Update(session *Session) error
}

type TOTPStore interface {
	Verify(userID uint, code string) error
}

type AuthService struct{
	jwtSecret []byte
	deviceSecret []byte
	sessionStore SessionStore
	deviceStore DeviceStore
	userStore UserStore
	totpStore TOTPStore
}

type Device struct{
	ID          uint      `json:"id"`
	UserID      uint      `json:"user_id"      gorm:"not null"`
	Fingerprint string    `json:"fingerprint"  gorm:"not null"`
	Name        string    `json:"name"`
	Platform    string    `json:"platform"`
	FirstSeenIP string    `json:"first_seen_ip"`
	LastSeenIP  string    `json:"last_seen_ip"`
	LastSeenAt  time.Time `json:"last_seen_at"`
	TrustedAt   time.Time `json:"trusted_at"`
	IsTrusted   bool      `json:"is_trusted"`
	CreatedAt   time.Time `json:"created_at"`
}

type Session struct{
	ID             uint      `json:"id"`
	UserID         uint      `json:"user_id"         gorm:"not null"`
	DeviceID       uint      `json:"device_id"       gorm:"not null"`
	AuthLevel      AuthLevel `json:"auth_level"`
	IPAddress      string    `json:"ip_address"`
	ExpiresAt      time.Time `json:"expires_at"`
	CreatedAt      time.Time `json:"created_at"`
	LastActivityAt time.Time `json:"last_activity_at"`
}

type User struct{
	ID                uint      `json:"id"`
	Email             string    `json:"email"              gorm:"uniqueIndex;not null"`
	PasswordHash      string    `json:"-"`
	WalletAddress     string    `json:"wallet_address"`
	TOTPEnabled       bool      `json:"totp_enabled"`
	BiometricEnabled  bool      `json:"biometric_enabled"`
	RequireDeviceAuth bool      `json:"require_device_auth"`
	CreatedAt         time.Time `json:"created_at"`
}

type DeviceFingerprint struct{
	UserAgent      string            `json:"user_agent"`
	AcceptLanguage string            `json:"accept_language"`
	ScreenRes      string            `json:"screen_res"`
	Timezone       string            `json:"timezone"`
	Platform       string            `json:"platform"`
	Plugins        []string          `json:"plugins"`
	Canvas         string            `json:"canvas"`
	WebGL          string            `json:"webgl"`
	Fonts          []string          `json:"fonts"`
	AudioContext   string            `json:"audio_context"`
	ClientHints    map[string]string `json:"client_hints"`
}

func (df *DeviceFingerprint) Hash() string {
	data, _ := json.Marshal(df)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

type JWTClaims struct {
	UserID      uint      `json:"user_id"`
	SessionID   uint      `json:"session_id"`
	DeviceID    uint      `json:"device_id"`
	AuthLevel   AuthLevel `json:"auth_level"`
	WalletAddr  string    `json:"wallet_address"`
	jwt.RegisteredClaims
}

func NewAuthService(jwtSecret, deviceSecret []byte, sessionStore SessionStore, deviceStore DeviceStore, userStore UserStore, totpStore TOTPStore) *AuthService {
	return &AuthService{
		jwtSecret:    jwtSecret,
		deviceSecret: deviceSecret,
		sessionStore: sessionStore,
		deviceStore:  deviceStore,
		userStore:    userStore,
		totpStore:    totpStore,
	}
}

type RegisterRequest struct{
	Email       string            `json:"email"`
	Password    string            `json:"password"`
	Fingerprint DeviceFingerprint `json:"fingerprint"`
}

func (a *AuthService) Register(req RegisterRequest, ip string) (*User, *Device, error) {
	if len(req.Password) < 12 {
		return nil, nil, errors.New("Password must be at least 12 characters")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, nil, err
	}

	user := &User{
		Email:             req.Email,
		PasswordHash:      string(hash),
		RequireDeviceAuth: true,
		CreatedAt:         time.Now(),
	}

	if err := a.userStore.Create(user); err != nil {
		return nil, nil, err
	}

	device := &Device{
		UserID:      user.ID,
		Fingerprint: req.Fingerprint.Hash(),
		Platform:    req.Fingerprint.Platform,
		FirstSeenIP: ip,
		LastSeenIP:  ip,
		LastSeenAt:  time.Now(),
		TrustedAt:   time.Now(),
		IsTrusted:   true,
		CreatedAt:   time.Now(),
	}

	if err := a.deviceStore.Create(device); err != nil {
		return nil, nil, err
	}

	return user, device, nil
}

type LoginRequest struct {
	Email       string            `json:"email"`
	Password    string            `json:"password"`
	Fingerprint DeviceFingerprint `json:"fingerprint"`
	TOTPCode    string            `json:"totp_code,omitempty"`
	BiometricSig string           `json:"biometric_sig,omitempty"`
}

type LoginResponse struct {
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token"`
	User         *User     `json:"user"`
	AuthLevel    AuthLevel `json:"auth_level"`
	RequiresMFA  bool      `json:"requires_mfa"`
	NewDevice    bool      `json:"new_device"`
}

func (a *AuthService) Login(req LoginRequest, ip string) (*LoginResponse, error) {
	user, err := a.userStore.GetByEmail(req.Email)
	if err != nil {
		return nil, errors.New("Invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, errors.New("Invalid credentials")
	}

	authLevel := AuthLevelPassword

	fingerprint := req.Fingerprint.Hash()
	device, err := a.deviceStore.GetByFingerPrint(user.ID, fingerprint)

	newDevice := false
	if err != nil || device == nil {
		device = &Device{
			UserID:      user.ID,
			Fingerprint: fingerprint,
			Platform:    req.Fingerprint.Platform,
			FirstSeenIP: ip,
			LastSeenIP:  ip,
			LastSeenAt:  time.Now(),
			IsTrusted:   false,
			CreatedAt:   time.Now(),
		}
		a.deviceStore.Create(device)
		newDevice = true
	} else {
		device.LastSeenIP = ip
		device.LastSeenAt = time.Now()
		a.deviceStore.Update(device)
		
		if device.IsTrusted {
			authLevel = AuthLevelDevice
		}
	}

	requiresMFA := user.TOTPEnabled || newDevice || !device.IsTrusted

	if requiresMFA {
		if user.TOTPEnabled && req.TOTPCode != "" {
			if err := a.totpStore.Verify(user.ID, req.TOTPCode); err != nil {
				return nil, errors.New("Invalid 2FA code")
			}
			authLevel = AuthLevel2FA
		}

		if user.BiometricEnabled && req.BiometricSig != "" {
			if err := a.verifyBiometric(user.ID, req.BiometricSig); err != nil {
				return nil, errors.New("Biometric verification failed")
			}
			authLevel = AuthLevelBiometric
		}

		if newDevice && authLevel < AuthLevel2FA {
			return &LoginResponse{
				User:         user,
				AuthLevel:    authLevel,
				RequiresMFA:  true,
				NewDevice:    true,
			}, nil
		}
	}

	session := &Session{
		UserID:         user.ID,
		DeviceID:       device.ID,
		AuthLevel:      authLevel,
		IPAddress:      ip,
		ExpiresAt:      time.Now().Add(24 * time.Hour),
		CreatedAt:      time.Now(),
		LastActivityAt: time.Now(),
	}

	if err := a.sessionStore.Create(session); err != nil {
		return nil, err
	}

	token, err := a.generateJWT(user, session, device)
	if err != nil {
		return nil, err
	}

	refreshToken, err := a.generateRefreshToken()
	if err != nil {
		return nil, err
	}

	// TODO: store hash of refresh token with session

	return &LoginResponse{
		Token:        token,
		RefreshToken: refreshToken,
		User:         user,
		AuthLevel:    authLevel,
		RequiresMFA:  false,
		NewDevice:    newDevice,
	}, nil
}

func (a *AuthService) generateJWT(user *User, session *Session, device *Device) (string, error) {
	claims := JWTClaims{
		UserID:     user.ID,
		SessionID:  session.ID,
		DeviceID:   device.ID,
		AuthLevel:  session.AuthLevel,
		WalletAddr: user.WalletAddress,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   strconv.FormatUint(uint64(user.ID), 10),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.jwtSecret)
}

func (a *AuthService) generateRefreshToken() (string, error) {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return "", err
	}
	
	encoded := base64.URLEncoding.EncodeToString(token)
	return encoded, nil
}

func (a *AuthService) verifyBiometric(userID uint, signature string) error {
	// Not yet implemented
	return nil
}

func (a *AuthService) AuthMiddleware(minLevel AuthLevel) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Missing authorization header", http.StatusUnauthorized)
				return
			}

			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			
			token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("Unexpected signing method")
				}
				return a.jwtSecret, nil
			})

			if err != nil || !token.Valid {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			claims, ok := token.Claims.(*JWTClaims)
			if !ok {
				http.Error(w, "Invalid token claims", http.StatusUnauthorized)
				return
			}

			session, err := a.sessionStore.Get(claims.SessionID)
			if err != nil || session == nil {
				http.Error(w, "Session expired", http.StatusUnauthorized)
				return
			}

			if time.Now().After(session.ExpiresAt) {
				http.Error(w, "Session expired", http.StatusUnauthorized)
				return
			}

			if session.AuthLevel < minLevel {
				http.Error(w, "Insufficient authentication level", http.StatusForbidden)
				return
			}

			session.LastActivityAt = time.Now()
			a.sessionStore.Update(session)

			r = r.WithContext(setAuthContext(r.Context(), claims))
			
			next.ServeHTTP(w, r)
		})
	}
}

type contextKey string

const authContextKey contextKey = "auth_claims"

func setAuthContext(ctx context.Context, claims *JWTClaims) context.Context {
	return context.WithValue(ctx, authContextKey, claims)
}

func GetAuthClaims(ctx context.Context) *JWTClaims {
	claims, _ := ctx.Value(authContextKey).(*JWTClaims)
	return claims
}

func (a *AuthService) HandleRegister(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	user, _, err := a.Register(req, r.RemoteAddr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

func (a *AuthService) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp, err := a.Login(req, r.RemoteAddr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if resp.RequiresMFA {
		w.WriteHeader(http.StatusAccepted)
	}
	json.NewEncoder(w).Encode(resp)
}
