package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"

	"trading-core/pkg/binance"
	"trading-core/pkg/config"
	"trading-core/pkg/db"
)

type HealthStatus struct {
	Service   string    `json:"service"`
	Status    string    `json:"status"`
	Message   string    `json:"message,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type HealthReport struct {
	Overall  string         `json:"overall"`
	Services []HealthStatus `json:"services"`
}

func main() {
	// Load environment
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found")
	}

	fmt.Println("🏥 DES-V2 Health Check")
	fmt.Println("=====================")
	fmt.Println()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	report := HealthReport{
		Overall:  "HEALTHY",
		Services: make([]HealthStatus, 0),
	}

	// 1. Config check
	report.Services = append(report.Services, checkConfig())

	// 2. Database check
	report.Services = append(report.Services, checkDatabase())

	// 3. Binance connectivity check
	report.Services = append(report.Services, checkBinance(ctx))

	// 4. API server check
	report.Services = append(report.Services, checkAPIServer(ctx))

	// 5. Python Worker check (if enabled)
	if os.Getenv("ENABLE_PYTHON_WORKER") == "true" {
		report.Services = append(report.Services, checkPythonWorker(ctx))
	}

	// Determine overall status
	for _, svc := range report.Services {
		if svc.Status == "UNHEALTHY" {
			report.Overall = "UNHEALTHY"
			break
		} else if svc.Status == "DEGRADED" && report.Overall != "UNHEALTHY" {
			report.Overall = "DEGRADED"
		}
	}

	// Print results
	fmt.Println()
	fmt.Println("Results:")
	fmt.Println("--------")
	for _, svc := range report.Services {
		statusIcon := "✓"
		if svc.Status == "UNHEALTHY" {
			statusIcon = "✗"
		} else if svc.Status == "DEGRADED" {
			statusIcon = "⚠"
		}
		fmt.Printf("%s %-20s %s %s\n", statusIcon, svc.Service, svc.Status, svc.Message)
	}

	fmt.Println()
	fmt.Printf("Overall Status: %s\n", report.Overall)

	// Output JSON if requested
	if len(os.Args) > 1 && os.Args[1] == "--json" {
		jsonData, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(jsonData))
	}

	// Exit code
	if report.Overall == "UNHEALTHY" {
		os.Exit(1)
	}
}

func checkConfig() HealthStatus {
	status := HealthStatus{
		Service:   "Configuration",
		Status:    "HEALTHY",
		Timestamp: time.Now(),
	}

	cfg, err := config.Load()
	if err != nil {
		status.Status = "UNHEALTHY"
		status.Message = fmt.Sprintf("Failed to load: %v", err)
		return status
	}

	if cfg.Port == "" {
		status.Status = "DEGRADED"
		status.Message = "Port not configured"
	}

	status.Message = fmt.Sprintf("Port=%s", cfg.Port)
	return status
}

func checkDatabase() HealthStatus {
	status := HealthStatus{
		Service:   "Database",
		Status:    "HEALTHY",
		Timestamp: time.Now(),
	}

	cfg, _ := config.Load()
	database, err := db.New(cfg.DBPath)
	if err != nil {
		status.Status = "UNHEALTHY"
		status.Message = fmt.Sprintf("Connection failed: %v", err)
		return status
	}
	defer database.Close()

	// Try a simple query
	err = database.Ping()
	if err != nil {
		status.Status = "UNHEALTHY"
		status.Message = fmt.Sprintf("Ping failed: %v", err)
		return status
	}

	status.Message = "Connected"
	return status
}

func checkBinance(ctx context.Context) HealthStatus {
	status := HealthStatus{
		Service:   "Binance API",
		Status:    "HEALTHY",
		Timestamp: time.Now(),
	}

	cfg, _ := config.Load()

	// Skip if no API key
	if cfg.BinanceAPIKey == "" {
		status.Status = "DEGRADED"
		status.Message = "No API key configured"
		return status
	}

	client := binance.NewClient(cfg.BinanceAPIKey, cfg.BinanceAPISecret, cfg.BinanceTestnet)

	// Test server time endpoint
	serverTime, err := client.GetServerTime()
	if err != nil {
		status.Status = "UNHEALTHY"
		status.Message = fmt.Sprintf("Connection failed: %v", err)
		return status
	}

	network := "MAINNET"
	if cfg.BinanceTestnet {
		network = "TESTNET"
	}

	status.Message = fmt.Sprintf("Connected to %s (time=%d)", network, serverTime)
	return status
}

func checkAPIServer(ctx context.Context) HealthStatus {
	status := HealthStatus{
		Service:   "API Server",
		Status:    "HEALTHY",
		Timestamp: time.Now(),
	}

	cfg, _ := config.Load()
	url := fmt.Sprintf("http://localhost:%s/health", cfg.Port)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		status.Status = "UNHEALTHY"
		status.Message = fmt.Sprintf("Not reachable: %v", err)
		return status
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		status.Status = "DEGRADED"
		status.Message = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return status
	}

	status.Message = "Running"
	return status
}

func checkPythonWorker(ctx context.Context) HealthStatus {
	status := HealthStatus{
		Service:   "Python Worker",
		Status:    "HEALTHY",
		Timestamp: time.Now(),
	}

	// For now, just check if the address is configured
	addr := os.Getenv("PYTHON_WORKER_ADDR")
	if addr == "" {
		status.Status = "DEGRADED"
		status.Message = "Address not configured"
		return status
	}

	// TODO: Add actual gRPC health check
	status.Message = fmt.Sprintf("Configured at %s", addr)
	return status
}
