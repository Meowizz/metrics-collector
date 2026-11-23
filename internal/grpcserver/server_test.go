package grpcserver_test

import (
	"context"
	"net"
	"net/netip"
	"testing"

	"github.com/Meowizz/metrics-collector/internal/audit"
	"github.com/Meowizz/metrics-collector/internal/grpcserver"
	appMiddleware "github.com/Meowizz/metrics-collector/internal/middleware"
	pb "github.com/Meowizz/metrics-collector/internal/proto"
	"github.com/Meowizz/metrics-collector/internal/repository"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

// setupServer создаёт in-memory gRPC сервер с bufconn для тестов.
// subnet — доверенная подсеть (пустой Prefix отключает проверку).
func setupServer(t *testing.T, subnet netip.Prefix) (*grpc.ClientConn, repository.Storage) {
	t.Helper()

	lis := bufconn.Listen(bufSize)
	storage := repository.NewMemStorage()
	auditSubj := audit.NewSubject()

	srv := grpc.NewServer(
		grpc.UnaryInterceptor(appMiddleware.GRPCAuthInterceptor(subnet)),
	)
	pb.RegisterMetricsServer(srv, grpcserver.NewMetricsServer(storage, auditSubj))

	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("Server exited: %v", err)
		}
	}()

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to dial bufnet: %v", err)
	}

	t.Cleanup(func() {
		conn.Close()
		srv.GracefulStop()
	})

	return conn, storage
}

// Тест 1: Сервис сохраняет метрики
func TestMetricsServer_UpdateMetrics(t *testing.T) {
	// Подсеть пустая — проверка отключена
	conn, storage := setupServer(t, netip.Prefix{})
	client := pb.NewMetricsClient(conn)

	ctx := context.Background()

	// Отправляем GAUGE
	req := &pb.UpdateMetricsRequest{}
	metric := &pb.Metric{}
	metric.SetId("TestGauge")
	metric.SetType(pb.Metric_GAUGE)
	metric.SetValue(42.5)
	req.SetMetrics([]*pb.Metric{metric})

	_, err := client.UpdateMetrics(ctx, req)
	if err != nil {
		t.Fatalf("UpdateMetrics failed: %v", err)
	}

	// Проверяем, что метрика сохранилась в хранилище
	val, ok := storage.GetGauge("TestGauge")
	if !ok {
		t.Fatal("metric TestGauge not found in storage")
	}
	if val != 42.5 {
		t.Errorf("expected 42.5, got %v", val)
	}
}

// Тест 2: Counter накапливается
func TestMetricsServer_UpdateMetrics_CounterAccumulates(t *testing.T) {
	conn, storage := setupServer(t, netip.Prefix{})
	client := pb.NewMetricsClient(conn)

	ctx := context.Background()

	// Отправляем counter дважды
	for i := 0; i < 2; i++ {
		req := &pb.UpdateMetricsRequest{}
		metric := &pb.Metric{}
		metric.SetId("TestCounter")
		metric.SetType(pb.Metric_COUNTER)
		metric.SetDelta(10)
		req.SetMetrics([]*pb.Metric{metric})

		_, err := client.UpdateMetrics(ctx, req)
		if err != nil {
			t.Fatalf("UpdateMetrics failed: %v", err)
		}
	}

	val, ok := storage.GetCounter("TestCounter")
	if !ok {
		t.Fatal("metric TestCounter not found")
	}
	if val != 20 {
		t.Errorf("expected 20 (10+10), got %v", val)
	}
}

// Тест 3: Interceptor — PermissionDenied при неверном IP
func TestGRPCAuthInterceptor_PermissionDenied(t *testing.T) {
	subnet, _ := netip.ParsePrefix("127.0.0.0/8")
	conn, _ := setupServer(t, subnet)
	client := pb.NewMetricsClient(conn)

	// Добавляем неверный IP в метаданные
	md := metadata.Pairs("x-real-ip", "192.168.1.1")
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	req := &pb.UpdateMetricsRequest{}
	req.SetMetrics([]*pb.Metric{})

	_, err := client.UpdateMetrics(ctx, req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}

	if st.Code() != codes.PermissionDenied {
		t.Errorf("expected PermissionDenied, got %v", st.Code())
	}
}

// Тест 4: Interceptor — пропускает при пустой подсети
func TestGRPCAuthInterceptor_NoSubnet(t *testing.T) {
	conn, _ := setupServer(t, netip.Prefix{})
	client := pb.NewMetricsClient(conn)

	md := metadata.Pairs("x-real-ip", "192.168.1.1")
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	req := &pb.UpdateMetricsRequest{}
	req.SetMetrics([]*pb.Metric{})

	_, err := client.UpdateMetrics(ctx, req)
	if err != nil {
		t.Fatalf("expected no error with empty subnet, got %v", err)
	}
}
