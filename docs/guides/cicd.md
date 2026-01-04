# CI/CD Setup

Automate Drift operations in your CI/CD pipeline.

## GitHub Actions

### Basic Deployment

```yaml
name: Deploy to Supabase

on:
  push:
    branches: [main, develop]

jobs:
  deploy:
    runs-on: macos-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.21'

      - name: Install Drift
        run: go install github.com/undrift/drift/cmd/drift@latest

      - name: Setup Supabase CLI
        uses: supabase/setup-cli@v1

      - name: Deploy
        env:
          SUPABASE_ACCESS_TOKEN: ${{ secrets.SUPABASE_ACCESS_TOKEN }}
        run: |
          drift deploy all --yes
```

### With Version Bump

```yaml
name: Release

on:
  push:
    branches: [main]

jobs:
  release:
    runs-on: macos-latest
    steps:
      - uses: actions/checkout@v4
        with:
          token: ${{ secrets.PAT_TOKEN }}  # For pushing version bump

      - name: Setup
        run: |
          go install github.com/undrift/drift/cmd/drift@latest

      - name: Bump Version
        run: |
          git config user.name "GitHub Actions"
          git config user.email "actions@github.com"
          drift version bump --commit --push

      - name: Deploy
        env:
          SUPABASE_ACCESS_TOKEN: ${{ secrets.SUPABASE_ACCESS_TOKEN }}
        run: |
          drift deploy all --yes
```

### Scheduled Backups

```yaml
name: Database Backup

on:
  schedule:
    - cron: '0 2 * * *'  # Daily at 2 AM

jobs:
  backup:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup
        run: |
          go install github.com/undrift/drift/cmd/drift@latest

      - name: Create Backup
        env:
          SUPABASE_ACCESS_TOKEN: ${{ secrets.SUPABASE_ACCESS_TOKEN }}
          PROD_PASSWORD: ${{ secrets.PROD_PASSWORD }}
        run: |
          drift backup create -o backup.sql.gz
          drift backup upload backup.sql.gz prod
```

## GitLab CI

```yaml
stages:
  - deploy
  - backup

deploy:
  stage: deploy
  image: golang:1.21
  before_script:
    - go install github.com/undrift/drift/cmd/drift@latest
    - curl -sL https://github.com/supabase/cli/releases/download/v1.123.0/supabase_linux_amd64.tar.gz | tar xz
    - mv supabase /usr/local/bin/
  script:
    - drift deploy all --yes
  variables:
    SUPABASE_ACCESS_TOKEN: $SUPABASE_ACCESS_TOKEN
  only:
    - main
    - develop

backup:
  stage: backup
  image: golang:1.21
  before_script:
    - go install github.com/undrift/drift/cmd/drift@latest
  script:
    - drift backup create -o backup.sql.gz
    - drift backup upload backup.sql.gz prod
  variables:
    SUPABASE_ACCESS_TOKEN: $SUPABASE_ACCESS_TOKEN
    PROD_PASSWORD: $PROD_PASSWORD
  only:
    - schedules
```

## Required Secrets

| Secret | Description | Required For |
|--------|-------------|--------------|
| `SUPABASE_ACCESS_TOKEN` | Supabase CLI auth token | All operations |
| `PROD_PASSWORD` | Database password | Backups |
| `PAT_TOKEN` | GitHub PAT (for pushing) | Version bump with push |

### Getting Supabase Access Token

```bash
supabase login
# Token is stored in ~/.supabase/access-token
cat ~/.supabase/access-token
```

## Branch-Based Deployment

Deploy to different environments based on branch:

```yaml
deploy:
  runs-on: macos-latest
  steps:
    - uses: actions/checkout@v4

    - name: Deploy to Production
      if: github.ref == 'refs/heads/main'
      run: drift deploy all --branch main --yes

    - name: Deploy to Development
      if: github.ref == 'refs/heads/develop'
      run: drift deploy all --branch develop --yes

    - name: Deploy to Feature
      if: startsWith(github.ref, 'refs/heads/feature/')
      run: drift deploy all --yes
```

## Xcode Cloud

For Xcode Cloud, add a custom build script:

```bash
#!/bin/bash
# ci_scripts/ci_post_clone.sh

# Install Drift
go install github.com/undrift/drift/cmd/drift@latest

# Generate config
drift env setup
```

## See Also

- [Deployment](../commands/deploy.md)
- [Backups Guide](backups.md)
