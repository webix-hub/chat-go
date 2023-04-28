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
	"time"

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
	aDir := filepath.Join(Config.Server.Data, "avatars")
	ensureFolders(aDir)
	fDir := filepath.Join(Config.Server.Data, "files")
	ensureFolders(fDir)

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

	db := data.NewDAO(conn, Config.Features)
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

	rapi := api.BuildAPI(db, Config.Features, Config.Livekit)
	db.SetHub(rapi.Events)

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
				token = r.URL.Query().Get("token")
			}

			if token != "" {
				id, device, err := verifyUserToken([]byte(token))
				if err != nil {
					log.Println("[token]", err.Error())
				} else {
					r = r.WithContext(context.WithValue(context.WithValue(r.Context(), "user_id", id), "device_id", device))
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
		device := newDeviceID()
		token, err := createUserToken(uid, device)
		if err != nil {
			log.Println("[token]", err.Error())
		}
		w.Write(token)
	})

	r.Post("/api/v1/chat/{chatId}/file", func(w http.ResponseWriter, r *http.Request) {
		if !Config.Features.WithFiles {
			panic(data.ErrFeatureDisabled)
		}

		uid := getUserId(r)
		cid := chiIntParam(r, "chatId")
		if !db.UsersCache.HasChat(uid, cid) {
			http.Error(w, "access denied", http.StatusForbidden)
			return
		}

		var limit = int64(10_000_000)
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		r.ParseMultipartForm(limit)

		file, name, err := r.FormFile("upload")
		if err != nil {
			log.Println(err.Error())
		}
		defer file.Close()

		err = db.Files.PostFile(cid, uid, file, name.Filename, fDir, Config.Server.Public)
		if err != nil {
			log.Println("file upload error", err.Error())
			format.JSON(w, 200, UploadResponse{Status: "error"})
		} else {
			format.JSON(w, 200, UploadResponse{Status: "server"})
		}
	})
	r.Get("/api/v1/files/{fileId}/{name}", func(w http.ResponseWriter, r *http.Request) {
		if !Config.Features.WithFiles {
			panic(data.ErrFeatureDisabled)
		}

		fid := chi.URLParam(r, "fileId")
		fInfo := db.Files.GetOne(fid)

		if fInfo.ID == 0 {
			http.Error(w, "", http.StatusNotFound)
			return
		}

		http.ServeFile(w, r, fInfo.Path)
	})

	r.Get("/api/v1/files/{fileId}/preview/{name}", func(w http.ResponseWriter, r *http.Request) {
		if !Config.Features.WithFiles {
			panic(data.ErrFeatureDisabled)
		}

		fid := chi.URLParam(r, "fileId")
		fInfo := db.Files.GetOne(fid)

		if fInfo.ID == 0 {
			http.Error(w, "", http.StatusNotFound)
			return
		}

		http.ServeFile(w, r, fInfo.Path+".preview")
	})

	r.Post("/api/v1/chat/{chatId}/avatar", func(w http.ResponseWriter, r *http.Request) {
		uid := getUserId(r)
		if uid == 0 {
			http.Error(w, "access denied", http.StatusForbidden)
			return
		}

		var limit = int64(4 << 20)
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		r.ParseMultipartForm(limit)

		file, _, err := r.FormFile("upload")
		if err != nil {
			log.Println(err.Error())
			http.Error(w, "can't handle file upload", http.StatusInternalServerError)
			return
		}
		defer file.Close()

		chat, _ := db.Chats.UploadAvatar(file, aDir, Config.Server.Public)
		format.JSON(w, 200, UploadResponse{Status: "server", Value: chat})
	})

	r.Get("/api/v1/chat/{chatId}/avatar/{file_name}", func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "file_name")
		filePath := filepath.Join(aDir, name)
		http.ServeFile(w, r, filePath)
	})

	fmt.Println("Listen at port ", Config.Server.Port)
	err = http.ListenAndServe(Config.Server.Port, r)
	log.Println(err.Error())
}

var dID = time.Now().Unix()

func newDeviceID() int64 {
	dID += 1
	return dID
}

func chiIntParam(r *http.Request, key string) int {
	val := chi.URLParam(r, key)
	res, err := strconv.Atoi(val)
	if err != nil {
		return 0
	}

	return res
}

func ensureFolders(path string) {
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		return
	}

	err = os.MkdirAll(path, os.ModePerm)
	if err != nil {
		log.Fatal("can't create working dir: ", path)
	}
}

func getUserId(r *http.Request) int {
	u := r.Context().Value("user_id")
	if u == nil {
		return 0
	}

	t, ok := u.(int)
	if !ok {
		return 0
	}

	return t
}

func getDeviceId(r *http.Request) int {
	u := r.Context().Value("device_id")
	if u == nil {
		return 0
	}

	t, ok := u.(int)
	if !ok {
		return 0
	}

	return t
}
