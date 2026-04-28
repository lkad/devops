package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID       string   `json:"userId"`
	Username     string   `json:"username"`
	Email        string   `json:"email"`
	Roles        []string `json:"roles"`
	Permissions  []string `json:"permissions"`
	Group        string   `json:"group"`
	jwt.RegisteredClaims
}

type JWTService struct {
	secret     []byte
	tokenExpiry time.Duration
}

func NewJWTService(secret string, expiry time.Duration) *JWTService {
	return &JWTService{
		secret:     []byte(secret),
		tokenExpiry: expiry,
	}
}

func (s *JWTService) GenerateToken(user *User) (string, error) {
	claims := &Claims{
		UserID:   user.ID.String(),
		Username: user.Username,
		Email:    user.Email,
		Roles:    user.Roles,
		Group:    user.Group,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.tokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "devops-toolkit",
			Subject:   user.Username,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

func (s *JWTService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

func (s *JWTService) RefreshToken(tokenString string) (string, error) {
	claims, err := s.ValidateToken(tokenString)
	if err != nil {
		return "", err
	}

	user := &User{
		ID:       mustParseUUID(claims.UserID),
		Username: claims.Username,
		Email:    claims.Email,
		Roles:    claims.Roles,
	}

	return s.GenerateToken(user)
}

func mustParseUUID(s string) (uuid [16]byte) {
	// Simplified - in production use github.com/google/uuid
	return
}
