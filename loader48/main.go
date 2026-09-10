package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/bytecodealliance/wasmtime-go/v48"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: loader <modules-dir> <hold-duration>")
		os.Exit(2)
	}

	hold, err := time.ParseDuration(os.Args[2])
	if err != nil {
		panic(err)
	}

	paths, err := filepath.Glob(filepath.Join(os.Args[1], "*.wasm"))
	if err != nil || len(paths) == 0 {
		fmt.Fprintf(os.Stderr, "no modules in %s: %v\n", os.Args[1], err)
		os.Exit(1)
	}
	sort.Strings(paths)

	engines := make([]*wasmtime.Engine, 0, len(paths))
	modules := make([]*wasmtime.Module, 0, len(paths))

	start := time.Now()
	for _, path := range paths {
		wasm, err := os.ReadFile(path)
		if err != nil {
			panic(err)
		}

		cfg := wasmtime.NewConfig()
		cfg.SetEpochInterruption(true)
		cfg.SetConsumeFuel(true)
		if err := cfg.CacheConfigLoadDefault(); err != nil {
			fmt.Fprintf(os.Stderr, "cache config: %v\n", err)
		}
		cfg.SetCraneliftOptLevel(wasmtime.OptLevelSpeedAndSize)
		cfg.SetNativeUnwindInfo(false)

		engine := wasmtime.NewEngineWithConfig(cfg)
		module, err := wasmtime.NewModule(engine, wasm)
		if err != nil {
			panic(err)
		}

		engines = append(engines, engine)
		modules = append(modules, module)
	}

	fmt.Printf("LOADED %d modules in %.1fs\n", len(modules), time.Since(start).Seconds())
	time.Sleep(hold)
	fmt.Printf("DONE %d\n", len(engines))
}
