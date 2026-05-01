package auth

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
)

type BlackListProvider interface {
	// Check if the given subject is blacklisted,
	// returns `true` if the `subjectId` is exactly ** IN The Blacklist **
	CheckBlackList(ctx context.Context, subjectId string) (bool, error)
}

type TextBasedBlackListProvider struct {
	dataCache []string
}

func NewNullBlackListProvider() *TextBasedBlackListProvider {
	return &TextBasedBlackListProvider{
		dataCache: make([]string, 0),
	}
}

// DefaultTxtBLLoader reads blacklist entries from r.
// Blank lines and lines whose first non-space character is '#' (comments)
// are silently skipped. All other lines are trimmed and returned as records.
func DefaultTxtBLLoader(r io.Reader) ([]string, error) {
	records := make([]string, 0)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		records = append(records, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read blacklist: %w", err)
	}
	return records, nil
}

func NewTextBasedBlackListProvider(txtFilePath string) (*TextBasedBlackListProvider, error) {
	f, err := os.Open(txtFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open blacklist file: %w", err)
	}
	defer f.Close()

	records, err := DefaultTxtBLLoader(f)
	if err != nil {
		return nil, fmt.Errorf("failed to load blacklist records from file: %w", err)
	}

	return &TextBasedBlackListProvider{
		dataCache: records,
	}, nil
}

func (t *TextBasedBlackListProvider) CheckBlackList(_ context.Context, subjectId string) (bool, error) {
	subjectId = strings.TrimSpace(subjectId)
	for _, item := range t.dataCache {
		if item == subjectId {
			return true, nil
		}
	}
	return false, nil
}
