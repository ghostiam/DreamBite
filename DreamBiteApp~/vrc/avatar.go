package vrc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type AvatarConfig struct {
	ID         string                    `json:"id"`
	Name       string                    `json:"name"`
	Parameters []*AvatarConfigParameters `json:"parameters"`
}

type AvatarConfigParameters struct {
	Name   string                 `json:"name"`
	Input  *AvatarConfigParameter `json:"input,omitempty"`
	Output *AvatarConfigParameter `json:"output"`
}

type AvatarConfigParameter struct {
	Address string                    `json:"address"`
	Type    AvatarConfigParameterType `json:"type"`
}

type AvatarConfigParameterType string

const (
	IntType   AvatarConfigParameterType = "Int"
	BoolType  AvatarConfigParameterType = "Bool"
	FloatType AvatarConfigParameterType = "Float"
)

func GetAvatarConfig(avatarID string) (*AvatarConfig, error) {
	// ~\AppData\LocalLow\VRChat\VRChat\OSC\usr_*\Avatars\*.json
	filePathGlob := filepath.Join(
		os.Getenv("USERPROFILE"), "AppData", "LocalLow", "VRChat", "VRChat", "OSC",
		"usr_*", "Avatars", avatarID+".json",
	)

	matches, err := filepath.Glob(filePathGlob)
	if err != nil {
		return nil, fmt.Errorf("glob: %w", err)
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no avatar config files found for avatar ID %s", avatarID)
	}

	// Sort files by creation time (newest first)
	type fileWithTime struct {
		path string
		time time.Time
	}
	filesWithTime := make([]fileWithTime, 0, len(matches))
	for _, match := range matches {
		var info os.FileInfo
		info, err = os.Stat(match)
		if err != nil {
			continue
		}
		filesWithTime = append(filesWithTime, fileWithTime{
			path: match,
			time: info.ModTime(),
		})
	}

	if len(filesWithTime) == 0 {
		return nil, fmt.Errorf("no accessible avatar config files found for avatar ID %s", avatarID)
	}

	// Sort by modification time descending (newest first)
	sort.Slice(filesWithTime, func(i, j int) bool {
		return filesWithTime[i].time.After(filesWithTime[j].time)
	})

	avatarConfigFile := filesWithTime[0].path

	file, err := os.Open(avatarConfigFile)
	if err != nil {
		return nil, fmt.Errorf("open avatar config file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	r, err := FixJSONReader(file)
	if err != nil {
		return nil, fmt.Errorf("fix JSON reader: %w", err)
	}

	var cfg AvatarConfig
	err = json.NewDecoder(r).Decode(&cfg)
	if err != nil {
		return nil, fmt.Errorf("decode avatar config file: %w", err)
	}

	return &cfg, nil
}

func FixJSONReader(f io.Reader) (*bufio.Reader, error) {
	r := bufio.NewReader(f)

	// Skip until '{'.
	_, err := r.ReadBytes('{')
	if err != nil {
		return nil, fmt.Errorf("read until '{': %w", err)
	}
	// Move back.
	err = r.UnreadByte()
	if err != nil {
		return nil, fmt.Errorf("unread byte: %w", err)
	}

	return r, nil
}
