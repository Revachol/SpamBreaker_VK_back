package httphandler

import (
	"net/http"
	"strings"

	jwtpkg "github.com/Revachol/SpamBreaker_VK_back/pkg/jwt"
	"github.com/gin-gonic/gin"
)

// Ключи контекста для данных из токена.
const (
	CtxModeratorID = "moderator_id"
	CtxUsername    = "moderator_username"
	CtxRole        = "moderator_role"
)

// User ID key for context
const CtxUserID = "user_id"

// JWTMiddleware проверяет Bearer-токен в заголовке Authorization.
// Используется для защищённых маршрутов.
func JWTMiddleware(jwtManager *jwtpkg.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, errorResponse{
				Error: "authorization header required",
			})
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, errorResponse{
				Error: "use format: Bearer <token>",
			})
			return
		}

		claims, err := jwtManager.Parse(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, errorResponse{
				Error: err.Error(),
			})
			return
		}

		c.Set(CtxModeratorID, claims.ModeratorID)
		c.Set(CtxUserID, claims.ModeratorID) // For backward compatibility with existing handlers
		c.Set(CtxUsername, claims.Username)
		c.Set(CtxRole, claims.Role)
		c.Next()
	}
}
