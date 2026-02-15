GO_BUILD_FLAGS = -buildvcs=false -buildmode=plugin -trimpath
PLUGIN_NAME = gitlab.so

.PHONY: build dev install clean

build:
	go build $(GO_BUILD_FLAGS) -o $(PLUGIN_NAME) .

dev: build
	mkdir -p /tmp/elephant/providers
	cp $(PLUGIN_NAME) /tmp/elephant/providers/

install: build
	cp $(PLUGIN_NAME) "$$(find ~/.config/elephant -maxdepth 0 2>/dev/null || echo ~/.config/elephant)/"

clean:
	rm -f $(PLUGIN_NAME)
