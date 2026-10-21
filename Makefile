KOBO ?= /run/media/$(USER)/KOBOeReader
CROSS ?= hellobro-cross
APPS = hellobro brotools
ADDS = $(KOBO)/.adds/hellobro

all: $(APPS)

# What every app is made of: the kobo package, and FBInk.
KOBO_SRC = $(wildcard kobo/*.go) go.mod go.sum
hellobro: $(wildcard cmd/hellobro/*.go)
brotools: $(wildcard cmd/brotools/*.go)

# cgo + FBInk need an ARM C toolchain, so the build runs in a container (tools/Containerfile).
$(APPS): $(KOBO_SRC) third_party/fbink/libfbink.a .cross-image
	podman run --rm --security-opt label=disable -v $(CURDIR):/src -w /src \
		-v $(shell go env GOMODCACHE):/go/pkg/mod -v hellobro-gocache:/root/.cache/go-build \
		-e GOOS=linux -e GOARCH=arm -e GOARM=7 -e CGO_ENABLED=1 -e CC=arm-linux-gnueabihf-gcc -e GOFLAGS=-mod=mod \
		$(CROSS) go build -trimpath -tags netgo,osusergo \
		-ldflags='-s -w -linkmode external -extldflags "-static"' -o $@ ./cmd/$@

# Without cgo: no linked-in FBInk, every update runs bin/fbink (slow, but no container needed).
%-nocgo: $(KOBO_SRC) $(wildcard cmd/*/*.go)
	GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o $@ ./cmd/$*

.cross-image: tools/Containerfile
	podman build -t $(CROSS) -f tools/Containerfile tools
	touch $@

# FBInk with image support (KOReader's can't show images); see build-fbink.sh.
bin/fbink third_party/fbink/libfbink.a: | .cross-image
	./build-fbink.sh

# this PC's address on the Kobo's Wi-Fi, for ssh.conf
PC_IP ?= $(shell ip -4 -o addr show wlp4s0 2>/dev/null | awk '{sub("/.*","",$$4); print $$4}')

# The apps live in .adds/hellobro, sharing the launcher, fbink and the
# touch calibration, each with its NickelMenu entry. install puts hellobro
# there; brotools, the diagnostics, only goes on with install-tools, and
# uninstall-tools takes it off again.
install: hellobro bin/fbink
	test -d $(KOBO)/.adds || { echo "Kobo not mounted at $(KOBO)"; exit 1; }
	mkdir -p $(ADDS)
	cp hellobro launch.sh bin/fbink $(ADDS)/
	rm -f $(ADDS)/hellobro.sh # the launcher before launch.sh
	cp nm/hellobro $(KOBO)/.adds/nm/hellobro
	@# a first ssh.conf pointing at hellobro-server on this PC; kept if it's there
	test -f $(ADDS)/ssh.conf || \
		printf '# name  user@host[:port]\nhellobro-server  bro@%s:2222\n' "$(PC_IP)" > $(ADDS)/ssh.conf
	sync

install-tools: brotools install
	cp brotools $(ADDS)/
	cp nm/brotools $(KOBO)/.adds/nm/brotools
	sync

uninstall-tools:
	test -d $(KOBO)/.adds || { echo "Kobo not mounted at $(KOBO)"; exit 1; }
	rm -f $(ADDS)/brotools $(ADDS)/brotools.log $(ADDS)/brotools-old.log $(KOBO)/.adds/nm/brotools
	sync

log:
	cat $(ADDS)/hellobro.log

log-tools:
	cat $(ADDS)/brotools.log

# The SSH test server (server/), for the PC: cd server && ./hellobro-server
server:
	cd server && go build -o hellobro-server .

# Host-side tests: the apps' (no cgo, so no ARM toolchain) and the server's.
test:
	CGO_ENABLED=0 go vet ./...
	CGO_ENABLED=0 go test ./...
	cd server && go test ./...

clean:
	rm -f $(APPS) $(APPS:=-nocgo)

.PHONY: all install install-tools uninstall-tools log log-tools clean server test
