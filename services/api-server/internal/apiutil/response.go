package apiutil

import "github.com/gin-gonic/gin"

type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details"`
}

func Error(c *gin.Context, status int, code string, message string, details any) {
	if details == nil {
		details = gin.H{}
	}

	c.AbortWithStatusJSON(status, ErrorBody{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}
