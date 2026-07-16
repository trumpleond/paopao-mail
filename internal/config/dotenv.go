package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// LoadDotEnv loads KEY=VALUE pairs from a .env file into the process environment.
// Existing environment variables are not overridden.
// Missing file is not an error.
//
// Search order (first existing wins):
//  1. path if non-empty
//  2. ./.env
//  3. directory of the executable / .env
func LoadDotEnv(path string) (loaded string, err error) {
	candidates := []string{}
	if strings.TrimSpace(path) != "" {
		candidates = append(candidates, path)
	}
	candidates = append(candidates, ".env")
	if exe, e := os.Executable(); e == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), ".env"))
	}
	if wd, e := os.Getwd(); e == nil {
		candidates = append(candidates, filepath.Join(wd, ".env"))
	}

	var file string
	for _, c := range candidates {
		if st, e := os.Stat(c); e == nil && !st.IsDir() {
			file = c
			break
		}
	}
	if file == "" {
		return "", nil
	}

	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	// support long lines
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// optional "export KEY=VAL"
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(line[len("export "):])
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if key == "" {
			continue
		}
		// strip surrounding quotes
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		// do not override real environment
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, val)
	}
	if err := sc.Err(); err != nil {
		return file, err
	}
	return file, nil
}
