package summer

import (
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/night-codes/conv"
	mongo "github.com/night-codes/mgo-wrapper"
	"gopkg.in/mgo.v2"
)

type (
	// ajaxFunc is alias for map[string]func(c *gin.Context)
	ajaxFunc map[string]func(c *gin.Context)
	// websocketFunc is alias for map[string]func(c *websocket.Conn)
	websocketFunc map[string]func(c *gin.Context, ws *websocket.Conn)

	//Module struct
	Module struct {
		*Panel
		Collection *mgo.Collection
		Settings   *ModuleSettings
	}

	//ModuleSettings struct
	ModuleSettings struct {
		Name             string // string identifier of module (must be unique)
		Title            string // visible module name
		Menu             *Menu  // parent menu (panel.MainMenu, panel.DropMenu etc.)
		MenuOrder        int
		MenuTitle        string
		PageRouteName    string // used to build page path: /{Path}/{module.PageRouteName}
		AjaxRouteName    string // used to build ajax path: /{Path}/ajax/{module.PageRouteName}/*method
		SocketsRouteName string // used to build websocket path: /{Path}/websocket/{module.PageRouteName}/*method
		CollectionName   string // MongoDB collection name
		TemplateName     string // template in views folder
		ajax             ajaxFunc
		websockets       websocketFunc
		Icon             string // module icon in title
		GroupTo          Simple // add module like tab to another module
		GroupTitle       string // tab title
		Rights           Rights // access rights required to access this page
		DisableAuth      bool   // the page can be viewed for unauthorised visitors
		OriginTemplate   bool   // do not use Footer and Header wraps in template render
		RouterGroup      *gin.RouterGroup
		AjaxRouterGroup  *gin.RouterGroup
		WsRouterGroup    *gin.RouterGroup
	}

	// Simple module interface
	Simple interface {
		init(settings *ModuleSettings, panel *Panel)
		Page(c *gin.Context)
		Ajax(c *gin.Context)
		Websockets(c *gin.Context)
		GetSettings() *ModuleSettings
	}
)

var (
	wsupgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

// Ajax  is default module's ajax method
func (m *Module) Ajax(c *gin.Context) {
	if allowIfs, ex := c.Get("Allow"); ex {
		if allow, ok := allowIfs.(bool); ok && allow {
			method := stripSlashes(strings.ToLower(c.Param("method")))
			for ajaxRoute, ajaxFunc := range m.Settings.ajax {
				if method == ajaxRoute {
					ajaxFunc(c)
					return
				}
			}
			c.String(404, `Method not found in module "`+m.Settings.Name+`"!`)
			return
		}
	}
	c.String(403, `Accesss denied`)
}

// Websockets  is default module's websockets method
func (m *Module) Websockets(c *gin.Context) {
	if allowIfs, ex := c.Get("Allow"); ex {
		if allow, ok := allowIfs.(bool); ok && allow {
			method := stripSlashes(strings.ToLower(c.Param("method")))
			for websocketsRoute, websocketsFunc := range m.Settings.websockets {
				if method == websocketsRoute {
					if conn, err := wsupgrader.Upgrade(c.Writer, c.Request, nil); err == nil {
						websocketsFunc(c, conn)
						return
					}
					break
				}
			}
			c.String(404, `Method not found in module "`+m.Settings.Name+`"!`)
			return
		}
	}
	c.String(403, `Accesss denied`)
}

// Page is default module's page rendering method
func (m *Module) Page(c *gin.Context) {
	c.HTML(200, m.Settings.TemplateName+".html", obj{})
}

// default module's initial method
func (m *Module) init(settings *ModuleSettings, panel *Panel) {
	m.Settings = settings
	m.Panel = panel
	if m.Collection == nil {
		m.Collection = mongo.DB(panel.DBName).C(settings.CollectionName)
	}
}

// GetSettings needs for correct settings getting from module struct
func (m *Module) GetSettings() *ModuleSettings {
	return m.Settings
}

// Create new module
func createModule(panel *Panel, settings *ModuleSettings, s Simple) Simple {

	if _, ex := panel.Modules.Get(settings.Name); ex {
		panic(`Repeated use of module name "` + settings.Name + `"`)
	}

	if settings.ajax == nil {
		settings.ajax = ajaxFunc{}
	}
	if settings.websockets == nil {
		settings.websockets = websocketFunc{}
	}
	st := reflect.ValueOf(s)
	for i := 0; i < st.NumMethod(); i++ {
		method := st.Method(i).Type().String()
		if len(method) > 17 && method[:17] == "func(*gin.Context" {
			name := strings.ToLower(st.Type().Method(i).Name)
			switch name {
			case "ajax", "page", "websockets":
				continue
			}
			if method == "func(*gin.Context)" {
				method := st.Method(i).Interface().(func(*gin.Context))
				settings.ajax[name] = method
			} else if method == "func(*gin.Context, *websocket.Conn)" {
				method := st.Method(i).Interface().(func(*gin.Context, *websocket.Conn))
				settings.websockets[name] = method
			}
		}
	}
	// default settings for some fields
	if len(settings.PageRouteName) == 0 {
		settings.PageRouteName = settings.Name
	}
	if len(settings.AjaxRouteName) == 0 {
		settings.AjaxRouteName = settings.PageRouteName
	}
	if len(settings.SocketsRouteName) == 0 {
		settings.SocketsRouteName = settings.PageRouteName
	}
	if len(settings.Title) == 0 {
		settings.Title = strings.Replace(settings.Name, "/", " ", -1)
	}
	if len(settings.MenuTitle) == 0 {
		settings.MenuTitle = settings.Title
	}
	if len(settings.GroupTitle) == 0 {
		settings.GroupTitle = settings.MenuTitle
	}
	if len(settings.CollectionName) == 0 {
		settings.CollectionName = strings.Replace(settings.Name, "/", "-", -1)
	}
	if len(settings.TemplateName) == 0 {
		settings.TemplateName = strings.Replace(settings.Name, "/", "-", -1)
	}
	if settings.OriginTemplate {
		settings.TemplateName = "summer-origin-" + settings.TemplateName
	}

	if !settings.DisableAuth {
		settings.Rights.Actions = uniqAppend(settings.Rights.Actions, []string{settings.Name})
	}
	panel.Groups.Add("root", settings.Name)

	// middleware for rights check
	preAllow := func(c *gin.Context) {
		if panel.DisableAuth {
			c.Set("Allow", true)
			c.Header("Allow", "true")
			return
		}
		userIfs, _ := c.Get("user")
		if user, ok := userIfs.(UsersStruct); ok {
			allow := checkRights(panel, settings.Rights, user.Rights)
			c.Set("Allow", allow)
			c.Header("Allow", conv.String(allow))
			return
		}
		c.Set("Allow", false)
		c.Header("Allow", "false")
	}

	// PAGE route
	settings.RouterGroup = panel.RouterGroup.Group(settings.PageRouteName)
	panel.auth.Auth(settings.RouterGroup, settings.DisableAuth)
	settings.RouterGroup.Use(func(c *gin.Context) {
		preAllow(c)
		login, _ := c.Get("login")
		c.Header("Module", settings.PageRouteName)
		c.Header("Login", conv.String(login))
		c.Header("Lang", getLang(c.GetHeader("Accept-Language")))
		c.Header("Title", settings.Title)
		c.Header("Path", panel.Path)
		c.Header("Ajax", settings.AjaxRouteName)
		c.Header("Socket", settings.SocketsRouteName)
		c.Header("Action", stripSlashes(c.Param("action")))
		header := c.Writer.Header()
		header["Css"] = panel.CSS
		header["Js"] = panel.JS
	})
	settings.RouterGroup.GET("/*action", s.Page)

	// AJAX routes
	settings.AjaxRouterGroup = panel.RouterGroup.Group("/ajax/" + settings.AjaxRouteName)
	panel.auth.Auth(settings.AjaxRouterGroup, settings.DisableAuth)
	settings.AjaxRouterGroup.Use(preAllow)
	settings.AjaxRouterGroup.POST("/*method", s.Ajax)

	// SOCKET routes
	settings.WsRouterGroup = panel.RouterGroup.Group("/websocket/" + settings.SocketsRouteName)
	panel.auth.Auth(settings.WsRouterGroup, settings.DisableAuth)
	settings.WsRouterGroup.Use(preAllow)
	settings.WsRouterGroup.GET("/*method", s.Websockets)

	s.init(settings, panel)
	panel.Modules.add(s)
	return s
}
