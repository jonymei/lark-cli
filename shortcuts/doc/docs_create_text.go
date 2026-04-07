// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package doc

import (
	"context"
	"fmt"
	"strings"

	"github.com/larksuite/cli/internal/validate"
	"github.com/larksuite/cli/shortcuts/common"
)

var DocsCreateText = common.Shortcut{
	Service:     "docs",
	Command:     "+create-text",
	Description: "Create a Lark document with plain text (OpenAPI)",
	Risk:        "write",
	AuthTypes:   []string{"user", "bot"},
	Scopes:      []string{"docx:document:create", "docx:document"},
	Flags: []common.Flag{
		{Name: "title", Desc: "document title"},
		{Name: "text", Desc: "plain text content", Required: true},
		{Name: "folder-token", Desc: "parent folder token"},
	},
	Validate: func(ctx context.Context, runtime *common.RuntimeContext) error {
		if runtime.Str("text") == "" {
			return common.FlagErrorf("--text is required")
		}
		return nil
	},
	DryRun: func(ctx context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
		return common.NewDryRunAPI().
			POST("/open-apis/docx/v1/documents").
			Desc("Create document, then add text blocks").
			Body(map[string]interface{}{
				"title":        runtime.Str("title"),
				"folder_token": runtime.Str("folder-token"),
			}).
			POST("/open-apis/docx/v1/documents/{document_id}/blocks/{block_id}/children").
			Body(map[string]interface{}{"children": "text blocks"})
	},
	Execute: func(ctx context.Context, runtime *common.RuntimeContext) error {
		title := runtime.Str("title")
		text := runtime.Str("text")
		folderToken := runtime.Str("folder-token")

		// Step 1: Create empty document
		createBody := map[string]interface{}{}
		if title != "" {
			createBody["title"] = title
		}
		if folderToken != "" {
			createBody["folder_token"] = folderToken
		}

		createResp, err := runtime.CallAPI("POST", "/open-apis/docx/v1/documents", nil, createBody)
		if err != nil {
			return err
		}

		// Try to extract document info from response
		// Response format may vary: data.document or data.data.document
		var documentID, blockID string

		// Try direct access: data.document
		if document, ok := createResp["document"].(map[string]interface{}); ok {
			documentID, _ = document["document_id"].(string)
			// block_id may not be in response, use document_id as root block
			blockID, _ = document["block_id"].(string)
			if blockID == "" {
				blockID = documentID
			}
		}

		// Try nested access: data.data.document (some private deployments)
		if documentID == "" {
			if dataData, ok := createResp["data"].(map[string]interface{}); ok {
				if document, ok := dataData["document"].(map[string]interface{}); ok {
					documentID, _ = document["document_id"].(string)
					blockID, _ = document["block_id"].(string)
					if blockID == "" {
						blockID = documentID
					}
				}
			}
		}

		// Try flat structure: data.document_id (alternative format)
		if documentID == "" {
			documentID, _ = createResp["document_id"].(string)
			blockID, _ = createResp["block_id"].(string)
			if blockID == "" {
				blockID = documentID
			}
		}

		if documentID == "" {
			// Provide detailed error with actual response structure
			return fmt.Errorf("invalid response: missing document_id. Response keys: %v", getKeys(createResp))
		}

		// Step 2: Add text content as blocks
		blocks := buildTextBlocks(text)
		if len(blocks) > 0 {
			addBlocksBody := map[string]interface{}{
				"children": blocks,
				"index":    0,
			}

			path := fmt.Sprintf("/open-apis/docx/v1/documents/%s/blocks/%s/children",
				validate.EncodePathSegment(documentID),
				validate.EncodePathSegment(blockID))

			_, err = runtime.CallAPI("POST", path, nil, addBlocksBody)
			if err != nil {
				return fmt.Errorf("document created but failed to add content: %w", err)
			}
		}

		runtime.Out(map[string]interface{}{
			"document_id": documentID,
			"block_id":    blockID,
			"title":       title,
			"doc_url":     fmt.Sprintf("https://%s/docx/%s", getDocDomain(runtime), documentID),
		}, nil)

		return nil
	},
}

// buildTextBlocks converts plain text to docx block structure
// Each paragraph becomes a separate text block
func buildTextBlocks(text string) []map[string]interface{} {
	lines := strings.Split(text, "\n")
	blocks := make([]map[string]interface{}, 0, len(lines))

	for _, line := range lines {
		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		block := map[string]interface{}{
			"block_type": 2, // text block
			"text": map[string]interface{}{
				"elements": []map[string]interface{}{
					{
						"text_run": map[string]interface{}{
							"content": line,
						},
					},
				},
				"style": map[string]interface{}{},
			},
		}
		blocks = append(blocks, block)
	}

	return blocks
}

// getDocDomain returns the document domain based on brand
func getDocDomain(runtime *common.RuntimeContext) string {
	if runtime.Config.Brand == "lark" {
		return "larksuite.com"
	}
	// Check if it's a custom domain
	brandStr := string(runtime.Config.Brand)
	if strings.HasPrefix(brandStr, "http://") || strings.HasPrefix(brandStr, "https://") {
		// Extract domain from custom URL
		domain := strings.TrimPrefix(brandStr, "https://")
		domain = strings.TrimPrefix(domain, "http://")
		domain = strings.TrimPrefix(domain, "open.")
		domain = strings.TrimSuffix(domain, "/")
		return domain
	}
	return "feishu.cn"
}

// getKeys returns the keys of a map for debugging
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
