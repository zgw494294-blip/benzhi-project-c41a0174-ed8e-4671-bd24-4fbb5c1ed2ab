package application

import (
	"cityflood/internal/domain"
	"cityflood/internal/storage"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type Service struct {
	store                  *storage.Store
	mu                     sync.Mutex
	idemMu                 sync.Mutex
	idempotencyResultCache map[string]storage.IdempotencyRecord
	now                    func() time.Time
}

func New(s *storage.Store) *Service {
	return &Service{
		store:                  s,
		idempotencyResultCache: make(map[string]storage.IdempotencyRecord),
		now:                    time.Now,
	}
}

func (a *Service) CreateFacility(in FacilityInput, idem string) (*domain.Facility, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if idem != "" {
		var f domain.Facility
		if a.store.GetIdempotent("facility:"+idem, &f) {
			return &f, nil
		}
	}
	st := a.store.Snapshot()
	if _, exists := st.Facilities[in.FacilityID]; exists {
		return nil, domain.ErrConflict
	}
	f, err := domain.NewFacility(in.FacilityID, in.Name, in.District, in.FacilityType, in.DesignCapacity, in.NormalWaterLevel, in.GateCount, in.PumpCount, in.EmergencyRoute, a.now())
	if err != nil {
		return nil, err
	}
	st.Facilities[f.FacilityID] = f
	actor := in.Actor
	if actor == "" {
		actor = "system"
	}
	_ = a.store.Commit("facility.created", f.FacilityID, actor, map[string]any{"name": f.Name, "summary": "创建设施档案"})
	if idem != "" {
		a.store.PutIdempotent("facility:"+idem, f)
	}
	return f, nil
}
func (a *Service) GetFacility(id string) (*domain.Facility, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	st := a.store.Snapshot()
	f, ok := st.Facilities[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	c := *f
	return &c, nil
}
func (a *Service) ChangeFacilityStatus(id string, in FacilityStatusInput) (*domain.Facility, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if in.EffectiveAt.IsZero() {
		in.EffectiveAt = a.now()
	}
	if in.Status == "" {
		in.Status = in.Action
	}
	if in.Status == "suspend" {
		in.Status = "suspended"
	}
	if in.Status == "resume" {
		in.Status = "active"
	}
	st := a.store.Snapshot()
	f, ok := st.Facilities[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	if err := f.ChangeStatus(in.Status, in.Reason, in.Actor, in.ExpectedVersion, in.EffectiveAt); err != nil {
		return nil, err
	}
	if _, err := a.store.CommitEvent("facility.status_changed", id, in.Actor, map[string]any{"status": in.Status, "reason": in.Reason, "effectiveAt": in.EffectiveAt, "version": f.Version}); err != nil {
		return nil, err
	}
	return f, nil
}
func (a *Service) CreateBatch(fid string, in BatchInput, idem string) (*domain.Batch, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	st := a.store.Snapshot()
	facility, ok := st.Facilities[fid]
	if !ok {
		return nil, domain.ErrNotFound
	}
	if facility.Status != "active" {
		return nil, domain.ErrState
	}
	if idem != "" {
		var b domain.Batch
		if a.store.GetIdempotent("batch:"+idem, &b) {
			return &b, nil
		}
	}
	if _, exists := st.Batches[in.BatchID]; exists {
		return nil, domain.ErrConflict
	}
	var b *domain.Batch
	var err error
	if !in.WindowStart.IsZero() || !in.WindowEnd.IsZero() {
		b, err = domain.NewBatchWithWindow(in.BatchID, fid, in.InspectionWindow, in.InspectorID, in.WindowStart, in.WindowEnd, a.now())
	} else {
		b, err = domain.NewBatch(in.BatchID, fid, in.InspectionWindow, in.InspectorID, a.now())
	}
	if err != nil {
		return nil, err
	}
	for _, existing := range st.Batches {
		if existing.FacilityID != fid || existing.Status == "frozen" || existing.Status == "permitted" {
			continue
		}
		if b.WindowStart.IsZero() && existing.WindowStart.IsZero() && existing.InspectionWindow == b.InspectionWindow {
			return nil, fmt.Errorf("%w: 与批次%s检查窗口冲突", domain.ErrConflict, existing.BatchID)
		}
		if !b.WindowStart.IsZero() && !existing.WindowStart.IsZero() && domain.WindowsOverlap(b.WindowStart, b.WindowEnd, existing.WindowStart, existing.WindowEnd) {
			_ = a.store.Commit("batch.window_conflict", b.BatchID, in.InspectorID, map[string]any{"conflictBatchID": existing.BatchID, "windowStart": b.WindowStart, "windowEnd": b.WindowEnd})
			return nil, fmt.Errorf("%w: 与批次%s检查窗口重叠", domain.ErrConflict, existing.BatchID)
		}
	}
	st.Batches[b.BatchID] = b
	_ = a.store.Commit("batch.created", b.BatchID, in.InspectorID, map[string]any{"windowStart": b.WindowStart, "windowEnd": b.WindowEnd, "windowValidation": "passed"})
	if idem != "" {
		a.store.PutIdempotent("batch:"+idem, b)
	}
	return cloneBatch(b), nil
}
func (a *Service) GetBatch(id string) (*domain.Batch, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	st := a.store.Snapshot()
	b, ok := st.Batches[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return b, nil
}
func (a *Service) GetBatchView(id string) (*BatchView, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	st := a.store.Snapshot()
	b, ok := st.Batches[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	view := &BatchView{Batch: cloneBatch(b), Defects: make([]DefectView, 0, len(b.DefectIDs))}
	if f := st.Facilities[b.FacilityID]; f != nil {
		view.FacilityStatus = f.Status
	}
	if len(b.Assessments) > 0 {
		view.LatestAssessment = &b.Assessments[len(b.Assessments)-1]
	}
	now := a.now()
	teams := map[string]*TeamSummary{}
	for _, did := range b.DefectIDs {
		if d := st.Defects[did]; d != nil {
			if d.RefreshEscalation(now) {
				_ = a.store.Commit("defect.escalated", d.DefectID, "system", map[string]any{"escalatedTo": d.EscalatedTo, "escalatedAt": d.EscalatedAt})
			}
			dueStatus := d.CurrentDueStatus(now)
			overdue := dueStatus.Overdue
			view.Defects = append(view.Defects, DefectView{Defect: cloneDefect(d), Overdue: overdue, Warning: dueStatus.Warning, Escalated: dueStatus.Escalated})
			if d.Status == "assigned" {
				view.AssignedCount++
			}
			if d.Status == "ready_review" {
				view.PendingReview++
			}
			if overdue {
				view.OverdueCount++
				view.OverdueDefectIDs = append(view.OverdueDefectIDs, d.DefectID)
			}
			if d.Status == "rework" {
				view.ReworkDefectIDs = append(view.ReworkDefectIDs, d.DefectID)
			}
			if d.MaintenanceTeam != "" && d.Status != "closed" {
				s := teams[d.MaintenanceTeam]
				if s == nil {
					s = &TeamSummary{MaintenanceTeam: d.MaintenanceTeam}
					teams[d.MaintenanceTeam] = s
				}
				s.PendingCount++
				if d.DueAt != nil && (s.NearestDueAt == nil || d.DueAt.Before(*s.NearestDueAt)) {
					t := *d.DueAt
					s.NearestDueAt = &t
				}
			}
		}
	}
	for _, s := range teams {
		view.TeamSummary = append(view.TeamSummary, *s)
	}
	sort.Slice(view.TeamSummary, func(i, j int) bool { return view.TeamSummary[i].MaintenanceTeam < view.TeamSummary[j].MaintenanceTeam })
	return view, nil
}

func (a *Service) GetAssessment(id string, sequence int) (any, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	st := a.store.Snapshot()
	b, ok := st.Batches[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	if sequence <= 0 {
		if len(b.Assessments) == 0 {
			return map[string]any{"assessment": nil}, nil
		}
		return b.Assessments[len(b.Assessments)-1], nil
	}
	for _, s := range b.Assessments {
		if s.Sequence == sequence {
			return s, nil
		}
	}
	return nil, domain.ErrNotFound
}
func cloneBatch(b *domain.Batch) *domain.Batch {
	if b == nil {
		return nil
	}
	raw, _ := json.Marshal(b)
	var c domain.Batch
	_ = json.Unmarshal(raw, &c)
	return &c
}
func cloneDefect(d *domain.Defect) *domain.Defect {
	if d == nil {
		return nil
	}
	raw, _ := json.Marshal(d)
	var c domain.Defect
	_ = json.Unmarshal(raw, &c)
	return &c
}
func (a *Service) GetAssessmentDiff(id string, from, to int) (*AssessmentDiff, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if from > to {
		return nil, fmt.Errorf("%w: from不能大于to", domain.ErrInvalid)
	}
	st := a.store.Snapshot()
	b, ok := st.Batches[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	var x, y *domain.AssessmentSnapshot
	for i := range b.Assessments {
		if b.Assessments[i].Sequence == from {
			x = &b.Assessments[i]
		}
		if b.Assessments[i].Sequence == to {
			y = &b.Assessments[i]
		}
	}
	if x == nil || y == nil {
		return nil, domain.ErrNotFound
	}
	set := func(s []string) map[string]bool {
		m := map[string]bool{}
		for _, v := range s {
			m[v] = true
		}
		return m
	}
	xs, ys := set(x.DefectIDs), set(y.DefectIDs)
	d := &AssessmentDiff{From: from, To: to, RiskFrom: x.RiskLevel, RiskTo: y.RiskLevel}
	for k := range ys {
		if !xs[k] {
			d.Added = append(d.Added, k)
		} else {
			d.Persistent = append(d.Persistent, k)
		}
	}
	for k := range xs {
		if !ys[k] {
			d.Removed = append(d.Removed, k)
		}
	}
	failed := func(s *domain.AssessmentSnapshot) map[string]bool {
		m := map[string]bool{}
		for _, r := range s.Rules {
			if r.Result == "fail" {
				m[r.Metric] = true
			}
		}
		return m
	}
	xf, yf := failed(x), failed(y)
	for k := range yf {
		if !xf[k] {
			d.AddedFailures = append(d.AddedFailures, k)
		} else {
			d.PersistentFailures = append(d.PersistentFailures, k)
		}
	}
	for k := range xf {
		if !yf[k] {
			d.RemovedFailures = append(d.RemovedFailures, k)
		}
	}
	sort.Strings(d.Added)
	sort.Strings(d.Removed)
	sort.Strings(d.Persistent)
	sort.Strings(d.AddedFailures)
	sort.Strings(d.RemovedFailures)
	sort.Strings(d.PersistentFailures)
	return d, nil
}

func (a *Service) GetAssessmentTrend(id string, from, to int) (*domain.TrendSummary, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	b, ok := a.store.Snapshot().Batches[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return domain.BuildAssessmentTrend(b, from, to, a.now())
}

func (a *Service) AdjustDefectDueDates(id string, in DueDateAdjustmentsInput) (*domain.Batch, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	st := a.store.Snapshot()
	b, ok := st.Batches[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	if strings.TrimSpace(in.Actor) == "" || len(in.Adjustments) == 0 {
		return nil, domain.ErrInvalid
	}
	if b.Status == "frozen" || b.Status == "permitted" {
		return nil, domain.ErrState
	}
	if err := b.RequireWindow(a.now()); err != nil {
		return nil, err
	}
	if in.ExpectedVersion > 0 && b.Version != in.ExpectedVersion {
		return nil, domain.ErrConflict
	}
	seen := map[string]bool{}
	now := a.now()
	for _, adj := range in.Adjustments {
		d := st.Defects[adj.DefectID]
		if seen[adj.DefectID] || d == nil || d.BatchID != id {
			return nil, domain.ErrInvalid
		}
		if adj.DueAt == nil || strings.TrimSpace(adj.Reason) == "" || strings.TrimSpace(adj.Responsible) == "" {
			return nil, domain.ErrInvalid
		}
		if d.Status != "assigned" && d.Status != "rework" {
			return nil, fmt.Errorf("%w: 缺陷%s当前状态不可调整期限", domain.ErrState, adj.DefectID)
		}
		seen[adj.DefectID] = true
	}
	for _, adj := range in.Adjustments {
		if err := st.Defects[adj.DefectID].AdjustDueDate(*adj.DueAt, adj.Reason, adj.Responsible, now); err != nil {
			return nil, err
		}
	}
	b.Version++
	_, err := a.store.CommitEvent("defects.due_dates_adjusted", id, in.Actor, map[string]any{"defectIDs": keys(seen), "count": len(seen)})
	return b, err
}
func (a *Service) SubmitItems(id string, in ItemsInput) (*domain.Batch, []domain.Defect, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if strings.TrimSpace(in.Actor) == "" {
		return nil, nil, domain.ErrInvalid
	}
	st := a.store.Snapshot()
	b, ok := st.Batches[id]
	if !ok {
		return nil, nil, domain.ErrNotFound
	}
	if in.ExpectedVersion > 0 && b.Version != in.ExpectedVersion {
		return nil, nil, domain.ErrConflict
	}
	f, ok := st.Facilities[b.FacilityID]
	if !ok {
		return nil, nil, domain.ErrNotFound
	}
	if err := b.RequireWindow(a.now()); err != nil {
		_ = a.store.Commit("batch.window_rejected", id, in.Actor, map[string]any{"operation": "items", "windowEnd": b.WindowEnd})
		return nil, nil, err
	}
	for i := range in.Items {
		if in.Items[i].CapturedAt.IsZero() {
			in.Items[i].CapturedAt = a.now()
		}
	}
	report := domain.ValidateItems(in.Items, f, b, a.now())
	b.LatestValidationReport = &report
	if !report.Valid {
		_ = a.store.Commit("items.validation_failed", id, in.Actor, map[string]any{"report": report})
		return nil, nil, &domain.ValidationError{Report: report}
	}
	if err := b.AddItems(in.Items, a.now()); err != nil {
		return nil, nil, err
	}
	ds := domain.EvaluateWithDefects(b, f, st.Defects, a.now())
	for i := range ds {
		d := ds[i]
		if old := st.Defects[d.DefectID]; old != nil {
			*old = d
		} else {
			st.Defects[d.DefectID] = &d
		}
	}
	_ = a.store.Commit("batch.assessed", id, in.Actor, map[string]any{"riskLevel": b.RiskLevel, "defects": len(ds)})
	return b, ds, nil
}
func (a *Service) ReviseItem(id string, in ItemRevisionInput) (*domain.Batch, []domain.Defect, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if strings.TrimSpace(in.Actor) == "" {
		return nil, nil, domain.ErrInvalid
	}
	if in.Item.Metric == "" && in.Metric != "" && in.Value == nil {
		return nil, nil, domain.ErrInvalid
	}
	st := a.store.Snapshot()
	b, ok := st.Batches[id]
	if !ok {
		return nil, nil, domain.ErrNotFound
	}
	if in.ExpectedVersion > 0 && b.Version != in.ExpectedVersion {
		return nil, nil, domain.ErrConflict
	}
	f := st.Facilities[b.FacilityID]
	if f == nil {
		return nil, nil, domain.ErrNotFound
	}
	if err := b.RequireWindow(a.now()); err != nil {
		_ = a.store.Commit("batch.window_rejected", id, in.Actor, map[string]any{"operation": "revision", "windowEnd": b.WindowEnd})
		return nil, nil, err
	}
	if in.Item.CapturedAt.IsZero() {
		in.Item.CapturedAt = a.now()
	}
	if in.Item.Metric == "" {
		in.Item.Metric = in.Metric
	}
	if in.Value != nil {
		in.Item.Value = *in.Value
	}
	if in.Item.Unit == "" {
		in.Item.Unit = in.Unit
	}
	if in.Item.Observation == "" {
		in.Item.Observation = in.Observation
	}
	if len(in.Item.EvidenceRefs) == 0 {
		in.Item.EvidenceRefs = in.EvidenceRefs
	}
	if in.Item.CapturedBy == "" {
		in.Item.CapturedBy = in.CapturedBy
	}
	if in.Item.ItemID == "" {
		in.Item.ItemID = in.ItemID
	}
	if !in.CapturedAt.IsZero() {
		in.Item.CapturedAt = in.CapturedAt
	}
	in.Item = domain.NormalizeItem(in.Item)
	validation := domain.ValidateItemRevision(in.Item, f, b, a.now())
	b.LatestValidationReport = &validation
	if len(validation.Issues) > 0 {
		_ = a.store.Commit("items.validation_failed", id, in.Actor, map[string]any{"report": validation})
		return nil, nil, &domain.ValidationError{Report: validation}
	}
	if err := b.ReviseItem(in.ItemID, in.Item, in.Reason, in.Actor, a.now()); err != nil {
		return nil, nil, err
	}
	ds := domain.EvaluateWithDefects(b, f, st.Defects, a.now())
	for i := range ds {
		d := ds[i]
		if old := st.Defects[d.DefectID]; old != nil {
			*old = d
		} else {
			st.Defects[d.DefectID] = &d
		}
	}
	_, err := a.store.CommitEvent("item.revised", id, in.Actor, map[string]any{"itemID": in.ItemID, "reason": in.Reason, "assessment": len(b.Assessments)})
	return b, ds, err
}

func (a *Service) AssignDefects(id string, in BatchAssignmentInput) (*domain.Batch, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	st := a.store.Snapshot()
	b, ok := st.Batches[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	if strings.TrimSpace(in.Actor) == "" || len(in.Assignments) == 0 {
		return nil, domain.ErrInvalid
	}
	if b.Status == "frozen" || b.Status == "permitted" {
		return nil, domain.ErrState
	}
	if in.ExpectedVersion > 0 && b.Version != in.ExpectedVersion {
		return nil, domain.ErrConflict
	}
	seen := map[string]bool{}
	now := a.now()
	for _, x := range in.Assignments {
		d := st.Defects[x.DefectID]
		if seen[x.DefectID] || d == nil || d.BatchID != id || strings.TrimSpace(x.MaintenanceTeam) == "" || strings.TrimSpace(x.Responsible) == "" || x.DueAt == nil || !x.DueAt.After(now) {
			return nil, domain.ErrInvalid
		}
		if d.Status != "open" && d.Status != "rework" {
			return nil, domain.ErrState
		}
		seen[x.DefectID] = true
	}
	for _, x := range in.Assignments {
		if err := st.Defects[x.DefectID].AssignTo(x.MaintenanceTeam, x.Responsible, x.DueAt, now); err != nil {
			return nil, err
		}
	}
	b.Version++
	_, err := a.store.CommitEvent("defects.assigned", id, in.Actor, map[string]any{"count": len(in.Assignments), "defectIDs": keys(seen)})
	return b, err
}
func keys(m map[string]bool) []string {
	r := make([]string, 0, len(m))
	for k := range m {
		r = append(r, k)
	}
	sort.Strings(r)
	return r
}
func (a *Service) AssignDefect(id string, in RectifyInput) (*domain.Defect, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	st := a.store.Snapshot()
	d, ok := st.Defects[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	if strings.TrimSpace(in.Actor) == "" {
		return nil, domain.ErrInvalid
	}
	if b := st.Batches[d.BatchID]; b != nil {
		if b.Status == "permitted" || b.Status == "frozen" {
			return nil, domain.ErrState
		}
		if in.ExpectedVersion > 0 && b.Version != in.ExpectedVersion {
			return nil, domain.ErrConflict
		}
		if err := b.RequireWindow(a.now()); err != nil {
			return nil, err
		}
	}
	if err := d.Assign(in.Assignee, in.DueAt); err != nil {
		return nil, err
	}
	if b := st.Batches[d.BatchID]; b != nil {
		b.Version++
	}
	_ = a.store.Commit("defect.assigned", id, in.Actor, map[string]any{"defectID": id, "assignee": in.Assignee, "dueAt": in.DueAt})
	return d, nil
}
func (a *Service) RectifyDefect(id string, in RectifyInput) (*domain.Defect, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	st := a.store.Snapshot()
	d, ok := st.Defects[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	if strings.TrimSpace(in.Actor) == "" {
		return nil, domain.ErrInvalid
	}
	if b := st.Batches[d.BatchID]; b != nil {
		if b.Status == "permitted" || b.Status == "frozen" {
			return nil, domain.ErrState
		}
		if in.ExpectedVersion > 0 && b.Version != in.ExpectedVersion {
			return nil, domain.ErrConflict
		}
	}
	if err := d.Rectify(in.Measures, in.Responsible, in.EvidenceRefs, a.now()); err != nil {
		return nil, err
	}
	if b := st.Batches[d.BatchID]; b != nil {
		b.Version++
		b.Status = "rectification"
	}
	_ = a.store.Commit("defect.rectified", id, in.Actor, map[string]any{"revision": len(d.RectificationRevisions)})
	return d, nil
}
func (a *Service) WithdrawRectification(id string, in WithdrawInput) (*domain.Defect, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	st := a.store.Snapshot()
	d, ok := st.Defects[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	b := st.Batches[d.BatchID]
	if b == nil {
		return nil, domain.ErrNotFound
	}
	if b.Status == "frozen" || b.Status == "permitted" {
		return nil, domain.ErrState
	}
	if err := b.RequireWindow(a.now()); err != nil {
		return nil, err
	}
	if in.ExpectedVersion > 0 && b.Version != in.ExpectedVersion {
		return nil, domain.ErrConflict
	}
	if len(d.RectificationRevisions) == 0 || in.Revision != len(d.RectificationRevisions) {
		return nil, domain.ErrState
	}
	if err := d.WithdrawLatest(in.Reason, in.Actor, a.now()); err != nil {
		return nil, err
	}
	b.Version++
	_, err := a.store.CommitEvent("rectification.withdrawn", id, in.Actor, map[string]any{"revision": in.Revision, "reason": in.Reason})
	return d, err
}

func (a *Service) ReviewDefects(id string, in ReviewDecisionsInput) (*domain.Batch, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	st := a.store.Snapshot()
	b, ok := st.Batches[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	if strings.TrimSpace(in.Actor) == "" || len(in.Decisions) == 0 {
		return nil, domain.ErrInvalid
	}
	if b.Status == "frozen" || b.Status == "permitted" {
		return nil, domain.ErrState
	}
	if err := b.RequireWindow(a.now()); err != nil {
		return nil, err
	}
	if in.ExpectedVersion > 0 && b.Version != in.ExpectedVersion {
		return nil, domain.ErrConflict
	}
	seen := map[string]bool{}
	rework := make([]domain.ReworkEntry, 0)
	closed := make([]string, 0)
	for _, x := range in.Decisions {
		d := st.Defects[x.DefectID]
		if seen[x.DefectID] || d == nil || d.BatchID != id {
			return nil, domain.ErrInvalid
		}
		if d.Status != "ready_review" {
			return nil, domain.ErrState
		}
		if !x.Pass && strings.TrimSpace(x.Comment) == "" {
			return nil, domain.ErrInvalid
		}
		if len(d.RectificationRevisions) == 0 || x.Revision != len(d.RectificationRevisions) {
			return nil, domain.ErrConflict
		}
		seen[x.DefectID] = true
	}
	readyCount := 0
	for _, did := range b.DefectIDs {
		if d := st.Defects[did]; d != nil && d.Status == "ready_review" {
			readyCount++
		}
	}
	if readyCount != len(in.Decisions) {
		return nil, fmt.Errorf("%w: decisions必须覆盖全部待复核缺陷", domain.ErrInvalid)
	}
	now := a.now()
	for _, x := range in.Decisions {
		d := st.Defects[x.DefectID]
		if err := d.ReviewBy(x.Pass, x.Comment, in.Actor, x.Revision, now); err != nil {
			return nil, err
		}
		d.ReviewedRevision = x.Revision
		if x.Pass {
			d.ReviewDecision = "pass"
			closed = append(closed, x.DefectID)
		} else {
			d.ReviewDecision = "rework"
			rework = append(rework, domain.ReworkEntry{DefectID: x.DefectID, Revision: x.Revision, Responsible: d.Responsible, RequiredEvidence: d.EvidenceRefs, Reason: x.Comment})
		}
	}
	allClosed := true
	for _, did := range b.DefectIDs {
		if d := st.Defects[did]; d == nil || d.Status != "closed" {
			allClosed = false
		}
	}
	b.Version++
	if allClosed {
		b.Status = "reviewed"
		b.ReviewedAt = &now
	} else {
		b.Status = "rectification"
	}
	b.ReviewHistory = append(b.ReviewHistory, domain.ReviewAttempt{At: now, Reviewer: in.Actor, Decisions: keys(seen), Accepted: allClosed})
	b.LatestReviewResult = &domain.ReviewResult{At: now, Accepted: allClosed, ClosedDefectIDs: closed, Rework: rework}
	_, err := a.store.CommitEvent("batch.defects_reviewed", id, in.Actor, map[string]any{"decisions": len(in.Decisions), "reviewed": allClosed})
	return b, err
}
func (a *Service) ReviewBatch(id string, in ReviewInput) (*domain.Batch, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	st := a.store.Snapshot()
	b, ok := st.Batches[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	if strings.TrimSpace(in.Actor) == "" {
		return nil, domain.ErrInvalid
	}
	if b.Status == "permitted" || b.Status == "frozen" {
		return nil, domain.ErrState
	}
	if err := b.RequireWindow(a.now()); err != nil {
		return nil, err
	}
	if in.ExpectedVersion > 0 && b.Version != in.ExpectedVersion {
		return nil, domain.ErrConflict
	}
	if in.Pass {
		for _, did := range b.DefectIDs {
			d := st.Defects[did]
			if d == nil {
				return nil, domain.ErrNotFound
			}
			if d.Status == "open" || d.Status == "assigned" || d.Status == "rework" {
				return nil, fmt.Errorf("仍有缺陷未完成整改")
			}
		}
	}
	if !in.Pass && strings.TrimSpace(in.Comment) == "" {
		return nil, domain.ErrInvalid
	}
	for _, did := range b.DefectIDs {
		d := st.Defects[did]
		if d == nil {
			return nil, domain.ErrNotFound
		}
		if d.Status != "ready_review" {
			return nil, domain.ErrState
		}
	}
	for _, did := range b.DefectIDs {
		d := st.Defects[did]
		if d == nil {
			return nil, domain.ErrNotFound
		}
		if err := d.ReviewBy(in.Pass, in.Comment, in.Actor, len(d.RectificationRevisions), a.now()); err != nil {
			return nil, err
		}
	}
	now := a.now()
	b.ReviewedAt = &now
	if in.Pass {
		b.Status = "reviewed"
	} else {
		b.Status = "rectification"
		b.ReviewComment = in.Comment
	}
	b.Version++
	_ = a.store.Commit("batch.reviewed", id, in.Actor, map[string]any{"pass": in.Pass})
	return b, nil
}
func (a *Service) IssuePermit(id, actor string, expected int) (*domain.Permit, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	st := a.store.Snapshot()
	b, ok := st.Batches[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	if expected > 0 && b.Version != expected {
		return nil, domain.ErrConflict
	}
	if actor == "" {
		return nil, domain.ErrInvalid
	}
	p, err := b.FreezeWithFacility(st.Defects, st.Facilities[b.FacilityID], actor, a.now())
	if err != nil {
		return nil, err
	}
	st.Permits[p.PermitID] = p
	eventID, err := a.store.CommitEvent("permit.issued", id, actor, map[string]any{"permitID": p.PermitID, "revision": p.Revision, "contentHash": p.ContentHash})
	if err != nil {
		return nil, err
	}
	p.AuditEventID = eventID
	_ = a.store.Persist()
	return p, nil
}
func (a *Service) GetPermit(id string) (*domain.Permit, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	p, ok := a.store.Snapshot().Permits[id]
	if !ok {
		return nil, fmt.Errorf("%w: permit", domain.ErrNotFound)
	}
	c := *p
	c.FrozenContent = append([]byte(nil), p.FrozenContent...)
	return &c, nil
}

type PermitVerification struct {
	Verified       bool     `json:"verified"`
	Validity       string   `json:"validity"`
	ContentHash    string   `json:"contentHash"`
	RecomputedHash string   `json:"recomputedHash"`
	Failures       []string `json:"failures"`
}

func (a *Service) VerifyPermit(id string) (*PermitVerification, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	st := a.store.Snapshot()
	p, ok := st.Permits[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	v := &PermitVerification{ContentHash: p.ContentHash}
	if len(p.FrozenContent) == 0 {
		v.Failures = append(v.Failures, "frozenContent缺失")
	} else {
		sum := sha256.Sum256(p.FrozenContent)
		v.RecomputedHash = hex.EncodeToString(sum[:])
		if v.RecomputedHash != p.ContentHash {
			v.Failures = append(v.Failures, "contentHash不一致")
		}
	}
	var manifest domain.FrozenManifest
	if len(p.FrozenContent) > 0 && json.Unmarshal(p.FrozenContent, &manifest) != nil {
		v.Failures = append(v.Failures, "frozenContent格式无效")
	}
	if manifest.Batch == nil || manifest.Batch.BatchID != p.BatchID || manifest.Batch.FrozenRevision != p.Revision || manifest.Facility.FacilityID != p.FacilityID {
		v.Failures = append(v.Failures, "冻结清单关联字段不一致")
	}
	if p.PermitID != id || p.BatchID == "" || p.Revision <= 0 {
		v.Failures = append(v.Failures, "许可关联字段不一致")
	}
	found := false
	prev := ""
	for _, e := range st.Events {
		copyEvent := e
		copyEvent.Hash = ""
		raw, _ := json.Marshal(copyEvent)
		sum := sha256.Sum256(raw)
		expectedHash := hex.EncodeToString(sum[:])
		if e.PrevHash != prev || e.Hash != expectedHash {
			v.Failures = append(v.Failures, "审计事件链校验失败")
			break
		}
		prev = e.Hash
		permitID, _ := e.Data["permitID"].(string)
		if e.ID == p.AuditEventID && e.Type == "permit.issued" && e.AggregateID == p.BatchID && permitID == p.PermitID {
			found = true
		}
	}
	if !found {
		v.Failures = append(v.Failures, "auditEventID无法关联")
	}
	now := a.now()
	if now.Before(p.IssuedAt) {
		v.Validity = "not_yet_valid"
	} else if now.After(p.ValidUntil) {
		v.Validity = "expired"
	} else {
		v.Validity = "valid"
	}
	v.Verified = len(v.Failures) == 0
	return v, nil
}
func (a *Service) GetIdempotency(key string) (storage.IdempotencyRecord, error) {
	r, ok := a.store.GetIdempotencyRecord(key)
	if !ok {
		return storage.IdempotencyRecord{}, domain.ErrNotFound
	}
	return r, nil
}
func (a *Service) ExecuteIdempotent(key, command, aggregate, requestID string, request any, status int, fn func() (any, error)) (any, int, string, error) {
	if strings.TrimSpace(key) == "" {
		out, err := fn()
		return out, status, requestID, err
	}
	a.idemMu.Lock()
	defer a.idemMu.Unlock()
	raw, _ := json.Marshal(request)
	sum := sha256.Sum256(raw)
	hash := hex.EncodeToString(sum[:])
	cacheKey := command + ":" + hash
	if cached, ok := a.idempotencyResultCache[cacheKey]; ok {
		var out any
		_ = json.Unmarshal(cached.Result, &out)
		return out, cached.HTTPStatus, cached.RequestID, nil
	}
	if old, ok := a.store.GetIdempotencyRecord(key); ok {
		if old.RequestHash != hash || old.Command != command {
			return nil, 409, requestID, fmt.Errorf("%w: Idempotency-Key请求摘要冲突", domain.ErrConflict)
		}
		if old.Status == "pending" {
			return old, 202, old.RequestID, nil
		}
		var out any
		_ = json.Unmarshal(old.Result, &out)
		return out, old.HTTPStatus, old.RequestID, nil
	}
	pending := storage.IdempotencyRecord{Key: key, Command: command, AggregateID: aggregate, RequestHash: hash, Status: "pending", HTTPStatus: status, RequestID: requestID}
	if err := a.store.PutIdempotencyRecord(pending); err != nil {
		return nil, status, requestID, err
	}
	out, err := fn()
	if err != nil {
		_ = a.store.DeleteIdempotencyRecord(key)
		return nil, status, requestID, err
	}
	result, _ := json.Marshal(out)
	rec := storage.IdempotencyRecord{Key: key, Command: command, AggregateID: aggregate, RequestHash: hash, Status: "completed", HTTPStatus: status, Result: result, RequestID: requestID, CompletedAt: a.now()}
	if err := a.store.PutIdempotencyRecord(rec); err != nil {
		return nil, status, requestID, err
	}
	a.idempotencyResultCache[cacheKey] = rec
	return out, status, requestID, nil
}
func (a *Service) ListEvents() []domain.AuditEvent {
	a.mu.Lock()
	defer a.mu.Unlock()
	events := a.store.Snapshot().Events
	return append([]domain.AuditEvent(nil), events...)
}
