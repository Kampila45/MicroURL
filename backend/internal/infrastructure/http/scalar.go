package http

import (
	"fmt"
	"net/http"
)

func (h *Handlers) ScalarUI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	specURL := fmt.Sprintf("%s/openapi.json", h.baseURL)

	html := fmt.Sprintf(`<!doctype html>
<html>
  <head>
    <title>API Documentation - MicroURL</title>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
  </head>
  <body>
    <script
      id="api-reference"
      data-configuration='{"theme": "purple", "layout": "modern", "spec": {"url": "%s"}}'
      type="application/json"
    ></script>
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
  </body>
</html>`, specURL)

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, html)
}

