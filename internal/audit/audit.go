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

// defaultBufSize — размер буфера по умолчанию для каналов observerWorker.
const defaultBufSize = 1024

// observerWorker — выделенная горутина-воркер с буферизованным каналом
// для одного наблюдателя. Гарантирует одну горутину на observer вместо
// создания новой горутны на каждое событие.
type observerWorker struct {
	observer Observer
	ch       chan Event
	done     chan struct{}
	wg       sync.WaitGroup
}

// newObserverWorker создаёт нового observerWorker, запускает горутину-воркер.
func newObserverWorker(o Observer, bufSize int) *observerWorker {
	w := &observerWorker{
		observer: o,
		ch:       make(chan Event, bufSize),
		done:     make(chan struct{}),
	}
	w.wg.Add(1)
	go w.run()
	return w
}

// run — основной цикл воркера: читает события из канала и передаёт их observer.
func (w *observerWorker) run() {
	defer w.wg.Done()

	for {
		select {
		case <-w.done:
			// Graceful shutdown: обрабатываем оставшиеся события
			for {
				select {
				case event := <-w.ch:
					w.observer.Update(event, context.Background())
				default:
					return
				}
			}
		case event := <-w.ch:
			w.observer.Update(event, context.Background())
		}
	}
}

// send отправляет событие в канал воркера.
// Non-blocking: при переполнении буфера отбрасывает самое старое событие.
// Возвращает true если событие принято, false если worker остановлен.
func (w *observerWorker) send(event Event) bool {
	select {
	case <-w.done:
		return false
	default:
	}

	select {
	case w.ch <- event:
		return true
	default:
		// Буфер полон — отбрасываем самое старое событие
		select {
		case <-w.ch:
		default:
		}
		// Повторная попытка после освобождения места
		select {
		case w.ch <- event:
			return true
		default:
			// Zero buffer и нет читателя — дропаем событие
			return false
		}
	}
}

// stop отправляет сигнал остановки воркеру.
func (w *observerWorker) stop() {
	close(w.done)
}

// wait ожидает завершения горутны-воркера.
func (w *observerWorker) wait() {
	w.wg.Wait()
}

// Subject — реализация паттерна Observer для управления подписками
// и уведомлениями наблюдателей о событиях аудита.
// Каждый наблюдатель имеет своего воркера с выделенной горутинной.
type Subject struct {
	workers map[Observer]*observerWorker
	mu      sync.RWMutex
}

// NewSubject создаёт новый пустой Subject для управления наблюдателями аудита.
// Возвращает указатель на инициализированный Subject.
func NewSubject() *Subject {
	return &Subject{workers: make(map[Observer]*observerWorker)}
}

// Attach добавляет наблюдателя в список подписчиков для получения событий аудита.
// Если данный observer уже подключён, метод ничего не делает.
func (s *Subject) Attach(o Observer) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.workers[o]; exists {
		return
	}

	s.workers[o] = newObserverWorker(o, defaultBufSize)
}

// Notify уведомляет всех наблюдателей о событии через их воркеры.
// Никаких новых горутин не создаётся.
func (s *Subject) Notify(event Event, ctx context.Context) {
	s.mu.RLock()
	workers := make(map[Observer]*observerWorker, len(s.workers))
	for k, v := range s.workers {
		workers[k] = v
	}
	s.mu.RUnlock()

	for _, w := range workers {
		w.send(event)
	}
}

// Detach удаляет наблюдателя и останавливает его воркер с graceful shutdown.
func (s *Subject) Detach(o Observer) {
	s.mu.Lock()
	w, exists := s.workers[o]
	if !exists {
		s.mu.Unlock()
		return
	}
	delete(s.workers, o)
	s.mu.Unlock()

	w.stop()
	w.wait()
}

// Shutdown полностью останавливает все воркеры и освобождает ресурсы.
func (s *Subject) Shutdown() {
	s.mu.Lock()
	workers := make([]*observerWorker, 0, len(s.workers))
	for _, w := range s.workers {
		workers = append(workers, w)
	}
	s.workers = make(map[Observer]*observerWorker)
	s.mu.Unlock()

	for _, w := range workers {
		w.stop()
		w.wait()
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
