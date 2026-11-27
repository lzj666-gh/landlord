package controllers

import (
	"net/http"
)

//站点入口：统一跳到房间列表
func Index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/room/list", http.StatusFound)
}
