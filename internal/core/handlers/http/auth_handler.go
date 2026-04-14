package httphandler

import (
	"net/http"

	"github.com/Revachol/SpamBreaker_VK_back/internal/core/service"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	"github.com/gin-gonic/gin"
)

// AuthHandler обрабатывает запросы аутентификации.
type AuthHandler struct {
	auth   *service.AuthUseCase
	logger logger.Log
}

func NewAuthHandler(auth *service.AuthUseCase, l logger.Log) *AuthHandler {
	return &AuthHandler{auth: auth, logger: l}
}

// --- DTOs ---

type registerRequest struct {
	Username        string `json:"username"         binding:"required"`
	Password        string `json:"password"         binding:"required"`
	ConfirmPassword string `json:"confirm_password" binding:"required"`
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type authResponse struct {
	Token    string `json:"token"`
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// --- Handlers ---

// Register godoc
//
//	@Summary     Регистрация модератора
//	@Tags        auth
//	@Accept      json
//	@Produce     json
//	@Param       body body     registerRequest true "Данные для регистрации"
//	@Success     201  {object} authResponse
//	@Failure     400  {object} errorResponse
//	@Router      /api/v1/auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnf("invalid register request: %v", err)
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	result, err := h.auth.Register(c.Request.Context(), service.RegisterInput{
		Username:        req.Username,
		Password:        req.Password,
		ConfirmPassword: req.ConfirmPassword,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, authResponse{
		Token:    result.Token,
		ID:       result.Moderator.ID,
		Username: result.Moderator.Username,
		Role:     result.Moderator.Role,
	})
}

// Login godoc
//
//	@Summary     Вход модератора
//	@Tags        auth
//	@Accept      json
//	@Produce     json
//	@Param       body body     loginRequest true "Логин и пароль"
//	@Success     200  {object} authResponse
//	@Failure     401  {object} errorResponse
//	@Router      /api/v1/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnf("invalid login request: %v", err)
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	result, err := h.auth.Login(c.Request.Context(), service.LoginInput{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, authResponse{
		Token:    result.Token,
		ID:       result.Moderator.ID,
		Username: result.Moderator.Username,
		Role:     result.Moderator.Role,
	})
}
