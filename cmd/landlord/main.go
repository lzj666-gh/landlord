package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/astaxie/beego/logs"
	_ "landlord/router"
)

func main() {
	if err := initConf(); err != nil {
		logs.Error("init conf failed: %v", err)
		return
	}
	if err := initSec(); err != nil {
		logs.Error("initialization failed: %v", err)
		return
	}
	defer func() {
		if gameConf.Db != nil {
			if err := gameConf.Db.Close(); err != nil {
				logs.Error("close sqlite database failed: %v", err)
			}
		}
	}()

	addr := flag.String("addr", fmt.Sprintf(":%d", gameConf.HttpPort), "http service address")
	flag.Parse()
	if err := http.ListenAndServe(*addr, nil); err != nil {
		logs.Error("ListenAndServe failed: %v", err)
	}
}

func init() {
	const pidDir = "pid"
	fileInfo, err := os.Stat(pidDir)
	if os.IsNotExist(err) {
		if err = os.Mkdir(pidDir, os.ModePerm); err != nil {
			logs.Error("create pid directory failed: %v", err)
			return
		}
		fileInfo, err = os.Stat(pidDir)
	}
	if err != nil {
		logs.Error("inspect pid path failed: %v", err)
		return
	}
	if !fileInfo.IsDir() {
		logs.Error("pid path exists and is not a directory")
		return
	}

	pidFile, err := os.OpenFile(pidDir+"/landlord.pid", os.O_RDWR|os.O_CREATE, 0766)
	if err != nil {
		logs.Error("open pid file failed: %v", err)
		return
	}
	defer pidFile.Close()
	if err := pidFile.Truncate(0); err != nil {
		logs.Error("truncate pid file failed: %v", err)
		return
	}
	if _, err := io.WriteString(pidFile, strconv.Itoa(os.Getpid())); err != nil {
		logs.Error("write pid file failed: %v", err)
	}
}
