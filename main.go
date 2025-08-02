package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
)

type ReviewComment struct {
	ID              int    `json:"id"`
	Body            string `json:"body"`
	Path            string `json:"path"`
	Line            *int   `json:"line"`
	StartLine       *int   `json:"start_line"`
	OriginalLine    *int   `json:"original_line,omitempty"`
	DiffHunk        string `json:"diff_hunk,omitempty"`
	Author          string `json:"author"`
	AuthorAssoc     string `json:"author_association,omitempty"`
	State           string `json:"state"`
	InReplyTo       *int   `json:"in_reply_to_id"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
	Outdated        bool   `json:"outdated,omitempty"`
	SubjectType     string `json:"subject_type,omitempty"`
}

type StatusCheck struct {
	Name         string `json:"name"`
	Status       string `json:"status"`
	Conclusion   string `json:"conclusion"`
	DetailsURL   string `json:"details_url"`
	WorkflowName string `json:"workflow_name,omitempty"`
	RunID        string `json:"run_id,omitempty"`
	StartedAt    string `json:"started_at"`
	CompletedAt  string `json:"completed_at"`
	CheckCommand string `json:"check_command,omitempty"`
}

type PRFeedback struct {
	PRNumber      int             `json:"pr_number"`
	Title         string          `json:"title"`
	URL           string          `json:"url"`
	Comments      []ReviewComment `json:"comments"`
	GeneralIssues []ReviewComment `json:"general_issues"`
	StatusChecks  []StatusCheck   `json:"status_checks"`
}

func main() {
	var jsonOutput bool
	var targetDir string
	var prNumber int
	var repoName string

	// Parse arguments
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		
		// Handle flags
		if arg == "--version" || arg == "-v" {
			fmt.Println("gh-pr-feedback v1.2.0")
			return
		}
		
		if arg == "--help" || arg == "-h" {
			printHelp()
			return
		}
		
		if arg == "--json" || arg == "-j" {
			jsonOutput = true
			continue
		}
		
		if arg == "--repo" || arg == "-R" {
			if i+1 < len(args) {
				repoName = args[i+1]
				i++ // Skip next arg
			} else {
				fmt.Fprintf(os.Stderr, "Error: --repo requires a value\n")
				os.Exit(1)
			}
			continue
		}
		
		// Handle positional argument (could be PR number or directory)
		if !strings.HasPrefix(arg, "-") {
			// Try to parse as PR number first
			if num, err := strconv.Atoi(arg); err == nil && num > 0 {
				prNumber = num
			} else if targetDir == "" {
				targetDir = arg
				// Validate directory exists
				if _, err := os.Stat(targetDir); os.IsNotExist(err) {
					fmt.Fprintf(os.Stderr, "Error: Directory '%s' does not exist\n", targetDir)
					os.Exit(1)
				}
			}
		}
	}
	
	if targetDir == "" {
		targetDir = "."
	}

	// Change to target directory if specified
	originalDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	if targetDir != "." {
		err = os.Chdir(targetDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error changing to directory '%s': %v\n", targetDir, err)
			os.Exit(1)
		}
		// Ensure we change back on exit
		defer func() {
			os.Chdir(originalDir)
		}()
	}

	client, err := api.DefaultRESTClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating GitHub client: %v\n", err)
		os.Exit(1)
	}

	// If PR number and repo are provided, use them directly
	if prNumber > 0 && repoName != "" {
		// Use provided PR number and repo
	} else if prNumber > 0 {
		// PR number provided but no repo - try to get repo from current directory
		_, currentRepo, err := getCurrentPR(client)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: PR number provided but couldn't determine repository.\n")
			fmt.Fprintf(os.Stderr, "Use --repo to specify the repository (e.g., --repo owner/name)\n")
			os.Exit(1)
		}
		repoName = currentRepo
	} else {
		// No PR number provided - get current PR
		currentPR, currentRepo, err := getCurrentPR(client)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			fmt.Fprintf(os.Stderr, "\nMake sure you're in a git repository with an open PR.\n")
			fmt.Fprintf(os.Stderr, "You can check PR status with: gh pr status\n")
			fmt.Fprintf(os.Stderr, "Or specify a PR number: gh pr-feedback 123 --repo owner/name\n")
			os.Exit(1)
		}
		prNumber = currentPR
		repoName = currentRepo
	}

	// Fetch PR details and review comments
	feedback, err := getPRFeedback(client, repoName, prNumber)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching PR feedback: %v\n", err)
		os.Exit(1)
	}

	// Output in requested format
	if jsonOutput {
		output, err := json.MarshalIndent(feedback, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(output))
	} else {
		printHumanReadable(feedback)
	}
}

func getCurrentPR(client *api.RESTClient) (int, string, error) {
	// Get PR for current branch
	cmd := exec.Command("gh", "pr", "view", "--json", "number")
	output, err := cmd.Output()
	if err != nil {
		return 0, "", fmt.Errorf("no PR found for current branch")
	}

	var pr struct {
		Number int `json:"number"`
	}

	err = json.Unmarshal(output, &pr)
	if err != nil {
		return 0, "", fmt.Errorf("failed to parse PR data: %w", err)
	}

	// Get repository name
	cmd = exec.Command("gh", "repo", "view", "--json", "nameWithOwner")
	output, err = cmd.Output()
	if err != nil {
		return 0, "", fmt.Errorf("failed to get repository info: %w", err)
	}

	var repo struct {
		NameWithOwner string `json:"nameWithOwner"`
	}

	err = json.Unmarshal(output, &repo)
	if err != nil {
		return 0, "", fmt.Errorf("failed to parse repository data: %w", err)
	}

	return pr.Number, repo.NameWithOwner, nil
}

func getPRFeedback(client *api.RESTClient, repo string, prNumber int) (*PRFeedback, error) {
	// Get PR details
	var pr struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		HTMLURL string `json:"html_url"`
	}
	endpoint := fmt.Sprintf("repos/%s/pulls/%d", repo, prNumber)
	err := client.Get(endpoint, &pr)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch PR details: %w", err)
	}

	feedback := &PRFeedback{
		PRNumber: pr.Number,
		Title:    pr.Title,
		URL:      pr.HTMLURL,
	}

	// Get review comments (line-specific comments)
	var reviewComments []struct {
		ID              int    `json:"id"`
		Body            string `json:"body"`
		Path            string `json:"path"`
		Line            *int   `json:"line"`
		StartLine       *int   `json:"start_line"`
		OriginalLine    *int   `json:"original_line"`
		DiffHunk        string `json:"diff_hunk"`
		AuthorAssoc     string `json:"author_association"`
		User            struct {
			Login string `json:"login"`
		} `json:"user"`
		InReplyToID     *int   `json:"in_reply_to_id"`
		CreatedAt       string `json:"created_at"`
		UpdatedAt       string `json:"updated_at"`
		Outdated        bool   `json:"outdated"`
		SubjectType     string `json:"subject_type"`
	}
	
	reviewEndpoint := fmt.Sprintf("repos/%s/pulls/%d/comments", repo, prNumber)
	err = client.Get(reviewEndpoint, &reviewComments)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch review comments: %w", err)
	}

	// Filter unresolved comments (not replies to other comments)
	for _, comment := range reviewComments {
		if comment.InReplyToID == nil { // Top-level comment, not a reply
			feedback.Comments = append(feedback.Comments, ReviewComment{
				ID:              comment.ID,
				Body:            comment.Body,
				Path:            comment.Path,
				Line:            comment.Line,
				StartLine:       comment.StartLine,
				OriginalLine:    comment.OriginalLine,
				DiffHunk:        comment.DiffHunk,
				Author:          comment.User.Login,
				AuthorAssoc:     comment.AuthorAssoc,
				State:           "unresolved",
				InReplyTo:       comment.InReplyToID,
				CreatedAt:       comment.CreatedAt,
				UpdatedAt:       comment.UpdatedAt,
				Outdated:        comment.Outdated,
				SubjectType:     comment.SubjectType,
			})
		}
	}

	// Get general PR comments (issue comments)
	var issueComments []struct {
		ID         int    `json:"id"`
		Body       string `json:"body"`
		AuthorAssoc string `json:"author_association"`
		User       struct {
			Login string `json:"login"`
		} `json:"user"`
		CreatedAt  string `json:"created_at"`
		UpdatedAt  string `json:"updated_at"`
	}
	
	issueEndpoint := fmt.Sprintf("repos/%s/issues/%d/comments", repo, prNumber)
	err = client.Get(issueEndpoint, &issueComments)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch issue comments: %w", err)
	}

	// Add all general PR comments (not line-specific)
	for _, comment := range issueComments {
		feedback.GeneralIssues = append(feedback.GeneralIssues, ReviewComment{
			ID:          comment.ID,
			Body:        comment.Body,
			Author:      comment.User.Login,
			AuthorAssoc: comment.AuthorAssoc,
			State:       "unresolved",
			CreatedAt:   comment.CreatedAt,
			UpdatedAt:   comment.UpdatedAt,
		})
	}

	// Get PR reviews
	var reviews []struct {
		ID         int    `json:"id"`
		Body       string `json:"body"`
		State      string `json:"state"`
		User       struct {
			Login string `json:"login"`
		} `json:"user"`
		AuthorAssoc string `json:"author_association"`
		SubmittedAt string `json:"submitted_at"`
	}
	
	reviewsEndpoint := fmt.Sprintf("repos/%s/pulls/%d/reviews", repo, prNumber)
	err = client.Get(reviewsEndpoint, &reviews)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch reviews: %w", err)
	}

	// Add review summary comments
	for _, review := range reviews {
		if review.Body != "" && review.State == "COMMENTED" {
			feedback.GeneralIssues = append(feedback.GeneralIssues, ReviewComment{
				ID:          review.ID,
				Body:        review.Body,
				Author:      review.User.Login,
				AuthorAssoc: review.AuthorAssoc,
				State:       "unresolved",
				CreatedAt:   review.SubmittedAt,
				UpdatedAt:   review.SubmittedAt,
			})
		}
	}

	// Get status checks
	statusChecks, err := getStatusChecks(repo, prNumber)
	if err != nil {
		// Don't fail the whole operation if status checks fail
		fmt.Fprintf(os.Stderr, "Warning: failed to fetch status checks: %v\n", err)
	} else {
		feedback.StatusChecks = statusChecks
	}

	return feedback, nil
}

func getStatusChecks(repo string, prNumber int) ([]StatusCheck, error) {
	// Use gh CLI to get status checks
	cmd := exec.Command("gh", "pr", "view", strconv.Itoa(prNumber), "--repo", repo, "--json", "statusCheckRollup")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get status checks: %w", err)
	}

	var result struct {
		StatusCheckRollup []struct {
			Name         string `json:"name"`
			Status       string `json:"status"`
			Conclusion   string `json:"conclusion"`
			DetailsURL   string `json:"detailsUrl"`
			WorkflowName string `json:"workflowName"`
			StartedAt    string `json:"startedAt"`
			CompletedAt  string `json:"completedAt"`
		} `json:"statusCheckRollup"`
	}

	err = json.Unmarshal(output, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse status checks: %w", err)
	}

	var statusChecks []StatusCheck
	for _, check := range result.StatusCheckRollup {
		// Only include failed or errored checks
		if check.Conclusion == "FAILURE" || check.Conclusion == "ERROR" || check.Conclusion == "CANCELLED" {
			statusCheck := StatusCheck{
				Name:         check.Name,
				Status:       check.Status,
				Conclusion:   check.Conclusion,
				DetailsURL:   check.DetailsURL,
				WorkflowName: check.WorkflowName,
				StartedAt:    check.StartedAt,
				CompletedAt:  check.CompletedAt,
			}

			// Extract run ID if it's a GitHub Actions workflow
			if strings.Contains(check.DetailsURL, "/actions/runs/") {
				runID := extractRunID(check.DetailsURL)
				if runID != "" {
					statusCheck.RunID = runID
					statusCheck.CheckCommand = fmt.Sprintf("gh run view %s", runID)
				}
			}

			statusChecks = append(statusChecks, statusCheck)
		}
	}

	return statusChecks, nil
}

func extractRunID(detailsURL string) string {
	// Extract run ID from URL: https://github.com/owner/repo/actions/runs/{run_id}/job/{job_id}
	parts := strings.Split(detailsURL, "/")
	for i, part := range parts {
		if part == "runs" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}


func printHelp() {
	fmt.Println("Usage: gh pr-feedback [flags] [pr-number|directory]")
	fmt.Println("Extracts unresolved review feedback from a PR")
	fmt.Println("")
	fmt.Println("Arguments:")
	fmt.Println("  pr-number        PR number to view feedback for")
	fmt.Println("  directory        Path to git repository (default: current directory)")
	fmt.Println("")
	fmt.Println("Flags:")
	fmt.Println("  -h, --help       Show help")
	fmt.Println("  -j, --json       Output in JSON format")
	fmt.Println("  -R, --repo       Repository name (owner/name)")
	fmt.Println("  -v, --version    Show version")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  gh pr-feedback                      # Current PR in current directory")
	fmt.Println("  gh pr-feedback 117                  # PR 117 in current repo")
	fmt.Println("  gh pr-feedback 117 --repo owner/name  # PR 117 in specified repo")
	fmt.Println("  gh pr-feedback /path/to/repo        # Current PR in specified directory")
}

func printHumanReadable(feedback *PRFeedback) {
	// Calculate counts
	commentCount := len(feedback.Comments) + len(feedback.GeneralIssues)
	checkCount := len(feedback.StatusChecks)
	
	// PR Title and metadata
	fmt.Printf("%s%s #%d%s\n", colorBold, feedback.Title, feedback.PRNumber, colorReset)
	fmt.Printf("%sOpen%s • %s\n", colorGreen, colorReset, colorGray + feedback.URL + colorReset)
	
	// Feedback summary
	if commentCount > 0 || checkCount > 0 {
		fmt.Printf("\n")
		if commentCount > 0 && checkCount > 0 {
			fmt.Printf("%s!%s Found %d unresolved comment(s) and %d failing check(s)\n", colorYellow, colorReset, commentCount, checkCount)
		} else if commentCount > 0 {
			fmt.Printf("%s!%s Found %d unresolved comment(s)\n", colorYellow, colorReset, commentCount)
		} else if checkCount > 0 {
			fmt.Printf("%sX%s Found %d failing check(s)\n", colorRed, colorReset, checkCount)
		}
	}
	fmt.Println()

	// Review Comments Section
	if len(feedback.Comments) > 0 || len(feedback.GeneralIssues) > 0 {
		// First show general review comments
		if len(feedback.GeneralIssues) > 0 {
			for _, review := range feedback.GeneralIssues {
				// Review header like GitHub
				fmt.Printf("%s%s%s commented %s(%s)%s • %s", 
					colorBold, review.Author, colorReset,
					colorGray, strings.Title(strings.ToLower(review.AuthorAssoc)), colorReset,
					colorGray)
				
				if review.CreatedAt != "" {
					if t, err := parseTime(review.CreatedAt); err == nil {
						ago := formatTimeAgo(time.Since(t))
						fmt.Printf("%s", ago)
					}
				}
				fmt.Printf("%s\n\n", colorReset)
				
				// Review body
				lines := strings.Split(review.Body, "\n")
				for _, line := range lines {
					fmt.Printf("%s\n", line)
				}
				fmt.Println()
			}
		}
		
		// Then show file-specific comments
		if len(feedback.Comments) > 0 {
			fmt.Println(strings.Repeat("─", 100))
			fmt.Println()
			
			for i, comment := range feedback.Comments {
				// Author and metadata on one line
				fmt.Printf("%s%s%s", colorBold, comment.Author, colorReset)
				if comment.AuthorAssoc != "" && comment.AuthorAssoc != "NONE" {
					fmt.Printf(" • %s", colorGray + strings.ToLower(comment.AuthorAssoc) + colorReset)
				}
				if comment.CreatedAt != "" {
					if t, err := parseTime(comment.CreatedAt); err == nil {
						ago := formatTimeAgo(time.Since(t))
						fmt.Printf(" • %s%s%s", colorGray, ago, colorReset)
					}
				}
				if comment.Outdated {
					fmt.Printf(" %s• Outdated%s", colorYellow, colorReset)
				}
				fmt.Println("\n")
				
				// Comment body
				lines := strings.Split(comment.Body, "\n")
				for _, line := range lines {
					fmt.Printf("%s\n", line)
				}
				fmt.Println()
				
				// File location in a box
				if comment.Path != "" {
					fmt.Printf("%s%s", colorBlue, comment.Path)
					if comment.Line != nil && *comment.Line > 0 {
						fmt.Printf(" on line %d", *comment.Line)
					}
					fmt.Printf("%s\n", colorReset)
					
					// Show diff context
					if comment.DiffHunk != "" && !comment.Outdated {
						fmt.Println()
						printDiffHunk(comment.DiffHunk)
					}
				}
				
				// Separator between comments
				if i < len(feedback.Comments)-1 {
					fmt.Println("\n" + strings.Repeat("─", 100) + "\n")
				}
			}
		}
	}


	// Status Checks Section
	if len(feedback.StatusChecks) > 0 {
		fmt.Println("\n" + strings.Repeat("─", 100) + "\n")
		fmt.Printf("%sFailed Checks%s\n\n", colorBold, colorReset)
		
		for _, check := range feedback.StatusChecks {
			symbol := "✗"
			symbolColor := colorRed
			if check.Conclusion == "CANCELLED" {
				symbol = "⊘"
				symbolColor = colorYellow
			}
			
			fmt.Printf("%s%s%s %s", symbolColor, symbol, colorReset, check.Name)
			
			// Duration
			if check.StartedAt != "" && check.CompletedAt != "" {
				start, _ := parseTime(check.StartedAt)
				end, _ := parseTime(check.CompletedAt)
				if !start.IsZero() && !end.IsZero() {
					diff := end.Sub(start)
					fmt.Printf(" %s(took %s)%s", colorGray, formatDuration(diff), colorReset)
				}
			}
			
			if check.CheckCommand != "" {
				fmt.Printf(" → %s%s%s", colorCyan, check.CheckCommand, colorReset)
			}
			fmt.Println()
		}
	}

}

func parseTime(timeStr string) (time.Time, error) {
	return time.Parse(time.RFC3339, timeStr)
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

func printDiffHunk(diffHunk string) {
	lines := strings.Split(diffHunk, "\n")
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		
		switch line[0] {
		case '+':
			fmt.Printf("    %s%s%s\n", colorGreen, line, colorReset)
		case '-':
			fmt.Printf("    %s%s%s\n", colorRed, line, colorReset)
		case '@':
			fmt.Printf("    %s%s%s\n", colorCyan, line, colorReset)
		default:
			fmt.Printf("    %s\n", line)
		}
	}
}

func formatTimeAgo(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day ago"
	}
	if days < 30 {
		return fmt.Sprintf("%d days ago", days)
	}
	if days < 365 {
		months := days / 30
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	}
	years := days / 365
	if years == 1 {
		return "1 year ago"
	}
	return fmt.Sprintf("%d years ago", years)
}