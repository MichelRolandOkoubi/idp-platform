package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	secret []byte
	db     DBInterface
}

type DBInterface interface {
	GetUserByUsername(username string) (*User, error)
	CreateUser(u *User) error
}

type User struct {
	ID           string
	Username     string
	Email        string
	PasswordHash string
	Roles        []string
}

type Claims struct {
	jwt.RegisteredClaims
	UserID   string   `json:"uid"`
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
}

func NewService(secret string, db DBInterface) *Service {
	return &Service{secret: []byte(secret), db: db}
}

func (s *Service) Login(username, password string) (string, *User, error) {
	user, err := s.db.GetUserByUsername(username)
	if err != nil {
		return "", nil, errors.New("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash), []byte(password),
	); err != nil {
		return "", nil, errors.New("invalid credentials")
	}

	token, err := s.generateToken(user)
	return token, user, err
}

func (s *Service) generateToken(user *User) (string, error) {
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   user.ID,
		},
		UserID:   user.ID,
		Username: user.Username,
		Roles:    user.Roles,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

func (s *Service) ValidateToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}
