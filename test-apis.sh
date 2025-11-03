#!/bin/bash

# Test script for Cadence Workflow APIs
# Make sure the worker is running before executing this script (go run main.go)

BASE_URL="http://localhost:8080"

echo "=== Testing Cadence Workflow APIs ==="
echo ""

# 1. Health Check
echo "1. Health Check"
echo "GET $BASE_URL/health"
curl -s $BASE_URL/health | jq .
echo ""
echo ""

# 2. Start a workflow
echo "2. Starting a new workflow"
echo "POST $BASE_URL/api/workflows/start"
START_RESPONSE=$(curl -s -X POST $BASE_URL/api/workflows/start \
  -H "Content-Type: application/json" \
  -d '{"name": "TestUser"}')

echo $START_RESPONSE | jq .
WORKFLOW_ID=$(echo $START_RESPONSE | jq -r '.workflowId')
RUN_ID=$(echo $START_RESPONSE | jq -r '.runId')
echo ""
echo "Workflow ID: $WORKFLOW_ID"
echo "Run ID: $RUN_ID"
echo ""

# Wait a bit for the workflow to execute
echo "Waiting 3 seconds for workflow to complete..."
sleep 3
echo ""

# 3. Get workflow status
echo "3. Getting workflow status"
echo "GET $BASE_URL/api/workflows/status?workflowId=$WORKFLOW_ID"
curl -s "$BASE_URL/api/workflows/status?workflowId=$WORKFLOW_ID" | jq .
echo ""
echo ""

# 4. Get workflow history
echo "4. Getting workflow history"
echo "GET $BASE_URL/api/workflows/history?workflowId=$WORKFLOW_ID"
curl -s "$BASE_URL/api/workflows/history?workflowId=$WORKFLOW_ID" | jq .
echo ""
echo ""

# 5. List all workflows
echo "5. Listing all workflows"
echo "GET $BASE_URL/api/workflows/list"
curl -s $BASE_URL/api/workflows/list | jq .
echo ""
echo ""

echo "=== Tests Complete ==="
