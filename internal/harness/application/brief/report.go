package brief

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"kode-stream/internal/harness/domain"
)

var ErrReportMissing = errors.New("report_missing")

const fenceOpen = "```json harness-report"

func ReadReport(worktreePath string, finalText string, role domain.Role) (domain.Report, error) {
	data, err := readReportFile(filepath.Join(worktreePath, ReportPath))
	if err != nil || len(bytes.TrimSpace(data)) == 0 {
		data = fencedReport(finalText)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return domain.Report{}, ErrReportMissing
	}
	if len(data) > domain.MaxReportBytes {
		return domain.Report{}, fmt.Errorf("%w: the report is larger than %d bytes", ErrReportMissing, domain.MaxReportBytes)
	}
	var report domain.Report
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&report); err != nil {
		return domain.Report{}, fmt.Errorf("%w: %v", ErrReportMissing, err)
	}
	return Validate(report, role)
}

// A report left by an earlier run must not let the next run pass.
func ClearReport(worktreePath string) error {
	err := os.Remove(filepath.Join(worktreePath, ReportPath))
	if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	if err == nil {
		err = os.MkdirAll(filepath.Join(worktreePath, filepath.Dir(ReportPath)), 0o755)
	}
	return err
}

func readReportFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("the report is not a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(io.LimitReader(file, domain.MaxReportBytes+1))
}

func fencedReport(text string) []byte {
	start := strings.LastIndex(text, fenceOpen)
	if start < 0 {
		return nil
	}
	rest := text[start+len(fenceOpen):]
	end := strings.Index(rest, "```")
	if end < 0 {
		return nil
	}
	return []byte(strings.TrimSpace(rest[:end]))
}

func Validate(report domain.Report, role domain.Role) (domain.Report, error) {
	if report.Schema == "" {
		report.Schema = domain.ReportSchema
	}
	if report.Schema != domain.ReportSchema {
		return report, fmt.Errorf("%w: schema %q is not %s", ErrReportMissing, report.Schema, domain.ReportSchema)
	}
	report.Status = strings.TrimSpace(report.Status)
	if report.Status == "" {
		return report, fmt.Errorf("%w: status is required", ErrReportMissing)
	}
	report.Summary = clip(report.Summary, domain.MaxSummaryChars)
	switch report.Difficulty {
	case "", "low", "medium", "high":
	default:
		report.Difficulty = "medium"
	}
	if role == domain.RolePlanReviewer || role == domain.RoleCodeReviewer {
		if report.Verdict != domain.VerdictApprove && report.Verdict != domain.VerdictChangesRequested {
			return report, fmt.Errorf("%w: verdict must be approve or changes_requested", ErrReportMissing)
		}
	}
	findings := make([]domain.Finding, 0, len(report.Findings))
	for i, finding := range report.Findings {
		if i >= 100 {
			break
		}
		if finding.Severity != "blocking" {
			finding.Severity = "advisory"
		}
		finding.Title = clip(finding.Title, 200)
		finding.Path = clip(finding.Path, 512)
		finding.Detail = clip(finding.Detail, domain.MaxDetailChars)
		if finding.Title == "" {
			continue
		}
		findings = append(findings, finding)
	}
	report.Findings = findings
	report.Subtasks = clipAll(report.Subtasks, 50, 300)
	report.FilesChanged = clipAll(report.FilesChanged, 500, 512)
	report.Open = clipAll(report.Open, 20, 600)
	report.PlanPath = clip(report.PlanPath, 512)
	report.PlanDrift = domain.Text(clip(string(report.PlanDrift), domain.MaxDetailChars))
	if report.Gate != nil {
		report.Gate.Command = clip(report.Gate.Command, 512)
	}
	return report, nil
}

func Blocking(findings []domain.Finding) (blocking, advisory int) {
	for _, finding := range findings {
		if finding.Severity == "blocking" {
			blocking++
		} else {
			advisory++
		}
	}
	return blocking, advisory
}

func clip(value string, limit int) string {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	return string([]rune(value)[:limit])
}

func clipAll(values []string, count, limit int) []string {
	out := []string{}
	for _, value := range values {
		if len(out) >= count {
			break
		}
		if value = clip(value, limit); value != "" {
			out = append(out, value)
		}
	}
	return out
}
