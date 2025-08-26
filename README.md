# Daphne
Daphne is a simulation of the sonic blockchain network and its main components, 
greatly simplified in comparison to the `sonic` repository. The simulation is local,
with different computers being simulated within the same process.

The goal of project Daphne is to establish an evaluation framework for candidate 
consensus algorithms for the Sonic networks. Additionally, the repository is 
intended to provide a reference for the overall operation of the Sonic network 
free of the sometimes convoluted code base of production level implementations.
This can enable swift prototyping of various solutions for the network, free of the
complexities of the main repository.

Message passing between simulated nodes is done synchronously as of yet, while asynchronous messaging is currently WIP.

The simulation is not strictly deterministic as it is multi-threaded.

# Useful commands

## Build Test 
To run all tests, run:
`go test ./...`

### Optional test flags
- `-count 1` asks to run all tests once, disregarding cached tests
- `-v` sets the test run to verbose, it will output how long each tests takes to run
- `-run ^TestMyTest$` will run only the tests fitting the regex
- `-cpuprof cpu.prof` will generate a cpu profile that can be reviewed with pprof
- `-memprof mem.prof` will generate a memory profile that can be reviewed with pprof

To open those profiles run:
`go tool pprof -http "localhost:8000" ./cpu.prof`

## Generating mocks
When testing, we frequently need to mock certain interfaces. For this we use `gomock`. 
It is installed by running
`go install go.uber.org/mock/mockgen@latest`

(Re)generating a mock is done via a command specific for that interface, given in the `.go` file that contains it. The commands are of the following form:
`//go:generate mockgen -source <source file> -destination=<mock file> -package=<package>`

Regenerating mocks should be done when there is a change to an interface being mocked.
## Lint
We use [golangci-lint](https://golangci-lint.run/) for static linters, to run it use
`golangci-lint run ./...`

To install it `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.1.6`