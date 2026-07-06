package writeguard

// Package writeguard validates guarded writes.

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"plan-manager/internal/common/models"
)

var (
	branchNamePattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)
	serviceNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
)

func ValidateStatus(status models.ItemStatus) error {
	for _, allowed := range models.StatusOrder {
		if status == allowed {
			return nil
		}
	}
	return fmt.Errorf("invalid item status %q", status)
}

func ValidateBranchName(branch string) error {
	branch = strings.TrimSpace(branch)
	switch {
	case branch == "":
		return fmt.Errorf("branch name is required")
	case strings.Contains(branch, ".."):
		return fmt.Errorf("branch name must not contain '..'")
	case strings.Contains(branch, "//"):
		return fmt.Errorf("branch name must not contain empty path segments")
	case strings.HasPrefix(branch, "/"), strings.HasSuffix(branch, "/"):
		return fmt.Errorf("branch name must not start or end with '/'")
	case strings.HasSuffix(branch, "."):
		return fmt.Errorf("branch name must not end with '.'")
	case strings.HasSuffix(branch, ".lock"):
		return fmt.Errorf("branch name must not end with '.lock'")
	case strings.Contains(branch, `\`), strings.Contains(branch, " "), strings.Contains(branch, "~"), strings.Contains(branch, "^"), strings.Contains(branch, ":"), strings.Contains(branch, "?"), strings.Contains(branch, "*"), strings.Contains(branch, "["):
		return fmt.Errorf("branch name contains invalid characters")
	case !branchNamePattern.MatchString(branch):
		return fmt.Errorf("branch name contains invalid characters")
	default:
		return nil
	}
}

func ValidateCommitMessage(message string) error {
	message = strings.TrimSpace(message)
	switch {
	case message == "":
		return fmt.Errorf("commit message is required")
	case len(message) > 500:
		return fmt.Errorf("commit message is too long")
	default:
		return nil
	}
}

func ValidateScopeName(service string) error {
	service = strings.TrimSpace(service)
	switch {
	case service == "":
		return fmt.Errorf("service name is required")
	case service == "." || service == "..":
		return fmt.Errorf("service name is invalid")
	case strings.Contains(service, "/"), strings.Contains(service, `\`):
		return fmt.Errorf("service name must be one path segment")
	case !serviceNamePattern.MatchString(service):
		return fmt.Errorf("service name contains invalid characters")
	default:
		return nil
	}
}

func ValidateIdentifierName(item string) error {
	item = strings.TrimSpace(item)
	switch {
	case item == "":
		return fmt.Errorf("item name is required")
	case item == "." || item == "..":
		return fmt.Errorf("item name is invalid")
	case strings.Contains(item, "/"), strings.Contains(item, `\`):
		return fmt.Errorf("item name must be one path segment")
	case strings.ContainsAny(item, `<>:"|?*`):
		return fmt.Errorf("item name contains invalid characters")
	case strings.IndexFunc(item, unicode.IsControl) >= 0:
		return fmt.Errorf("item name contains invalid characters")
	default:
		return nil
	}
}
