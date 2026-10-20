package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Event — событие аудита, фиксирующее изменения метрик.
type Event struct {
	TS      int64    `json:"ts"`
	Metrics []string `json:"metrics"`
	IP      string   `json:"ip_address"`
}

// Observer — интерфейс наблюдателя для получения событий аудита.
type Observer interface {
	Update(event Event, ctx context.Context)
}

// Subject — реализация паттерна Observer для управления подписками
// и уведомлениями наблюдателей о событиях аудита.
type Subject struct {
	observers []Observer
	mu        sync.Mutex
}

// NewSubject создаёт новый пустой Subject для управления наблюдателями аудита.
// Возвращает указатель на инициализированный Subject.
func NewSubject() *Subject {
	return &Subject{observers: make([]Observer, 0)}
}

// Attach добавляет наблюдателя в список подписчиков для получения событий аудита.
func (s *Subject) Attach(o Observer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observers = append(s.observers, o)
}

// Notify уведомляет всех наблюдателей о событии в отдельных горутинах.
func (s *Subject) Notify(event Event, ctx context.Context) {
	s.mu.Lock()
	observers := make([]Observer, len(s.observers))
	copy(observers, s.observers)
	s.mu.Unlock()

	for _, o := range observers {
		go o.Update(event, ctx)
	}
}

// FileAuditObserver — наблюдатель аудита, записывающий события в файл
// в формате JSON (по одной записи на строку).
type FileAuditObserver struct {
	file   *os.File
	logger *zap.Logger
	mu     sync.Mutex
}

// NewFileAuditObserver создаёт новый FileAuditObserver, открывая файл по указанному пути
// в режиме дозаписи с созданием при отсутствии.
// Возвращает указатель на инициализированный FileAuditObserver или ошибку.
func NewFileAuditObserver(filePath string, logger *zap.Logger) (*FileAuditObserver, error) {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия файла аудита: %w", err)
	}
	return &FileAuditObserver{
		file:   file,
		logger: logger,
	}, nil
}

// Update записывает событие аудита в файл в формате JSON.
func (f *FileAuditObserver) Update(event Event, ctx context.Context) {
	data, err := json.Marshal(event)
	f.mu.Lock()
	defer f.mu.Unlock()

	if err != nil {
		f.logger.Error("ошибка маршалига события аудита", zap.Error(err))
		return
	}

	data = append(data, '\n')

	if _, err := f.file.Write(data); err != nil {
		f.logger.Error("ошибка записи в файл", zap.Error(err))
		return
	}
}

// Close закрывает файл аудита и освобождает ресурсы.
func (f *FileAuditObserver) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.file.Close()
}

// HTTPAuditObserver — наблюдатель аудита, отправляющий события на удалённый HTTP-сервер
// через POST-запрос в формате JSON.
type HTTPAuditObserver struct {
	client *http.Client
	logger *zap.Logger
	url    string
}

// NewHTTPObserver создаёт новый HTTPAuditObserver с указанным URL для отправки событий.
// Время ожидания HTTP-клиента — 5 секунд.
// Возвращает указатель на инициализированный HTTPAuditObserver.
func NewHTTPObserver(host string, logger *zap.Logger) *HTTPAuditObserver {
	return &HTTPAuditObserver{
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		logger: logger,
		url:    host,
	}
}

// Update отправляет событие аудита на удалённый HTTP-сервер через POST-запрос.
func (h *HTTPAuditObserver) Update(event Event, ctx context.Context) {
	data, err := json.Marshal(event)
	if err != nil {
		h.logger.Error("ошибка маршалига события аудита", zap.Error(err))
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.url, bytes.NewBuffer(data))
	if err != nil {
		h.logger.Error("ошибка создания http request", zap.Error(err))
		return
	}

	var resp *http.Response
	var retryErr error
	delays := []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}

	for attempt := 0; attempt <= 3; attempt++ {
		resp, retryErr = h.client.Do(req)
		if retryErr == nil {
			break
		}
		if attempt < 3 {
			time.Sleep(delays[attempt])
		}
	}

	if retryErr != nil {
		h.logger.Error("Audit HTTP send error", zap.Error(retryErr))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		h.logger.Error("Audit HTTP receiver returned error status", zap.Int("status", resp.StatusCode))
	}
}
