# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Overview

This is the CUE proposal repository containing design documents for changes to the CUE language, libraries, and tools. The repository follows a formal proposal process documented in README.md.

## Key Commands

### CI Generation
```bash
cd internal/ci && go generate  # Regenerate GitHub workflows and codereview.cfg
cue cmd gen                    # Generate workflows from internal/ci/ci_tool.cue
```

### CUE Operations
```bash
cue export                     # Export CUE configurations
cue vet                        # Validate CUE files
cue fmt                        # Format CUE files
```

### Go Operations
```bash
go mod tidy                    # Clean up Go module dependencies
go build ./...                 # Build all Go packages
```

### Proposal Publishing

```bash
go run scripts/publish/publish.go [--dry-run] [--use-ai] [commit-ref]
```
Complete implementation with colored output: Handles proposal detection, GitHub discussion management, file renaming, discussion link updates, CL submission via git codereview, trybot runs, and AI-powered summary generation.

Both tools:
- Work with git commits containing proposal files
- Support both draft (`xxxx-*.md`) and numbered (`NNNN-*.md`) proposals  
- Include dry-run mode for testing
- Default to HEAD if no commit specified

## Architecture

### Proposal Structure
- `/designs/` - Contains all proposal documents organized by category
  - `/designs/language/` - Language feature proposals
  - `/designs/modules/` and `/designs/modules.v3/` - Module system proposals
- Draft proposals use `xxxx-shortname.md` naming convention
- Published proposals use `NNNN-shortname.md` where NNNN is the GitHub discussion number
- `/scripts/` - Contains utility scripts
  - `publish-proposal.sh` - Legacy bash implementation
  - `/scripts/publish/` - Go implementation of proposal workflow
    - `publish.go` - Main publishing tool with full workflow automation
    - `publish_test.go` - Comprehensive test suite
    - `test.sh` - Test runner script

### CI Infrastructure
- `/internal/ci/` - CUE-based CI configuration system
  - `ci_tool.cue` - Main tool for generating workflows
  - `gen.go` - Go generate trigger (//go:generate cue cmd gen)
  - `base/` - Base CI configuration shared across CUE projects
  - `github/` - GitHub-specific workflow definitions
  - `repo/` - Repository-specific configuration

The CI system uses CUE to define GitHub workflows declaratively, then generates YAML files. This allows for type-safe, reusable CI configuration.

### Module Configuration
- `cue.mod/module.cue` - CUE module definition (v0.8.0) with dependency on cue.dev/x/githubactions
- `go.mod` - Go module for CI tooling (go1.18)

## Writing Guidelines

- Design documents should be wrapped at 80 columns
- Each sentence should start on a new line for better diff readability
- Use the proposal template at `/designs/TEMPLATE.md` when creating new design documents
- Comments on PRs should focus on grammar/spelling, not content (content discussion happens in GitHub Discussions)

## Development Workflow

1. Proposals start as GitHub Discussions
2. If accepted for detailed design, create design document in `/designs/`
3. Use `cue cmd gen` to regenerate CI workflows after configuration changes
4. Follow the existing naming conventions and file organization


## Generated Files

The following files are automatically generated and should not be edited manually:
- `.github/workflows/*.yml` - Generated from CUE definitions in `/internal/ci/`
- `codereview.cfg` - Generated from CUE configuration