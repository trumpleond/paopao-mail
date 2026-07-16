package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

	"paopao-api/internal/store"
)

// EmailService proxies the upstream GetLastEmails API.
type EmailService struct {
	baseURL    string
	httpClient *http.Client
	accounts   *store.AccountStore
}

func NewEmailService(baseURL string, timeoutSec int, accounts *store.AccountStore) *EmailService {
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	return &EmailService{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
		},
		accounts: accounts,
	}
}

// UpstreamResponse matches query.paopaodw.com shape.
type UpstreamResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// MailItem is one message.
type MailItem struct {
	Date    string `json:"Date"`
	From    string `json:"From"`
	To      string `json:"To"`
	Subject string `json:"Subject"`
	Body    string `json:"Body"`
}

// MailData holds inbox and junk.
type MailData struct {
	Inbox []MailItem `json:"inbox"`
	Junk  []MailItem `json:"junk"`
}

// GetEmailsResult is API-facing payload.
type GetEmailsResult struct {
	Inbox             []MailItem `json:"inbox"`
	Junk              []MailItem `json:"junk"`
	Codes             []string   `json:"codes,omitempty"`
	Raw               any        `json:"raw,omitempty"`
	AccountID         int64      `json:"account_id,omitempty"`
	Email             string     `json:"email,omitempty"`
	AutoDisabled      bool       `json:"auto_disabled,omitempty"`
	AutoDisableReason string     `json:"auto_disable_reason,omitempty"`
}

// ResolvedCred is credential plus pool account metadata when known.
type ResolvedCred struct {
	Credential string
	AccountID  int64
	Email      string
}

// UpstreamError is a non-success business code from the mail provider.
type UpstreamError struct {
	Code              int
	Message           string
	AccountID         int64
	Email             string
	AutoDisabled      bool
	AutoDisableReason string
}

func (e *UpstreamError) Error() string {
	if e.AutoDisabled {
		return fmt.Sprintf("upstream code %d: %s（已自动禁用该账号）", e.Code, e.Message)
	}
	return fmt.Sprintf("upstream code %d: %s", e.Code, e.Message)
}

// IsAuthMissing reports whether the error is upstream "no authorization" (code 201).
func IsAuthMissing(err error) bool {
	var ue *UpstreamError
	return errors.As(err, &ue) && ue.Code == 201
}

// ResolveCredential builds email----password for upstream.
// Prefer account_id; else email as plain address (lookup password); else email already as credential.
func (s *EmailService) ResolveCredential(accountID int64, email, password string) (*ResolvedCred, error) {
	if accountID > 0 {
		a, err := s.accounts.GetByID(accountID)
		if err != nil {
			return nil, err
		}
		return &ResolvedCred{
			Credential: a.Credential(),
			AccountID:  a.ID,
			Email:      a.Email,
		}, nil
	}

	email = strings.TrimSpace(email)
	password = strings.TrimSpace(password)
	if email == "" {
		return nil, fmt.Errorf("%w: email or account_id required", store.ErrInvalidInput)
	}

	// already credential form
	if strings.Contains(email, "----") {
		addr, pwd, ok := store.ParseCredentialLine(email)
		if !ok {
			return nil, fmt.Errorf("%w: invalid credential format", store.ErrInvalidInput)
		}
		rc := &ResolvedCred{Credential: addr + "----" + pwd, Email: addr}
		if a, err := s.accounts.GetByEmail(addr); err == nil {
			rc.AccountID = a.ID
			rc.Email = a.Email
			// prefer pool password if caller only passed credential email part weirdly — use credential as-is
		}
		return rc, nil
	}

	if password != "" {
		rc := &ResolvedCred{Credential: email + "----" + password, Email: email}
		if a, err := s.accounts.GetByEmail(email); err == nil {
			rc.AccountID = a.ID
		}
		return rc, nil
	}

	// lookup in pool
	a, err := s.accounts.GetByEmail(email)
	if err != nil {
		return nil, fmt.Errorf("account not in pool and no password provided: %w", err)
	}
	return &ResolvedCred{
		Credential: a.Credential(),
		AccountID:  a.ID,
		Email:      a.Email,
	}, nil
}

// GetLastEmails calls upstream and optionally extracts codes.
// When upstream returns code 201 (no auth info), the account is auto-disabled if it exists in the pool.
func (s *EmailService) GetLastEmails(ctx context.Context, cred *ResolvedCred, num, boxType int) (*GetEmailsResult, error) {
	if cred == nil || strings.TrimSpace(cred.Credential) == "" {
		return nil, fmt.Errorf("%w: credential required", store.ErrInvalidInput)
	}
	if num <= 0 {
		num = 5
	}
	if boxType <= 0 {
		boxType = 3
	}

	u, err := url.Parse(s.baseURL + "/api/GetLastEmails")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("email", cred.Credential)
	q.Set("clientId", "")
	q.Set("refreshToken", "")
	q.Set("num", fmt.Sprintf("%d", num))
	q.Set("boxType", fmt.Sprintf("%d", boxType))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upstream request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("read upstream body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upstream http %d: %s", resp.StatusCode, truncate(string(body), 300))
	}

	var up UpstreamResponse
	if err := json.Unmarshal(body, &up); err != nil {
		return nil, fmt.Errorf("decode upstream json: %w; body=%s", err, truncate(string(body), 200))
	}

	// code 201: 未找到该邮箱的授权信息 → auto-disable pool account
	if up.Code == 201 {
		ue := &UpstreamError{
			Code:    up.Code,
			Message: up.Message,
			Email:   cred.Email,
		}
		if ue.Email == "" {
			if addr, _, ok := store.ParseCredentialLine(cred.Credential); ok {
				ue.Email = addr
			}
		}
		accountID := cred.AccountID
		if accountID <= 0 && ue.Email != "" {
			if a, err := s.accounts.GetByEmail(ue.Email); err == nil {
				accountID = a.ID
			}
		}
		if accountID > 0 {
			note := fmt.Sprintf("auto-disabled: upstream %d %s", up.Code, up.Message)
			if err := s.accounts.Disable(accountID, note); err == nil {
				ue.AccountID = accountID
				ue.AutoDisabled = true
				ue.AutoDisableReason = note
			} else {
				// still surface upstream error even if disable failed
				ue.AccountID = accountID
				ue.Message = up.Message + "（自动禁用失败: " + err.Error() + "）"
			}
		}
		return nil, ue
	}

	if up.Code != 200 && up.Code != 0 {
		return nil, &UpstreamError{Code: up.Code, Message: up.Message, Email: cred.Email, AccountID: cred.AccountID}
	}

	result := &GetEmailsResult{
		AccountID: cred.AccountID,
		Email:     cred.Email,
	}
	if len(up.Data) > 0 && string(up.Data) != "null" {
		var md MailData
		if err := json.Unmarshal(up.Data, &md); err != nil {
			// keep raw if shape differs
			var raw any
			_ = json.Unmarshal(up.Data, &raw)
			result.Raw = raw
			result.Inbox = []MailItem{}
			result.Junk = []MailItem{}
		} else {
			if md.Inbox == nil {
				md.Inbox = []MailItem{}
			}
			if md.Junk == nil {
				md.Junk = []MailItem{}
			}
			result.Inbox = md.Inbox
			result.Junk = md.Junk
			result.Codes = ExtractCodes(md.Inbox, md.Junk)
		}
	} else {
		result.Inbox = []MailItem{}
		result.Junk = []MailItem{}
	}
	return result, nil
}

var (
	// FB3-PDG style, spaced codes, pure digits 4-8
	reCodeHyphen  = regexp.MustCompile(`\b([A-Z0-9]{2,5}-[A-Z0-9]{2,5})\b`)
	reCodeDigits  = regexp.MustCompile(`(?i)(?:code|验证码|驗證碼|otp|pin)[^\dA-Z]{0,20}([0-9]{4,8})\b`)
	reStandalone6 = regexp.MustCompile(`\b([0-9]{6})\b`)
)

// ExtractCodes pulls likely verification codes from subjects and bodies.
func ExtractCodes(inbox, junk []MailItem) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(c string) {
		c = strings.TrimSpace(c)
		if c == "" {
			return
		}
		if _, ok := seen[c]; ok {
			return
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}

	scan := func(items []MailItem) {
		for _, m := range items {
			text := stripHTML(m.Subject + "\n" + m.Body)
			for _, g := range reCodeHyphen.FindAllStringSubmatch(text, -1) {
				add(g[1])
			}
			for _, g := range reCodeDigits.FindAllStringSubmatch(text, -1) {
				add(g[1])
			}
			// standalone 6-digit only if short plain context (avoid picking dates)
			if len(reCodeDigits.FindAllString(text, -1)) == 0 {
				for _, g := range reStandalone6.FindAllStringSubmatch(text, -1) {
					add(g[1])
				}
			}
		}
	}
	scan(inbox)
	scan(junk)
	return out
}

func stripHTML(s string) string {
	// crude tag strip + entity decode minimal
	var b strings.Builder
	b.Grow(len(s))
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			b.WriteByte(' ')
		case !inTag:
			if unicode.IsSpace(r) {
				b.WriteByte(' ')
			} else {
				b.WriteRune(r)
			}
		}
	}
	// collapse spaces
	return strings.Join(strings.Fields(b.String()), " ")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
