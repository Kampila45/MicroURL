package http

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type OpenAPISpec struct {
	OpenAPI string                 `json:"openapi"`
	Info    OpenAPIInfo            `json:"info"`
	Servers []OpenAPIServer        `json:"servers"`
	Paths   map[string]OpenAPIPath `json:"paths"`
}

type OpenAPIInfo struct {
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

type OpenAPIServer struct {
	URL         string `json:"url"`
	Description string `json:"description"`
}

type OpenAPIPath map[string]OpenAPIOperation

type OpenAPIOperation struct {
	Summary     string                     `json:"summary"`
	Description string                     `json:"description"`
	Tags        []string                   `json:"tags,omitempty"`
	Parameters  []OpenAPIParameter         `json:"parameters,omitempty"`
	RequestBody *OpenAPIRequestBody        `json:"requestBody,omitempty"`
	Responses   map[string]OpenAPIResponse `json:"responses"`
}

type OpenAPIParameter struct {
	Name        string                 `json:"name"`
	In          string                 `json:"in"`
	Required    bool                   `json:"required"`
	Description string                 `json:"description"`
	Schema      map[string]interface{} `json:"schema"`
}

type OpenAPIRequestBody struct {
	Required bool                      `json:"required"`
	Content  map[string]OpenAPIContent `json:"content"`
}

type OpenAPIContent struct {
	Schema map[string]interface{} `json:"schema"`
}

type OpenAPIResponse struct {
	Description string                    `json:"description"`
	Headers     map[string]OpenAPIHeader  `json:"headers,omitempty"`
	Content     map[string]OpenAPIContent `json:"content,omitempty"`
}

type OpenAPIHeader struct {
	Description string                 `json:"description,omitempty"`
	Schema      map[string]interface{} `json:"schema,omitempty"`
}

func (h *Handlers) OpenAPISpec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	spec := OpenAPISpec{
		OpenAPI: "3.1.0",
		Info: OpenAPIInfo{
			Title:       "MicroURL API",
			Version:     "1.0.0",
			Description: "A production-ready URL shortener API built with Go following Hexagonal Architecture principles.",
		},
		Servers: []OpenAPIServer{
			{
				URL:         h.baseURL,
				Description: "API Server",
			},
		},
		Paths: map[string]OpenAPIPath{
			"/health": {
				"get": OpenAPIOperation{
					Summary:     "Health Check",
					Description: "Checks the health status of the API, database, and cache connectivity.",
					Tags:        []string{"Health"},
					Responses: map[string]OpenAPIResponse{
						"200": {
							Description: "Service is healthy or degraded",
							Content: map[string]OpenAPIContent{
								"application/json": {
									Schema: map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"status": map[string]interface{}{
												"type":        "string",
												"enum":        []string{"healthy", "degraded", "unhealthy"},
												"example":     "healthy",
												"description": "Overall service status",
											},
											"message": map[string]interface{}{
												"type":        "string",
												"example":     "All systems operational",
												"description": "Human-readable status message",
											},
											"database": map[string]interface{}{
												"type": "object",
												"properties": map[string]interface{}{
													"status": map[string]interface{}{
														"type":        "string",
														"enum":        []string{"healthy", "unhealthy"},
														"example":     "healthy",
														"description": "Database connection status",
													},
													"message": map[string]interface{}{
														"type":        "string",
														"example":     "Database connection is active and responding",
														"description": "Database status message",
													},
												},
											},
											"cache": map[string]interface{}{
												"type": "object",
												"properties": map[string]interface{}{
													"status": map[string]interface{}{
														"type":        "string",
														"enum":        []string{"healthy", "degraded"},
														"example":     "healthy",
														"description": "Cache connection status",
													},
													"message": map[string]interface{}{
														"type":        "string",
														"example":     "Cache connection is active and responding",
														"description": "Cache status message",
													},
												},
											},
											"timestamp": map[string]interface{}{
												"type":        "string",
												"format":      "date-time",
												"example":     "2025-01-01T12:00:00Z",
												"description": "Current server timestamp",
											},
										},
									},
								},
							},
						},
						"503": {
							Description: "Service is unhealthy",
						},
					},
				},
			},
			"/api/links": {
				"post": OpenAPIOperation{
					Summary:     "Create Short Link",
					Description: "Creates a new short link for the provided URL.",
					Tags:        []string{"Links"},
					RequestBody: &OpenAPIRequestBody{
						Required: true,
						Content: map[string]OpenAPIContent{
							"application/json": {
								Schema: map[string]interface{}{
									"type":     "object",
									"required": []string{"url"},
									"properties": map[string]interface{}{
										"url": map[string]interface{}{
											"type":        "string",
											"format":      "uri",
											"maxLength":   2048,
											"example":     "https://example.com/very/long/url",
											"description": "Public HTTP or HTTPS destination URL to shorten (max 2048 chars)",
										},
										"custom_slug": map[string]interface{}{
											"type":        "string",
											"minLength":   3,
											"maxLength":   64,
											"example":     "portfolio-2026",
											"description": "Optional custom alias (normalized to lowercase [a-z0-9_-])",
										},
										"expires_at": map[string]interface{}{
											"type":        "string",
											"format":      "date-time",
											"example":     "2025-12-31T23:59:59Z",
											"description": "Optional future RFC3339 expiration date for the link",
										},
										"captcha_token": map[string]interface{}{
											"type":        "string",
											"example":     "0.mF_...",
											"description": "Cloudflare Turnstile captcha verification token",
										},
									},
								},
							},
						},
					},
					Responses: map[string]OpenAPIResponse{
						"201": {
							Description: "Short link created successfully",
							Content: map[string]OpenAPIContent{
								"application/json": {
									Schema: map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"id": map[string]interface{}{
												"type":        "string",
												"example":     "1735123456789-abc123xyz",
												"description": "Unique identifier for the link",
											},
											"slug": map[string]interface{}{
												"type":        "string",
												"example":     "aBc123Xy",
												"description": "Short slug for the link",
											},
											"original_url": map[string]interface{}{
												"type":        "string",
												"example":     "https://example.com/very/long/url",
												"description": "The original URL",
											},
											"created_at": map[string]interface{}{
												"type":        "string",
												"format":      "date-time",
												"example":     "2025-01-01T12:00:00Z",
												"description": "Creation timestamp",
											},
											"expires_at": map[string]interface{}{
												"type":        "string",
												"format":      "date-time",
												"example":     "2025-12-31T23:59:59Z",
												"description": "Expiration timestamp (if set)",
											},
											"short_url": map[string]interface{}{
												"type":        "string",
												"example":     "http://localhost:8080/aBc123Xy",
												"description": "Complete short URL",
											},
											"qr_code_url": map[string]interface{}{
												"type":        "string",
												"example":     "http://localhost:8080/api/qr/aBc123Xy",
												"description": "URL to generate QR code for the short link",
											},
										},
									},
								},
							},
						},
						"400": {
							Description: "Invalid request",
						},
						"403": {
							Description: "CAPTCHA verification failed",
						},
						"409": {
							Description: "Slug collision",
						},
						"413": {
							Description: "Request body too large",
						},
						"429": {
							Description: "Rate limit exceeded",
						},
						"500": {
							Description: "Internal server error",
						},
					},
				},
			},
			"/api/links/{slug}/stats": {
				"get": OpenAPIOperation{
					Summary:     "Get Link Click Analytics",
					Description: "Returns aggregate total redirects and daily UTC click buckets (7d or 30d).",
					Tags:        []string{"Analytics"},
					Parameters: []OpenAPIParameter{
						{
							Name:        "slug",
							In:          "path",
							Required:    true,
							Description: "The short link slug or custom alias",
							Schema:      map[string]interface{}{"type": "string", "example": "portfolio-2026"},
						},
						{
							Name:        "range",
							In:          "query",
							Required:    false,
							Description: "Date window: 7d (default) or 30d",
							Schema:      map[string]interface{}{"type": "string", "enum": []string{"7d", "30d"}},
						},
					},
					Responses: map[string]OpenAPIResponse{
						"200": {Description: "Aggregate click statistics"},
						"400": {Description: "Invalid range query parameter"},
						"404": {Description: "Link not found"},
					},
				},
			},
			"/api/qr/{slug}": {
				"get": OpenAPIOperation{
					Summary:     "Generate QR Code",
					Description: "Generates a QR code image (PNG) for the short link associated with the given slug.",
					Tags:        []string{"Links"},
					Parameters: []OpenAPIParameter{
						{
							Name:        "slug",
							In:          "path",
							Required:    true,
							Description: "The short link slug",
							Schema: map[string]interface{}{
								"type":    "string",
								"example": "aBc123Xy",
							},
						},
					},
					Responses: map[string]OpenAPIResponse{
						"200": {
							Description: "QR code image (PNG)",
							Content: map[string]OpenAPIContent{
								"image/png": {
									Schema: map[string]interface{}{
										"type":   "string",
										"format": "binary",
									},
								},
							},
						},
						"400": {
							Description: "Invalid slug format",
						},
						"404": {
							Description: "Link not found",
						},
						"410": {
							Description: "Link expired",
						},
						"500": {
							Description: "Internal server error",
						},
					},
				},
			},
			"/{slug}": {
				"get": OpenAPIOperation{
					Summary:     "Resolve Short Link",
					Description: "Redirects to the original URL associated with the slug.",
					Tags:        []string{"Links"},
					Parameters: []OpenAPIParameter{
						{
							Name:        "slug",
							In:          "path",
							Required:    true,
							Description: "The short link slug",
							Schema: map[string]interface{}{
								"type":    "string",
								"example": "aBc123Xy",
							},
						},
					},
					Responses: map[string]OpenAPIResponse{
						"302": {
							Description: "Redirects to original URL",
							Headers: map[string]OpenAPIHeader{
								"X-Cache": {
									Description: "Whether the redirect was served from the cache (HIT) or read from PostgreSQL (MISS)",
									Schema: map[string]interface{}{
										"type":    "string",
										"enum":    []string{"HIT", "MISS"},
										"example": "HIT",
									},
								},
							},
						},
						"404": {
							Description: "Link not found",
						},
						"410": {
							Description: "Link expired",
						},
					},
				},
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(spec); err != nil {
		http.Error(w, fmt.Sprintf("failed to encode OpenAPI spec: %v", err), http.StatusInternalServerError)
		return
	}
}
