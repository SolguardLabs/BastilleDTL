package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/solguardlabs/bastilledtl/src/scenario"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "run":
		run(os.Args[2:])
	case "default":
		writeDefault()
	default:
		usage()
		os.Exit(2)
	}
}

func run(args []string) {
	flags := flag.NewFlagSet("run", flag.ExitOnError)
	if err := flags.Parse(args); err != nil {
		log.Fatal(err)
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "run requires a fixture path")
		os.Exit(2)
	}
	definition, err := scenario.LoadFile(flags.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	result, err := scenario.Run(definition)
	if err != nil {
		log.Fatal(err)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		log.Fatal(err)
	}
}

func writeDefault() {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(scenario.DefaultBootstrap()); err != nil {
		log.Fatal(err)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  bastilledtl run <scenario.json>")
	fmt.Fprintln(os.Stderr, "  bastilledtl default")
}
