APP_NAME  := fortunebot
INSTALL_DIR := $(HOME)/.local/bin
LDFLAGS   := -s -w

.PHONY: build run install clean uninstall

build:
	go build -trimpath -ldflags="$(LDFLAGS)" -o $(APP_NAME) ./cmd/fortunebot

run: build
	./$(APP_NAME)

install: build
	mkdir -p $(INSTALL_DIR)
	install -m 755 $(APP_NAME) $(INSTALL_DIR)/$(APP_NAME)
	@echo "Installed to $(INSTALL_DIR)/$(APP_NAME)"
	@echo "Ensure $(INSTALL_DIR) is on your PATH (add to ~/.zshrc: export PATH=\"\$$HOME/.local/bin:\$$PATH\")"

clean:
	rm -f $(APP_NAME)

uninstall:
	rm -f $(INSTALL_DIR)/$(APP_NAME)
	@echo "Removed $(INSTALL_DIR)/$(APP_NAME). User config/log/cache left untouched."
