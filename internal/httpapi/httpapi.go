package httpapi

import (
	"cityflood/internal/application"
	"cityflood/internal/domain"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
)

type API struct {
	app              *application.Service
	requests         uint64
	responseEnvelope map[string]any
}

func New(app *application.Service) *API {
	return &API{app: app, responseEnvelope: make(map[string]any)}
}
func (a *API) Handler() http.Handler                            { return http.HandlerFunc(a.serve) }
func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) { a.serve(w, r) }
func (a *API) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	id := atomic.AddUint64(&a.requests, 1)
	responseID := fmtID(id)
	w.Header().Set("X-Request-ID", responseID)
	path := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(path, "/")
	if len(parts) > 0 && parts[0] == "api" {
		parts = parts[1:]
	}
	var out any
	var err error
	status := http.StatusOK
	run := func(command, aggregate string, request any, success int, fn func() (any, error)) {
		out, status, responseID, err = a.app.ExecuteIdempotent(r.Header.Get("Idempotency-Key"), command, aggregate, responseID, request, success, fn)
	}
	if len(parts) == 2 && parts[0] == "facilities" && parts[1] == "" {
		_ = parts
	}
	switch {
	case r.Method == "POST" && len(parts) == 1 && parts[0] == "facilities":
		var in application.FacilityInput
		err = decode(r, &in)
		if err == nil {
			run("facility.create", in.FacilityID, in, http.StatusCreated, func() (any, error) { return a.app.CreateFacility(in, "") })
		}
	case r.Method == "GET" && len(parts) == 2 && parts[0] == "facilities":
		out, err = a.app.GetFacility(parts[1])
	case (r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH") && len(parts) == 3 && parts[0] == "facilities" && parts[2] == "status":
		var in application.FacilityStatusInput
		err = decode(r, &in)
		if err == nil {
			in.ExpectedVersion = headerVersion(r, in.ExpectedVersion)
			run("facility.status", parts[1], in, http.StatusOK, func() (any, error) { return a.app.ChangeFacilityStatus(parts[1], in) })
		}
	case r.Method == "POST" && len(parts) == 3 && parts[0] == "facilities" && parts[2] == "inspection-batches":
		var in application.BatchInput
		err = decode(r, &in)
		if err == nil {
			if in.WindowStart.IsZero() || in.WindowEnd.IsZero() {
				err = errors.New("windowStart和windowEnd不能为空且必须包含时区")
			} else {
				run("batch.create", in.BatchID, in, http.StatusCreated, func() (any, error) { return a.app.CreateBatch(parts[1], in, "") })
			}
		}
	case r.Method == "GET" && len(parts) == 2 && parts[0] == "inspection-batches":
		out, err = a.app.GetBatchView(parts[1])
	case r.Method == "GET" && len(parts) == 3 && parts[0] == "inspection-batches" && (parts[2] == "assessments" || parts[2] == "risk-assessments"):
		if r.URL.Query().Get("trend") == "true" {
			from, _ := strconv.Atoi(r.URL.Query().Get("from"))
			to, _ := strconv.Atoi(r.URL.Query().Get("to"))
			out, err = a.app.GetAssessmentTrend(parts[1], from, to)
		} else {
			seq, _ := strconv.Atoi(r.URL.Query().Get("sequence"))
			out, err = a.app.GetAssessment(parts[1], seq)
		}
	case r.Method == "GET" && len(parts) == 3 && parts[0] == "inspection-batches" && parts[2] == "assessment-diff":
		from, _ := strconv.Atoi(r.URL.Query().Get("from"))
		to, _ := strconv.Atoi(r.URL.Query().Get("to"))
		if r.URL.Query().Get("trend") == "true" {
			out, err = a.app.GetAssessmentTrend(parts[1], from, to)
		} else {
			out, err = a.app.GetAssessmentDiff(parts[1], from, to)
		}
	case r.Method == "POST" && len(parts) == 3 && parts[0] == "inspection-batches" && parts[2] == "items":
		var in application.ItemsInput
		err = decode(r, &in)
		if err == nil {
			in.ExpectedVersion = headerVersion(r, in.ExpectedVersion)
			run("items.submit", parts[1], in, http.StatusOK, func() (any, error) {
				b, ds, e := a.app.SubmitItems(parts[1], in)
				var report any
				if b != nil {
					report = b.LatestValidationReport
				}
				return map[string]any{"batch": b, "defects": ds, "validationReport": report}, e
			})
		}
	case r.Method == "POST" && len(parts) == 5 && parts[0] == "inspection-batches" && parts[2] == "items" && (parts[4] == "revisions" || parts[4] == "revise"):
		var in application.ItemRevisionInput
		err = decode(r, &in)
		if err == nil {
			in.ItemID = parts[3]
			in.ExpectedVersion = headerVersion(r, in.ExpectedVersion)
			run("item.revise", parts[1], in, http.StatusOK, func() (any, error) {
				b, ds, e := a.app.ReviseItem(parts[1], in)
				var report any
				if b != nil {
					report = b.LatestValidationReport
				}
				return map[string]any{"batch": b, "defects": ds, "validationReport": report}, e
			})
		}
	case r.Method == "POST" && len(parts) == 4 && parts[0] == "inspection-batches" && parts[2] == "items" && parts[3] == "revise":
		var in application.ItemRevisionInput
		err = decode(r, &in)
		if err == nil {
			in.ExpectedVersion = headerVersion(r, in.ExpectedVersion)
			run("item.revise", parts[1], in, http.StatusOK, func() (any, error) {
				b, ds, e := a.app.ReviseItem(parts[1], in)
				var report any
				if b != nil {
					report = b.LatestValidationReport
				}
				return map[string]any{"batch": b, "defects": ds, "validationReport": report}, e
			})
		}
	case r.Method == "POST" && len(parts) == 3 && parts[0] == "inspection-batches" && parts[2] == "defect-assignments":
		var in application.BatchAssignmentInput
		err = decode(r, &in)
		if err == nil {
			in.ExpectedVersion = headerVersion(r, in.ExpectedVersion)
			if in.Action == "adjust_due_dates" || len(in.Adjustments) > 0 {
				run("defects.due_dates", parts[1], in, http.StatusOK, func() (any, error) {
					return a.app.AdjustDefectDueDates(parts[1], application.DueDateAdjustmentsInput{Adjustments: in.Adjustments, ExpectedVersion: in.ExpectedVersion, Actor: in.Actor})
				})
			} else {
				run("defects.assign", parts[1], in, http.StatusOK, func() (any, error) { return a.app.AssignDefects(parts[1], in) })
			}
		}
	case r.Method == "POST" && len(parts) == 4 && parts[0] == "inspection-batches" && parts[2] == "defects" && parts[3] == "due-date-adjustments":
		var in application.DueDateAdjustmentsInput
		err = decode(r, &in)
		if err == nil {
			in.ExpectedVersion = headerVersion(r, in.ExpectedVersion)
			run("defects.due_dates", parts[1], in, http.StatusOK, func() (any, error) { return a.app.AdjustDefectDueDates(parts[1], in) })
		}
	case r.Method == "POST" && len(parts) == 4 && parts[0] == "inspection-batches" && parts[2] == "defects" && parts[3] == "assignments":
		var in application.BatchAssignmentInput
		err = decode(r, &in)
		if err == nil {
			in.ExpectedVersion = headerVersion(r, in.ExpectedVersion)
			if in.Action == "adjust_due_dates" || len(in.Adjustments) > 0 {
				run("defects.due_dates", parts[1], in, http.StatusOK, func() (any, error) {
					return a.app.AdjustDefectDueDates(parts[1], application.DueDateAdjustmentsInput{Adjustments: in.Adjustments, ExpectedVersion: in.ExpectedVersion, Actor: in.Actor})
				})
			} else {
				run("defects.assign", parts[1], in, http.StatusOK, func() (any, error) { return a.app.AssignDefects(parts[1], in) })
			}
		}
	case r.Method == "POST" && len(parts) == 3 && parts[0] == "defects" && parts[2] == "assign":
		var in application.RectifyInput
		err = decode(r, &in)
		if err == nil {
			in.ExpectedVersion = headerVersion(r, in.ExpectedVersion)
			run("defect.assign", parts[1], in, http.StatusOK, func() (any, error) { return a.app.AssignDefect(parts[1], in) })
		}
	case r.Method == "POST" && len(parts) == 3 && parts[0] == "defects" && parts[2] == "rectify":
		var in application.RectifyInput
		err = decode(r, &in)
		if err == nil {
			in.ExpectedVersion = headerVersion(r, in.ExpectedVersion)
			run("defect.rectify", parts[1], in, http.StatusOK, func() (any, error) { return a.app.RectifyDefect(parts[1], in) })
		}
	case r.Method == "POST" && len(parts) == 4 && parts[0] == "defects" && parts[2] == "rectifications" && parts[3] == "withdraw":
		var in application.WithdrawInput
		err = decode(r, &in)
		if err == nil {
			in.ExpectedVersion = headerVersion(r, in.ExpectedVersion)
			run("rectification.withdraw", parts[1], in, http.StatusOK, func() (any, error) { return a.app.WithdrawRectification(parts[1], in) })
		}
	case r.Method == "POST" && len(parts) == 5 && parts[0] == "defects" && parts[2] == "rectifications" && parts[4] == "withdraw":
		var in application.WithdrawInput
		err = decode(r, &in)
		if err == nil {
			if in.Revision == 0 {
				in.Revision, _ = strconv.Atoi(parts[3])
			}
			in.ExpectedVersion = headerVersion(r, in.ExpectedVersion)
			run("rectification.withdraw", parts[1], in, http.StatusOK, func() (any, error) { return a.app.WithdrawRectification(parts[1], in) })
		}
	case r.Method == "POST" && len(parts) == 3 && parts[0] == "inspection-batches" && parts[2] == "review":
		var body struct {
			Decisions       []application.ReviewDecision `json:"decisions"`
			Pass            bool                         `json:"pass"`
			Comment         string                       `json:"comment"`
			ExpectedVersion int                          `json:"expectedVersion"`
			Actor           string                       `json:"actor"`
		}
		err = decode(r, &body)
		if err == nil {
			body.ExpectedVersion = headerVersion(r, body.ExpectedVersion)
			if len(body.Decisions) > 0 {
				in := application.ReviewDecisionsInput{Decisions: body.Decisions, ExpectedVersion: body.ExpectedVersion, Actor: body.Actor}
				run("batch.review", parts[1], body, http.StatusOK, func() (any, error) { return a.app.ReviewDefects(parts[1], in) })
			} else {
				in := application.ReviewInput{Pass: body.Pass, Comment: body.Comment, ExpectedVersion: body.ExpectedVersion, Actor: body.Actor}
				run("batch.review", parts[1], body, http.StatusOK, func() (any, error) { return a.app.ReviewBatch(parts[1], in) })
			}
		}
	case r.Method == "POST" && len(parts) == 3 && parts[0] == "inspection-batches" && parts[2] == "permit":
		var in struct {
			IssuedBy        string `json:"issuedBy"`
			ExpectedVersion int    `json:"expectedVersion"`
		}
		err = decode(r, &in)
		if err == nil {
			in.ExpectedVersion = headerVersion(r, in.ExpectedVersion)
			run("permit.issue", parts[1], in, http.StatusCreated, func() (any, error) { return a.app.IssuePermit(parts[1], in.IssuedBy, in.ExpectedVersion) })
		}
	case r.Method == "GET" && len(parts) == 2 && parts[0] == "permits":
		out, err = a.app.GetPermit(parts[1])
	case r.Method == "GET" && len(parts) == 3 && parts[0] == "permits" && (parts[2] == "verification" || parts[2] == "verify"):
		out, err = a.app.VerifyPermit(parts[1])
	case r.Method == "GET" && len(parts) == 2 && (parts[0] == "idempotency" || parts[0] == "idempotency-keys"):
		out, err = a.app.GetIdempotency(parts[1])
	case r.Method == "GET" && len(parts) == 1 && parts[0] == "audit-events":
		out = a.app.ListEvents()
	default:
		status = http.StatusNotFound
		err = domain.ErrNotFound
	}
	w.Header().Set("X-Request-ID", responseID)
	if err != nil {
		status = mapErr(err)
		a.responseEnvelope["requestID"] = responseID
		a.responseEnvelope["error"] = err.Error()
		delete(a.responseEnvelope, "data")
		delete(a.responseEnvelope, "validationReport")
		var validation *domain.ValidationError
		if errors.As(err, &validation) {
			a.responseEnvelope["validationReport"] = validation.Report
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(a.responseEnvelope)
		return
	}
	a.responseEnvelope["requestID"] = responseID
	a.responseEnvelope["data"] = out
	delete(a.responseEnvelope, "error")
	delete(a.responseEnvelope, "validationReport")
	w.WriteHeader(status)
	w.Header().Set("X-Request-ID", responseID)
	_ = json.NewEncoder(w).Encode(a.responseEnvelope)
}
