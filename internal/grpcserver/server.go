// Package grpcserver реализует gRPC-сервис приёма метрик.
package grpcserver

import (
	"context"

	"github.com/Meowizz/metrics-collector/internal/audit"
	models "github.com/Meowizz/metrics-collector/internal/model"
	pb "github.com/Meowizz/metrics-collector/internal/proto"
	"github.com/Meowizz/metrics-collector/internal/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type contextKey string

const ipContextKey contextKey = "client-ip"

// MetricsServer реализует сервис Metrics из metrics.proto.
type MetricsServer struct {
	pb.UnimplementedMetricsServer
	storage repository.Storage
}

func NewMetricsServer(storage repository.Storage) *MetricsServer {
	return &MetricsServer{
		storage: storage,
	}
}

func (s *MetricsServer) UpdateMetrics(ctx context.Context, updReq *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {
	metrics := make([]models.Metrics, 0, len(updReq.GetMetrics()))
	clientIP, _ := ctx.Value(ipContextKey).(string)
	if clientIP == "" {
		clientIP = "unknown"
	}
	for _, protoMetric := range updReq.GetMetrics() {
		switch protoMetric.GetType() {

		case pb.Metric_GAUGE:
			v := protoMetric.GetValue()
			metrics = append(metrics, models.Metrics{
				ID:    protoMetric.GetId(),
				MType: "gauge",
				Value: &v,
			})

		case pb.Metric_COUNTER:
			d := protoMetric.GetDelta()
			metrics = append(metrics, models.Metrics{
				ID:    protoMetric.GetId(),
				MType: "counter",
				Delta: &d,
			})

		default:
			return nil, status.Errorf(codes.InvalidArgument, "unknown metric type: %s", protoMetric.GetType())
		}
	}

	err := s.storage.UpdateBatch(metrics)
	if err != nil {
		if s.audi != nil {
			s.audit.Notify(audit.Event{TS: 0, Metrics: metricNames, IP: clientIP}, ctx)
		}
		return nil, status.Error(codes.Internal, "failed to update metrics")
	}

	if s.audit != nil {
		s.audit.Notify(audit.Event{TS: 0, Metrics: metricNames, IP: clientIP}, ctx)
	}
	return &pb.UpdateMetricsResponse{}, nil
}
