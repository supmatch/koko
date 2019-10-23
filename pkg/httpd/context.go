package httpd

import "github.com/supmatch/koko/pkg/model"

type WebContext struct {
	User       *model.User
	Connection *Client
	Client     *Client
}
