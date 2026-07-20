// Package client — тонкий HTTP-клиент к write-API Pingvera (/api/v1/*).
// Никакой бизнес-логики: провайдер Terraform только сериализует/десериализует
// JSON и прокидывает Bearer-токен. Организация определяется токеном на
// стороне hub, поэтому клиент её никуда не передаёт.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client — авторизованный клиент API Pingvera.
type Client struct {
	endpoint string // база, без хвостового /, напр. https://app.pingvera.ru
	token    string // Bearer pv_...
	http     *http.Client
}

// New создаёт клиент. endpoint нормализуется (обрезается хвостовой /).
func New(endpoint, token string) *Client {
	return &Client{
		endpoint: strings.TrimRight(endpoint, "/"),
		token:    token,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

// APIError — ошибка API с телом ответа (единый формат {"error": "..."}, см.
// internal/api/auth_handlers.go writeErr в основном репозитории).
type APIError struct {
	StatusCode int
	Message    string
	Body       string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("pingvera API: %d %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("pingvera API: %d %s", e.StatusCode, e.Body)
}

// errBody — тело ответа об ошибке в формате {"error": "..."}.
type errBody struct {
	Error string `json:"error"`
}

// do выполняет запрос с Bearer-заголовком, разбирает JSON-ответ в out (может
// быть nil для 204/пустых ответов). Коды >=400 превращаются в *APIError.
func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, rdr)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var eb errBody
		_ = json.Unmarshal(respBody, &eb) // best-effort, тело может быть пустым
		return &APIError{StatusCode: resp.StatusCode, Message: eb.Error, Body: string(respBody)}
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// IsNotFound — true, если ошибка API соответствует 404 (ресурс удалён на
// сервере вне Terraform — вызывающая сторона должна убрать ресурс из state).
func IsNotFound(err error) bool {
	var ae *APIError
	if e, ok := err.(*APIError); ok {
		ae = e
	}
	return ae != nil && ae.StatusCode == http.StatusNotFound
}

// ===== Мониторы (internal/api/monitors.go) =====

// Monitor — зеркало monitorJSON основного репозитория (только поля,
// нужные провайдеру; сверено с internal/api/monitors.go).
type Monitor struct {
	PublicID          string          `json:"public_id"`
	Type              string          `json:"type"`
	Name              string          `json:"name"`
	Target            string          `json:"target"`
	IntervalS         int             `json:"interval_s"`
	Config            json.RawMessage `json:"config,omitempty"`
	Enabled           bool            `json:"enabled"`
	Muted             bool            `json:"muted"`
	FailThreshold     int             `json:"fail_threshold"`
	DegradedLatencyMs int             `json:"degraded_latency_ms"`
	Tags              []string        `json:"tags"`
}

// MonitorCreateInput — тело POST /api/v1/monitors. org в теле не шлём —
// организация определяется токеном (org из тела игнорируется write-путём).
type MonitorCreateInput struct {
	Type              string          `json:"type"`
	Name              string          `json:"name"`
	Target            string          `json:"target"`
	IntervalS         int             `json:"interval_s,omitempty"`
	Config            json.RawMessage `json:"config,omitempty"`
	FailThreshold     int             `json:"fail_threshold,omitempty"`
	DegradedLatencyMs int             `json:"degraded_latency_ms,omitempty"`
	Tags              []string        `json:"tags,omitempty"`
}

// MonitorUpdateInput — тело PATCH /api/v1/monitors/{id}. Указатели —
// «отправить только изменённое», nil-поле сервер не трогает. type/target
// сюда не входят: они RequiresReplace, PATCH их не поддерживает.
type MonitorUpdateInput struct {
	Name              *string          `json:"name,omitempty"`
	Enabled           *bool            `json:"enabled,omitempty"`
	IntervalS         *int             `json:"interval_s,omitempty"`
	Config            *json.RawMessage `json:"config,omitempty"`
	FailThreshold     *int             `json:"fail_threshold,omitempty"`
	DegradedLatencyMs *int             `json:"degraded_latency_ms,omitempty"`
	Muted             *bool            `json:"muted,omitempty"`
}

// CreateMonitor -> POST /api/v1/monitors.
func (c *Client) CreateMonitor(ctx context.Context, in MonitorCreateInput) (*Monitor, error) {
	var out struct {
		Monitor Monitor `json:"monitor"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/v1/monitors", in, &out); err != nil {
		return nil, err
	}
	return &out.Monitor, nil
}

// ListMonitors -> GET /api/v1/monitors. Единичного GET /monitors/{id} у API
// нет — чтение всегда идёт через список (см. FindMonitor).
func (c *Client) ListMonitors(ctx context.Context) ([]Monitor, error) {
	var out struct {
		Monitors []Monitor `json:"monitors"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v1/monitors", nil, &out); err != nil {
		return nil, err
	}
	return out.Monitors, nil
}

// FindMonitor ищет монитор по public_id в списке. found=false, если сервер
// его не вернул (значит удалён вне Terraform).
func (c *Client) FindMonitor(ctx context.Context, publicID string) (*Monitor, bool, error) {
	all, err := c.ListMonitors(ctx)
	if err != nil {
		return nil, false, err
	}
	for i := range all {
		if all[i].PublicID == publicID {
			return &all[i], true, nil
		}
	}
	return nil, false, nil
}

// UpdateMonitor -> PATCH /api/v1/monitors/{id}. 204 No Content — ответ не
// разбираем, вызывающая сторона сама перечитает актуальное состояние (Read).
func (c *Client) UpdateMonitor(ctx context.Context, publicID string, in MonitorUpdateInput) error {
	return c.do(ctx, http.MethodPatch, "/api/v1/monitors/"+publicID, in, nil)
}

// DeleteMonitor -> DELETE /api/v1/monitors/{id} (204).
func (c *Client) DeleteMonitor(ctx context.Context, publicID string) error {
	return c.do(ctx, http.MethodDelete, "/api/v1/monitors/"+publicID, nil, nil)
}

// ===== Статус-страницы (internal/api/statuspages.go) =====

// spMon — элемент monitors[]/hosts[] в ответе API: {public_id, name}.
type spMon struct {
	PublicID string `json:"public_id"`
	Name     string `json:"name"`
}

// StatusPage — зеркало statusPageJSON основного репозитория.
type StatusPage struct {
	PublicID     string  `json:"public_id"`
	Slug         string  `json:"slug"`
	Title        string  `json:"title"`
	Enabled      bool    `json:"enabled"`
	Monitors     []spMon `json:"monitors"`
	Hosts        []spMon `json:"hosts"`
	LogoURL      string  `json:"logo_url"`
	BrandColor   string  `json:"brand_color"`
	HeaderMD     string  `json:"header_md"`
	FooterMD     string  `json:"footer_md"`
	EmbedDomains string  `json:"embed_domains"`
	Timezone     string  `json:"timezone"`
	Theme        string  `json:"theme"`
	CustomDomain string  `json:"custom_domain"`
	DomainStatus string  `json:"domain_status"`
	DomainError  string  `json:"domain_error"`
	DomainCNAME  string  `json:"domain_cname"`
	HasPassword  bool    `json:"has_password"`
}

// MonitorIDs — public_id мониторов из Monitors[] (для маппинга в state-set).
func (p *StatusPage) MonitorIDs() []string { return spIDs(p.Monitors) }

// HostIDs — public_id серверов из Hosts[] (для маппинга в state-set).
func (p *StatusPage) HostIDs() []string { return spIDs(p.Hosts) }

func spIDs(in []spMon) []string {
	out := make([]string, 0, len(in))
	for _, m := range in {
		out = append(out, m.PublicID)
	}
	return out
}

// StatusPageInput — общее тело POST (create) и PUT (update, полная замена
// конфигурации) /api/v1/status-pages{,/{id}}.
type StatusPageInput struct {
	Slug         string   `json:"slug"`
	Title        string   `json:"title"`
	Monitors     []string `json:"monitors"`
	Hosts        []string `json:"hosts"`
	LogoURL      string   `json:"logo_url,omitempty"`
	BrandColor   string   `json:"brand_color,omitempty"`
	HeaderMD     string   `json:"header_md,omitempty"`
	FooterMD     string   `json:"footer_md,omitempty"`
	EmbedDomains string   `json:"embed_domains,omitempty"`
	Timezone     string   `json:"timezone,omitempty"`
	Theme        string   `json:"theme,omitempty"`
	CustomDomain *string  `json:"custom_domain,omitempty"` // PUT: nil=не трогаем, ''=убрать
}

// CreateStatusPage -> POST /api/v1/status-pages.
func (c *Client) CreateStatusPage(ctx context.Context, in StatusPageInput) (*StatusPage, error) {
	var out struct {
		StatusPage StatusPage `json:"status_page"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/v1/status-pages", in, &out); err != nil {
		return nil, err
	}
	return &out.StatusPage, nil
}

// ListStatusPages -> GET /api/v1/status-pages (ключ ответа "status_pages",
// сверено с handleStatusPageList).
func (c *Client) ListStatusPages(ctx context.Context) ([]StatusPage, error) {
	var out struct {
		StatusPages []StatusPage `json:"status_pages"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v1/status-pages", nil, &out); err != nil {
		return nil, err
	}
	return out.StatusPages, nil
}

// FindStatusPage ищет страницу по public_id в списке (единичного GET нет).
func (c *Client) FindStatusPage(ctx context.Context, publicID string) (*StatusPage, bool, error) {
	all, err := c.ListStatusPages(ctx)
	if err != nil {
		return nil, false, err
	}
	for i := range all {
		if all[i].PublicID == publicID {
			return &all[i], true, nil
		}
	}
	return nil, false, nil
}

// UpdateStatusPage -> PUT /api/v1/status-pages/{id} (полная замена).
func (c *Client) UpdateStatusPage(ctx context.Context, publicID string, in StatusPageInput) (*StatusPage, error) {
	var out struct {
		StatusPage StatusPage `json:"status_page"`
	}
	if err := c.do(ctx, http.MethodPut, "/api/v1/status-pages/"+publicID, in, &out); err != nil {
		return nil, err
	}
	return &out.StatusPage, nil
}

// DeleteStatusPage -> DELETE /api/v1/status-pages/{id} (204).
func (c *Client) DeleteStatusPage(ctx context.Context, publicID string) error {
	return c.do(ctx, http.MethodDelete, "/api/v1/status-pages/"+publicID, nil, nil)
}
