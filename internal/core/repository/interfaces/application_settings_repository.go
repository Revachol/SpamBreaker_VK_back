package interfaces

import (
	"context"

	"github.com/Revachol/SpamBreaker_VK_back/internal/domain"
)

// ApplicationSettingsRepository — порт для работы с настройками приложений.
type ApplicationSettingsRepository interface {
	Create(ctx context.Context, settings *domain.ApplicationSettings) error
	GetByApplicationID(ctx context.Context, applicationID string) (*domain.ApplicationSettings, error)
	Update(ctx context.Context, settings *domain.ApplicationSettings) error
	Delete(ctx context.Context, id string) error
}
