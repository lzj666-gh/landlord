package common

import (
	"database/sql"
	"errors"
	"time"

	"github.com/astaxie/beego/logs"
)

var (
	ErrItemNotFound  = errors.New("item not found")
	ErrAlreadyOwned  = errors.New("item already owned")
	ErrNotEnoughCoin = errors.New("not enough coin")
	ErrNotOwned      = errors.New("item not owned")
	ErrFreeItem      = errors.New("item is free")
	ErrNoAccount     = errors.New("account not found")
)

// Profile 是牌桌/商店需要的账号状态。
type Profile struct {
	Coin       int
	Background string
	CardFace   string
}

// LoadProfile 读取账号金币和已穿戴外观；金币低于 MinCoin 时补到 MinCoin。
// 数据库不可用（单元测试）时给一份可用的默认值。
func LoadProfile(userID int) (Profile, error) {
	profile := Profile{
		Coin:       StartCoin,
		Background: DefaultItemID(SkinBackground),
		CardFace:   DefaultItemID(SkinCardFace),
	}
	db := GameConfInfo.Db
	if db == nil || userID <= 0 {
		return profile, nil
	}

	err := db.QueryRow("SELECT COALESCE(coin, 0) FROM account WHERE id=?", userID).Scan(&profile.Coin)
	if err == sql.ErrNoRows {
		return profile, ErrNoAccount
	}
	if err != nil {
		return profile, err
	}

	var background, cardFace string
	err = db.QueryRow("SELECT COALESCE(background, ''), COALESCE(cardface, '') FROM account_skin WHERE account_id=?", userID).
		Scan(&background, &cardFace)
	if err != nil && err != sql.ErrNoRows {
		return profile, err
	}
	if item, ok := FindShopItem(background); ok && item.Kind == SkinBackground {
		profile.Background = background
	}
	if item, ok := FindShopItem(cardFace); ok && item.Kind == SkinCardFace {
		profile.CardFace = cardFace
	}

	if profile.Coin < MinCoin {
		logs.Info("account[%d] coin %d below %d, refill to %d", userID, profile.Coin, MinCoin, MinCoin)
		profile.Coin = MinCoin
		if err := SaveCoin(userID, profile.Coin); err != nil {
			logs.Error("refill coin for account[%d] failed: %v", userID, err)
		}
	}
	return profile, nil
}

// SaveCoin 把金币写回账号。
func SaveCoin(userID, coin int) error {
	db := GameConfInfo.Db
	if db == nil || userID <= 0 {
		return nil
	}
	_, err := db.Exec("UPDATE account SET coin=?, updated_date=? WHERE id=?",
		coin, time.Now().Format("2006-01-02 15:04:05"), userID)
	return err
}

// OwnedItems 返回账号已购买的商品 ID。
func OwnedItems(userID int) (map[string]bool, error) {
	owned := make(map[string]bool)
	db := GameConfInfo.Db
	if db == nil || userID <= 0 {
		return owned, nil
	}
	rows, err := db.Query("SELECT item_id FROM account_item WHERE account_id=?", userID)
	if err != nil {
		return owned, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return owned, err
		}
		owned[id] = true
	}
	return owned, rows.Err()
}

// BuyItem 扣金币并把商品记到账号名下，返回购买后的账号状态。
func BuyItem(userID int, itemID string) (Profile, error) {
	empty := Profile{}
	db := GameConfInfo.Db
	if db == nil || userID <= 0 {
		return empty, ErrNoAccount
	}
	item, ok := FindShopItem(itemID)
	if !ok {
		return empty, ErrItemNotFound
	}
	if item.Price <= 0 {
		return empty, ErrFreeItem
	}

	tx, err := db.Begin()
	if err != nil {
		return empty, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	var coin int
	if err = tx.QueryRow("SELECT COALESCE(coin, 0) FROM account WHERE id=?", userID).Scan(&coin); err != nil {
		return empty, ErrNoAccount
	}
	if coin < item.Price {
		return empty, ErrNotEnoughCoin
	}

	result, err := tx.Exec("INSERT OR IGNORE INTO account_item(account_id, item_id, created_date) VALUES(?,?,?)",
		userID, item.ID, time.Now().Format("2006-01-02 15:04:05"))
	if err != nil {
		return empty, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return empty, err
	}
	if affected == 0 {
		return empty, ErrAlreadyOwned
	}

	if _, err = tx.Exec("UPDATE account SET coin=?, updated_date=? WHERE id=?",
		coin-item.Price, time.Now().Format("2006-01-02 15:04:05"), userID); err != nil {
		return empty, err
	}
	if err = tx.Commit(); err != nil {
		return empty, err
	}
	committed = true
	logs.Info("account[%d] bought item[%s] for %d coin", userID, item.ID, item.Price)
	return LoadProfile(userID)
}

// EquipItem 穿上已购买（或默认免费）的外观。
func EquipItem(userID int, itemID string) (Profile, error) {
	db := GameConfInfo.Db
	if db == nil || userID <= 0 {
		return Profile{}, ErrNoAccount
	}
	item, ok := FindShopItem(itemID)
	if !ok {
		return Profile{}, ErrItemNotFound
	}
	if !item.Default {
		owned, err := OwnedItems(userID)
		if err != nil {
			return Profile{}, err
		}
		if !owned[item.ID] {
			return Profile{}, ErrNotOwned
		}
	}

	column := "background"
	if item.Kind == SkinCardFace {
		column = "cardface"
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	if _, err := db.Exec("INSERT OR IGNORE INTO account_skin(account_id, background, cardface, updated_date) VALUES(?,?,?,?)",
		userID, DefaultItemID(SkinBackground), DefaultItemID(SkinCardFace), now); err != nil {
		return Profile{}, err
	}
	if _, err := db.Exec("UPDATE account_skin SET "+column+"=?, updated_date=? WHERE account_id=?",
		item.ID, now, userID); err != nil {
		return Profile{}, err
	}
	return LoadProfile(userID)
}
