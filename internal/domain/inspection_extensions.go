package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

func BuildAssessmentTrend(b *Batch, from, to int, now time.Time) (*TrendSummary, error) {
	if b == nil {
		return nil, ErrNotFound
	}
	if len(b.Assessments) == 0 {
		return nil, fmt.Errorf("%w: 批次没有风险评估快照", ErrNotFound)
	}
	bySequence := make(map[int]AssessmentSnapshot, len(b.Assessments))
	for _, snapshot := range b.Assessments {
		if snapshot.Sequence <= 0 || bySequence[snapshot.Sequence].Sequence != 0 {
			return nil, fmt.Errorf("%w: 风险评估序号缺失或重复", ErrInvalid)
		}
		bySequence[snapshot.Sequence] = snapshot
	}
	for sequence := 1; sequence <= len(b.Assessments); sequence++ {
		if _, ok := bySequence[sequence]; !ok {
			return nil, fmt.Errorf("%w: 风险评估缺少序号 %d", ErrInvalid, sequence)
		}
	}
	if from == 0 {
		from = 1
	}
	if to == 0 {
		to = len(b.Assessments)
	}
	if from > to {
		return nil, fmt.Errorf("%w: from不能大于to", ErrInvalid)
	}
	if _, ok := bySequence[from]; !ok {
		return nil, fmt.Errorf("%w: 不存在起始评估序号 %d", ErrNotFound, from)
	}
	if _, ok := bySequence[to]; !ok {
		return nil, fmt.Errorf("%w: 不存在结束评估序号 %d", ErrNotFound, to)
	}
	result := &TrendSummary{From: from, To: to, GeneratedAt: now}
	metrics := map[string]*MetricTrend{}
	for sequence := 1; sequence <= to; sequence++ {
		snapshot := bySequence[sequence]
		failed := make([]string, 0)
		for _, rule := range snapshot.Rules {
			m := metrics[rule.Metric]
			if m == nil {
				m = &MetricTrend{Metric: rule.Metric}
				metrics[rule.Metric] = m
			}
			if rule.Result == "fail" {
				m.FailureCount++
				if m.FirstFailure == 0 {
					m.FirstFailure = sequence
				}
				failed = append(failed, rule.Metric)
			} else if rule.Result == "pass" {
				m.LatestPass = sequence
			}
		}
		if sequence < from {
			continue
		}
		for _, defectID := range snapshot.AutoClosedDefectIDs {
			result.AutoClosed = append(result.AutoClosed, AutoClosedTrend{DefectID: defectID, Sequence: sequence, ItemRevision: snapshot.AutoClosedRevisions[defectID]})
		}
		direction := "unchanged"
		if sequence > 1 {
			direction = riskDirection(bySequence[sequence-1].RiskLevel, snapshot.RiskLevel)
		}
		sort.Strings(failed)
		result.Points = append(result.Points, TrendPoint{Sequence: sequence, RiskLevel: snapshot.RiskLevel, FailedMetrics: failed, ActiveDefectCount: len(snapshot.DefectIDs), Direction: direction})
	}
	for _, m := range metrics {
		result.Metrics = append(result.Metrics, *m)
	}
	sort.Slice(result.Metrics, func(i, j int) bool { return result.Metrics[i].Metric < result.Metrics[j].Metric })
	return result, nil
}

func riskDirection(previous, current string) string {
	order := map[string]int{"unknown": 0, "low": 1, "medium": 2, "high": 3}
	if order[current] > order[previous] {
		return "up"
	}
	if order[current] < order[previous] {
		return "down"
	}
	return "unchanged"
}

type DueStatus struct {
	Overdue   bool
	Warning   bool
	Escalated bool
}

func (d *Defect) CurrentDueStatus(now time.Time) DueStatus {
	if d == nil || d.DueAt == nil || d.Status == "closed" || d.Status == "ready_review" {
		return DueStatus{}
	}
	if now.After(*d.DueAt) {
		return DueStatus{Overdue: true, Escalated: true}
	}
	threshold := 7 * 24 * time.Hour
	if d.Severity == "high" {
		threshold = 48 * time.Hour
	} else if d.Severity == "medium" {
		threshold = 72 * time.Hour
	}
	return DueStatus{Warning: d.DueAt.Sub(now) <= threshold}
}

func (d *Defect) AdjustDueDate(due time.Time, reason, responsible string, now time.Time) error {
	if d.Status != "assigned" && d.Status != "rework" {
		return fmt.Errorf("%w: 仅assigned或rework缺陷可调整期限", ErrState)
	}
	if !due.After(now) || strings.TrimSpace(reason) == "" || strings.TrimSpace(responsible) == "" {
		return fmt.Errorf("%w: 新期限必须是未来时间，且原因和责任人不能为空", ErrInvalid)
	}
	copyDue := due
	d.DueAt = &copyDue
	d.Responsible = responsible
	d.Assignee = responsible
	d.DueDateHistory = append(d.DueDateHistory, DueDateRevision{DueAt: &copyDue, Reason: reason, Responsible: responsible, ChangedAt: now})
	d.EscalatedAt, d.EscalatedTo = nil, ""
	d.UpdatedAt = now
	return nil
}

func (d *Defect) RefreshEscalation(now time.Time) bool {
	status := d.CurrentDueStatus(now)
	if !status.Escalated || d.EscalatedAt != nil {
		return false
	}
	target := d.MaintenanceTeam
	if target == "" {
		target = d.Responsible
	}
	d.EscalatedAt, d.EscalatedTo = &now, target
	d.EscalationHistory = append(d.EscalationHistory, DefectEscalation{At: now, To: target, Reason: "整改期限已逾期"})
	return true
}
