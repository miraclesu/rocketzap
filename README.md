rocketzap
========

[RocketChat](https://rocket.chat/) hook for [zap](https://github.com/uber-go/zap). 

## Use

```go
package main

import (
	"log"
	"time"

	"github.com/miraclesu/rocketzap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	hook := &rocketzap.RocketHook{
		HookURL: "localhost:3000",
		Channel: "general",

		NotifyUsers:    []string{"miracle", "gopher"},
		AcceptedLevels: rocketzap.LevelThreshold(zapcore.InfoLevel),

		UserID:   "rocket_user_id",
		Token:    "rocket_token",
		Email:    "suchuangji@gmail.com",
		Password: "password",

		Duration: -1,
		Batch:    1,
	}
	if err := hook.Run(); err != nil {
		log.Fatalln(err.Error())
	}

	logger, err := zap.NewDevelopment(zap.Hooks(hook.GetHook()))
	if err != nil {
		log.Fatalln(err.Error())
	}

	logger.Debug("don't need to send a message")
	logger.Error("an error happened!")
	time.Sleep(1 * time.Second)
}
```

## Parameters

#### Required
  * HookURL
  * Channel
  * UserID & Token or Email & Password

#### Optional
  * AcceptedLevels
  * Disabled
  * Title
  * Alias
  * Emoji
  * Avatar
  * NotifyUsers
  * Duration
  * Batch
## Installation

    go get github.com/miraclesu/rocketzap
