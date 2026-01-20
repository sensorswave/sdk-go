package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/sensorswave/sdk-go"
)

// Usage:
//
//	go run ./pkg/sensorswave/example \
//		--source-token=xxx \
//		--project-secret=xxx \
//		--project-id=123 \
//		--track-endpoint=http://localhost:8106/in/track \
//		--meta-endpoint=http://localhost:8107/api/ab/ff/all4eval \
//		--dynamic-key=example_dynamic_config_key \
//		--gate-key=example_gate_toggle_key \
//		--experiment-key=example_experiment_key
const (
	totalUsers              = 1000
	appVersion              = "11.0"
	defaultDynamicConfigKey = "example_dynamic_config_key"
	defaultGateKey          = "example_gate_toggle_key"
	defaultExperimentKey    = "example_experiment_key"
)

type exampleArgs struct {
	sourceToken      string
	projectSecret    string
	projectID        int64
	endpoint         string
	metaEndpoint     string
	dynamicConfigKey string
	gateKey          string
	experimentKey    string
}

func main() {
	args, err := parseArgs()
	if err != nil {
		log.Fatalf("invalid arguments: %v", err)
	}

	if err := runExample(args); err != nil {
		log.Fatalf("example failed: %v", err)
	}
}

func parseArgs() (exampleArgs, error) {
	var args exampleArgs

	flag.StringVar(&args.sourceToken, "source-token", "", "project token used by the SDK client")
	flag.StringVar(&args.projectSecret, "project-secret", "", "project secret used by the SDK client")
	flag.StringVar(&args.endpoint, "endpoint", "http://localhost:8106", "track endpoint base url")
	flag.StringVar(&args.metaEndpoint, "meta-endpoint", "http://localhost:8110", "FF meta endpoint base url")
	flag.StringVar(&args.dynamicConfigKey, "dynamic-key", defaultDynamicConfigKey, "feature flag key for dynamic config example")
	flag.StringVar(&args.gateKey, "gate-key", defaultGateKey, "feature flag key for gate example")
	flag.StringVar(&args.experimentKey, "experiment-key", defaultExperimentKey, "feature flag key for experiment example")
	flag.Parse()

	if args.sourceToken == "" {
		return args, fmt.Errorf("project-token is required")
	}
	if args.projectSecret == "" {
		return args, fmt.Errorf("project-secret is required")
	}

	return args, nil
}

// runExample configures the SDK client, generates users sharing the same
// $app_version property, and runs three FF examples: dynamic config, gate,
// and experiment.
func runExample(args exampleArgs) error {
	cfg := sensorswave.DefaultConfig(args.endpoint, args.sourceToken)

	abCfg := sensorswave.DefaultABConfig(args.sourceToken, args.projectSecret)
	abCfg.WithMetaEndpoint(args.metaEndpoint)
	abCfg.WithLoadMetaInterval(10 * time.Second)
	cfg.WithABConfig(abCfg)

	client, err := sensorswave.New(cfg)
	if err != nil {
		return fmt.Errorf("create sensorswave client: %w", err)
	}
	defer client.Close()

	users := buildUsers(totalUsers, appVersion)

	fmt.Println("== Dynamic Config Example (color config) ==")
	if err := runDynamicConfigExample(client, users, args.dynamicConfigKey); err != nil {
		return err
	}

	fmt.Println("\n== Gate Example (boolean toggle) ==")
	if err := runGateExample(client, users, args.gateKey); err != nil {
		return err
	}

	fmt.Println("\n== Experiment Example (multi-variant) ==")
	if err := runExperimentExample(client, users, args.experimentKey); err != nil {
		return err
	}

	return nil
}

func runDynamicConfigExample(client sensorswave.Client, users []sensorswave.ABUser, featureKey string) error {
	distribution := make(map[string]int, len(users))
	for _, user := range users {
		result, err := client.ABEval(user, featureKey)
		if err != nil {
			return fmt.Errorf("dynamic config eval failed for user %s: %w", user.LoginID, err)
		}

		color := result.GetString("color", "black")
		distribution[color]++
	}

	for color, count := range distribution {
		fmt.Printf("  variant(color=%s): %d users\n", color, count)
	}
	return nil
}

func runGateExample(client sensorswave.Client, users []sensorswave.ABUser, featureKey string) error {
	var pass, fail int
	for _, user := range users {
		result, err := client.ABEval(user, featureKey)
		if err != nil {
			return fmt.Errorf("gate eval failed for user %s: %w", user.LoginID, err)
		}

		if result.VariantID == nil {
			continue
		}
		if result.CheckGate() {
			pass++
		} else {
			fail++
		}
	}

	fmt.Printf("  gate %s -> pass:%d fail:%d\n", featureKey, pass, fail)
	return nil
}

func runExperimentExample(client sensorswave.Client, users []sensorswave.ABUser, featureKey string) error {
	variantCounts := make(map[string]int)
	labelCounts := make(map[string]int)
	var enabledTrue int

	for _, user := range users {
		result, err := client.ABEval(user, featureKey)
		if err != nil {
			return fmt.Errorf("experiment eval failed for user %s: %w", user.LoginID, err)
		}

		if result.VariantID == nil {
			continue
		}

		variant := *result.VariantID
		variantCounts[variant]++

		label := result.GetString("label", "control")
		labelCounts[label]++

		if result.GetBool("enabled", false) {
			enabledTrue++
		}
	}

	for variant, count := range variantCounts {
		fmt.Printf("  exp variant(%s): %d users\n", variant, count)
	}
	for label, count := range labelCounts {
		fmt.Printf("    -> payload label=%s, hits=%d\n", label, count)
	}
	fmt.Printf("  exp payload enabled=true for %d users (false for %d)\n", enabledTrue, len(users)-enabledTrue)

	return nil
}

func buildUsers(total int, appVersion string) []sensorswave.ABUser {
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	const loginIDLength = 12

	users := make([]sensorswave.ABUser, 0, total)
	for i := 0; i < total; i++ {
		users = append(users, sensorswave.ABUser{
			AnonID:  fmt.Sprintf("anon-%03d-%s", i, randomID(rnd, loginIDLength)),
			LoginID: fmt.Sprintf("user-%s", randomID(rnd, loginIDLength)),
			Props: sensorswave.Properties{
				sensorswave.PspAppVer: appVersion,
			},
		})
	}
	return users
}

func randomID(rnd *rand.Rand, length int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	buf := make([]byte, length)
	for i := range buf {
		buf[i] = letters[rnd.Intn(len(letters))]
	}
	return string(buf)
}
