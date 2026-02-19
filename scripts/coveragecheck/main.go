package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	minimum := flag.Float64("min", 85, "minimum total coverage percentage")
	file := flag.String("file", "coverage.out", "go coverage profile path")
	flag.Parse()

	total, err := totalCoverage(*file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "coverage check failed: %v\n", err)
		os.Exit(1)
	}

	if total < *minimum {
		fmt.Fprintf(os.Stderr, "coverage %.2f%% is below required %.2f%%\n", total, *minimum)
		os.Exit(1)
	}

	fmt.Printf("coverage %.2f%% (min %.2f%%)\n", total, *minimum)
}

func totalCoverage(filePath string) (float64, error) {
	file, err := openCoverageFile(filePath)
	if err != nil {
		return 0, fmt.Errorf("open coverage file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	scanner := bufio.NewScanner(file)
	if err := consumeModeLine(scanner); err != nil {
		return 0, err
	}

	covered, total, err := collectCoverage(scanner)
	if err != nil {
		return 0, err
	}

	if total == 0 {
		return 0, fmt.Errorf("coverage profile has zero statements")
	}

	return (float64(covered) / float64(total)) * 100, nil
}

func collectCoverage(scanner *bufio.Scanner) (int64, int64, error) {
	var (
		covered int64
		total   int64
	)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		numStatements, count, err := parseCoverageLine(line)
		if err != nil {
			return 0, 0, err
		}

		total += numStatements
		if count > 0 {
			covered += numStatements
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, 0, fmt.Errorf("scan coverage file: %w", err)
	}

	return covered, total, nil
}

func openCoverageFile(filePath string) (*os.File, error) {
	//nolint:gosec // file path is provided by local CI/hook command configuration.
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", filePath, err)
	}

	return file, nil
}

func consumeModeLine(scanner *bufio.Scanner) error {
	if scanner.Scan() {
		return nil
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read coverage header: %w", err)
	}

	return fmt.Errorf("coverage profile is empty")
}

func parseCoverageLine(line string) (int64, int64, error) {
	parts := strings.Fields(line)
	if len(parts) != 3 {
		return 0, 0, fmt.Errorf("invalid coverage line: %q", line)
	}

	numStatements, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse statements in %q: %w", line, err)
	}

	count, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse count in %q: %w", line, err)
	}

	return numStatements, count, nil
}
