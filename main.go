package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// --- Structs for Configuration and Payload ---

// Config defines the application configuration structure parsed from YAML.
type Config struct {
	Server struct {
		Port int `yaml:"port"`
	} `yaml:"server"`
	Otel struct {
		ServiceName string `yaml:"serviceName"`
		Otlp        struct {
			Endpoint string `yaml:"endpoint"`
			ApiKey   string `yaml:"apiKey"`
		} `yaml:"otlp"`
	} `yaml:"otel"`
}

// WebhookPayload defines the structure of the incoming JSON from the speedtest service.
type WebhookPayload struct {
	ResultID     int     `json:"result_id"`
	SiteName     string  `json:"site_name"`
	Service      string  `json:"service"`
	ServerName   string  `json:"serverName"`
	ServerID     int     `json:"serverId"`
	ISP          string  `json:"isp"`
	Ping         float64 `json:"ping"`
	Download     float64 `json:"download"`
	Upload       float64 `json:"upload"`
	PacketLoss   float64 `json:"packetLoss"`
	SpeedtestURL string  `json:"speedtest_url"`
	URL          string  `json:"url"`
}

// --- Global OTel Variables ---

var (
	tracer            trace.Tracer
	meter             metric.Meter
	pingHistogram     metric.Float64Histogram
	downloadHistogram metric.Float64Histogram
	uploadHistogram   metric.Float64Histogram
)

// --- OTel Initialization ---

// apiKeyCredentials implements credentials.PerRPCCredentials for adding the New Relic API key.
type apiKeyCredentials struct {
	apiKey string
}

// GetRequestMetadata gets the current request metadata, adding the api-key header.
func (a apiKeyCredentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"api-key": a.apiKey,
	}, nil
}

// RequireTransportSecurity indicates that a secure connection is required.
func (a apiKeyCredentials) RequireTransportSecurity() bool {
	return true
}

func main() {
	err := godotenv.Load()
	if err != nil {
		// We log a warning instead of a fatal error because in a production environment,
		// variables are often set directly, not from a file.
		log.Errorf("Warning: Could not load .env file: %v", err)
	}
	if err := run(); err != nil {
		log.Fatalln(err)
	}
}

func run() error {
	// Handle SIGINT (CTRL+C) gracefully.
	ctx, ctxCan := signal.NotifyContext(context.Background(), os.Interrupt)
	defer ctxCan()

	// Set up OpenTelemetry.
	otelShutdown, err := setupOTelSDK(ctx)
	if err != nil {
		return err
	}
	// Handle shutdown properly so nothing leaks.
	defer func() {
		err = errors.Join(err, otelShutdown(context.Background()))
	}()

	tracer = otel.Tracer("speedtest-webhook/tracer")
	meter = otel.Meter("speedtest-webhook/meter")

	pingHistogram, err = meter.Float64Histogram("speedtest.ping", metric.WithDescription("Ping latency"), metric.WithUnit("ms"))
	if err != nil {
		log.Fatalf("Failed to create ping histogram: %v", err)
	}
	downloadHistogram, err = meter.Float64Histogram("speedtest.download", metric.WithDescription("Download speed"), metric.WithUnit("bps"))
	if err != nil {
		log.Fatalf("Failed to create download histogram: %v", err)
	}
	uploadHistogram, err = meter.Float64Histogram("speedtest.upload", metric.WithDescription("Upload speed"), metric.WithUnit("bps"))
	if err != nil {
		log.Fatalf("Failed to create upload histogram: %v", err)
	}

	portRaw := os.Getenv("STW_SERVER_PORT")
	if portRaw == "" {
		return fmt.Errorf("missing env var STW_SERVER_PORT")
	}

	port, err := strconv.Atoi(portRaw)
	if err != nil {
		return fmt.Errorf("inavlid value for env var STW_SERVER_PORT %s", portRaw)
	}

	mux := http.NewServeMux()
	otelWebhook := otelhttp.WithRouteTag("/webhook", http.HandlerFunc(webhookHandler))
	mux.Handle("/webhook", otelWebhook)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: otelhttp.NewHandler(mux, "/"),
	}

	// --- Graceful Shutdown ---
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Infof("Server starting on port %d", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Could not listen on port %d: %v\n", port, err)
		}
	}()

	<-stop

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return err
	}

	log.Info("Server gracefully stopped.")

	return nil
}

// webhookHandler processes incoming POST requests.
func webhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, span := tracer.Start(r.Context(), "handleWebhookRequest")
	defer span.End()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		span.RecordError(err)
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}

	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		span.RecordError(err)
		http.Error(w, "Error parsing JSON payload", http.StatusBadRequest)
		return
	}

	log.Printf("Received speedtest result for server ID: %d", payload.ServerID)

	metricOpts := metric.WithAttributes(
		attribute.String("server.id", strconv.Itoa(payload.ServerID)),
		attribute.String("server.name", payload.ServerName),
		attribute.String("isp", payload.ISP),
	)
	pingHistogram.Record(ctx, payload.Ping, metricOpts)
	downloadHistogram.Record(ctx, payload.Download, metricOpts)
	uploadHistogram.Record(ctx, payload.Upload, metricOpts)

	span.AddEvent("speedtest.result", trace.WithAttributes(
		attribute.Int("result_id", payload.ResultID),
		attribute.String("site_name", payload.SiteName),
		attribute.String("service", payload.Service),
		attribute.String("server.name", payload.ServerName),
		attribute.Int("server.id", payload.ServerID),
		attribute.String("isp", payload.ISP),
		attribute.Float64("ping", payload.Ping),
		attribute.Float64("download.bps", payload.Download),
		attribute.Float64("upload.bps", payload.Upload),
		attribute.Float64("packet.loss", payload.PacketLoss),
		attribute.String("speedtest.url", payload.SpeedtestURL),
	))

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Webhook received and processed.")
}
