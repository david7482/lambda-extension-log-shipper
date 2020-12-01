package stdout

import (
	"os"

	"github.com/rs/zerolog"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/david7482/lambda-extension-log-shipper/forwardservice"
	"github.com/david7482/lambda-extension-log-shipper/logservice"
)

type stdout struct {
	cfg        config
	logger     zerolog.Logger
	lambdaName string
	awsRegion  string
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
		Envar("LS_STDOUT_ENABLE").
		Default("true").Bool()
}

func (s *stdout) Init(params forwardservice.ForwarderParams) {
	s.lambdaName = params.LambdaName
	s.awsRegion = params.AWSRegion
	s.logger = s.logger.With().Str("lambdaName", s.lambdaName).Str("awsRegion", s.awsRegion).Logger()
}

func (s *stdout) IsEnable() bool {
	return *s.cfg.Enable
}

func (s *stdout) SendLog(logs []logservice.Log) {
	for _, log := range logs {
		s.logger.Log().Time("time", log.Time).Str("lambdaRequestId", log.RequestID).RawJSON("content", log.Content).Send()
	}
}
