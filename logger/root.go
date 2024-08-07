// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

// Package logger wrapped zap as a basic logging implement.
package logger

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	root          *zap.Logger
	defaultOption = &Option{
		Level: DEBUG,
		Flags: OPT_DEFAULT,
	}

	SchemeTCP = "tcp"
	SchemeUDP = "udp"
)

const (
	NameKeyMod   = "mod"
	NameKeyMsg   = "msg"
	NameKeyLevel = "lev"
	NameKeyTime  = "ts"
	NameKeyPos   = "pos"
)

func doSetGlobalRootLogger(fpath, level string, options int) error {
	if fpath == "" {
		return fmt.Errorf("fpath should not empty")
	}

	mtx.Lock()
	defer mtx.Unlock()

	if root != nil {
		return nil
	}

	var err error
	root, err = newRootLogger(fpath, level, options)
	if err != nil {
		return err
	}

	return nil
}

// SetGlobalRootLogger deprecated, use InitRoot() instead.
func SetGlobalRootLogger(fpath, level string, options int) error {
	return doSetGlobalRootLogger(fpath, level, options)
}

// InitRoot used to setup global root logger, include
//   - log level
//   - log path
//   - set to disk file(with or without rotate)
//   - set to some remtoe TCP/UDP server
//   - a bounch of other OPT_XXXs
func InitRoot(opt *Option) error {
	if opt == nil {
		opt = defaultOption
	}

	switch opt.Level {
	case DEBUG, INFO, WARN, ERROR, PANIC, FATAL, DPANIC:
	case "": // 默认使用 DEBUG
		opt.Level = DEBUG

	default:
		return fmt.Errorf("invalid log level `%s'", opt.Level)
	}

	if opt.Flags == 0 {
		opt.Flags = OPT_DEFAULT
	}

	if opt.Path != "" && (opt.Flags&OPT_STDOUT != 0) {
		return fmt.Errorf("set stdout logging with log path '%s', flag:%b", opt.Path, opt.Flags)
	}

	switch opt.Path {
	case "":
		if v, ok := os.LookupEnv("LOGGER_PATH"); ok {
			opt.Path = v
			return setRootLoggerFromEnv(opt)
		}

		return doSetStdoutLogger(opt)

	default:
		return doSetGlobalRootLogger(opt.Path, opt.Level, opt.Flags)
	}
}

func newRootLogger(fpath, level string, options int) (*zap.Logger, error) {
	if fpath == "" {
		return newNormalRootLogger(fpath, level, options)
	}

	u, err := url.Parse(fpath)
	if err != nil {
		return nil, err
	}

	switch strings.ToLower(u.Scheme) {
	case SchemeTCP, SchemeUDP: // logs sending to some remote TCP/UDP server
		return newCustomizeRootLogger(level,
			options,
			&remoteEndpoint{protocol: u.Scheme, host: u.Host})

	default: // they must be some disk path file
		if _, err := os.Stat(fpath); err != nil { // create file if not exists
			if err := os.MkdirAll(filepath.Dir(fpath), 0o600); err != nil {
				return nil, fmt.Errorf("MkdirAll(%s): %w", fpath, err)
			}

			// create empty log file
			if err := os.WriteFile(fpath, nil, 0o600); err != nil {
				return nil, fmt.Errorf("WriteFile(%s): %w", fpath, err)
			}
		}
	}

	// auto-rotate disk logging file
	if options&OPT_ROTATE != 0 &&
		options&OPT_STDOUT == 0 && // can't rotate stdout
		fpath != os.DevNull { // can't rotate(rename) /dev/null
		return newCustomizeRootLogger(level, options, &lumberjack.Logger{
			Filename:   fpath,
			MaxSize:    MaxSize,
			MaxBackups: MaxBackups,
			MaxAge:     MaxAge,
		})
	}

	return newNormalRootLogger(fpath, level, options)
}

// InitCustomizeRoot used to setup global root logger, include
//   - log path
//   - log maxsize
//   - log compress

func InitCustomizeRoot(opt *Option) (*zap.Logger, error) {
	mtx.Lock()
	defer mtx.Unlock()

	lumberLog := &lumberjack.Logger{
		Filename: opt.Path,
		MaxSize:  opt.MaxSize,
		Compress: opt.Compress,
	}

	c := cron.New(cron.WithSeconds())
	if _, err := c.AddFunc("50 59 * * * *",
		func() {
			if err := lumberLog.Rotate(); err != nil {
				log.Printf("lumberLog.Rotate: %s, ignored", err.Error())
			}
		}); err != nil {
		return nil, err
	}
	c.Start()

	return newOnlyMessageRootLogger(lumberLog)
}
