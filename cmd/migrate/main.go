package main

import (
	"context"
	"database/sql"
	"log"
	"time"

	"followingfeed/config"
	"followingfeed/migrations"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	db, err := sql.Open("mysql", cfg.MySQL.GetDSN())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = db.PingContext(ctx); err != nil {
		log.Fatalf("ping mysql: %v", err)
	}
	if err = migrations.ApplyAll(ctx, db); err != nil {
		log.Fatal(err)
	}
}
