package agent

import (
	"log"
	"sync"
	"time"

	"github.com/Meowizz/metrics-collector/internal/collector"
	"github.com/Meowizz/metrics-collector/internal/sender"
)

type BatchSender struct {
	sender      *sender.Sender
	buffer      []*collector.Metric
	mu          sync.Mutex
	maxBatchSize int
	ticker      *time.Ticker
}

func NewBatchSender(s *sender.Sender, maxBatchSize int, interval time.Duration) *BatchSender {
	bs := &BatchSender{
		sender:       s,
		buffer:       make([]*collector.Metric, 0, maxBatchSize),
		maxBatchSize: maxBatchSize,
		ticker:       time.NewTicker(interval),
	}

	go bs.runTimer()

	return bs
}

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
		log.Printf("Failed to send batch: %v", err)
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

func (bs *BatchSender) FlushAndStop() {
	bs.ticker.Stop()

	bs.mu.Lock()
	defer bs.mu.Unlock()
	bs.flush()
}

func (bs *BatchSender) Stop() {
	bs.ticker.Stop()
}
