package main

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/astaxie/beego/logs"
	_ "github.com/mattn/go-sqlite3"
)

func conversionLogLevel(logLevel string) int {
	switch logLevel {
	case "debug":
		return logs.LevelDebug
	case "warn":
		return logs.LevelWarn
	case "info":
		return logs.LevelInfo
	case "trace":
		return logs.LevelTrace
	default:
		return logs.LevelDebug
	}
}

func initLogger() error {
	config := map[string]interface{}{
		"filename": gameConf.LogPath,
		"level":    conversionLogLevel(gameConf.LogLevel),
	}
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal logger configuration: %v", err)
	}
	if err := logs.SetLogger(logs.AdapterFile, string(configJSON)); err != nil {
		return fmt.Errorf("configure logger: %v", err)
	}
	return nil
}

func initSqlite() error {
	var err error
	gameConf.Db, err = sql.Open("sqlite3", gameConf.DbPath)
	if err != nil {
		return fmt.Errorf("open sqlite database: %v", err)
	}
	const schema = `CREATE TABLE IF NOT EXISTS "account" (
		"id" INTEGER NOT NULL,
		"email" TEXT(32),
		"username" TEXT(16),
		"password" TEXT(100),
		"coin" INTEGER,
		"created_date" TEXT(32),
		"updated_date" TEXT(32),
		PRIMARY KEY ("id")
	)`
	if _, err := gameConf.Db.Exec(schema); err != nil {
		return fmt.Errorf("create account table: %v", err)
	}
	const itemSchema = `CREATE TABLE IF NOT EXISTS "account_item" (
		"account_id" INTEGER NOT NULL,
		"item_id" TEXT(32) NOT NULL,
		"created_date" TEXT(32),
		PRIMARY KEY ("account_id", "item_id")
	)`
	if _, err := gameConf.Db.Exec(itemSchema); err != nil {
		return fmt.Errorf("create account_item table: %v", err)
	}
	const skinSchema = `CREATE TABLE IF NOT EXISTS "account_skin" (
		"account_id" INTEGER NOT NULL,
		"background" TEXT(32),
		"cardface" TEXT(32),
		"updated_date" TEXT(32),
		PRIMARY KEY ("account_id")
	)`
	if _, err := gameConf.Db.Exec(skinSchema); err != nil {
		return fmt.Errorf("create account_skin table: %v", err)
	}
	return nil
}

func initSec() error {
	if err := initLogger(); err != nil {
		return err
	}
	if err := initSqlite(); err != nil {
		return err
	}
	logs.Info("initialization succeeded")
	return nil
}
