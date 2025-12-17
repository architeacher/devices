GO := go

## Set an output prefix, which is the local directory if not specified.
ARTIFACTS_DIR ?= ${CURDIR}/.artifacts

## Go generated files.
GO_GENERATED_DIR = ${ARTIFACTS_DIR}/go

## The number of parallel tests.
PARALLEL_TESTS ?= 8

## Test timeout.
TEST_TIMEOUT ?= 8s

## Set profiler path.
PROFILER_PATH ?= $(GO_GENERATED_DIR)/profiler

## Package to benchmark (default to internal packages).
BENCH_PKG ?= ./internal/runtime

## Bench memory profile path.
BENCH_CPU_PROFILE ?= $(PROFILER_PATH)/mem.out
## Bench cpu profile path.
BENCH_MEMORY_PROFILE ?= $(PROFILER_PATH)/cpu.out
## Bench binary profiler path.
BENCH_PROFILE ?= $(PROFILER_PATH)/bench.bin

## The times that each test profile and benchmark would run.
BENCH_PROFILE_COUNT ?= 1

## Bench tests timeout.
BENCH_PROFILER_TIMEOUT ?= 10m

# Goroutine blocking profile path.
BLOCK_TRACE_PROFILE ?= $(PROFILER_PATH)/blocking.out
# Mutex contention profile path.
MUTEX_TRACE_PROFILE ?= $(PROFILER_PATH)/mutex.out
# Execution trace profile path.
EXEC_TRACE_PROFILE ?= $(PROFILER_PATH)/trace.out

.PHONY: profile
profile: dump-assembly profile-pdf profile-png profile-svg ## to get benchmark profiles in pdf, png and svg formats.

$(PROFILER_PATH):
	if [ ! -d "${PROFILER_PATH}" ] ; then mkdir -p $@ 2>&1 ; fi

$(BENCH_PROFILE): $(PROFILER_PATH)
	$(call printMessage,"Benchmarking with blocking cpu memory and mutex profiles",$(INFO_CLR))
	CGO_ENABLED=1 \
	$(GO) test \
		-bench=. \
		-benchmem \
		-blockprofile "${BLOCK_TRACE_PROFILE}" \
		-count="${BENCH_PROFILE_COUNT}" \
		-cpu=1,4,8 \
		-cpuprofile "${BENCH_CPU_PROFILE}" \
    	-memprofile "${BENCH_MEMORY_PROFILE}" \
		-mutexprofile "${MUTEX_TRACE_PROFILE}" \
		-o $@	\
		-parallel "${PARALLEL_TESTS}" \
		-race \
		-run=NONE \
		-tags=bench,trace \
		-timeout "${TEST_TIMEOUT}" \
		-trace "${EXEC_TRACE_PROFILE}" \
		$(GO_FLAGS) \
		"$(BENCH_PKG)" 2>&1

.PHONY: bench
bench: $(BENCH_PROFILE) ## to run benchmark tests.

.PHONY: clean-profiling
clean-profiling: ## to clean generated profiling files.
	$(call printMessage,"🧹 Cleaning up generated profiling files",$(WARN_CLR))
	rm -rf "${PROFILER_PATH}" 2>&1

.PHONY: cpu-profile-serve
cpu-profile-serve: $(BENCH_PROFILE) ## to serve cpu benchmark profile over http - useful only if building remote/headless.
	$(call printMessage,"Serving cpu profile on port 8081",$(INFO_CLR))
	$(GO) tool pprof -http=":8081" "${BENCH_PROFILE}" "${BENCH_CPU_PROFILE}" 2>&1

.PHONY: dump-assembly
dump-assembly: bench ## to generate compiler assembly.
	$(call printMessage,"Generating compiler assembly",$(INFO_CLR))
	$(GO) tool objdump "${BENCH_PROFILE}" > "${PROFILER_PATH}/profile.asm" 2>&1

.PHONY: mem-profile-serve
mem-profile-serve: bench ## to serve memory benchmark profile over http - useful only if building remote/headless.
	$(call printMessage,"Serving memory profile on port 8082",$(INFO_CLR))
	$(GO) tool pprof -http=":8082" "${BENCH_PROFILE}" "${BENCH_MEMORY_PROFILE}" 2>&1

.PHONY: profile-%
profile-%: bench ## to get benchmark profiles in specified format (pdf, png, svg).
	$(eval FORMAT=$(firstword $(subst -, , $*)))
	$(call printMessage,"Generating bench profile in ${FORMAT} format",$(INFO_CLR))
	$(GO) tool pprof -$(FORMAT) "${BENCH_PROFILE}" "${BENCH_CPU_PROFILE}" > "${PROFILER_PATH}/cpu.$(FORMAT)" 2>&1
	$(GO) tool pprof -$(FORMAT) "${BENCH_PROFILE}" "${BENCH_MEMORY_PROFILE}" > "${PROFILER_PATH}/mem.$(FORMAT)" 2>&1

.PHONY: trace-serve
trace-serve: bench ## to serve runtime execution tracer.
	$(call printMessage,"Serving runtime execution tracer",$(INFO_CLR))
	$(GO) tool trace "${EXEC_TRACE_PROFILE}" 2>&1
