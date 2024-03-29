package logservice

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/david7482/lambda-extension-log-shipper/extension"
)

//go:generate mockgen -destination=automocks/logapiclient.go -package=automocks . LogAPIClient
type LogAPIClient interface {
	SubscribeLogs(ctx context.Context, types []extension.LogType, params extension.SubscribeLogsParams) (res extension.SubscribeResponse, err error)
}

type LogType string

const (
	PlatformStart       LogType = "platform.start"
	PlatformReport      LogType = "platform.report"
	PlatformFault       LogType = "platform.fault"
	PlatformLogsDropped LogType = "platform.logsDropped"
	Function            LogType = "function"
)

type Log struct {
	Time      time.Time
	Type      LogType `faker:"oneof: platform.start, platform.report, platform.fault, platform.logsDropped, function"`
	RequestID string
	Content   []byte
}

type Message struct {
	Time   time.Time       `json:"time"`
	Type   string          `json:"type"`
	Record json.RawMessage `json:"record"`
}

type StartRecord struct {
	RequestID string `json:"requestId"`
}

type ReportRecord struct {
	RequestID string          `json:"requestId"`
	Metrics   json.RawMessage `json:"metrics"`
}

type ServiceParams struct {
	LogAPIClient         LogAPIClient
	LogTypes             []extension.LogType
	LogsQueue            chan []Log
	ListenPort           int
	MaxItems             int
	MaxBytes             int
	TimeoutMS            int
	EnablePlatformReport bool
}

type LogService struct {
	logAPIClient         LogAPIClient
	logTypes             []extension.LogType
	logsQueue            chan<- []Log
	listenPort           int
	maxItems             int
	maxBytes             int
	timeoutMS            int
	enablePlatformReport bool
}

func New(params ServiceParams) *LogService {
	return &LogService{
		logAPIClient:         params.LogAPIClient,
		logTypes:             params.LogTypes,
		logsQueue:            params.LogsQueue,
		listenPort:           params.ListenPort,
		maxItems:             params.MaxItems,
		maxBytes:             params.MaxBytes,
		timeoutMS:            params.TimeoutMS,
		enablePlatformReport: params.EnablePlatformReport,
	}
}

func (s *LogService) Run(ctx context.Context, wg *sync.WaitGroup) {
	router := http.NewServeMux()
	router.HandleFunc("/", s.logHandler)

	server := &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", s.listenPort),
		Handler: router,
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}

	go func() {
		// Wait for ctx done
		<-ctx.Done()

		// Give 3 second to shutdown server
		ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
		defer cancel()
		server.SetKeepAlivesEnabled(false)
		_ = server.Shutdown(ctx)

		// Close log queue channel to notify forwarder
		close(s.logsQueue)

		// Notify when server is closed
		zerolog.Ctx(ctx).Info().Msg("log service is closed")
		wg.Done()
	}()

	go func() {
		zerolog.Ctx(ctx).Info().Msgf("log service is running on http://%s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zerolog.Ctx(ctx).Fatal().Err(err).Str("Addr", server.Addr).Msg("Fail to start log service")
		}
	}()

	// Subscribe to logs API after log service is running
	// Logs start being delivered only after the subscription happens.
	_, err := s.logAPIClient.SubscribeLogs(ctx, s.logTypes, extension.SubscribeLogsParams{
		ListenPort: s.listenPort,
		MaxItems:   s.maxItems,
		MaxBytes:   s.maxBytes,
		TimeoutMS:  s.timeoutMS,
	})
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).Msg("fail to subscribe log API")
	}
}

func (s *LogService) logHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		zerolog.Ctx(ctx).Error().Err(err).Msg("fail to read all response body from log API")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var messages []Message
	err = json.Unmarshal(body, &messages)
	if err != nil {
		zerolog.Ctx(ctx).Error().Err(err).Msg("fail to parse the logs")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var logs []Log
	var requestID string
	for _, msg := range messages {
		switch LogType(msg.Type) {
		case PlatformStart:
			var startRecord StartRecord
			if err := json.Unmarshal(msg.Record, &startRecord); err != nil {
				zerolog.Ctx(ctx).Error().Err(err).Msg("fail to parse platform.start record")
				continue
			}
			requestID = startRecord.RequestID
		case PlatformReport:
			// Check if we need to send platform report to forwarders
			if !s.enablePlatformReport {
				continue
			}

			var reportRecord ReportRecord
			if err := json.Unmarshal(msg.Record, &reportRecord); err != nil {
				zerolog.Ctx(ctx).Error().Err(err).Msg("fail to parse platform.report record")
				continue
			}
			logs = append(logs, Log{
				Time:      msg.Time,
				Type:      LogType(msg.Type),
				RequestID: reportRecord.RequestID,
				Content:   reportRecord.Metrics,
			})
		case Function, PlatformFault, PlatformLogsDropped:
			logs = append(logs, Log{
				Time:      msg.Time,
				Type:      LogType(msg.Type),
				RequestID: requestID,
				Content:   msg.Record,
			})
		default:
			zerolog.Ctx(ctx).Debug().Str("type", msg.Type).Msg("ignored log with unsupported type")
		}
		zerolog.Ctx(ctx).Debug().Str("requestId", requestID).Msg(msg.Type)
	}

	// write logs into logsQueue in batch
	if len(logs) > 0 {
		s.logsQueue <- logs
	}
}
