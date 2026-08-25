package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func NewFacility(id, name, district, typ string, capacity, normal float64, gates, pumps int, route string, now time.Time) (*Facility, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("%w: facilityID不能为空", ErrInvalid)
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("%w: 设施名称不能为空", ErrInvalid)
	}
	if capacity <= 0 {
		return nil, fmt.Errorf("%w: 设计容量必须大于零", ErrInvalid)
	}
	if normal < 0 || gates < 0 || pumps < 0 {
		return nil, fmt.Errorf("%w: 水位、闸门数和泵组数不能为负数", ErrInvalid)
	}
	return &Facility{FacilityID: id, Name: name, District: district, FacilityType: typ, DesignCapacity: capacity, NormalWaterLevel: normal, GateCount: gates, PumpCount: pumps, EmergencyRoute: route, Status: "active", StatusChangedAt: now, Version: 1, CreatedAt: now, UpdatedAt: now}, nil
}
func NewBatch(id, facility, window, inspector string, now time.Time) (*Batch, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(facility) == "" || strings.TrimSpace(inspector) == "" || strings.TrimSpace(window) == "" {
		return nil, fmt.Errorf("%w: batchID、facilityID、inspectionWindow和inspectorID不能为空", ErrInvalid)
	}
	return &Batch{BatchID: id, FacilityID: facility, InspectionWindow: window, InspectorID: inspector, Status: "draft", RiskLevel: "unknown", Version: 1, CreatedAt: now}, nil
}

func ParseInspectionWindow(start, end time.Time) (time.Time, time.Time, error) {
	if start.IsZero() || end.IsZero() {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: 检查窗口必须包含时区", ErrInvalid)
	}
	start = start.Round(0)
	end = end.Round(0)
	if !end.After(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: 检查窗口结束时间必须晚于开始时间", ErrInvalid)
	}
	return start, end, nil
}
func NewBatchWithWindow(id, facility, label, inspector string, start, end, now time.Time) (*Batch, error) {
	if strings.TrimSpace(label) == "" {
		label = start.Format(time.RFC3339) + "至" + end.Format(time.RFC3339)
	}
	s, e, err := ParseInspectionWindow(start, end)
	if err != nil {
		return nil, err
	}
	b, err := NewBatch(id, facility, label, inspector, now)
	if err != nil {
		return nil, err
	}
	b.WindowStart, b.WindowEnd = s, e
	return b, nil
}
func WindowsOverlap(aStart, aEnd, bStart, bEnd time.Time) bool {
	return aStart.Before(bEnd) && bStart.Before(aEnd)
}
func (b *Batch) WindowActive(now time.Time) bool {
	return !b.WindowStart.IsZero() && !b.WindowEnd.IsZero() && !now.Before(b.WindowStart) && now.Before(b.WindowEnd)
}
func (b *Batch) RequireWindow(now time.Time) error {
	if b.WindowStart.IsZero() || b.WindowEnd.IsZero() {
		return nil
	}
	if !b.WindowActive(now) {
		return ErrWindowExpired
	}
	return nil
}

func ValidateItems(items []CheckItem, f *Facility, b *Batch, now time.Time) ValidationReport {
	return validateItems(items, f, b, now, true)
}
func ValidateItemRevision(item CheckItem, f *Facility, b *Batch, now time.Time) ValidationReport {
	return validateItems([]CheckItem{item}, f, b, now, false)
}
func validateItems(items []CheckItem, f *Facility, b *Batch, now time.Time, requireAll bool) ValidationReport {
	r := ValidationReport{GeneratedAt: now, Total: len(items)}
	required := map[string]bool{"water_level": false, "gate": false, "pump": false, "emergency_route": false}
	seenID, seenMetric := map[string]bool{}, map[string]bool{}
	for idx := range items {
		items[idx].Metric = strings.ToLower(strings.TrimSpace(items[idx].Metric))
		items[idx].Unit = normalizeUnit(items[idx].Metric, items[idx].Unit)
		item := items[idx]
		path := fmt.Sprintf("items[%d]", idx)
		add := func(code, reason, suggestion string) {
			r.Issues = append(r.Issues, ValidationIssue{Path: path, Metric: item.Metric, Code: code, Reason: reason, Suggestion: suggestion})
		}
		if strings.TrimSpace(item.ItemID) == "" {
			add("missing_item_id", "缺少项目编号", "填写唯一 itemID")
		} else if seenID[item.ItemID] {
			add("duplicate_item_id", "itemID重复", "为每条读数提供唯一编号")
		} else {
			seenID[item.ItemID] = true
		}
		if item.Metric == "" {
			add("missing_metric", "缺少指标", "填写四类必填指标之一")
		} else if seenMetric[item.Metric] {
			add("duplicate_metric", "metric重复", "每个指标只提交一次")
		} else {
			seenMetric[item.Metric] = true
		}
		if _, ok := required[item.Metric]; !ok {
			add("unknown_metric", "指标不在检查清单内", "使用 water_level、gate、pump 或 emergency_route")
		} else {
			required[item.Metric] = true
		}
		if item.Value < 0 {
			add("value_range", "数值不能为负", "提交非负现场读数")
		}
		switch item.Metric {
		case "water_level":
			if f != nil && item.Value > f.NormalWaterLevel*2 {
				add("value_range", "水位读数超过设施允许范围", "核对水位和单位")
			}
		case "gate":
			if f != nil && item.Value > float64(f.GateCount) {
				add("value_range", "闸门读数超过设施闸门数", "核对闸门读数")
			}
		case "pump":
			if f != nil && item.Value > float64(f.PumpCount) {
				add("value_range", "泵组读数超过设施泵组数", "核对泵组读数")
			}
		}
		if strings.TrimSpace(item.Unit) == "" {
			add("unit", "缺少单位", "填写现场读数单位")
		} else if expected := map[string]string{"water_level": "m", "gate": "扇", "pump": "台", "emergency_route": "状态"}[item.Metric]; expected != "" && item.Unit != expected {
			add("unit", "单位与指标不匹配", "使用规范单位 "+expected)
		}
		if strings.TrimSpace(item.CapturedBy) == "" {
			add("captured_by", "缺少采集人", "填写采集人")
		}
		if item.CapturedAt.IsZero() {
			add("captured_at", "缺少采集时间", "填写带时区的采集时间")
		} else if b != nil && (item.CapturedAt.Before(b.WindowStart) || item.CapturedAt.After(b.WindowEnd)) && !b.WindowStart.IsZero() {
			add("captured_at", "采集时间不在检查窗口内", "使用窗口内的现场时间")
		}
		if len(item.EvidenceRefs) == 0 {
			add("evidence", "缺少证据引用", "至少提供一条 evidenceRefs")
		} else {
			r.EvidenceReferences += len(item.EvidenceRefs)
			evidence := map[string]bool{}
			for _, ref := range item.EvidenceRefs {
				if strings.TrimSpace(ref) == "" || evidence[ref] {
					add("evidence", "证据引用为空或重复", "提供不重复的有效证据引用")
				}
				evidence[ref] = true
			}
		}
		if strings.TrimSpace(item.Observation) == "" {
			add("observation", "缺少观察结论", "填写现场观察结论")
		}
	}
	if requireAll {
		for metric, ok := range required {
			if !ok {
				r.Issues = append(r.Issues, ValidationIssue{Path: "items", Metric: metric, Code: "required", Reason: "缺少必填检查项目", Suggestion: "补充该指标读数"})
			}
		}
	}
	r.Passed = r.Total - len(r.Issues)
	if r.Passed < 0 {
		r.Passed = 0
	}
	if r.Total > 0 {
		r.Completeness = r.Passed * 100 / r.Total
	}
	r.Valid = len(r.Issues) == 0 && (!requireAll || len(items) == 4)
	if !r.Valid {
		r.Completeness = 0
	}
	return r
}

func normalizeUnit(metric, unit string) string {
	u := strings.ToLower(strings.TrimSpace(unit))
	switch metric {
	case "water_level":
		if u == "米" || u == "meter" || u == "meters" {
			return "m"
		}
		if u == "m" {
			return "m"
		}
	case "gate":
		if u == "gate" || u == "gates" || u == "count" || u == "扇" {
			return "扇"
		}
	case "pump":
		if u == "pump" || u == "pumps" || u == "count" || u == "台" {
			return "台"
		}
	case "emergency_route":
		if u == "status" || u == "状态" {
			return "状态"
		}
	}
	return strings.TrimSpace(unit)
}

func NormalizeItem(item CheckItem) CheckItem {
	item.Metric = strings.ToLower(strings.TrimSpace(item.Metric))
	item.Unit = normalizeUnit(item.Metric, item.Unit)
	return item
}

func (f *Facility) ChangeStatus(status, reason, actor string, expected int, at time.Time) error {
	if strings.TrimSpace(actor) == "" || strings.TrimSpace(reason) == "" || at.IsZero() {
		return ErrInvalid
	}
	if expected > 0 && f.Version != expected {
		return ErrConflict
	}
	if status != "suspended" && status != "active" {
		return ErrInvalid
	}
	if status == f.Status {
		return ErrState
	}
	if status == "suspended" && f.Status != "active" {
		return ErrState
	}
	if status == "active" && f.Status != "suspended" {
		return ErrState
	}
	f.Status, f.StatusReason, f.StatusChangedAt = status, reason, at
	f.UpdatedAt = at
	f.Version++
	return nil
}

func (b *Batch) AddItems(items []CheckItem, now time.Time) error {
	if b.Status == "reviewed" || b.Status == "frozen" || b.Status == "permitted" {
		return ErrState
	}
	if len(items) == 0 {
		return ErrInvalid
	}
	required := map[string]bool{"water_level": false, "gate": false, "pump": false, "emergency_route": false}
	itemIDs := map[string]bool{}
	for i := range items {
		if strings.TrimSpace(items[i].ItemID) == "" || itemIDs[items[i].ItemID] || items[i].Metric == "" || items[i].CapturedBy == "" || strings.TrimSpace(items[i].Unit) == "" || strings.TrimSpace(items[i].Observation) == "" || len(items[i].EvidenceRefs) == 0 || (items[i].decoded && !items[i].valuePresent) {
			return ErrInvalid
		}
		itemIDs[items[i].ItemID] = true
		if _, ok := required[items[i].Metric]; !ok || items[i].Value < 0 {
			return ErrInvalid
		}
		items[i].BatchID = b.BatchID
		if items[i].Revision == 0 {
			items[i].Revision = 1
		}
		if items[i].CapturedAt.IsZero() {
			items[i].CapturedAt = now
		}
		if items[i].CapturedAt.After(now) || items[i].CapturedAt.Before(b.CreatedAt) {
			return fmt.Errorf("采集时间超出检查窗口")
		}
		required[items[i].Metric] = true
	}
	for k, v := range required {
		if !v {
			return fmt.Errorf("缺少必填检查项目: %s", k)
		}
	}
	b.Items = append(b.Items, items...)
	b.Status = "assessed"
	b.SubmittedAt = &now
	b.Version++
	return nil
}

func (b *Batch) ReviseItem(itemID string, next CheckItem, reason, actor string, now time.Time) error {
	if b.Status == "reviewed" || b.Status == "frozen" || b.Status == "permitted" {
		return ErrState
	}
	if strings.TrimSpace(itemID) == "" || strings.TrimSpace(reason) == "" || strings.TrimSpace(actor) == "" {
		return ErrInvalid
	}
	if next.Metric == "" || next.Value < 0 || (next.decoded && !next.valuePresent) || strings.TrimSpace(next.CapturedBy) == "" || strings.TrimSpace(next.Unit) == "" || strings.TrimSpace(next.Observation) == "" || len(next.EvidenceRefs) == 0 {
		return ErrInvalid
	}
	evidence := map[string]bool{}
	for _, ref := range next.EvidenceRefs {
		if strings.TrimSpace(ref) == "" || evidence[ref] {
			return ErrInvalid
		}
		evidence[ref] = true
	}
	if next.CapturedAt.Before(b.CreatedAt) || next.CapturedAt.After(now) {
		return ErrInvalid
	}
	for i := range b.Items {
		if b.Items[i].ItemID != itemID {
			continue
		}
		if b.Items[i].Metric != next.Metric {
			return ErrInvalid
		}
		old := b.Items[i]
		old.SupersededBy = old.Revision + 1
		b.ItemHistory = append(b.ItemHistory, old)
		next.ItemID, next.BatchID, next.Revision = itemID, b.BatchID, old.Revision+1
		next.RevisionReason = reason
		b.Items[i] = next
		b.Version++
		return nil
	}
	return ErrNotFound
}

func Evaluate(b *Batch, f *Facility, now time.Time) []Defect {
	return EvaluateWithDefects(b, f, nil, now)
}

func EvaluateWithDefects(b *Batch, f *Facility, existing map[string]*Defect, now time.Time) []Defect {
	var ds []Defect
	resultIndex := map[string]int{}
	liveDefects := map[string]*Defect{}
	b.RiskLevel = "low"
	failedMetrics := map[string]bool{}
	for i := range b.Items {
		it := &b.Items[i]
		sev := ""
		finding := ""
		switch it.Metric {
		case "water_level":
			if it.Value > f.NormalWaterLevel {
				sev = "high"
				finding = "水位超过正常控制线"
			}
		case "gate":
			if it.Value < float64(f.GateCount) {
				sev = "medium"
				finding = "可用闸门数量不足"
			}
		case "pump":
			if it.Value < float64(f.PumpCount) {
				sev = "medium"
				finding = "可用泵组数量不足"
			}
		case "emergency_route":
			if it.Value < 1 {
				sev = "high"
				finding = "应急通道不畅通"
			}
		}
		if sev != "" {
			failedMetrics[it.Metric] = true
			it.RuleResult = "fail"
			if sev == "high" {
				b.RiskLevel = "high"
			} else if b.RiskLevel == "low" {
				b.RiskLevel = "medium"
			}
			if idx, ok := resultIndex[it.Metric]; ok {
				if live := liveDefects[it.Metric]; live != nil {
					live.Severity, live.Finding, live.UpdatedAt = sev, finding, now
					ds[idx] = *live
				} else {
					ds[idx].Severity, ds[idx].Finding, ds[idx].UpdatedAt = sev, finding, now
				}
				continue
			}
			var found *Defect
			for _, id := range b.DefectIDs {
				if d := existing[id]; d != nil && d.Category == it.Metric && d.Status != "closed" {
					found = d
					break
				}
			}
			if found != nil {
				found.Severity, found.Finding, found.UpdatedAt = sev, finding, now
				ds = append(ds, *found)
				liveDefects[it.Metric] = found
			} else {
				id := fmt.Sprintf("%s-D%d", b.BatchID, len(b.DefectIDs)+1)
				d := Defect{DefectID: id, BatchID: b.BatchID, Category: it.Metric, Severity: sev, Finding: finding, Status: "open", CreatedAt: now, UpdatedAt: now}
				ds = append(ds, d)
				b.DefectIDs = append(b.DefectIDs, id)
			}
			resultIndex[it.Metric] = len(ds) - 1
		} else {
			it.RuleResult = "pass"
		}
	}
	for _, id := range b.DefectIDs {
		if d := existing[id]; d != nil && !failedMetrics[d.Category] && d.Status != "closed" {
			d.Status, d.ReviewDecision, d.UpdatedAt = "closed", "auto_resolved", now
			d.AutoClosedAtAssessment = len(b.Assessments) + 1
			for _, item := range b.Items {
				if item.Metric == d.Category {
					d.AutoClosedByRevision = item.Revision
					break
				}
			}
		}
	}
	if len(ds) == 0 {
		b.RiskLevel = "low"
	}
	b.Status = "rectification"
	b.Version++
	rules := make([]AssessmentRule, 0, len(b.Items))
	for _, it := range b.Items {
		r := AssessmentRule{Metric: it.Metric, Observed: it.Value, Result: it.RuleResult}
		switch it.Metric {
		case "water_level":
			r.Threshold, r.Expected = "<= normalWaterLevel", f.NormalWaterLevel
		case "gate":
			r.Threshold, r.Expected = ">= gateCount", float64(f.GateCount)
		case "pump":
			r.Threshold, r.Expected = ">= pumpCount", float64(f.PumpCount)
		case "emergency_route":
			r.Threshold, r.Expected = "= 1", 1
		}
		for _, d := range ds {
			if d.Category == it.Metric {
				r.Severity, r.Finding = d.Severity, d.Finding
			}
		}
		rules = append(rules, r)
	}
	activeIDs := make([]string, 0, len(ds))
	for _, d := range ds {
		activeIDs = append(activeIDs, d.DefectID)
	}
	autoClosed := make([]string, 0)
	autoClosedRevisions := map[string]int{}
	for _, id := range b.DefectIDs {
		if d := existing[id]; d != nil && d.Status == "closed" && d.ReviewDecision == "auto_resolved" && d.AutoClosedAtAssessment == len(b.Assessments)+1 {
			autoClosed = append(autoClosed, id)
			autoClosedRevisions[id] = d.AutoClosedByRevision
		}
	}
	b.Assessments = append(b.Assessments, AssessmentSnapshot{Sequence: len(b.Assessments) + 1, InputRevision: b.Version, Rules: rules, RiskLevel: b.RiskLevel, DefectIDs: activeIDs, AutoClosedDefectIDs: autoClosed, AutoClosedRevisions: autoClosedRevisions, CreatedAt: now})
	return ds
}

func (d *Defect) Assign(assignee string, due *time.Time) error {
	if d.Status == "closed" {
		return ErrState
	}
	if assignee == "" || due == nil {
		return ErrInvalid
	}
	d.Assignee = assignee
	d.DueAt = due
	d.Status = "assigned"
	d.UpdatedAt = time.Now()
	return nil
}
func (d *Defect) Rectify(measures, responsible string, evidence []string, now time.Time) error {
	if strings.TrimSpace(measures) == "" || strings.TrimSpace(responsible) == "" || len(evidence) == 0 {
		return ErrInvalid
	}
	if d.Status != "assigned" && d.Status != "rework" && d.Status != "open" {
		return ErrState
	}
	seen := map[string]bool{}
	for _, ref := range evidence {
		if strings.TrimSpace(ref) == "" || seen[ref] {
			return ErrInvalid
		}
		seen[ref] = true
	}
	for _, old := range d.RectificationRevisions {
		if old.Status != "withdrawn" && sameStrings(old.EvidenceRefs, evidence) {
			return ErrInvalid
		}
	}
	rev := RectificationRevision{Revision: len(d.RectificationRevisions) + 1, Measures: measures, Responsible: responsible, CompletedAt: now, EvidenceRefs: evidence, Result: "submitted", Status: "submitted"}
	d.RectificationRevisions = append(d.RectificationRevisions, rev)
	d.EvidenceRefs = append(d.EvidenceRefs, evidence...)
	d.Status = "ready_review"
	d.UpdatedAt = now
	return nil
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	set := map[string]int{}
	for _, v := range a {
		set[v]++
	}
	for _, v := range b {
		if set[v] == 0 {
			return false
		}
		set[v]--
	}
	for _, n := range set {
		if n != 0 {
			return false
		}
	}
	return true
}

func (d *Defect) ReviewBy(pass bool, comment, reviewer string, revision int, now time.Time) error {
	if d.Status != "ready_review" || len(d.RectificationRevisions) == 0 || revision != len(d.RectificationRevisions) {
		return ErrState
	}
	if strings.TrimSpace(reviewer) == "" || (!pass && strings.TrimSpace(comment) == "") {
		return ErrInvalid
	}
	r := &d.RectificationRevisions[revision-1]
	if r.Status != "submitted" {
		return ErrState
	}
	decision := "pass"
	if !pass {
		decision = "rework"
	}
	r.Status, r.Result, r.ReviewDecision, r.ReviewComment, r.ReviewedBy, r.ReviewedAt = "reviewed", decision, decision, comment, reviewer, &now
	d.ReviewComment, d.ReviewedRevision, d.ReviewDecision = comment, revision, decision
	if pass {
		d.Status = "closed"
	} else {
		d.Status = "rework"
	}
	d.UpdatedAt = now
	return nil
}

func (d *Defect) AssignTo(team, responsible string, due *time.Time, now time.Time) error {
	if d.Status != "open" && d.Status != "rework" {
		return ErrState
	}
	if strings.TrimSpace(team) == "" || strings.TrimSpace(responsible) == "" || due == nil || !due.After(now) {
		return ErrInvalid
	}
	d.MaintenanceTeam, d.Responsible, d.Assignee, d.DueAt, d.Status, d.UpdatedAt = team, responsible, responsible, due, "assigned", now
	return nil
}

func (d *Defect) WithdrawLatest(reason, actor string, now time.Time) error {
	if d.Status == "closed" || strings.TrimSpace(reason) == "" || strings.TrimSpace(actor) == "" || len(d.RectificationRevisions) == 0 {
		return ErrState
	}
	i := len(d.RectificationRevisions) - 1
	r := &d.RectificationRevisions[i]
	if r.Status != "submitted" || r.Responsible != actor {
		return ErrState
	}
	r.Status, r.Result, r.WithdrawnAt, r.WithdrawalReason = "withdrawn", "withdrawn", &now, reason
	d.Status, d.UpdatedAt = "assigned", now
	return nil
}
func (d *Defect) Review(pass bool, comment string, now time.Time) error {
	return d.ReviewBy(pass, comment, "legacy-reviewer", len(d.RectificationRevisions), now)
}

func (b *Batch) Freeze(defects map[string]*Defect, issuedBy string, now time.Time) (*Permit, error) {
	return b.FreezeWithFacility(defects, nil, issuedBy, now)
}

func (b *Batch) FreezeWithFacility(defects map[string]*Defect, facility *Facility, issuedBy string, now time.Time) (*Permit, error) {
	if b.Status != "reviewed" || b.Status == "frozen" || b.Status == "permitted" || issuedBy == "" {
		return nil, ErrState
	}
	if len(b.DefectIDs) > 0 {
		for _, id := range b.DefectIDs {
			d := defects[id]
			if d == nil || d.Status != "closed" {
				return nil, fmt.Errorf("仍有缺陷未通过复核")
			}
		}
	}
	b.Status = "frozen"
	b.FrozenRevision = b.Version
	b.Version++
	batchDefects := make(map[string]*Defect, len(b.DefectIDs))
	for _, id := range b.DefectIDs {
		batchDefects[id] = defects[id]
	}
	frozen := *b
	frozen.Status = "frozen"
	var ff FrozenFacility
	if facility != nil {
		ff = FrozenFacility{FacilityID: facility.FacilityID, FacilityType: facility.FacilityType, DesignCapacity: facility.DesignCapacity, NormalWaterLevel: facility.NormalWaterLevel, GateCount: facility.GateCount, PumpCount: facility.PumpCount, EmergencyRoute: facility.EmergencyRoute, Version: facility.Version}
	}
	payload, _ := json.Marshal(FrozenManifest{Facility: ff, Batch: &frozen, Defects: batchDefects})
	sum := sha256.Sum256(payload)
	hash := hex.EncodeToString(sum[:])
	p := &Permit{PermitID: fmt.Sprintf("permit-%s-%d", b.BatchID, b.FrozenRevision), FacilityID: b.FacilityID, BatchID: b.BatchID, Revision: b.FrozenRevision, RiskLevel: b.RiskLevel, IssuedBy: issuedBy, IssuedAt: now, ValidUntil: now.AddDate(0, 6, 0), ContentHash: hash, FrozenContent: payload, AuditEventID: fmt.Sprintf("audit-%s", b.BatchID)}
	b.Status = "permitted"
	return p, nil
}
