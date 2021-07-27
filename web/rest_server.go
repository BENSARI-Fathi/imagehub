package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/BENSARI-Fathi/imagehub/utils"
	"github.com/BENSARI-Fathi/imagehub/web/auth"
	"github.com/BENSARI-Fathi/imagehub/web/db"
	"github.com/BENSARI-Fathi/imagehub/web/middleware"
	"github.com/BENSARI-Fathi/imagehub/web/views"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v7"
	"github.com/joho/godotenv"
	cors "github.com/rs/cors/wrapper/gin"
)

var (
	client *redis.Client
)

func init() {
	if err := godotenv.Load(); err != nil {
		log.Print("No .env file found")
	}
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	// Setup redis client
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	password := os.Getenv("DB_PASSWORD")

	client = utils.NewRedisDB(host, port, password)
	rd := auth.NewAuth(client)
	tk := auth.NewToken()
	mg, err := db.NewMongoClient()
	if err != nil {
		log.Fatal(err)
	}
	account := views.NewAccount(rd, tk, mg)
	repos := views.NewRepository(rd, tk, mg)

	// Set a lower memory limit for multipart forms (default is 32 MiB)
	router.MaxMultipartMemory = utils.MAX_FILE_SIZE

	// setup cors
	router.Use(cors.AllowAll())

	// Setup routing
	router.POST("upload_pp/", middleware.TokenAuthMiddleware(), account.UpdateProfilePicture)

	api := router.Group("api/v1")
	{
		api.GET("user/:id", middleware.TokenAuthMiddleware(), account.UserDetail)
		api.POST("login", account.Login)
		api.GET("logout", middleware.TokenAuthMiddleware(), account.Logout)
		api.POST("token/refresh", account.Refresh)
		api.GET("repos", repos.GetRepos)
		api.GET("repos/archives", repos.GetArchive)
		api.POST("repos/link", repos.GenerateReposUrl)
		api.GET("repos/search", repos.SearchRepository)
		api.GET("repos/:id", repos.GetFolderDetail)
	}

	// serve static and media file
	router.Use(static.Serve("/avatar", static.LocalFile("avatar", false)))
	// router.Use(static.Serve(utils.MEDIA_URL, static.LocalFile("../images", false)))
	router.StaticFS(utils.MEDIA_URL, http.Dir("../images"))
	router.Use(static.Serve("/", static.LocalFile("build", true)))

	// Setup web server
	restPort := os.Getenv("REST_PORT")
	restHost := os.Getenv("REST_HOST")

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", restPort),
		Handler: router,
	}
	go func() {
		log.Printf("Listening and serving HTTP on ==> %s:%s", restHost, restPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()
	//Wait for interrupt signal to gracefully shutdown the server with a timeout of 10 seconds
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("Shutdown Server ...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server Shutdown:", err)
	}
	log.Println("Server exiting")
}
