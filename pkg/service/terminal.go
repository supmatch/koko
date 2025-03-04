package service

import (
	"fmt"

	"github.com/supmatch/koko/pkg/logger"
	"github.com/supmatch/koko/pkg/model"
)

func RegisterTerminal(name, token, comment string) (res model.Terminal) {
	if client.Headers == nil {
		client.Headers = make(map[string]string)
	}
	client.Headers["Authorization"] = fmt.Sprintf("BootstrapToken %s", token)
	data := map[string]string{"name": name, "comment": comment}
	_, err := client.Post(TerminalRegisterURL, data, &res)
	if err != nil {
		logger.Error(err)
	}
	return
}

func TerminalHeartBeat(sIds []string) (res []model.TerminalTask) {

	data := map[string][]string{
		"sessions": sIds,
	}
	_, err := authClient.Post(TerminalHeartBeatURL, data, &res)
	if err != nil {
		logger.Error(err)
	}
	return
}

func CreateSession(data map[string]interface{}) bool {
	var res map[string]interface{}
	_, err := authClient.Post(SessionListURL, data, &res)
	if err == nil {
		return true
	}
	logger.Error(err)
	return false
}

func FinishSession(data map[string]interface{}) {

	var res map[string]interface{}
	if sid, ok := data["id"]; ok {
		payload := map[string]interface{}{
			"is_finished": true,
			"date_end":    data["date_end"],
		}
		Url := fmt.Sprintf(SessionDetailURL, sid)
		_, err := authClient.Patch(Url, payload, &res)
		if err != nil {
			logger.Error(err)
		}
	}

}

func FinishReply(sid string) bool {
	var res map[string]interface{}
	data := map[string]bool{"has_replay": true}
	Url := fmt.Sprintf(SessionDetailURL, sid)
	_, err := authClient.Patch(Url, data, &res)
	if err != nil {
		logger.Error(err)
		return false
	}
	return true
}

func FinishTask(tid string) bool {
	var res map[string]interface{}
	data := map[string]bool{"is_finished": true}
	Url := fmt.Sprintf(FinishTaskURL, tid)
	_, err := authClient.Patch(Url, data, &res)
	if err != nil {
		logger.Error(err)
		return false
	}
	return true
}

func PushSessionReplay(sessionID, gZipFile string) (err error) {
	var res map[string]interface{}
	Url := fmt.Sprintf(SessionReplayURL, sessionID)
	err = authClient.UploadFile(Url, gZipFile, &res)
	if err != nil {
		logger.Error(err)
	}
	return
}

func PushSessionCommand(commands []*model.Command) (err error) {
	_, err = authClient.Post(SessionCommandURL, commands, nil)
	if err != nil {
		logger.Error(err)
	}
	return
}

func PushFTPLog(data *model.FTPLog) (err error) {
	_, err = authClient.Post(FTPLogListURL, data, nil)
	if err != nil {
		logger.Error(err)
	}
	return
}
