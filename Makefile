GOFILES:=$(shell find . -name '*.go' | grep -v -E '(./vendor)')
DOCKER_HUB_AGENT_NAME?=henderiw/fsc-demo-agent
DOCKER_HUB_CTRLR_NAME?=henderiw/fsc-demo-controller
VERSION?=v0.1.0

all: \
	bin/linux/fsc-agent \
	bin/linux/fsc-controller \
	#bin/darwin/fsc-agent \
	#bin/darwin/fsc-controller

#images: GVERSION=$(shell $(CURDIR)/git-version.sh)
images: bin/linux/fsc-agent bin/linux/fsc-controller
	docker build -f Dockerfile-agent -t fsc-demo-agent:$(VERSION) .
	docker build -f Dockerfile-controller -t fsc-demo-controller:$(VERSION) .

tag: 
	docker tag fsc-demo-controller:$(VERSION) $(DOCKER_HUB_CTRLR_NAME):$(VERSION)
	docker tag fsc-demo-agent:$(VERSION) $(DOCKER_HUB_AGENT_NAME):$(VERSION)

push:
	docker push $(DOCKER_HUB_AGENT_NAME):$(VERSION)
	docker push $(DOCKER_HUB_CTRLR_NAME):$(VERSION)

check:
	@find . -name vendor -prune -o -name '*.go' -exec gofmt -s -d {} +
	@go vet $(zsh go list ./... | grep -v '/vendor/')
	@go test -v $(zsh go list ./... | grep -v '/vendor/')

.PHONY: vendor
vendor:
	glide update --strip-vendor
	glide-vc

clean:
	rm -rf bin

bin/%: LDFLAGS=-X github.com/fsc-demo-wim/fsc-demo/common.Version=$(zsh $(CURDIR)/git-version.sh)
bin/%: $(GOFILES)
	mkdir -p $(dir $@)
	GOOS=$(word 1, $(subst /, ,$*)) GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $@ github.com/fsc-demo-wim/fsc-demo/$(notdir $@)