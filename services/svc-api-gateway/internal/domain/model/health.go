package model

import "time"

type (
	HealthStatus string

	DependencyStatus string

	DependencyCheck struct {
		Status      DependencyStatus
		LatencyMs   uint64
		Message     string
		LastChecked time.Time
		Error       string
	}

	LivenessReport struct {
		Status    HealthStatus
		Timestamp time.Time
		Version   string
	}

	ReadinessReport struct {
		Status    HealthStatus
		Timestamp time.Time
		Version   string
		Checks    map[string]DependencyCheck
	}

	HealthReport struct {
		Status    HealthStatus
		Timestamp time.Time
		Version   VersionInfo
		Uptime    UptimeInfo
		Checks    map[string]DependencyCheck
		System    SystemInfo
	}

	VersionInfo struct {
		API   string
		Build string
		Go    string
	}

	UptimeInfo struct {
		StartedAt       time.Time
		Duration        string
		DurationSeconds uint64
	}

	SystemInfo struct {
		Memory     MemoryInfo
		Goroutines uint
		CPUCores   uint
	}

	MemoryInfo struct {
		AllocMB      float64
		TotalAllocMB float64
		SysMB        float64
		GCCycles     uint32
	}
)

const (
	HealthStatusOK          HealthStatus = "ok"
	HealthStatusDegraded    HealthStatus = "degraded"
	HealthStatusDown        HealthStatus = "down"
	HealthStatusMaintenance HealthStatus = "maintenance"

	DependencyStatusUp       DependencyStatus = "up"
	DependencyStatusDown     DependencyStatus = "down"
	DependencyStatusDegraded DependencyStatus = "degraded"
	DependencyStatusUnknown  DependencyStatus = "unknown"
)
