package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"paopao-api/internal/service"
)

// EmailHandler proxies upstream mail fetch.
type EmailHandler struct {
	svc *service.EmailService
}

func NewEmailHandler(svc *service.EmailService) *EmailHandler {
	return &EmailHandler{svc: svc}
}

// GetEmails GET /api/emails
// Query: account_id | email | password | num | boxType
// When upstream returns code 201, the pool account is auto-disabled.
func (h *EmailHandler) GetEmails(c *gin.Context) {
	var accountID int64
	if v := c.Query("account_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			Fail(c, http.StatusBadRequest, 400, "invalid account_id")
			return
		}
		accountID = id
	}

	email := c.Query("email")
	password := c.Query("password")
	num, _ := strconv.Atoi(c.DefaultQuery("num", "2"))
	boxType, _ := strconv.Atoi(c.DefaultQuery("boxType", c.DefaultQuery("box_type", "3")))

	cred, err := h.svc.ResolveCredential(accountID, email, password)
	if err != nil {
		FailErr(c, err)
		return
	}

	res, err := h.svc.GetLastEmails(c.Request.Context(), cred, num, boxType)
	if err != nil {
		var ue *service.UpstreamError
		if errors.As(err, &ue) {
			// 201 = no auth → already auto-disabled when possible
			httpStatus := http.StatusBadGateway
			if ue.Code == 201 {
				httpStatus = http.StatusUnprocessableEntity // 422
			}
			c.JSON(httpStatus, APIResponse{
				Code:    ue.Code,
				Message: err.Error(),
				Data: gin.H{
					"upstream_code":       ue.Code,
					"upstream_message":    ue.Message,
					"account_id":          ue.AccountID,
					"email":               ue.Email,
					"auto_disabled":       ue.AutoDisabled,
					"auto_disable_reason": ue.AutoDisableReason,
				},
			})
			return
		}
		Fail(c, http.StatusBadGateway, 502, err.Error())
		return
	}
	OK(c, res)
}
