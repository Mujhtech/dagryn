package response

import (
	"net/http"

	"github.com/go-chi/render"
)

// Response is the base response struct
type Response struct {
	StatusCode int `json:"-"`
}

// ServerResponse represents a standard API response
//
//	@Description	Standard API response wrapper
type ServerResponse struct {
	Response
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message" example:"Operation successful"`
	Error   string      `json:"error,omitempty" example:""`
}

func (res Response) Render(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	render.Status(r, res.StatusCode)
	return nil
}

func BadRequest(w http.ResponseWriter, r *http.Request, err error) error {
	_ = render.Render(w, r, ServerResponse{
		Response: Response{
			StatusCode: http.StatusBadRequest,
		},
		Message: "Bad Request",
		Error:   err.Error(),
	})

	return nil
}

func NotFound(w http.ResponseWriter, r *http.Request, err error) error {
	_ = render.Render(w, r, ServerResponse{
		Response: Response{
			StatusCode: http.StatusNotFound,
		},
		Message: "Not Found",
		Error:   err.Error(),
	})

	return nil
}

func MethodNotAllowed(w http.ResponseWriter, r *http.Request, err error) error {
	_ = render.Render(w, r, ServerResponse{
		Response: Response{
			StatusCode: http.StatusMethodNotAllowed,
		},
		Message: "Method Not Allowed",
		Error:   err.Error(),
	})

	return nil
}

func InternalServerError(w http.ResponseWriter, r *http.Request, err error) error {
	_ = render.Render(w, r, ServerResponse{
		Response: Response{
			StatusCode: http.StatusInternalServerError,
		},
		Message: "Internal Server Error",
		Error:   err.Error(),
	})

	return nil
}

func Ok(w http.ResponseWriter, r *http.Request, message string, data interface{}) error {
	_ = render.Render(w, r, ServerResponse{
		Response: Response{
			StatusCode: http.StatusOK,
		},
		Message: message,
		Data:    data,
	})

	return nil
}

func Created(w http.ResponseWriter, r *http.Request, message string, data interface{}) error {
	_ = render.Render(w, r, ServerResponse{
		Response: Response{
			StatusCode: http.StatusCreated,
		},
		Message: message,
		Data:    data,
	})

	return nil
}

func Unauthorized(w http.ResponseWriter, r *http.Request, err error) error {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	_ = render.Render(w, r, ServerResponse{
		Response: Response{
			StatusCode: http.StatusUnauthorized,
		},
		Message: "Unauthorized",
		Error:   errMsg,
	})

	return nil
}

func Forbidden(w http.ResponseWriter, r *http.Request, err error) error {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	_ = render.Render(w, r, ServerResponse{
		Response: Response{
			StatusCode: http.StatusForbidden,
		},
		Message: "Forbidden",
		Error:   errMsg,
	})

	return nil
}

func TooManyRequests(w http.ResponseWriter, r *http.Request, err error) error {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	_ = render.Render(w, r, ServerResponse{
		Response: Response{
			StatusCode: http.StatusTooManyRequests,
		},
		Message: "Too Many Requests",
		Error:   errMsg,
	})

	return nil
}

func Redirect(w http.ResponseWriter, r *http.Request, url string, status int) error {
	http.Redirect(w, r, url, status)
	return nil
}

func NoContent(w http.ResponseWriter, r *http.Request) error {
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func Conflict(w http.ResponseWriter, r *http.Request, err error) error {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	_ = render.Render(w, r, ServerResponse{
		Response: Response{
			StatusCode: http.StatusConflict,
		},
		Message: "Conflict",
		Error:   errMsg,
	})

	return nil
}

func Gone(w http.ResponseWriter, r *http.Request, err error) error {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	_ = render.Render(w, r, ServerResponse{
		Response: Response{
			StatusCode: http.StatusGone,
		},
		Message: "Gone",
		Error:   errMsg,
	})

	return nil
}

func PaymentRequired(w http.ResponseWriter, r *http.Request, err error) error {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	_ = render.Render(w, r, ServerResponse{
		Response: Response{
			StatusCode: http.StatusPaymentRequired,
		},
		Message: "Quota Exceeded",
		Error:   errMsg,
	})

	return nil
}

func RequestEntityTooLarge(w http.ResponseWriter, r *http.Request, err error) error {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	_ = render.Render(w, r, ServerResponse{
		Response: Response{
			StatusCode: http.StatusRequestEntityTooLarge,
		},
		Message: "Request Entity Too Large",
		Error:   errMsg,
	})

	return nil
}

func ServiceUnavailable(w http.ResponseWriter, r *http.Request, err error) error {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	_ = render.Render(w, r, ServerResponse{
		Response: Response{
			StatusCode: http.StatusServiceUnavailable,
		},
		Message: "Service Unavailable",
		Error:   errMsg,
	})

	return nil
}

func PreconditionRequired(w http.ResponseWriter, r *http.Request, message string, data interface{}) error {
	_ = render.Render(w, r, ServerResponse{
		Response: Response{
			StatusCode: http.StatusPreconditionRequired,
		},
		Message: message,
		Data:    data,
	})

	return nil
}
