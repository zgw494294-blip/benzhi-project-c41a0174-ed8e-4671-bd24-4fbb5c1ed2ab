package httpapi

import (
	"cityflood/internal/domain"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func decode(r *http.Request, v any) error {
	defer r.Body.Close()
	if r.Body == http.NoBody {
		return errors.New("请求体不能为空")
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil {
		return errors.New("JSON格式错误")
	}
	return nil
}

func mapErr(err error) int {
	var validation *domain.ValidationError
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return 404
	case errors.Is(err, domain.ErrConflict):
		return 409
	case errors.Is(err, domain.ErrInvalid):
		return 400
	case errors.Is(err, domain.ErrState):
		return 422
	case errors.Is(err, domain.ErrWindowExpired):
		return 422
	case errors.As(err, &validation):
		return 400
	default:
		return 400
	}
}

func headerVersion(r *http.Request, current int) int {
	if current > 0 {
		return current
	}
	v := strings.Trim(r.Header.Get("If-Match"), "\"")
	n, _ := strconv.Atoi(v)
	return n
}

func fmtID(i uint64) string { return time.Now().UTC().Format("20060102T150405") + "-" + itoa(i) }

func itoa(i uint64) string {
	if i == 0 {
		return "0"
	}
	b := make([]byte, 0, 20)
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
