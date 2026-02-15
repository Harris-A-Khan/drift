package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/undrift/drift/internal/config"
	"github.com/undrift/drift/internal/supabase"
	"github.com/undrift/drift/internal/ui"
)

// ResolveTargetOptions controls Supabase branch resolution behavior.
type ResolveTargetOptions struct {
	GitBranch             string
	OverrideBranch        string
	FallbackBranch        string
	AllowInteractive      bool
	DisallowProdSelection bool
	PromptLabel           string
}

// ResolveSupabaseTarget resolves the effective Supabase branch with explicit fallback behavior.
func ResolveSupabaseTarget(client *supabase.Client, opts ResolveTargetOptions) (*supabase.BranchInfo, error) {
	targetBranch := opts.GitBranch
	info := &supabase.BranchInfo{
		GitBranch: opts.GitBranch,
	}

	if opts.OverrideBranch != "" {
		targetBranch = opts.OverrideBranch
		info.IsOverride = true
		info.OverrideFrom = opts.GitBranch
	}
	if IsVerbose() {
		ui.Infof("Branch resolution: git=%s override=%s fallback=%s", opts.GitBranch, opts.OverrideBranch, opts.FallbackBranch)
	}

	branch, env, err := client.ResolveBranch(targetBranch)
	if err != nil {
		return nil, err
	}
	if branch != nil {
		if opts.DisallowProdSelection && isProductionSupabaseBranch(branch) && targetBranch != opts.GitBranch {
			return nil, fmt.Errorf("refusing to target production branch '%s' via override; remove override or use a non-production branch", targetBranch)
		}
		if IsVerbose() {
			ui.Infof("Resolved exact branch match: %s (%s) -> project %s", branch.GitBranch, env, branch.ProjectRef)
		}
		return buildBranchInfo(client, info, branch, env, false), nil
	}

	fallback := strings.TrimSpace(opts.FallbackBranch)
	if fallback != "" {
		fallbackBranch, fallbackEnv, err := client.ResolveBranch(fallback)
		if err != nil {
			return nil, err
		}
		if fallbackBranch == nil {
			return nil, fmt.Errorf("fallback branch '%s' was not found", fallback)
		}
		if opts.DisallowProdSelection && isProductionSupabaseBranch(fallbackBranch) {
			return nil, fmt.Errorf("refusing to use production branch '%s' as fallback target", fallback)
		}
		if IsVerbose() {
			ui.Infof("Using configured fallback branch: %s (%s)", fallbackBranch.GitBranch, fallbackEnv)
		}
		return buildBranchInfo(client, info, fallbackBranch, fallbackEnv, true), nil
	}

	if opts.AllowInteractive && !IsYes() {
		prompt := opts.PromptLabel
		if prompt == "" {
			prompt = fmt.Sprintf("No Supabase branch for '%s'. Select fallback target", targetBranch)
		}
		fallbackBranch, fallbackEnv, err := promptForFallbackBranch(client, prompt, opts.DisallowProdSelection)
		if err != nil {
			return nil, err
		}
		if IsVerbose() {
			ui.Infof("Using interactive fallback branch: %s (%s)", fallbackBranch.GitBranch, fallbackEnv)
		}
		return buildBranchInfo(client, info, fallbackBranch, fallbackEnv, true), nil
	}

	return nil, fmt.Errorf(
		"no Supabase branch found for '%s'. Set --fallback-branch or configure supabase.fallback_branch in .drift.local.yaml",
		targetBranch,
	)
}

// ResolveSupabaseTargetForCurrentBranch resolves against the current config and branch inputs.
func ResolveSupabaseTargetForCurrentBranch(client *supabase.Client, cfg *config.Config, gitBranch, explicitTargetBranch string) (*supabase.BranchInfo, error) {
	override := ""
	disallowProdSelection := false

	if explicitTargetBranch != "" {
		gitBranch = explicitTargetBranch
		disallowProdSelection = true
	} else if cfg != nil && cfg.Supabase.OverrideBranch != "" {
		override = cfg.Supabase.OverrideBranch
		disallowProdSelection = true
	}

	fallback := GetFallbackBranch()
	if fallback == "" && cfg != nil {
		fallback = cfg.Supabase.FallbackBranch
	}

	return ResolveSupabaseTarget(client, ResolveTargetOptions{
		GitBranch:             gitBranch,
		OverrideBranch:        override,
		FallbackBranch:        fallback,
		AllowInteractive:      true,
		DisallowProdSelection: disallowProdSelection,
	})
}

func promptForFallbackBranch(client *supabase.Client, label string, disallowProd bool) (*supabase.Branch, supabase.Environment, error) {
	branches, err := client.GetBranches()
	if err != nil {
		return nil, "", err
	}

	type candidate struct {
		Branch supabase.Branch
		Env    supabase.Environment
	}
	var candidates []candidate
	for _, b := range branches {
		branch := b
		env := environmentForBranch(&branch)
		if disallowProd && isProductionSupabaseBranch(&branch) {
			continue
		}
		candidates = append(candidates, candidate{
			Branch: branch,
			Env:    env,
		})
	}

	if len(candidates) == 0 {
		return nil, "", fmt.Errorf("no non-production Supabase branches are available for fallback")
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Env != candidates[j].Env {
			return envWeight(candidates[i].Env) < envWeight(candidates[j].Env)
		}
		return candidates[i].Branch.GitBranch < candidates[j].Branch.GitBranch
	})

	options := make([]string, len(candidates))
	for i, c := range candidates {
		options[i] = fmt.Sprintf("%s (%s)", c.Branch.GitBranch, c.Env)
	}

	idx, _, err := ui.PromptSelectWithIndex(label, options)
	if err != nil {
		return nil, "", err
	}

	selected := candidates[idx]
	return &selected.Branch, selected.Env, nil
}

func buildBranchInfo(client *supabase.Client, base *supabase.BranchInfo, branch *supabase.Branch, env supabase.Environment, isFallback bool) *supabase.BranchInfo {
	info := &supabase.BranchInfo{
		GitBranch:      base.GitBranch,
		SupabaseBranch: branch,
		Environment:    env,
		ProjectRef:     branch.ProjectRef,
		APIURL:         client.GetBranchURL(branch.ProjectRef),
		IsFallback:     isFallback,
		IsOverride:     base.IsOverride,
		OverrideFrom:   base.OverrideFrom,
	}

	if project, err := client.FindProjectByRef(branch.ProjectRef); err == nil && project != nil {
		info.Region = project.Region
	}

	return info
}

func environmentForBranch(branch *supabase.Branch) supabase.Environment {
	if branch.IsDefault {
		return supabase.EnvProduction
	}
	if branch.Persistent {
		return supabase.EnvDevelopment
	}
	return supabase.EnvFeature
}

func isProductionSupabaseBranch(branch *supabase.Branch) bool {
	if branch == nil {
		return false
	}
	if branch.IsDefault {
		return true
	}
	return isProtectedBranchName(branch.Name) || isProtectedBranchName(branch.GitBranch)
}

func isProtectedBranchName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "main", "master", "production", "prod":
		return true
	default:
		return false
	}
}

func envWeight(env supabase.Environment) int {
	switch env {
	case supabase.EnvDevelopment:
		return 0
	case supabase.EnvFeature:
		return 1
	case supabase.EnvProduction:
		return 2
	default:
		return 3
	}
}
