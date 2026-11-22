package sender

import (
	"context"
	"fmt"

	"github.com/Meowizz/metrics-collector/internal/collector"
	pb "github.com/Meowizz/metrics-collector/internal/proto"
	"github.com/Meowizz/metrics-collector/pkg"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// GRPCSender — отправитель метрик на сервер через gRPC.
type GRPCSender struct {
	conn   *grpc.ClientConn
	client pb.MetricsClient
	ip     string
}

// NewGRPCSender создаёт новый gRPC-клиент для отправки метрик.
// target — адрес gRPC-сервера (например, "localhost:50051").
// ip — IP-адрес агента, который будет передаваться в метаданных x-real-ip.
func NewGRPCSender(target string, ip string) (*GRPCSender, error) {
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection: %w", err)
	}
	return &GRPCSender{
		conn:   conn,
		client: pb.NewMetricsClient(conn),
		ip:     ip,
	}, nil
}

// Close закрывает gRPC-соединение.
func (s *GRPCSender) Close() error {
	if s.conn != nil {
		s.conn.Close()
	}
	return nil
}

// SendBatch отправляет пакет метрик одним gRPC-запросом UpdateMetrics.
// Повторяет при Unavailable, завершается с ErrNotRetriable при PermissionDenied/InvalidArgument.
func (s *GRPCSender) SendBatch(metric []*collector.Metric) error {
	if len(metric) == 0 {
		return nil
	}

	pbMetrics := make([]*pb.Metric, len(metric))
	for _, m := range metric {
		pm := &pb.Metric{}
		pm.SetId(m.Name)

		switch v := m.Value.(type) {
		case float64:
			pm.SetType(pb.Metric_GAUGE)
			pm.SetValue(v)
		case int64:
			pm.SetType(pb.Metric_COUNTER)
			pm.SetDelta(v)
		default:
			return fmt.Errorf("unknown metric type: %T", v)
		}
		pbMetrics = append(pbMetrics, pm)
	}

	req := &pb.UpdateMetricsRequest{}
	req.SetMetrics(pbMetrics)

	md := metadata.Pairs("x-real-ip", s.ip)
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	return pkg.DoWithRetry(func() error {
		_, err := s.client.UpdateMetrics(ctx, req)
		if err == nil {
			return nil
		}

		st, ok := status.FromError(err)
		if !ok {
			return fmt.Errorf("gRPC network error:%w", err)
		}
		switch st.Code() {
		case codes.Unavailable:
			return fmt.Errorf("gRPC connection unavailable: %w", err)
		case codes.PermissionDenied, codes.InvalidArgument:
			return fmt.Errorf("gRPC error %s: %w", st.Code(), pkg.ErrNotRetriable)
		default:
			return fmt.Errorf("gRPC error %s: %w", st.Code(), pkg.ErrNotRetriable)
		}
	})
}

var _ MectricSender = (*GRPCSender)(nil)
