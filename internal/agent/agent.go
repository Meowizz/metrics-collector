package agent

import (
	"sync"
	"time"

	"github.com/Meowizz/metrics-collector/internal/collector"
	"github.com/Meowizz/metrics-collector/internal/sender"
	"go.uber.org/zap"
)

// BatchSender — буферизатор метрик, отправляющий их батчами при достижении
// максимального размера или по таймеру.
type BatchSender struct {
	sender       *sender.Sender
	buffer       []*collector.Metric
	logger       *zap.Logger
	mu           sync.Mutex
	maxBatchSize int
	ticker       *time.Ticker
}

// NewBatchSender создаёт новый BatchSender с указанным отправителем,
// максимальным размером батча и интервалом таймера для периодической отправки.
// Возвращает указатель на инициализированный BatchSender. Запускает фоновый таймер.
func NewBatchSender(s *sender.Sender, maxBatchSize int, interval time.Duration, logger *zap.Logger) *BatchSender {
	bs := &BatchSender{
		sender:       s,
		buffer:       make([]*collector.Metric, 0, maxBatchSize),
		logger:       logger,
		maxBatchSize: maxBatchSize,
		ticker:       time.NewTicker(interval),
	}

	go bs.runTimer()

	return bs
}

// Add добавляет метрики в буфер и отправляет батч при достижении лимита.
func (bs *BatchSender) Add(metrics []*collector.Metric) {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	bs.buffer = append(bs.buffer, metrics...)

	if len(bs.buffer) >= bs.maxBatchSize {
		bs.flush()
	}
}

func (bs *BatchSender) flush() {
	if len(bs.buffer) == 0 {
		return
	}

	if err := bs.sender.SendBatch(bs.buffer); err != nil {
		bs.logger.Error("Failed to send batch", zap.Error(err))
	}

	bs.buffer = bs.buffer[:0]
}

func (bs *BatchSender) runTimer() {
	for range bs.ticker.C {
		bs.mu.Lock()
		bs.flush()
		bs.mu.Unlock()
	}
}

// FlushAndStop отправляет оставшиеся метрики и останавливает таймер.
func (bs *BatchSender) FlushAndStop() {
	bs.ticker.Stop()

	bs.mu.Lock()
	defer bs.mu.Unlock()
	bs.flush()
}

func (bs *BatchSender) Stop() {
	bs.ticker.Stop()
}

// WorkerPool — пул воркеров для асинхронной отправки метрик через канал.
type WorkerPool struct {
	ingestChan  chan []*collector.Metric
	logger      *zap.Logger
	sender      *sender.Sender
	workerCount int
	wg          sync.WaitGroup
}

// NewWorkerPool создаёт новый WorkerPool с указанным отправителем и количеством воркеров.
// Возвращает указатель на инициализированный WorkerPool с буферизованным каналом (ёмкость 20).
func NewWorkerPool(s *sender.Sender, workerCount int, logger *zap.Logger) *WorkerPool {
	return &WorkerPool{
		ingestChan:  make(chan []*collector.Metric, 20),
		logger:      logger,
		sender:      s,
		workerCount: workerCount,
	}
}

// StartWorkingPool запускает пул воркеров для асинхронной отправки метрик.
func (wp *WorkerPool) StartWorkingPool() {
	for i := 0; i < wp.workerCount; i++ {
		wp.wg.Add(1)
		go func(workerID int) {
			defer wp.wg.Done()
			for batch := range wp.ingestChan {
				if len(batch) == 0 {
					continue
				}
				err := wp.sender.Send(batch)
				if err != nil {
					wp.logger.Error("Ошибка отправки", zap.Int("worker", workerID), zap.Error(err))
				}
			}
		}(i)
	}
}

// Ingest отправляет метрики в канал для обработки воркерами.
func (wp *WorkerPool) Ingest(metrics []*collector.Metric) {
	wp.ingestChan <- metrics
}

// Close закрывает канал и ожидает завершения всех воркеров.
func (wp *WorkerPool) Close() {
	close(wp.ingestChan)
	wp.wg.Wait()
}
