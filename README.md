# Daphne
The goal of project Daphne is to establish an evaluation framework for candidate 
consensus algorithms for the Sonic networks. Additionally, the repository is 
intended to provide a reference for the overall operation of the Sonic network 
free of the sometimes convoluted code base of production level implementations.

# Useful commands

## Build Test 
`go test ./... -count 1 -v`

where:
- `-count 1` asks to run all tests once, disregarding cached tests
- `-v` sets the test run to verbose, it will output how long each tests takes to run

### Optional test flags
- `-run ^TestMyTest$` will run only the tests fitting the regex
- `-cpuprof cpu.prof` will generate a cpu profile that can be reviewed with pprof
- `-memprof mem.prof` will generate a memory profile that can be reviewed with pprof

To open those profiles
`go tool pprof -http "localhost:8000" ./cpu.prof`

## Lint
We use [golangci-lint](https://golangci-lint.run/) for static linters, to run it use
`golangci-lint run ./...`

To install it `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.1.6`