# Architecture Diagram: HCP Terraform Cost Estimation Run Task

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         HCP Terraform / Terraform Enterprise                 │
│                                                                               │
│  ┌──────────────┐         ┌──────────────┐         ┌──────────────┐        │
│  │ Terraform    │────────▶│  Run Task    │────────▶│   Callback   │        │
│  │   Plan       │         │   Request    │         │   Response   │        │
│  └──────────────┘         └──────────────┘         └──────────────┘        │
│                                  │                         ▲                 │
└──────────────────────────────────┼─────────────────────────┼─────────────────┘
                                   │                         │
                                   │ POST /runtask           │ PATCH callback
                                   │ (HMAC signed)           │
                                   ▼                         │
┌─────────────────────────────────────────────────────────────────────────────┐
│                        Cost Estimation Run Task Server                       │
│                              (Port 22180)                                    │
│                                                                               │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                         main.go (Entry Point)                         │  │
│  │  • Parse CLI flags (addr, path, hmacKey)                              │  │
│  │  • Initialize ScaffoldingRunTask                                      │  │
│  │  • Start HTTP server                                                  │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                   │                                          │
│                                   ▼                                          │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                    internal/runtask/run_task_handler.go               │  │
│  │                                                                         │  │
│  │  ┌─────────────────────────────────────────────────────────────────┐  │  │
│  │  │ HandleRequests()                                                 │  │  │
│  │  │  • Register /runtask endpoint                                   │  │  │
│  │  │  • Register /healthcheck endpoint                               │  │  │
│  │  └─────────────────────────────────────────────────────────────────┘  │  │
│  │                                   │                                     │  │
│  │                                   ▼                                     │  │
│  │  ┌─────────────────────────────────────────────────────────────────┐  │  │
│  │  │ handleTFCRequestWrapper()                                        │  │  │
│  │  │  1. Parse JSON request                                          │  │  │
│  │  │  2. Verify HMAC signature                                       │  │  │
│  │  │  3. Handle endpoint validation                                  │  │  │
│  │  │  4. Call VerifyRequest()                                        │  │  │
│  │  │  5. Retrieve TFC Plan (if post-plan/pre-apply)                 │  │  │
│  │  │  6. Call VerifyPlan()                                           │  │  │
│  │  │  7. Send callback response                                      │  │  │
│  │  └─────────────────────────────────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                   │                                          │
│                                   ▼                                          │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │              internal/runtask/run_task_scaffolding.go                 │  │
│  │                                                                         │  │
│  │  ┌─────────────────────────────────────────────────────────────────┐  │  │
│  │  │ ScaffoldingRunTask                                              │  │  │
│  │  │  • config (addr, path, hmacKey)                                │  │  │
│  │  │  • logger                                                       │  │  │
│  │  │  • c3xEstimator                                                 │  │  │
│  │  │  • agentCoordinator                                             │  │  │
│  │  │  • budgetLimit                                                  │  │  │
│  │  └─────────────────────────────────────────────────────────────────┘  │  │
│  │                                   │                                     │  │
│  │                                   ▼                                     │  │
│  │  ┌─────────────────────────────────────────────────────────────────┐  │  │
│  │  │ VerifyPlan()                                                    │  │  │
│  │  │  1. Call c3xEstimator.EstimateCost()                           │  │  │
│  │  │  2. Check budget limit                                          │  │  │
│  │  │  3. Call agentCoordinator.AnalyzeAll()                         │  │  │
│  │  │  4. Build callback response with outcomes                      │  │  │
│  │  └─────────────────────────────────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                   │                                          │
│                    ┌──────────────┴──────────────┐                          │
│                    ▼                              ▼                          │
│  ┌─────────────────────────────┐   ┌──────────────────────────────────┐    │
│  │  internal/c3x/estimator.go  │   │  internal/agents/coordinator.go  │    │
│  │                             │   │                                  │    │
│  │  ┌───────────────────────┐ │   │  ┌────────────────────────────┐ │    │
│  │  │ EstimateCost()        │ │   │  │ AnalyzeAll()               │ │    │
│  │  │  • Call c3x API       │ │   │  │  • Run agents in parallel  │ │    │
│  │  │  • Parse response     │ │   │  │  • Collect responses       │ │    │
│  │  │  • Return estimate    │ │   │  │  • Format markdown         │ │    │
│  │  └───────────────────────┘ │   │  └────────────────────────────┘ │    │
│  └─────────────────────────────┘   │                │                │    │
│                                     │    ┌───────────┴───────────┐   │    │
│                                     │    ▼                       ▼   │    │
│                                     │  ┌──────────┐   ┌──────────┐  │    │
│                                     │  │ Security │   │ Pricing  │  │    │
│                                     │  │  Agent   │   │  Agent   │  │    │
│                                     │  └──────────┘   └──────────┘  │    │
│                                     └──────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────────┘
                    │                              │
                    ▼                              ▼
┌──────────────────────────────┐   ┌──────────────────────────────────────┐
│   c3x Pricing API Server     │   │      Google Gemini AI (ADK)          │
│      (Port 4000)              │   │                                      │
│                               │   │  ┌────────────────────────────────┐ │
│  • PostgreSQL database        │   │  │ Security Risk Analyzer         │ │
│  • Pricing data scraper       │   │  │  • Check public exposure       │ │
│  • Cost calculation engine    │   │  │  • Check encryption            │ │
│  • AWS/Azure/GCP pricing      │   │  │  • Check IAM permissions       │ │
│                               │   │  └────────────────────────────────┘ │
└──────────────────────────────┘   │                                      │
                                    │  ┌────────────────────────────────┐ │
                                    │  │ Pricing Optimization Advisor   │ │
                                    │  │  • Calculate savings           │ │
                                    │  │  • Compare instance types      │ │
                                    │  │  • Analyze reserved capacity   │ │
                                    │  └────────────────────────────────┘ │
                                    └──────────────────────────────────────┘
```

## Component Flow

### 1. Request Flow (Left to Right)
```
HCP Terraform → Run Task Server → Cost Estimator → c3x API
                                 ↓
                          AI Agent Coordinator → Google Gemini AI
                                 ↓
                          Callback Response → HCP Terraform
```

### 2. Data Flow

```
┌─────────────────┐
│ Terraform Plan  │
│  (JSON format)  │
└────────┬────────┘
         │
         ▼
┌─────────────────────────────────────────┐
│  Cost Estimation                        │
│  • Parse resources                      │
│  • Query c3x pricing API                │
│  • Calculate monthly costs              │
│  • Check budget limits                  │
└────────┬────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────┐
│  AI Agent Analysis (Parallel)           │
│                                          │
│  ┌────────────────┐  ┌────────────────┐│
│  │ Security Agent │  │ Pricing Agent  ││
│  │                │  │                ││
│  │ • Analyze      │  │ • Analyze      ││
│  │   security     │  │   cost         ││
│  │   risks        │  │   optimization ││
│  │ • Generate     │  │ • Generate     ││
│  │   findings     │  │   findings     ││
│  └────────────────┘  └────────────────┘│
└────────┬────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────┐
│  Callback Response                      │
│  • Status (passed/warning/failed)       │
│  • Cost estimate outcome                │
│  • AI agent outcomes (if enabled)       │
│  • Markdown formatted results           │
└─────────────────────────────────────────┘
```


## Deployment Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Docker Compose                        │
│                                                           │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  │
│  │ PostgreSQL   │  │ c3x Pricing  │  │ Run Task     │  │
│  │   Database   │◀─│     API      │◀─│   Server     │  │
│  │              │  │              │  │              │  │
│  │ Port: 5432   │  │ Port: 4000   │  │ Port: 22180  │  │
│  └──────────────┘  └──────────────┘  └──────────────┘  │
│                                              ▲           │
└──────────────────────────────────────────────┼───────────┘
                                               │
                                               │ HTTPS
                                               │
                                    ┌──────────┴──────────┐
                                    │  Reverse Proxy      │
                                    │  (nginx/traefik)    │
                                    └──────────┬──────────┘
                                               │
                                               ▼
                                    ┌─────────────────────┐
                                    │  HCP Terraform      │
                                    └─────────────────────┘