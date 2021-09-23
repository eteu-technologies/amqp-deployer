package main

import (
	"io"
	"log"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type zapWriter struct {
	msg string
	log *log.Logger
}

func (zw *zapWriter) Write(p []byte) (n int, err error) {
	n = len(p)
	zw.log.Print(string(p))
	return
}

func NewZapWriter(level zapcore.Level, pipe string, fields ...zap.Field) io.Writer {
	logger := zap.L().With(zap.String("section", "action"), zap.String("out", pipe)).With(fields...)
	log, err := zap.NewStdLogAt(logger, level)
	if err != nil {
		// XXX: cannot fail, but fall back somehow.
		log = zap.NewStdLog(logger)
	}

	return &zapWriter{
		msg: pipe,
		log: log,
	}
}
