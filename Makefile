.PHONY: help
help:
	@echo "make <image|binary|clean>"

.PHONY: image
image: binary
	./build.sh image

.PHONY: binary
binary: _output/token-resource
_output/token-resource: cmd/token-resource/main.go
	./build.sh binary

.PHONY: clean
clean:
	rm -rf _output
