APP_NAME := fortunebot

.PHONY: build run clean

build:
	GO111MODULE=on go build -o $(APP_NAME) ./cmd/fortunebot

run: build
	./$(APP_NAME)

install: build
	mkdir -p ~/.local/bin
	cp $(APP_NAME) ~/.local/bin/$(APP_NAME)
	@echo "Installed to ~/.local/bin/$(APP_NAME). Ensure ~/.local/bin is on your PATH."

clean:
	rm -f $(APP_NAME)
