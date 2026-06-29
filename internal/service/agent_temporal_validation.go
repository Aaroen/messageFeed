package service

import (
	"fmt"
	"messagefeed/internal/agent/timeintent"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	agentTemporalEvidenceCurrent      = "current"
	agentTemporalEvidenceUnknown      = "unknown"
	agentTemporalEvidenceFuture       = "future"
	agentTemporalEvidenceStale        = "stale"
	agentTemporalEvidenceDateConflict = "date_conflict"
	agentTemporalQueryFutureDate      = "future_date"
	agentTemporalQueryStaleDate       = "stale_date"
)

var (
	agentChineseFullDatePattern = regexp.MustCompile(`((?:19|20)\d{2})年\s*(\d{1,2})月\s*(\d{1,2})日?`)
	agentSeparatedDatePattern   = regexp.MustCompile(`((?:19|20)\d{2})[-/.](\d{1,2})[-/.](\d{1,2})`)
	agentCompactDatePattern     = regexp.MustCompile(`(?:^|[^0-9])((?:19|20)\d{2})(\d{2})(\d{2})(?:[^0-9]|$)`)
	agentMonthDayPattern        = regexp.MustCompile(`(\d{1,2})月\s*(\d{1,2})日?`)
)

type agentCalendarDate struct {
	Year    int
	Month   time.Month
	Day     int
	Raw     string
	HasYear bool
}

type agentTemporalValidationResult struct {
	OK       bool
	Status   string
	Reason   string
	Matched  []string
	Filtered int
}

// validateAgentToolTemporalRequest 只校验模型工具参数是否引入了未被用户授权的异常日期。
// 它不判断用户意图，也不根据市场、新闻等领域词表做分支，避免把语义规划重新固化到后端。
func validateAgentToolTemporalRequest(value string, userMessage string, now time.Time) agentTemporalValidationResult {
	dates := extractAgentCalendarDates(value, now, agentTimeLocation())
	if len(dates) == 0 {
		return agentTemporalValidationResult{OK: true, Status: agentTemporalEvidenceUnknown}
	}
	currentDay := agentStartOfLocalDay(now, agentTimeLocation())
	for _, date := range dates {
		dateDay := date.Time(agentTimeLocation())
		if dateDay.After(currentDay) && !agentUserMessageAllowsDate(userMessage, dateDay, now) {
			return agentTemporalValidationResult{
				OK:      false,
				Status:  agentTemporalQueryFutureDate,
				Reason:  "工具参数包含当前运行日之后的日期，但用户原始消息没有授权该未来日期。",
				Matched: []string{date.Raw},
			}
		}
		if currentDay.Sub(dateDay) > 30*24*time.Hour && !agentUserMessageAllowsDate(userMessage, dateDay, now) {
			return agentTemporalValidationResult{
				OK:      false,
				Status:  agentTemporalQueryStaleDate,
				Reason:  "工具参数包含明显早于当前任务窗口的日期，但用户原始消息没有授权该历史日期。",
				Matched: []string{date.Raw},
			}
		}
	}
	return agentTemporalValidationResult{OK: true, Status: agentTemporalEvidenceCurrent, Matched: agentCalendarDateLabels(dates)}
}

// assessAgentWebEvidenceTemporalStatus 评估网页证据的时间一致性。
// 返回值用于工具层过滤 future、stale、date_conflict 证据，让模型只基于时效可信的材料继续分析。
func assessAgentWebEvidenceTemporalStatus(query string, userMessage string, evidenceText string, publishedAt string, now time.Time) agentTemporalValidationResult {
	location := agentTimeLocation()
	currentDay := agentStartOfLocalDay(now, location)
	evidenceDates := extractAgentCalendarDates(strings.Join([]string{evidenceText, publishedAt}, " "), now, location)
	if publishedDate, ok := parseAgentPublishedDate(publishedAt, location); ok {
		evidenceDates = append(evidenceDates, agentCalendarDate{
			Year:    publishedDate.Year(),
			Month:   publishedDate.Month(),
			Day:     publishedDate.Day(),
			Raw:     publishedAt,
			HasYear: true,
		})
	}
	evidenceDates = compactAgentCalendarDates(evidenceDates)
	if len(evidenceDates) == 0 {
		return agentTemporalValidationResult{OK: true, Status: agentTemporalEvidenceUnknown}
	}
	desiredDates := compactAgentCalendarDates(append(
		extractAgentCalendarDates(query, now, location),
		extractAgentCalendarDates(userMessage, now, location)...,
	))
	for _, evidenceDate := range evidenceDates {
		evidenceDay := evidenceDate.Time(location)
		if evidenceDay.After(currentDay) && !agentUserMessageAllowsDate(userMessage, evidenceDay, now) {
			return agentTemporalValidationResult{
				OK:      false,
				Status:  agentTemporalEvidenceFuture,
				Reason:  "证据日期晚于当前运行日，且用户没有授权未来日期。",
				Matched: []string{evidenceDate.Raw},
			}
		}
		if agentDateConflictsWithDesiredDates(evidenceDate, desiredDates, location) {
			return agentTemporalValidationResult{
				OK:      false,
				Status:  agentTemporalEvidenceDateConflict,
				Reason:  "证据日期与工具查询或用户消息中的目标日期存在年份冲突。",
				Matched: []string{evidenceDate.Raw},
			}
		}
	}
	latest := latestAgentCalendarDate(evidenceDates, location)
	if !latest.IsZero() && currentDay.Sub(latest) > 7*24*time.Hour && !agentDateAllowedByText(userMessage, latest, now, location) {
		return agentTemporalValidationResult{
			OK:      false,
			Status:  agentTemporalEvidenceStale,
			Reason:  "证据日期早于当前运行日超过 7 天，且用户没有指定该历史日期。",
			Matched: agentCalendarDateLabels(evidenceDates),
		}
	}
	return agentTemporalValidationResult{OK: true, Status: agentTemporalEvidenceCurrent, Matched: agentCalendarDateLabels(evidenceDates)}
}

// filterAgentWebSearchResultsByTemporalEvidence 在工具返回给模型前剔除时间异常的搜索结果。
// 过滤摘要保留在工具文本中，便于 Web 流水线和模型复核为什么某些候选没有进入证据集。
func filterAgentWebSearchResultsByTemporalEvidence(query string, userMessage string, now time.Time, results []agentWebSearchResult) ([]agentWebSearchResult, map[string]int) {
	counts := map[string]int{
		agentTemporalEvidenceFuture:       0,
		agentTemporalEvidenceStale:        0,
		agentTemporalEvidenceDateConflict: 0,
	}
	filtered := make([]agentWebSearchResult, 0, len(results))
	for _, result := range results {
		status := assessAgentWebEvidenceTemporalStatus(
			query,
			userMessage,
			strings.Join([]string{result.Title, result.URL, result.Snippet}, " "),
			result.PublishedAt,
			now,
		)
		switch status.Status {
		case agentTemporalEvidenceFuture, agentTemporalEvidenceStale, agentTemporalEvidenceDateConflict:
			counts[status.Status]++
			continue
		default:
			filtered = append(filtered, result)
		}
	}
	return filtered, counts
}

// formatAgentTemporalFilterSummary 把时效过滤结果压缩为工具观察文本。
// 这是给模型和 Web 审计读取的结构化工程事实，不作为最终用户回复模板。
func formatAgentTemporalFilterSummary(counts map[string]int) string {
	if len(counts) == 0 {
		return "future=0, stale=0, date_conflict=0"
	}
	keys := []string{agentTemporalEvidenceFuture, agentTemporalEvidenceStale, agentTemporalEvidenceDateConflict}
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+strconv.Itoa(counts[key]))
	}
	return strings.Join(parts, ", ")
}

func extractAgentCalendarDates(text string, now time.Time, location *time.Location) []agentCalendarDate {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if location == nil {
		location = agentTimeLocation()
	}
	now = now.In(location)
	var dates []agentCalendarDate
	add := func(year int, month int, day int, raw string, hasYear bool) {
		if year < 1900 || year > 2100 || month < 1 || month > 12 || day < 1 || day > 31 {
			return
		}
		candidate := time.Date(year, time.Month(month), day, 0, 0, 0, 0, location)
		if candidate.Year() != year || int(candidate.Month()) != month || candidate.Day() != day {
			return
		}
		dates = append(dates, agentCalendarDate{Year: year, Month: time.Month(month), Day: day, Raw: strings.TrimSpace(raw), HasYear: hasYear})
	}
	for _, match := range agentChineseFullDatePattern.FindAllStringSubmatch(text, -1) {
		year, month, day := atoiAgentDatePart(match[1]), atoiAgentDatePart(match[2]), atoiAgentDatePart(match[3])
		add(year, month, day, match[0], true)
	}
	for _, match := range agentSeparatedDatePattern.FindAllStringSubmatch(text, -1) {
		year, month, day := atoiAgentDatePart(match[1]), atoiAgentDatePart(match[2]), atoiAgentDatePart(match[3])
		add(year, month, day, match[0], true)
	}
	for _, match := range agentCompactDatePattern.FindAllStringSubmatch(text, -1) {
		year, month, day := atoiAgentDatePart(match[1]), atoiAgentDatePart(match[2]), atoiAgentDatePart(match[3])
		add(year, month, day, match[1]+match[2]+match[3], true)
	}
	for _, match := range agentMonthDayPattern.FindAllStringSubmatch(text, -1) {
		month, day := atoiAgentDatePart(match[1]), atoiAgentDatePart(match[2])
		add(now.Year(), month, day, match[0], false)
	}
	return compactAgentCalendarDates(dates)
}

func compactAgentCalendarDates(dates []agentCalendarDate) []agentCalendarDate {
	if len(dates) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	result := make([]agentCalendarDate, 0, len(dates))
	for _, date := range dates {
		key := fmt.Sprintf("%04d-%02d-%02d-%t", date.Year, date.Month, date.Day, date.HasYear)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, date)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Time(agentTimeLocation()).Before(result[j].Time(agentTimeLocation()))
	})
	return result
}

func (d agentCalendarDate) Time(location *time.Location) time.Time {
	if location == nil {
		location = agentTimeLocation()
	}
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, location)
}

func latestAgentCalendarDate(dates []agentCalendarDate, location *time.Location) time.Time {
	var latest time.Time
	for _, date := range dates {
		current := date.Time(location)
		if latest.IsZero() || current.After(latest) {
			latest = current
		}
	}
	return latest
}

func agentCalendarDateLabels(dates []agentCalendarDate) []string {
	labels := make([]string, 0, len(dates))
	for _, date := range dates {
		if strings.TrimSpace(date.Raw) != "" {
			labels = append(labels, date.Raw)
		}
	}
	return compactNonEmptyStrings(labels)
}

func agentUserMessageAllowsDate(userMessage string, target time.Time, now time.Time) bool {
	return agentDateAllowedByText(userMessage, target, now, agentTimeLocation())
}

func agentDateAllowedByText(text string, target time.Time, now time.Time, location *time.Location) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	targetDay := agentStartOfLocalDay(target, location)
	for _, date := range extractAgentCalendarDates(text, now, location) {
		if date.HasYear && agentSameLocalDay(date.Time(location), targetDay, location) {
			return true
		}
		if !date.HasYear && date.Month == targetDay.Month() && date.Day == targetDay.Day() {
			return true
		}
	}
	parsed := timeintent.Parse(text, now, location)
	if parsed.HasInstant() {
		return agentSameLocalDay(parsed.InstantAt, targetDay, location)
	}
	if parsed.HasRange() {
		start := agentStartOfLocalDay(parsed.StartAt, location)
		end := agentStartOfLocalDay(parsed.EndAt.Add(-time.Nanosecond), location)
		return !targetDay.Before(start) && !targetDay.After(end)
	}
	return false
}

func agentDateConflictsWithDesiredDates(evidenceDate agentCalendarDate, desiredDates []agentCalendarDate, location *time.Location) bool {
	if len(desiredDates) == 0 {
		return false
	}
	for _, desired := range desiredDates {
		desiredDay := desired.Time(location)
		if desired.Month != evidenceDate.Month || desired.Day != evidenceDate.Day {
			continue
		}
		if desired.HasYear && desired.Year != evidenceDate.Year {
			return true
		}
		if !desired.HasYear && desiredDay.Year() != evidenceDate.Year {
			return true
		}
	}
	return false
}

func parseAgentPublishedDate(value string, location *time.Location) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	if location == nil {
		location = agentTimeLocation()
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return agentStartOfLocalDay(parsed, location), true
	}
	if parsed, err := time.Parse(time.RFC1123Z, value); err == nil {
		return agentStartOfLocalDay(parsed, location), true
	}
	if parsed, err := time.Parse(time.RFC1123, value); err == nil {
		return agentStartOfLocalDay(parsed, location), true
	}
	if parsed, err := time.Parse("2006-01-02 15:04:05", value); err == nil {
		return agentStartOfLocalDay(parsed, location), true
	}
	return time.Time{}, false
}

func agentStartOfLocalDay(value time.Time, location *time.Location) time.Time {
	if location == nil {
		location = agentTimeLocation()
	}
	value = value.In(location)
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, location)
}

func agentSameLocalDay(left time.Time, right time.Time, location *time.Location) bool {
	left = agentStartOfLocalDay(left, location)
	right = agentStartOfLocalDay(right, location)
	return left.Equal(right)
}

func atoiAgentDatePart(value string) int {
	parsed, _ := strconv.Atoi(strings.TrimSpace(value))
	return parsed
}
