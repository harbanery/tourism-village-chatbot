package config

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/snowzach/rotatefilehook"
	"github.com/spf13/viper"
)

type CustomTextFormatter struct {
	SessionID string
}

func (f *CustomTextFormatter) Format(entry *logrus.Entry) ([]byte, error) {

	// Format the log message
	message := fmt.Sprintf("%s [%s] %s\n",
		entry.Time.Format("2006-01-02 15:04:05"), // Date-time
		entry.Level.String(),                     // Log level
		entry.Message,                            // Log message
	)

	return []byte(message), nil
}

func NewLogger(viper *viper.Viper) *logrus.Logger {

	log := logrus.New()

	// Create a new instance of the custom formatter
	customFormatter := &CustomTextFormatter{}

	log.SetLevel(logrus.Level(viper.GetInt32("log.level")))
	log.SetFormatter(customFormatter)

	date := time.Now().Format("2006-01-02")

	rotateFileHook, err := rotatefilehook.NewRotateFileHook(rotatefilehook.RotateFileConfig{
		Filename:   "logs/" + date + ".log",
		MaxSize:    50, // megabytes
		MaxBackups: 3,  // amouts
		MaxAge:     1,  //days
		Level:      logrus.AllLevels[log.Level],
		Formatter: &logrus.TextFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
			FullTimestamp:   true,
			ForceColors:     true,
			PadLevelText:    true,
		},
	})

	if err != nil {
		logrus.Errorf("Failed to initialize file rotate hook: %v", err)
	}

	rotateFileErrorHook, err := rotatefilehook.NewRotateFileHook(rotatefilehook.RotateFileConfig{
		Filename: "logs/error.log",
		Level:    logrus.AllLevels[log.Level],
		Formatter: &logrus.TextFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
			FullTimestamp:   true,
			ForceColors:     true,
			PadLevelText:    true,
		},
	})

	if err != nil {
		logrus.Errorf("Failed to initialize file rotate hook: %v", err)
	}

	log.AddHook(rotateFileHook)
	log.AddHook(rotateFileErrorHook)

	return log
}
