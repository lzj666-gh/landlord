package common

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

var (
	Pokers       = make(map[string]*Combination, 16384)
	TypeToPokers = make(map[string][]*Combination, 38)
)

type Combination struct {
	Type  string
	Score int
	Poker string
}

func init() {
	workingDir, err := os.Getwd()
	if err != nil {
		panic(fmt.Errorf("find poker rules: get working directory: %v", err))
	}
	path, err := findRulePath(workingDir)
	if err != nil {
		panic(fmt.Errorf("find poker rules: %v", err))
	}
	jsonStrByte, err := ioutil.ReadFile(path)
	if err != nil {
		panic(fmt.Errorf("read poker rules %q: %v", path, err))
	}
	var rule = make(map[string][]string)
	err = json.Unmarshal(jsonStrByte, &rule)
	if err != nil {
		panic(fmt.Errorf("decode poker rules %q: %v", path, err))
	}
	for pokerType, pokers := range rule {
		for score, poker := range pokers {
			cards := SortStr(poker)
			p := &Combination{
				Type:  pokerType,
				Score: score,
				Poker: cards,
			}
			Pokers[cards] = p
			TypeToPokers[pokerType] = append(TypeToPokers[pokerType], p)
		}
	}
}

func findRulePath(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			path := filepath.Join(dir, "rule.json")
			if _, err := os.Stat(path); err != nil {
				return "", fmt.Errorf("project rule.json not found at %q: %v", path, err)
			}
			return path, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("project root containing go.mod not found from %q", startDir)
}
