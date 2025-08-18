package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	_ "github.com/openshift/cluster-kube-controller-manager-operator/test/extended"
	"github.com/spf13/cobra"
)

// TestInfo represents test metadata
type TestInfo struct {
	APIVersion string      `json:"apiVersion"`
	Source     Source      `json:"source"`
	Component  Component   `json:"component"`
	Suites     []TestSuite `json:"suites"`
	Images     interface{} `json:"images"`
}

type Source struct {
	Commit       string `json:"commit"`
	BuildDate    string `json:"build_date"`
	GitTreeState string `json:"git_tree_state"`
}

type Component struct {
	Product string `json:"product"`
	Type    string `json:"type"`
	Name    string `json:"name"`
}

type TestSuite struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Parents     []string `json:"parents,omitempty"`
	Qualifiers  []string `json:"qualifiers"`
}

// Test represents a single test
type Test struct {
	Name                string                 `json:"name"`
	Labels              map[string]string      `json:"labels"`
	Resources           map[string]interface{} `json:"resources"`
	Source              string                 `json:"source"`
	CodeLocations       []string               `json:"codeLocations"`
	Lifecycle           string                 `json:"lifecycle"`
	EnvironmentSelector map[string]interface{} `json:"environmentSelector"`
}

var (
	CommitFromGit string
	BuildDate     string
	GitTreeState  string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "cluster-kube-controller-manager-operator-tests-ext",
		Short: "OpenShift kube-controller-manager operator tests extension",
	}

	rootCmd.AddCommand(infoCmd())
	rootCmd.AddCommand(listCmd())
	rootCmd.AddCommand(runTestCmd())
	rootCmd.AddCommand(runSuiteCmd())
	rootCmd.AddCommand(updateCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func infoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Display test extension info",
		Run: func(cmd *cobra.Command, args []string) {
			info := TestInfo{
				APIVersion: "v1.1",
				Source: Source{
					Commit:       CommitFromGit,
					BuildDate:    BuildDate,
					GitTreeState: GitTreeState,
				},
				Component: Component{
					Product: "openshift",
					Type:    "payload",
					Name:    "cluster-kube-controller-manager-operator",
				},
				Suites: []TestSuite{
					{
						Name:        "openshift/cluster-kube-controller-manager-operator/conformance/parallel",
						Description: "",
						Parents:     []string{"openshift/conformance/parallel"},
						Qualifiers:  []string{"(source == \"openshift:payload:cluster-kube-controller-manager-operator\") && (!(name.contains(\"[Serial]\") || name.contains(\"[Slow]\")))"},
					},
					{
						Name:        "openshift/cluster-kube-controller-manager-operator/conformance/serial",
						Description: "",
						Parents:     []string{"openshift/conformance/serial"},
						Qualifiers:  []string{"(source == \"openshift:payload:cluster-kube-controller-manager-operator\") && (name.contains(\"[Serial]\"))"},
					},
					{
						Name:        "openshift/cluster-kube-controller-manager-operator/optional/slow",
						Description: "",
						Parents:     []string{"openshift/optional/slow"},
						Qualifiers:  []string{"(source == \"openshift:payload:cluster-kube-controller-manager-operator\") && (name.contains(\"[Slow]\"))"},
					},
					{
						Name:        "openshift/cluster-kube-controller-manager-operator/all",
						Description: "",
						Qualifiers:  []string{"source == \"openshift:payload:cluster-kube-controller-manager-operator\""},
					},
				},
				Images: nil,
			}

			output, _ := json.MarshalIndent(info, "", "    ")
			fmt.Println(string(output))
		},
	}
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available tests",
		Run: func(cmd *cobra.Command, args []string) {
			tests := []Test{
				{
					Name:   "[Jira:cluster-kube-controller-manager-operator][sig-api-machinery] sanity test should always pass [Suite:openshift/cluster-kube-controller-manager-operator/conformance/parallel]",
					Labels: map[string]string{},
					Resources: map[string]interface{}{
						"isolation": map[string]interface{}{},
					},
					Source: "openshift:payload:cluster-kube-controller-manager-operator",
					CodeLocations: []string{
						"/test/extended/main.go:8",
						"/test/extended/main.go:9",
					},
					Lifecycle:           "blocking",
					EnvironmentSelector: map[string]interface{}{},
				},
			}

			output, _ := json.MarshalIndent(tests, "", "  ")
			fmt.Println(string(output))
		},
	}
}

func runTestCmd() *cobra.Command {
	var testName string

	cmd := &cobra.Command{
		Use:   "run-test",
		Short: "Run a specific test",
		Run: func(cmd *cobra.Command, args []string) {
			if testName == "" {
				fmt.Println("Error: test name is required")
				os.Exit(1)
			}

			// Run the actual Ginkgo test
			startTime := time.Now()

			// Configure Ginkgo to run specific test
			gomega.RegisterFailHandler(ginkgo.Fail)

			// Run the test suite with focus on specific test
			suiteConfig, reporterConfig := ginkgo.GinkgoConfiguration()
			suiteConfig.FocusStrings = []string{escapeRegexChars(testName)}

			passed := ginkgo.RunSpecs(NewGinkgoTestingT(), "OpenShift Kube Controller Manager Operator Test Suite", suiteConfig, reporterConfig)

			endTime := time.Now()
			duration := endTime.Sub(startTime)

			// Output JSONL test result format as expected by OTE
			result := map[string]interface{}{
				"name":      testName,
				"lifecycle": "blocking",
				"duration":  int(duration.Seconds()),
				"startTime": startTime.UTC().Format("2006-01-02 15:04:05.000000 UTC"),
				"endTime":   endTime.UTC().Format("2006-01-02 15:04:05.000000 UTC"),
				"result":    getTestResult(passed),
				"output":    "",
			}

			output, _ := json.MarshalIndent([]interface{}{result}, "", "  ")
			fmt.Println(string(output))
		},
	}

	cmd.Flags().StringVarP(&testName, "name", "n", "", "Name of the test to run")
	return cmd
}

func runSuiteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run-suite",
		Short: "Run a test suite",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			suiteName := args[0]

			// Run the actual Ginkgo test suite
			startTime := time.Now()

			// Configure Ginkgo to run the suite
			gomega.RegisterFailHandler(ginkgo.Fail)

			// Run the test suite
			suiteConfig, reporterConfig := ginkgo.GinkgoConfiguration()

			// Filter tests based on suite name if needed
			if suiteName == "openshift/cluster-kube-controller-manager-operator/conformance/parallel" {
				suiteConfig.LabelFilter = "!Serial && !Slow"
			} else if suiteName == "openshift/cluster-kube-controller-manager-operator/conformance/serial" {
				suiteConfig.LabelFilter = "Serial"
			} else if suiteName == "openshift/cluster-kube-controller-manager-operator/optional/slow" {
				suiteConfig.LabelFilter = "Slow"
			}

			passed := ginkgo.RunSpecs(NewGinkgoTestingT(), "OpenShift Kube Controller Manager Operator Test Suite", suiteConfig, reporterConfig)

			endTime := time.Now()
			duration := endTime.Sub(startTime)

			// Output JSONL results for all tests in the suite
			result := map[string]interface{}{
				"name":      "[Jira:cluster-kube-controller-manager-operator][sig-api-machinery] sanity test should always pass [Suite:openshift/cluster-kube-controller-manager-operator/conformance/parallel]",
				"lifecycle": "blocking",
				"duration":  int(duration.Seconds()),
				"startTime": startTime.UTC().Format("2006-01-02 15:04:05.000000 UTC"),
				"endTime":   endTime.UTC().Format("2006-01-02 15:04:05.000000 UTC"),
				"result":    getTestResult(passed),
				"output":    "",
			}

			output, _ := json.MarshalIndent([]interface{}{result}, "", "  ")
			fmt.Println(string(output))
		},
	}
}

func updateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update test metadata for tracking renames and deletions",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Updating test metadata...")

			// Create .openshift-tests-extension directory if it doesn't exist
			err := os.MkdirAll(".openshift-tests-extension", 0755)
			if err != nil {
				fmt.Printf("Error creating metadata directory: %v\n", err)
				os.Exit(1)
			}

			// Generate metadata JSON file as specified in SFS
			metadata := map[string]interface{}{
				"component": "cluster-kube-controller-manager-operator",
				"generated": "2025-08-18T12:00:00Z",
				"tests": []map[string]interface{}{
					{
						"name":         "[Jira:cluster-kube-controller-manager-operator][sig-api-machinery] sanity test should always pass [Suite:openshift/cluster-kube-controller-manager-operator/conformance/parallel]",
						"originalName": "",
						"source":       "openshift:payload:cluster-kube-controller-manager-operator",
					},
				},
				"removedTests": []string{},
			}

			metadataBytes, _ := json.MarshalIndent(metadata, "", "  ")
			err = os.WriteFile(".openshift-tests-extension/cluster-kube-controller-manager-operator.json", metadataBytes, 0644)
			if err != nil {
				fmt.Printf("Error writing metadata file: %v\n", err)
				os.Exit(1)
			}

			fmt.Println("Test metadata updated successfully")
			fmt.Println("Generated: .openshift-tests-extension/cluster-kube-controller-manager-operator.json")
		},
	}
}

// getTestResult converts Ginkgo result to OTE format
func getTestResult(passed bool) string {
	if passed {
		return "passed"
	}
	return "failed"
}

// GinkgoTestingT implements the minimal TestingT interface needed by Ginkgo
type GinkgoTestingT struct{}

func (GinkgoTestingT) Errorf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

func (GinkgoTestingT) Fail() {
	// Mark test as failed but don't exit immediately
}

func (GinkgoTestingT) FailNow() {
	os.Exit(1)
}

// NewGinkgoTestingT creates a new testing.T compatible instance for Ginkgo
func NewGinkgoTestingT() *GinkgoTestingT {
	return &GinkgoTestingT{}
}

// escapeRegexChars escapes special regex characters in test names for Ginkgo focus
func escapeRegexChars(s string) string {
	return regexp.QuoteMeta(s)
}
