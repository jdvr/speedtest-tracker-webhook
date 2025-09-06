# Speedtest Tracker Webhook

A lightweight Go server that receives webhooks from [speedtest-tracker](https://github.com/alexjustesen/speedtest-tracker) and forwards the data to OpenTelemetry (OTEL) backends like New Relic.

## Overview

This service acts as a bridge between speedtest-tracker and observability platforms by:
- Receiving HTTP POST webhooks containing speedtest results
- Converting the data into OpenTelemetry metrics and traces
- Forwarding telemetry data to OTEL-compatible backends (New Relic, Jaeger, etc.)

## Features

- **OpenTelemetry Integration**: Full OTEL support with traces, metrics, and logs
- **Metrics Collection**: Tracks ping latency, download speed, and upload speed as histograms
- **Distributed Tracing**: Creates spans for each webhook request with detailed attributes
- **Graceful Shutdown**: Proper cleanup of resources and connections
- **Docker Support**: Multi-architecture container images (amd64/arm64)
- **Environment Configuration**: Flexible configuration via environment variables

## Metrics

The service collects the following metrics:

| Metric Name | Type | Description | Unit |
|-------------|------|-------------|------|
| `speedtest.ping` | Histogram | Ping latency measurements | ms |
| `speedtest.download` | Histogram | Download speed measurements | bps |
| `speedtest.upload` | Histogram | Upload speed measurements | bps |

All metrics include the following attributes:
- `server.id`: Speedtest server ID
- `server.name`: Speedtest server name
- `isp`: Internet Service Provider name

## Configuration

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `STW_SERVER_PORT` | Yes | `1214` | HTTP server port |
| `OTEL_SERVICE_NAME` | Yes | `speedtest-tracker-webhook` | Service name for telemetry |
| `OTEL_RESOURCE_ATTRIBUTES` | No | - | Additional resource attributes |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Yes | - | OTLP endpoint URL |
| `OTEL_EXPORTER_OTLP_HEADERS` | Yes | - | OTLP headers (e.g., API key) |
| `OTEL_ATTRIBUTE_VALUE_LENGTH_LIMIT` | No | `4095` | Max attribute value length |
| `OTEL_EXPORTER_OTLP_COMPRESSION` | No | `gzip` | Compression method |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | No | `http/protobuf` | OTLP protocol |
| `OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE` | No | `delta` | Metrics temporality |

### New Relic Configuration

For New Relic integration, use these settings:

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=https://otlp.nr-data.net
export OTEL_EXPORTER_OTLP_HEADERS=api-key=YOUR_NEW_RELIC_API_KEY
```

Replace `YOUR_NEW_RELIC_API_KEY` with your actual New Relic Ingest API key.

## Installation

### Using Docker (Recommended)

1. Pull the latest image:
```bash
docker pull ghcr.io/jdvr/speedtest-tracker-webhook:latest
```

2. Create environment file:
```bash
cp .env_dist .env
# Edit .env with your configuration
```

3. Run the container:
```bash
docker run -d \
  --name speedtest-webhook \
  --env-file .env \
  -p 1214:1214 \
  ghcr.io/jdvr/speedtest-tracker-webhook:latest
```

### Building from Source

1. Clone the repository:
```bash
git clone https://github.com/jdvr/speedtest-tracker-webhook.git
cd speedtest-tracker-webhook
```

2. Install dependencies:
```bash
go mod download
```

3. Build the application:
```bash
go build -o speedtest-tracker-webhook
```

4. Run with environment variables:
```bash
export STW_SERVER_PORT=1214
export OTEL_SERVICE_NAME=speedtest-tracker-webhook
export OTEL_EXPORTER_OTLP_ENDPOINT=https://otlp.nr-data.net
export OTEL_EXPORTER_OTLP_HEADERS=api-key=YOUR_API_KEY
./speedtest-tracker-webhook
```

## Usage

### Configuring Speedtest Tracker

In your speedtest-tracker configuration, set the webhook URL to:

```
http://your-server:1214/webhook
```

### Webhook Payload

The service expects JSON payloads with the following structure:

```json
{
  "result_id": 123,
  "site_name": "Home",
  "service": "speedtest",
  "serverName": "Test Server",
  "serverId": 456,
  "isp": "Example ISP",
  "ping": 25.5,
  "download": 100000000,
  "upload": 50000000,
  "packetLoss": 0.0,
  "speedtest_url": "https://speedtest.net/result/123456789",
  "url": "https://your-speedtest-tracker.com/admin/results/123"
}
```

### API Endpoints

- `POST /webhook` - Receives speedtest results and processes them

## Development

### Prerequisites

- Go 1.25 or later
- Docker (optional)

### Running Locally

1. Copy the environment template:
```bash
cp .env_dist .env
```

2. Edit `.env` with your configuration

3. Run the application:
```bash
go run .
```

### Building Docker Image

```bash
docker build -t speedtest-tracker-webhook .
```

## Dependencies

- **OpenTelemetry**: Complete observability stack
- **Logrus**: Structured logging
- **GoDotEnv**: Environment variable loading
- **HTTP instrumentation**: Automatic HTTP metrics and tracing

## License

This project is open source. Please check the repository for license details.

## Related Projects

- [speedtest-tracker](https://github.com/alexjustesen/speedtest-tracker) - The source of webhook data
- [OpenTelemetry Go](https://github.com/open-telemetry/opentelemetry-go) - OTEL SDK used
- [New Relic OpenTelemetry Examples](https://github.com/newrelic/newrelic-opentelemetry-examples) - Configuration reference
