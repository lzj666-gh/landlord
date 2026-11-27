package common

import (
	"database/sql"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// 起一个只属于当前测试的 sqlite，避免碰真实账号库
func setupAccountDB(t *testing.T, coin int) func() {
	t.Helper()
	dir, err := ioutil.TempDir("", "landlord-account")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	db, err := sql.Open("sqlite3", filepath.Join(dir, "test.db"))
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("open sqlite: %v", err)
	}
	schema := []string{
		`CREATE TABLE account (id INTEGER PRIMARY KEY, email TEXT, username TEXT, password TEXT, coin INTEGER, created_date TEXT, updated_date TEXT)`,
		`CREATE TABLE account_item (account_id INTEGER NOT NULL, item_id TEXT NOT NULL, created_date TEXT, PRIMARY KEY (account_id, item_id))`,
		`CREATE TABLE account_skin (account_id INTEGER NOT NULL, background TEXT, cardface TEXT, updated_date TEXT, PRIMARY KEY (account_id))`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			os.RemoveAll(dir)
			t.Fatalf("create schema: %v", err)
		}
	}
	if _, err := db.Exec("INSERT INTO account(id, username, password, coin) VALUES(1, 'tester', 'x', ?)", coin); err != nil {
		db.Close()
		os.RemoveAll(dir)
		t.Fatalf("insert account: %v", err)
	}

	old := GameConfInfo.Db
	GameConfInfo.Db = db
	return func() {
		GameConfInfo.Db = old
		db.Close()
		os.RemoveAll(dir)
	}
}

func TestBuyItemDeductsCoinAndRecordsOwnership(t *testing.T) {
	cleanup := setupAccountDB(t, 10000)
	defer cleanup()

	profile, err := BuyItem(1, "card_minimal")
	if err != nil {
		t.Fatalf("buy card_minimal: %v", err)
	}
	if profile.Coin != 8000 {
		t.Fatalf("coin after buy = %d, want 8000", profile.Coin)
	}
	owned, err := OwnedItems(1)
	if err != nil {
		t.Fatalf("list owned: %v", err)
	}
	if !owned["card_minimal"] {
		t.Fatal("bought item is not recorded as owned")
	}

	if _, err := BuyItem(1, "card_minimal"); err != ErrAlreadyOwned {
		t.Fatalf("second buy err = %v, want ErrAlreadyOwned", err)
	}
	profile, err = LoadProfile(1)
	if err != nil {
		t.Fatalf("load profile: %v", err)
	}
	if profile.Coin != 8000 {
		t.Fatalf("coin after duplicate buy = %d, want 8000", profile.Coin)
	}

	profile, err = EquipItem(1, "card_minimal")
	if err != nil {
		t.Fatalf("equip bought card: %v", err)
	}
	if profile.CardFace != "card_minimal" {
		t.Fatalf("cardface = %q, want card_minimal", profile.CardFace)
	}
	if profile.Background != DefaultItemID(SkinBackground) {
		t.Fatalf("background changed to %q while equipping a card face", profile.Background)
	}

	if _, err := EquipItem(1, "card_midnight"); err != ErrNotOwned {
		t.Fatalf("equip unowned err = %v, want ErrNotOwned", err)
	}
	profile, err = EquipItem(1, "card_classic")
	if err != nil {
		t.Fatalf("equip default card: %v", err)
	}
	if profile.CardFace != "card_classic" {
		t.Fatalf("cardface = %q, want default", profile.CardFace)
	}
}

func TestBuyItemRejectsWhenCoinIsNotEnough(t *testing.T) {
	cleanup := setupAccountDB(t, 500)
	defer cleanup()

	if _, err := BuyItem(1, "bg_starry"); err != ErrNotEnoughCoin {
		t.Fatalf("buy with 500 coin err = %v, want ErrNotEnoughCoin", err)
	}
	owned, err := OwnedItems(1)
	if err != nil {
		t.Fatalf("list owned: %v", err)
	}
	if owned["bg_starry"] {
		t.Fatal("item was recorded even though payment failed")
	}
	profile, err := LoadProfile(1)
	if err != nil {
		t.Fatalf("load profile: %v", err)
	}
	if profile.Coin != MinCoin {
		t.Fatalf("coin = %d after failed buy, want refill to %d", profile.Coin, MinCoin)
	}
}

func TestBuyFreeItemIsRejected(t *testing.T) {
	cleanup := setupAccountDB(t, 10000)
	defer cleanup()

	if _, err := BuyItem(1, "bg_classic"); err != ErrFreeItem {
		t.Fatalf("buying the free background err = %v, want ErrFreeItem", err)
	}
}

func TestLoadProfileRefillsCoinBelowMinimum(t *testing.T) {
	cleanup := setupAccountDB(t, 20)
	defer cleanup()

	profile, err := LoadProfile(1)
	if err != nil {
		t.Fatalf("load profile: %v", err)
	}
	if profile.Coin != MinCoin {
		t.Fatalf("coin = %d, want %d", profile.Coin, MinCoin)
	}
	var stored int
	if err := GameConfInfo.Db.QueryRow("SELECT coin FROM account WHERE id=1").Scan(&stored); err != nil {
		t.Fatalf("read stored coin: %v", err)
	}
	if stored != MinCoin {
		t.Fatalf("stored coin = %d, want %d", stored, MinCoin)
	}
}

func TestLoadProfileUsesDefaultSkins(t *testing.T) {
	cleanup := setupAccountDB(t, 10000)
	defer cleanup()

	profile, err := LoadProfile(1)
	if err != nil {
		t.Fatalf("load profile: %v", err)
	}
	if profile.Background != DefaultItemID(SkinBackground) || profile.CardFace != DefaultItemID(SkinCardFace) {
		t.Fatalf("default skins = %q/%q", profile.Background, profile.CardFace)
	}
}

func TestBuyItemRefillsAfterSpendingBelowMinimum(t *testing.T) {
	cleanup := setupAccountDB(t, 2500)
	defer cleanup()

	profile, err := BuyItem(1, "card_minimal")
	if err != nil {
		t.Fatalf("buy card_minimal: %v", err)
	}
	if profile.Coin != MinCoin {
		t.Fatalf("coin = %d after spending down to 500, want refill to %d", profile.Coin, MinCoin)
	}
}
