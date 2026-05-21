package main

import (
	"os"

	"github.com/hashicorp/terraform-run-task-scaffolding-go/internal/config"
	"github.com/hashicorp/terraform-run-task-scaffolding-go/internal/helpers"
	"github.com/hashicorp/terraform-run-task-scaffolding-go/internal/runtask"
)

func main() {
	// Load configuration from CLI args, files, and environment variables
	cfg, err := config.Load()
	if err != nil {
		helpers.Errorf("Failed to load configuration: %v", err)
		os.Exit(1)
	}

	// Initialize logger with configuration
	if err := helpers.Initialize(cfg.LogLevel, cfg.LogFormat); err != nil {
		// If initialization fails, continue with defaults
		helpers.Warnf("Failed to initialize logger: %v", err)
	}

	helpers.Infof("Starting HCP Terraform Cost Estimation Run Task")
	helpers.Infof("Server address: %s, Path: %s", cfg.ServerAddr, cfg.ServerPath)

	task := runtask.NewRunTask()
	task.SetAppConfig(cfg)
	task.Configure(cfg.ServerAddr, cfg.ServerPath, cfg.HMACKey)
	runtask.HandleRequests(task)
}
