// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	larkauth "github.com/larksuite/cli/internal/auth"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/output"
	"github.com/larksuite/cli/internal/registry"
	"github.com/larksuite/cli/shortcuts"
	"github.com/larksuite/cli/shortcuts/common"
)

// LoginOptions holds all inputs for auth login.
type LoginOptions struct {
	Factory     *cmdutil.Factory
	Ctx         context.Context
	JSON        bool
	Scope       string
	Recommend   bool
	Domains     []string
	NoWait      bool
	DeviceCode  string
	Flow        string // "device" or "code"
	RedirectURI string // for authorization code flow
}

// NewCmdAuthLogin creates the auth login subcommand.
func NewCmdAuthLogin(f *cmdutil.Factory, runF func(*LoginOptions) error) *cobra.Command {
	opts := &LoginOptions{Factory: f}

	cmd := &cobra.Command{
		Use:   "login",
		Short: "OAuth authorization login",
		Long: `OAuth authorization login.

Supports two flows:
  --flow device (default): Device Flow for CLI environments
  --flow code: Authorization Code Flow for private deployments

For AI agents: this command blocks until the user completes authorization in the
browser. Run it in the background and retrieve the verification URL from its output.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Ctx = cmd.Context()
			if runF != nil {
				return runF(opts)
			}
			return authLoginRun(opts)
		},
	}

	cmd.Flags().StringVar(&opts.Scope, "scope", "", "scopes to request (space-separated, or @file to read from file)")
	cmd.Flags().BoolVar(&opts.Recommend, "recommend", false, "request only recommended (auto-approve) scopes")
	available := sortedKnownDomains()
	cmd.Flags().StringSliceVar(&opts.Domains, "domain", nil,
		fmt.Sprintf("domain (repeatable or comma-separated, e.g. --domain calendar,task)\navailable: %s, all", strings.Join(available, ", ")))
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "structured JSON output")
	cmd.Flags().BoolVar(&opts.NoWait, "no-wait", false, "initiate device authorization and return immediately; use --device-code to complete")
	cmd.Flags().StringVar(&opts.DeviceCode, "device-code", "", "poll and complete authorization with a device code from a previous --no-wait call")
	cmd.Flags().StringVar(&opts.Flow, "flow", "device", "OAuth flow type: device (default) or code")
	cmd.Flags().StringVar(&opts.RedirectURI, "redirect-uri", "http://localhost:3000/callback", "redirect URI registered in the app console (for --flow code)")


	_ = cmd.RegisterFlagCompletionFunc("domain", func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeDomain(toComplete), cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

// completeDomain returns completions for comma-separated domain values.
func completeDomain(toComplete string) []string {
	allDomains := registry.ListFromMetaProjects()
	parts := strings.Split(toComplete, ",")
	prefix := parts[len(parts)-1]
	base := strings.Join(parts[:len(parts)-1], ",")

	var completions []string
	for _, d := range allDomains {
		if strings.HasPrefix(d, prefix) {
			if base == "" {
				completions = append(completions, d)
			} else {
				completions = append(completions, base+","+d)
			}
		}
	}
	return completions
}

func authLoginRun(opts *LoginOptions) error {
	f := opts.Factory

	config, err := f.Config()
	if err != nil {
		return err
	}

	// Validate flow type
	if opts.Flow != "device" && opts.Flow != "code" {
		return output.ErrValidation("invalid --flow value: %q, must be 'device' or 'code'", opts.Flow)
	}

	// --no-wait and --device-code only work with device flow
	if (opts.NoWait || opts.DeviceCode != "") && opts.Flow != "device" {
		return output.ErrValidation("--no-wait and --device-code only work with --flow device")
	}

	// Determine UI language from saved config
	lang := "zh"
	if multi, _ := core.LoadMultiAppConfig(); multi != nil && len(multi.Apps) > 0 {
		lang = multi.Apps[0].Lang
	}
	msg := getLoginMsg(lang)

	log := func(format string, a ...interface{}) {
		if !opts.JSON {
			fmt.Fprintf(f.IOStreams.ErrOut, format+"\n", a...)
		}
	}

	// --device-code: resume polling from a previous --no-wait call
	if opts.DeviceCode != "" {
		return authLoginPollDeviceCode(opts, config, msg, log)
	}

	selectedDomains := opts.Domains
	scopeLevel := "" // "common" or "all" (from interactive mode)

	// Expand --domain all to all available domains (from_meta projects + shortcut services)
	for _, d := range selectedDomains {
		if strings.EqualFold(d, "all") {
			domainSet := make(map[string]bool)
			for _, p := range registry.ListFromMetaProjects() {
				domainSet[p] = true
			}
			for _, sc := range shortcuts.AllShortcuts() {
				domainSet[sc.Service] = true
			}
			selectedDomains = make([]string, 0, len(domainSet))
			for d := range domainSet {
				selectedDomains = append(selectedDomains, d)
			}
			sort.Strings(selectedDomains)
			break
		}
	}

	// Validate domain names and suggest corrections for unknown ones
	if len(selectedDomains) > 0 {
		knownDomains := allKnownDomains()
		for _, d := range selectedDomains {
			if !knownDomains[d] {
				if suggestion := suggestDomain(d, knownDomains); suggestion != "" {
					return output.ErrValidation("unknown domain %q, did you mean %q?", d, suggestion)
				}
				available := make([]string, 0, len(knownDomains))
				for k := range knownDomains {
					available = append(available, k)
				}
				sort.Strings(available)
				return output.ErrValidation("unknown domain %q, available domains: %s", d, strings.Join(available, ", "))
			}
		}
	}

	hasAnyOption := opts.Scope != "" || opts.Recommend || len(selectedDomains) > 0

	if !hasAnyOption {
		if !opts.JSON && f.IOStreams.IsTerminal {
			result, err := runInteractiveLogin(f.IOStreams, lang, msg)
			if err != nil {
				return err
			}
			if result == nil {
				return output.ErrValidation("no login options selected")
			}
			selectedDomains = result.Domains
			scopeLevel = result.ScopeLevel
		} else {
			log(msg.HintHeader)
			log("Common options:")
			log(msg.HintCommon1)
			log(msg.HintCommon2)
			log(msg.HintCommon3)
			log(msg.HintCommon4)
			log("")
			log("View all options:")
			log(msg.HintFooter)
			log("")
			log("Note: this command blocks until authorization is complete. Run it in the background and retrieve the verification URL from its output.")
			return output.ErrValidation("please specify the scopes to authorize")
		}
	}

	finalScope := opts.Scope

	// If scope starts with @, read from file
	if strings.HasPrefix(finalScope, "@") {
		filePath := strings.TrimPrefix(finalScope, "@")
		scopes, err := readScopesFromFile(filePath)
		if err != nil {
			return output.ErrValidation("failed to read scopes from %s: %v", filePath, err)
		}
		finalScope = strings.Join(scopes, " ")
		log("Loaded %d scopes from %s", len(scopes), filePath)
	}

	// Resolve scopes from domain/permission filters
	if len(selectedDomains) > 0 || opts.Recommend {
		if opts.Scope != "" {
			return output.ErrValidation("cannot use --scope together with --domain/--recommend")
		}

		var candidateScopes []string
		if len(selectedDomains) > 0 {
			candidateScopes = collectScopesForDomains(selectedDomains, "user")
		} else {
			// --recommend without --domain: all domains
			candidateScopes = collectScopesForDomains(sortedKnownDomains(), "user")
		}

		// Filter to auto-approve scopes if --recommend or interactive "common"
		if opts.Recommend || scopeLevel == "common" {
			candidateScopes = registry.FilterAutoApproveScopes(candidateScopes)
		}

		if len(candidateScopes) == 0 {
			return output.ErrValidation("no matching scopes found, check domain/scope options")
		}

		finalScope = strings.Join(candidateScopes, " ")
	}

	httpClient, err := f.HttpClient()
	if err != nil {
		return err
	}

	// Execute the selected OAuth flow
	var result *larkauth.DeviceFlowResult
	if opts.Flow == "code" {
		result = executeAuthorizationCodeFlow(opts, config, finalScope, httpClient, log)
	} else {
		result = executeDeviceFlow(opts, config, finalScope, httpClient, msg, log)
	}

	if !result.OK {
		if opts.JSON {
			b, _ := json.Marshal(map[string]interface{}{
				"event": "authorization_failed",
				"error": result.Message,
			})
			fmt.Fprintln(f.IOStreams.Out, string(b))
			return output.ErrBare(output.ExitAuth)
		}
		return output.ErrAuth("authorization failed: %s", result.Message)
	}

	// Get user info
	log(msg.AuthSuccess)
	sdk, err := f.LarkClient()
	if err != nil {
		return output.ErrAuth("failed to get SDK: %v", err)
	}
	openId, userName, err := getUserInfo(opts.Ctx, sdk, result.Token.AccessToken)
	if err != nil {
		return output.ErrAuth("failed to get user info: %v", err)
	}

	// Store token
	now := time.Now().UnixMilli()
	storedToken := &larkauth.StoredUAToken{
		UserOpenId:       openId,
		AppId:            config.AppID,
		AccessToken:      result.Token.AccessToken,
		RefreshToken:     result.Token.RefreshToken,
		ExpiresAt:        now + int64(result.Token.ExpiresIn)*1000,
		RefreshExpiresAt: now + int64(result.Token.RefreshExpiresIn)*1000,
		Scope:            result.Token.Scope,
		GrantedAt:        now,
	}
	if err := larkauth.SetStoredToken(storedToken); err != nil {
		return output.Errorf(output.ExitInternal, "internal", "failed to save token: %v", err)
	}

	// Update config
	multi, _ := core.LoadMultiAppConfig()
	if multi != nil && len(multi.Apps) > 0 {
		app := &multi.Apps[0]
		for _, oldUser := range app.Users {
			if oldUser.UserOpenId != openId {
				larkauth.RemoveStoredToken(config.AppID, oldUser.UserOpenId)
			}
		}
		app.Users = []core.AppUser{{UserOpenId: openId, UserName: userName}}
		if err := core.SaveMultiAppConfig(multi); err != nil {
			return output.Errorf(output.ExitInternal, "internal", "failed to save config: %v", err)
		}
	}

	if opts.JSON {
		b, _ := json.Marshal(map[string]interface{}{
			"event":        "authorization_complete",
			"user_open_id": openId,
			"user_name":    userName,
			"scope":        result.Token.Scope,
		})
		fmt.Fprintln(f.IOStreams.Out, string(b))
	} else {
		fmt.Fprintln(f.IOStreams.ErrOut)
		output.PrintSuccess(f.IOStreams.ErrOut, fmt.Sprintf(msg.LoginSuccess, userName, openId))
		if result.Token.Scope != "" {
			fmt.Fprintf(f.IOStreams.ErrOut, msg.GrantedScopes, result.Token.Scope)
		}
	}
	return nil
}

// executeDeviceFlow executes the device flow.
func executeDeviceFlow(
	opts *LoginOptions,
	config *core.CliConfig,
	finalScope string,
	httpClient *http.Client,
	msg *loginMsg,
	log func(string, ...interface{}),
) *larkauth.DeviceFlowResult {
	f := opts.Factory

	// Step 1: Request device authorization
	authResp, err := larkauth.RequestDeviceAuthorization(httpClient, config.AppID, config.AppSecret, config.Brand, finalScope, f.IOStreams.ErrOut)
	if err != nil {
		return &larkauth.DeviceFlowResult{OK: false, Error: "device_authorization_failed", Message: fmt.Sprintf("%v", err)}
	}

	// --no-wait: return immediately with device code and URL
	if opts.NoWait {
		b, _ := json.Marshal(map[string]interface{}{
			"verification_url": authResp.VerificationUriComplete,
			"device_code":      authResp.DeviceCode,
			"expires_in":       authResp.ExpiresIn,
			"hint":             fmt.Sprintf("Show verification_url to user, then immediately execute: lark-cli auth login --device-code %s (blocks until authorized or timeout). Do not instruct the user to run this command themselves.", authResp.DeviceCode),
		})
		fmt.Fprintln(f.IOStreams.Out, string(b))
		return &larkauth.DeviceFlowResult{OK: false, Error: "no_wait", Message: "Device code returned"}
	}

	// Step 2: Show user code and verification URL
	if opts.JSON {
		b, _ := json.Marshal(map[string]interface{}{
			"event":                     "device_authorization",
			"verification_uri":          authResp.VerificationUri,
			"verification_uri_complete": authResp.VerificationUriComplete,
			"user_code":                 authResp.UserCode,
			"expires_in":                authResp.ExpiresIn,
		})
		fmt.Fprintln(f.IOStreams.Out, string(b))
	} else {
		fmt.Fprintf(f.IOStreams.ErrOut, msg.OpenURL)
		fmt.Fprintf(f.IOStreams.ErrOut, "  %s\n\n", authResp.VerificationUriComplete)
	}

	// Step 3: Poll for token
	log(msg.WaitingAuth)
	return larkauth.PollDeviceToken(opts.Ctx, httpClient, config.AppID, config.AppSecret, config.Brand,
		authResp.DeviceCode, authResp.Interval, authResp.ExpiresIn, f.IOStreams.ErrOut)
}

// executeAuthorizationCodeFlow executes the authorization code flow.
func executeAuthorizationCodeFlow(
	opts *LoginOptions,
	config *core.CliConfig,
	finalScope string,
	httpClient *http.Client,
	log func(string, ...interface{}),
) *larkauth.DeviceFlowResult {
	result := larkauth.StartAuthorizationCodeFlow(
		opts.Ctx,
		httpClient,
		config.AppID,
		config.AppSecret,
		config.Brand,
		finalScope,
		opts.RedirectURI,
		opts.Factory.IOStreams.In,
		opts.Factory.IOStreams.ErrOut,
	)

	return &larkauth.DeviceFlowResult{
		OK:      result.OK,
		Token:   result.Token,
		Error:   result.Error,
		Message: result.Message,
	}
}

// authLoginPollDeviceCode resumes the device flow by polling with a device code
// obtained from a previous --no-wait call.
func authLoginPollDeviceCode(opts *LoginOptions, config *core.CliConfig, msg *loginMsg, log func(string, ...interface{})) error {
	f := opts.Factory

	httpClient, err := f.HttpClient()
	if err != nil {
		return err
	}
	log(msg.WaitingAuth)
	result := larkauth.PollDeviceToken(opts.Ctx, httpClient, config.AppID, config.AppSecret, config.Brand,
		opts.DeviceCode, 5, 180, f.IOStreams.ErrOut)

	if !result.OK {
		return output.ErrAuth("authorization failed: %s", result.Message)
	}
	if result.Token == nil {
		return output.ErrAuth("authorization succeeded but no token returned")
	}

	// Get user info
	log(msg.AuthSuccess)
	sdk, err := f.LarkClient()
	if err != nil {
		return output.ErrAuth("failed to get SDK: %v", err)
	}
	openId, userName, err := getUserInfo(opts.Ctx, sdk, result.Token.AccessToken)
	if err != nil {
		return output.ErrAuth("failed to get user info: %v", err)
	}

	// Store token
	now := time.Now().UnixMilli()
	storedToken := &larkauth.StoredUAToken{
		UserOpenId:       openId,
		AppId:            config.AppID,
		AccessToken:      result.Token.AccessToken,
		RefreshToken:     result.Token.RefreshToken,
		ExpiresAt:        now + int64(result.Token.ExpiresIn)*1000,
		RefreshExpiresAt: now + int64(result.Token.RefreshExpiresIn)*1000,
		Scope:            result.Token.Scope,
		GrantedAt:        now,
	}
	if err := larkauth.SetStoredToken(storedToken); err != nil {
		return output.Errorf(output.ExitInternal, "internal", "failed to save token: %v", err)
	}

	// Update config — overwrite Users to single user, clean old tokens
	multi, _ := core.LoadMultiAppConfig()
	if multi != nil && len(multi.Apps) > 0 {
		app := &multi.Apps[0]
		for _, oldUser := range app.Users {
			if oldUser.UserOpenId != openId {
				larkauth.RemoveStoredToken(config.AppID, oldUser.UserOpenId)
			}
		}
		app.Users = []core.AppUser{{UserOpenId: openId, UserName: userName}}
		if err := core.SaveMultiAppConfig(multi); err != nil {
			return output.Errorf(output.ExitInternal, "internal", "failed to save config: %v", err)
		}
	}

	output.PrintSuccess(f.IOStreams.ErrOut, fmt.Sprintf(msg.LoginSuccess, userName, openId))
	return nil
}

// collectScopesForDomains collects API scopes (from from_meta projects) and
// shortcut scopes for the given domain names.
func collectScopesForDomains(domains []string, identity string) []string {
	scopeSet := make(map[string]bool)

	// 1. API scopes from from_meta projects
	for _, s := range registry.CollectScopesForProjects(domains, identity) {
		scopeSet[s] = true
	}

	// 2. Shortcut scopes matching by Service (only include shortcuts supporting the identity)
	domainSet := make(map[string]bool, len(domains))
	for _, d := range domains {
		domainSet[d] = true
	}
	for _, sc := range shortcuts.AllShortcuts() {
		if domainSet[sc.Service] && shortcutSupportsIdentity(sc, identity) {
			for _, s := range sc.ScopesForIdentity(identity) {
				scopeSet[s] = true
			}
		}
	}

	// 3. Deduplicate and sort
	result := make([]string, 0, len(scopeSet))
	for s := range scopeSet {
		result = append(result, s)
	}
	sort.Strings(result)
	return result
}

// readScopesFromFile reads scopes from a file, one per line, ignoring empty lines and comments.
func readScopesFromFile(filePath string) ([]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var scopes []string
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		scopes = append(scopes, line)
	}
	return scopes, nil
}

// allKnownDomains returns all valid domain names (from_meta projects + shortcut services).
func allKnownDomains() map[string]bool {
	domains := make(map[string]bool)
	for _, p := range registry.ListFromMetaProjects() {
		domains[p] = true
	}
	for _, sc := range shortcuts.AllShortcuts() {
		domains[sc.Service] = true
	}
	return domains
}

// sortedKnownDomains returns all valid domain names sorted alphabetically.
func sortedKnownDomains() []string {
	m := allKnownDomains()
	domains := make([]string, 0, len(m))
	for d := range m {
		domains = append(domains, d)
	}
	sort.Strings(domains)
	return domains
}

// shortcutSupportsIdentity checks if a shortcut supports the given identity ("user" or "bot").
// Empty AuthTypes defaults to ["user"].
func shortcutSupportsIdentity(sc common.Shortcut, identity string) bool {
	authTypes := sc.AuthTypes
	if len(authTypes) == 0 {
		authTypes = []string{"user"}
	}
	for _, t := range authTypes {
		if t == identity {
			return true
		}
	}
	return false
}

// suggestDomain finds the best "did you mean" match for an unknown domain.
func suggestDomain(input string, known map[string]bool) string {
	// Check common cases: prefix match or input is a substring
	for k := range known {
		if strings.HasPrefix(k, input) || strings.HasPrefix(input, k) {
			return k
		}
	}
	return ""
}
