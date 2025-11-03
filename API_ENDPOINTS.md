# Cadence Workflow API Endpoints

This document describes the HTTP API endpoints for interacting with Cadence workflows.

## Base URL
All endpoints are served on `http://localhost:8080`

## Endpoints

### 1. Health Check
**GET** `/health`

Check if the service is running.

**Response:**
```json
{
  "status": "healthy"
}
```

**Example:**
```bash
curl http://localhost:8080/health
```

---

### 2. Start Workflow
**POST** `/api/workflows/start`

Start a new workflow execution.

**Request Body:**
```json
{
  "name": "John"
}
```

**Response:**
```json
{
  "workflowId": "hello-world-20241103-120000",
  "runId": "abc123-def456-ghi789",
  "message": "Workflow started successfully"
}
```

**Example:**
```bash
curl -X POST http://localhost:8080/api/workflows/start \
  -H "Content-Type: application/json" \
  -d '{"name": "Alice"}'
```

---

### 3. List Workflows
**GET** `/api/workflows/list`

List all workflow executions from the last 24 hours (both open and closed).

**Response:**
```json
{
  "workflows": [
    {
      "workflowId": "hello-world-20241103-120000",
      "runId": "abc123-def456-ghi789",
      "workflowType": "helloWorldWorkflow",
      "startTime": "2024-11-03T12:00:00Z",
      "closeTime": "2024-11-03T12:00:05Z",
      "status": "COMPLETED",
      "executionTime": "5s"
    },
    {
      "workflowId": "hello-world-20241103-110000",
      "runId": "xyz789-abc123-def456",
      "workflowType": "helloWorldWorkflow",
      "startTime": "2024-11-03T11:00:00Z",
      "status": "RUNNING",
      "executionTime": "1h0m0s"
    }
  ],
  "count": 2
}
```

**Example:**
```bash
curl http://localhost:8080/api/workflows/list
```

---

### 4. Get Workflow Status
**GET** `/api/workflows/status?workflowId={workflowId}&runId={runId}`

Get detailed status information about a specific workflow execution.

**Query Parameters:**
- `workflowId` (required): The workflow execution ID
- `runId` (optional): The specific run ID. If not provided, returns the latest run.

**Response:**
```json
{
  "workflowId": "hello-world-20241103-120000",
  "runId": "abc123-def456-ghi789",
  "workflowType": "helloWorldWorkflow",
  "startTime": "2024-11-03T12:00:00Z",
  "closeTime": "2024-11-03T12:00:05Z",
  "status": "COMPLETED",
  "executionTime": "5s",
  "historyLength": 15,
  "pendingActivities": [
    {
      "activityId": "1",
      "activityType": "helloWorldActivity",
      "state": "STARTED",
      "attempt": 1,
      "scheduledTime": "2024-11-03T12:00:01Z",
      "lastStartedTime": "2024-11-03T12:00:02Z"
    }
  ]
}
```

**Note:** `pendingActivities` field only appears if there are activities currently pending.

**Example:**
```bash
curl "http://localhost:8080/api/workflows/status?workflowId=hello-world-20241103-120000"
```

---

### 5. Get Workflow History
**GET** `/api/workflows/history?workflowId={workflowId}&runId={runId}`

Get the complete event history of a workflow execution, including all activity events.

**Query Parameters:**
- `workflowId` (required): The workflow execution ID
- `runId` (optional): The specific run ID. If not provided, returns history for the latest run.

**Response:**
```json
{
  "workflowId": "hello-world-20241103-120000",
  "runId": "abc123-def456-ghi789",
  "events": [
    {
      "eventId": 1,
      "eventType": "WorkflowExecutionStarted",
      "timestamp": "2024-11-03T12:00:00Z",
      "details": "Workflow started with input"
    },
    {
      "eventId": 2,
      "eventType": "DecisionTaskScheduled",
      "timestamp": "2024-11-03T12:00:00Z",
      "details": ""
    },
    {
      "eventId": 3,
      "eventType": "ActivityTaskScheduled",
      "timestamp": "2024-11-03T12:00:01Z",
      "details": "Activity: helloWorldActivity"
    },
    {
      "eventId": 4,
      "eventType": "ActivityTaskStarted",
      "timestamp": "2024-11-03T12:00:02Z",
      "details": "Activity execution started"
    },
    {
      "eventId": 5,
      "eventType": "ActivityTaskCompleted",
      "timestamp": "2024-11-03T12:00:03Z",
      "details": "Activity completed successfully"
    },
    {
      "eventId": 6,
      "eventType": "WorkflowExecutionCompleted",
      "timestamp": "2024-11-03T12:00:05Z",
      "details": "Workflow completed successfully"
    }
  ],
  "count": 6
}
```

**Example:**
```bash
curl "http://localhost:8080/api/workflows/history?workflowId=hello-world-20241103-120000"
```

---

## Workflow Event Types

The history endpoint may return the following event types:

- **WorkflowExecutionStarted**: Workflow execution has begun
- **DecisionTaskScheduled**: A decision task has been scheduled
- **DecisionTaskStarted**: A decision task has started
- **DecisionTaskCompleted**: A decision task has completed
- **ActivityTaskScheduled**: An activity has been scheduled for execution
- **ActivityTaskStarted**: An activity execution has started
- **ActivityTaskCompleted**: An activity has completed successfully
- **ActivityTaskFailed**: An activity has failed
- **ActivityTaskTimedOut**: An activity has timed out
- **WorkflowExecutionCompleted**: Workflow completed successfully
- **WorkflowExecutionFailed**: Workflow execution failed
- **WorkflowExecutionTimedOut**: Workflow execution timed out
- **WorkflowExecutionCanceled**: Workflow execution was canceled

## Testing the APIs

### Prerequisites
1. Start infrastructure (PostgreSQL + Cadence):
```bash
docker-compose up -d
```

2. Wait for Cadence to be ready (check logs):
```bash
docker-compose logs -f cadence
```

Note: PostgreSQL will initialize automatically. First startup may take a bit longer as Cadence sets up the database schema.

3. Start the worker:
```bash
go run main.go
```

### Test Workflow

1. **Start a workflow:**
```bash
curl -X POST http://localhost:8080/api/workflows/start \
  -H "Content-Type: application/json" \
  -d '{"name": "TestUser"}'
```

Save the `workflowId` from the response.

2. **Check workflow status:**
```bash
curl "http://localhost:8080/api/workflows/status?workflowId=YOUR_WORKFLOW_ID"
```

3. **View workflow history:**
```bash
curl "http://localhost:8080/api/workflows/history?workflowId=YOUR_WORKFLOW_ID"
```

4. **List all workflows:**
```bash
curl http://localhost:8080/api/workflows/list
```

## Status Values

Workflows can have the following statuses:

- **RUNNING**: Workflow is currently executing
- **COMPLETED**: Workflow completed successfully
- **FAILED**: Workflow execution failed
- **CANCELED**: Workflow was canceled
- **TERMINATED**: Workflow was terminated
- **CONTINUED_AS_NEW**: Workflow continued as a new execution
- **TIMED_OUT**: Workflow execution timed out

## Activity States

Activities in `pendingActivities` can have the following states:

- **SCHEDULED**: Activity has been scheduled but not started
- **STARTED**: Activity execution has begun
- **CANCEL_REQUESTED**: Cancellation has been requested for the activity

## Error Responses

All endpoints may return the following error responses:

**400 Bad Request:**
```json
Invalid request body
```

**404 Not Found:**
```
Failed to get workflow status: EntityNotExistsError...
```

**405 Method Not Allowed:**
```
Method not allowed
```

**500 Internal Server Error:**
```
Failed to start workflow: ...
```
