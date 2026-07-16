package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"paopao-api/internal/store"
)

// APIResponse is the unified JSON envelope.
type APIResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: data})
}

func Fail(c *gin.Context, httpStatus int, code int, message string) {
	c.JSON(httpStatus, APIResponse{Code: code, Message: message})
}

func FailErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		Fail(c, http.StatusNotFound, 404, err.Error())
	case errors.Is(err, store.ErrNoAvailable):
		Fail(c, http.StatusNotFound, 404, err.Error())
	case errors.Is(err, store.ErrInvalidInput):
		Fail(c, http.StatusBadRequest, 400, err.Error())
	default:
		Fail(c, http.StatusInternalServerError, 500, err.Error())
	}
}
