package main

import (
	"bufio"
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"regexp"

	"golang.org/x/sys/unix"
)

//go:embed ecodes.go.template
var tmplFS embed.FS

// default headers
var defaultSources = []string{
	"/usr/include/linux/input.h",
	"/usr/include/linux/input-event-codes.h",
}

// KEY_A 30
var macroRegex = regexp.MustCompile(`^#define +((?:KEY|ABS|REL|SW|MSC|LED|BTN|REP|SND|ID|EV|BUS|SYN|FF)_\w+)\s+(\w+)`)

type Code struct {
	Name  string
	Value string
}

type Context struct {
	Uname string
	Codes []Code
}

func main() {
	sources := defaultSources
	if len(os.Args) > 1 {
		sources = os.Args[1:]
	}

	if err := run(sources); err != nil {
		log.Fatal(err.Error())
	}
}

func run(sources []string) error {
	codes, err := getCodes(sources)
	if err != nil {
		return fmt.Errorf("get codes: %w", err)
	}

	ctx := Context{
		Uname: getUname(),
		Codes: codes,
	}

	tpl, err := loadTemplate()
	if err != nil {
		return fmt.Errorf("load template: %w", err)
	}

	var buf bytes.Buffer

	if err := tpl.Execute(&buf, ctx); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("format go source: %w", err)
	}

	if _, err := os.Stdout.Write(formatted); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	return nil
}

func getCodes(headers []string) ([]Code, error) {
	var result []Code

	for _, h := range headers {
		abs, err := filepath.Abs(h)
		if err != nil {
			return nil, fmt.Errorf("get absolute path: %w", err)
		}

		codes, err := readHeader(abs)
		if err != nil {
			continue
		}

		result = append(result, codes...)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no input macros found in: %v", headers)
	}

	return result, nil
}

func getUname() string {
	hn, _ := os.Hostname()
	return fmt.Sprintf("%s %s", hn, runtimeGOOSArch())
}

func runtimeGOOSArch() string {
	var u unix.Utsname

	if err := unix.Uname(&u); err != nil {
		return "unknown unknown"
	}

	sysname := charsToString(u.Sysname[:])
	release := charsToString(u.Release[:])
	machine := charsToString(u.Machine[:])

	return fmt.Sprintf("%s %s (%s)", sysname, release, machine)
}

func charsToString(ca []byte) string {
	for i := range len(ca) {
		if ca[i] == 0 {
			return string(ca[:i])
		}
	}

	return string(ca)
}

func loadTemplate() (*template.Template, error) {
	data, err := tmplFS.ReadFile("ecodes.go.template")
	if err != nil {
		return nil, err
	}

	return template.New("ecodes").Parse(string(data))
}

func readHeader(path string) ([]Code, error) {
	// #nosec G304 G703 -- reading trusted system headers only
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer func() { _ = file.Close() }()

	var result []Code

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()

		m := macroRegex.FindStringSubmatch(line)
		if m != nil {
			result = append(result, Code{
				Name:  m[1],
				Value: m[2],
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan file %s: %w", path, err)
	}

	return result, nil
}
