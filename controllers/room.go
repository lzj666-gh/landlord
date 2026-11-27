package controllers

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/astaxie/beego/logs"
	"landlord/common"
	"landlord/service"
)

//页面里注入给前端脚本的用户信息
type pageUser struct {
	Uid        int    `json:"uid"`
	Username   string `json:"username"`
	RoomId     int    `json:"room_id,omitempty"`
	Coin       int    `json:"coin"`
	Background string `json:"background,omitempty"`
	CardFace   string `json:"cardface,omitempty"`
	Bg         string `json:"bg,omitempty"`
	Card       string `json:"card,omitempty"`
}

type roomListData struct {
	User     string
	LoggedIn bool
	Username string
	Coin     int
	Rooms    []service.RoomInfo
}

type roomPageData struct {
	User     string
	LoggedIn bool
	Username string
	Coin     int
	RoomId   int
	RoomName string
}

//取当前登录用户；cookie 失效时顺手清掉
func currentUser(w http.ResponseWriter, r *http.Request) (common.Session, bool) {
	cookie, err := r.Cookie("session")
	if err != nil {
		return common.Session{}, false
	}
	session, ok := common.GetSession(cookie.Value)
	if !ok {
		clearAuthCookies(w, r)
		return common.Session{}, false
	}
	return session, true
}

func marshalPageUser(session common.Session, loggedIn bool, roomId int, profile common.Profile) string {
	user := pageUser{RoomId: roomId}
	if loggedIn {
		user.Uid = session.UserID
		user.Username = session.Username
		user.Coin = profile.Coin
		user.Background = profile.Background
		user.CardFace = profile.CardFace
		user.Bg = common.ItemAsset(common.SkinBackground, profile.Background)
		user.Card = common.ItemAsset(common.SkinCardFace, profile.CardFace)
	}
	data, err := json.Marshal(user)
	if err != nil {
		logs.Error("marshal page user failed: %v", err)
		return `{"uid":0}`
	}
	return string(data)
}

//当前账号的金币和穿戴；未登录或读取失败时用默认外观
func profileFor(session common.Session, loggedIn bool) common.Profile {
	profile := common.Profile{
		Coin:       common.StartCoin,
		Background: common.DefaultItemID(common.SkinBackground),
		CardFace:   common.DefaultItemID(common.SkinCardFace),
	}
	if !loggedIn {
		return profile
	}
	loaded, err := common.LoadProfile(session.UserID)
	if err != nil {
		logs.Error("load profile for user[%d] failed: %v", session.UserID, err)
		return profile
	}
	return loaded
}

func renderPage(w http.ResponseWriter, name string, data interface{}) {
	t, err := template.ParseFiles("templates/" + name)
	if err != nil {
		logs.Error("can't find template file %s: %v", name, err)
		http.Error(w, "page unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		logs.Error("render template %s failed: %v", name, err)
	}
}

//房间列表：/room/list
func RoomList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	session, loggedIn := currentUser(w, r)
	profile := profileFor(session, loggedIn)
	data := roomListData{
		User:     marshalPageUser(session, loggedIn, 0, profile),
		LoggedIn: loggedIn,
		Coin:     profile.Coin,
		Rooms:    service.RoomsInfo(),
	}
	if loggedIn {
		data.Username = session.Username
	}
	renderPage(w, "roomlist.html", data)
}

//牌桌页：/room/{rid}
func RoomPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ridText := strings.Trim(strings.TrimPrefix(r.URL.Path, "/room/"), "/")
	roomId, err := strconv.Atoi(ridText)
	if err != nil || !service.RoomExists(roomId) {
		http.Redirect(w, r, "/room/list", http.StatusFound)
		return
	}

	session, loggedIn := currentUser(w, r)
	profile := profileFor(session, loggedIn)
	data := roomPageData{
		User:     marshalPageUser(session, loggedIn, roomId, profile),
		LoggedIn: loggedIn,
		Coin:     profile.Coin,
		RoomId:   roomId,
	}
	if loggedIn {
		data.Username = session.Username
	}
	for _, info := range service.RoomsInfo() {
		if info.RoomId == roomId {
			data.RoomName = info.Name
		}
	}
	renderPage(w, "poker.html", data)
}
