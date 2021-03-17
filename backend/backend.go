package backend

import (
	"database/sql"
	"html/template"
	"io"
	"math/rand"
	"strconv"
	"time"

	"../config"
	"../database"
	"../gitsearch"

	"golang.org/x/crypto/acme/autocert"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo"
	"github.com/labstack/echo-contrib/session"
)

type Request struct {
	Id       int    `json:"id"`
	Data     string `json:"data"`
	SourceIp string `json:"source_ip"`
	Time     string `json:"time"`
}

type SingleRequest struct {
	R     *Request
	Table string
}
type Result struct {
	Error string `json:"error"`
}

type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

type ErrorContext struct {
	Error string
}

//start backend
func StartBack(db *sql.DB) {
	e := echo.New()
	//pass db pointer to echo handler
	t := &Template{
		templates: template.Must(template.ParseGlob("frontend/templates/*")),
	}

	secret := []byte(RandStringBytes(20))
	e.AutoTLSManager.Cache = autocert.DirCache("/var/www/.cache")
	e.Use(session.Middleware(sessions.NewCookieStore(secret)))
	e.Renderer = t
	e.Use(func(h echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			cc := &database.DBContext{Context: c, Db: db}
			return h(cc)
		}
	})

	credentials.username = config.Settings.AdminCredentials.Username
	credentials.password = config.Settings.AdminCredentials.Password

	//e.Pre(middleware.HTTPSRedirect())
	e.File("/", "frontend/index.html", loginRequired)
	e.File("/settings", "frontend/settings.html", loginRequired)
	e.Static("/static", "frontend/static/")

	e.GET("/api/get/:datatype/:status", getReports, loginRequired)
	e.GET("/api/mark/:datatype/:result_id/:status", markResult, loginRequired)

	e.GET("/login", loginPage)
	e.POST("/login", handleLogin)

	e.HideBanner = true
	e.Debug = true
	//e.Logger.Fatal(e.StartAutoTLS(":1234"))
	e.Logger.Fatal(e.Start(":1234"))
}

//handler for getting requests from database

func markResult(c echo.Context) (err error) {
	return c.String(404, "Not Found")
}

func getReports(c echo.Context) (err error) {
	var result gitsearch.WebUIResult
	var limit, offset int

	limitParam := c.FormValue("limit")
	if limitParam == "" {
		limit = 0
	} else {
		limit, err = strconv.Atoi(limitParam)
	}

	if err != nil {
		return err
	}
	offsetParam := c.FormValue("offset")
	if offsetParam == "" {
		offset = 0
	} else {
		offset, err = strconv.Atoi(offsetParam)
	}

	switch c.Param("datatype") {
	case "github":
		result, err = gitsearch.GetGitReports(c.Param("status"), limit, offset)
		return c.JSON(200, result)

	default:
		return c.String(404, "Not Found")
	}

	return c.String(404, "Not Found")
}

const letterBytes = "abcdefghijklmnopqrstuvwxyz"

func RandStringBytes(n int) string {
	rand.Seed(time.Now().Unix())
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
