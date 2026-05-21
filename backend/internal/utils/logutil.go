package utils

import (
	"log"

	"github.com/gookit/color"
)

var (
	Success  = color.New(color.FgGreen, color.OpBold)   // ✓ 里程碑
	Info     = color.New(color.FgCyan)                  // 一般信息
	Warn     = color.New(color.FgYellow)                // 警告
	Error    = color.New(color.FgRed, color.OpBold)     // 致命
	Security = color.New(color.FgMagenta, color.OpBold) // 安全
	Subtle   = color.New(color.FgGray)                  // debug / infra 噪音
)

func LogSuccess(format string, args ...interface{}) {
	log.Print(Success.Sprintf("✓ "+format, args...))
}

func LogInfo(format string, args ...interface{}) {
	log.Print(Info.Sprintf(format, args...))
}

func LogWarn(format string, args ...interface{}) {
	log.Print(Warn.Sprintf("[WARN] "+format, args...))
}

func LogInfra(format string, args ...interface{}) {
	log.Print(Subtle.Sprintf("[infra] "+format, args...))
}

func LogFatal(format string, args ...interface{}) {
	log.Fatal(Error.Sprintf("FATAL: "+format, args...))
}
