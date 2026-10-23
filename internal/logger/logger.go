package logger

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

var Log *zap.Logger = zap.NewNop()

type (
	responseData struct {
		status int
		size   int
	}

	loggingResponseWriter struct {
		http.ResponseWriter
		responseData *responseData
	}
)

func (r *loggingResponseWriter) Write(b []byte) (int, error) {

	size, err := r.ResponseWriter.Write(b)
	r.responseData.size += size
	return size, err
}

func (r *loggingResponseWriter) WriteHeader(statusCode int) {

	r.ResponseWriter.WriteHeader(statusCode)
	r.responseData.status = statusCode
}

// Initialize инициализирует глобальный zap-логгер с указанным уровнем логирования.
// Поддерживаемые уровни: debug, info, warn, error, fatal.
// Заменяет глобальную переменную Log на настроенный логгер.
// Возвращает ошибку при некорректном уровне или проблеме конфигурации.
func Initialize(level string) error {
	lvl, err := zap.ParseAtomicLevel(level)

	if err != nil {
		return err
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = lvl
	zl, err := cfg.Build()

	if err != nil {
		return err
	}

	Log = zl
	return nil
}

// WithLogging — middleware для логирования HTTP-запросов с помощью zap.
// Записывает URI, метод, код статуса ответа, длительность и размер ответа.
func WithLogging(h http.Handler) http.Handler {
	logFn := func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		responseData := &responseData{
			status: http.StatusOK,
			size:   0,
		}
		lw := loggingResponseWriter{
			ResponseWriter: w,
			responseData:   responseData,
		}
		h.ServeHTTP(&lw, r)

		duration := time.Since(start)

		Log.Info("Incoming HTTP request",
			zap.String("uri", r.RequestURI),
			zap.String("method", r.Method),
			zap.Int("status", responseData.status), // получаем перехваченный код статуса ответа
			zap.Duration("duration", duration),
			zap.Int("size", responseData.size), // получаем перехваченный размер ответа
		)
	}
	return http.HandlerFunc(logFn)
}
