package domain

import "time"

type TrendPoint struct {
	Sequence          int      `json:"sequence"`
	RiskLevel         string   `json:"riskLevel"`
	FailedMetrics     []string `json:"failedMetrics"`
	ActiveDefectCount int      `json:"activeDefectCount"`
	Direction         string   `json:"direction"`
}

type MetricTrend struct {
	Metric       string `json:"metric"`
	FailureCount int    `json:"failureCount"`
	FirstFailure int    `json:"firstFailure"`
	LatestPass   int    `json:"latestPass"`
}

type TrendSummary struct {
	From        int               `json:"from"`
	To          int               `json:"to"`
	GeneratedAt time.Time         `json:"generatedAt"`
	Points      []TrendPoint      `json:"points"`
	Metrics     []MetricTrend     `json:"metrics"`
	AutoClosed  []AutoClosedTrend `json:"autoClosed,omitempty"`
}

type AutoClosedTrend struct {
	DefectID     string `json:"defectID"`
	Sequence     int    `json:"sequence"`
	ItemRevision int    `json:"itemRevision"`
}
