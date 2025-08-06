# CUE Proposal Publishing Tool

A Go implementation of the CUE proposal publishing workflow that automates the entire process from draft creation to GitHub discussion updates.

## Features

- 🎨 **Colored terminal output** for better readability
- 📝 **Draft and numbered proposal support** (xxxx-*.md and NNNN-*.md)
- 🤖 **AI-powered summaries** using Claude CLI (optional)
- 🔄 **Complete workflow automation**:
  - GitHub discussion creation/verification
  - File renaming with discussion numbers
  - Discussion Channel link updates
  - CL submission via git codereview
  - Trybot execution with cueckoo
- 🧪 **Dry-run mode** for testing without making changes
- 📚 **Historical commit support** with auto-stash

## Installation

```bash
go install github.com/cue-lang/proposal/scripts/publish@latest
```

Or run directly:
```bash
go run github.com/cue-lang/proposal/scripts/publish@latest [options]
```

## Usage

```bash
# Process HEAD commit in dry-run mode
go run publish.go --dry-run

# Process specific commit with AI summaries
go run publish.go --use-ai abc123

# Process HEAD commit and publish
go run publish.go

# Run from repository root
go run scripts/publish/publish.go [options]
```

### Options

- `--dry-run`: Preview changes without modifying anything
- `--use-ai`: Use Claude AI for generating proposal summaries
- `[commit-ref]`: Git commit reference (default: HEAD)

## Workflow Steps

1. **Find proposal files** in the specified commit
2. **Run tests** (go test, cue workflow generation)
3. **Create/verify GitHub discussion**
4. **Rename proposal file** (xxxx-*.md → NNNN-*.md)
5. **Update Discussion Channel link** in the document
6. **Submit CL** via git codereview mail
7. **Run trybots** with cueckoo
8. **Update discussion** with proposal content and summary

## File Structure

```
scripts/publish/
├── publish.go       # Main implementation
├── publish_test.go  # Comprehensive test suite
├── test.sh         # Test runner script
├── go.mod          # Go module definition
└── README.md       # This file
```

## Testing

Run the test suite:
```bash
# All tests
go test ./...

# With coverage
go test -cover ./...

# Specific test
go test -v -run TestPublisherDraftProposal

# Using test script
./test.sh
```

## Requirements

- Go 1.18+
- Git with configured user
- `gh` CLI (authenticated)
- `git-codereview` tool (for CL submission)
- `cueckoo` (optional, for trybots)
- `claude` CLI (optional, for AI summaries)

## Development

To contribute to this tool:

1. Make changes to `publish.go`
2. Add/update tests in `publish_test.go`
3. Run tests: `go test ./...`
4. Format code: `go fmt ./...`
5. Submit PR with your changes

## License

Part of the CUE project. See repository license for details.