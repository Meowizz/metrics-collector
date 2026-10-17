package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// Event — событие аудита, фиксирующее изменения метрик.
type Event struct {
	TS      int      `json:"ts"`
	Metrics []string `json:"metrics"`
	IP      string   `json:"ip_address"`
}

// Observer — интерфейс наблюдателя для получения событий аудита.
type Observer interface {
	Update(event Event)
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

func (s *Subject) Attach(o Observer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observers = append(s.observers, o)
}

func (s *Subject) Notify(event Event) {
	s.mu.Lock()
	observers := make([]Observer, len(s.observers))
	copy(observers, s.observers)
	s.mu.Unlock()

	for _, o := range observers {
		go o.Update(event)
	}
}

// FileAuditObserver — наблюдатель аудита, записывающий события в файл
// в формате JSON (по одной записи на строку).
type FileAuditObserver struct {
	file *os.File
	mu   sync.Mutex
}

// NewFileAuditObserver создаёт новый FileAuditObserver, открывая файл по указанному пути
// в режиме дозаписи с созданием при отсутствии.
// Возвращает указатель на инициализированный FileAuditObserver или ошибку.
func NewFileAuditObserver(filePath string) (*FileAuditObserver, error) {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия файла аудита: %w", err)
	}
	return &FileAuditObserver{
		file: file,
	}, nil
}

func (f *FileAuditObserver) Update(event Event) {
	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := json.Marshal(event)

	if err != nil {
		fmt.Printf("ошибка маршалига события аудита: %v", err)
		return
	}

	data = append(data, '\n')

	if _, err := f.file.Write(data); err != nil {
		fmt.Printf("ошибка записи в файл: %v", err)
		return
	}
}

func (f *FileAuditObserver) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.file.Close()
}

// HTTPAuditObserver — наблюдатель аудита, отправляющий события на удалённый HTTP-сервер
// через POST-запрос в формате JSON.
type HTTPAuditObserver struct {
	client *http.Client
	url    string
}

// NewHTTPObserver создаёт новый HTTPAuditObserver с указанным URL для отправки событий.
// Время ожидания HTTP-клиента — 5 секунд.
// Возвращает указатель на инициализированный HTTPAuditObserver.
func NewHTTPObserver(host string) *HTTPAuditObserver {
	return &HTTPAuditObserver{
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		url: host,
	}
}

func (h *HTTPAuditObserver) Update(event Event) {
	data, err := json.Marshal(event)

	if err != nil {
		log.Printf("ошибка маршалига события аудита: %v", err)
		return
	}

	req, err := http.NewRequest(http.MethodPost, h.url, bytes.NewBuffer(data))

	if err != nil {
		log.Printf("ошибка создания http request: %v", err)
		return
	}
	resp, err := h.client.Do(req)
	if err != nil {
		log.Printf("Audit HTTP send error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("Audit HTTP receiver returned error status: %s", resp.Status)
	}
}
