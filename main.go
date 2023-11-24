package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/mod/sumdb/dirhash"
	"gopkg.in/yaml.v3"
)

func main() {
	if len(os.Args) == 1 {
		printHelpMsg()
		return
	}

	content, err := os.ReadFile("./dev.yaml")
	if err != nil {
		fmt.Printf("Can't open dev.yaml file: %v\n", err)
		os.Exit(1)
	}

	var targets map[string]target
	if err := yaml.Unmarshal(content, &targets); err != nil {
		fmt.Printf("Invalid yaml: %v\n", err)
		os.Exit(1)
	}

	target, ok := targets[os.Args[1]]
	if !ok {
		fmt.Printf("Invalid target: %s\n", os.Args[1])
		os.Exit(1)
	}

	target.name = os.Args[1]
	target.run()
}

func printHelpMsg() {
	fmt.Println("Dev build system for Panda project.")
	fmt.Println("Usage:\n\tdev <target>")
}

type target struct {
	name string

	Cmds  []string `yaml:"cmd"`
	Deps  []string `yaml:"deps"`
	Srcs  []string `yaml:"srcs"`
	Cache bool     `yaml:"cache"`
}

func (t target) run() {
	if t.isCached() {
		fmt.Println("it's cached bro")
		return
	}

	for _, c := range t.Cmds {
		prog := strings.Split(c, " ")[0]
		args := strings.Split(c, " ")[1:]

		cmd := exec.Command(prog, args...)
		output, err := cmd.Output()
		if err != nil {
			fmt.Printf("Target failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Print(string(output))
	}

	t.cache()
}

type Cache struct {
	TargetName string            `json:"name"`
	Output     string            `json:"output"`
	Hashes     map[string]string `json:"hashes"`
}

func (t target) cache() {
	if !t.Cache {
		return
	}

	cacheJSON, _ := json.Marshal(Cache{
		TargetName: t.name,
		Hashes:     t.hashSrcs(),
		Output:     "",
	})

	if err := os.Mkdir(".dev", os.ModePerm); err != nil && !errors.Is(err, os.ErrExist) {
		fmt.Printf("Can't create cache directory: %v\n", err)
		os.Exit(1)
	}

	f, err := os.Create("./.dev/" + t.name + ".json")
	if err != nil && !errors.Is(err, os.ErrExist) {
		fmt.Printf("Can't save cache for target %s: %v\n", t.name, err)
		os.Exit(1)
	}

	if _, err := f.Write(cacheJSON); err != nil {
		fmt.Printf("Can't save cache for target %s: %v\n", t.name, err)
		os.Exit(1)
	}
}

func (t target) hashSrcs() map[string]string {
	hashes := make(map[string]string)
	for _, src := range t.Srcs {
		matches, _ := filepath.Glob(src)
		for _, p := range matches {
			hashes[p] = hashFile(p)
		}
	}
	return hashes
}

func (t target) isCached() bool {
	jsonCache, err := os.ReadFile("./.dev/" + t.name + ".json")
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		fmt.Printf("Can't save cache for target %s: %v\n", t.name, err)
		os.Exit(1)
	}

	var cache Cache
	if err := json.Unmarshal(jsonCache, &cache); err != nil {
		return false
	}

	hashes := t.hashSrcs()

	if len(hashes) != len(cache.Hashes) {
		return false
	}

	for k := range hashes {
		if hashes[k] != cache.Hashes[k] {
			return false
		}
	}

	return true
}

func hashFile(p string) string {
	f, err := os.Open(p)

	if err != nil {
		fmt.Printf("Can't open src %s: %v\n", p, err)
		os.Exit(1)
	}

	fileInfo, err := f.Stat()
	if err != nil {
		fmt.Printf("Can't open src %s: %v\n", p, err)
		os.Exit(1)
	}

	if fileInfo.IsDir() {
		h, err := dirhash.HashDir(p, "", dirhash.DefaultHash)
		if err != nil {
			panic(err)
		}
		return h
	} else {
		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			panic(err)
		}
		return base64.StdEncoding.EncodeToString(h.Sum(nil))
	}
}
