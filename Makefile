KOBO ?= /run/media/$(USER)/KOBOeReader
CROSS ?= hellobro-cross
APPS = hellobro brotools
ADDS = $(KOBO)/.adds/hellobro

all: $(APPS)

# What every app is made of: the kobo package, and FBInk.
KOBO_SRC = $(wildcard kobo/*.go) go.mod go.sum
hellobro: $(wildcard cmd/hellobro/*.go book/*.go pdf/*.go) third_party/mupdf/libmupdf.a
brotools: $(wildcard cmd/brotools/*.go)

# The build's version: the commit, and when it was built (hellobro --version).
VERSION := $(shell git describe --always --dirty 2>/dev/null || echo dev)-$(shell date +%m%d.%H%M)

# cgo + FBInk need an ARM C toolchain, so the build runs in a container (tools/Containerfile).
$(APPS): $(KOBO_SRC) $(wildcard share/*.go radio/*.go audio/*.go) third_party/fbink/libfbink.a .cross-image
	podman run --rm --security-opt label=disable -v $(CURDIR):/src -w /src \
		-v $(shell go env GOMODCACHE):/go/pkg/mod -v hellobro-gocache:/root/.cache/go-build \
		-e GOOS=linux -e GOARCH=arm -e GOARM=7 -e CGO_ENABLED=1 -e CC=arm-linux-gnueabihf-gcc -e GOFLAGS=-mod=mod \
		$(CROSS) go build -trimpath -tags netgo,osusergo \
		-ldflags='-s -w -X main.version=$(VERSION) -linkmode external -extldflags "-static"' -o $@ ./cmd/$@

# Without cgo: no linked-in FBInk, every update runs bin/fbink (slow, but no container needed).
%-nocgo: $(KOBO_SRC) $(wildcard cmd/*/*.go)
	GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o $@ ./cmd/$*

.cross-image: tools/Containerfile
	podman build -t $(CROSS) -f tools/Containerfile tools
	touch $@

# MuPDF, for the reader's PDFs: a pinned release, built once, not in git; see build-mupdf.sh.
third_party/mupdf/libmupdf.a: build-mupdf.sh | .cross-image
	./build-mupdf.sh

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
	@# a first ssh.conf pointing at hellobro-server on this PC, which hellobro
	@# takes into hellobro.json (keeping it as ssh.conf.old): only the first time
	test -f $(ADDS)/ssh.conf || test -f $(ADDS)/ssh.conf.old || \
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

# Over Wi-Fi, with file sharing on in hellobro (Settings) and this
# computer paired: the new build goes next to the running one, which
# checks it and restarts into it (keeping the old one as hellobro.old).
# From afar, through the relay (README):
#   make push KOBO_HOST=127.0.0.1 KOBO_PORT=2023 KOBO_JUMP=me@vps
# or a Host in ~/.ssh/config that says all that: make push KOBO_HOST=kobo-afar.
# The port is that Host's if it sets one, else file sharing's 2022.
KOBO_HOST ?= kobo.local
KOBO_PORT ?= $(or $(shell ssh -G $(KOBO_HOST) 2>/dev/null | awk '$$1 == "port" && $$2 != 22 { print $$2 }'),2022)
KOBO_JUMP ?=
KOBO_DIR = /mnt/onboard/.adds/hellobro
SSH_TO = -p $(KOBO_PORT) $(if $(KOBO_JUMP),-J $(KOBO_JUMP))
SCP_TO = -P $(KOBO_PORT) $(if $(KOBO_JUMP),-J $(KOBO_JUMP))
push: hellobro
	scp $(SCP_TO) hellobro $(KOBO_HOST):$(KOBO_DIR)/hellobro.new
	scp $(SCP_TO) launch.sh $(KOBO_HOST):$(KOBO_DIR)/launch.sh.new
	ssh $(SSH_TO) $(KOBO_HOST) 'cd $(KOBO_DIR) && mv -f launch.sh.new launch.sh' # it may be running: a new file, not over it
	ssh $(SSH_TO) $(KOBO_HOST) hellobro-update

# The same, back to the build before.
push-rollback:
	ssh $(SSH_TO) $(KOBO_HOST) 'cd $(KOBO_DIR) && cp hellobro.old hellobro.new' && \
	ssh $(SSH_TO) $(KOBO_HOST) hellobro-update

# A CPU profile of hellobro on the Kobo, PROFILE_SECS long (30), kept in
# cpu.pprof here: go tool pprof -http=: cpu.pprof for the whole of it.
PROFILE_SECS ?= 30
push-profile:
	ssh $(SSH_TO) $(KOBO_HOST) hellobro-profile $(PROFILE_SECS) > cpu.pprof || \
		{ cat cpu.pprof; rm -f $(CURDIR)/cpu.pprof; exit 1; }
	go tool pprof -top -nodecount=30 cpu.pprof

# A release: a zip to unpack onto the Kobo over USB (what install puts
# there, but ssh.conf), the bare build for updates over Wi-Fi, and their
# sums. Tag first, so the version is the tag.
REL := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
DIST = $(CURDIR)/dist
dist: hellobro bin/fbink
	rm -rf $(DIST)/$(REL) && mkdir -p $(DIST)/$(REL)/.adds/hellobro $(DIST)/$(REL)/.adds/nm
	cp hellobro launch.sh bin/fbink $(DIST)/$(REL)/.adds/hellobro/
	cp nm/hellobro $(DIST)/$(REL)/.adds/nm/
	rm -f $(DIST)/hellobro-$(REL).zip
	cd $(DIST)/$(REL) && zip -qrX $(DIST)/hellobro-$(REL).zip .adds
	cp hellobro $(DIST)/hellobro
	cd $(DIST) && sha256sum hellobro-$(REL).zip hellobro > SHA256SUMS
	rm -rf $(DIST)/$(REL)
	ls -l $(DIST)

# The log, over Wi-Fi.
push-log:
	ssh $(SSH_TO) $(KOBO_HOST) cat $(KOBO_DIR)/hellobro.log

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
	rm -rf $(DIST)

.PHONY: all install install-tools uninstall-tools log log-tools clean dist server test push push-rollback push-log push-profile
