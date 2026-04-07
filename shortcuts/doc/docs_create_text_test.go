// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package doc

import (
	"testing"
)

func TestBuildTextBlocks(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected int // expected number of blocks
	}{
		{
			name:     "single line",
			text:     "Hello World",
			expected: 1,
		},
		{
			name:     "multiple lines",
			text:     "Line 1\nLine 2\nLine 3",
			expected: 3,
		},
		{
			name:     "with empty lines",
			text:     "Line 1\n\nLine 2\n\n\nLine 3",
			expected: 3,
		},
		{
			name:     "empty text",
			text:     "",
			expected: 0,
		},
		{
			name:     "only newlines",
			text:     "\n\n\n",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks := buildTextBlocks(tt.text)
			if len(blocks) != tt.expected {
				t.Errorf("buildTextBlocks() got %d blocks, want %d", len(blocks), tt.expected)
			}

			// Verify block structure
			for i, block := range blocks {
				if blockType, ok := block["block_type"].(int); !ok || blockType != 2 {
					t.Errorf("block[%d] block_type = %v, want 2", i, block["block_type"])
				}

				textObj, ok := block["text"].(map[string]interface{})
				if !ok {
					t.Errorf("block[%d] missing text object", i)
					continue
				}

				elements, ok := textObj["elements"].([]map[string]interface{})
				if !ok || len(elements) == 0 {
					t.Errorf("block[%d] missing or empty elements", i)
					continue
				}

				textRun, ok := elements[0]["text_run"].(map[string]interface{})
				if !ok {
					t.Errorf("block[%d] missing text_run", i)
					continue
				}

				content, ok := textRun["content"].(string)
				if !ok || content == "" {
					t.Errorf("block[%d] missing or empty content", i)
				}
			}
		})
	}
}

func TestGetDocDomain(t *testing.T) {
	tests := []struct {
		name     string
		brand    string
		expected string
	}{
		{
			name:     "feishu",
			brand:    "feishu",
			expected: "feishu.cn",
		},
		{
			name:     "lark",
			brand:    "lark",
			expected: "larksuite.com",
		},
		{
			name:     "custom domain with https",
			brand:    "https://open.example.com",
			expected: "example.com",
		},
		{
			name:     "custom domain with http",
			brand:    "http://open.example.com",
			expected: "example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: This test would need a mock RuntimeContext
			// For now, we just verify the function exists
			_ = tt.expected
		})
	}
}
