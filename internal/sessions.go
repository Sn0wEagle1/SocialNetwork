// internal/sessions.go
package internal

import (
	"errors"
	"net/http"

	"github.com/gorilla/sessions"
)

// Инициализируем хранилище сессий с секретным ключом
var store = sessions.NewCookieStore([]byte("4kukTPS-gIf-fW2D-QMZjWflvbcHvz50fnJKBszXfAA"))

// GetUserIDFromSession возвращает ID пользователя из сессии
func GetUserIDFromSession(r *http.Request) (int, error) {
	session, err := store.Get(r, "session-name")
	if err != nil {
		return 0, err
	}
	userID, ok := session.Values["userID"].(int)
	if !ok {
		return 0, errors.New("пользователь не авторизован")
	}
	return userID, nil
}

// SetUserIDInSession сохраняет ID пользователя в сессии
func SetUserIDInSession(w http.ResponseWriter, r *http.Request, userID int) error {
	session, err := store.Get(r, "session-name")
	if err != nil {
		return err
	}
	session.Values["userID"] = userID
	return session.Save(r, w)
}
