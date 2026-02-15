package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/undrift/drift/internal/config"
	"github.com/undrift/drift/internal/ui"
	"gopkg.in/yaml.v3"
)

var configSetSecretCmd = &cobra.Command{
	Use:   "set-secret [name]",
	Short: "Interactive secret policy setup",
	Long: `Interactively configure secret behavior and placement across .drift.yaml and .drift.local.yaml.

This wizard helps you configure:
  - supabase.secrets_to_push
  - supabase.default_secrets
  - environments.<env>.secrets overrides (local)
  - environments.<env>.skip_secrets policies`,
	Args: cobra.MaximumNArgs(1),
	RunE: runConfigSetSecret,
}

func runConfigSetSecret(cmd *cobra.Command, args []string) error {
	if !RequireInit() {
		return nil
	}

	cfg, err := config.LoadWithLocal()
	if err != nil {
		return err
	}

	var secretName string
	if len(args) > 0 {
		secretName = strings.TrimSpace(args[0])
	} else {
		secretName, err = ui.PromptString("Secret name", "")
		if err != nil {
			return err
		}
		secretName = strings.TrimSpace(secretName)
	}
	if secretName == "" {
		return fmt.Errorf("secret name is required")
	}

	ui.Header("Configure Secret")
	ui.KeyValue("Secret", secretName)
	ui.NewLine()

	defaultValue := ""
	if cfg.Supabase.DefaultSecrets != nil {
		defaultValue = cfg.Supabase.DefaultSecrets[secretName]
	}

	includeInPush := true
	if len(cfg.Supabase.SecretsToPush) > 0 {
		includeInPush = sliceContains(cfg.Supabase.SecretsToPush, secretName)
	}
	includeInPush, err = ui.PromptYesNo(fmt.Sprintf("Include %s in supabase.secrets_to_push?", secretName), includeInPush)
	if err != nil {
		return err
	}

	setDefault := defaultValue != ""
	setDefault, err = ui.PromptYesNo("Set shared default value in .drift.yaml?", setDefault)
	if err != nil {
		return err
	}
	if setDefault {
		defaultValue, err = ui.PromptString(fmt.Sprintf("Default value for %s", secretName), defaultValue)
		if err != nil {
			return err
		}
	}

	devOverrideVal := ""
	if dev := cfg.GetEnvironmentConfig("development"); dev != nil && dev.Secrets != nil {
		devOverrideVal = dev.Secrets[secretName]
	}
	setDevOverride := devOverrideVal != ""
	setDevOverride, err = ui.PromptYesNo("Set development override in .drift.local.yaml?", setDevOverride)
	if err != nil {
		return err
	}
	if setDevOverride {
		devOverrideVal, err = ui.PromptString(fmt.Sprintf("Development value for %s", secretName), devOverrideVal)
		if err != nil {
			return err
		}
	}

	featureOverrideVal := ""
	if feature := cfg.GetEnvironmentConfig("feature"); feature != nil && feature.Secrets != nil {
		featureOverrideVal = feature.Secrets[secretName]
	}
	setFeatureOverride := featureOverrideVal != ""
	setFeatureOverride, err = ui.PromptYesNo("Set feature override in .drift.local.yaml?", setFeatureOverride)
	if err != nil {
		return err
	}
	if setFeatureOverride {
		featureOverrideVal, err = ui.PromptString(fmt.Sprintf("Feature value for %s", secretName), featureOverrideVal)
		if err != nil {
			return err
		}
	}

	pushToProduction := true
	if prodCfg := cfg.GetEnvironmentConfig("production"); prodCfg != nil {
		pushToProduction = !sliceContains(prodCfg.SkipSecrets, secretName)
	}
	pushToProduction, err = ui.PromptYesNo("Push this secret to production?", pushToProduction)
	if err != nil {
		return err
	}

	mainPath := cfg.ConfigPath()
	localPath := filepath.Join(cfg.ProjectRoot(), config.LocalConfigFilename)

	mainDoc, err := loadYAMLDoc(mainPath, false)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", mainPath, err)
	}

	updateSupabaseSecretsToPush(mainDoc, secretName, includeInPush)
	if setDefault {
		updateSupabaseDefaultSecret(mainDoc, secretName, defaultValue)
	}
	updateEnvironmentSkipSecret(mainDoc, "production", secretName, !pushToProduction)

	if err := writeYAMLDoc(mainPath, mainDoc); err != nil {
		return fmt.Errorf("failed to write %s: %w", mainPath, err)
	}

	localDoc := map[string]interface{}{}
	localChanged := false
	if setDevOverride || setFeatureOverride {
		if !config.LocalConfigExists() {
			if err := config.WriteLocalConfig(localPath); err != nil {
				return fmt.Errorf("failed to create %s: %w", config.LocalConfigFilename, err)
			}
		}

		localDoc, err = loadYAMLDoc(localPath, true)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", localPath, err)
		}

		if setDevOverride {
			updateEnvironmentSecretValue(localDoc, "development", secretName, devOverrideVal)
			localChanged = true
		}
		if setFeatureOverride {
			updateEnvironmentSecretValue(localDoc, "feature", secretName, featureOverrideVal)
			localChanged = true
		}
	}

	if localChanged {
		if err := writeYAMLDoc(localPath, localDoc); err != nil {
			return fmt.Errorf("failed to write %s: %w", localPath, err)
		}
	}

	ui.NewLine()
	ui.Success(fmt.Sprintf("Configured secret policy for %s", secretName))
	ui.KeyValue("Shared Config", mainPath)
	if localChanged {
		ui.KeyValue("Local Overrides", localPath)
	}
	ui.NewLine()
	ui.SubHeader("Resolution Order")
	ui.List("feature override (.drift.local.yaml)")
	ui.List("development override (.drift.local.yaml)")
	ui.List("default secret (.drift.yaml)")
	ui.List("production skip policy (when configured)")

	return nil
}

func loadYAMLDoc(path string, allowMissing bool) (map[string]interface{}, error) {
	doc := make(map[string]interface{})
	data, err := os.ReadFile(path)
	if err != nil {
		if allowMissing && os.IsNotExist(err) {
			return doc, nil
		}
		return nil, err
	}
	if strings.TrimSpace(string(data)) == "" {
		return doc, nil
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func writeYAMLDoc(path string, doc map[string]interface{}) error {
	data, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func ensureChildMap(parent map[string]interface{}, key string) map[string]interface{} {
	if existing, ok := parent[key]; ok {
		if m, ok := existing.(map[string]interface{}); ok {
			return m
		}
	}
	m := make(map[string]interface{})
	parent[key] = m
	return m
}

func toStringSlice(raw interface{}) []string {
	switch v := raw.(type) {
	case []string:
		return append([]string{}, v...)
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	default:
		return nil
	}
}

func setStringSlice(parent map[string]interface{}, key string, values []string) {
	out := make([]interface{}, 0, len(values))
	for _, v := range values {
		out = append(out, v)
	}
	parent[key] = out
}

func updateSupabaseSecretsToPush(doc map[string]interface{}, secret string, include bool) {
	supabase := ensureChildMap(doc, "supabase")
	current := toStringSlice(supabase["secrets_to_push"])

	filtered := make([]string, 0, len(current)+1)
	for _, existing := range current {
		if existing == secret || strings.TrimSpace(existing) == "" {
			continue
		}
		filtered = append(filtered, existing)
	}
	if include {
		filtered = append(filtered, secret)
	}
	setStringSlice(supabase, "secrets_to_push", filtered)
}

func updateSupabaseDefaultSecret(doc map[string]interface{}, secret, value string) {
	supabase := ensureChildMap(doc, "supabase")
	defaults := ensureChildMap(supabase, "default_secrets")
	defaults[secret] = value
}

func updateEnvironmentSecretValue(doc map[string]interface{}, envName, secret, value string) {
	envs := ensureChildMap(doc, "environments")
	env := ensureChildMap(envs, envName)
	secrets := ensureChildMap(env, "secrets")
	secrets[secret] = value
}

func updateEnvironmentSkipSecret(doc map[string]interface{}, envName, secret string, skip bool) {
	envs := ensureChildMap(doc, "environments")
	env := ensureChildMap(envs, envName)
	current := toStringSlice(env["skip_secrets"])

	filtered := make([]string, 0, len(current)+1)
	for _, existing := range current {
		if existing == secret || strings.TrimSpace(existing) == "" {
			continue
		}
		filtered = append(filtered, existing)
	}
	if skip {
		filtered = append(filtered, secret)
	}
	setStringSlice(env, "skip_secrets", filtered)
}

func sliceContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
