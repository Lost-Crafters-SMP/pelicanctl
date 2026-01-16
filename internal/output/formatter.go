// Package output provides output formatting for the Pelican CLI.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/jedib0t/go-pretty/v6/table"
)

// OutputFormat represents the output format.
//
//nolint:revive // Type name stutters but is acceptable for clarity when used as output.OutputFormat
type OutputFormat string

const (
	OutputFormatTable OutputFormat = "table"
	OutputFormatJSON  OutputFormat = "json"
)

// ResourceType identifies the type of resource.
type ResourceType string

const (
	ResourceTypeClientServer   ResourceType = "client.server"
	ResourceTypeAdminServer    ResourceType = "admin.server"
	ResourceTypeAdminNode      ResourceType = "admin.node"
	ResourceTypeAdminUser      ResourceType = "admin.user"
	ResourceTypeClientBackup   ResourceType = "client.backup"
	ResourceTypeClientDatabase ResourceType = "client.database"
	ResourceTypeClientFile     ResourceType = "client.file"
	ResourceTypeServerResource ResourceType = "client.server.resources"
)

// TableConfig defines which fields to show for a specific resource type.
type TableConfig struct {
	Fields  []string // Field names to display (supports dot notation for nested)
	Headers []string // Display names for headers (optional, defaults to field names)
}

const (
	maxArrayKeyWidth  = 20
	maxTruncateLength = 50
	maxStringLength   = 100
	defaultMaxDepth   = 10
	defaultKeyWidth   = 25
	defaultIndentSize = 2
)

var (
	// Styles for table output.
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	warningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	infoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))

	// tableConfigs defines field mappings for each resource type.
	tableConfigs = map[ResourceType]TableConfig{
		ResourceTypeClientServer: {
			Fields:  []string{"id", "uuid", "attributes.name"},
			Headers: []string{"ID", "UUID", "Name"},
		},
		ResourceTypeAdminServer: {
			Fields:  []string{"id", "uuid", "attributes.name", "attributes.node"},
			Headers: []string{"ID", "UUID", "Name", "Node"},
		},
		ResourceTypeAdminNode: {
			Fields:  []string{"id", "attributes.name"},
			Headers: []string{"ID", "Name"},
		},
		ResourceTypeAdminUser: {
			Fields:  []string{"id", "attributes.email", "attributes.username"},
			Headers: []string{"ID", "Email", "Username"},
		},
		ResourceTypeClientBackup: {
			Fields:  []string{"uuid", "name", "created_at"},
			Headers: []string{"UUID", "Name", "Created At"},
		},
		ResourceTypeClientDatabase: {
			Fields:  []string{"name", "username"},
			Headers: []string{"Name", "Username"},
		},
		ResourceTypeClientFile: {
			Fields:  []string{"name", "type"},
			Headers: []string{"Name", "Type"},
		},
		ResourceTypeServerResource: {
			Fields:  []string{"state", "resources.memory_bytes", "resources.cpu_absolute"},
			Headers: []string{"State", "Memory", "CPU"},
		},
	}
)

// Formatter handles output formatting.
type Formatter struct {
	format OutputFormat
	writer io.Writer
}

// NewFormatter creates a new formatter.
func NewFormatter(format OutputFormat, writer io.Writer) *Formatter {
	return &Formatter{
		format: format,
		writer: writer,
	}
}

// Print formats and prints data based on the format type.
func (f *Formatter) Print(data any) error {
	switch f.format {
	case OutputFormatJSON:
		return f.printJSON(data)
	case OutputFormatTable:
		return f.printTable(data)
	default:
		return f.printTable(data)
	}
}

// PrintWithConfig formats and prints data with explicit resource type configuration.
func (f *Formatter) PrintWithConfig(data any, resourceType ResourceType) error {
	if f.format == OutputFormatJSON {
		return f.printJSON(data)
	}

	// Handle []map[string]any (list views)
	if list, ok := data.([]map[string]any); ok && len(list) > 0 {
		return f.printListTableWithConfig(list, resourceType)
	}

	// Handle map[string]any (detail views)
	if m, ok := data.(map[string]any); ok {
		return f.printFormattedDetail(m)
	}

	// Fallback to generic printTable
	return f.printTable(data)
}

// printJSON prints data as formatted JSON.
func (f *Formatter) printJSON(data any) error {
	encoder := json.NewEncoder(f.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// printTable prints data as a table (fallback to JSON for complex types).
func (f *Formatter) printTable(data any) error {
	// Handle strings directly
	if str, ok := data.(string); ok {
		_, err := fmt.Fprintln(f.writer, str)
		return err
	}

	// Handle []map[string]any (list views)
	if list, ok := data.([]map[string]any); ok && len(list) > 0 {
		return f.printListTable(list)
	}

	// Handle map[string]any (detail views)
	if m, ok := data.(map[string]any); ok {
		return f.printFormattedDetail(m)
	}

	// Handle []string
	if strList, ok := data.([]string); ok {
		for _, s := range strList {
			if _, err := fmt.Fprintln(f.writer, s); err != nil {
				return err
			}
		}
		return nil
	}

	// Fallback to JSON for unrecognized types
	encoder := json.NewEncoder(f.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// printListTable prints a list of maps as a table with selected fields.
func (f *Formatter) printListTable(list []map[string]any) error {
	if len(list) == 0 {
		return nil
	}

	// Select important fields for list view
	fields := f.selectListFields(list)
	if len(fields) == 0 {
		// If no fields match, show all available fields from first item
		for k := range list[0] {
			fields = append(fields, k)
		}
		sort.Strings(fields)
	}

	// Build headers
	headers := fields

	// Build rows
	rows := make([][]string, len(list))
	for i, item := range list {
		row := make([]string, len(fields))
		for j, field := range fields {
			val := f.formatValue(item[field])
			row[j] = val
		}
		rows[i] = row
	}

	return f.PrintTable(headers, rows)
}

// printListTableWithConfig prints a list of maps using explicit resource type configuration.
func (f *Formatter) printListTableWithConfig(list []map[string]any, resourceType ResourceType) error {
	if len(list) == 0 {
		return nil
	}

	// Get table configuration for this resource type
	config, ok := tableConfigs[resourceType]
	if !ok {
		// Fallback to generic detection if no config found
		return f.printListTable(list)
	}

	// Use configured fields or fallback to all available fields
	fields := config.Fields
	if len(fields) == 0 {
		// Extract all unique keys
		for k := range list[0] {
			fields = append(fields, k)
		}
		sort.Strings(fields)
	}

	// Use configured headers or derive from field names
	headers := config.Headers
	if len(headers) == 0 || len(headers) != len(fields) {
		headers = make([]string, len(fields))
		for i, field := range fields {
			// Use last part of dot notation as header name
			parts := strings.Split(field, ".")
			lastPart := parts[len(parts)-1]
			// Capitalize first letter
			if len(lastPart) > 0 {
				headers[i] = strings.ToUpper(lastPart[:1]) + lastPart[1:]
			} else {
				headers[i] = lastPart
			}
		}
	}

	// Convert headers to table.Row
	headerRow := make(table.Row, len(headers))
	for i, h := range headers {
		headerRow[i] = h
	}

	// Build rows using field extraction with dot notation
	rows := make([]table.Row, len(list))
	for i, item := range list {
		row := make(table.Row, len(fields))
		for j, field := range fields {
			val := f.extractField(item, field)
			row[j] = val
		}
		rows[i] = row
	}

	return f.printPrettyTable(headerRow, rows)
}

// extractField extracts a field value using dot notation for nested fields.
// Also handles fallback: if field not found, tries "attributes.{field}" path.
func (f *Formatter) extractField(item map[string]any, fieldPath string) string {
	// Try direct path first
	val := f.getNestedField(item, fieldPath)
	if val != nil {
		return f.formatValue(val)
	}

	// Try attributes.{field} as fallback
	if !strings.Contains(fieldPath, ".") {
		attrsPath := "attributes." + fieldPath
		val = f.getNestedField(item, attrsPath)
		if val != nil {
			return f.formatValue(val)
		}
	}

	// Also try direct top-level field
	if directVal, ok := item[fieldPath]; ok {
		return f.formatValue(directVal)
	}

	return "-"
}

// getNestedField extracts a nested field using dot notation.
func (f *Formatter) getNestedField(item map[string]any, fieldPath string) any {
	parts := strings.Split(fieldPath, ".")
	val := any(item)

	for _, part := range parts {
		switch v := val.(type) {
		case map[string]any:
			var found bool
			val, found = v[part]
			if !found {
				return nil
			}
		case map[any]any:
			var found bool
			val, found = v[part]
			if !found {
				return nil
			}
		default:
			return nil
		}
	}

	return val
}

// printFormattedDetail prints a map in a formatted key-value style similar to kubectl describe.
func (f *Formatter) printFormattedDetail(m map[string]any) error {
	output := f.formatNestedMap(m, 0, defaultMaxDepth, defaultKeyWidth, defaultIndentSize)
	_, err := fmt.Fprint(f.writer, output)
	return err
}

// formatNestedMap formats a map with proper indentation and nesting.
//
//nolint:gocognit // Complex formatting logic requires high cognitive complexity
func (f *Formatter) formatNestedMap(m map[string]any, depth int, maxDepth int, keyWidth int, indentSize int) string {
	if depth > maxDepth {
		return "[...]\n"
	}

	indent := strings.Repeat(" ", depth*indentSize)
	var result strings.Builder

	// Sort keys for consistent output
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Separate simple fields from nested structures
	var simpleFields []string
	var nestedFields []string

	for _, key := range keys {
		val := m[key]
		if f.isNestedStructure(val) {
			nestedFields = append(nestedFields, key)
		} else {
			simpleFields = append(simpleFields, key)
		}
	}

	// Print simple fields first
	for _, key := range simpleFields {
		val := m[key]
		formattedKey := f.formatKey(key, keyWidth, indentSize)
		formattedVal := f.formatDetailValue(val)
		result.WriteString(fmt.Sprintf("%s%s: %s\n", indent, formattedKey, formattedVal))
	}

	// Print nested structures
	for _, key := range nestedFields {
		val := m[key]
		formattedKey := f.formatKey(key, keyWidth, indentSize)

		switch v := val.(type) {
		case map[string]any:
			if len(v) == 0 {
				result.WriteString(fmt.Sprintf("%s%s: {}\n", indent, formattedKey))
			} else {
				result.WriteString(fmt.Sprintf("%s%s:\n", indent, formattedKey))
				nested := f.formatNestedMap(v, depth+1, maxDepth, keyWidth, indentSize)
				result.WriteString(nested)
			}
		case map[any]any:
			converted := make(map[string]any)
			for k, val := range v {
				if kStr, ok := k.(string); ok {
					converted[kStr] = val
				} else {
					converted[fmt.Sprintf("%v", k)] = val
				}
			}
			if len(converted) == 0 {
				result.WriteString(fmt.Sprintf("%s%s: {}\n", indent, formattedKey))
			} else {
				result.WriteString(fmt.Sprintf("%s%s:\n", indent, formattedKey))
				nested := f.formatNestedMap(converted, depth+1, maxDepth, keyWidth, indentSize)
				result.WriteString(nested)
			}
		case []any:
			if len(v) == 0 {
				result.WriteString(fmt.Sprintf("%s%s: []\n", indent, formattedKey))
			} else {
				result.WriteString(fmt.Sprintf("%s%s:\n", indent, formattedKey))
				arrayOutput := f.formatArray(v, depth+1, maxDepth, indentSize)
				result.WriteString(arrayOutput)
			}
		default:
			// Fallback for other types
			formattedVal := f.formatDetailValue(val)
			result.WriteString(fmt.Sprintf("%s%s: %s\n", indent, formattedKey, formattedVal))
		}
	}

	return result.String()
}

// formatKey formats a key name with proper width.
func (f *Formatter) formatKey(key string, keyWidth int, _ int) string {
	// Convert snake_case and camelCase to Title Case
	formatted := f.humanizeKey(key)
	if len(formatted) <= keyWidth {
		return formatted
	}
	return formatted[:keyWidth]
}

// humanizeKey converts key names to more readable format.
func (f *Formatter) humanizeKey(key string) string {
	// Handle common abbreviations
	replacements := map[string]string{
		"id":   "ID",
		"uuid": "UUID",
		"url":  "URL",
		"api":  "API",
		"cpu":  "CPU",
		"ram":  "RAM",
		"gb":   "GB",
		"mb":   "MB",
	}

	lower := strings.ToLower(key)
	if replacement, ok := replacements[lower]; ok {
		return replacement
	}

	// Convert snake_case to Title Case
	if strings.Contains(key, "_") {
		parts := strings.Split(key, "_")
		var result strings.Builder
		for i, part := range parts {
			if i > 0 {
				result.WriteString(" ")
			}
			if len(part) > 0 {
				result.WriteString(strings.ToUpper(part[:1]) + strings.ToLower(part[1:]))
			}
		}
		return result.String()
	}

	// Convert camelCase to Title Case
	if len(key) == 0 {
		return key
	}

	var result strings.Builder
	for i, r := range key {
		switch {
		case i == 0:
			result.WriteRune(unicode.ToUpper(r))
		case unicode.IsUpper(r):
			result.WriteString(" ")
			result.WriteRune(r)
		default:
			result.WriteRune(r)
		}
	}
	return result.String()
}

// formatArray formats an array with proper indentation.
//
//nolint:gocognit // Complex formatting logic requires high cognitive complexity
func (f *Formatter) formatArray(arr []any, depth int, maxDepth int, indentSize int) string {
	if depth > maxDepth {
		return "[...]\n"
	}

	indent := strings.Repeat(" ", depth*indentSize)
	var result strings.Builder

	for i, item := range arr {
		switch v := item.(type) {
		case map[string]any:
			result.WriteString(fmt.Sprintf("%s- ", indent))
			nested := f.formatNestedMap(v, depth, maxDepth, maxArrayKeyWidth, indentSize)
			// Remove the indent from first line since we already have the "- "
			lines := strings.Split(strings.TrimRight(nested, "\n"), "\n")
			if len(lines) > 0 {
				firstLine := strings.TrimPrefix(lines[0], indent)
				result.WriteString(firstLine)
				result.WriteString("\n")
				for _, line := range lines[1:] {
					if line != "" {
						result.WriteString(line)
						result.WriteString("\n")
					}
				}
			}
		case map[any]any:
			converted := make(map[string]any)
			for k, val := range v {
				if kStr, ok := k.(string); ok {
					converted[kStr] = val
				} else {
					converted[fmt.Sprintf("%v", k)] = val
				}
			}
			result.WriteString(fmt.Sprintf("%s- ", indent))
			nested := f.formatNestedMap(converted, depth, maxDepth, maxArrayKeyWidth, indentSize)
			lines := strings.Split(strings.TrimRight(nested, "\n"), "\n")
			if len(lines) > 0 {
				firstLine := strings.TrimPrefix(lines[0], indent)
				result.WriteString(firstLine)
				result.WriteString("\n")
				for _, line := range lines[1:] {
					if line != "" {
						result.WriteString(line)
						result.WriteString("\n")
					}
				}
			}
		default:
			formattedVal := f.formatDetailValue(item)
			result.WriteString(fmt.Sprintf("%s- %s\n", indent, formattedVal))
		}

		// Add blank line between array items if they're complex
		if i < len(arr)-1 && f.isNestedStructure(item) {
			result.WriteString("\n")
		}
	}

	return result.String()
}

// formatDetailValue formats a single value for detail view.
func (f *Formatter) formatDetailValue(val any) string {
	if val == nil {
		return "<none>"
	}

	switch v := val.(type) {
	case string:
		// Truncate very long strings
		if len(v) > maxStringLength {
			return v[:maxStringLength-3] + "..."
		}
		return v
	case int:
		return strconv.Itoa(v)
	case int8:
		return strconv.FormatInt(int64(v), 10)
	case int16:
		return strconv.FormatInt(int64(v), 10)
	case int32:
		return strconv.Itoa(int(v))
	case int64:
		return strconv.FormatInt(v, 10)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case float32:
		if v == float32(int32(v)) {
			return strconv.Itoa(int(int32(v)))
		}
		return fmt.Sprintf("%.2f", v)
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return fmt.Sprintf("%.2f", v)
	case bool:
		return strconv.FormatBool(v)
	case []any:
		if len(v) == 0 {
			return "[]"
		}
		// For short arrays, show inline; for longer ones, use multiline
		if len(v) <= 3 && !f.hasNestedStructures(v) {
			parts := make([]string, len(v))
			for i, item := range v {
				parts[i] = f.formatDetailValue(item)
			}
			return "[" + strings.Join(parts, ", ") + "]"
		}
		return fmt.Sprintf("[%d items]", len(v))
	case map[string]any, map[any]any:
		return "{...}"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// isNestedStructure checks if a value is a nested structure (map or array).
func (f *Formatter) isNestedStructure(val any) bool {
	switch val.(type) {
	case map[string]any, map[any]any, []any:
		return true
	default:
		return false
	}
}

// hasNestedStructures checks if an array contains nested structures.
func (f *Formatter) hasNestedStructures(arr []any) bool {
	return slices.ContainsFunc(arr, f.isNestedStructure)
}

// selectListFields returns important fields to display for a list view.
//
//nolint:gocognit // Field selection logic requires high cognitive complexity
func (f *Formatter) selectListFields(list []map[string]any) []string {
	if len(list) == 0 {
		return nil
	}

	// Collect all unique keys from all items
	allKeys := make(map[string]bool)
	for _, item := range list {
		for k := range item {
			allKeys[k] = true
		}
	}

	// Try to detect resource type and select appropriate fields
	// Check first item for common identifiers
	first := list[0]

	// Common field patterns to try
	var fields []string

	// Servers (uuid, name, status)
	if _, hasUUID := first["uuid"]; hasUUID {
		fields = append(fields, "uuid")
		if _, hasName := first["name"]; hasName {
			fields = append(fields, "name")
		}
		if _, hasStatus := first["status"]; hasStatus {
			fields = append(fields, "status")
		}
		return fields
	}

	// Nodes/Users (id, name/email/username)
	if _, hasID := first["id"]; hasID {
		fields = append(fields, "id")
		for _, key := range []string{"name", "email", "username"} {
			if _, has := first[key]; has {
				fields = append(fields, key)
			}
		}
		return fields
	}

	// Backups (uuid, name/filename, created_at/date)
	if _, hasUUID := first["uuid"]; hasUUID {
		fields = append(fields, "uuid")
		for _, key := range []string{"name", "filename"} {
			if _, has := first[key]; has {
				fields = append(fields, key)
				break
			}
		}
		for _, key := range []string{"created_at", "date"} {
			if _, has := first[key]; has {
				fields = append(fields, key)
				break
			}
		}
		return fields
	}

	// Databases (name, username)
	if _, hasName := first["name"]; hasName {
		fields = append(fields, "name")
		if _, hasUsername := first["username"]; hasUsername {
			fields = append(fields, "username")
		}
		return fields
	}

	// Files (name, type)
	if _, hasName := first["name"]; hasName {
		fields = append(fields, "name")
		if _, hasType := first["type"]; hasType {
			fields = append(fields, "type")
		}
		return fields
	}

	// If no pattern matches, return empty to trigger fallback (all fields)
	return nil
}

// formatValue formats a value for display in a table cell.
func (f *Formatter) formatValue(val any) string {
	if val == nil {
		return "-"
	}

	switch v := val.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case int8:
		return strconv.FormatInt(int64(v), 10)
	case int16:
		return strconv.FormatInt(int64(v), 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case float32:
		// Check if it's a whole number (no fractional part)
		if v == float32(int32(v)) {
			return strconv.Itoa(int(int32(v)))
		}
		return fmt.Sprintf("%.2f", v)
	case float64:
		// Check if it's a whole number (no fractional part)
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return fmt.Sprintf("%.2f", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case []any, map[string]any:
		// For nested objects/arrays, convert to JSON string
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		// Truncate if too long
		jsonStr := string(jsonBytes)
		if len(jsonStr) > maxTruncateLength {
			return jsonStr[:maxTruncateLength-3] + "..."
		}
		return jsonStr
	default:
		return fmt.Sprintf("%v", v)
	}
}

// PrintTable prints a table with headers and rows.
func (f *Formatter) PrintTable(headers []string, rows [][]string) error {
	if f.format == OutputFormatJSON {
		// Convert table to JSON array of objects
		data := make([]map[string]string, len(rows))
		for i, row := range rows {
			data[i] = make(map[string]string)
			for j, header := range headers {
				if j < len(row) {
					data[i][header] = row[j]
				}
			}
		}
		return f.printJSON(data)
	}

	// Convert [][]string to []table.Row
	tableRows := make([]table.Row, len(rows))
	for i, row := range rows {
		tableRow := make(table.Row, len(row))
		for j, cell := range row {
			tableRow[j] = cell
		}
		tableRows[i] = tableRow
	}

	// Convert headers to table.Row
	headerRow := make(table.Row, len(headers))
	for i, h := range headers {
		headerRow[i] = h
	}

	return f.printPrettyTable(headerRow, tableRows)
}

// printPrettyTable prints a table using go-pretty.
func (f *Formatter) printPrettyTable(headers table.Row, rows []table.Row) error {
	t := table.NewWriter()
	t.SetOutputMirror(f.writer)
	t.AppendHeader(headers)
	t.AppendRows(rows)
	t.SetStyle(table.StyleColoredBright)
	t.Style().Options.SeparateRows = false
	t.Style().Options.DrawBorder = true
	t.Style().Options.SeparateColumns = true
	t.Render()
	return nil
}

// PrintSuccess prints a success message.
func (f *Formatter) PrintSuccess(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if f.format == OutputFormatJSON {
		// In JSON mode, write status messages to stderr for pipeability
		encoder := json.NewEncoder(os.Stderr)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(map[string]string{"status": "success", "message": msg})
		return
	}
	_, _ = fmt.Fprintln(f.writer, successStyle.Render("✓ "+msg))
}

// PrintError prints an error message.
func (f *Formatter) PrintError(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if f.format == OutputFormatJSON {
		// In JSON mode, write status messages to stderr for pipeability
		encoder := json.NewEncoder(os.Stderr)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(map[string]string{"status": "error", "message": msg})
		return
	}
	_, _ = fmt.Fprintln(f.writer, errorStyle.Render("✗ "+msg))
}

// PrintWarning prints a warning message.
func (f *Formatter) PrintWarning(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if f.format == OutputFormatJSON {
		// In JSON mode, write status messages to stderr for pipeability
		encoder := json.NewEncoder(os.Stderr)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(map[string]string{"status": "warning", "message": msg})
		return
	}
	_, _ = fmt.Fprintln(f.writer, warningStyle.Render("⚠ "+msg))
}

// PrintInfo prints an info message.
func (f *Formatter) PrintInfo(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if f.format == OutputFormatJSON {
		// In JSON mode, write status messages to stderr for pipeability
		encoder := json.NewEncoder(os.Stderr)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(map[string]string{"status": "info", "message": msg})
		return
	}
	_, _ = fmt.Fprintln(f.writer, infoStyle.Render("ℹ "+msg))
}
