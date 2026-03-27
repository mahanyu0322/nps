package routers

import (
	"ehang.io/nps/web/controllers"
	"github.com/astaxie/beego"
)

func Init() {
	web_base_url := beego.AppConfig.String("web_base_url")
	if len(web_base_url) > 0 {
		ns := beego.NewNamespace(web_base_url,
			beego.NSRouter("/", &controllers.IndexController{}, "*:Index"),
			beego.NSAutoRouter(&controllers.IndexController{}),
			beego.NSAutoRouter(&controllers.LoginController{}),
			beego.NSAutoRouter(&controllers.ClientController{}),
			beego.NSAutoRouter(&controllers.AuthController{}),
			beego.NSRouter("/blacklist/list", &controllers.BlacklistController{}, "*:List"),
			beego.NSRouter("/blacklist/config", &controllers.BlacklistController{}, "*:Config"),
			beego.NSRouter("/blacklist/add", &controllers.BlacklistController{}, "*:Add"),
			beego.NSRouter("/blacklist/del", &controllers.BlacklistController{}, "*:Del"),
			beego.NSRouter("/blacklist/getlist", &controllers.BlacklistController{}, "*:GetList"),
			beego.NSRouter("/blacklist/addToWhitelist", &controllers.BlacklistController{}, "*:AddToWhitelist"),
			beego.NSRouter("/blacklist/removeFromWhitelist", &controllers.BlacklistController{}, "*:RemoveFromWhitelist"),
		)
		beego.AddNamespace(ns)
	} else {
		beego.Router("/", &controllers.IndexController{}, "*:Index")
		beego.AutoRouter(&controllers.IndexController{})
		beego.AutoRouter(&controllers.LoginController{})
		beego.AutoRouter(&controllers.ClientController{})
		beego.AutoRouter(&controllers.AuthController{})
		beego.Router("/blacklist/list", &controllers.BlacklistController{}, "*:List")
		beego.Router("/blacklist/config", &controllers.BlacklistController{}, "*:Config")
		beego.Router("/blacklist/add", &controllers.BlacklistController{}, "*:Add")
		beego.Router("/blacklist/del", &controllers.BlacklistController{}, "*:Del")
		beego.Router("/blacklist/getlist", &controllers.BlacklistController{}, "*:GetList")
		beego.Router("/blacklist/addToWhitelist", &controllers.BlacklistController{}, "*:AddToWhitelist")
		beego.Router("/blacklist/removeFromWhitelist", &controllers.BlacklistController{}, "*:RemoveFromWhitelist")
	}
}
