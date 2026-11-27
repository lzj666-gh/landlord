package router

import (
	"landlord/controllers"
	"landlord/service"
	"net/http"
)

func init() {
	//页面路由：/ -> /room/list，/room/{rid} 是牌桌页
	http.HandleFunc("/", controllers.Index)
	http.HandleFunc("/room/list", controllers.RoomList)
	http.HandleFunc("/room/", controllers.RoomPage)

	//金币商店：页面 + JSON 接口
	http.HandleFunc("/shop", controllers.ShopPage)
	http.HandleFunc("/api/shop", controllers.ShopState)
	http.HandleFunc("/api/shop/buy", controllers.ShopBuy)
	http.HandleFunc("/api/shop/equip", controllers.ShopEquip)

	http.HandleFunc("/login", controllers.Login)
	http.HandleFunc("/loginOut", controllers.LoginOut)
	http.HandleFunc("/reg", controllers.Register)

	http.HandleFunc("/ws", service.ServeWs)

	// 设置静态目录
	static := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", static))
}
