package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: convert <input_json_path> <output_toml_path>")
		os.Exit(1)
	}

	inputPath := os.Args[1]
	outputPath := os.Args[2]

	data, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Printf("Error reading input: %v\n", err)
		os.Exit(1)
	}

	var rawRules [][]interface{}
	if err := json.Unmarshal(data, &rawRules); err != nil {
		// Try TOML parsing if JSON fails? No, the input is clearly JSON-like array.
		fmt.Printf("Error parsing input: %v\n", err)
		os.Exit(1)
	}

	out, err := os.Create(outputPath)
	if err != nil {
		fmt.Printf("Error creating output: %v\n", err)
		os.Exit(1)
	}
	defer out.Close()

	fmt.Fprintln(out, "# Generated from Cealing-Host")
	fmt.Fprintln(out, "[alter_hostname]")
	for _, rule := range rawRules {
		if len(rule) < 2 {
			continue
		}
		domains, ok := rule[0].([]interface{})
		if !ok {
			continue
		}
		sni, _ := rule[1].(string)

		for _, d := range domains {
			domain, ok := d.(string)
			if !ok {
				continue
			}
			if strings.HasPrefix(domain, "#") {
				continue
			}
			fmt.Fprintf(out, "%q = %q\n", domain, sni)
		}
	}

	fmt.Fprintln(out, "\n[hosts]")
	for _, rule := range rawRules {
		if len(rule) < 3 {
			continue
		}
		domains, ok := rule[0].([]interface{})
		if !ok {
			continue
		}
		ip, ok := rule[2].(string)
		if !ok || ip == "" {
			continue
		}

		for _, d := range domains {
			domain, ok := d.(string)
			if !ok {
				continue
			}
			if strings.HasPrefix(domain, "#") {
				continue
			}
			fmt.Fprintf(out, "%q = %q\n", domain, ip)
		}
	}

	fmt.Fprintln(out, "\n[cert_verify]")

	fmt.Printf("Successfully converted %d rules to %s\n", len(rawRules), outputPath)
}
