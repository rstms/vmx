# go makefile

program != basename $$(pwd)

go_version = go1.24.5

latest_release != gh release list --json tagName --jq '.[0].tagName' | tr -d v
version != cat VERSION
rstms_modules != awk <go.mod '/^module/{next} /rstms/{print $$1}'
latest_module_release = $(shell gh --repo $(1) release list --json tagName --jq '.[0].tagName')
gitclean = $(if $(shell git status --porcelain),$(error git status is dirty),$(info git status is clean))

install_dir = /usr/local/bin
postinstall =

$(program): build

build: fmt
	fix go build

fmt: go.sum
	fix go fmt . ./...

go.mod:
	$(go_version) mod init

go.sum: go.mod
	go mod tidy

install: build
	go install

test: fmt
	go test -v -failfast . ./...

debug: fmt
	go test -v -failfast -count=1 -run $(test) . ./...

release:
	$(gitclean)
	@$(if $(update),gh release delete -y v$(version),)
	gh release create v$(version) --notes "v$(version)"

update:
	@echo checking dependencies for updated versions 
	@$(foreach module,$(rstms_modules),go get $(module)@$(call latest_module_release,$(module));)
	curl -L -o cmd/common.go https://raw.githubusercontent.com/rstms/go-common/master/proxy_common_go
	sed <cmd/common.go >ws/common.go 's/^package cmd/package ws/'

logclean: 
	echo >/var/log/vmx

clean: logclean
	rm -f $(program) *.core *.vmx.*
	go clean

sterile: clean
	which $(program) && go clean -i || true
	go clean -cache || true
	go clean -modcache || true
	rm -f go.mod go.sum
