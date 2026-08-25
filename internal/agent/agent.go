package agent

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Meowizz/metrics-collector/internal/collector"
	"github.com/Meowizz/metrics-collector/internal/sender"
)

type BatchSender struct {
	sender       *sender.Sender
	buffer       []*collector.Metric
	mu           sync.Mutex
	maxBatchSize int
	ticker       *time.Ticker
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

type WorkerPool struct {
	ingestChan chan []*collector.Metric
	sender     *sender.Sender
	workerCount int
}

func NewWorkerPool(s *sender.Sender, workerCount int) *WorkerPool {
	return &WorkerPool{
		ingestChan: make(chan []*collector.Metric, 20),
		sender:     s,
		workerCount: workerCount,
	}
}

func (wp *WorkerPool) StartWorkingPool(wg *sync.WaitGroup) {
	for i := 0; i <= wp.workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for batch := range wp.ingestChan {
				if len(batch)==0 {
					continue
				}
				err := wp.sender.Send(batch)
				if err != nil {
					fmt.Printf("[Worker %d] Ошибка отправки: %v",workerID,err)
				}
			}
		}(i)
	}
}

func (wp *WorkerPool) Ingest (metrics []*collector.Metric){
	wp.ingestChan <- metrics
}

func (wp *WorkerPool) Close(){
	close(wp.ingestChan)
}
