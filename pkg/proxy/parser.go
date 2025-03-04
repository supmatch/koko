package proxy

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/supmatch/koko/pkg/i18n"
	"github.com/supmatch/koko/pkg/logger"
	"github.com/supmatch/koko/pkg/model"
	"github.com/supmatch/koko/pkg/utils"
)

var (
	// Todo: Vim过滤依然存在问题
	vimEnterMark = []byte("\x1b[?25l\x1b[37;1H\x1b[1m")
	vimExitMark  = []byte("\x1b[37;1H\x1b[K\x1b")

	zmodemRecvStartMark = []byte("rz waiting to receive.**\x18B0100")
	zmodemSendStartMark = []byte("**\x18B00000000000000")
	zmodemCancelMark    = []byte("\x18\x18\x18\x18\x18")
	zmodemEndMark       = []byte("**\x18B0800000000022d")
	zmodemStateSend     = "send"
	zmodemStateRecv     = "recv"

	charEnter = []byte("\r")
)

func newParser() *Parser {
	parser := &Parser{}
	parser.initial()
	return parser
}

// Parse 解析用户输入输出, 拦截过滤用户输入输出
type Parser struct {
	userOutputChan chan []byte
	srvOutputChan  chan []byte
	cmdRecordChan  chan [2]string

	inputInitial  bool
	inputPreState bool
	inputState    bool
	zmodemState   string
	inVimState    bool
	once          *sync.Once
	lock          *sync.RWMutex

	command         string
	output          string
	cmdInputParser  *CmdParser
	cmdOutputParser *CmdParser

	cmdFilterRules []model.SystemUserFilterRule
	closed         chan struct{}
}

func (p *Parser) initial() {
	p.once = new(sync.Once)
	p.lock = new(sync.RWMutex)

	p.cmdInputParser = NewCmdParser()
	p.cmdOutputParser = NewCmdParser()

	p.closed = make(chan struct{})
	p.cmdRecordChan = make(chan [2]string, 1024)
}

// ParseStream 解析数据流
func (p *Parser) ParseStream(userInChan, srvInChan <-chan []byte) (userOut, srvOut <-chan []byte) {

	p.userOutputChan = make(chan []byte, 1)
	p.srvOutputChan = make(chan []byte, 1)

	go func() {
		defer func() {
			// 会话结束，结算命令结果
			p.sendCommandRecord()
			close(p.cmdRecordChan)
			close(p.userOutputChan)
			close(p.srvOutputChan)
			_ = p.cmdOutputParser.Close()
			_ = p.cmdInputParser.Close()
			logger.Debug("Parser parse stream routine done")
		}()
		for {
			select {
			case <-p.closed:
				return
			case b, ok := <-userInChan:
				if !ok {
					return
				}
				b = p.ParseUserInput(b)
				p.userOutputChan <- b
			case b, ok := <-srvInChan:
				if !ok {
					return
				}
				b = p.ParseServerOutput(b)
				p.srvOutputChan <- b
			}
		}
	}()
	return p.userOutputChan, p.srvOutputChan
}

// Todo: parseMultipleInput 依然存在问题

// parseInputState 切换用户输入状态, 并结算命令和结果
func (p *Parser) parseInputState(b []byte) []byte {
	if p.inVimState || p.zmodemState != "" {
		return b
	}
	p.inputPreState = p.inputState
	if bytes.Contains(b, charEnter) {
		// 连续输入enter key, 结算上一条可能存在的命令结果
		p.sendCommandRecord()

		p.inputState = false
		// 用户输入了Enter，开始结算命令
		p.parseCmdInput()
		if cmd, ok := p.IsCommandForbidden(); !ok {
			fbdMsg := utils.WrapperWarn(fmt.Sprintf(i18n.T("Command `%s` is forbidden"), cmd))
			p.cmdOutputParser.WriteData([]byte(fbdMsg))
			p.srvOutputChan <- []byte("\r\n" + fbdMsg)
			return []byte{utils.CharCleanLine, '\r'}
		}
	} else {
		p.inputState = true
		// 用户又开始输入，并上次不处于输入状态，开始结算上次命令的结果
		if !p.inputPreState {
			p.sendCommandRecord()
		}
	}
	return b
}

// parseCmdInput 解析命令的输入
func (p *Parser) parseCmdInput() {
	p.command = p.cmdInputParser.Parse()
}

// parseCmdOutput 解析命令输出
func (p *Parser) parseCmdOutput() {
	p.output = p.cmdOutputParser.Parse()
}

// ParseUserInput 解析用户的输入
func (p *Parser) ParseUserInput(b []byte) []byte {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.once.Do(func() {
		p.inputInitial = true
	})
	nb := p.parseInputState(b)
	return nb
}

// parseZmodemState 解析数据，查看是不是处于zmodem状态
// 处于zmodem状态不会再解析命令
func (p *Parser) parseZmodemState(b []byte) {
	if len(b) < 20 {
		return
	}
	if p.zmodemState == "" {
		if len(b) > 25 && bytes.Contains(b[:50], zmodemRecvStartMark) {
			p.zmodemState = zmodemStateRecv
			logger.Debug("Zmodem in recv state")
		} else if bytes.Contains(b[:24], zmodemSendStartMark) {
			p.zmodemState = zmodemStateSend
			logger.Debug("Zmodem in send state")
		}
	} else {
		if bytes.Contains(b[:24], zmodemEndMark) {
			logger.Debug("Zmodem end")
			p.zmodemState = ""
		} else if bytes.Contains(b, zmodemCancelMark) {
			logger.Debug("Zmodem cancel")
			p.zmodemState = ""
		}
	}
}

// parseVimState 解析vim的状态，处于vim状态中，里面输入的命令不再记录
func (p *Parser) parseVimState(b []byte) {
	if p.zmodemState == "" && !p.inVimState && bytes.Contains(b, vimEnterMark) {
		p.inVimState = true
		logger.Debug("In vim state: true")
	}
	if p.zmodemState == "" && p.inVimState && bytes.Contains(b, vimExitMark) {
		p.inVimState = false
		logger.Debug("In vim state: false")
	}
}

// splitCmdStream 将服务器输出流分离到命令buffer和命令输出buffer
func (p *Parser) splitCmdStream(b []byte) {
	p.parseVimState(b)
	p.parseZmodemState(b)
	if p.zmodemState != "" || p.inVimState || !p.inputInitial {
		return
	}
	if p.inputState {
		p.cmdInputParser.WriteData(b)
		return
	}
	p.cmdOutputParser.WriteData(b)
}

// ParseServerOutput 解析服务器输出
func (p *Parser) ParseServerOutput(b []byte) []byte {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.splitCmdStream(b)
	return b
}

// SetCMDFilterRules 设置命令过滤规则
func (p *Parser) SetCMDFilterRules(rules []model.SystemUserFilterRule) {
	p.cmdFilterRules = rules
}

// IsCommandForbidden 判断命令是不是在过滤规则中
func (p *Parser) IsCommandForbidden() (string, bool) {
	for _, rule := range p.cmdFilterRules {
		allowed, cmd := rule.Match(p.command)
		switch allowed {
		case model.ActionAllow:
			return "", true
		case model.ActionDeny:
			return cmd, false
		default:

		}
	}
	return "", true
}

func (p *Parser) IsInZmodemRecvState() bool {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.zmodemState != ""
}

// Close 关闭parser
func (p *Parser) Close() {
	select {
	case <-p.closed:
		return
	default:
		close(p.closed)

	}
}

func (p *Parser) sendCommandRecord() {
	if p.command != "" {
		p.parseCmdOutput()
		p.cmdRecordChan <- [2]string{p.command, p.output}
		p.command = ""
		p.output = ""
	}
}
