// cmd/main.go
package main

import (
	"log"
	"net/http"
	"social-network/internal"
)

func main() {
	internal.InitDB()

	http.HandleFunc("/", internal.HomeHandler)
	http.HandleFunc("/login", internal.LoginHandler)
	http.HandleFunc("/register", internal.RegisterHandler)
	http.HandleFunc("/posts", internal.PostsHandler)
	http.HandleFunc("/logout", internal.LogoutHandler)
	http.HandleFunc("/profile", internal.ProfileHandler)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	log.Println("Сервер запущен на http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
