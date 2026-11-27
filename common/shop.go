package common

// 金币经济与商店商品目录。
// 单局结算公式：底注(房间 EntranceFee) × 叫分(1-3) × 倍数(炸弹/王炸翻倍)。
const (
	StartCoin = 10000 // 注册初始金币
	MinCoin   = 1000  // 低于这个数就补到 1000
)

const (
	SkinBackground = "background"
	SkinCardFace   = "cardface"
)

// ShopItem 是一件可购买/可穿戴的外观。
type ShopItem struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Desc     string `json:"desc"`
	Price    int    `json:"price"`
	Asset    string `json:"asset"`
	Default  bool   `json:"default"`
	Owned    bool   `json:"owned"`
	Equipped bool   `json:"equipped"`
}

var shopItems = []ShopItem{
	// 背景
	{ID: "bg_classic", Kind: SkinBackground, Name: "经典绿绒", Desc: "默认牌桌，深绿绒布与木质桌沿", Price: 0, Asset: "/static/i/bg/classic.png", Default: true},
	{ID: "bg_starry", Kind: SkinBackground, Name: "星夜", Desc: "深蓝星空下的绒面牌桌", Price: 3000, Asset: "/static/i/bg/starry.png"},
	{ID: "bg_neon", Kind: SkinBackground, Name: "赛博霓虹", Desc: "青紫霓虹映在黑色桌面上", Price: 6000, Asset: "/static/i/bg/neon.png"},
	{ID: "bg_sakura", Kind: SkinBackground, Name: "夜樱", Desc: "夜色、暖灯与落在桌上的樱花瓣", Price: 9000, Asset: "/static/i/bg/sakura.png"},

	// 牌面
	{ID: "card_classic", Kind: SkinCardFace, Name: "经典牌面", Desc: "默认牌面，红黑配色", Price: 0, Asset: "/static/i/poker.png", Default: true},
	{ID: "card_minimal", Kind: SkinCardFace, Name: "简约白", Desc: "纯白卡面，细边框，干净利落", Price: 2000, Asset: "/static/i/cards/minimal.png"},
	{ID: "card_midnight", Kind: SkinCardFace, Name: "暗夜鎏金", Desc: "深蓝卡面配金色花色", Price: 4500, Asset: "/static/i/cards/midnight.png"},
	{ID: "card_celadon", Kind: SkinCardFace, Name: "青瓷", Desc: "米白底、青绿边、砖红花色", Price: 7000, Asset: "/static/i/cards/celadon.png"},
}

// ShopItems 返回商品目录的副本，避免调用方改到全局数据。
func ShopItems() []ShopItem {
	items := make([]ShopItem, len(shopItems))
	copy(items, shopItems)
	return items
}

// FindShopItem 按 ID 查找商品。
func FindShopItem(id string) (ShopItem, bool) {
	for _, item := range shopItems {
		if item.ID == id {
			return item, true
		}
	}
	return ShopItem{}, false
}

// DefaultItemID 返回某类外观的默认商品 ID。
func DefaultItemID(kind string) string {
	for _, item := range shopItems {
		if item.Kind == kind && item.Default {
			return item.ID
		}
	}
	return ""
}

// ItemAsset 返回某个外观的静态资源路径；ID 不合法时退回默认外观。
func ItemAsset(kind, id string) string {
	if item, ok := FindShopItem(id); ok && item.Kind == kind {
		return item.Asset
	}
	if item, ok := FindShopItem(DefaultItemID(kind)); ok {
		return item.Asset
	}
	return ""
}
