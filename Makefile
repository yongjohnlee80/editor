# Build outputs go to bin/, which is not tracked.
BIN  := bin
CMDS := $(notdir $(wildcard cmd/*))
GO   ?= go
PKG  := github.com/yongjohnlee80/editor

# VERSION is the only thing that needs stamping. The commit, its time and
# whether the tree was dirty come from -buildvcs (on by default) and are read
# back with debug.ReadBuildInfo, so `go build` and `go install` report them
# correctly too — not just builds that went through this Makefile.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X '$(PKG).Version=$(VERSION)'
# -trimpath so a binary embeds no absolute paths from this checkout.
GOFLAGS ?= -trimpath

.PHONY: all build run sample edit-sample test race vet fmt tidy version clean

all: build

## build: compile every cmd/* into bin/
build: $(addprefix $(BIN)/,$(CMDS))

$(BIN)/%: FORCE
	@mkdir -p $(BIN)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ ./cmd/$*

## run: build and open a file (make run FILE=notes.md)
run: $(BIN)/editor
	./$(BIN)/editor $(FILE)

## sample: copy the sample ADR into bin/ as a scratch file to edit
# bin/ is gitignored, so the copy is a build artifact: edit it, save it, delete
# it, and the tracked original under testdata/ is untouched.
sample: | $(BIN)
	cp testdata/sample-adr.md $(BIN)/sample-adr.md
	@echo "wrote $(BIN)/sample-adr.md"

## edit-sample: build, refresh the scratch copy, and open it
edit-sample: $(BIN)/editor sample
	./$(BIN)/editor $(BIN)/sample-adr.md

$(BIN):
	@mkdir -p $(BIN)

## version: what the built binary reports
version: $(BIN)/editor
	./$(BIN)/editor --version

test:
	$(GO) test ./...

race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

tidy:
	$(GO) mod tidy

clean:
	rm -rf $(BIN)

FORCE:
