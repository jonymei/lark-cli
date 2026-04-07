// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package core

import "testing"

func TestResolveEndpoints_Feishu(t *testing.T) {
	ep := ResolveEndpoints(BrandFeishu)
	if ep.Open != "https://open.feishu.cn" {
		t.Errorf("Open = %q, want feishu.cn", ep.Open)
	}
	if ep.Accounts != "https://accounts.feishu.cn" {
		t.Errorf("Accounts = %q, want feishu.cn", ep.Accounts)
	}
	if ep.MCP != "https://mcp.feishu.cn" {
		t.Errorf("MCP = %q, want feishu.cn", ep.MCP)
	}
}

func TestResolveEndpoints_Lark(t *testing.T) {
	ep := ResolveEndpoints(BrandLark)
	if ep.Open != "https://open.larksuite.com" {
		t.Errorf("Open = %q, want larksuite.com", ep.Open)
	}
	if ep.Accounts != "https://accounts.larksuite.com" {
		t.Errorf("Accounts = %q, want larksuite.com", ep.Accounts)
	}
	if ep.MCP != "https://mcp.larksuite.com" {
		t.Errorf("MCP = %q, want larksuite.com", ep.MCP)
	}
}

func TestResolveEndpoints_EmptyDefaultsToFeishu(t *testing.T) {
	ep := ResolveEndpoints("")
	if ep.Open != "https://open.feishu.cn" {
		t.Errorf("Open = %q, want feishu.cn for empty brand", ep.Open)
	}
}

func TestResolveEndpoints_CustomURL(t *testing.T) {
	tests := []struct {
		name     string
		brand    LarkBrand
		wantOpen string
		wantAcct string
		wantMCP  string
	}{
		{
			name:     "private deployment with open subdomain",
			brand:    "https://open.xfchat.iflytek.com",
			wantOpen: "https://open.xfchat.iflytek.com",
			wantAcct: "https://accounts.xfchat.iflytek.com",
			wantMCP:  "https://mcp.xfchat.iflytek.com",
		},
		{
			name:     "private deployment with open subdomain and trailing slash",
			brand:    "https://open.xfchat.iflytek.com/",
			wantOpen: "https://open.xfchat.iflytek.com",
			wantAcct: "https://accounts.xfchat.iflytek.com",
			wantMCP:  "https://mcp.xfchat.iflytek.com",
		},
		{
			name:     "private deployment with http",
			brand:    "http://open.example.com",
			wantOpen: "http://open.example.com",
			wantAcct: "http://accounts.example.com",
			wantMCP:  "http://mcp.example.com",
		},
		{
			name:     "private deployment without subdomain",
			brand:    "https://lark.example.com",
			wantOpen: "https://lark.example.com",
			wantAcct: "https://lark.example.com",
			wantMCP:  "https://lark.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ep := ResolveEndpoints(tt.brand)
			if ep.Open != tt.wantOpen {
				t.Errorf("Open = %q, want %q", ep.Open, tt.wantOpen)
			}
			if ep.Accounts != tt.wantAcct {
				t.Errorf("Accounts = %q, want %q", ep.Accounts, tt.wantAcct)
			}
			if ep.MCP != tt.wantMCP {
				t.Errorf("MCP = %q, want %q", ep.MCP, tt.wantMCP)
			}
		})
	}
}

func TestResolveOpenBaseURL(t *testing.T) {
	if got := ResolveOpenBaseURL(BrandFeishu); got != "https://open.feishu.cn" {
		t.Errorf("ResolveOpenBaseURL(feishu) = %q", got)
	}
	if got := ResolveOpenBaseURL(BrandLark); got != "https://open.larksuite.com" {
		t.Errorf("ResolveOpenBaseURL(lark) = %q", got)
	}
}
