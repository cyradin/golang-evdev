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
		file, err := os.Open(h)
		if err != nil {
			continue
		}
		defer file.Close()

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
	s := make([]byte, len(ca))
	var i int
	for i = 0; i < len(ca); i++ {
		if ca[i] == 0 {
			break
		}
		s[i] = byte(ca[i])
	}
	return string(s[:i])
}

func loadTemplate() (*template.Template, error) {
	data, err := tmplFS.ReadFile("ecodes.go.template")
	if err != nil {
		return nil, err
	}

	return template.New("ecodes").Parse(string(data))
}
