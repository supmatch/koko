package httpd

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/LeeEirc/elfinder"
	"github.com/gorilla/mux"

	"github.com/supmatch/koko/pkg/cctx"
	"github.com/supmatch/koko/pkg/common"
	"github.com/supmatch/koko/pkg/config"
	"github.com/supmatch/koko/pkg/logger"
	"github.com/supmatch/koko/pkg/model"
	"github.com/supmatch/koko/pkg/service"
)

func AuthDecorator(handler http.HandlerFunc) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		cookies := strings.Split(request.Header.Get("Cookie"), ";")
		var csrfToken string
		var sessionid string
		var remoteIP string
		for _, line := range cookies {
			if strings.Contains(line, "csrftoken") {
				csrfToken = strings.Split(line, "=")[1]
			}
			if strings.Contains(line, "sessionid") {
				sessionid = strings.Split(line, "=")[1]
			}
		}
		user, err := service.CheckUserCookie(sessionid, csrfToken)
		if err != nil {
			loginUrl := fmt.Sprintf("/users/login/?next=%s", request.URL.Path)
			http.Redirect(responseWriter, request, loginUrl, http.StatusFound)
			return
		}
		xForwardFors := strings.Split(request.Header.Get("X-Forwarded-For"), ",")
		if len(xForwardFors) >= 1 {
			remoteIP = xForwardFors[0]
		} else {
			remoteIP = strings.Split(request.RemoteAddr, ":")[0]
		}
		ctx := context.WithValue(request.Context(), cctx.ContextKeyUser, user)
		ctx = context.WithValue(ctx, cctx.ContextKeyRemoteAddr, remoteIP)
		handler(responseWriter, request.WithContext(ctx))
	}
}

func sftpHostFinder(wr http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	tmpl := template.Must(template.ParseFiles("./templates/elfinder/file_manager.html"))
	hostID := vars["host"]
	_ = tmpl.Execute(wr, hostID)
}

func sftpFinder(wr http.ResponseWriter, req *http.Request) {
	tmpl := template.Must(template.ParseFiles("./templates/elfinder/file_manager.html"))
	_ = tmpl.Execute(wr, "_")
}

func sftpHostConnectorView(wr http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	hostID := vars["host"]
	user := req.Context().Value(cctx.ContextKeyUser).(*model.User)
	remoteIP := req.Context().Value(cctx.ContextKeyRemoteAddr).(string)
	switch req.Method {
	case "GET":
		if err := req.ParseForm(); err != nil {
			http.Error(wr, err.Error(), http.StatusBadRequest)
			return
		}
	case "POST":
		err := req.ParseMultipartForm(32 << 20)
		if err != nil {
			http.Error(wr, err.Error(), http.StatusBadRequest)
			return
		}
	}
	sid := req.Form.Get("sid")
	userV, ok := GetUserVolume(sid)
	if !ok {
		switch strings.TrimSpace(hostID) {
		case "_":
			userV = NewUserVolume(user, remoteIP, "")
		default:
			userV = NewUserVolume(user, remoteIP, hostID)
		}
		addUserVolume(sid, userV)
	}
	logger.Debugf("Elfinder connector sid: %s", sid)
	conf := config.GetConf()
	maxSize := common.ConvertSizeToBytes(conf.ZipMaxSize)
	options := map[string]string{
		"ZipMaxSize": strconv.Itoa(maxSize),
		"ZipTmpPath": conf.ZipTmpPath,
	}
	conn := elfinder.NewElFinderConnectorWithOption([]elfinder.Volume{userV}, options)
	conn.ServeHTTP(wr, req)
}
