package internal

import (
	"errors"
)

// User представляет пользователя в системе
type User struct {
	ID       int    // ID пользователя
	Username string // Имя пользователя
}

// FindUsersByName ищет пользователей по имени, исключая текущего пользователя
func FindUsersByName(name string, currentUserID int) ([]User, error) {
	query := `
		SELECT id, username
		FROM users
		WHERE username ILIKE $1 AND id != $2
	`
	rows, err := DB.Query(query, "%"+name+"%", currentUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Username); err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}

// AddFriend добавляет дружескую связь
func AddFriend(userID, friendID int) error {
	// Проверяем, существует ли уже дружба
	var count int
	err := DB.QueryRow(`
		SELECT COUNT(*)
		FROM friendships
		WHERE (user_id = $1 AND friend_id = $2) OR (user_id = $2 AND friend_id = $1)
	`, userID, friendID).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("friendship already exists")
	}

	// Вставляем новую запись дружбы
	_, err = DB.Exec(`
		INSERT INTO friendships (user_id, friend_id)
		VALUES ($1, $2)
	`, userID, friendID)
	return err
}
