// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"strings"
)

// Config holds all configuration options for the k6-connectrpc plugin.
type Config struct {
	OutputFormat      string
	ClientSuffix      string
	IncludeMocks      bool
	IncludeValidation bool
	StreamingWrappers bool
	ExternalWrappers  bool
}

// Default configuration values.
const (
	DefaultOutputFormat      = "js"
	DefaultClientSuffix      = "Client"
	DefaultIncludeMocks      = false
	DefaultIncludeValidation = true
	DefaultStreamingWrappers = true
	DefaultExternalWrappers  = true
)

// RegisterFlags registers all configuration flags and returns a Config pointer.
func RegisterFlags(flagSet *flag.FlagSet) *Config {
	cfg := &Config{}

	flagSet.StringVar(
		&cfg.OutputFormat,
		"output_format",
		DefaultOutputFormat,
		"Output format: 'js', 'ts', or 'both'",
	)

	flagSet.StringVar(
		&cfg.ClientSuffix,
		"client_suffix",
		DefaultClientSuffix,
		"Suffix for generated client class names",
	)

	flagSet.BoolVar(
		&cfg.IncludeMocks,
		"include_mocks",
		DefaultIncludeMocks,
		"Generate mock response helpers",
	)

	flagSet.BoolVar(
		&cfg.IncludeValidation,
		"include_validation",
		DefaultIncludeValidation,
		"Generate request validation helpers",
	)

	flagSet.BoolVar(
		&cfg.StreamingWrappers,
		"streaming_wrappers",
		DefaultStreamingWrappers,
		"Generate streaming wrapper classes",
	)

	flagSet.BoolVar(
		&cfg.ExternalWrappers,
		"external_wrappers",
		DefaultExternalWrappers,
		"Import wrapper classes from xk6-connectrpc instead of generating inline",
	)

	return cfg
}

// Validate checks if the configuration is valid and returns an error if not.
func (c *Config) Validate() error {
	// Validate output format
	validFormats := []string{"js", "ts", "both"}
	if !contains(validFormats, c.OutputFormat) {
		return fmt.Errorf("invalid output_format %q, must be one of: %s",
			c.OutputFormat, strings.Join(validFormats, ", "))
	}

	// Validate client suffix is a valid identifier
	if !isValidIdentifier(c.ClientSuffix) {
		return fmt.Errorf("client_suffix %q is not a valid identifier", c.ClientSuffix)
	}

	return nil
}

// ShouldGenerateJS returns true if JavaScript output should be generated.
func (c *Config) ShouldGenerateJS() bool {
	return c.OutputFormat == "js" || c.OutputFormat == "both"
}

// ShouldGenerateTS returns true if TypeScript output should be generated.
func (c *Config) ShouldGenerateTS() bool {
	return c.OutputFormat == "ts" || c.OutputFormat == "both"
}

// contains checks if a slice contains a string.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// isValidIdentifier checks if a string is a valid JavaScript/TypeScript identifier.
func isValidIdentifier(s string) bool {
	if len(s) == 0 {
		return false
	}

	// Simple validation - starts with letter or underscore,
	// followed by letters, digits, or underscores
	if !isLetter(s[0]) && s[0] != '_' {
		return false
	}

	for i := 1; i < len(s); i++ {
		if !isLetter(s[i]) && !isDigit(s[i]) && s[i] != '_' {
			return false
		}
	}

	return true
}

func isLetter(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}
