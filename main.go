package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/cadence/.gen/go/cadence/workflowserviceclient"
	"go.uber.org/cadence/.gen/go/shared"
	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/client"
	"go.uber.org/cadence/compatibility"
	"go.uber.org/cadence/worker"
	"go.uber.org/cadence/workflow"

	"github.com/uber-go/tally"
	apiv1 "github.com/uber/cadence-idl/go/proto/api/v1"
	"go.uber.org/yarpc"
	"go.uber.org/yarpc/transport/grpc"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var HostPort = "127.0.0.1:7833"
var Domain = "test-domain"
var TaskListName = "test-worker"
var ClientName = "test-worker"
var CadenceService = "cadence-frontend"

var (
	cadenceClient client.Client
	logger        *zap.Logger
)

func main() {
	logger = buildLogger()
	service := buildCadenceClient()

	// Initialize Cadence client for API operations
	cadenceClient = client.NewClient(service, Domain, &client.Options{})

	// Start worker
	startWorker(logger, service)

	// Setup HTTP handlers
	setupHTTPHandlers()

	logger.Info("Starting HTTP server on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		panic(err)
	}
}

func buildLogger() *zap.Logger {
	config := zap.NewDevelopmentConfig()
	config.Level.SetLevel(zapcore.InfoLevel)

	var err error
	logger, err := config.Build()
	if err != nil {
		panic("Failed to setup logger")
	}

	return logger
}

func buildCadenceClient() workflowserviceclient.Interface {
	dispatcher := yarpc.NewDispatcher(yarpc.Config{
		Name: ClientName,
		Outbounds: yarpc.Outbounds{
			CadenceService: {Unary: grpc.NewTransport().NewSingleOutbound(HostPort)},
		},
	})
	if err := dispatcher.Start(); err != nil {
		panic("Failed to start dispatcher")
	}

	clientConfig := dispatcher.ClientConfig(CadenceService)

	return compatibility.NewThrift2ProtoAdapter(
		apiv1.NewDomainAPIYARPCClient(clientConfig),
		apiv1.NewWorkflowAPIYARPCClient(clientConfig),
		apiv1.NewWorkerAPIYARPCClient(clientConfig),
		apiv1.NewVisibilityAPIYARPCClient(clientConfig),
	)
}

func startWorker(logger *zap.Logger, service workflowserviceclient.Interface) {

	// TaskListName identifies set of client workflows, activities, and workers.
	// It could be your group or client or application name.
	workerOptions := worker.Options{
		Logger:       logger,
		MetricsScope: tally.NewTestScope(TaskListName, map[string]string{}),
	}

	worker := worker.New(
		service,
		Domain,
		TaskListName,
		workerOptions)
	// if err != nil {
	// 	panic("Failed to initialize worker")
	// }

	// add the following lines to the function startWorker before calling worker.Start()
	worker.RegisterWorkflow(helloWorldWorkflow)
	worker.RegisterActivity(helloWorldActivity)

	err := worker.Start()
	if err != nil {
		panic("Failed to start worker")
	}

	logger.Info("Started Worker.", zap.String("worker", TaskListName))
}

func helloWorldWorkflow(ctx workflow.Context, name string) (*string, error) {
	ao := workflow.ActivityOptions{
		ScheduleToStartTimeout: time.Minute,
		StartToCloseTimeout:    time.Minute,
		HeartbeatTimeout:       time.Second * 20,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	logger := workflow.GetLogger(ctx)
	logger.Info("helloworld workflow started")
	var helloworldResult string
	err := workflow.ExecuteActivity(ctx, helloWorldActivity, name).Get(ctx, &helloworldResult)
	if err != nil {
		logger.Error("Activity failed.", zap.Error(err))
		return nil, err
	}

	logger.Info("Workflow completed.", zap.String("Result", helloworldResult))

	return &helloworldResult, nil
}

func helloWorldActivity(ctx context.Context, name string) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("helloworld activity started")
	return "Hello " + name + "!", nil
}

// setupHTTPHandlers configures all HTTP endpoints
func setupHTTPHandlers() {
	http.HandleFunc("/api/workflows/start", startWorkflowHandler)
	http.HandleFunc("/api/workflows/list", listWorkflowsHandler)
	http.HandleFunc("/api/workflows/status", getWorkflowStatusHandler)
	http.HandleFunc("/api/workflows/history", getWorkflowHistoryHandler)
	http.HandleFunc("/health", healthCheckHandler)
}

// healthCheckHandler returns service health status
func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

// startWorkflowHandler starts a new workflow execution
func startWorkflowHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		req.Name = "World"
	}

	workflowOptions := client.StartWorkflowOptions{
		ID:                              "hello-world-" + time.Now().Format("20060102-150405"),
		TaskList:                        TaskListName,
		ExecutionStartToCloseTimeout:    time.Minute * 10,
		DecisionTaskStartToCloseTimeout: time.Minute,
	}

	ctx := context.Background()
	we, err := cadenceClient.StartWorkflow(ctx, workflowOptions, helloWorldWorkflow, req.Name)
	if err != nil {
		logger.Error("Failed to start workflow", zap.Error(err))
		http.Error(w, "Failed to start workflow: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"workflowId": we.ID,
		"runId":      we.RunID,
		"message":    "Workflow started successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// listWorkflowsHandler lists all workflow executions
func listWorkflowsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := context.Background()

	pageSize := int32(100)
	var nextPageToken []byte

	// Calculate timestamps for last 24 hours (in nanoseconds since epoch)
	earliestTime := time.Now().Add(-24 * time.Hour).UnixNano()
	latestTime := time.Now().UnixNano()

	// List open workflows
	openResp, err := cadenceClient.ListOpenWorkflow(ctx, &shared.ListOpenWorkflowExecutionsRequest{
		Domain:          &Domain,
		MaximumPageSize: &pageSize,
		NextPageToken:   nextPageToken,
		StartTimeFilter: &shared.StartTimeFilter{
			EarliestTime: &earliestTime,
			LatestTime:   &latestTime,
		},
	})
	if err != nil {
		logger.Error("Failed to list open workflows", zap.Error(err))
		http.Error(w, "Failed to list open workflows: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// List closed workflows
	closedResp, err := cadenceClient.ListClosedWorkflow(ctx, &shared.ListClosedWorkflowExecutionsRequest{
		Domain:          &Domain,
		MaximumPageSize: &pageSize,
		NextPageToken:   nextPageToken,
		StartTimeFilter: &shared.StartTimeFilter{
			EarliestTime: &earliestTime,
			LatestTime:   &latestTime,
		},
	})
	if err != nil {
		logger.Error("Failed to list closed workflows", zap.Error(err))
		http.Error(w, "Failed to list closed workflows: "+err.Error(), http.StatusInternalServerError)
		return
	}

	type WorkflowInfo struct {
		WorkflowID    string     `json:"workflowId"`
		RunID         string     `json:"runId"`
		WorkflowType  string     `json:"workflowType"`
		StartTime     time.Time  `json:"startTime"`
		CloseTime     *time.Time `json:"closeTime,omitempty"`
		Status        string     `json:"status"`
		ExecutionTime string     `json:"executionTime"`
	}

	var workflows []WorkflowInfo

	// Add open workflows
	for _, exec := range openResp.Executions {
		startTime := time.Unix(0, *exec.StartTime)
		executionTime := time.Since(startTime)

		workflowType := ""
		if exec.Type != nil && exec.Type.Name != nil {
			workflowType = *exec.Type.Name
		}

		workflows = append(workflows, WorkflowInfo{
			WorkflowID:    *exec.Execution.WorkflowId,
			RunID:         *exec.Execution.RunId,
			WorkflowType:  workflowType,
			StartTime:     startTime,
			Status:        "RUNNING",
			ExecutionTime: executionTime.String(),
		})
	}

	// Add closed workflows
	for _, exec := range closedResp.Executions {
		startTime := time.Unix(0, *exec.StartTime)
		var closeTime *time.Time
		var executionTime time.Duration

		if exec.CloseTime != nil {
			ct := time.Unix(0, *exec.CloseTime)
			closeTime = &ct
			executionTime = ct.Sub(startTime)
		}

		status := "COMPLETED"
		if exec.CloseStatus != nil {
			status = exec.CloseStatus.String()
		}

		workflowType := ""
		if exec.Type != nil && exec.Type.Name != nil {
			workflowType = *exec.Type.Name
		}

		workflows = append(workflows, WorkflowInfo{
			WorkflowID:    *exec.Execution.WorkflowId,
			RunID:         *exec.Execution.RunId,
			WorkflowType:  workflowType,
			StartTime:     startTime,
			CloseTime:     closeTime,
			Status:        status,
			ExecutionTime: executionTime.String(),
		})
	}

	response := map[string]interface{}{
		"workflows": workflows,
		"count":     len(workflows),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// getWorkflowStatusHandler gets the status of a specific workflow
func getWorkflowStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	workflowID := r.URL.Query().Get("workflowId")
	runID := r.URL.Query().Get("runId")

	if workflowID == "" {
		http.Error(w, "workflowId parameter is required", http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	// Describe workflow execution
	descResp, err := cadenceClient.DescribeWorkflowExecution(ctx, workflowID, runID)
	if err != nil {
		logger.Error("Failed to describe workflow", zap.Error(err))
		http.Error(w, "Failed to get workflow status: "+err.Error(), http.StatusInternalServerError)
		return
	}

	executionInfo := descResp.WorkflowExecutionInfo
	startTime := time.Unix(0, *executionInfo.StartTime)

	var closeTime *time.Time
	var executionTime time.Duration

	if executionInfo.CloseTime != nil {
		ct := time.Unix(0, *executionInfo.CloseTime)
		closeTime = &ct
		executionTime = ct.Sub(startTime)
	} else {
		executionTime = time.Since(startTime)
	}

	status := "RUNNING"
	if executionInfo.CloseStatus != nil {
		status = executionInfo.CloseStatus.String()
	}

	workflowType := ""
	if executionInfo.Type != nil && executionInfo.Type.Name != nil {
		workflowType = *executionInfo.Type.Name
	}

	response := map[string]interface{}{
		"workflowId":    *executionInfo.Execution.WorkflowId,
		"runId":         *executionInfo.Execution.RunId,
		"workflowType":  workflowType,
		"startTime":     startTime,
		"closeTime":     closeTime,
		"status":        status,
		"executionTime": executionTime.String(),
		"historyLength": executionInfo.HistoryLength,
	}

	// Add pending activities if any
	if len(descResp.PendingActivities) > 0 {
		var activities []map[string]interface{}
		for _, activity := range descResp.PendingActivities {
			activityType := ""
			if activity.ActivityType != nil && activity.ActivityType.Name != nil {
				activityType = *activity.ActivityType.Name
			}

			activityInfo := map[string]interface{}{
				"activityId":   *activity.ActivityID,
				"activityType": activityType,
				"state":        activity.State.String(),
				"attempt":      *activity.Attempt,
			}

			if activity.ScheduledTimestamp != nil {
				scheduledTime := time.Unix(0, *activity.ScheduledTimestamp)
				activityInfo["scheduledTime"] = scheduledTime
			}

			if activity.LastStartedTimestamp != nil {
				lastStartedTime := time.Unix(0, *activity.LastStartedTimestamp)
				activityInfo["lastStartedTime"] = lastStartedTime
			}

			activities = append(activities, activityInfo)
		}
		response["pendingActivities"] = activities
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// getWorkflowHistoryHandler gets the complete history of a workflow including activity status
func getWorkflowHistoryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	workflowID := r.URL.Query().Get("workflowId")
	runID := r.URL.Query().Get("runId")

	if workflowID == "" {
		http.Error(w, "workflowId parameter is required", http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	// Get workflow history
	iter := cadenceClient.GetWorkflowHistory(ctx, workflowID, runID, false, shared.HistoryEventFilterTypeAllEvent)

	type HistoryEvent struct {
		EventID   int64     `json:"eventId"`
		EventType string    `json:"eventType"`
		Timestamp time.Time `json:"timestamp"`
		Details   string    `json:"details"`
	}

	var events []HistoryEvent

	for iter.HasNext() {
		event, err := iter.Next()
		if err != nil {
			logger.Error("Failed to get history event", zap.Error(err))
			break
		}

		historyEvent := HistoryEvent{
			EventID:   *event.EventId,
			EventType: event.EventType.String(),
			Timestamp: time.Unix(0, *event.Timestamp),
		}

		// Extract relevant details based on event type
		switch event.EventType.String() {
		case "WorkflowExecutionStarted":
			if event.WorkflowExecutionStartedEventAttributes != nil {
				historyEvent.Details = "Workflow started with input"
			}
		case "ActivityTaskScheduled":
			if event.ActivityTaskScheduledEventAttributes != nil &&
				event.ActivityTaskScheduledEventAttributes.ActivityType != nil &&
				event.ActivityTaskScheduledEventAttributes.ActivityType.Name != nil {
				historyEvent.Details = "Activity: " + *event.ActivityTaskScheduledEventAttributes.ActivityType.Name
			}
		case "ActivityTaskStarted":
			historyEvent.Details = "Activity execution started"
		case "ActivityTaskCompleted":
			if event.ActivityTaskCompletedEventAttributes != nil {
				historyEvent.Details = "Activity completed successfully"
			}
		case "ActivityTaskFailed":
			if event.ActivityTaskFailedEventAttributes != nil && event.ActivityTaskFailedEventAttributes.Reason != nil {
				historyEvent.Details = "Activity failed: " + *event.ActivityTaskFailedEventAttributes.Reason
			}
		case "WorkflowExecutionCompleted":
			historyEvent.Details = "Workflow completed successfully"
		case "WorkflowExecutionFailed":
			if event.WorkflowExecutionFailedEventAttributes != nil && event.WorkflowExecutionFailedEventAttributes.Reason != nil {
				historyEvent.Details = "Workflow failed: " + *event.WorkflowExecutionFailedEventAttributes.Reason
			}
		}

		events = append(events, historyEvent)
	}

	response := map[string]interface{}{
		"workflowId": workflowID,
		"runId":      runID,
		"events":     events,
		"count":      len(events),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
