# Build outputs go to bin/, which is not tracked.
BIN     := bin
CMDS    := $(notdir $(wildcard cmd/*))
GO      ?= go
# Trim paths so a binary does not embed this checkout's absolute paths.
GOFLAGS ?= -trimpath

.PHONY: all build run test vet fmt tidy clean

all: build

## build: compile every cmd/* into bin/
build: $(addprefix $(BIN)/,$(CMDS))

$(BIN)/%: FORCE
	@mkdir -p $(BIN)
	$(GO) build $(GOFLAGS) -o $@ ./cmd/$*

## run: build and run the editor (make run FILE=notes.md)
run: $(BIN)/editor
	./$(BIN)/editor $(FILE)

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

tidy:
	$(GO) mod tidy

clean:
	rm -rf $(BIN)

FORCE:
