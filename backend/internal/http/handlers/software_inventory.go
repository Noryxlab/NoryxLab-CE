package handlers

import (
	"encoding/csv"
	"net/http"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/inventory"
)

func (h Handlers) GetSoftwareInventory(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminModule(w, r, "overview"); !ok {
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(inventory.Raw())
}

// ExportSoftwareInventoryCSV serves the same content as a spreadsheet.
//
// Compliance work happens in spreadsheets. Offering only JSON means somebody
// converts it by hand, and a hand-converted inventory is one nobody ever
// regenerates - so it dates from the day it was made and says so to nobody.
func (h Handlers) ExportSoftwareInventoryCSV(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminModule(w, r, "overview"); !ok {
		return
	}
	document, err := inventory.Parse()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read the software inventory"})
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="noryx-software-inventory.csv"`)

	writer := csv.NewWriter(w)
	defer writer.Flush()
	_ = writer.Write([]string{"name", "version", "licence", "component", "licence source", "role", "upstream"})
	for _, item := range document.Items {
		_ = writer.Write([]string{
			item.Name, item.Version, item.Licence, item.Component,
			item.Origin, item.Role, item.Upstream,
		})
	}
}
