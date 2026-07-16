package handler

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"paopao-api/internal/store"
)

// AccountHandler serves account pool APIs.
type AccountHandler struct {
	store *store.AccountStore
}

func NewAccountHandler(s *store.AccountStore) *AccountHandler {
	return &AccountHandler{store: s}
}

// Import POST /api/accounts/import
// Body: plain text lines email----password, or JSON { "text": "...", "overwrite": false }
func (h *AccountHandler) Import(c *gin.Context) {
	overwrite := c.Query("overwrite") == "1" || strings.EqualFold(c.Query("overwrite"), "true")

	ct := c.ContentType()
	var text string

	if strings.Contains(ct, "application/json") {
		var body struct {
			Text      string `json:"text"`
			Lines     string `json:"lines"`
			Overwrite bool   `json:"overwrite"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid json: "+err.Error())
			return
		}
		text = body.Text
		if text == "" {
			text = body.Lines
		}
		if body.Overwrite {
			overwrite = true
		}
	} else {
		b, err := io.ReadAll(io.LimitReader(c.Request.Body, 32<<20))
		if err != nil {
			Fail(c, http.StatusBadRequest, 400, "read body failed")
			return
		}
		text = string(b)
	}

	if strings.TrimSpace(text) == "" {
		Fail(c, http.StatusBadRequest, 400, "empty import body")
		return
	}

	res, err := h.store.ImportLines(text, overwrite)
	if err != nil {
		FailErr(c, err)
		return
	}
	OK(c, res)
}

// Pick POST /api/accounts/pick  body: { "platform": "xai" }
func (h *AccountHandler) Pick(c *gin.Context) {
	var body struct {
		Platform string `json:"platform"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		// also allow query
		body.Platform = c.Query("platform")
	}
	if body.Platform == "" {
		body.Platform = c.Query("platform")
	}
	if strings.TrimSpace(body.Platform) == "" {
		Fail(c, http.StatusBadRequest, 400, "platform required")
		return
	}

	res, err := h.store.PickRandom(body.Platform)
	if err != nil {
		FailErr(c, err)
		return
	}
	OK(c, res)
}

// Mark POST /api/accounts/:id/mark  body: { "platform": "xai" }
func (h *AccountHandler) Mark(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		Fail(c, http.StatusBadRequest, 400, "invalid id")
		return
	}
	var body struct {
		Platform string `json:"platform"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.Platform == "" {
		body.Platform = c.Query("platform")
	}
	if err := h.store.Mark(id, body.Platform); err != nil {
		FailErr(c, err)
		return
	}
	OK(c, gin.H{"account_id": id, "platform": body.Platform, "marked": true})
}

// MarkByEmail POST /api/accounts/mark  body: { "email": "...", "platform": "..." }
func (h *AccountHandler) MarkByEmail(c *gin.Context) {
	var body struct {
		Email    string `json:"email"`
		Platform string `json:"platform"`
		ID       int64  `json:"id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		Fail(c, http.StatusBadRequest, 400, "invalid json")
		return
	}
	if body.ID > 0 {
		if err := h.store.Mark(body.ID, body.Platform); err != nil {
			FailErr(c, err)
			return
		}
		OK(c, gin.H{"account_id": body.ID, "platform": body.Platform, "marked": true})
		return
	}
	if body.Email == "" || body.Platform == "" {
		Fail(c, http.StatusBadRequest, 400, "email and platform required")
		return
	}
	if err := h.store.MarkByEmail(body.Email, body.Platform); err != nil {
		FailErr(c, err)
		return
	}
	OK(c, gin.H{"email": body.Email, "platform": body.Platform, "marked": true})
}

// Unmark POST /api/accounts/:id/unmark
func (h *AccountHandler) Unmark(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		Fail(c, http.StatusBadRequest, 400, "invalid id")
		return
	}
	var body struct {
		Platform string `json:"platform"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.Platform == "" {
		body.Platform = c.Query("platform")
	}
	if err := h.store.Unmark(id, body.Platform); err != nil {
		FailErr(c, err)
		return
	}
	OK(c, gin.H{"account_id": id, "platform": body.Platform, "unmarked": true})
}

// List GET /api/accounts
func (h *AccountHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if page < 1 {
		page = 1
	}
	f := store.ListFilter{
		Platform: c.Query("platform"),
		Unused:   c.Query("unused") == "1" || strings.EqualFold(c.Query("unused"), "true"),
		Status:   c.Query("status"),
		Offset:   (page - 1) * pageSize,
		Limit:    pageSize,
	}
	list, total, err := h.store.List(f)
	if err != nil {
		FailErr(c, err)
		return
	}
	OK(c, gin.H{
		"items":     list,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// Get GET /api/accounts/:id
func (h *AccountHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		Fail(c, http.StatusBadRequest, 400, "invalid id")
		return
	}
	detail, err := h.store.GetByID(id)
	if err != nil {
		FailErr(c, err)
		return
	}
	OK(c, detail)
}

// Update PATCH /api/accounts/:id
func (h *AccountHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		Fail(c, http.StatusBadRequest, 400, "invalid id")
		return
	}
	var body struct {
		Status   *string `json:"status"`
		Note     *string `json:"note"`
		Password *string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		Fail(c, http.StatusBadRequest, 400, "invalid json")
		return
	}
	detail, err := h.store.Update(id, body.Status, body.Note, body.Password)
	if err != nil {
		FailErr(c, err)
		return
	}
	OK(c, detail)
}

// Delete DELETE /api/accounts/:id
func (h *AccountHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		Fail(c, http.StatusBadRequest, 400, "invalid id")
		return
	}
	if err := h.store.Delete(id); err != nil {
		FailErr(c, err)
		return
	}
	OK(c, gin.H{"deleted": true, "id": id})
}

// Stats GET /api/stats
func (h *AccountHandler) Stats(c *gin.Context) {
	st, err := h.store.Stats()
	if err != nil {
		FailErr(c, err)
		return
	}
	OK(c, st)
}
