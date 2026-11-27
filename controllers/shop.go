package controllers

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/astaxie/beego/logs"
	"landlord/common"
)

type shopState struct {
	Coin     int               `json:"coin"`
	Equipped shopEquipped      `json:"equipped"`
	Items    []common.ShopItem `json:"items"`
}

type shopEquipped struct {
	Background string `json:"background"`
	CardFace   string `json:"cardface"`
}

type shopPageData struct {
	User     string
	LoggedIn bool
	Username string
	Coin     int
	Items    []common.ShopItem
	Equipped shopEquipped
}

//商品目录 + 账号的拥有/穿戴状态
func buildShopState(userID int) (shopState, error) {
	profile, err := common.LoadProfile(userID)
	if err != nil {
		return shopState{}, err
	}
	owned, err := common.OwnedItems(userID)
	if err != nil {
		return shopState{}, err
	}
	state := shopState{
		Coin: profile.Coin,
		Equipped: shopEquipped{
			Background: profile.Background,
			CardFace:   profile.CardFace,
		},
		Items: common.ShopItems(),
	}
	for i := range state.Items {
		item := &state.Items[i]
		item.Owned = item.Default || owned[item.ID]
		item.Equipped = (item.Kind == common.SkinBackground && item.ID == profile.Background) ||
			(item.Kind == common.SkinCardFace && item.ID == profile.CardFace)
	}
	return state, nil
}

//商店页：/shop
func ShopPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	session, loggedIn := currentUser(w, r)
	if !loggedIn {
		http.Redirect(w, r, "/room/list", http.StatusFound)
		return
	}
	state, err := buildShopState(session.UserID)
	if err != nil {
		logs.Error("load shop state for user[%d] failed: %v", session.UserID, err)
		http.Error(w, "商店暂时不可用", http.StatusInternalServerError)
		return
	}
	renderPage(w, "shop.html", shopPageData{
		User: marshalPageUser(session, true, 0, common.Profile{
			Coin:       state.Coin,
			Background: state.Equipped.Background,
			CardFace:   state.Equipped.CardFace,
		}),
		LoggedIn: true,
		Username: session.Username,
		Coin:     state.Coin,
		Items:    state.Items,
		Equipped: state.Equipped,
	})
}

//GET /api/shop：余额、已拥有、当前穿戴
func ShopState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{"ok": false, "error": "method_not_allowed"})
		return
	}
	session, ok := currentUser(w, r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{"ok": false, "error": "请先登录"})
		return
	}
	state, err := buildShopState(session.UserID)
	if err != nil {
		logs.Error("load shop state for user[%d] failed: %v", session.UserID, err)
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"ok": false, "error": "商店暂时不可用"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "coin": state.Coin, "equipped": state.Equipped, "items": state.Items})
}

//POST /api/shop/buy：花金币买外观
func ShopBuy(w http.ResponseWriter, r *http.Request) {
	session, itemID, ok := shopRequest(w, r)
	if !ok {
		return
	}
	profile, err := common.BuyItem(session.UserID, itemID)
	if err != nil {
		writeShopError(w, err)
		return
	}
	state, err := buildShopState(session.UserID)
	if err != nil {
		logs.Error("load shop state after buy for user[%d] failed: %v", session.UserID, err)
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"ok": false, "error": "购买成功，但刷新余额失败"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true, "coin": profile.Coin, "message": "购买成功",
		"equipped": state.Equipped, "items": state.Items,
	})
}

//POST /api/shop/equip：穿戴已拥有的外观
func ShopEquip(w http.ResponseWriter, r *http.Request) {
	session, itemID, ok := shopRequest(w, r)
	if !ok {
		return
	}
	profile, err := common.EquipItem(session.UserID, itemID)
	if err != nil {
		writeShopError(w, err)
		return
	}
	state, err := buildShopState(session.UserID)
	if err != nil {
		logs.Error("load shop state after equip for user[%d] failed: %v", session.UserID, err)
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"ok": false, "error": "穿戴成功，但刷新状态失败"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true, "coin": profile.Coin, "message": "已穿戴",
		"equipped": state.Equipped, "items": state.Items,
	})
}

//解析并校验商店 POST 请求：必须是同源表单、带 item_id
func shopRequest(w http.ResponseWriter, r *http.Request) (common.Session, string, bool) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{"ok": false, "error": "method_not_allowed"})
		return common.Session{}, "", false
	}
	if !sameOrigin(r) {
		writeJSON(w, http.StatusForbidden, map[string]interface{}{"ok": false, "error": "非法来源"})
		return common.Session{}, "", false
	}
	session, ok := currentUser(w, r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{"ok": false, "error": "请先登录"})
		return common.Session{}, "", false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthRequestSize)
	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "invalid_form"})
		return common.Session{}, "", false
	}
	itemID := strings.TrimSpace(r.Form.Get("item_id"))
	if itemID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "缺少商品"})
		return common.Session{}, "", false
	}
	return session, itemID, true
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true // 非浏览器请求（curl / 测试）没有 Origin
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Host, r.Host)
}

func writeShopError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	message := "商店暂时不可用"
	switch {
	case errors.Is(err, common.ErrNotEnoughCoin):
		status, message = http.StatusBadRequest, "金币不足"
	case errors.Is(err, common.ErrAlreadyOwned):
		status, message = http.StatusConflict, "已经拥有这件外观"
	case errors.Is(err, common.ErrNotOwned):
		status, message = http.StatusForbidden, "请先购买再使用"
	case errors.Is(err, common.ErrItemNotFound):
		status, message = http.StatusNotFound, "商品不存在"
	case errors.Is(err, common.ErrFreeItem):
		status, message = http.StatusBadRequest, "默认外观免费，无需购买"
	}
	if status == http.StatusInternalServerError {
		logs.Error("shop operation failed: %v", err)
	}
	writeJSON(w, status, map[string]interface{}{"ok": false, "error": message})
}
