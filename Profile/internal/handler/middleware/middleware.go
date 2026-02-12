package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
)

const (
	authorizationHeader = "Authorization"
)

func AuthMiddleware(jwtSecret string, log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader(authorizationHeader)
		if header == "" {
			log.Warn("auth middlware: auth header is empty")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "auth header is empty",
			})
			return
		}

		headerParts := strings.Split(header, " ")
		if headerParts[0] != "Bearer" || len(headerParts) != 2 {
			log.Warn("auth middleware: invalid auth header format")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid auth header format",
			})
			return
		}

		if len(headerParts[1]) == 0 {
			log.Error("auth middleware: token is empty")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "token is empty",
			})
			return
		}

		tokenString := headerParts[1]

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return []byte(jwtSecret), nil
		})

		if err != nil {
			log.Error("auth middleware: failed to parse token", slog.Any("error", err))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid token",
			})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)

		if !ok || !token.Valid {
			log.Warn("auth middleware: token is not valid or claims are corrupted")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "token is not valid",
			})
			return
		}

		userID, ok := claims["sub"].(string)
		if !ok {
			log.Warn("auth middlware: 'sub' claims is missing or not a string")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid token payload",
			})
			return
		}

		userName, ok := claims["name"].(string)
		if !ok {
			log.Warn("auth middleware: 'name' claim is missing or not a string")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid token payload",
			})
			return 
		}

		c.Set("userID", userID)
		c.Set("userName", userName)
		c.Next()
	}
}
