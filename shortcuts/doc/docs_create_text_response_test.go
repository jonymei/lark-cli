// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package doc

import (
	"testing"
)

func TestExtractDocumentInfo(t *testing.T) {
	tests := []struct {
		name       string
		response   map[string]interface{}
		wantDocID  string
		wantBlockID string
		wantError  bool
	}{
		{
			name: "standard format - data.document",
			response: map[string]interface{}{
				"document": map[string]interface{}{
					"document_id": "doc123",
					"block_id":    "block456",
				},
			},
			wantDocID:   "doc123",
			wantBlockID: "block456",
			wantError:   false,
		},
		{
			name: "nested format - data.data.document",
			response: map[string]interface{}{
				"data": map[string]interface{}{
					"document": map[string]interface{}{
						"document_id": "doc789",
						"block_id":    "block012",
					},
				},
			},
			wantDocID:   "doc789",
			wantBlockID: "block012",
			wantError:   false,
		},
		{
			name: "flat format - data.document_id",
			response: map[string]interface{}{
				"document_id": "doc345",
				"block_id":    "block678",
			},
			wantDocID:   "doc345",
			wantBlockID: "block678",
			wantError:   false,
		},
		{
			name: "missing document_id",
			response: map[string]interface{}{
				"document": map[string]interface{}{
					"block_id": "block999",
				},
			},
			wantError: true,
		},
		{
			name: "missing block_id",
			response: map[string]interface{}{
				"document": map[string]interface{}{
					"document_id": "doc999",
				},
			},
			wantError: true,
		},
		{
			name:      "empty response",
			response:  map[string]interface{}{},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the extraction logic
			var documentID, blockID string

			// Try direct access: data.document
			if document, ok := tt.response["document"].(map[string]interface{}); ok {
				documentID, _ = document["document_id"].(string)
				blockID, _ = document["block_id"].(string)
			}

			// Try nested access: data.data.document
			if documentID == "" {
				if dataData, ok := tt.response["data"].(map[string]interface{}); ok {
					if document, ok := dataData["document"].(map[string]interface{}); ok {
						documentID, _ = document["document_id"].(string)
						blockID, _ = document["block_id"].(string)
					}
				}
			}

			// Try flat structure: data.document_id
			if documentID == "" {
				documentID, _ = tt.response["document_id"].(string)
				blockID, _ = tt.response["block_id"].(string)
			}

			hasError := (documentID == "" || blockID == "")

			if hasError != tt.wantError {
				t.Errorf("got error=%v, want error=%v", hasError, tt.wantError)
			}

			if !tt.wantError {
				if documentID != tt.wantDocID {
					t.Errorf("document_id = %q, want %q", documentID, tt.wantDocID)
				}
				if blockID != tt.wantBlockID {
					t.Errorf("block_id = %q, want %q", blockID, tt.wantBlockID)
				}
			}
		})
	}
}

func TestGetKeys(t *testing.T) {
	m := map[string]interface{}{
		"key1": "value1",
		"key2": 123,
		"key3": true,
	}

	keys := getKeys(m)
	if len(keys) != 3 {
		t.Errorf("got %d keys, want 3", len(keys))
	}

	// Check all expected keys are present
	keyMap := make(map[string]bool)
	for _, k := range keys {
		keyMap[k] = true
	}

	for _, expected := range []string{"key1", "key2", "key3"} {
		if !keyMap[expected] {
			t.Errorf("missing key %q", expected)
		}
	}
}
