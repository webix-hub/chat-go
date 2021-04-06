package main

import (
	"context"
	"fmt"
	"log"
	"mkozhukh/chat/api"
	"mkozhukh/chat/data"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"
	"github.com/unrolled/render"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

type UploadResponse struct {
	Status string `json:"status"`
	Value  string `json:"value"`
}

var format = render.New()

func main() {
	Config.LoadFromFile("./config.yml")

	var conn *gorm.DB
	var err error
	if Config.DB.Path == "" {
		// mysql
		connectString := fmt.Sprintf("%s:%s@tcp(%s)/%s?multiStatements=true&parseTime=true",
			Config.DB.User, Config.DB.Password, Config.DB.Host, Config.DB.Database)
		conn, err = gorm.Open("mysql", connectString)
	} else {
		// sqlite
		conn, err = gorm.Open("sqlite3", Config.DB.Path)
	}

	db := data.NewDAO(conn)
	importDemoData(conn)

	if err != nil {
		log.Fatal("Can't connect to the database", err.Error())
	}
	defer conn.Close()
	//dao := data.NewDAO(conn)

	// File storage
	err = os.MkdirAll(filepath.Join(Config.Server.Data, "avatars"), 0770)
	if err != nil {
		log.Fatal("Can't create data folder", err)
	}

	rapi := api.BuildAPI(db)

	// Router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	crs := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
		MaxAge:           300,
	})
	r.Use(crs.Handler)

	// remote auth
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.Header.Get("Remote-Token")
			if token == "" {
				if r.Method == http.MethodGet {
					token = r.URL.Query().Get("token")
				}
			}

			if token != "" {
				id, err := verifyUserToken([]byte(token))
				if err != nil {
					log.Println("[token]", err.Error())
				} else {
					r = r.WithContext(context.WithValue(r.Context(), "user_id", id))
				}
			}
			next.ServeHTTP(w, r)
		})
	})

	r.Get("/api/v1", rapi.ServeHTTP)
	r.Post("/api/v1", rapi.ServeHTTP)
	r.Get("/api/status", rapi.ServeStatus)

	// DEMO ONLY, imitate login
	r.Get("/login", func(w http.ResponseWriter, r *http.Request) {
		uid, _ := strconv.Atoi(r.URL.Query().Get("id"))
		token, err := createUserToken(uid)
		if err != nil {
			log.Println("[token]", err.Error())
		}
		w.Write(token)
	})

	r.Post("/api/v1/chat/{chatid}/avatar", func(w http.ResponseWriter, r *http.Request) {
		var limit = int64(4 << 20)
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		r.ParseMultipartForm(limit)

		file, _, err := r.FormFile("upload")

		defer file.Close()
		if err != nil {
			log.Println(err.Error())
		}

		chat, _ := db.Chats.UpdateAvatar(chi.URLParam(r, "chatid"), file, Config.Server.Data, Config.Server.Public)
		format.JSON(w, 200, UploadResponse{Status: "server", Value: chat})
	})

	r.Get("/api/v1/chat/{chatid}/avatar/{file_name}", func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "file_name")
		filePath := filepath.Join(Config.Server.Data, name)
		http.ServeFile(w, r, filePath)
	})

	fmt.Println("Listen at port ", Config.Server.Port)
	go startStunServer("udp", Config.Server.Stun)
	err = http.ListenAndServe(Config.Server.Port, r)
	log.Println(err.Error())
}
