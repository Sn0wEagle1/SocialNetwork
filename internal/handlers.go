package internal

import (
	"database/sql"
	"fmt"
	"html/template"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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
	if r.Method == http.MethodPost {
		username := r.FormValue("username")
		email := r.FormValue("email")
		password := r.FormValue("password")

		var errorMsg string

		// Проверка имени пользователя
		if len(username) < 4 || len(username) > 20 {
			errorMsg = "Имя пользователя должно быть длиной от 4 до 20 символов"
		} else if strings.Contains(username, " ") {
			errorMsg = "Имя пользователя не должно содержать пробелов"
		} else if len(password) < 5 {
			errorMsg = "Пароль должен быть длиной не менее 5 символов"
		}

		if errorMsg != "" {
			log.Printf("Ошибка проверки: %v\n", errorMsg)

			// Отправляем данные обратно в форму
			tmplPath := filepath.Join("web", "templates", "register.html")
			tmpl, err := template.ParseFiles(tmplPath)
			if err != nil {
				log.Println("Ошибка при загрузке шаблона register.html:", err)
				http.Error(w, "Не удалось загрузить шаблон register.html", http.StatusInternalServerError)
				return
			}

			tmpl.Execute(w, map[string]string{
				"ErrorMsg": errorMsg,
				"Username": username,
				"Email":    email,
			})
			return
		}

		// Хеширование пароля
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("Ошибка хеширования пароля: %v\n", err)
			http.Error(w, "Ошибка при хешировании пароля", http.StatusInternalServerError)
			return
		}

		// Сохранение пользователя в БД
		_, err = DB.Exec("INSERT INTO users (username, email, password_hash) VALUES ($1, $2, $3)", username, email, passwordHash)
		if err != nil {
			log.Printf("Ошибка при сохранении пользователя: %v\n", err)

			if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "уникальности") {
				errorMsg = "Пользователь с таким email уже зарегистрирован"
			} else {
				errorMsg = "Не удалось сохранить пользователя из-за неизвестной ошибки"
			}

			tmplPath := filepath.Join("web", "templates", "register.html")
			tmpl, err := template.ParseFiles(tmplPath)
			if err != nil {
				log.Println("Ошибка при загрузке шаблона register.html:", err)
				http.Error(w, "Не удалось загрузить шаблон register.html", http.StatusInternalServerError)
				return
			}

			tmpl.Execute(w, map[string]string{
				"ErrorMsg": errorMsg,
				"Username": username,
				"Email":    email,
			})
			return
		}

		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	tmplPath := filepath.Join("web", "templates", "register.html")
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		log.Println("Ошибка при загрузке шаблона register.html:", err)
		http.Error(w, "Не удалось загрузить шаблон register.html", http.StatusInternalServerError)
		return
	}

	tmpl.Execute(w, nil)
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
	// Обрабатываем только GET-запросы
	if r.Method != http.MethodGet {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	// Получаем ID пользователя из сессии
	userID, err := GetUserIDFromSession(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Получаем данные пользователя из базы
	var profileData ProfileData
	err = DB.QueryRow(`
		SELECT username, COALESCE(avatar_url, '/static/avatar.jpg'), registration_date
		FROM users
		WHERE id = $1
	`, userID).Scan(&profileData.Username, &profileData.AvatarURL, &profileData.RegistrationDate)
	if err != nil {
		log.Println("Ошибка при получении данных пользователя:", err)
		http.Error(w, "Ошибка при загрузке профиля", http.StatusInternalServerError)
		return
	}

	// Получаем количество постов и друзей
	err = DB.QueryRow(`SELECT COUNT(*) FROM posts WHERE user_id = $1`, userID).Scan(&profileData.PostCount)
	if err != nil {
		log.Println("Ошибка при получении количества постов:", err)
	}

	err = DB.QueryRow(`SELECT COUNT(*) FROM friendships WHERE user_id = $1`, userID).Scan(&profileData.FriendCount)
	if err != nil {
		log.Println("Ошибка при получении количества друзей:", err)
	}

	// Загружаем посты пользователя
	rows, err := DB.Query(`
		SELECT content, created_at
		FROM posts
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		log.Println("Ошибка при запросе постов:", err)
		http.Error(w, "Ошибка при загрузке постов", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var post Post
		var createdAt time.Time
		if err := rows.Scan(&post.Content, &createdAt); err != nil {
			log.Println("Ошибка при чтении поста:", err)
			continue
		}
		post.CreatedAt = createdAt.Format("02.01.2006 15:04")
		posts = append(posts, post)
	}

	// Если нет постов, помечаем
	profileData.Posts = posts
	profileData.NoPosts = len(posts) == 0
	profileData.IsCurrentUser = true

	// Рендерим профиль пользователя
	tmplPath := filepath.Join("web", "templates", "profile.html")
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		log.Println("Ошибка при загрузке шаблона profile.html:", err)
		http.Error(w, "Не удалось загрузить шаблон", http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, profileData)
	if err != nil {
		log.Println("Ошибка при рендеринге шаблона:", err)
	}
}

func CreatePostHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		// Получаем ID текущего пользователя из сессии
		userID, err := GetUserIDFromSession(r)
		if err != nil {
			log.Printf("Ошибка при получении ID пользователя: %v\n", err)
			http.Error(w, "Вы не авторизованы", http.StatusUnauthorized)
			return
		}

		// Получаем данные из формы
		content := r.FormValue("content")
		file, header, err := r.FormFile("image")
		var imagePath string

		if err == nil && header != nil {
			defer file.Close()

			// Сохраняем файл
			imagePath, err = SaveUploadedFile(file, header)
			if err != nil {
				log.Printf("Ошибка при сохранении изображения: %v\n", err)
				http.Error(w, "Ошибка при загрузке файла", http.StatusInternalServerError)
				return
			}
		} else if err != http.ErrMissingFile {
			log.Printf("Ошибка при загрузке файла: %v\n", err)
			http.Error(w, "Ошибка при загрузке файла", http.StatusInternalServerError)
			return
		}

		// Сохраняем пост в базе данных
		_, err = DB.Exec(`
			INSERT INTO posts (user_id, content, image_url, created_at)
			VALUES ($1, $2, $3, NOW())
		`, userID, content, imagePath)
		if err != nil {
			log.Printf("Ошибка при сохранении поста: %v\n", err)
			http.Error(w, "Ошибка при создании поста", http.StatusInternalServerError)
			return
		}

		// Перенаправляем на страницу с постами
		http.Redirect(w, r, "/posts", http.StatusSeeOther)
		return
	}

	// Рендеринг страницы создания поста
	tmplPath := filepath.Join("web", "templates", "create-post.html")
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		log.Printf("Ошибка при загрузке шаблона create-post.html: %v\n", err)
		http.Error(w, "Не удалось загрузить шаблон", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

func SaveUploadedFile(file multipart.File, header *multipart.FileHeader) (string, error) {
	uploadDir := "./uploads"
	err := os.MkdirAll(uploadDir, os.ModePerm)
	if err != nil {
		return "", fmt.Errorf("не удалось создать директорию для загрузки: %v", err)
	}

	filePath := filepath.Join(uploadDir, header.Filename)
	out, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("не удалось создать файл: %v", err)
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		return "", fmt.Errorf("ошибка при сохранении файла: %v", err)
	}

	return filePath, nil
}

func FindFriendsHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := GetUserIDFromSession(r) // Получение текущего пользователя
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		name := r.URL.Query().Get("name")
		var results []User

		if name != "" {
			results, err = FindUsersByName(name, userID)
			if err != nil {
				http.Error(w, "Failed to search users", http.StatusInternalServerError)
				return
			}
		}

		data := struct {
			Results []User
		}{
			Results: results,
		}

		tmplPath := filepath.Join("web", "templates", "find-friends.html")
		tmpl, err := template.ParseFiles(tmplPath)
		if err != nil {
			log.Fatalf("Error parsing template: %v", err)
		}
		tmpl.Execute(w, data)

	case http.MethodPost:
		friendID, err := strconv.Atoi(r.FormValue("friend_id"))
		if err != nil || friendID <= 0 {
			http.Error(w, "Invalid friend ID", http.StatusBadRequest)
			return
		}

		err = AddFriend(userID, friendID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to add friend: %v", err), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/find-friends", http.StatusSeeOther)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
