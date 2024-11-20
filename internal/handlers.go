package internal

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Post struct {
	Author    string
	Content   string
	CreatedAt string
}

type ProfileData struct {
	Username         string
	AvatarURL        string
	RegistrationDate time.Time
	PostCount        int
	FriendCount      int
	IsCurrentUser    bool
	NoPosts          bool
	Posts            []Post
}

// HomeHandler рендерит главную страницу
func HomeHandler(w http.ResponseWriter, r *http.Request) {
	tmplPath := filepath.Join("web", "templates", "index.html")
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		log.Println("Ошибка при загрузке шаблона index.html:", err)
		http.Error(w, "Не удалось загрузить шаблон index.html", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

// LoginHandler рендерит страницу авторизации и проверяет учетные данные
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	var errorMsg string

	if r.Method == http.MethodPost {
		email := r.FormValue("email")
		password := r.FormValue("password")

		// Проверка пользователя в базе данных
		var userID int
		var hashedPassword string
		err := DB.QueryRow("SELECT id, password_hash FROM users WHERE email = $1", email).Scan(&userID, &hashedPassword)
		if err == sql.ErrNoRows || bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password)) != nil {
			errorMsg = "Неверный email или пароль"
		} else if err != nil {
			http.Error(w, "Ошибка при запросе к базе данных", http.StatusInternalServerError)
			return
		}

		// Если ошибки нет, устанавливаем сессию
		if errorMsg == "" {
			err = SetUserIDInSession(w, r, userID)
			if err != nil {
				log.Println("Ошибка при установке сессии:", err)
				http.Error(w, "Ошибка при авторизации", http.StatusInternalServerError)
				return
			}

			// Перенаправление после успешного входа
			http.Redirect(w, r, "/posts", http.StatusSeeOther)
			return
		}
	}

	// Рендеринг страницы входа с возможным сообщением об ошибке
	tmplPath := filepath.Join("web", "templates", "login.html")
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		log.Println("Ошибка при загрузке шаблона login.html:", err)
		http.Error(w, "Не удалось загрузить шаблон", http.StatusInternalServerError)
		return
	}

	// Передаём сообщение об ошибке в шаблон
	tmpl.Execute(w, map[string]string{"ErrorMsg": errorMsg})
}

// RegisterHandler рендерит страницу регистрации и обрабатывает ввод пользователя
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	var errorMsg string
	var formData = map[string]string{
		"Username": "",
		"Email":    "",
	}

	if r.Method == http.MethodPost {
		username := r.FormValue("username")
		email := r.FormValue("email")
		password := r.FormValue("password")

		// Сохраняем данные для повторного отображения в форме
		formData["Username"] = username
		formData["Email"] = email

		// Проверка на пустые поля
		if username == "" || email == "" || password == "" {
			errorMsg = "Все поля обязательны для заполнения"
		} else if len(password) < 5 { // Проверка длины пароля
			errorMsg = "Пароль должен содержать минимум 5 символов"
		} else {
			// Хеширование пароля
			passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err != nil {
				http.Error(w, "Ошибка при хешировании пароля", http.StatusInternalServerError)
				return
			}

			// Попытка сохранить пользователя
			_, err = DB.Exec("INSERT INTO users (username, email, password_hash) VALUES ($1, $2, $3)", username, email, passwordHash)
			if err != nil {
				// Логируем текст ошибки для диагностики
				log.Printf("Ошибка при сохранении пользователя: %v\n", err)

				if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "уникальности") {
					errorMsg = "Пользователь с таким email уже зарегистрирован"
				} else {
					errorMsg = "Не удалось сохранить пользователя из-за неизвестной ошибки"
				}
			}
		}

		// Если ошибок нет, перенаправляем на страницу авторизации
		if errorMsg == "" {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
	}

	// Рендеринг страницы регистрации с сообщением об ошибке и заполненными данными
	tmplPath := filepath.Join("web", "templates", "register.html")
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		log.Println("Ошибка при загрузке шаблона register.html:", err)
		http.Error(w, "Не удалось загрузить шаблон register.html", http.StatusInternalServerError)
		return
	}

	// Передаём сообщение об ошибке и заполненные данные в шаблон
	tmpl.Execute(w, map[string]interface{}{
		"ErrorMsg": errorMsg,
		"FormData": formData,
	})
}

func PostsHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := GetUserIDFromSession(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Получение информации о пользователе
	var username, avatarURL sql.NullString
	err = DB.QueryRow(`
        SELECT username, COALESCE(avatar_url, '/static/avatar.jpg')
        FROM users
        WHERE id = $1
    `, userID).Scan(&username, &avatarURL)
	if err != nil {
		log.Println("Ошибка при получении данных пользователя:", err)
		http.Error(w, "Ошибка при загрузке данных пользователя", http.StatusInternalServerError)
		return
	}

	// Устанавливаем значения для отображения
	userNameValue := "Имя пользователя"
	if username.Valid {
		userNameValue = username.String
	}
	avatarURLValue := "/static/avatar.jpg"
	if avatarURL.Valid {
		avatarURLValue = avatarURL.String
	}

	// Проверяем наличие друзей
	var friendCount int
	err = DB.QueryRow(`
        SELECT COUNT(*)
        FROM friendships
        WHERE user_id = $1
    `, userID).Scan(&friendCount)
	if err != nil {
		log.Println("Ошибка при подсчете друзей:", err)
		http.Error(w, "Ошибка при загрузке данных друзей", http.StatusInternalServerError)
		return
	}

	// Если нет друзей, отображаем сообщение и кнопку "Добавить друзей"
	if friendCount == 0 {
		tmplPath := filepath.Join("web", "templates", "posts.html")
		tmpl, err := template.ParseFiles(tmplPath)
		if err != nil {
			log.Println("Ошибка при загрузке шаблона posts.html:", err)
			http.Error(w, "Не удалось загрузить шаблон posts.html", http.StatusInternalServerError)
			return
		}

		data := struct {
			Username  string
			AvatarURL string
			Posts     []Post
			NoFriends bool
		}{
			Username:  userNameValue,
			AvatarURL: avatarURLValue,
			Posts:     nil,
			NoFriends: true,
		}

		tmpl.Execute(w, data)
		return
	}

	// Загружаем посты друзей
	rows, err := DB.Query(`
        SELECT u.username, p.content, p.created_at
        FROM posts p
        JOIN friendships f ON p.user_id = f.friend_id
        JOIN users u ON p.user_id = u.id
        WHERE f.user_id = $1
        ORDER BY p.created_at DESC
        LIMIT 10
    `, userID)
	if err != nil {
		log.Println("Ошибка при запросе постов:", err)
		http.Error(w, "Ошибка при загрузке постов", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	posts := []Post{}
	for rows.Next() {
		var post Post
		if err := rows.Scan(&post.Author, &post.Content, &post.CreatedAt); err != nil {
			log.Println("Ошибка при чтении данных поста:", err)
			continue
		}
		posts = append(posts, post)
	}

	noPosts := len(posts) == 0

	// Рендеринг шаблона
	tmplPath := filepath.Join("web", "templates", "posts.html")
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		log.Println("Ошибка при загрузке шаблона posts.html:", err)
		http.Error(w, "Не удалось загрузить шаблон posts.html", http.StatusInternalServerError)
		return
	}

	data := struct {
		Username  string
		AvatarURL string
		Posts     []Post
		NoFriends bool
		NoPosts   bool
	}{
		Username:  userNameValue,
		AvatarURL: avatarURLValue,
		Posts:     posts,
		NoFriends: false,
		NoPosts:   noPosts,
	}

	tmpl.Execute(w, data)
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")
	delete(session.Values, "userID") // Удаляем ID пользователя из сессии
	session.Save(r, w)
	http.Redirect(w, r, "/", http.StatusSeeOther) // Перенаправляем на главную
}

func ProfileHandler(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из сессии
	userID, err := GetUserIDFromSession(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Получаем данные пользователя
	var profileData ProfileData
	err = DB.QueryRow(`
		SELECT username, COALESCE(avatar_url, '../static/avatar.jpg'), registration_date
		FROM users
		WHERE id = $1
	`, userID).Scan(&profileData.Username, &profileData.AvatarURL, &profileData.RegistrationDate)
	if err != nil {
		log.Println("Ошибка при получении данных пользователя:", err)
		http.Error(w, "Ошибка при загрузке профиля", http.StatusInternalServerError)
		return
	}

	// Получаем количество постов и друзей пользователя
	err = DB.QueryRow(`
		SELECT COUNT(*) FROM posts WHERE user_id = $1
	`, userID).Scan(&profileData.PostCount)
	if err != nil {
		log.Println("Ошибка при получении количества постов:", err)
	}

	err = DB.QueryRow(`
		SELECT COUNT(*) FROM friendships WHERE user_id = $1
	`, userID).Scan(&profileData.FriendCount)
	if err != nil {
		log.Println("Ошибка при получении количества друзей:", err)
	}

	// Загружаем посты пользователя
	rows, err := DB.Query(`
		SELECT content, created_at FROM posts WHERE user_id = $1 ORDER BY created_at DESC
	`, userID)
	if err != nil {
		log.Println("Ошибка при запросе постов:", err)
		http.Error(w, "Ошибка при загрузке постов", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var post Post
		err = rows.Scan(&post.Content, &post.CreatedAt)
		if err != nil {
			log.Println("Ошибка при чтении данных поста:", err)
			continue
		}
		profileData.Posts = append(profileData.Posts, post)
	}

	// Проверка на наличие постов
	profileData.NoPosts = len(profileData.Posts) == 0
	profileData.IsCurrentUser = true // пока для тестов текущего пользователя

	// Рендерим шаблон профиля
	tmplPath := filepath.Join("web", "templates", "profile.html")
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		log.Println("Ошибка при загрузке шаблона profile.html:", err)
		http.Error(w, "Не удалось загрузить шаблон profile.html", http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, profileData)
	if err != nil {
		log.Println("Ошибка при выполнении шаблона profile.html:", err)
	}
}
