package service

import (
	"errors"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"riskledger/internal/domain"
)

func (s *Service) LoginWithOTP(email, password, code string) (domain.User, error) {
	user, err := s.Login(email, password)
	if err != nil {
		return domain.User{}, err
	}
	if user.TwoFactorEnabled && !totp.Validate(strings.TrimSpace(code), user.TwoFactorSecret) {
		return domain.User{}, errors.New("two-factor code required or invalid")
	}
	return user, nil
}

func (s *Service) Enable2FA(userID string) (string, error) {
	user, ok := s.store.UserByID(userID)
	if !ok {
		return "", errors.New("user not found")
	}
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "RiskLedger", AccountName: user.Email, SecretSize: 20, Algorithm: otp.AlgorithmSHA1})
	if err != nil {
		return "", err
	}
	user.TwoFactorSecret = key.Secret()
	user.TwoFactorEnabled = true
	user.UpdatedAt = time.Now()
	if err := s.store.UpdateUser(user); err != nil {
		return "", err
	}
	return key.URL(), nil
}
