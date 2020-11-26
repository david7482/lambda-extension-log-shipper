package stdout

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/david7482/lambda-extension-log-shipper/logservice"
)

type stdout struct {
	cfg    config
	logger zerolog.Logger
}

type config struct {
	Enable *bool
}

func New() *stdout {
	return &stdout{
		logger: zerolog.New(os.Stdout).With().Str("forwarder", "stdout").Timestamp().Logger(),
	}
}

func (s *stdout) SetupConfigs(app *kingpin.Application) {
	s.cfg.Enable = app.
		Flag("stdout-enable", "Enable the stdout forwarder").
		Default("true").Bool()
}

func (s *stdout) IsEnable() bool {
	return *s.cfg.Enable
}

func (s *stdout) SendLog(requestID string, time time.Time, content []byte) {
	s.logger.Info().Time("time", time).Str("requestID", requestID).RawJSON("content", content).Send()
}

func (s *stdout) SendMetrics(requestID string, time time.Time, metrics logservice.Metrics) {
	s.logger.Info().Time("time", time).Str("requestID", requestID).Interface("metrics", metrics).Send()
}
