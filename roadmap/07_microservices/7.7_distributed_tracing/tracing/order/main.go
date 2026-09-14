package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/Kefir4c/go-knowledge-base/roadmap/07_microservices/7.7_distributed_tracing/tracing/telemetry"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

var tracer = otel.Tracer("order-service")

// PaymentClient — HTTP-клиент с OTel-инструментацией.
// Автоматически прокидывает traceparent во все запросы.
var paymentClient = &http.Client{
	Transport: otelhttp.NewTransport(http.DefaultTransport),
	Timeout:   5 * time.Second,
}

type OrderRequest struct {
	UserID string  `json:"user_id"`
	Amount float64 `json:"amount"`
}

type OrderResponse struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

func main() {
	ctx := context.Background()

	shutdown, err := telemetry.InitTracer(ctx, "order-service")
	if err != nil {
		log.Fatal("init tracer: %w", err)
	}
	defer func() {
		// Дать батчам отправиться перед выходом.
		_ = shutdown(context.Background())
	}()

	// otelhttp.NewHandler автоматически:
	//   - создаёт SERVER span на каждый запрос,
	//   - извлекает traceparent из headers,
	//   - кладёт span в request.Context().
	mux := http.NewServeMux()
	mux.Handle("/orders", otelhttp.NewHandler(
		http.HandlerFunc(handleCreateOrder),
		"POST /orders",
	))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	log.Println("order-service listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Manual span для бизнес-операции, которую auto-instrumentation не видит.
	ctx, span := tracer.Start(ctx, "create-order")
	defer span.End()

	var req OrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid json")
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	span.SetAttributes(
		attribute.String("user_id", req.UserID),
		attribute.Float64("amount", req.Amount))

	orderID := fmt.Sprintf("ord-%d", time.Now().UnixNano())
	span.SetAttributes(attribute.String("order.id", orderID))

	// Business step 1: валидация
	if err := validateOrder(ctx, req); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "validation failed")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Business step 2: вызов payment-service.
	// Тот же ctx → trace context автоматически уедет в headers.
	if err := chargePayment(ctx, orderID, req.Amount); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "payment failed")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := OrderResponse{OrderID: orderID, Status: "confirmed"}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// validateOrder — отдельный span для бизнес-шага.
func validateOrder(ctx context.Context, req OrderRequest) error {
	_, span := tracer.Start(ctx, "validate_order")
	defer span.End()

	span.SetAttributes(attribute.Bool("order.validated", true))

	if req.UserID == "" {
		err := fmt.Errorf("user_id required")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	if req.Amount <= 0 {
		err := fmt.Errorf("amount must be positive")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	// Имитация бизнес-логики.
	time.Sleep(20 * time.Millisecond)
	return nil
}

// chargePayment — вызов внешнего сервиса.
// otelhttp.Transport создаст CLIENT span и прокинул traceparent.
func chargePayment(ctx context.Context, orderID string, amount float64) error {
	ctx, span := tracer.Start(ctx, "charge_payment")
	defer span.End()

	span.SetAttributes(
		attribute.String("payment.order_id", orderID),
		attribute.Float64("payment.amount", amount),
	)

	body := fmt.Sprintf(`{"order_id":%q,"amount":%f}`, orderID, amount)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		"http://localhost:8081/charge", io.NopCloser(
			// bytes.NewReader бы подошёл, но так короче:
			stringReader(body),
		),
	)
	req.Header.Set("Content-Type", "application/json")

	resp, err := paymentClient.Do(req)
	if err != nil {
		return fmt.Errorf("call payment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		err := fmt.Errorf("payment returned %d", resp.StatusCode)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

// stringReader — хелпер, чтобы не тащить bytes.NewReader.
func stringReader(s string) io.Reader {
	return &sr{s: s}
}

type sr struct {
	s string
	i int
}

func (r *sr) Read(p []byte) (int, error) {
	if r.i >= len(r.s) {
		return 0, io.EOF
	}
	n := copy(p, r.s[r.i:])
	r.i += n
	return n, nil
}
