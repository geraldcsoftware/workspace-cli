BIN_DIR := bin
BIN := $(BIN_DIR)/space

.PHONY: build launch

build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN) ./cmd/space

launch: build
	$(BIN) $(ARGS)

