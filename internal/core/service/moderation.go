package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
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
	buffer     repository.BufferRepository
	logger     logger.Log
}

func NewModerationUseCase(
	classifier domain.Classifier,
	repo repository.MessageRepository,
	buffer repository.BufferRepository,
	logger logger.Log,
) *ModerationUseCase {
	return &ModerationUseCase{
		classifier: classifier,
		repo:       repo,
		buffer:     buffer,
		logger:     logger,
	}
}

// CheckText — основной юзкейс: валидирует текст, отправляет в ML, сохраняет результат.
// applicationID опционален: передаётся ботом, чтобы привязать запись к приложению.
func (uc *ModerationUseCase) CheckText(
	ctx context.Context,
	text, applicationID, messageID string,
	sendAt time.Time,
) (*domain.CheckRecord, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("text must not be empty")
	}
	if len([]rune(text)) > 5000 {
		return nil, fmt.Errorf("text too long: max 5000 characters")
	}
	msg, err := uc.buffer.List(ctx, applicationID)
	if err != nil {
		uc.logger.Errorf("buffer.List error: %v", err)
		return nil, err
	}
	msg = append(msg, domain.BMessage{Text: text, SendAt: sendAt})

	verdict, err := uc.classifier.Classify(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("classification failed: %w", err)
	}

	record := &domain.CheckRecord{
		ID:            uuid.NewString(),
		MessageID:     strings.TrimSpace(messageID),
		Text:          text,
		Verdict:       *verdict,
		ApplicationID: applicationID,
		CreatedAt:     sendAt,
	}

	uc.saveRecord(ctx, record)

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

// CheckTextForcedNegative сохраняет запись с вердиктом "negative" без вызова ML.
// Используется, когда текст содержит запрещённое слово.
func (uc *ModerationUseCase) CheckTextForcedNegative(ctx context.Context, text, applicationID string) (*domain.CheckRecord, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("text must not be empty")
	}
	record := &domain.CheckRecord{
		ID:   uuid.NewString(),
		Text: text,
		Verdict: domain.Verdict{
			Label:      "negative",
			Confidence: 1.0,
			AllScores:  map[string]float64{"negative": 1.0, "neutral": 0.0, "positive": 0.0},
		},
		ApplicationID: applicationID,
		CreatedAt:     time.Now().UTC(),
	}
	uc.saveRecord(ctx, record)
	return record, nil
}

// GetHistoryByApp возвращает историю проверок для конкретного приложения.
func (uc *ModerationUseCase) GetHistoryByApp(
	ctx context.Context,
	applicationID string,
	limit, offset int,
) ([]*domain.CheckRecord, error) {
	return uc.repo.ListByApplication(ctx, applicationID, limit, offset)
}

// GetRecord возвращает одну запись по ID.
func (uc *ModerationUseCase) GetRecord(ctx context.Context, id string) (*domain.CheckRecord, error) {
	if id == "" {
		return nil, fmt.Errorf("id must not be empty")
	}
	return uc.repo.GetByID(ctx, id)
}

func (uc *ModerationUseCase) saveRecord(ctx context.Context, record *domain.CheckRecord) {
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := uc.repo.Save(ctx, record); err != nil {
			uc.logger.Warnf("failed to save check record: %v\n", err)
		}
	}()

	if uc.buffer != nil && record.ApplicationID != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := uc.buffer.Add(ctx, record.ApplicationID, domain.BMessage{Text: record.Text, SendAt: record.CreatedAt}); err != nil {
				uc.logger.Warnf("failed to save check record to buffer: %v\n", err)
			}
		}()
	}

	wg.Wait()
}
