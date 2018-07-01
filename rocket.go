// Package rocketrus provides a RocketChat hook for the zap logging package.
package rocketzap

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/RocketChat/Rocket.Chat.Go.SDK/models"
	"github.com/RocketChat/Rocket.Chat.Go.SDK/rest"
	"go.uber.org/zap/zapcore"
)

var (
	NotRunningErr = fmt.Errorf("RocketHook doesn't running, please call Run function first")
)

// Supported log levels
var AllLevels = []zapcore.Level{
	zapcore.DebugLevel,
	zapcore.InfoLevel,
	zapcore.WarnLevel,
	zapcore.ErrorLevel,
	zapcore.PanicLevel,
	zapcore.DPanicLevel,
	zapcore.FatalLevel,
}

// RocketHook is a logrus Hook for dispatching messages to the specified
// channel on RocketChat.
type RocketHook struct {
	HookURL string
	Channel string
	// If UserID and Token are present, will use UserID and Token auth rocket.chat API
	// otherwise Email and the Password are mandatory.
	UserID   string
	Token    string
	Email    string
	Password string

	// Messages with a log level not contained in this array
	// will not be dispatched. If nil, all messages will be dispatched.
	AcceptedLevels []zapcore.Level
	Disabled       bool
	// Title name for log
	Title  string
	Alias  string
	Emoji  string
	Avatar string
	// Notify users with @user in RocketChat.
	NotifyUsers []string
	// batch send message duration, uion/second, default is 10 seconds
	// if duration is negative, RocketHook will block ticker message send
	// e.g. Duration:10, Batch:8 means received more than(include equal) 8 logs in 10 seconds will send as one message to RocketChat immediately, or after 10 seconds received any(>=1) logs will send as one message to RocketChat.
	// e.g. Duration:-1, Batch:8 means only received more than(include equal) 8 logs then send to RocketChat
	Duration int64
	// batch send message, default is 8
	Batch int

	running bool
	msg     *models.PostMessage
	msgChan chan *models.Attachment

	*models.UserCredentials
	*rest.Client
}

// Levels sets which levels to sent to RocketChat
func (rh *RocketHook) Levels() []zapcore.Level {
	if len(rh.AcceptedLevels) == 0 {
		return AllLevels
	}
	return rh.AcceptedLevels
}

func (rh *RocketHook) isAcceptedLevel(level zapcore.Level) bool {
	for _, l := range rh.Levels() {
		if l == level {
			return true
		}
	}
	return false
}

// LevelThreshold - Returns every logging level above and including the given parameter.
func LevelThreshold(l zapcore.Level) []zapcore.Level {
	return AllLevels[l+1:]
}

// Run start RocketHook message processor
func (rh *RocketHook) Run() error {
	index := strings.Index(rh.HookURL, "://")
	serverUrl := &url.URL{
		Scheme: "http",
	}
	if index > 0 {
		serverUrl.Host = rh.HookURL[index+len("://"):]
		if strings.HasPrefix(rh.HookURL, "https") {
			serverUrl.Scheme = "https"
		}
	} else {
		serverUrl.Host = rh.HookURL
	}

	rh.Client = rest.NewClient(serverUrl, false)
	rh.UserCredentials = &models.UserCredentials{
		ID:       rh.UserID,
		Token:    rh.Token,
		Email:    rh.Email,
		Password: rh.Password,
	}
	if err := rh.Client.Login(rh.UserCredentials); err != nil {
		return err
	}
	rh.msgChan = make(chan *models.Attachment, 16)
	if rh.Duration == 0 {
		rh.Duration = 10
	}
	if rh.Batch <= 0 {
		rh.Batch = 8
	}

	var atUsers string
	if len(rh.NotifyUsers) > 0 {
		atUsers = strings.Join(rh.NotifyUsers, " @")
		atUsers = "@" + atUsers
	}

	rh.msg = &models.PostMessage{
		Channel: rh.Channel,
		Alias:   rh.Alias,
		Emoji:   rh.Emoji,
		Avatar:  rh.Avatar,
		Text:    fmt.Sprintf("%s\n*%s logs*", atUsers, rh.Title),
	}

	go rh.send()
	rh.running = true
	return nil
}

func (rh *RocketHook) send() {
	var (
		timer    *time.Timer
		duration time.Duration
	)
	if rh.Duration < 0 {
		timer = &time.Timer{
			C: nil,
		}
	} else {
		duration = time.Duration(rh.Duration) * time.Second
		timer = time.NewTimer(duration)
	}

	for {
		select {
		case msg := <-rh.msgChan:
			rh.msg.Attachments = append(rh.msg.Attachments, *msg)
			if len(rh.msg.Attachments) >= rh.Batch {
				rh.postMessage()
				if rh.Duration > 0 {
					timer.Reset(duration)
				}
			}
		case <-timer.C:
			if len(rh.msg.Attachments) == 0 {
				timer.Reset(duration)
				continue
			}

			rh.postMessage()
			timer.Reset(duration)
		}
	}
}

func (rh *RocketHook) postMessage() {
	rh.Client.PostMessage(rh.msg)
	rh.msg.Attachments = rh.msg.Attachments[:0]
	if cap(rh.msg.Attachments) > 1024 {
		rh.msg.Attachments = make([]models.Attachment, 0, 16)
	}
}

// Fire -  Sent event to RocketChat
func (rh *RocketHook) GetHook() func(zapcore.Entry) error {
	return func(e zapcore.Entry) error {
		if rh.Disabled {
			return nil
		}
		if !rh.running {
			return NotRunningErr
		}
		if !rh.isAcceptedLevel(e.Level) {
			return nil
		}

		color := ""
		switch e.Level {
		case zapcore.DebugLevel:
			color = "purple"
		case zapcore.InfoLevel:
			color = "green"
		case zapcore.ErrorLevel, zapcore.PanicLevel, zapcore.DPanicLevel, zapcore.FatalLevel:
			color = "red"
		default:
			color = "yellow"
		}

		stack := ""
		if len(e.Stack) > 0 {
			stack = "\n\nStack:\n" + e.Stack
		}
		msg := &models.Attachment{
			Color: color,
			Title: e.Level.String() + " log",
			Ts:    e.Time.String(),
			Text:  e.Message + "\n\nCaller:\n" + e.Caller.String() + stack,
		}

		rh.msgChan <- msg
		return nil
	}
}
