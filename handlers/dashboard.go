package handlers

import (
	"net/http"
)

// DashboardHandlers handles dashboard-related HTTP requests
type DashboardHandlers struct {
	*BaseHandlers
}

// NewDashboardHandlers creates new dashboard handlers
func NewDashboardHandlers(base *BaseHandlers) *DashboardHandlers {
	return &DashboardHandlers{
		BaseHandlers: base,
	}
}

// IndexDashboard shows the main dashboard
func (h *DashboardHandlers) IndexDashboard(w http.ResponseWriter, r *http.Request) {
	h.Render(w, r, "dashboard.html", nil)
}