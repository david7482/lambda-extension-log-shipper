package logservice

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/david7482/lambda-extension-log-shipper/extension"
)

type logAPIClient interface {
	SubscribeLogs(ctx context.Context, types []extension.LogType, params extension.SubscribeLogsParams) (res extension.SubscribeResponse, err error)
}

type Log struct {
	Time      time.Time
	Type      string
	RequestID string
	Metrics   Metrics
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
	RequestID string  `json:"requestId"`
	Metrics   Metrics `json:"metrics"`
}

type Metrics struct {
	DurationMs      float64 `json:"durationMs"`
	MaxMemoryUsedMB int     `json:"maxMemoryUsedMB"`
	InitDurationMs  float64 `json:"initDurationMs,omitempty"`
}

type LogServiceParams struct {
	LogAPIClient logAPIClient
	LogTypes     []extension.LogType
	LogQueue     chan<- Log
	ListenPort   int
	MaxItems     int
	MaxBytes     int
	TimeoutMS    int
}

type LogService struct {
	logAPIClient logAPIClient
	logTypes     []extension.LogType
	logQueue     chan<- Log
	listenPort   int
	maxItems     int
	maxBytes     int
	timeoutMS    int
}

func New(params LogServiceParams) *LogService {
	return &LogService{
		logAPIClient: params.LogAPIClient,
		logTypes:     params.LogTypes,
		logQueue:     params.LogQueue,
		listenPort:   params.ListenPort,
		maxItems:     params.MaxItems,
		maxBytes:     params.MaxBytes,
		timeoutMS:    params.TimeoutMS,
	}
}

func (s *LogService) Run(ctx context.Context, wg *sync.WaitGroup) {
	router := http.NewServeMux()
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			zerolog.Ctx(ctx).Error().Err(err).Msg("fail to read all response body from log API")
			return
		}

		var messages []Message
		err = json.Unmarshal(body, &messages)
		if err != nil {
			zerolog.Ctx(ctx).Error().Err(err).Msg("fail to parse the logs")
			return
		}

		var requestID string
		for _, msg := range messages {
			switch msg.Type {
			case "platform.start":
				var startRecord StartRecord
				if err := json.Unmarshal(msg.Record, &startRecord); err != nil {
					zerolog.Ctx(ctx).Error().Err(err).Msg("fail to parse platform.start record")
					continue
				}
				requestID = startRecord.RequestID

			case "platform.report":
				var reportRecord ReportRecord
				if err := json.Unmarshal(msg.Record, &reportRecord); err != nil {
					zerolog.Ctx(ctx).Error().Err(err).Msg("fail to parse platform.report record")
					continue
				}

				s.logQueue <- Log{
					Time:      msg.Time,
					Type:      msg.Type,
					RequestID: reportRecord.RequestID,
					Metrics:   reportRecord.Metrics,
				}
			case "function", "platform.fault", "platform.logsDropped":
				s.logQueue <- Log{
					Time:      msg.Time,
					Type:      msg.Type,
					RequestID: requestID,
					Content:   msg.Record,
				}
			default:
				//zerolog.Ctx(ctx).Info().Str("type", msg.Type).Msg("ignored log with unsupported type")
			}
		}
	})

	server := &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", s.listenPort),
		Handler: router,
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
		close(s.logQueue)

		// Notify when server is closed
		zerolog.Ctx(ctx).Info().Msg("log service is closed")
		wg.Done()
	}()

	go func() {
		zerolog.Ctx(ctx).Info().Msgf("log service is on http://%s", server.Addr)
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
