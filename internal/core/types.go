// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package core

// LarkBrand represents the Lark platform brand.
// "feishu" targets China-mainland, "lark" targets international.
// Any other string is treated as a custom base URL.
type LarkBrand string

const (
	BrandFeishu LarkBrand = "feishu"
	BrandLark   LarkBrand = "lark"
)

// Endpoints holds resolved endpoint URLs for different Lark services.
type Endpoints struct {
	Open     string // e.g. "https://open.feishu.cn"
	Accounts string // e.g. "https://accounts.feishu.cn"
	MCP      string // e.g. "https://mcp.feishu.cn"
}

// ResolveEndpoints resolves endpoint URLs based on brand.
// If brand is a custom URL (starts with http:// or https://), it will be used as the base URL.
func ResolveEndpoints(brand LarkBrand) Endpoints {
	switch brand {
	case BrandLark:
		return Endpoints{
			Open:     "https://open.larksuite.com",
			Accounts: "https://accounts.larksuite.com",
			MCP:      "https://mcp.larksuite.com",
		}
	case BrandFeishu, "":
		return Endpoints{
			Open:     "https://open.feishu.cn",
			Accounts: "https://accounts.feishu.cn",
			MCP:      "https://mcp.feishu.cn",
		}
	default:
		// Treat as custom base URL for private deployment
		baseURL := string(brand)
		return derivePrivateEndpoints(baseURL)
	}
}

// ResolveOpenBaseURL returns the Open API base URL for the given brand.
func ResolveOpenBaseURL(brand LarkBrand) string {
	return ResolveEndpoints(brand).Open
}

// derivePrivateEndpoints derives endpoint URLs for private deployment.
// It handles two cases:
// 1. If baseURL is like "https://open.example.com", derive accounts.example.com and mcp.example.com
// 2. If baseURL is like "https://example.com", use it for all endpoints (fallback)
func derivePrivateEndpoints(baseURL string) Endpoints {
	// Parse the URL to extract scheme, subdomain, and base domain
	// Example: https://open.xfchat.iflytek.com -> scheme=https, subdomain=open, baseDomain=xfchat.iflytek.com

	// Simple string parsing (avoid importing net/url for this simple case)
	scheme := "https://"
	if len(baseURL) > 7 && baseURL[:7] == "http://" {
		scheme = "http://"
		baseURL = baseURL[7:]
	} else if len(baseURL) > 8 && baseURL[:8] == "https://" {
		scheme = "https://"
		baseURL = baseURL[8:]
	}

	// Remove trailing slash
	if len(baseURL) > 0 && baseURL[len(baseURL)-1] == '/' {
		baseURL = baseURL[:len(baseURL)-1]
	}

	// Check if it starts with "open."
	if len(baseURL) > 5 && baseURL[:5] == "open." {
		baseDomain := baseURL[5:] // e.g., "xfchat.iflytek.com"
		return Endpoints{
			Open:     scheme + "open." + baseDomain,
			Accounts: scheme + "accounts." + baseDomain,
			MCP:      scheme + "mcp." + baseDomain,
		}
	}

	// Fallback: use the same URL for all endpoints
	fullURL := scheme + baseURL
	return Endpoints{
		Open:     fullURL,
		Accounts: fullURL,
		MCP:      fullURL,
	}
}
