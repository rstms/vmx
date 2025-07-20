# go makefile

program != basename $$(pwd)
windows != env | grep ^WINDIR=


latest_release != gh release list --json tagName --jq '.[0].tagName' | tr -d v
version != cat VERSION

gitclean = $(if $(shell git status --porcelain),$(error git status is dirty),$(info git status is clean))

install_dir = /usr/local/bin
postinstall =

$(program): build

build: fmt
	fix go build

fmt: go.sum
	fix go fmt . ./...

go.mod:
	go mod init

go.sum: go.mod
	go mod tidy

install: build
	$(if $(windows),go install,doas install -m 0755 $(program) $(install_dir)/$(program) $(postinstall))

test: fmt
	go test -v -failfast . ./...

debug: fmt
	go test -v -failfast -count=1 -run $(test) . ./...

release:
	$(gitclean)
	@$(if $(update),gh release delete -y v$(version),)
	gh release create v$(version) --notes "v$(version)"

clean:
	rm -f $(program) *.core *.vmx.*
	go clean
	echo >/var/log/vmx

sterile: clean
	which $(program) && go clean -i || true
	go clean -r || true
	go clean -cache
	go clean -modcache
	rm -f go.mod go.sum
