package main

import (
	"os"

	"github.com/astaxie/beego/config"
	"github.com/astaxie/beego/logs"
	"landlord/common"
)

var gameConf = &common.GameConfInfo

func initConf() error {
	environment := os.Getenv("ENV")
	if environment != "dev" && environment != "testing" && environment != "product" {
		environment = "product"
	}
	logs.Info("the running environment is : %s", environment)
	conf, err := config.NewConfig("ini", "conf/app.conf")
	if err != nil {
		logs.Error("new conf failed, err: %v", err)
		return err
	}

	environment += "::"
	gameConf.HttpPort, err = conf.Int(environment + "http_port")
	if err != nil {
		logs.Error("init http_port failed, err: %v", err)
		return err
	}
	gameConf.LogPath = conf.String(environment + "log_path")
	if gameConf.LogPath == "" {
		gameConf.LogPath = "./game.log"
	}
	gameConf.LogLevel = conf.String(environment + "log_level")
	if gameConf.LogLevel == "" {
		gameConf.LogLevel = "debug"
	}
	gameConf.DbPath = conf.String(environment + "db_path")
	if gameConf.DbPath == "" {
		gameConf.DbPath = "./db/landlord.db"
	}
	return nil
}
