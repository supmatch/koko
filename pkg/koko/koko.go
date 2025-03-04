package koko

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/supmatch/koko/pkg/config"
	"github.com/supmatch/koko/pkg/httpd"
	"github.com/supmatch/koko/pkg/i18n"
	"github.com/supmatch/koko/pkg/logger"
	"github.com/supmatch/koko/pkg/service"
	"github.com/supmatch/koko/pkg/sshd"
)

const Version = "1.5.3"

type Coco struct {
}

func (c *Coco) Start() {
	fmt.Println(time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("Koko Version %s, more see https://www.jumpserver.org\n", Version)
	fmt.Println("Quit the server with CONTROL-C.")
	go sshd.StartServer()
	go httpd.StartHTTPServer()
}

func (c *Coco) Stop() {
	sshd.StopServer()
	httpd.StopHTTPServer()
	logger.Info("Quit The Coco")
}

func RunForever() {
	ctx,cancelFunc := context.WithCancel(context.Background())
	bootstrap(ctx)
	gracefulStop := make(chan os.Signal)
	signal.Notify(gracefulStop, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	app := &Coco{}
	app.Start()
	<-gracefulStop
	cancelFunc()
	app.Stop()
}

func bootstrap(ctx context.Context) {
	config.Initial()
	i18n.Initial()
	logger.Initial()
	service.Initial(ctx)
	Initial()
}
