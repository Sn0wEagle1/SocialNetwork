package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv" // Импортируем пакет для преобразования типа
	"strings"

	"github.com/golang-jwt/jwt/v4"
)

var jwtKey = []byte("your_secret_key") // Используйте защищённый секретный ключ

type contextKey string

const userContextKey contextKey = "userID"

// SetUserContext добавляет UserID в контекст
func SetUserContext(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userContextKey, userID)
}

// GetUserFromContext возвращает UserID из контекста
// Получение userID из контекста
func GetUserIDFromContext(ctx context.Context) (string, error) {
	userID, ok := ctx.Value(userContextKey).(string)
	if !ok {
		return "", fmt.Errorf("user ID not found in context")
	}
	return userID, nil
}

// Middleware для проверки JWT
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Получение токена из заголовка Authorization
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Токен отсутствует", http.StatusUnauthorized)
			return
		}

		// Проверяем формат заголовка
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			http.Error(w, "Некорректный токен", http.StatusUnauthorized)
			return
		}

		// Проверяем валидность токена
		token, claims, err := ParseJWT(tokenString)
		if err != nil || !token.Valid {
			http.Error(w, "Невалидный токен", http.StatusUnauthorized)
			return
		}

		// Преобразуем UserID в строку перед передачей в SetUserContext
		r = r.WithContext(SetUserContext(r.Context(), strconv.Itoa(claims.UserID)))
		next.ServeHTTP(w, r)
	})
}

// Структура claims для хранения ID пользователя
type Claims struct {
	UserID int `json:"user_id"`
	jwt.StandardClaims
}

// ParseJWT парсит и проверяет токен
func ParseJWT(tokenString string) (*jwt.Token, *Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})
	return token, claims, err
}
