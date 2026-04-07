// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package doc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"

	"github.com/larksuite/cli/internal/output"
	"github.com/larksuite/cli/internal/validate"
	"github.com/larksuite/cli/shortcuts/common"
)

var DocsCreateMarkdown = common.Shortcut{
	Service:     "docs",
	Command:     "+create-markdown",
	Description: "Create a Lark document from Markdown file (OpenAPI import)",
	Risk:        "write",
	AuthTypes:   []string{"user", "bot"},
	Scopes:      []string{"drive:drive:readonly", "drive:drive"},
	Flags: []common.Flag{
		{Name: "file", Desc: "Markdown file path (.md, .markdown, .mark)", Required: true},
		{Name: "title", Desc: "document title (default: filename)"},
		{Name: "folder-token", Desc: "parent folder token (default: root folder)"},
	},
	Validate: func(ctx context.Context, runtime *common.RuntimeContext) error {
		filePath := runtime.Str("file")
		if filePath == "" {
			return common.FlagErrorf("--file is required")
		}
		ext := strings.ToLower(filepath.Ext(filePath))
		if ext != ".md" && ext != ".markdown" && ext != ".mark" {
			return common.FlagErrorf("file must be .md, .markdown, or .mark")
		}
		return nil
	},
	DryRun: func(ctx context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
		filePath := runtime.Str("file")
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filePath)), ".")

		return common.NewDryRunAPI().
			Desc("Step 1: Upload Markdown file").
			POST("/open-apis/drive/v1/medias/upload_all").
			Body(map[string]interface{}{
				"file_name":   filepath.Base(filePath),
				"parent_type": "ccm_import_open",
				"extra":       fmt.Sprintf(`{"obj_type":"docx","file_extension":"%s"}`, ext),
				"file":        "@" + filePath,
			}).
			POST("/open-apis/drive/v1/import_tasks").
			Desc("Step 2: Create import task").
			Body(map[string]interface{}{
				"file_extension": ext,
				"file_token":     "{file_token}",
				"type":           "docx",
			}).
			GET("/open-apis/drive/v1/import_tasks/{ticket}").
			Desc("Step 3: Poll import result")
	},
	Execute: func(ctx context.Context, runtime *common.RuntimeContext) error {
		filePath := runtime.Str("file")
		title := runtime.Str("title")
		folderToken := runtime.Str("folder-token")

		// Validate and get file info
		safeFilePath, err := validate.SafeInputPath(filePath)
		if err != nil {
			return output.ErrValidation("unsafe file path: %s", err)
		}
		filePath = safeFilePath

		info, err := os.Stat(filePath)
		if err != nil {
			return output.ErrValidation("cannot read file: %s", err)
		}
		fileSize := info.Size()
		if fileSize > 20*1024*1024 {
			return output.ErrValidation("file %.1fMB exceeds 20MB limit", float64(fileSize)/1024/1024)
		}

		fileName := filepath.Base(filePath)
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filePath)), ".")
		if title == "" {
			title = strings.TrimSuffix(fileName, filepath.Ext(fileName))
		}

		fmt.Fprintf(runtime.IO().ErrOut, "Uploading Markdown: %s (%s)\n", fileName, common.FormatSize(fileSize))

		// Step 1: Upload Markdown file as temporary import file
		fileToken, err := uploadMarkdownForImport(ctx, runtime, filePath, fileName, ext, fileSize)
		if err != nil {
			return fmt.Errorf("upload failed: %w", err)
		}

		// Get folder token (use root if not specified)
		if folderToken == "" {
			fmt.Fprintf(runtime.IO().ErrOut, "Getting root folder token...\n")
			folderToken, err = getRootFolderToken(ctx, runtime)
			if err != nil {
				return fmt.Errorf("get root folder failed: %w", err)
			}
		}

		fmt.Fprintf(runtime.IO().ErrOut, "Creating import task...\n")

		// Step 2: Create import task
		ticket, err := createImportTask(ctx, runtime, fileToken, ext, title, folderToken)
		if err != nil {
			return fmt.Errorf("create import task failed: %w", err)
		}

		fmt.Fprintf(runtime.IO().ErrOut, "Polling import result (ticket: %s)...\n", ticket)

		// Step 3: Poll import result
		docToken, docURL, err := pollImportResult(ctx, runtime, ticket)
		if err != nil {
			return fmt.Errorf("import failed: %w", err)
		}

		fmt.Fprintf(runtime.IO().ErrOut, "✓ Document created successfully\n")

		result := map[string]interface{}{
			"document_id": docToken,
			"title":       title,
		}
		if docURL != "" {
			result["doc_url"] = docURL
		} else {
			// Fallback to constructed URL
			result["doc_url"] = fmt.Sprintf("https://%s/docx/%s", getDocDomain(runtime), docToken)
		}

		runtime.Out(result, nil)

		return nil
	},
}

// uploadMarkdownForImport uploads a Markdown file for import
func uploadMarkdownForImport(ctx context.Context, runtime *common.RuntimeContext, filePath, fileName, ext string, fileSize int64) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Build SDK Formdata
	fd := larkcore.NewFormdata()
	fd.AddField("file_name", fileName)
	fd.AddField("parent_type", "ccm_import_open") // Special parent_type for import
	fd.AddField("size", fmt.Sprintf("%d", fileSize))
	// extra field specifies the target document type and file extension
	extra := fmt.Sprintf(`{"obj_type":"docx","file_extension":"%s"}`, ext)
	fd.AddField("extra", extra)
	fd.AddFile("file", f)

	apiResp, err := runtime.DoAPI(&larkcore.ApiReq{
		HttpMethod: http.MethodPost,
		ApiPath:    "/open-apis/drive/v1/medias/upload_all",
		Body:       fd,
	}, larkcore.WithFileUpload())
	if err != nil {
		var exitErr *output.ExitError
		if errors.As(err, &exitErr) {
			return "", err
		}
		return "", output.ErrNetwork("upload failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(apiResp.RawBody, &result); err != nil {
		return "", output.Errorf(output.ExitAPI, "api_error", "upload failed: invalid response JSON: %v", err)
	}

	if larkCode := int(common.GetFloat(result, "code")); larkCode != 0 {
		msg, _ := result["msg"].(string)
		return "", output.ErrAPI(larkCode, fmt.Sprintf("upload failed: [%d] %s", larkCode, msg), result["error"])
	}

	data, _ := result["data"].(map[string]interface{})
	fileToken, _ := data["file_token"].(string)
	if fileToken == "" {
		return "", output.Errorf(output.ExitAPI, "api_error", "upload failed: no file_token returned")
	}
	return fileToken, nil
}

// getRootFolderToken gets the root folder token for the current user
func getRootFolderToken(ctx context.Context, runtime *common.RuntimeContext) (string, error) {
	data, err := runtime.CallAPI("GET", "/open-apis/drive/explorer/v2/root_folder/meta", nil, nil)
	if err != nil {
		return "", err
	}

	token, _ := data["token"].(string)
	if token == "" {
		return "", output.Errorf(output.ExitAPI, "api_error", "no root folder token returned")
	}

	return token, nil
}

// createImportTask creates an import task
func createImportTask(ctx context.Context, runtime *common.RuntimeContext, fileToken, ext, title, folderToken string) (string, error) {
	body := map[string]interface{}{
		"file_extension": ext,
		"file_token":     fileToken,
		"type":           "docx",
		"file_name":      title,
		"point": map[string]interface{}{
			"mount_type": 1, // 1 = folder (required)
			"mount_key":  folderToken,
		},
	}

	data, err := runtime.CallAPI("POST", "/open-apis/drive/v1/import_tasks", nil, body)
	if err != nil {
		return "", err
	}

	ticket, _ := data["ticket"].(string)
	if ticket == "" {
		return "", output.Errorf(output.ExitAPI, "api_error", "no ticket returned")
	}

	return ticket, nil
}

// pollImportResult polls the import task result
func pollImportResult(ctx context.Context, runtime *common.RuntimeContext, ticket string) (string, string, error) {
	maxAttempts := 30
	pollInterval := 2 * time.Second

	for i := 0; i < maxAttempts; i++ {
		if i > 0 {
			time.Sleep(pollInterval)
		}

		data, err := runtime.CallAPI("GET", fmt.Sprintf("/open-apis/drive/v1/import_tasks/%s", ticket), nil, nil)
		if err != nil {
			return "", "", err
		}

		result, _ := data["result"].(map[string]interface{})
		if result == nil {
			return "", "", output.Errorf(output.ExitAPI, "api_error", "invalid response: no result")
		}

		jobStatus := int(common.GetFloat(result, "job_status"))
		jobErrorMsg, _ := result["job_error_msg"].(string)

		switch jobStatus {
		case 0: // Success
			token, _ := result["token"].(string)
			if token == "" {
				return "", "", output.Errorf(output.ExitAPI, "api_error", "import succeeded but no token returned")
			}
			url, _ := result["url"].(string)
			return token, url, nil

		case 1, 2: // In progress (1=pending, 2=processing)
			fmt.Fprintf(runtime.IO().ErrOut, "  Import in progress... (%d/%d)\n", i+1, maxAttempts)
			continue

		default: // Failed (other status codes)
			if jobErrorMsg == "" {
				jobErrorMsg = "unknown error"
			}
			// Include full result for debugging
			resultJSON, _ := json.Marshal(result)
			return "", "", output.Errorf(output.ExitAPI, "import_failed", "import failed: %s (full result: %s)", jobErrorMsg, string(resultJSON))
		}
	}

	return "", "", output.Errorf(output.ExitAPI, "timeout", "import timeout after %d attempts", maxAttempts)
}
