package main

import (
	"context"
	"log"
	"net/http"

	"tack/internal/api"
	"tack/internal/db"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	pool, err := db.OpenPool(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	defer pool.Close()

	handler := api.NewHandler(pool)

	http.ListenAndServe(":3000", handler.Routes())
}
