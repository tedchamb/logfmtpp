package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

			formattedBytes, err := json.MarshalIndent(data, "", "    ")
			if err != nil {
				fmt.Println(input)
			} else {
				if err := formatJson(bytes.NewReader(formattedBytes), os.Stdout, true); err != nil {
					_, err := fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					if err != nil {
						return
					}
				}
			}
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

const (
	colorKey   = "\033[36m"   // Regular Cyan for keys
	colorStr   = "\033[1;92m" // Bright Green for string values
	colorNum   = "\033[1;93m" // Bright Yellow for numbers
	colorBool  = "\033[1;95m" // Bright Magenta for booleans
	colorNull  = "\033[1;91m" // Bright Red for null
	colorBrace = "\033[37m"   // Regular White for braces/brackets
	colorReset = "\033[0m"    // Reset color
)

var quote = colorBrace + "\"" + colorReset
var colon = colorBrace + ":" + colorReset
var comma = colorBrace + "," + colorReset

func formatJson(reader io.Reader, writer io.Writer, expandStringEncodedJsonValues bool) error {
	err := formatJsonFromDecoder(json.NewDecoder(reader), writer, 0, 0, expandStringEncodedJsonValues)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(writer)
	if err != nil {
		return err
	}

	return nil
}

// ColorizeJSON processes JSON input and writes colorized output to the writer
func formatJsonFromDecoder(decoder *json.Decoder, writer io.Writer, indentDepth, jsonDepth int, expandStringEncodedJsonValues bool) error {
	const indent = "    "
	lastTokenKind := kindOther

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break // End of JSON input
		}
		if err != nil {
			return fmt.Errorf("error decoding JSON: %w", err)
		}

		// Process token types
		switch v := token.(type) {
		case json.Delim: // Braces or brackets
			if v == '{' || token == '[' {
				// open
				_, err = fmt.Fprintf(writer, "%s%s%s", colorBrace, v, colorReset)
				if err != nil {
					return err
				}
				err = formatJsonFromDecoder(decoder, writer, indentDepth+1, jsonDepth+1, expandStringEncodedJsonValues)
				if err != nil {
					return err
				}
				lastTokenKind = kindValue
			} else {
				// close
				if jsonDepth == 0 || indentDepth == 0 {
					return fmt.Errorf("unexpected closing delimiter: %s", v)
				}
				_, err = fmt.Fprintf(writer, "\n%s%s%s%s", strings.Repeat(indent, indentDepth-1), colorBrace, v, colorReset)
				if err != nil {
					return err
				}
				return nil
			}
		case string: // Strings
			if lastTokenKind == kindKey {
				// Value
				expandWritten := false
				if expandStringEncodedJsonValues && (strings.HasPrefix(v, "{") || strings.HasPrefix(v, "[")) {
					var buffer bytes.Buffer
					var bufferWriter io.Writer = &buffer
					err = formatJsonFromDecoder(json.NewDecoder(strings.NewReader(v)), bufferWriter, indentDepth, 0, expandStringEncodedJsonValues)
					if err == nil && len(buffer.Bytes()) > 0 {
						_, err = io.Copy(writer, &buffer)
						if err != nil {
							return err
						}
						expandWritten = true
					}
				}

				if !expandWritten {
					//escapedValue, _ := EscapeJSON(v)
					_, err = fmt.Fprintf(writer, "%s%s%s%s%s", quote, colorStr, v, colorReset, quote)
					if err != nil {
						return err
					}
				}

				lastTokenKind = kindValue
			} else {
				// Key
				if lastTokenKind == kindValue {
					_, err = fmt.Fprintf(writer, comma)
					if err != nil {
						return err
					}
				}
				_, err = fmt.Fprintf(writer, "\n%s%s%s%s%s%s%s ", strings.Repeat(indent, indentDepth), quote, colorKey, v, colorReset, quote, colon)
				if err != nil {
					return err
				}
				lastTokenKind = kindKey
			}
		case json.Number: // Numbers
			_, err = fmt.Fprintf(writer, "%s%v%s", colorNum, v, colorReset)
			if err != nil {
				return err
			}
			lastTokenKind = kindValue
		case float64: // Numbers
			_, err = fmt.Fprintf(writer, "%s%v%s", colorNum, v, colorReset)
			if err != nil {
				return err
			}
			lastTokenKind = kindValue
		case bool: // Booleans
			_, err = fmt.Fprintf(writer, "%s%t%s", colorBool, v, colorReset)
			if err != nil {
				return err
			}
			lastTokenKind = kindValue
		case nil: // Null
			_, err = fmt.Fprintf(writer, "%snull%s", colorNull, colorReset)
			if err != nil {
				return err
			}
			lastTokenKind = kindValue
		default:
			_, err = fmt.Fprintf(writer, ">>>%v<<<", v)
			if err != nil {
				return err
			}
			lastTokenKind = kindOther
		}
	}

	if jsonDepth > 0 {
		return errors.New("unexpected end of json")
	}

	return nil
}

type tokenKind int

const (
	kindOther tokenKind = iota
	kindKey
	kindValue
)

func escapeJSON(input string) (string, error) {
	// Use json.Marshal to escape special characters
	escapedBytes, err := json.Marshal(input)
	if err != nil {
		return "", err
	}

	// Marshal wraps the string in quotes; remove them
	escapedString := string(escapedBytes[1 : len(escapedBytes)-1])
	return escapedString, nil
}
