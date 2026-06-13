package service

import (
	"context"
	"fmt"
	"strings"

	repository "github.com/Revachol/SpamBreaker_VK_back/internal/core/repository/interfaces"
	"github.com/Revachol/SpamBreaker_VK_back/internal/domain"
	jwtpkg "github.com/Revachol/SpamBreaker_VK_back/pkg/jwt"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// AuthUseCase содержит бизнес-логику регистрации и входа.
type AuthUseCase struct {
	repo       repository.ModeratorRepository
	jwtManager *jwtpkg.Manager
	logger     logger.Log
}

func NewAuthUseCase(
	repo repository.ModeratorRepository,
	jwtManager *jwtpkg.Manager,
	l logger.Log,
) *AuthUseCase {
	return &AuthUseCase{repo: repo, jwtManager: jwtManager, logger: l}
}

// --- Input / Output ---

type RegisterInput struct {
	Username        string
	Password        string
	ConfirmPassword string
}

type LoginInput struct {
	Username string
	Password string
}

type AuthResult struct {
	Token     string
	Moderator *domain.Moderator
}

// --- Use Cases ---

// Register создаёт нового модератора и возвращает JWT.
func (uc *AuthUseCase) Register(ctx context.Context, input RegisterInput) (*AuthResult, error) {
	input.Username = strings.TrimSpace(input.Username)

	// Валидация.
	if input.Username == "" {
		return nil, fmt.Errorf("username must not be empty")
	}
	if len(input.Username) < 3 || len(input.Username) > 64 {
		return nil, fmt.Errorf("username must be between 3 and 64 characters")
	}
	if len(input.Password) < 6 {
		return nil, fmt.Errorf("password must be at least 6 characters")
	}
	if input.Password != input.ConfirmPassword {
		return nil, fmt.Errorf("passwords do not match")
	}

	// Проверяем уникальность.
	existing, err := uc.repo.GetByUsername(ctx, input.Username)
	if err != nil {
		return nil, fmt.Errorf("register: check username: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("username already taken")
	}

	// Хешируем пароль.
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		uc.logger.Errorf("Error hashing password: %s", err)
		return nil, fmt.Errorf("register: hash password: %w", err)
	}

	mod := &domain.Moderator{
		ID:           uuid.NewString(),
		Username:     input.Username,
		PasswordHash: string(hash),
		IsActive:     true,
	}

	if err := uc.repo.Create(ctx, mod); err != nil {
		return nil, fmt.Errorf("register: save moderator: %w", err)
	}

	token, err := uc.jwtManager.Generate(mod.ID, mod.Username)
	if err != nil {
		uc.logger.Errorf("Error generating token: %s", err)
		return nil, fmt.Errorf("register: generate token: %w", err)
	}

	return &AuthResult{Token: token, Moderator: mod}, nil
}

// Login проверяет пароль и возвращает JWT.
func (uc *AuthUseCase) Login(ctx context.Context, input LoginInput) (*AuthResult, error) {
	input.Username = strings.TrimSpace(input.Username)

	if input.Username == "" {
		return nil, fmt.Errorf("username must not be empty")
	}
	if input.Password == "" {
		return nil, fmt.Errorf("password must not be empty")
	}

	mod, err := uc.repo.GetByUsername(ctx, input.Username)
	if err != nil {
		return nil, fmt.Errorf("login: %w", err)
	}
	// Одинаковое сообщение для "не найден" и "неверный пароль" — защита от перебора.
	if mod == nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	if !mod.IsActive {
		return nil, fmt.Errorf("account is disabled")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(mod.PasswordHash), []byte(input.Password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	token, err := uc.jwtManager.Generate(mod.ID, mod.Username)
	if err != nil {
		uc.logger.Errorf("Error generating token: %s", err)
		return nil, fmt.Errorf("login: generate token: %w", err)
	}

	return &AuthResult{Token: token, Moderator: mod}, nil
}
