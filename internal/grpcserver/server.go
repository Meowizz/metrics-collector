// Package grpcserver реализует gRPC-сервис приёма метрик.
package grpcserver

import (
	"context"
	"time"

	"github.com/Meowizz/metrics-collector/internal/audit"
	"github.com/Meowizz/metrics-collector/internal/middleware"
	models "github.com/Meowizz/metrics-collector/internal/model"
	pb "github.com/Meowizz/metrics-collector/internal/proto"
	"github.com/Meowizz/metrics-collector/internal/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AuditSubject interface {
	Notify(event audit.Event, ctx context.Context)
}

// MetricsServer реализует сервис Metrics из metrics.proto.
type MetricsServer struct {
	pb.UnimplementedMetricsServer
	storage repository.Storage
	audit   AuditSubject
}

func NewMetricsServer(storage repository.Storage, auditSubject AuditSubject) *MetricsServer {
	return &MetricsServer{
		storage: storage,
		audit:   auditSubject,
	}
}

func (s *MetricsServer) UpdateMetrics(ctx context.Context, updReq *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {
	metrics := make([]models.Metrics, 0, len(updReq.GetMetrics()))
	metricNames := make([]string, 0, len(updReq.GetMetrics()))
	clientIP, _ := ctx.Value(middleware.ClientIPKey).(string)
	if clientIP == "" {
		clientIP = "unknown"
	}
	for _, protoMetric := range updReq.GetMetrics() {
		metricNames = append(metricNames, protoMetric.GetId())

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
			return nil, status.Errorf(codes.InvalidArgument, "unknown metric type: %v", protoMetric.GetType())
		}
	}

	err := s.storage.UpdateBatch(metrics)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to update metrics")
	}

	if s.audit != nil {
		s.audit.Notify(audit.Event{TS: time.Now().Unix(), Metrics: metricNames, IP: clientIP}, ctx)
	}
	return &pb.UpdateMetricsResponse{}, nil
}
