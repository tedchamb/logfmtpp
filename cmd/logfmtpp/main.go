package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/go-logfmt/logfmt"
)

func main() {
	fieldsToSkip := []string{
		"deployment.environment",
		"gh.sdk.name",
		"gh.sdk.version",
		"service.instance.id",
		"service.name",
		"telemetry.sdk.name",
	}

	scanner := bufio.NewScanner(os.Stdin)

	var data map[string]string

	for scanner.Scan() {
		data = make(map[string]string)
		input := scanner.Text()
		if strings.HasPrefix(input, "Timestamp=") {
			dec := logfmt.NewDecoder(bytes.NewBufferString(input))
			for dec.ScanRecord() {
				for dec.ScanKeyval() {
					key := string(dec.Key())
					reportField := true
					for _, toSkip := range fieldsToSkip {
						if toSkip == key {
							reportField = false
							break
						}
					}
					if reportField {
						data[string(dec.Key())] = string(dec.Value())
					}
				}
			}

			if err := dec.Err(); err != nil {
				_, err := fmt.Fprintf(os.Stderr, "Error converting logfmt to JSON: %v\n", err)
				if err != nil {
					os.Exit(1)
				}
			}

			formattedJSON, err := json.MarshalIndent(data, "", "    ")
			if err != nil {
				fmt.Println(input)
			}

			fmt.Println(string(formattedJSON))
		} else {
			fmt.Println(input)
		}
	}

	if err := scanner.Err(); err != nil {
		_, err := fmt.Fprintln(os.Stderr, "Error reading input:", err)
		if err != nil {
			return
		}
	}
}
