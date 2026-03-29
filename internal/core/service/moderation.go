package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	repository "github.com/Revachol/SpamBreaker_VK_back/internal/core/repository/interfaces"
	"github.com/Revachol/SpamBreaker_VK_back/internal/domain"
	"github.com/google/uuid"
)

// ModerationUseCase содержит всю бизнес-логику проверки текстов.
type ModerationUseCase struct {
	classifier domain.Classifier
	repo       repository.MessageRepository
}

func NewModerationUseCase(
	classifier domain.Classifier,
	repo repository.MessageRepository,
) *ModerationUseCase {
	return &ModerationUseCase{
		classifier: classifier,
		repo:       repo,
	}
}

// CheckText — основной юзкейс: валидирует текст, отправляет в ML, сохраняет результат.
func (uc *ModerationUseCase) CheckText(ctx context.Context, text string) (*domain.CheckRecord, error) {
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
		ID:        uuid.NewString(),
		Text:      text,
		Verdict:   *verdict,
		CreatedAt: time.Now().UTC(),
	}

	// Сохраняем в репозиторий. Ошибка сохранения не блокирует ответ клиенту —
	// логируем, но всё равно возвращаем результат.
	if err := uc.repo.Save(ctx, record); err != nil {
		// TODO: заменить на нормальный logger
		fmt.Printf("[WARN] failed to save check record: %v\n", err)
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

// GetRecord возвращает одну запись по ID.
func (uc *ModerationUseCase) GetRecord(ctx context.Context, id string) (*domain.CheckRecord, error) {
	if id == "" {
		return nil, fmt.Errorf("id must not be empty")
	}
	return uc.repo.GetByID(ctx, id)
}
