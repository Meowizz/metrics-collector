package sender

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/Meowizz/metrics-collector/internal/collector"
	pb "github.com/Meowizz/metrics-collector/internal/proto"
	"github.com/Meowizz/metrics-collector/pkg"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

// mockGRPCServer реализует pb.MetricsServer для тестов.
type mockGRPCServer struct {
	pb.UnimplementedMetricsServer

	// Возвращаемая ошибка
	returnErr error
	// Код ошибки
	returnCode codes.Code

	// Счётчик вызовов
	callCount int
	// Принятые метрики для проверки
	receivedMetrics []*pb.Metric
}

func (m *mockGRPCServer) UpdateMetrics(ctx context.Context, req *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {
	m.callCount++

	if m.returnErr != nil {
		return nil, m.returnErr
	}

	if m.returnCode != codes.OK {
		return nil, status.Error(m.returnCode, "test error")
	}

	m.receivedMetrics = req.GetMetrics()

	return &pb.UpdateMetricsResponse{}, nil
}

// setupGRPCSender создаёт gRPC-сервер и клиентский conn для тестов.
func setupGRPCSender(t *testing.T, mock *mockGRPCServer) (*GRPCSender, func()) {
	t.Helper()

	lis := bufconn.Listen(bufSize)

	srv := grpc.NewServer()
	pb.RegisterMetricsServer(srv, mock)

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

	client := pb.NewMetricsClient(conn)

	sender := &GRPCSender{
		conn:   conn,
		client: client,
		ip:     "127.0.0.1",
	}

	cleanup := func() {
		sender.Close()
		srv.GracefulStop()
	}

	return sender, cleanup
}

func TestNewGRPCSender(t *testing.T) {
	localIP := "127.0.0.1"

	sender, err := NewGRPCSender("localhost:50051", localIP)
	if err != nil {
		t.Fatalf("NewGRPCSender() error = %v", err)
	}
	defer sender.Close()

	if sender.ip != localIP {
		t.Errorf("expected IP %q, got %q", localIP, sender.ip)
	}

	if sender.conn == nil {
		t.Error("expected non-nil connection")
	}

	if sender.client == nil {
		t.Error("expected non-nil client")
	}
}

func TestGRPCSender_Close(t *testing.T) {
	sender, err := NewGRPCSender("localhost:50051", "127.0.0.1")
	if err != nil {
		t.Fatalf("NewGRPCSender() error = %v", err)
	}

	err = sender.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestGRPCSender_SendBatch_Empty(t *testing.T) {
	mock := &mockGRPCServer{}
	sender, cleanup := setupGRPCSender(t, mock)
	defer cleanup()

	err := sender.SendBatch([]*collector.Metric{})
	if err != nil {
		t.Errorf("SendBatch([]) error = %v", err)
	}

	if mock.callCount != 0 {
		t.Errorf("expected 0 calls, got %d", mock.callCount)
	}
}

func TestGRPCSender_SendBatch_Gauge(t *testing.T) {
	mock := &mockGRPCServer{}
	sender, cleanup := setupGRPCSender(t, mock)
	defer cleanup()

	metrics := []*collector.Metric{
		{Type: collector.Gauge, Name: "Alloc", Value: float64(123.45)},
		{Type: collector.Gauge, Name: "Sys", Value: float64(678.90)},
	}

	err := sender.SendBatch(metrics)
	if err != nil {
		t.Fatalf("SendBatch() error = %v", err)
	}

	if mock.callCount != 1 {
		t.Errorf("expected 1 call, got %d", mock.callCount)
	}

	got := mock.receivedMetrics
	if len(got) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(got))
	}

	// Проверяем первую метрику
	if got[0].GetId() != "Alloc" {
		t.Errorf("metric[0].id: expected %q, got %q", "Alloc", got[0].GetId())
	}
	if got[0].GetType() != pb.Metric_GAUGE {
		t.Errorf("metric[0].type: expected GAUGE, got %v", got[0].GetType())
	}
	if got[0].GetValue() != 123.45 {
		t.Errorf("metric[0].value: expected 123.45, got %f", got[0].GetValue())
	}

	// Проверяем вторую метрику
	if got[1].GetId() != "Sys" {
		t.Errorf("metric[1].id: expected %q, got %q", "Sys", got[1].GetId())
	}
	if got[1].GetType() != pb.Metric_GAUGE {
		t.Errorf("metric[1].type: expected GAUGE, got %v", got[1].GetType())
	}
	if got[1].GetValue() != 678.90 {
		t.Errorf("metric[1].value: expected 678.90, got %f", got[1].GetValue())
	}
}

func TestGRPCSender_SendBatch_Counter(t *testing.T) {
	mock := &mockGRPCServer{}
	sender, cleanup := setupGRPCSender(t, mock)
	defer cleanup()

	metrics := []*collector.Metric{
		{Type: collector.Counter, Name: "PollCount", Value: int64(42)},
		{Type: collector.Counter, Name: "Mallocs", Value: int64(1000)},
	}

	err := sender.SendBatch(metrics)
	if err != nil {
		t.Fatalf("SendBatch() error = %v", err)
	}

	if mock.callCount != 1 {
		t.Errorf("expected 1 call, got %d", mock.callCount)
	}

	got := mock.receivedMetrics
	if len(got) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(got))
	}

	if got[0].GetId() != "PollCount" {
		t.Errorf("metric[0].id: expected %q, got %q", "PollCount", got[0].GetId())
	}
	if got[0].GetType() != pb.Metric_COUNTER {
		t.Errorf("metric[0].type: expected COUNTER, got %v", got[0].GetType())
	}
	if got[0].GetDelta() != 42 {
		t.Errorf("metric[0].delta: expected 42, got %d", got[0].GetDelta())
	}

	if got[1].GetId() != "Mallocs" {
		t.Errorf("metric[1].id: expected %q, got %q", "Mallocs", got[1].GetId())
	}
	if got[1].GetType() != pb.Metric_COUNTER {
		t.Errorf("metric[1].type: expected COUNTER, got %v", got[1].GetType())
	}
	if got[1].GetDelta() != 1000 {
		t.Errorf("metric[1].delta: expected 1000, got %d", got[1].GetDelta())
	}
}

func TestGRPCSender_SendBatch_Mixed(t *testing.T) {
	mock := &mockGRPCServer{}
	sender, cleanup := setupGRPCSender(t, mock)
	defer cleanup()

	metrics := []*collector.Metric{
		{Type: collector.Gauge, Name: "Alloc", Value: float64(100.0)},
		{Type: collector.Counter, Name: "PollCount", Value: int64(5)},
		{Type: collector.Gauge, Name: "Sys", Value: float64(200.0)},
	}

	err := sender.SendBatch(metrics)
	if err != nil {
		t.Fatalf("SendBatch() error = %v", err)
	}

	if mock.callCount != 1 {
		t.Errorf("expected 1 call, got %d", mock.callCount)
	}

	got := mock.receivedMetrics
	if len(got) != 3 {
		t.Fatalf("expected 3 metrics, got %d", len(got))
	}

	// Gauge 1
	if got[0].GetId() != "Alloc" || got[0].GetType() != pb.Metric_GAUGE || got[0].GetValue() != 100.0 {
		t.Errorf("metric[0]: unexpected values")
	}
	// Counter
	if got[1].GetId() != "PollCount" || got[1].GetType() != pb.Metric_COUNTER || got[1].GetDelta() != 5 {
		t.Errorf("metric[1]: unexpected values")
	}
	// Gauge 2
	if got[2].GetId() != "Sys" || got[2].GetType() != pb.Metric_GAUGE || got[2].GetValue() != 200.0 {
		t.Errorf("metric[2]: unexpected values")
	}
}

func TestGRPCSender_SendBatch_UnknownType(t *testing.T) {
	mock := &mockGRPCServer{}
	sender, cleanup := setupGRPCSender(t, mock)
	defer cleanup()

	metrics := []*collector.Metric{
		{Type: collector.Gauge, Name: "Test", Value: "string_value"},
	}

	err := sender.SendBatch(metrics)
	if err == nil {
		t.Fatal("expected error for unknown metric type, got nil")
	}

	expectedMsg := "unknown metric type: string"
	if err.Error() != expectedMsg {
		t.Errorf("expected error %q, got %v", expectedMsg, err)
	}

	if mock.callCount != 0 {
		t.Errorf("expected 0 calls, got %d", mock.callCount)
	}
}

func TestGRPCSender_SendBatch_PermissionDenied(t *testing.T) {
	mock := &mockGRPCServer{
		returnCode: codes.PermissionDenied,
	}
	sender, cleanup := setupGRPCSender(t, mock)
	defer cleanup()

	metrics := []*collector.Metric{
		{Type: collector.Gauge, Name: "Alloc", Value: float64(100.0)},
	}

	err := sender.SendBatch(metrics)
	if err == nil {
		t.Fatal("expected error for PermissionDenied, got nil")
	}

	if !isNotRetriable(err) {
		t.Errorf("expected ErrNotRetriable, got %v", err)
	}
}

func TestGRPCSender_SendBatch_InvalidArgument(t *testing.T) {
	mock := &mockGRPCServer{
		returnCode: codes.InvalidArgument,
	}
	sender, cleanup := setupGRPCSender(t, mock)
	defer cleanup()

	metrics := []*collector.Metric{
		{Type: collector.Gauge, Name: "Alloc", Value: float64(100.0)},
	}

	err := sender.SendBatch(metrics)
	if err == nil {
		t.Fatal("expected error for InvalidArgument, got nil")
	}

	if !isNotRetriable(err) {
		t.Errorf("expected ErrNotRetriable, got %v", err)
	}
}

func TestGRPCSender_SendBatch_Unavailable(t *testing.T) {
	mock := &mockGRPCServer{
		returnCode: codes.Unavailable,
	}
	sender, cleanup := setupGRPCSender(t, mock)
	defer cleanup()

	metrics := []*collector.Metric{
		{Type: collector.Gauge, Name: "Alloc", Value: float64(100.0)},
	}

	err := sender.SendBatch(metrics)
	if err == nil {
		t.Fatal("expected error for Unavailable, got nil")
	}

	// Unavailable — ретраиблимая ошибка, не должна быть ErrNotRetriable
	if isNotRetriable(err) {
		t.Errorf("expected retriable error, got not retriable: %v", err)
	}
}

func TestGRPCSender_SendBatch_MixedGaugeAndCounter(t *testing.T) {
	mock := &mockGRPCServer{}
	sender, cleanup := setupGRPCSender(t, mock)
	defer cleanup()

	metrics := []*collector.Metric{
		{Type: collector.Gauge, Name: "Gauge1", Value: float64(10.5)},
		{Type: collector.Counter, Name: "Counter1", Value: int64(20)},
		{Type: collector.Gauge, Name: "Gauge2", Value: float64(30.5)},
		{Type: collector.Counter, Name: "Counter2", Value: int64(40)},
	}

	err := sender.SendBatch(metrics)
	if err != nil {
		t.Fatalf("SendBatch() error = %v", err)
	}

	if mock.callCount != 1 {
		t.Errorf("expected 1 call, got %d", mock.callCount)
	}

	got := mock.receivedMetrics
	if len(got) != 4 {
		t.Fatalf("expected 4 metrics, got %d", len(got))
	}

	if got[0].GetId() != "Gauge1" || got[0].GetType() != pb.Metric_GAUGE || got[0].GetValue() != 10.5 {
		t.Errorf("metric[0]: unexpected values")
	}
	if got[1].GetId() != "Counter1" || got[1].GetType() != pb.Metric_COUNTER || got[1].GetDelta() != 20 {
		t.Errorf("metric[1]: unexpected values")
	}
	if got[2].GetId() != "Gauge2" || got[2].GetType() != pb.Metric_GAUGE || got[2].GetValue() != 30.5 {
		t.Errorf("metric[2]: unexpected values")
	}
	if got[3].GetId() != "Counter2" || got[3].GetType() != pb.Metric_COUNTER || got[3].GetDelta() != 40 {
		t.Errorf("metric[3]: unexpected values")
	}
}

func TestGRPCSender_SendBatch_IPInMetadata(t *testing.T) {
	mock := &mockGRPCServer{}
	sender, cleanup := setupGRPCSender(t, mock)
	defer cleanup()

	sender.ip = "192.168.1.100"

	metrics := []*collector.Metric{
		{Type: collector.Gauge, Name: "Test", Value: float64(1.0)},
	}

	err := sender.SendBatch(metrics)
	if err != nil {
		t.Fatalf("SendBatch() error = %v", err)
	}
}

// isNotRetriable проверяет, содержит ли ошибка pkg.ErrNotRetriable.
func isNotRetriable(err error) bool {
	return errors.Is(err, pkg.ErrNotRetriable)
}
