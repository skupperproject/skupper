package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/skupperproject/skupper/cmd/network-console-collector/internal/api"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
)

func New(logger *slog.Logger, records store.Interface) api.ServerInterface {
	return &server{
		logger:  logger,
		records: records,
	}
}

type server struct {
	logger  *slog.Logger
	records store.Interface
}

func (c *server) logWriteError(r *http.Request, err error) {
	requestLogger(c.logger, r).Error("failed to write response", slog.Any("error", err))
}

func handleCollection[T any](w http.ResponseWriter, _ *http.Request, response api.CollectionResponseSetter[T], records []T) error {
	response.SetResults(records)
	response.SetCount(int64(len(records)))
	response.SetTimeRangeCount(int64(len(records)))
	if err := encodeResponse(w, http.StatusOK, response); err != nil {
		return fmt.Errorf("response write error: %s", err)
	}
	return nil
}

func handleSingle[T any](w http.ResponseWriter, _ *http.Request, response api.ResponseSetter[T], getter func() (T, bool)) error {
	var (
		out    any = response
		status     = http.StatusOK
	)

	if record, ok := getter(); ok {
		response.SetResults(record)
	} else {
		status = http.StatusNotFound
		out = api.ErrorNotFound{
			Code: "ErrNotFound",
		}
	}
	if err := encodeResponse(w, status, out); err != nil {
		return fmt.Errorf("response write error: %s", err)
	}
	return nil
}

func encodeResponse(w http.ResponseWriter, status int, v interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		return fmt.Errorf("json encoding error: %s", err)
	}
	return nil
}

func requestLogger(log *slog.Logger, r *http.Request) *slog.Logger {
	return log.With(
		slog.String("endpoint", r.URL.Path),
	)
}
