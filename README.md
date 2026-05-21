# HCP Terraform Cost Estimation Run Task and AI analysis

This tool integrates with HCP Terraform as a run task to provide automated cost estimation and AI-powered analysis for infrastructure changes.

It analyzes Terraform plan files to calculate monthly cloud costs across AWS, Azure, and GCP using the c3x pricing engine, while optional AI agents identify security risks and cost optimization opportunities.

The tool supports budget guardrails to automatically fail runs exceeding cost thresholds, and includes secret detection to redact sensitive data (API keys, passwords, tokens) from all outputs and logs.

All analysis results are presented directly in the Terraform Cloud UI as structured markdown reports with actionable recommendations.

No external API dependencies are required for cost estimation, making it suitable for air-gapped environments.

## Quick start

Using the docker-compose file is the easiest way to get started.

* Pricing api will run locally
* Wormhole will be used to expose your local runtask to the internet and HCP

First you need to buoild the docker image and run the docker compose.

```shell
Copy the example environment file and configure it
cp .env.example .env
# Edit .env file to set POSTGRES_PASSWORD and HMAC_KEY (minimum required)

# Build the Docker image and start all services
docker-compose up -d
```

Then you need to sync the database with the diffferent pricing data.

```shell
docker-compose exec c3x-pricing-api ./c3x-pricing-api scrape --vendor all
```

Get the wormhole URL from the logs and configure it in the HCP Terraform run task.

```shell
docker-compose logs wormhole | grep -E "(https://|Forwarding|URL)" | tail -20
```

## Cost estimations

HCP Terraform run task for cloud cost estimation using [c3x](https://github.com/c3xdev/c3x). Analyzes Terraform plans and provides cost estimates with optional budget guardrails.

- Cost estimation for AWS, Azure, and GCP via c3x
- Budget guardrails (auto-fail runs exceeding limits)
- Detailed per-resource cost breakdown
- Offline mode support
- No external API keys required

## AI Analysis

Optional AI-powered analysis using Google's Gemini AI and [Agent Development Kit (ADK)](https://github.com/google/adk). Two specialized agents analyze Terraform plans in parallel:

- **Security Risk Analyzer** ([`prompts/security_system.txt`](prompts/security_system.txt)) - Identifies vulnerabilities (public exposure, missing encryption, overly permissive IAM, compliance violations)
- **Pricing Optimization Advisor** ([`prompts/pricing_system.txt`](prompts/pricing_system.txt)) - Recommends cost savings (right-sizing, reserved instances, storage optimization, scheduling)

Agents run concurrently after cost estimation and provide structured findings with severity levels (critical/high/medium/low). Configure via environment variables (`AGENT_SECURITY_ENABLED`, `AGENT_PRICING_ENABLED`) and optionally fail runs on critical findings with `AGENT_FAIL_ON_CRITICAL=true`.

## Architecture

### Data Flow

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

See [ARCHITECTURE_DIAGRAM.md](ARCHITECTURE_DIAGRAM.md) for detailed component architecture.

## Features



## Prerequisites

- Terraform Cloud or Terraform Enterprise >= v202206-1
- [Manage Run Tasks](https://developer.hashicorp.com/terraform/cloud-docs/users-teams-organizations/permissions#manage-run-tasks) permission
- Docker + Docker Compose /Podman OR Go >= 1.20

## Quick Start

```bash
git clone <this-repository>
cd hashicorp-cost-estimation-run-task
cp .env.example .env
# Edit .env: set POSTGRES_PASSWORD and HMAC_KEY
docker-compose up -d
docker-compose exec c3x-pricing-api ./c3x-pricing-api scrape --vendor all
```

Run task available at `http://localhost:22180/runtask`

## Configuration

Configuration priority: **CLI arguments** > **Secret files** > **Environment variables**

### Core Configuration

| Environment Variable | CLI Flag | Required | Default | Description |
|---------------------|----------|----------|---------|-------------|
| `LOG_LEVEL` | `--log-level` | No | `INFO` | Logging verbosity: `DEBUG`, `INFO`, `WARN`, `ERROR` |
| `LOG_FORMAT` | `--log-format` | No | `text` | Log format: `text`, `json` |
| `SERVER_ADDR` | `--addr` | No | `22180` | Server port |
| `SERVER_PATH` | `--path` | No | `/runtask` | URL path for run task |
| `POSTGRES_PASSWORD` | - | Yes | - | PostgreSQL password |
| `HMAC_KEY` | `--hmac-key` | Yes | `secret123` | HMAC secret for HCP Terraform |
| - | `--hmac-key-file` | No | - | Path to file containing HMAC key (more secure) |
| `C3X_PRICING_API_ENDPOINT` | - | No | `http://localhost:4000` | c3x pricing API endpoint |
| `C3X_BUDGET_LIMIT` | - | No | - | Max monthly cost USD (e.g., `1000.00`) |
| `C3X_API_KEY` | - | No | - | API key if pricing API requires auth |
| `C3X_API_KEY_FILE` | `--c3x-api-key-file` | No | - | Path to file containing C3X API key (more secure) |
| `GCP_API_KEY` | - | No | - | Google Cloud API key for pricing data |
| - | `--gcp-api-key-file` | No | - | Path to file containing GCP API key (more secure) |

### AI Agent Configuration

| Environment Variable | Required | Default | Description |
|---------------------|----------|---------|-------------|
| `GOOGLE_API_KEY` or `GEMINI_API_KEY` | Yes* | - | Google API key for Gemini AI agents (must be environment variable) |
| `GEMINI_MODEL` | No | `gemini-3.1-flash-lite` | Gemini model to use for AI agents |
| `AGENT_SECURITY_ENABLED` | No | `false` | Enable Security Risk Analyzer agent |
| `AGENT_PRICING_ENABLED` | No | `false` | Enable Pricing Optimization Advisor agent |
| `AGENT_FAIL_ON_CRITICAL` | No | `false` | Fail run if agents find critical issues |

*Required only if any agent is enabled

**Important:** The ADK library requires `GOOGLE_API_KEY` or `GEMINI_API_KEY` to be set as environment variables. File-based secrets are NOT supported for these keys.

**Example with CLI flags:**
```bash
# Set Google API key as environment variable (required for ADK)
export GOOGLE_API_KEY="your-api-key-here"

./cost-estimation-run-task \
  --addr 8080 \
  --hmac-key-file /secrets/hmac-key \
  --c3x-api-key-file /secrets/c3x-api-key \
  --log-level DEBUG
```



### Getting a Google API Key

1. Visit [Google AI Studio](https://aistudio.google.com/app/apikey)
2. Create a new API key
3. Add it to your `.env` file as `GOOGLE_API_KEY`


## Local Testing with Wormhole

Use [Wormhole](https://github.com/hashicorp/wormhole) to expose your local run task to HCP Terraform:

```bash
docker-compose up -d
docker-compose exec c3x-pricing-api ./c3x-pricing-api scrape --vendor all
wormhole http 22180
```

Configure in HCP Terraform:
- **Endpoint**: `https://your-wormhole-url/runtask`
- **HMAC Key**: From `.env` file
- **Enforcement**: Advisory or Mandatory


## Troubleshooting

**Pricing API not responding:**
```bash
docker-compose ps
curl http://localhost:4000/healthz
docker-compose logs c3x-pricing-api
```

**No pricing data:**
```bash
curl http://localhost:4000/status
docker-compose exec c3x-pricing-api ./c3x-pricing-api scrape --vendor all
```

**Cost estimation fails:**
```bash
docker-compose logs cost-estimation-run-task
docker-compose exec cost-estimation-run-task wget -O- http://c3x-pricing-api:4000/healthz
```

**Budget checks not working:**
- Verify `C3X_BUDGET_LIMIT` in `.env`
- Check logs for "Budget limit set to $X.XX"
- Use valid float format: `1000.00` not `$1,000`

## License

MPL-2.0 - see [LICENSE](LICENSE)

## Related Projects

- [c3x](https://github.com/c3xdev/c3x) - Cloud cost estimation tool
- [HCP Terraform Run Tasks](https://developer.hashicorp.com/terraform/cloud-docs/integrations/run-tasks) - Official docs