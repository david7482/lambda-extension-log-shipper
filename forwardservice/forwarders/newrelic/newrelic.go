package newrelic

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/rs/zerolog"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/david7482/lambda-extension-log-shipper/forwardservice"
	"github.com/david7482/lambda-extension-log-shipper/logservice"
	"github.com/david7482/lambda-extension-log-shipper/utils"
)

type Newrelic struct {
	cfg        config
	logger     zerolog.Logger
	httpClient *http.Client
	params     forwardservice.ForwarderParams
}

type config struct {
	Enable     *bool
	LicenseKey *string
}

type NRCommon struct {
	Attributes map[string]interface{} `json:"attributes"`
}

type NRDetailedLog struct {
	Common NRCommon `json:"common"`
	Logs   []NRLog  `json:"logs"`
}

type NRLog struct {
	Timestamp  int64                  `json:"timestamp"`
	Message    json.RawMessage        `json:"message"`
	Attributes map[string]interface{} `json:"attributes"`
}

func New() *Newrelic {
	return &Newrelic{
		logger:     zerolog.New(os.Stdout).With().Str("forwarder", "newrelic").Timestamp().Logger(),
		httpClient: &http.Client{},
	}
}

func (s *Newrelic) SetupConfigs(app *kingpin.Application) {
	s.cfg.Enable = app.
		Flag("newrelic-enable", "Enable the newrelic forwarder").
		Envar("LS_NEWRELIC_ENABLE").
		Default("true").Bool()
	s.cfg.LicenseKey = app.
		Flag("newrelic-license-key", "The NewRelic licence key to ingest the logs").
		Envar("LS_NEWRELIC_LICENSE_KEY").
		Default("").String()
}

func (s *Newrelic) Init(params forwardservice.ForwarderParams) {
	s.params = params
	s.logger = s.logger.With().Str("lambdaName", s.params.LambdaName).Str("awsRegion", s.params.AWSRegion).Logger()
}

func (s *Newrelic) IsEnable() bool {
	return *s.cfg.Enable
}

func (s *Newrelic) SendLog(logs []logservice.Log) {
	// Build NR logs payload
	var detailedLog NRDetailedLog
	detailedLog.Common.Attributes = map[string]interface{}{
		"service":    s.params.LambdaName,
		"tag":        s.params.LambdaName,
		"plugin":     "lambda-extension-log-shipper",
		"aws.region": s.params.AWSRegion,
	}
	for _, log := range logs {
		nrlog := NRLog{
			Timestamp: log.Time.UnixNano() / 1e6,
			Message:   log.Content,
			Attributes: map[string]interface{}{
				"aws.lambdaRequestId":  log.RequestID,
				"aws.lambdaExtLogType": log.Type,
			},
		}
		if log.Type == logservice.PlatformReport {
			nrlog.Message = []byte(`"aws lambda report"`)
			nrlog.Attributes["aws"] = json.RawMessage(log.Content)
		}

		detailedLog.Logs = append(detailedLog.Logs, nrlog)
	}

	// Compress NR logs payload
	uncompressed, err := json.Marshal([]NRDetailedLog{detailedLog})
	if err != nil {
		s.logger.Error().Err(err).Msg("fail to marshal NR logs")
		return
	}
	s.logger.Debug().RawJSON("rawjson", uncompressed).Send()

	compressed, err := utils.Compress(uncompressed)
	if err != nil {
		s.logger.Error().Err(err).Msg("fail to compress NR logs")
		return
	}

	// Build NR logs request
	httpReq, err := http.NewRequest("POST", "https://log-api.newrelic.com/log/v1", compressed)
	if err != nil {
		s.logger.Error().Err(err).Msg("fail to build NR logs request")
		return
	}
	httpReq.Header.Add("Content-Encoding", "gzip")
	httpReq.Header.Add("Content-Type", "application/json")
	httpReq.Header.Add("User-Agent", "lambda-extension-log-shipper/1")
	httpReq.Header.Add("X-License-Key", *s.cfg.LicenseKey)

	// Make the request
	httpRes, err := s.httpClient.Do(httpReq)
	if err != nil {
		s.logger.Error().Err(err).Msg("fail to send logs to NR")
		return
	}
	defer httpRes.Body.Close()
	body, err := ioutil.ReadAll(httpRes.Body)
	if err != nil {
		s.logger.Error().Err(err).Msg("fail to read NR logs response")
		return
	}
	if httpRes.StatusCode != http.StatusAccepted {
		s.logger.Error().Msgf("NR logs response, status: %s, response: %s", httpRes.Status, string(body))
		return
	}
}
