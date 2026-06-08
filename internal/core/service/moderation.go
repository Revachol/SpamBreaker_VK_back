package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	repository "github.com/Revachol/SpamBreaker_VK_back/internal/core/repository/interfaces"
	"github.com/Revachol/SpamBreaker_VK_back/internal/domain"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	"github.com/google/uuid"
)

// ModerationUseCase содержит всю бизнес-логику проверки текстов.
type ModerationUseCase struct {
	classifier domain.Classifier
	repo       repository.MessageRepository
	logger     logger.Log
}

func NewModerationUseCase(
	classifier domain.Classifier,
	repo repository.MessageRepository,
	logger logger.Log,
) *ModerationUseCase {
	return &ModerationUseCase{
		classifier: classifier,
		repo:       repo,
		logger:     logger,
	}
}

// CheckText — основной юзкейс: валидирует текст, отправляет в ML, сохраняет результат.
// applicationID опционален: передаётся ботом, чтобы привязать запись к приложению.
func (uc *ModerationUseCase) CheckText(ctx context.Context, text, applicationID string) (*domain.CheckRecord, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("text must not be empty")
	}
	if len([]rune(text)) > 5000 {
		return nil, fmt.Errorf("text too long: max 5000 characters")
	}

	verdict, err := uc.classifier.Classify(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("classification failed: %w", err)
	}

	record := &domain.CheckRecord{
		ID:            uuid.NewString(),
		Text:          text,
		Verdict:       *verdict,
		ApplicationID: applicationID,
		CreatedAt:     time.Now().UTC(),
	}

	// Сохраняем в репозиторий. Ошибка сохранения не блокирует ответ клиенту —
	// логируем, но всё равно возвращаем результат.
	if err := uc.repo.Save(ctx, record); err != nil {
		uc.logger.Warnf("failed to save check record: %v\n", err)
	}

	return record, nil
}

// GetHistory возвращает историю проверок с пагинацией.
func (uc *ModerationUseCase) GetHistory(ctx context.Context, limit, offset int) ([]*domain.CheckRecord, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return uc.repo.List(ctx, limit, offset)
}

// GetHistoryByApp возвращает историю проверок для конкретного приложения.
func (uc *ModerationUseCase) GetHistoryByApp(
	ctx context.Context,
	applicationID string,
	limit, offset int,
) ([]*domain.CheckRecord, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return uc.repo.ListByApplication(ctx, applicationID, limit, offset)
}

// GetRecord возвращает одну запись по ID.
func (uc *ModerationUseCase) GetRecord(ctx context.Context, id string) (*domain.CheckRecord, error) {
	if id == "" {
		return nil, fmt.Errorf("id must not be empty")
	}
	return uc.repo.GetByID(ctx, id)
}
