# Daphne
Daphne is a simulation of the Sonic blockchain network and its main components, 
greatly simplified in comparison to the `sonic` repository. The simulation is local,
with different nodes being simulated within the same process.

The goal of project Daphne is to establish an evaluation framework for candidate 
consensus algorithms for the Sonic networks. Additionally, the repository is 
intended to provide a reference for the overall operation of the Sonic network 
free of the sometimes convoluted code base of production level implementations.
The aim is to enable swift prototyping of various solutions for the network, free of the
complexities of the real-world, production-ready code.

For the effective evaluation of long-running or resource0heavy scenarios, the Daphne framework can be run within a discrete event simulation. It leverages the Go programming language's `synctest` facility to run concurrent Go code natively within a discrete event simulation environment.

The simulation is not strictly deterministic as it is multi-threaded.


## Currently available Consensus Implementations

- Chain-based BFT: Implementations of standard synchronous and partially synchronous protocols, including `Streamlet`, `Tendermint`, and `HotStuff 2`.

- Universal DAG Consensus (`UniDAG`): Daphne utilizes a specialized sub-architecture for Directed Acyclic Graph (DAG) protocols. `UniDAG` explicitly decouples the graph's structural maintenance and network propagation from the specific consensus ordering algorithm. `Lachesis`, `Atropos` and `Mysticeti` protocols are currently implemented via this framework and available for simulation.

## Using Daphne

The main utility provided by Daphne is its chain simulation environment and its
associated analysis. Currently, Daphne offers two modes for running simulations:
- `eval` ... running a single scenarios
- `study` ... running a range of scenario systematically and repeatedly

The `eval` mode is intended for the in-depth evaluation of specific aspects of
a scenario. In particular, it is utilized for investigating identified issues or
for debugging protocol issues.

The `study` mode is intended for collecting data for parameter studies, enabling
the derivation of empirical data for scalability analysis and side-by-side
comparison of different protocols.

### Running an Evaluation

To run an evaluation, use the following command:
```bash
go run ./daphne eval <desired flags>
```
A few example options offered by the evaluation tool are
- `--sim-time` or `-s` to enable simulation time instead of real time (DES mode)
- `--num-nodes` or `-n` to determine the number of nodes on the network to be evaluated
- `--duration` or `-d` to set the time span to be evaluated
- `--tx-per-seconds` or `-t` to set the network load to be simulated

For more parameters and options see the commands help page using
```bash
go run ./daphne eval help
```

#### Analyzing Results
The evaluation command produces an event file (by default `output.parquet`). This
file can be loaded into one of the evaluation analysis Jupyter notebooks 
provided in the [analysis](./analysis/eval) directory for
further investigation.

### Running a Study
To run a study, use the following command:
```bash
go run ./daphne study <study-type> <desired flags>
```
Among the available studies are
- `load` ... runs a range of configurations varying the network size and
the number of transactions per second
- `broadcast` ... runs a range of configurations varying the network size and 
utilized broadcasting protocols
- `consensus` ... runs a range of configurations varying the network size and 
utilized consensus protocols

See
```bash
go run ./daphne study help
```
for more study types.

Besides the study types, a range of flags to customize the study execution are
offered:
- `--sim-time` or `-s` to enable simulation time instead of real time
- `--repetitions` or `-r` to determine the number of repetitions for each configuration
- `--duration` or `-d` to set the time span to be evaluated for each configuration

For more parameters and options see the commands help page using
```bash
go run ./daphne study help
```

#### Analyzing Results
The evaluation command produces an event file (by default `data.parquet`). This
file can be loaded into dedicated Jupyter notebooks -- at least one for each
study type -- provided in the [analysis](./analysis/study) directory for further investigation.


# Useful commands

## Build
To build the project, run:

`go build ./...`.

If the build is successful, nothing will be output by the command.

## Run Tests
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

`mockgen -source <source file> -destination=<mock file> -package=<package>`,

however, their usage is facilitated by `//go:generate` comments in source files, enabling mocks to be generated via

`go generate <path to file>` for generating a particular mock or `go generate ./...` for generating all mocks.

Regenerating mocks should be done when there is a change to an interface being mocked.
## Lint
We use [golangci-lint](https://golangci-lint.run/) for static linters, to run it use

`golangci-lint run ./...`.

To install it run 

`go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.1.6`.

# Known issues
In this section will be laid out known issues or bugs in the project. Currently, there are no known unaddressed issues.