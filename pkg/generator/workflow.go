package generator

import (
	"fmt"
	"math/rand"
)

// WorkflowStep represents a single step in a workflow
type WorkflowStep struct {
	Service     string // Service name
	Operation   string // Operation name
	SpanKind    string // "server", "client", "internal"
	DurationMs  int    // Base duration in ms
	CanParallel bool   // Can this step have parallel children?
}

// Workflow defines a business workflow with service call chain
type Workflow struct {
	Name        string
	Description string
	Steps       []WorkflowStep
}

// WorkflowContext holds context that flows through a workflow
type WorkflowContext struct {
	WorkflowName  string
	UserID        string
	OrderID       string
	ProductID     string
	SessionID     string
	CartID        string
	PaymentID     string
	ShipmentID    string
	RequestID     string
	CorrelationID string
}

// Define available workflows
var workflows = map[string]Workflow{
	"place_order": {
		Name:        "place_order",
		Description: "User places an order",
		Steps: []WorkflowStep{
			{Service: "frontend", Operation: "POST /api/orders", SpanKind: "server", DurationMs: 100, CanParallel: true},
			{Service: "auth", Operation: "ValidateToken", SpanKind: "server", DurationMs: 20, CanParallel: false},
			{Service: "backend", Operation: "ProcessOrder", SpanKind: "server", DurationMs: 150, CanParallel: true},
			{Service: "cache", Operation: "GET", SpanKind: "client", DurationMs: 5, CanParallel: false}, // Check inventory cache
			{Service: "database", Operation: "SELECT products", SpanKind: "client", DurationMs: 30, CanParallel: false},
			{Service: "payment", Operation: "ProcessPayment", SpanKind: "client", DurationMs: 200, CanParallel: false},
			{Service: "database", Operation: "INSERT orders", SpanKind: "client", DurationMs: 40, CanParallel: false},
			{Service: "shipping", Operation: "CreateShipment", SpanKind: "client", DurationMs: 80, CanParallel: false},
			{Service: "notification", Operation: "SendEmail", SpanKind: "client", DurationMs: 50, CanParallel: false},
		},
	},
	"user_login": {
		Name:        "user_login",
		Description: "User login flow",
		Steps: []WorkflowStep{
			{Service: "frontend", Operation: "POST /api/auth/login", SpanKind: "server", DurationMs: 80, CanParallel: true},
			{Service: "auth", Operation: "Authenticate", SpanKind: "server", DurationMs: 100, CanParallel: true},
			{Service: "cache", Operation: "GET", SpanKind: "client", DurationMs: 5, CanParallel: false}, // Check session cache
			{Service: "database", Operation: "SELECT users", SpanKind: "client", DurationMs: 25, CanParallel: false},
			{Service: "analytics", Operation: "TrackEvent", SpanKind: "client", DurationMs: 20, CanParallel: false},
		},
	},
	"browse_products": {
		Name:        "browse_products",
		Description: "User browses product catalog",
		Steps: []WorkflowStep{
			{Service: "frontend", Operation: "GET /api/products", SpanKind: "server", DurationMs: 60, CanParallel: true},
			{Service: "cache", Operation: "GET", SpanKind: "client", DurationMs: 3, CanParallel: false}, // Try cache first
			{Service: "database", Operation: "SELECT products", SpanKind: "client", DurationMs: 50, CanParallel: false},
			{Service: "analytics", Operation: "TrackEvent", SpanKind: "client", DurationMs: 15, CanParallel: false},
		},
	},
	"search_products": {
		Name:        "search_products",
		Description: "User searches for products",
		Steps: []WorkflowStep{
			{Service: "frontend", Operation: "GET /api/products/search", SpanKind: "server", DurationMs: 70, CanParallel: true},
			{Service: "backend", Operation: "SearchProducts", SpanKind: "server", DurationMs: 120, CanParallel: true},
			{Service: "database", Operation: "SELECT products", SpanKind: "client", DurationMs: 80, CanParallel: false},
			{Service: "cache", Operation: "SET", SpanKind: "client", DurationMs: 5, CanParallel: false}, // Cache results
			{Service: "analytics", Operation: "TrackEvent", SpanKind: "client", DurationMs: 15, CanParallel: false},
		},
	},
	"view_dashboard": {
		Name:        "view_dashboard",
		Description: "User views dashboard",
		Steps: []WorkflowStep{
			{Service: "frontend", Operation: "GET /dashboard", SpanKind: "server", DurationMs: 90, CanParallel: true},
			{Service: "auth", Operation: "ValidateToken", SpanKind: "client", DurationMs: 15, CanParallel: false},
			{Service: "backend", Operation: "GetDashboardData", SpanKind: "server", DurationMs: 100, CanParallel: true},
			{Service: "database", Operation: "SELECT", SpanKind: "client", DurationMs: 40, CanParallel: false},
			{Service: "analytics", Operation: "QueryData", SpanKind: "client", DurationMs: 30, CanParallel: false},
		},
	},
	"process_refund": {
		Name:        "process_refund",
		Description: "Process a refund",
		Steps: []WorkflowStep{
			{Service: "frontend", Operation: "POST /api/refunds", SpanKind: "server", DurationMs: 100, CanParallel: true},
			{Service: "auth", Operation: "Authorize", SpanKind: "client", DurationMs: 20, CanParallel: false},
			{Service: "backend", Operation: "ProcessRefund", SpanKind: "server", DurationMs: 150, CanParallel: true},
			{Service: "database", Operation: "SELECT orders", SpanKind: "client", DurationMs: 30, CanParallel: false},
			{Service: "payment", Operation: "Refund", SpanKind: "client", DurationMs: 180, CanParallel: false},
			{Service: "database", Operation: "UPDATE orders", SpanKind: "client", DurationMs: 35, CanParallel: false},
			{Service: "notification", Operation: "SendEmail", SpanKind: "client", DurationMs: 45, CanParallel: false},
		},
	},
	"user_registration": {
		Name:        "user_registration",
		Description: "New user registration",
		Steps: []WorkflowStep{
			{Service: "frontend", Operation: "POST /api/users/register", SpanKind: "server", DurationMs: 90, CanParallel: true},
			{Service: "backend", Operation: "ValidateInput", SpanKind: "server", DurationMs: 40, CanParallel: false},
			{Service: "database", Operation: "SELECT users", SpanKind: "client", DurationMs: 25, CanParallel: false}, // Check if exists
			{Service: "database", Operation: "INSERT users", SpanKind: "client", DurationMs: 35, CanParallel: false},
			{Service: "auth", Operation: "CreateSession", SpanKind: "client", DurationMs: 30, CanParallel: false},
			{Service: "analytics", Operation: "TrackEvent", SpanKind: "client", DurationMs: 15, CanParallel: false},
		},
	},
}

// SelectWorkflow selects a workflow based on weights
func SelectWorkflow(weights map[string]float64, rng *rand.Rand) string {
	if len(weights) == 0 {
		// Default uniform distribution
		workflowNames := make([]string, 0, len(workflows))
		for name := range workflows {
			workflowNames = append(workflowNames, name)
		}
		return workflowNames[rng.Intn(len(workflowNames))]
	}

	// Normalize weights
	totalWeight := 0.0
	for _, weight := range weights {
		totalWeight += weight
	}

	if totalWeight == 0 {
		// Fallback to uniform
		workflowNames := make([]string, 0, len(workflows))
		for name := range workflows {
			workflowNames = append(workflowNames, name)
		}
		return workflowNames[rng.Intn(len(workflowNames))]
	}

	// Weighted random selection
	r := rng.Float64() * totalWeight
	currentWeight := 0.0

	for workflowName, weight := range weights {
		currentWeight += weight
		if r <= currentWeight {
			// Verify workflow exists
			if _, exists := workflows[workflowName]; exists {
				return workflowName
			}
		}
	}

	// Fallback to first workflow
	for name := range workflows {
		return name
	}

	return "place_order" // Ultimate fallback
}

// GetWorkflow returns a workflow by name
func GetWorkflow(name string) (Workflow, bool) {
	wf, ok := workflows[name]
	return wf, ok
}

// GenerateWorkflowContext creates a new workflow context with business IDs
func GenerateWorkflowContext(workflowName string, rng *rand.Rand, cardConfig map[string]int) *WorkflowContext {
	cm := GetCardinalityManager()

	ctx := &WorkflowContext{
		WorkflowName:  workflowName,
		UserID:        cm.GetValue("customer_id", rng, cardConfig), // Reuse customer_id pool
		SessionID:     cm.GetValue("session_id", rng, cardConfig),
		RequestID:     cm.GetValue("request_id", rng, cardConfig),
		CorrelationID: cm.GetValue("correlation_id", rng, cardConfig),
	}

	// Generate workflow-specific IDs
	switch workflowName {
	case "place_order", "process_refund":
		ctx.OrderID = cm.GetValue("order_id", rng, cardConfig)
		ctx.PaymentID = cm.GetValue("payment_id", rng, cardConfig)
		if workflowName == "place_order" {
			ctx.ProductID = fmt.Sprintf("product-%06d", rng.Intn(10000)+1)
			ctx.ShipmentID = cm.GetValue("shipment_id", rng, cardConfig)
		}
	case "browse_products", "search_products":
		ctx.ProductID = fmt.Sprintf("product-%06d", rng.Intn(10000)+1)
	case "user_registration":
		ctx.UserID = cm.GetValue("customer_id", rng, cardConfig) // New user
	}

	return ctx
}

// GetWorkflowOperationName returns the operation name for a workflow step
func GetWorkflowOperationName(workflowName string, stepIndex int) string {
	wf, ok := workflows[workflowName]
	if !ok || stepIndex >= len(wf.Steps) {
		return "unknown-operation"
	}
	return wf.Steps[stepIndex].Operation
}

// GetWorkflowService returns the service name for a workflow step
func GetWorkflowService(workflowName string, stepIndex int) string {
	wf, ok := workflows[workflowName]
	if !ok || stepIndex >= len(wf.Steps) {
		return "frontend"
	}
	return wf.Steps[stepIndex].Service
}

// GetWorkflowSpanKind returns the span kind for a workflow step
func GetWorkflowSpanKind(workflowName string, stepIndex int) string {
	wf, ok := workflows[workflowName]
	if !ok || stepIndex >= len(wf.Steps) {
		return "server"
	}
	return wf.Steps[stepIndex].SpanKind
}

// GetWorkflowStepDuration returns the base duration for a workflow step
func GetWorkflowStepDuration(workflowName string, stepIndex int) int {
	wf, ok := workflows[workflowName]
	if !ok || stepIndex >= len(wf.Steps) {
		return 50
	}
	return wf.Steps[stepIndex].DurationMs
}

// GetWorkflowSteps returns all steps for a workflow
func GetWorkflowSteps(workflowName string) []WorkflowStep {
	wf, ok := workflows[workflowName]
	if !ok {
		return []WorkflowStep{}
	}
	return wf.Steps
}
