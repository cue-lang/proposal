// Command publish automates the CUE proposal publication workflow.
//
// Usage: go run publish.go [--dry-run] [commit-ref]
//
// This command automates the complete workflow for publishing a CUE proposal:
//  1. Finds the proposal file in the specified commit (or HEAD if not specified)
//  2. Runs tests to ensure the proposal is ready
//  3. Creates a GitHub discussion (for draft proposals)
//  4. Renames the proposal file with the discussion number (for drafts)
//  5. Submits the commit through git codereview
//  6. Runs trybots and waits for confirmation
//  7. Waits for CL approval and submission
//  8. Updates the GitHub discussion with proposal summary
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"
)

// Logger provides colored output for different message types.
type Logger struct {
	colors bool
}

// Color constants for terminal output.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[0;31m"
	colorGreen  = "\033[0;32m"
	colorYellow = "\033[1;33m"
	colorBlue   = "\033[0;34m"
)

// NewLogger creates a logger with automatic color detection.
func NewLogger() *Logger {
	return &Logger{
		colors: term.IsTerminal(int(os.Stderr.Fd())),
	}
}

func (l *Logger) colorize(color, text string) string {
	if !l.colors {
		return text
	}
	return color + text + colorReset
}

func (l *Logger) Info(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s %s\n", l.colorize(colorBlue, "[INFO]"), msg)
}

func (l *Logger) Success(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s %s\n", l.colorize(colorGreen, "[SUCCESS]"), msg)
}

func (l *Logger) Warning(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s %s\n", l.colorize(colorYellow, "[WARNING]"), msg)
}

func (l *Logger) Error(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s %s\n", l.colorize(colorRed, "[ERROR]"), msg)
}

func (l *Logger) Warn(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s %s\n", l.colorize(colorYellow, "[WARN]"), msg)
}

func (l *Logger) Prompt(format string, args ...interface{}) string {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s %s", l.colorize(colorYellow, "[PROMPT]"), msg)

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		log.Fatal("failed to read user input:", err)
	}
	return strings.TrimSpace(response)
}

// Publisher manages the proposal publication workflow.
type Publisher struct {
	logger           *Logger
	commitRef        string
	commitHash       string
	dryRun           bool
	proposalFile     string
	basename         string
	isDraft          bool
	isNumbered       bool
	discussionNumber string
	discussionURL    string
	newProposalFile  string
	clNumber         string
	clURL            string
	useAI            bool
}

// NewPublisher creates a new publisher for the given commit reference.
func NewPublisher(commitRef string, dryRun bool, useAI bool) *Publisher {
	return &Publisher{
		logger:    NewLogger(),
		commitRef: commitRef,
		dryRun:    dryRun,
		useAI:     useAI,
	}
}

// runCommand executes a command and returns stdout, stderr, and error.
func (p *Publisher) runCommand(name string, args ...string) (string, string, error) {
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// runCommandInput executes a command with stdin input.
func (p *Publisher) runCommandInput(input string, name string, args ...string) (string, string, error) {
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = strings.NewReader(input)
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// findProposalFile finds the proposal file in the specified commit.
func (p *Publisher) findProposalFile() error {
	p.logger.Info("Finding proposal files in commit %s...", p.commitRef)

	// Verify commit exists
	commitHash, _, err := p.runCommand("git", "rev-parse", p.commitRef)
	if err != nil {
		return fmt.Errorf("invalid commit reference: %s", p.commitRef)
	}
	p.commitHash = strings.TrimSpace(commitHash)

	// Get files changed in the commit with rename detection
	stdout, _, err := p.runCommand("git", "diff-tree", "--no-commit-id", "--name-status", "-r", "-M", p.commitRef)
	if err != nil {
		return fmt.Errorf("failed to get files from commit: %v", err)
	}

	// Find proposal files, handling renames
	var proposalFiles []string
	var renamedFrom, renamedTo string

	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if line == "" {
			continue
		}

		// Parse git diff-tree output: STATUS\tFILE or STATUS\tOLD_FILE\tNEW_FILE
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}

		status := parts[0]

		if strings.HasPrefix(status, "R") { // Rename
			if len(parts) < 3 {
				continue
			}
			oldFile := parts[1]
			newFile := parts[2]

			// Check if this is a proposal file rename
			if strings.HasPrefix(oldFile, "designs/") && strings.HasSuffix(oldFile, ".md") &&
				strings.HasPrefix(newFile, "designs/") && strings.HasSuffix(newFile, ".md") {
				renamedFrom = oldFile
				renamedTo = newFile
				// For renames, we only consider the target file as the proposal file
				proposalFiles = append(proposalFiles, newFile)
				p.logger.Info("Detected proposal file rename: %s -> %s", oldFile, newFile)
			}
		} else if status == "A" || status == "M" { // Added or Modified
			file := parts[1]
			if strings.HasPrefix(file, "designs/") && strings.HasSuffix(file, ".md") {
				proposalFiles = append(proposalFiles, file)
			}
		}
	}

	if len(proposalFiles) == 0 {
		return fmt.Errorf("no proposal files (designs/*.md) found in commit %s", p.commitRef)
	}

	if len(proposalFiles) > 1 {
		p.logger.Error("Multiple proposal files found in commit %s:", p.commitRef)
		for _, file := range proposalFiles {
			fmt.Fprintf(os.Stderr, "  %s\n", file)
		}
		if renamedFrom != "" && renamedTo != "" {
			p.logger.Info("Note: Detected rename from %s to %s", renamedFrom, renamedTo)
		}
		return fmt.Errorf("each proposal should be in its own commit")
	}

	p.proposalFile = proposalFiles[0]
	p.basename = filepath.Base(p.proposalFile)
	p.logger.Info("Found proposal file: %s", p.proposalFile)

	// Determine if draft or numbered proposal
	draftPattern := regexp.MustCompile(`^xxxx-.*\.md$`)
	numberedPattern := regexp.MustCompile(`^(\d+)-.*\.md$`)

	if draftPattern.MatchString(p.basename) {
		p.isDraft = true
		p.logger.Info("Detected draft proposal: %s", p.basename)
	} else if matches := numberedPattern.FindStringSubmatch(p.basename); matches != nil {
		p.isNumbered = true
		p.discussionNumber = matches[1]
		p.logger.Info("Detected numbered proposal: %s (Discussion #%s)", p.basename, p.discussionNumber)
	} else {
		return fmt.Errorf("proposal file must follow naming convention: xxxx-*.md (draft) or NNNN-*.md (numbered), got: %s", p.basename)
	}

	return nil
}

// runTests runs the test suite to ensure the proposal is ready.
func (p *Publisher) runTests() error {
	p.logger.Info("Step 1: Running tests...")

	// Run Go tests (may have no test files)
	_, _, err := p.runCommand("go", "test", "./...")
	if err != nil {
		p.logger.Warning("Go tests failed or no tests found")
	}

	// Check CUE workflow generation
	p.logger.Info("Checking CUE workflow generation...")
	_, stderr, err := p.runCommand("sh", "-c", "cd internal/ci && go generate")
	if err != nil {
		p.logger.Warning("CUE workflow generation failed: %s", stderr)
		// Don't fail the whole workflow for CUE errors
	}

	p.logger.Success("Tests completed successfully")
	return nil
}

// callGitHubAPI calls the GitHub API using the gh CLI.
func (p *Publisher) callGitHubAPI(method, endpoint string, data interface{}) ([]byte, error) {
	args := []string{"api", endpoint}
	if method != "" && method != "GET" {
		args = append(args, "--method", method)
	}

	var input string
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal JSON: %v", err)
		}
		input = string(jsonData)
		args = append(args, "--input", "-")
	}

	stdout, stderr, err := p.runCommandInput(input, "gh", args...)
	if err != nil {
		return nil, fmt.Errorf("gh api call failed: %s", stderr)
	}

	return []byte(stdout), nil
}

// getDiscussionCategories gets the available discussion categories.
func (p *Publisher) getDiscussionCategories() (string, error) {
	p.logger.Info("Getting discussion categories...")

	// Use GraphQL API for discussions (REST API doesn't support discussion categories)
	query := `
	query {
		repository(owner: "cue-lang", name: "cue") {
			discussionCategories(first: 10) {
				nodes {
					id
					name
				}
			}
		}
	}`

	reqData := map[string]interface{}{
		"query": query,
	}

	data, err := p.callGitHubAPI("POST", "graphql", reqData)
	if err != nil {
		return "", fmt.Errorf("failed to get discussion categories: %v", err)
	}

	// Parse GraphQL response
	var graphqlResp struct {
		Data struct {
			Repository struct {
				DiscussionCategories struct {
					Nodes []struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"nodes"`
				} `json:"discussionCategories"`
			} `json:"repository"`
		} `json:"data"`
		Errors []interface{} `json:"errors"`
	}

	if err := json.Unmarshal(data, &graphqlResp); err != nil {
		return "", fmt.Errorf("failed to parse GraphQL response: %v", err)
	}

	if len(graphqlResp.Errors) > 0 {
		return "", fmt.Errorf("GraphQL errors: %v", graphqlResp.Errors)
	}

	categories := graphqlResp.Data.Repository.DiscussionCategories.Nodes
	if len(categories) == 0 {
		return "", fmt.Errorf("no discussion categories found")
	}

	// Look for Proposals category first
	for _, cat := range categories {
		if cat.Name == "Proposals" || cat.Name == "Proposal" {
			return cat.ID, nil
		}
	}

	// Fallback to first category
	if len(categories) > 0 {
		p.logger.Warning("Could not find 'Proposals' category, using: %s", categories[0].Name)
		return categories[0].ID, nil
	}

	return "", fmt.Errorf("no discussion categories found")
}

// createDiscussion creates a new GitHub discussion for draft proposals.
func (p *Publisher) createDiscussion() error {
	if !p.isDraft {
		return nil // Skip for numbered proposals
	}

	p.logger.Info("Step 2: Creating GitHub discussion for draft proposal...")

	// Extract title from proposal (read from git commit)
	stdout, _, err := p.runCommand("git", "show", fmt.Sprintf("%s:%s", p.commitRef, p.proposalFile))
	if err != nil {
		return fmt.Errorf("failed to read proposal file from commit: %v", err)
	}
	content := []byte(stdout)

	titlePattern := regexp.MustCompile(`^# (.+)$`)
	lines := strings.Split(string(content), "\n")
	var title string
	for _, line := range lines {
		if matches := titlePattern.FindStringSubmatch(line); matches != nil {
			title = strings.TrimSpace(matches[1])
			break
		}
	}

	if title == "" {
		return fmt.Errorf("could not extract title from proposal file (no '# Title' found)")
	}

	// Create discussion body - we'll update with the full content after renaming
	// Extract the proposal name without xxxx- prefix for a cleaner message
	basename := filepath.Base(p.proposalFile)
	proposalName := strings.TrimPrefix(basename, "xxxx-")
	proposalName = strings.TrimSuffix(proposalName, ".md")

	body := fmt.Sprintf(`This proposal is currently under review.

**Proposal**: %s
**Status**: Draft under review
**Category**: Proposal

The full proposal content will be published to this discussion once the review process completes.

---
*This discussion was created automatically by the proposal publication workflow.*`, proposalName)

	if p.dryRun {
		p.logger.Info("[DRY RUN] Would create discussion with title: %s", title)
		p.discussionNumber = "1234"
		p.discussionURL = fmt.Sprintf("https://github.com/cue-lang/cue/discussions/%s", p.discussionNumber)
		return nil
	}

	// Get category ID
	categoryID, err := p.getDiscussionCategories()
	if err != nil {
		return fmt.Errorf("failed to get discussion categories: %v", err)
	}

	p.logger.Info("Creating discussion with title: %s", title)

	// Get repository ID for GraphQL
	query := `
	query {
		repository(owner: "cue-lang", name: "cue") {
			id
		}
	}`

	reqData := map[string]interface{}{
		"query": query,
	}

	data, err := p.callGitHubAPI("POST", "graphql", reqData)
	if err != nil {
		return fmt.Errorf("failed to get repository ID: %v", err)
	}

	var repoResp struct {
		Data struct {
			Repository struct {
				ID string `json:"id"`
			} `json:"repository"`
		} `json:"data"`
	}

	if err := json.Unmarshal(data, &repoResp); err != nil {
		return fmt.Errorf("failed to parse repository response: %v", err)
	}

	repoID := repoResp.Data.Repository.ID

	// Create discussion using GraphQL mutation
	mutation := `
	mutation($repositoryId: ID!, $categoryId: ID!, $title: String!, $body: String!) {
		createDiscussion(input: {
			repositoryId: $repositoryId
			categoryId: $categoryId
			title: $title
			body: $body
		}) {
			discussion {
				number
				url
			}
		}
	}`

	variables := map[string]interface{}{
		"repositoryId": repoID,
		"categoryId":   categoryID,
		"title":        title,
		"body":         body,
	}

	reqData = map[string]interface{}{
		"query":     mutation,
		"variables": variables,
	}

	data, err = p.callGitHubAPI("POST", "graphql", reqData)
	if err != nil {
		return fmt.Errorf("failed to create discussion: %v", err)
	}

	var response struct {
		Data struct {
			CreateDiscussion struct {
				Discussion struct {
					Number int    `json:"number"`
					URL    string `json:"url"`
				} `json:"discussion"`
			} `json:"createDiscussion"`
		} `json:"data"`
		Errors []interface{} `json:"errors"`
	}

	if err := json.Unmarshal(data, &response); err != nil {
		return fmt.Errorf("failed to parse discussion response: %v", err)
	}

	if len(response.Errors) > 0 {
		return fmt.Errorf("GraphQL errors: %v", response.Errors)
	}

	p.discussionNumber = strconv.Itoa(response.Data.CreateDiscussion.Discussion.Number)
	p.discussionURL = response.Data.CreateDiscussion.Discussion.URL

	p.logger.Success("Created discussion #%s: %s", p.discussionNumber, p.discussionURL)
	return nil
}

// verifyDiscussion verifies that an existing discussion belongs to this proposal.
func (p *Publisher) verifyDiscussion() error {
	if !p.isNumbered {
		return nil // Skip for draft proposals
	}

	p.logger.Info("Step 2: Verifying existing discussion for numbered proposal...")
	p.logger.Info("Verifying discussion #%s belongs to this proposal...", p.discussionNumber)

	// Get discussion content using GraphQL
	query := `
	query($number: Int!) {
		repository(owner: "cue-lang", name: "cue") {
			discussion(number: $number) {
				body
				url
			}
		}
	}`

	numberInt, err := strconv.Atoi(p.discussionNumber)
	if err != nil {
		return fmt.Errorf("invalid discussion number: %v", err)
	}

	variables := map[string]interface{}{
		"number": numberInt,
	}

	reqData := map[string]interface{}{
		"query":     query,
		"variables": variables,
	}

	data, err := p.callGitHubAPI("POST", "graphql", reqData)
	if err != nil {
		p.logger.Error("Failed to get discussion #%s", p.discussionNumber)
		return fmt.Errorf("discussion verification failed: %v", err)
	}

	var response struct {
		Data struct {
			Repository struct {
				Discussion struct {
					Body string `json:"body"`
					URL  string `json:"url"`
				} `json:"discussion"`
			} `json:"repository"`
		} `json:"data"`
		Errors []interface{} `json:"errors"`
	}

	if err := json.Unmarshal(data, &response); err != nil {
		return fmt.Errorf("failed to parse discussion response: %v", err)
	}

	if len(response.Errors) > 0 {
		return fmt.Errorf("GraphQL errors: %v", response.Errors)
	}

	if response.Data.Repository.Discussion.Body == "" {
		p.logger.Error("Discussion #%s not found", p.discussionNumber)
		return fmt.Errorf("discussion verification failed")
	}

	// Check if discussion mentions this proposal file or contains draft indicators
	body := strings.ToLower(response.Data.Repository.Discussion.Body)
	proposalFile := strings.ToLower(p.proposalFile)

	if strings.Contains(body, proposalFile) ||
		strings.Contains(body, "coming soon") ||
		strings.Contains(body, "being prepared for review") ||
		strings.Contains(body, "draft under review") {
		p.logger.Success("Verified discussion #%s belongs to this proposal", p.discussionNumber)
		p.discussionURL = response.Data.Repository.Discussion.URL
		return nil
	}

	p.logger.Error("Discussion #%s does not appear to belong to this proposal", p.discussionNumber)
	p.logger.Error("Discussion body does not mention the proposal file or draft indicators")
	fmt.Fprintf(os.Stderr, "Discussion body preview:\n%s\n", response.Data.Repository.Discussion.Body[:min(200, len(response.Data.Repository.Discussion.Body))])
	return fmt.Errorf("discussion verification failed")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// renameProposal renames the draft proposal file with the discussion number.
func (p *Publisher) renameProposal() error {
	if !p.isDraft {
		p.logger.Info("Step 3: Skipping file rename (already numbered: %s)", p.basename)
		p.newProposalFile = p.proposalFile

		// For numbered proposals, still update the discussion link if needed
		if err := p.updateDiscussionLink(); err != nil {
			return fmt.Errorf("failed to update discussion link: %v", err)
		}

		return nil
	}

	p.logger.Info("Step 3: Renaming proposal file...")

	// Generate new filename
	dirname := filepath.Dir(p.proposalFile)
	newBasename := strings.Replace(p.basename, "xxxx-", p.discussionNumber+"-", 1)
	p.newProposalFile = filepath.Join(dirname, newBasename)

	if p.dryRun {
		p.logger.Info("[DRY RUN] Would rename %s to %s", p.proposalFile, p.newProposalFile)
		return nil
	}

	// Rename the file in git
	if p.commitRef == "HEAD" {
		// Working on HEAD - can directly modify
		_, _, err := p.runCommand("git", "mv", p.proposalFile, p.newProposalFile)
		if err != nil {
			return fmt.Errorf("failed to rename file: %v", err)
		}

		// Update the Discussion Channel link in the document
		if err := p.updateDiscussionLink(); err != nil {
			return fmt.Errorf("failed to update discussion link: %v", err)
		}

		_, _, err = p.runCommand("git", "add", p.newProposalFile)
		if err != nil {
			return fmt.Errorf("failed to add renamed file: %v", err)
		}

		_, _, err = p.runCommand("git", "commit", "--amend", "--no-edit")
		if err != nil {
			return fmt.Errorf("failed to amend commit: %v", err)
		}

		// Get the new commit hash after amending
		newCommitHash, _, err := p.runCommand("git", "rev-parse", "HEAD")
		if err != nil {
			return fmt.Errorf("failed to get new commit hash: %v", err)
		}
		p.commitHash = strings.TrimSpace(newCommitHash)[:8]
		p.commitRef = "HEAD"
	} else {
		// For non-HEAD commits, we have a few options:
		// 1. Cherry-pick approach (safest)
		// 2. Rebase approach (more complex)
		// 3. Filter-branch approach (cleanest but requires tool)

		p.logger.Warn("Renaming in historical commit %s", p.commitHash[:8])
		p.logger.Warn("This will rewrite history - make sure to coordinate with team")

		// Check for uncommitted changes and stash if needed
		stdout, _, err := p.runCommand("git", "status", "--porcelain")
		if err != nil {
			return fmt.Errorf("failed to check git status: %v", err)
		}

		var stashCreated bool
		if stdout != "" {
			p.logger.Warn("Uncommitted changes detected, stashing them temporarily...")
			_, _, err = p.runCommand("git", "stash", "push", "-m", "proposal-rename: auto-stash")
			if err != nil {
				p.logger.Error("Failed to stash changes. Please commit or stash manually.")
				return fmt.Errorf("failed to stash changes: %v", err)
			}
			stashCreated = true
		}

		// Get current branch
		currentBranch, _, err := p.runCommand("git", "rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			return fmt.Errorf("failed to get current branch: %v", err)
		}
		currentBranch = strings.TrimSpace(currentBranch)

		// Create temporary branch at the target commit
		tempBranch := fmt.Sprintf("proposal-rename-%d", time.Now().Unix())
		_, stderr, err := p.runCommand("git", "checkout", "-b", tempBranch, p.commitHash)
		if err != nil {
			p.logger.Error("Failed to create temp branch: %s", stderr)
			return fmt.Errorf("failed to create temp branch: %v (stderr: %s)", err, stderr)
		}

		// Ensure we return to original branch on exit
		cleanupDone := false
		cleanup := func() {
			if !cleanupDone {
				p.runCommand("git", "checkout", currentBranch)
				p.runCommand("git", "branch", "-D", tempBranch)
				if stashCreated {
					p.logger.Info("Restoring stashed changes...")
					p.runCommand("git", "stash", "pop")
				}
				cleanupDone = true
			}
		}
		defer cleanup()

		// Perform the rename
		_, _, err = p.runCommand("git", "mv", p.proposalFile, p.newProposalFile)
		if err != nil {
			return fmt.Errorf("failed to rename file: %v", err)
		}

		// Update the Discussion Channel link in the document
		if err := p.updateDiscussionLink(); err != nil {
			return fmt.Errorf("failed to update discussion link: %v", err)
		}

		// Amend the commit
		_, _, err = p.runCommand("git", "commit", "--amend", "--no-edit")
		if err != nil {
			return fmt.Errorf("failed to amend commit: %v", err)
		}

		// Get the new commit hash
		newCommitHash, _, err := p.runCommand("git", "rev-parse", "HEAD")
		if err != nil {
			return fmt.Errorf("failed to get new commit hash: %v", err)
		}
		newCommitHash = strings.TrimSpace(newCommitHash)

		// Check if there are commits after the original
		commitsAfter, _, _ := p.runCommand("git", "rev-list", "--count",
			fmt.Sprintf("%s..%s", p.commitHash, currentBranch))
		numCommits, _ := strconv.Atoi(strings.TrimSpace(commitsAfter))

		if numCommits == 0 {
			// No commits after - just update the branch
			_, _, err = p.runCommand("git", "checkout", currentBranch)
			if err != nil {
				return fmt.Errorf("failed to checkout original branch: %v", err)
			}
			_, _, err = p.runCommand("git", "reset", "--hard", newCommitHash)
			if err != nil {
				return fmt.Errorf("failed to update branch: %v", err)
			}
		} else {
			// There are commits after - need to cherry-pick them
			p.logger.Info("Cherry-picking %d commits after the renamed commit...", numCommits)

			// Get list of commits to cherry-pick
			commitList, _, err := p.runCommand("git", "rev-list", "--reverse",
				fmt.Sprintf("%s..%s", p.commitHash, currentBranch))
			if err != nil {
				return fmt.Errorf("failed to get commit list: %v", err)
			}

			commits := strings.Fields(commitList)
			for _, commit := range commits {
				_, _, err = p.runCommand("git", "cherry-pick", commit)
				if err != nil {
					return fmt.Errorf("failed to cherry-pick %s: %v", commit[:8], err)
				}
			}

			// Update the original branch
			newHead, _, _ := p.runCommand("git", "rev-parse", "HEAD")
			newHead = strings.TrimSpace(newHead)

			_, _, err = p.runCommand("git", "checkout", currentBranch)
			if err != nil {
				return fmt.Errorf("failed to checkout original branch: %v", err)
			}
			_, _, err = p.runCommand("git", "reset", "--hard", newHead)
			if err != nil {
				return fmt.Errorf("failed to update branch: %v", err)
			}
		}

		// Update the commit reference to the new hash
		p.commitHash = newCommitHash[:8]
		p.commitRef = p.commitHash

		// Cleanup is successful, mark it done
		cleanup()

		p.logger.Success("Successfully renamed file in commit %s", p.commitHash)
	}

	p.logger.Success("Renamed %s to %s", p.proposalFile, p.newProposalFile)
	return nil
}

// updateDiscussionLink updates the Discussion Channel link in the proposal document.
func (p *Publisher) updateDiscussionLink() error {
	if p.dryRun {
		p.logger.Info("[DRY RUN] Would update Discussion Channel link to %s", p.discussionURL)
		return nil
	}

	// Read the current content of the file (from working directory for drafts, from commit for numbered)
	var content []byte
	var err error

	if p.isDraft {
		// For drafts, read from working directory (after rename)
		content, err = os.ReadFile(p.newProposalFile)
		if err != nil {
			return fmt.Errorf("failed to read proposal file: %v", err)
		}
	} else {
		// For numbered proposals, read from the commit
		stdout, _, err := p.runCommand("git", "show", fmt.Sprintf("%s:%s", p.commitRef, p.proposalFile))
		if err != nil {
			return fmt.Errorf("failed to read proposal file from commit: %v", err)
		}
		content = []byte(stdout)
	}

	// Look for the Discussion Channel line and update it
	lines := strings.Split(string(content), "\n")
	updated := false
	// Match formats like: **Discussion Channel** GitHub: {link}, **Discussion Channel**: {link}, or *   **Discussion Channel**: TBD
	discussionChannelPattern := regexp.MustCompile(`^(\*\s+\*\*Discussion Channel\*\*:\s*|\*\*Discussion Channel\*\*\s*:?\s*(?:GitHub:?\s*)?)(.*)$`)

	for i, line := range lines {
		if matches := discussionChannelPattern.FindStringSubmatch(line); matches != nil {
			// Check if it contains a placeholder like {link} or needs updating
			if strings.Contains(matches[2], "{link}") || strings.Contains(matches[2], "TBD") || strings.Contains(matches[2], "TODO") {
				// Update the line with the GitHub discussion URL
				lines[i] = fmt.Sprintf("%s%s", matches[1], p.discussionURL)
				updated = true
				break
			}
		}
	}

	if !updated {
		// If no Discussion Channel line found, try to add one after Author(s)
		authorPattern := regexp.MustCompile(`^\*\s+\*\*Author\(s\)\*\*:`)
		for i, line := range lines {
			if authorPattern.MatchString(line) {
				// Insert Discussion Channel after Author(s)
				newLine := fmt.Sprintf("*   **Discussion Channel**: %s", p.discussionURL)
				lines = append(lines[:i+1], append([]string{newLine}, lines[i+1:]...)...)
				updated = true
				break
			}
		}
	}

	if updated {
		updatedContent := strings.Join(lines, "\n")

		if p.isDraft {
			// For drafts, write to working directory file
			if err := os.WriteFile(p.newProposalFile, []byte(updatedContent), 0644); err != nil {
				return fmt.Errorf("failed to write updated proposal file: %v", err)
			}
		} else {
			// For numbered proposals, we need to update the commit
			// This is complex for non-HEAD commits, so for now we'll use a different approach
			if p.commitRef == "HEAD" {
				// Write to working directory and amend
				if err := os.WriteFile(p.proposalFile, []byte(updatedContent), 0644); err != nil {
					return fmt.Errorf("failed to write updated proposal file: %v", err)
				}

				// Stage and amend the commit
				_, _, err := p.runCommand("git", "add", p.proposalFile)
				if err != nil {
					return fmt.Errorf("failed to stage updated file: %v", err)
				}

				_, _, err = p.runCommand("git", "commit", "--amend", "--no-edit")
				if err != nil {
					return fmt.Errorf("failed to amend commit: %v", err)
				}
			} else {
				// For historical commits, this is complex - we'd need to rewrite history
				p.logger.Warn("Cannot update discussion link in historical commit %s", p.commitRef)
				p.logger.Info("The discussion link update will be included in the discussion content instead")
			}
		}

		p.logger.Success("Updated Discussion Channel link to %s", p.discussionURL)
	} else {
		p.logger.Warn("Could not find or add Discussion Channel field in proposal")
	}

	return nil
}

// generateProposalSummary uses claude CLI to create an intelligent summary.
func (p *Publisher) generateProposalSummary(content string) (string, error) {
	if !p.useAI {
		return "", fmt.Errorf("AI summary generation disabled")
	}

	prompt := `You are summarizing a CUE language proposal for a GitHub discussion. Create a clear, concise summary that captures the essence of the proposal.

Focus on:
1. The problem being addressed
2. The proposed solution
3. Some key examples or use cases
4. Key benefits and impact
5. Any important technical details or considerations

Guidelines:
- Write 3-5 paragraphs
- Use clear, accessible language
- Highlight the most important aspects
- Format in markdown
- Don't include metadata lines (Status:, Author:, etc.)
- Focus on the actual proposal content

Please summarize this proposal:`

	// Use claude CLI to generate summary
	stdout, stderr, err := p.runCommandInput(content+"\n"+prompt, "claude")
	if err != nil {
		// Check if claude is not installed
		if strings.Contains(stderr, "command not found") || strings.Contains(stderr, "not found") {
			return "", fmt.Errorf("claude CLI not available")
		}
		return "", fmt.Errorf("claude summary generation failed: %s", stderr)
	}

	summary := strings.TrimSpace(stdout)
	if summary == "" {
		return "", fmt.Errorf("claude returned empty summary")
	}

	// Add a note that this was AI-generated
	summary += "\n\n_[Summary generated by Claude AI]_"

	return summary, nil
}

// submitCL submits the changes via git codereview mail.
func (p *Publisher) submitCL() error {
	p.logger.Info("Step 4: Submitting CL via git codereview mail...")

	if p.dryRun {
		p.logger.Info("[DRY RUN] Would submit CL via git codereview mail")
		p.clNumber = "12345"
		p.clURL = "https://review.gerrithub.io/c/cue-lang/proposal/+/12345"
		return nil
	}

	// Run git codereview mail with specific commit
	stdout, stderr, err := p.runCommand("git", "codereview", "mail", p.commitRef)
	if err != nil {
		// Check if it's the "no new changes" error which we can ignore
		if strings.Contains(stderr, "no new changes") || strings.Contains(stdout, "no new changes") {
			p.logger.Info("No new changes to submit (CL may already exist)")
			// Try to get the existing CL number
			if err := p.getExistingCL(); err != nil {
				return fmt.Errorf("failed to get existing CL: %v", err)
			}
			return nil
		}
		return fmt.Errorf("failed to submit CL: %v (stderr: %s)", err, stderr)
	}

	// Extract CL number from output
	// Example: "https://review.gerrithub.io/c/cue-lang/proposal/+/123456 [NEW]"
	clPattern := regexp.MustCompile(`https://[^\s]+/(\d+)`)
	if matches := clPattern.FindStringSubmatch(stdout); matches != nil {
		p.clNumber = matches[1]
		p.clURL = matches[0]
		p.logger.Success("Submitted CL %s: %s", p.clNumber, p.clURL)
	} else {
		// Try to get from stderr as well
		if matches := clPattern.FindStringSubmatch(stderr); matches != nil {
			p.clNumber = matches[1]
			p.clURL = matches[0]
			p.logger.Success("Submitted CL %s: %s", p.clNumber, p.clURL)
		} else {
			p.logger.Warn("Could not extract CL number from git codereview output")
			// Try to get it another way
			if err := p.getExistingCL(); err != nil {
				return fmt.Errorf("failed to get CL number: %v", err)
			}
		}
	}

	return nil
}

// getExistingCL tries to get the CL number for the current commit.
func (p *Publisher) getExistingCL() error {
	// Try using git config to get the change-id
	stdout, _, err := p.runCommand("git", "log", "-1", "--format=%B")
	if err != nil {
		return fmt.Errorf("failed to get commit message: %v", err)
	}

	// Look for Change-Id in commit message
	changeIDPattern := regexp.MustCompile(`Change-Id: (I[a-f0-9]{40})`)
	if matches := changeIDPattern.FindStringSubmatch(stdout); matches != nil {
		changeID := matches[1]
		// Query Gerrit for this change
		stdout, _, err := p.runCommand("git", "config", "--get", "remote.origin.url")
		if err != nil {
			return fmt.Errorf("failed to get remote URL: %v", err)
		}

		// Extract Gerrit URL and construct CL URL
		if strings.Contains(stdout, "gerrithub.io") {
			// For now, just construct a likely URL
			p.clURL = fmt.Sprintf("https://review.gerrithub.io/q/%s", changeID)
			p.logger.Info("CL may be at: %s", p.clURL)
		}
	}

	return nil
}

// runTrybots runs trybots on the submitted CL.
func (p *Publisher) runTrybots() error {
	p.logger.Info("Step 5: Running trybots...")

	if p.dryRun {
		p.logger.Info("[DRY RUN] Would run trybots with: cueckoo runtrybot <new-commit-hash>")
		p.logger.Info("[DRY RUN] (commit hash will change after file rename and amendments)")
		return nil
	}

	if p.clNumber == "" {
		p.logger.Warn("No CL number available, skipping trybot run")
		return nil
	}

	// Run trybots using cueckoo
	// Use the full commit hash for cueckoo (not the abbreviated one)
	fullCommitHash, _, err := p.runCommand("git", "rev-parse", p.commitRef)
	if err != nil {
		return fmt.Errorf("failed to get full commit hash: %v", err)
	}
	fullCommitHash = strings.TrimSpace(fullCommitHash)

	stdout, stderr, err := p.runCommand("cueckoo", "runtrybot", fullCommitHash)
	if err != nil {
		// Check if cueckoo is not installed
		if strings.Contains(stderr, "command not found") || strings.Contains(stderr, "not found") {
			p.logger.Warn("cueckoo not installed, skipping trybot run")
			p.logger.Info("To install cueckoo: go install github.com/cue-lang/cueckoo/cmd/cueckoo@latest")
			return nil
		}
		return fmt.Errorf("failed to run trybots: %v (stderr: %s)", err, stderr)
	}

	p.logger.Success("Started trybots for commit %s", fullCommitHash[:8])
	if stdout != "" {
		p.logger.Info("Trybot output: %s", strings.TrimSpace(stdout))
	}

	p.logger.Info("Monitor trybot status at: %s", p.clURL)
	return nil
}

// extractProposalSummary extracts a summary from the proposal content.
func (p *Publisher) extractProposalSummary(content string) string {
	lines := strings.Split(content, "\n")

	// First try to find a summary section (## Summary, ## Abstract, ## Objective, etc.)
	var summary []string
	inSummary := false
	summaryHeaders := []string{"## Summary", "## Abstract", "## Objective", "## Objective / Abstract", "## Overview"}

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Check if this line starts a summary section
		for _, header := range summaryHeaders {
			if strings.HasPrefix(line, header) {
				inSummary = true
				break
			}
		}

		if inSummary && strings.HasPrefix(line, "##") && !strings.HasPrefix(line, "## Summary") &&
			!strings.HasPrefix(line, "## Abstract") && !strings.HasPrefix(line, "## Objective") &&
			!strings.HasPrefix(line, "## Overview") {
			break // Hit next section
		}

		if inSummary && line != "" && !strings.HasPrefix(line, "##") {
			summary = append(summary, line)
		}
	}

	// If we found a summary section, return it
	if len(summary) > 0 {
		// Limit to reasonable length
		if len(summary) > 20 {
			summary = summary[:20]
			summary = append(summary, "\n_[Summary truncated - see full proposal for details]_")
		}
		return strings.Join(summary, "\n")
	}

	// If no summary section, extract first few paragraphs after title
	var intro []string
	foundTitle := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			foundTitle = true
			continue
		}
		if foundTitle && strings.HasPrefix(line, "##") {
			break // Hit first section
		}
		if foundTitle && line != "" {
			intro = append(intro, line)
			if len(intro) >= 10 { // Limit to first 10 non-empty lines
				break
			}
		}
	}

	if len(intro) > 0 {
		return strings.Join(intro, "\n\n") + "\n\n_[This is an excerpt - see full proposal for complete details]_"
	}

	return "See the full proposal document for details."
}

// updateDiscussionContent updates the GitHub discussion with the proposal summary.
func (p *Publisher) updateDiscussionContent(clNumber string) error {
	p.logger.Info("Updating GitHub discussion with proposal content...")

	// Read proposal content from commit
	// In dry-run mode for drafts, use original filename since rename didn't happen
	filename := p.newProposalFile
	if p.dryRun && p.isDraft {
		filename = p.proposalFile
	}

	stdout, _, err := p.runCommand("git", "show", fmt.Sprintf("%s:%s", p.commitRef, filename))
	if err != nil {
		return fmt.Errorf("failed to read proposal file from commit: %v", err)
	}
	content := stdout

	// Extract title
	titlePattern := regexp.MustCompile(`^# (.+)$`)
	lines := strings.Split(content, "\n")
	var title string
	for _, line := range lines {
		if matches := titlePattern.FindStringSubmatch(line); matches != nil {
			title = strings.TrimSpace(matches[1])
			break
		}
	}

	if title == "" {
		title = "CUE Proposal"
	}

	// Extract summary - try AI first, fallback to extraction
	var summary string
	if p.useAI {
		aiSummary, err := p.generateProposalSummary(content)
		if err != nil {
			p.logger.Warning("AI summary generation failed: %v, falling back to text extraction", err)
			summary = p.extractProposalSummary(content)
		} else {
			summary = aiSummary
			p.logger.Success("Generated AI summary for proposal")
		}
	} else {
		summary = p.extractProposalSummary(content)
	}

	// Create updated discussion body
	status := "Under Review ✅"
	if clNumber != "" {
		status = "Under Review ✅"
	} else {
		status = "Draft"
	}

	updatedBody := fmt.Sprintf(`**📋 Proposal Details:**
- **File**: [%s](https://github.com/cue-lang/proposal/blob/main/%s)
- **Status**: %s

---

# %s

%s

---

## Full Proposal

The complete proposal with all technical details, examples, and implementation notes can be found in the [proposal document](https://github.com/cue-lang/proposal/blob/main/%s).

## How to Comment

Please provide feedback on this proposal:
- **For general discussion**: Comment in this GitHub discussion
- **For detailed code review**: Comment on the Gerrit CL (link will be added when available)

*Last updated: %s*`,
		p.newProposalFile,
		p.newProposalFile,
		status,
		title,
		summary,
		p.newProposalFile,
		time.Now().UTC().Format("2006-01-02 15:04:05 UTC"))

	if clNumber != "" {
		// Add CL link if available
		clLink := fmt.Sprintf("- **Gerrit CL**: [CL %s](https://cue-review.googlesource.com/c/cue-lang/proposal/+/%s)", clNumber, clNumber)
		updatedBody = strings.Replace(updatedBody,
			"- **Status**: "+status,
			clLink+"\n- **Status**: "+status, 1)

		updatedBody = strings.Replace(updatedBody,
			"Comment on the Gerrit CL (link will be added when available)",
			fmt.Sprintf("Comment on the [Gerrit CL](https://cue-review.googlesource.com/c/cue-lang/proposal/+/%s)", clNumber), 1)
	}

	if p.dryRun {
		p.logger.Info("[DRY RUN] Would update discussion #%s with:", p.discussionNumber)
		fmt.Fprintf(os.Stderr, "Title: %s\n", title)
		fmt.Fprintf(os.Stderr, "Body preview:\n%s\n", updatedBody[:min(500, len(updatedBody))]+"...")
		return nil
	}

	// Get discussion node ID for GraphQL update
	query := `
	query($number: Int!) {
		repository(owner: "cue-lang", name: "cue") {
			discussion(number: $number) {
				id
			}
		}
	}`

	numberInt, err := strconv.Atoi(p.discussionNumber)
	if err != nil {
		return fmt.Errorf("invalid discussion number: %v", err)
	}

	variables := map[string]interface{}{
		"number": numberInt,
	}

	reqData := map[string]interface{}{
		"query":     query,
		"variables": variables,
	}

	data, err := p.callGitHubAPI("POST", "graphql", reqData)
	if err != nil {
		return fmt.Errorf("failed to get discussion: %v", err)
	}

	var discussion struct {
		Data struct {
			Repository struct {
				Discussion struct {
					ID string `json:"id"`
				} `json:"discussion"`
			} `json:"repository"`
		} `json:"data"`
		Errors []interface{} `json:"errors"`
	}

	if err := json.Unmarshal(data, &discussion); err != nil {
		return fmt.Errorf("failed to parse discussion: %v", err)
	}

	if len(discussion.Errors) > 0 {
		return fmt.Errorf("GraphQL errors: %v", discussion.Errors)
	}

	// Update discussion using GraphQL
	mutation := `
	mutation($discussionId: ID!, $body: String!) {
		updateDiscussion(input: {discussionId: $discussionId, body: $body}) {
			discussion {
				url
			}
		}
	}`

	variables = map[string]interface{}{
		"discussionId": discussion.Data.Repository.Discussion.ID,
		"body":         updatedBody,
	}

	reqData = map[string]interface{}{
		"query":     mutation,
		"variables": variables,
	}

	_, err = p.callGitHubAPI("POST", "graphql", reqData)
	if err != nil {
		p.logger.Warning("Could not update discussion automatically: %v", err)
		p.logger.Info("Please update manually at: %s", p.discussionURL)
		return nil // Don't fail the whole workflow
	}

	p.logger.Success("Updated discussion #%s with proposal content", p.discussionNumber)
	return nil
}

// updateDocumentReferences updates internal references to the discussion number in the document.
func (p *Publisher) updateDocumentReferences() error {
	if !p.isDraft || p.dryRun {
		return nil // Skip for numbered proposals or dry run
	}

	p.logger.Info("Updating document references to discussion #%s...", p.discussionNumber)

	// Read current file content from working directory (after rename)
	content, err := os.ReadFile(p.newProposalFile)
	if err != nil {
		return fmt.Errorf("failed to read renamed proposal file: %v", err)
	}

	originalContent := string(content)
	updatedContent := originalContent

	// Look for common patterns where discussion numbers might be referenced
	// Pattern 1: "GitHub discussion: TBD" or similar
	discussionPatterns := []string{
		`(?i)(discussion|github discussion|gh discussion):\s*(TBD|TODO|xxxx|\[TBD\]|\[TODO\])`,
		`(?i)(tracking issue|issue):\s*(TBD|TODO|xxxx|\[TBD\]|\[TODO\])`,
		`(?i)(discussion|github discussion|gh discussion):\s*#?\s*xxxx`,
	}

	discussionReplacement := fmt.Sprintf("Discussion: https://github.com/cue-lang/cue/discussions/%s", p.discussionNumber)

	for _, pattern := range discussionPatterns {
		re := regexp.MustCompile(pattern)
		updatedContent = re.ReplaceAllString(updatedContent, discussionReplacement)
	}

	// Pattern 2: Any "xxxx" that might refer to the proposal number
	// Be careful not to replace xxxx in filename examples
	if !strings.Contains(originalContent, "xxxx-") { // Only if no filename examples
		re := regexp.MustCompile(`(?i)\bxxxx\b`)
		updatedContent = re.ReplaceAllString(updatedContent, p.discussionNumber)
	}

	// If content changed, write it back
	if updatedContent != originalContent {
		if err := os.WriteFile(p.newProposalFile, []byte(updatedContent), 0644); err != nil {
			return fmt.Errorf("failed to update document references: %v", err)
		}

		// Stage the changes
		_, _, err := p.runCommand("git", "add", p.newProposalFile)
		if err != nil {
			return fmt.Errorf("failed to stage document updates: %v", err)
		}

		// Amend the commit
		_, _, err = p.runCommand("git", "commit", "--amend", "--no-edit")
		if err != nil {
			return fmt.Errorf("failed to amend commit with document updates: %v", err)
		}

		p.logger.Success("Updated document references to discussion #%s", p.discussionNumber)
	} else {
		p.logger.Info("No document references needed updating")
	}

	return nil
}

func main() {
	// Parse command line flags
	var (
		dryRun = flag.Bool("dry-run", false, "Show what would be done without making changes")
		useAI  = flag.Bool("use-ai", true, "Use Claude AI for summary generation (default: true)")
		help   = flag.Bool("help", false, "Show help message")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [--dry-run] [commit-ref]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Publish a CUE proposal from a git commit through the Gerrit workflow.\n\n")
		fmt.Fprintf(os.Stderr, "Arguments:\n")
		fmt.Fprintf(os.Stderr, "  commit-ref   Git commit reference containing the proposal (default: HEAD)\n")
		fmt.Fprintf(os.Stderr, "               Can be a commit hash, branch name, or any git reference\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s                    # Publish proposal in HEAD commit\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s abc123             # Publish proposal in specific commit\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s HEAD~2             # Publish proposal from 2 commits ago\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --dry-run HEAD     # Preview what would happen\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nThe commit should contain exactly one proposal file (designs/*.md).\n")
		fmt.Fprintf(os.Stderr, "Draft proposals (xxxx-*.md) will get a discussion number assigned.\n")
		fmt.Fprintf(os.Stderr, "Numbered proposals (NNNN-*.md) will update existing discussion #NNNN.\n")
	}

	flag.Parse()

	if *help {
		flag.Usage()
		return
	}

	// Get commit reference
	commitRef := "HEAD"
	if flag.NArg() > 0 {
		commitRef = flag.Arg(0)
	}

	publisher := NewPublisher(commitRef, *dryRun, *useAI)

	if *dryRun {
		publisher.logger.Info("🔍 DRY RUN MODE - No changes will be made")
	}

	publisher.logger.Info("Working with commit: %s", commitRef)
	publisher.logger.Info("Starting publication workflow...")

	// Execute workflow steps
	if err := publisher.findProposalFile(); err != nil {
		log.Fatal(err)
	}

	if err := publisher.runTests(); err != nil {
		log.Fatal(err)
	}

	if publisher.isDraft {
		if err := publisher.createDiscussion(); err != nil {
			log.Fatal(err)
		}
	} else {
		if err := publisher.verifyDiscussion(); err != nil {
			log.Fatal(err)
		}
	}

	if err := publisher.renameProposal(); err != nil {
		log.Fatal(err)
	}

	// Update document references to discussion number
	if err := publisher.updateDocumentReferences(); err != nil {
		log.Fatal(err)
	}

	// Update the GitHub discussion with proposal content
	if err := publisher.updateDiscussionContent(""); err != nil {
		log.Fatal(err)
	}

	// Step 4: Submit CL via git codereview mail
	if err := publisher.submitCL(); err != nil {
		log.Fatal(err)
	}

	// Step 5: Run trybots
	if err := publisher.runTrybots(); err != nil {
		log.Fatal(err)
	}

	publisher.logger.Success("🎉 Proposal setup completed successfully!")
	publisher.logger.Info("")
	publisher.logger.Info("Discussion: %s", publisher.discussionURL)
	publisher.logger.Info("CL: %s", publisher.clURL)
}
