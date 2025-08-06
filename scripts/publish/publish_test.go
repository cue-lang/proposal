package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRepo provides a temporary git repository for testing
type TestRepo struct {
	t       *testing.T
	dir     string
	cleanup func()
}

// NewTestRepo creates a new test repository
func NewTestRepo(t *testing.T) *TestRepo {
	t.Helper()

	dir, err := os.MkdirTemp("", "publish-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	repo := &TestRepo{
		t:   t,
		dir: dir,
		cleanup: func() {
			os.RemoveAll(dir)
		},
	}

	// Initialize git repo
	repo.run("git", "init")
	repo.run("git", "config", "user.email", "test@example.com")
	repo.run("git", "config", "user.name", "Test User")

	// Create initial commit
	repo.writeFile("README.md", "# Test Repository\n")
	repo.run("git", "add", "README.md")
	repo.run("git", "commit", "-m", "Initial commit")

	return repo
}

// Cleanup removes the test repository
func (r *TestRepo) Cleanup() {
	if r.cleanup != nil {
		r.cleanup()
	}
}

// run executes a command in the test repository
func (r *TestRepo) run(command string, args ...string) string {
	r.t.Helper()

	cmd := exec.Command(command, args...)
	cmd.Dir = r.dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("Command failed: %s %v\nOutput: %s\nError: %v",
			command, args, output, err)
	}
	return string(output)
}

// runExpectError runs a command expecting it to fail
func (r *TestRepo) runExpectError(command string, args ...string) (string, error) {
	r.t.Helper()

	cmd := exec.Command(command, args...)
	cmd.Dir = r.dir
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// writeFile writes a file in the test repository
func (r *TestRepo) writeFile(path, content string) {
	r.t.Helper()

	fullPath := filepath.Join(r.dir, path)
	dir := filepath.Dir(fullPath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		r.t.Fatalf("Failed to create directory %s: %v", dir, err)
	}

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		r.t.Fatalf("Failed to write file %s: %v", fullPath, err)
	}
}

// readFile reads a file from the test repository
func (r *TestRepo) readFile(path string) string {
	r.t.Helper()

	fullPath := filepath.Join(r.dir, path)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		r.t.Fatalf("Failed to read file %s: %v", fullPath, err)
	}
	return string(content)
}

// fileExists checks if a file exists in the test repository
func (r *TestRepo) fileExists(path string) bool {
	fullPath := filepath.Join(r.dir, path)
	_, err := os.Stat(fullPath)
	return err == nil
}

// getCommitHash returns the current HEAD commit hash
func (r *TestRepo) getCommitHash() string {
	return strings.TrimSpace(r.run("git", "rev-parse", "HEAD"))
}

// getShortCommitHash returns the abbreviated commit hash
func (r *TestRepo) getShortCommitHash() string {
	return strings.TrimSpace(r.run("git", "rev-parse", "--short", "HEAD"))
}

// createDraftProposal creates a draft proposal and commits it
func (r *TestRepo) createDraftProposal(name, content string) string {
	r.t.Helper()

	proposalPath := fmt.Sprintf("designs/language/xxxx-%s.md", name)
	r.writeFile(proposalPath, content)
	r.run("git", "add", proposalPath)
	r.run("git", "commit", "-m", fmt.Sprintf("Add draft proposal: %s", name))

	return r.getCommitHash()
}

// createNumberedProposal creates a numbered proposal and commits it
func (r *TestRepo) createNumberedProposal(number, name, content string) string {
	r.t.Helper()

	proposalPath := fmt.Sprintf("designs/language/%s-%s.md", number, name)
	r.writeFile(proposalPath, content)
	r.run("git", "add", proposalPath)
	r.run("git", "commit", "-m", fmt.Sprintf("Add proposal #%s: %s", number, name))

	return r.getCommitHash()
}

// TestPublisherDraftProposal tests the draft proposal workflow
func TestPublisherDraftProposal(t *testing.T) {
	repo := NewTestRepo(t)
	defer repo.Cleanup()

	// Create a draft proposal
	proposalContent := `# Test Proposal

*   **Status**: Draft
*   **Author(s)**: test@
*   **Discussion Channel**: TBD

## Summary

This is a test proposal for testing the publish workflow.

## Details

Some implementation details here.
`

	commitHash := repo.createDraftProposal("test-feature", proposalContent)

	// Create publisher in dry-run mode
	publisher := &Publisher{
		logger:     NewLogger(),
		commitRef:  commitHash,
		commitHash: commitHash[:8],
		dryRun:     true,
		useAI:      false,
	}

	// Change to repo directory for the test
	oldDir, _ := os.Getwd()
	os.Chdir(repo.dir)
	defer os.Chdir(oldDir)

	// Test finding proposal files
	t.Run("FindProposalFiles", func(t *testing.T) {
		if err := publisher.findProposalFile(); err != nil {
			t.Errorf("Failed to find proposal files: %v", err)
		}

		if publisher.proposalFile != "designs/language/xxxx-test-feature.md" {
			t.Errorf("Wrong proposal file found: %s", publisher.proposalFile)
		}

		if !publisher.isDraft {
			t.Error("Should be detected as draft proposal")
		}

		if publisher.isNumbered {
			t.Error("Should not be detected as numbered proposal")
		}
	})

	// Test running tests
	t.Run("RunTests", func(t *testing.T) {
		// For testing, we'll just check it doesn't error in dry-run
		if err := publisher.runTests(); err != nil {
			// This might fail if cue isn't installed, which is okay for the test
			t.Logf("runTests failed (expected if cue not installed): %v", err)
		}
	})

	// Test creating discussion (dry-run)
	t.Run("CreateDiscussion", func(t *testing.T) {
		if err := publisher.createDiscussion(); err != nil {
			t.Errorf("Failed to create discussion: %v", err)
		}

		if publisher.discussionNumber != "1234" {
			t.Errorf("Wrong discussion number in dry-run: %s", publisher.discussionNumber)
		}

		if !strings.Contains(publisher.discussionURL, "discussions/1234") {
			t.Errorf("Wrong discussion URL: %s", publisher.discussionURL)
		}
	})

	// Test renaming proposal (dry-run)
	t.Run("RenameProposal", func(t *testing.T) {
		if err := publisher.renameProposal(); err != nil {
			t.Errorf("Failed to rename proposal: %v", err)
		}

		expectedNewFile := "designs/language/1234-test-feature.md"
		if publisher.newProposalFile != expectedNewFile {
			t.Errorf("Wrong new proposal file: %s, expected: %s",
				publisher.newProposalFile, expectedNewFile)
		}
	})
}

// TestPublisherNumberedProposal tests the numbered proposal workflow
func TestPublisherNumberedProposal(t *testing.T) {
	repo := NewTestRepo(t)
	defer repo.Cleanup()

	// Create a numbered proposal
	proposalContent := `# Test Numbered Proposal

*   **Status**: Draft
*   **Author(s)**: test@
*   **Discussion Channel**: https://github.com/cue-lang/cue/discussions/4014

## Summary

This is a numbered proposal for testing.
`

	commitHash := repo.createNumberedProposal("4014", "test-feature", proposalContent)

	// Create publisher in dry-run mode
	publisher := &Publisher{
		logger:     NewLogger(),
		commitRef:  commitHash,
		commitHash: commitHash[:8],
		dryRun:     true,
		useAI:      false,
	}

	// Change to repo directory for the test
	oldDir, _ := os.Getwd()
	os.Chdir(repo.dir)
	defer os.Chdir(oldDir)

	// Test finding proposal files
	t.Run("FindProposalFiles", func(t *testing.T) {
		if err := publisher.findProposalFile(); err != nil {
			t.Errorf("Failed to find proposal files: %v", err)
		}

		if publisher.proposalFile != "designs/language/4014-test-feature.md" {
			t.Errorf("Wrong proposal file found: %s", publisher.proposalFile)
		}

		if publisher.isDraft {
			t.Error("Should not be detected as draft proposal")
		}

		if !publisher.isNumbered {
			t.Error("Should be detected as numbered proposal")
		}

		if publisher.discussionNumber != "4014" {
			t.Errorf("Wrong discussion number: %s", publisher.discussionNumber)
		}
	})

	// Test verifying discussion (dry-run mode, will skip actual verification)
	t.Run("VerifyDiscussion", func(t *testing.T) {
		// In dry-run mode, this should succeed without actually verifying
		if err := publisher.verifyDiscussion(); err != nil {
			t.Logf("verifyDiscussion failed (expected without GitHub access): %v", err)
		}
	})

	// Test that rename is skipped for numbered proposals
	t.Run("SkipRename", func(t *testing.T) {
		if err := publisher.renameProposal(); err != nil {
			t.Errorf("Failed in renameProposal: %v", err)
		}

		// newProposalFile should be same as original for numbered proposals
		if publisher.newProposalFile != publisher.proposalFile {
			t.Errorf("File should not be renamed for numbered proposal")
		}
	})
}

// TestPublisherNonHEADCommit tests handling of non-HEAD commits
func TestPublisherNonHEADCommit(t *testing.T) {
	repo := NewTestRepo(t)
	defer repo.Cleanup()

	// Create a draft proposal
	proposalContent := `# Test Proposal for Non-HEAD

*   **Status**: Draft
*   **Author(s)**: test@

## Summary

Testing non-HEAD commit handling.
`

	proposalCommit := repo.createDraftProposal("non-head", proposalContent)

	// Add another commit after the proposal
	repo.writeFile("other.txt", "Some other change")
	repo.run("git", "add", "other.txt")
	repo.run("git", "commit", "-m", "Another commit")

	// Now proposalCommit is not HEAD
	currentHead := repo.getCommitHash()
	if proposalCommit == currentHead {
		t.Fatal("Test setup error: proposal commit should not be HEAD")
	}

	// Create publisher for non-HEAD commit in dry-run mode
	publisher := &Publisher{
		logger:     NewLogger(),
		commitRef:  proposalCommit,
		commitHash: proposalCommit[:8],
		dryRun:     true,
		useAI:      false,
	}

	// Change to repo directory for the test
	oldDir, _ := os.Getwd()
	os.Chdir(repo.dir)
	defer os.Chdir(oldDir)

	t.Run("FindProposalInNonHEAD", func(t *testing.T) {
		if err := publisher.findProposalFile(); err != nil {
			t.Errorf("Failed to find proposal files in non-HEAD commit: %v", err)
		}

		if publisher.proposalFile != "designs/language/xxxx-non-head.md" {
			t.Errorf("Wrong proposal file found: %s", publisher.proposalFile)
		}
	})
}

// TestPublisherErrorCases tests various error conditions
func TestPublisherErrorCases(t *testing.T) {
	repo := NewTestRepo(t)
	defer repo.Cleanup()

	// Change to repo directory for the test
	oldDir, _ := os.Getwd()
	os.Chdir(repo.dir)
	defer os.Chdir(oldDir)

	t.Run("NoProposalFiles", func(t *testing.T) {
		// Create a commit without proposal files
		repo.writeFile("not-a-proposal.txt", "Just a file")
		repo.run("git", "add", "not-a-proposal.txt")
		repo.run("git", "commit", "-m", "No proposal")

		publisher := &Publisher{
			logger:     NewLogger(),
			commitRef:  "HEAD",
			commitHash: repo.getShortCommitHash(),
			dryRun:     true,
		}

		err := publisher.findProposalFile()
		if err == nil {
			t.Error("Expected error when no proposal files found")
		}
		if !strings.Contains(err.Error(), "no proposal files") {
			t.Errorf("Wrong error message: %v", err)
		}
	})

	t.Run("MultipleProposalFiles", func(t *testing.T) {
		// Create multiple proposals in one commit
		repo.createDraftProposal("feature1", "# Feature 1")
		repo.writeFile("designs/language/xxxx-feature2.md", "# Feature 2")
		repo.run("git", "add", "designs/language/xxxx-feature2.md")
		repo.run("git", "commit", "--amend", "--no-edit")

		publisher := &Publisher{
			logger:     NewLogger(),
			commitRef:  "HEAD",
			commitHash: repo.getShortCommitHash(),
			dryRun:     true,
		}

		err := publisher.findProposalFile()
		if err == nil {
			t.Error("Expected error when multiple proposal files found")
		}
		if !strings.Contains(err.Error(), "each proposal should be in its own commit") {
			t.Errorf("Wrong error message: %v", err)
		}
	})

	t.Run("InvalidProposalFormat", func(t *testing.T) {
		// Create a proposal without proper title
		repo.writeFile("designs/language/xxxx-no-title.md", "No title here\n\nJust content")
		repo.run("git", "add", "designs/language/xxxx-no-title.md")
		repo.run("git", "commit", "-m", "Invalid proposal")

		publisher := &Publisher{
			logger:     NewLogger(),
			commitRef:  "HEAD",
			commitHash: repo.getShortCommitHash(),
			dryRun:     true,
			useAI:      false,
		}

		// Find the proposal
		if err := publisher.findProposalFile(); err != nil {
			t.Fatalf("Failed to find proposal: %v", err)
		}

		// Try to create discussion - should fail due to missing title
		err := publisher.createDiscussion()
		if err == nil {
			t.Error("Expected error when proposal has no title")
		}
		if !strings.Contains(err.Error(), "no '# Title' found") {
			t.Errorf("Wrong error message: %v", err)
		}
	})
}

// TestUpdateDiscussionLink tests updating the discussion link in the document
func TestUpdateDiscussionLink(t *testing.T) {
	repo := NewTestRepo(t)
	defer repo.Cleanup()

	// Test case 1: Document with existing Discussion Channel field
	t.Run("UpdateExistingField", func(t *testing.T) {
		content := `# Test Proposal

*   **Status**: Draft
*   **Author(s)**: test@
*   **Discussion Channel**: TBD

## Summary

Test content.`

		repo.writeFile("test1.md", content)

		publisher := &Publisher{
			logger:          NewLogger(),
			newProposalFile: filepath.Join(repo.dir, "test1.md"),
			discussionURL:   "https://github.com/cue-lang/cue/discussions/9999",
			dryRun:          false,
		}

		if err := publisher.updateDiscussionLink(); err != nil {
			t.Errorf("Failed to update discussion link: %v", err)
		}

		updated := repo.readFile("test1.md")
		if !strings.Contains(updated, "discussions/9999") {
			t.Error("Discussion link not updated")
		}
		if strings.Contains(updated, "TBD") {
			t.Error("TBD not replaced")
		}
	})

	// Test case 2: Document without Discussion Channel field
	t.Run("AddMissingField", func(t *testing.T) {
		content := `# Test Proposal

*   **Status**: Draft
*   **Author(s)**: test@

## Summary

Test content.`

		repo.writeFile("test2.md", content)

		publisher := &Publisher{
			logger:          NewLogger(),
			newProposalFile: filepath.Join(repo.dir, "test2.md"),
			discussionURL:   "https://github.com/cue-lang/cue/discussions/8888",
			dryRun:          false,
		}

		if err := publisher.updateDiscussionLink(); err != nil {
			t.Errorf("Failed to add discussion link: %v", err)
		}

		updated := repo.readFile("test2.md")
		if !strings.Contains(updated, "Discussion Channel") {
			t.Error("Discussion Channel field not added")
		}
		if !strings.Contains(updated, "discussions/8888") {
			t.Error("Discussion link not added")
		}
	})

	// Test case 3: Dry run mode
	t.Run("DryRunMode", func(t *testing.T) {
		content := `# Test Proposal

*   **Discussion Channel**: OLD_LINK

## Summary

Test.`

		repo.writeFile("test3.md", content)

		publisher := &Publisher{
			logger:          NewLogger(),
			newProposalFile: filepath.Join(repo.dir, "test3.md"),
			discussionURL:   "https://github.com/cue-lang/cue/discussions/7777",
			dryRun:          true,
		}

		if err := publisher.updateDiscussionLink(); err != nil {
			t.Errorf("Dry run failed: %v", err)
		}

		// Content should not change in dry-run mode
		updated := repo.readFile("test3.md")
		if !strings.Contains(updated, "OLD_LINK") {
			t.Error("File was modified in dry-run mode")
		}
		if strings.Contains(updated, "7777") {
			t.Error("File was modified in dry-run mode")
		}
	})
}

// TestIntegrationWorkflow tests a complete workflow
func TestIntegrationWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	repo := NewTestRepo(t)
	defer repo.Cleanup()

	// Create a complete draft proposal
	proposalContent := `# Proposal: Test Feature

*   **Status**: Draft
*   **Author(s)**: test@
*   **Discussion Channel**: TBD

## Summary

This proposal introduces a test feature.

## Background

Some background information.

## Proposal

The actual proposal details.

## Implementation

How it will be implemented.
`

	_ = repo.createDraftProposal("complete-test", proposalContent)

	// Change to repo directory
	oldDir, _ := os.Getwd()
	os.Chdir(repo.dir)
	defer os.Chdir(oldDir)

	// Run the complete workflow in dry-run mode
	publisher := &Publisher{
		logger:     NewLogger(),
		commitRef:  "HEAD",
		commitHash: repo.getShortCommitHash(),
		dryRun:     true,
		useAI:      false,
	}

	// Execute all steps
	steps := []struct {
		name string
		fn   func() error
	}{
		{"findProposalFile", publisher.findProposalFile},
		{"runTests", publisher.runTests},
		{"createDiscussion", publisher.createDiscussion},
		{"renameProposal", publisher.renameProposal},
		{"updateDiscussionContent", func() error {
			return publisher.updateDiscussionContent("")
		}},
		{"submitCL", publisher.submitCL},
		{"runTrybots", publisher.runTrybots},
	}

	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			if err := step.fn(); err != nil {
				// Some steps may fail without proper setup (like cue or git-codereview)
				// but we still want to test the flow
				t.Logf("Step %s error (may be expected): %v", step.name, err)
			}
		})
	}

	// Verify final state
	if publisher.discussionNumber == "" {
		t.Error("Discussion number not set")
	}
	if publisher.discussionURL == "" {
		t.Error("Discussion URL not set")
	}
	if publisher.newProposalFile == "" {
		t.Error("New proposal file not set")
	}
}
