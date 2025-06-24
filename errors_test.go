package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPError(t *testing.T) {
	tests := []struct {
		name    string
		err     HTTPError
		wantMsg string
	}{
		{
			name:    "error without cause",
			err:     HTTPError{Status: 404, Message: "Not found"},
			wantMsg: "Not found",
		},
		{
			name:    "error with cause",
			err:     HTTPError{Status: 500, Message: "Database error", Err: errors.New("connection failed")},
			wantMsg: "Database error: connection failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("HTTPError.Error() = %v, want %v", got, tt.wantMsg)
			}
		})
	}
}

func TestHandleError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "nil error",
			err:        nil,
			wantStatus: http.StatusOK,
			wantBody:   "",
		},
		{
			name:       "HTTPError",
			err:        NotFound("gameserver"),
			wantStatus: http.StatusNotFound,
			wantBody:   "gameserver not found\n",
		},
		{
			name:       "generic error",
			err:        errors.New("something went wrong"),
			wantStatus: http.StatusInternalServerError,
			wantBody:   "Internal server error\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			HandleError(w, tt.err, "test context")

			if tt.err != nil {
				if w.Code != tt.wantStatus {
					t.Errorf("HandleError() status = %v, want %v", w.Code, tt.wantStatus)
				}
				if w.Body.String() != tt.wantBody {
					t.Errorf("HandleError() body = %v, want %v", w.Body.String(), tt.wantBody)
				}
			}
		})
	}
}

func TestRequireMethod(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		required   string
		shouldFail bool
	}{
		{
			name:       "matching method",
			method:     "POST",
			required:   "POST",
			shouldFail: false,
		},
		{
			name:       "non-matching method",
			method:     "GET",
			required:   "POST",
			shouldFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/", nil)
			err := RequireMethod(req, tt.required)

			if tt.shouldFail && err == nil {
				t.Error("RequireMethod() should have failed but didn't")
			}
			if !tt.shouldFail && err != nil {
				t.Errorf("RequireMethod() failed unexpectedly: %v", err)
			}
		})
	}
}
