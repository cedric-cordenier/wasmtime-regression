# wasmtime-go v48 uses more memory than v47 to compile modules

Loading n wasm modules with `wasmtime.NewModule`, one `Engine` per module, costs
substantially more container memory under wasmtime-go v48.0.0 than v47.0.0, for
the same input and the same engine configuration.

## Layout

    main.go     driver: builds the guest, writes n unique copies, runs both containers
    guest/      the wasm module source, built for GOOS=wasip1
    loader47/   loads every module with wasmtime-go v47, then holds them
    loader48/   same, against v48

Each loader is a separate Go module because wasmtime-go carries its major
version in the import path. The driver mounts one loader directory and the
module directory into a container, waits for the loader to report `LOADED`, and
then reads `docker stats`.

## Running

    go run .
    go run . -n 1
    go run . -n 20 -threads 10

## Notes

Both loaders mirror the engine configuration used in production: epoch
interruption, fuel consumption, the default on-disk compilation cache, Cranelift
`OptLevelSpeedAndSize`, and native unwind info off.
