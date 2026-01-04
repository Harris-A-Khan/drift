# Environment Variables

Drift uses environment variables for sensitive data and CI/CD integration.

## Required Variables

These are needed for certain operations:

| Variable | Description | Used By |
|----------|-------------|---------|
| `PROD_PASSWORD` | Production database password | `drift backup`, `drift storage setup` |
| `PROD_PROJECT_REF` | Production project reference | Fallback when not linked |

## Optional Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `SUPABASE_PROJECT_REF` | Project reference | Auto-detected from `.supabase/` |
| `DRIFT_DEBUG` | Enable debug output | Not set |

## CI/CD Variables

For automated deployments, set these in your CI/CD environment:

```bash
# GitHub Actions example
env:
  SUPABASE_ACCESS_TOKEN: ${{ secrets.SUPABASE_ACCESS_TOKEN }}
  PROD_PROJECT_REF: ${{ secrets.PROD_PROJECT_REF }}
  PROD_PASSWORD: ${{ secrets.PROD_PASSWORD }}
```

## Supabase CLI Variables

Drift inherits Supabase CLI environment variables:

| Variable | Description |
|----------|-------------|
| `SUPABASE_ACCESS_TOKEN` | Supabase API token for CLI |
| `SUPABASE_DB_PASSWORD` | Database password |

## Setting Variables

### Shell (temporary)

```bash
export PROD_PASSWORD="your-password"
drift backup create
```

### .env file

Create a `.env` file (add to `.gitignore`!):

```bash
PROD_PASSWORD=your-password
PROD_PROJECT_REF=abcdefghij
```

Load with:

```bash
source .env
drift backup create
```

### direnv

Using [direnv](https://direnv.net/):

```bash
# .envrc
export PROD_PASSWORD="your-password"
```

### CI/CD Secrets

#### GitHub Actions

```yaml
name: Deploy

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup Supabase CLI
        uses: supabase/setup-cli@v1

      - name: Deploy
        env:
          SUPABASE_ACCESS_TOKEN: ${{ secrets.SUPABASE_ACCESS_TOKEN }}
        run: |
          drift deploy all --yes
```

#### GitLab CI

```yaml
deploy:
  script:
    - drift deploy all --yes
  variables:
    SUPABASE_ACCESS_TOKEN: $SUPABASE_ACCESS_TOKEN
```

## Debug Mode

Enable verbose output:

```bash
export DRIFT_DEBUG=1
drift env setup
```

## See Also

- [Configuration](drift-yaml.md)
- [CI/CD Setup](../guides/cicd.md)
