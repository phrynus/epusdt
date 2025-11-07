package log

import (
	"fmt"
	"os"

	"time"

	"github.com/assimon/luuu/config"
	"github.com/natefinch/lumberjack"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Sugar *zap.SugaredLogger

func Init() {
	encoder := getEncoder()

	// 根据 log_debug 配置决定日志输出目标
	var core zapcore.Core
	if config.LogDebug {
		// Debug 模式：同时输出到文件和控制台
		fileWriter := getLogWriter()
		consoleWriter := zapcore.AddSync(os.Stdout)

		// 创建多个输出目标
		core = zapcore.NewTee(
			zapcore.NewCore(encoder, fileWriter, zapcore.DebugLevel),
			zapcore.NewCore(getConsoleEncoder(), consoleWriter, zapcore.DebugLevel),
		)
	} else {
		// 生产模式：只输出到文件
		fileWriter := getLogWriter()
		core = zapcore.NewCore(encoder, fileWriter, zapcore.DebugLevel)
	}

	logger := zap.New(core, zap.AddCaller())
	Sugar = logger.Sugar()
}

func getEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	return zapcore.NewJSONEncoder(encoderConfig)
}

// getConsoleEncoder 获取控制台友好的编码器（使用 Console 格式而非 JSON）
func getConsoleEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder // 使用彩色输出
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	return zapcore.NewConsoleEncoder(encoderConfig)
}

func getLogWriter() zapcore.WriteSyncer {
	file := fmt.Sprintf("%s/log_%s.log",
		config.LogSavePath,
		time.Now().Format("20060102"))
	lumberJackLogger := &lumberjack.Logger{
		Filename:   file,
		MaxSize:    viper.GetInt("log_max_size"),
		MaxBackups: viper.GetInt("max_backups"),
		MaxAge:     viper.GetInt("log_max_age"),
		Compress:   false,
	}
	return zapcore.AddSync(lumberJackLogger)
}
