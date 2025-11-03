# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a learning project for Uber Cadence, a distributed workflow orchestration engine. The codebase implements a simple "Hello World" workflow to demonstrate Cadence's workflow and activity patterns.

## Architecture

The application consists of a single `main.go` file with these key components:

1. **Cadence Client Setup** (`buildCadenceClient`): Creates a YARPC dispatcher to communicate with Cadence server via gRPC on port 7833, using a Thrift-to-Proto adapter for compatibility.

2. **Worker Initialization** (`startWorker`): Registers and starts a Cadence worker that listens on the "test-worker" task list in the "test-domain" domain. Workers execute both workflows and activities.

3. **Workflow Definition** (`helloWorldWorkflow`): Orchestrates the execution of activities with defined timeouts. Returns a string pointer.

4. **Activity Definition** (`helloWorldActivity`): Simple activity that takes a name and returns a greeting string.

## Development Commands

### Start Infrastructure
```bash
docker-compose up -d
```
This starts:
- PostgreSQL (port 5432) - Cadence's persistence layer
- Cadence server (multiple ports, gRPC on 7833)
- Cadence Web UI (port 8088)
- Prometheus (port 9090) - metrics collection
- Grafana (port 3000) - metrics visualization
- Node Exporter (port 9100) - system metrics

### Run the Worker
```bash
go run main.go
```
Starts the worker process and an HTTP API server on port 8080.

### Build
```bash
go build -o cadence-worker main.go
```

### Dependencies
```bash
go mod download
go mod tidy
```

## HTTP API Endpoints

The application exposes REST APIs on port 8080 for workflow management:

- **POST** `/api/workflows/start` - Start a new workflow execution
- **GET** `/api/workflows/list` - List all workflows (last 24 hours)
- **GET** `/api/workflows/status?workflowId={id}&runId={runId}` - Get workflow status and execution details
- **GET** `/api/workflows/history?workflowId={id}&runId={runId}` - Get complete workflow event history with activity status
- **GET** `/health` - Health check endpoint

See `API_ENDPOINTS.md` for detailed API documentation and examples.

## Important Configuration

- **HostPort**: `127.0.0.1:7833` - Cadence gRPC endpoint
- **Domain**: `test-domain` - Cadence domain for workflow isolation
- **TaskListName**: `test-worker` - Worker task queue identifier
- **CadenceService**: `cadence-frontend` - YARPC service name

## Cadence Concepts in This Codebase

- **Workflows** (main.go:100-120): Stateful, durable functions that coordinate activities. Must be deterministic.
- **Activities** (main.go:122-126): Individual units of work that can have side effects (API calls, DB operations, etc.)
- **Worker** (main.go:70-98): Process that polls task lists and executes registered workflows/activities
- **YARPC Dispatcher**: RPC framework used for client-server communication

## Web Interfaces

- Cadence Web UI: http://localhost:8088 - View workflows, history, and execution details
- Prometheus: http://localhost:9090 - Query metrics
- Grafana: http://localhost:3000 - Visualize metrics dashboards
