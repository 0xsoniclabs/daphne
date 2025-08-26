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

The simulation is not strictly deterministic as it is multi-threaded.

# Useful commands

## Build
To build the project, run:

`go build ./...`.

If the build is successful, nothing will be output by the command.

## Build Test 
To run all tests, run:

`go test ./...`

All tests are expected to pass.

### Optional test flags
- `-count 1` asks to run all tests once, disregarding cached tests
- `-v` sets the test run to verbose, it will output how long each tests takes to run
- `-race` runs the program with a race detector on, meaning it will detect race conditions if they exist
- `-cover` shows coverage for each package tested
- `-run ^TestMyTest$` will run only the tests fitting the regex
- `-cpuprof cpu.prof` will generate a cpu profile that can be reviewed with pprof
- `-memprof mem.prof` will generate a memory profile that can be reviewed with pprof

To open those profiles run:

`go tool pprof -http "localhost:8000" ./cpu.prof`

## Generating mocks
When testing, we frequently need to mock certain interfaces. For this we use `gomock`. 
It is installed by running

`go install go.uber.org/mock/mockgen@latest`.

(Re)generating a mock is done via a command specific for that interface, given in the `.go` file that contains it. The commands are of the following form:

`//go:generate mockgen -source <source file> -destination=<mock file> -package=<package>`.

Regenerating mocks should be done when there is a change to an interface being mocked.
## Lint
We use [golangci-lint](https://golangci-lint.run/) for static linters, to run it use

`golangci-lint run ./...`.

To install it run 

`go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.1.6`.

# Future work

## The `main` function
Notably, the project currently lacks a `main` function, meaning the only way to interact with its code is via testing. As future work, a proper entry point will be developed, with the facility to specify a network scenario that is to be simulated, along with its parameters.

## Various consensus protocols
Currently, the project lacks implementations of important consensus protocols we are interested in comparing, such as Lachesis, Tendermint etc. This, among other things, constitutes the gist of the ongoing efforts on the project.

## Asynchronous messaging
Message passing between simulated nodes is done synchronously as of yet, while asynchronous messaging is currently WIP.

# Known issues
In this section will be laid out known issues or bugs in the project. Currently, there are no known unaddressed issues.