GOFILES:=$(shell find . -name '*.go' | grep -v -E '(./vendor)')

all: \
	bin/linux/fsc-agent \
	bin/linux/fsc-controller \
	#bin/darwin/fsc-agent \
	#bin/darwin/fsc-controller

images: GVERSION=$(shell $(CURDIR)/git-version.sh)
images: bin/linux/fsc-agent bin/linux/fsc-controller
	docker build -f Dockerfile-agent -t fsc-demo-agent:$(GVERSION) .
	docker build -f Dockerfile-controller -t fsc-demo-controller:$(GVERSION) .

check:
	@find . -name vendor -prune -o -name '*.go' -exec gofmt -s -d {} +
	@go vet $(shell go list ./... | grep -v '/vendor/')
	@go test -v $(shell go list ./... | grep -v '/vendor/')

.PHONY: vendor
vendor:
	glide update --strip-vendor
	glide-vc

clean:
	rm -rf bin

bin/%: LDFLAGS=-X github.com/henderiw/fsc-demo/common.Version=$(shell $(CURDIR)/git-version.sh)
bin/%: $(GOFILES)
	mkdir -p $(dir $@)
	GOOS=$(word 1, $(subst /, ,$*)) GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $@ github.com/henderiw/fsc-demo/$(notdir $@)