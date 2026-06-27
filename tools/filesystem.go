package tools

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var readFileTool = Tool{
	Name:        "read_file",
	Description: "Read the contents of a local file in the workspace directory",
	Args: map[string]Arg{
		"path": {
			Type:        "string",
			Description: "The relative path to the file",
		},
	},
	Required: []string{"path"},
	Handler:  handleReadFile,
}

func handleReadFile(args map[string]any) (string, error) {
	pathVal, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid required string argument 'path'")
	}
	content, err := os.ReadFile(pathVal)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

var grepFileTool = Tool{
	Name:        "grep_file",
	Description: "Search for lines matching a specific pattern inside a local file",
	Args: map[string]Arg{
		"path": {
			Type:        "string",
			Description: "The relative path to the file",
		},
		"pattern": {
			Type:        "string",
			Description: "The text substring or regular expression pattern to search for",
		},
		"mode": {
			Type:        "string",
			Description: "The matching strategy to use",
			Enum:        []string{"exact", "insensitive", "regex"},
		},
	},
	Required: []string{"path", "pattern"},
	Handler:  handleGrepFile,
}

func handleGrepFile(args map[string]any) (string, error) {
	pathVal, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid required string argument 'path'")
	}
	patternVal, ok := args["pattern"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid required string argument 'pattern'")
	}
	modeVal := "exact"
	if incomingMode, exists := args["mode"].(string); exists {
		modeVal = incomingMode
	}

	file, err := os.Open(pathVal)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var matches []string
	scanner := bufio.NewScanner(file)
	lineNum := 0

	var re *regexp.Regexp
	if modeVal == "regex" {
		re, err = regexp.Compile(patternVal)
		if err != nil {
			return "", fmt.Errorf("invalid regular expression: %w", err)
		}
	}

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		matched := false
		switch modeVal {
		case "exact":
			matched = strings.Contains(line, patternVal)
		case "insensitive":
			matched = strings.Contains(strings.ToLower(line), strings.ToLower(patternVal))
		case "regex":
			matched = re.MatchString(line)
		default:
			return "", fmt.Errorf("unsupported match mode: %s", modeVal)
		}
		if matched {
			matches = append(matches, fmt.Sprintf("%d: %s", lineNum, line))
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "No matches found.", nil
	}
	return strings.Join(matches, "\n"), nil
}

var lsDirTool = Tool{
	Name:        "ls_dir",
	Description: "List files and directories inside a target local folder",
	Args: map[string]Arg{
		"path": {
			Type:        "string",
			Description: "The relative path to the directory (or the current directory if empty)",
		},
	},
	Required: []string{},
	Handler:  handleListDir,
}

func handleListDir(args map[string]any) (string, error) {
	dirPath := "."
	if pathVal, exists := args["path"].(string); exists && pathVal != "" {
		dirPath = pathVal
	}
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return "", err
	}

	var result []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		result = append(result, name)
	}

	if len(result) == 0 {
		return "Directory is empty.", nil
	}
	return strings.Join(result, "\n"), nil
}
