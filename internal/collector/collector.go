package collector

import (
	"runtime"
)

type metricType string

const (
	Counter metricType = "counter"
	Gauge   metricType = "gauge"
)

type Metric struct {
	Type  metricType
	Name  string
	Value interface{}
}

type Collector struct {
	metrics map[string]*Metric
}

func NewCollector() *Collector {
	return &Collector{
		metrics: make(map[string]*Metric),
	}
}

func (c *Collector) Collect() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	c.metrics["Alloc"] = &Metric{Type: Gauge, Name: "Alloc", Value: float64(memStats.Alloc)}
	c.metrics["BuckHashSys"] = &Metric{Type: Gauge, Name: "BuckHashSys", Value: float64(memStats.BuckHashSys)}
	c.metrics["Frees"] = &Metric{Type: Gauge, Name: "Frees", Value: float64(memStats.Frees)}
	c.metrics["GCCPUFraction"] = &Metric{Type: Gauge, Name: "GCCPUFraction", Value: memStats.GCCPUFraction}
	c.metrics["GCSys"] = &Metric{Type: Gauge, Name: "GCSys", Value: float64(memStats.GCSys)}
	c.metrics["HeapAlloc"] = &Metric{Type: Gauge, Name: "HeapAlloc", Value: float64(memStats.HeapAlloc)}
	c.metrics["HeapIdle"] = &Metric{Type: Gauge, Name: "HeapIdle", Value: float64(memStats.HeapIdle)}
	c.metrics["HeapInuse"] = &Metric{Type: Gauge, Name: "HeapInuse", Value: float64(memStats.HeapInuse)}
	c.metrics["HeapObjects"] = &Metric{Type: Gauge, Name: "HeapObjects", Value: float64(memStats.HeapObjects)}
	c.metrics["HeapReleased"] = &Metric{Type: Gauge, Name: "HeapReleased", Value: float64(memStats.HeapReleased)}
	c.metrics["HeapSys"] = &Metric{Type: Gauge, Name: "HeapSys", Value: float64(memStats.HeapSys)}
	c.metrics["LastGC"] = &Metric{Type: Gauge, Name: "LastGC", Value: float64(memStats.LastGC)}
	c.metrics["Lookups"] = &Metric{Type: Gauge, Name: "Lookups", Value: float64(memStats.Lookups)}
	c.metrics["MCacheInuse"] = &Metric{Type: Gauge, Name: "MCacheInuse", Value: float64(memStats.MCacheInuse)}
	c.metrics["MCacheSys"] = &Metric{Type: Gauge, Name: "MCacheSys", Value: float64(memStats.MCacheSys)}
	c.metrics["MSpanInuse"] = &Metric{Type: Gauge, Name: "MSpanInuse", Value: float64(memStats.MSpanInuse)}
	c.metrics["MSpanSys"] = &Metric{Type: Gauge, Name: "MSpanSys", Value: float64(memStats.MSpanSys)}
	c.metrics["Mallocs"] = &Metric{Type: Gauge, Name: "Mallocs", Value: float64(memStats.Mallocs)}
	c.metrics["NextGC"] = &Metric{Type: Gauge, Name: "NextGC", Value: float64(memStats.NextGC)}
	c.metrics["NumForcedGC"] = &Metric{Type: Gauge, Name: "NumForcedGC", Value: float64(memStats.NumForcedGC)}
	c.metrics["NumGC"] = &Metric{Type: Gauge, Name: "NumGC", Value: float64(memStats.NumGC)}
	c.metrics["OtherSys"] = &Metric{Type: Gauge, Name: "OtherSys", Value: float64(memStats.OtherSys)}
	c.metrics["PauseTotalNs"] = &Metric{Type: Gauge, Name: "PauseTotalNs", Value: float64(memStats.PauseTotalNs)}
	c.metrics["StackInuse"] = &Metric{Type: Gauge, Name: "StackInuse", Value: float64(memStats.StackInuse)}
	c.metrics["StackSys"] = &Metric{Type: Gauge, Name: "StackSys", Value: float64(memStats.StackSys)}
	c.metrics["Sys"] = &Metric{Type: Gauge, Name: "Sys", Value: float64(memStats.Sys)}
	c.metrics["TotalAlloc"] = &Metric{Type: Gauge, Name: "TotalAlloc", Value: float64(memStats.TotalAlloc)}

	if existing, exists := c.metrics["PollCount"]; exists {
		currentValue := existing.Value.(int64)
		existing.Value = currentValue + 1
	} else {
		c.metrics["PollCount"] = &Metric{Type: Counter, Name: "PollCount", Value: int64(1)}
	}
}

func (c *Collector) GetMetrics() []*Metric {
	result := make([]*Metric, 0, len(c.metrics))
	for _, metric := range c.metrics {
		result = append(result, metric)
	}
	return result
}
