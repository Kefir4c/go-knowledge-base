package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Kefir4c/go-knowledge-base/roadmap/07_microservices/7.7_distributed_tracing/tracing/telemetry"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("payment-service")

type ChargeRequest struct {
	OrderID string  `json:"order_id"`
	Amount  float64 `json:"amount"`
}

func main() {
	ctx := context.Background()

	shutdown, err := telemetry.InitTracer(ctx, "payment-service")
	if err != nil {
		log.Fatalf("init tracer: %w", err)
	}
	defer func() { _ = shutdown(context.Background()) }()

	mux := http.NewServeMux()
	mux.Handle("/charge", otelhttp.NewHandler(
		http.HandlerFunc(handleCharge),
		"POST /charge",
	))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	log.Println("payment-service listening on :8081")
	log.Fatal(http.ListenAndServe(":8081", mux))
}

func handleCharge(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	_, span := tracer.Start(ctx, "process_payment")
	defer span.End()

	var req ChargeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid json")
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	span.SetAttributes(
		attribute.String("payment.order_id", req.OrderID),
		attribute.Float64("payment.amount", req.Amount),
	)

	// Шаг 1: "проверка баланса" — отдельный span.
	if err := checkBalance(ctx, req.Amount); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, err.Error(), http.StatusPaymentRequired)
		return
	}

	// Шаг 2: "обращение к банку" — имитация долгого вызова.
	if err := callBank(ctx, req.Amount); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":   "charged",
		"order_id": req.OrderID,
	})
}

func checkBalance(ctx context.Context, amount float64) error {
	_, span := tracer.Start(ctx, "check_balance")
	defer span.End()

	span.SetAttributes(attribute.Float64("balance.check.amount", amount))
	time.Sleep(15 * time.Second)
	return nil
}

// callBank — долгий вызов, чтобы в waterfall было видно узкое место.
func callBank(ctx context.Context, amount float64) error {
	_, span := tracer.Start(ctx, "call_bank_api")
	defer span.End()

	span.SetAttributes(
		attribute.String("bank.provider", "demo-bank"),
		attribute.Float64("bank.amount", amount),
	)

	// Имитация latency банка.
	time.Sleep(300 * time.Millisecond)

	// событие внутри span'а
	span.AddEvent("bank_confirmed",
		trace.WithAttributes(
			attribute.String("bank.txn_id", fmt.Sprintf("txn-%d", time.Now().UnixNano())),
		),
	)
	return nil
}
