# Installation

## Prerequisites

Before installing Drift, ensure you have:

- **Go 1.21+** - [Download Go](https://golang.org/dl/)
- **Supabase CLI** - [Install Supabase CLI](https://supabase.com/docs/guides/cli)
- **Git** - Version control system

## Install via Go

The easiest way to install Drift:

```bash
go install github.com/undrift/drift/cmd/drift@latest
```

This installs the `drift` binary to your `$GOPATH/bin` directory.

## Build from Source

Clone and build the project:

```bash
git clone https://github.com/Harris-A-Khan/drift.git
cd drift
go build -o drift ./cmd/drift

# Optionally move to PATH
sudo mv drift /usr/local/bin/
```

## Verify Installation

```bash
drift --version
# drift version 0.2.1

drift --help
```

## Supabase CLI Setup

Drift requires the Supabase CLI to be installed and authenticated:

```bash
# Install Supabase CLI
brew install supabase/tap/supabase

# Login to Supabase
supabase login

# Link your project (in project directory)
supabase link --project-ref <your-project-ref>
```

## Shell Completion

Drift supports shell completion for bash, zsh, fish, and PowerShell:

```bash
# Bash
drift completion bash > /etc/bash_completion.d/drift

# Zsh
drift completion zsh > "${fpath[1]}/_drift"

# Fish
drift completion fish > ~/.config/fish/completions/drift.fish
```
