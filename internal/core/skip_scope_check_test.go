// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package core

import "testing"

func TestCliConfig_SkipScopeCheck(t *testing.T) {
	config := CliConfig{
		AppID:          "cli_test",
		AppSecret:      "secret",
		Brand:          BrandFeishu,
		DefaultAs:      "user",
		UserOpenId:     "ou_test",
		UserName:       "test",
		SkipScopeCheck: true,
	}

	if !config.SkipScopeCheck {
		t.Error("SkipScopeCheck should be true")
	}

	config.SkipScopeCheck = false
	if config.SkipScopeCheck {
		t.Error("SkipScopeCheck should be false")
	}
}
