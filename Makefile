KOBO ?= /run/media/$(USER)/KOBOeReader
CROSS ?= hellobro-cross

# cgo + FBInk need an ARM C toolchain, so the build runs in a container (tools/Containerfile).
hellobro: $(wildcard *.go) go.mod go.sum third_party/fbink/libfbink.a .cross-image
	podman run --rm --security-opt label=disable -v $(CURDIR):/src -w /src \
		-v $(shell go env GOMODCACHE):/go/pkg/mod -v hellobro-gocache:/root/.cache/go-build \
		-e GOOS=linux -e GOARCH=arm -e GOARM=7 -e CGO_ENABLED=1 -e CC=arm-linux-gnueabihf-gcc -e GOFLAGS=-mod=mod \
		$(CROSS) go build -trimpath -tags netgo,osusergo \
		-ldflags='-s -w -linkmode external -extldflags "-static"' -o $@ .

# Without cgo: no linked-in FBInk, every update runs bin/fbink (slow, but no container needed).
hellobro-nocgo: $(wildcard *.go) go.mod go.sum
	GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o $@ .

.cross-image: tools/Containerfile
	podman build -t $(CROSS) -f tools/Containerfile tools
	touch $@

# FBInk with image support (KOReader's can't show images); see build-fbink.sh.
bin/fbink third_party/fbink/libfbink.a: | .cross-image
	./build-fbink.sh

# this PC's address on the Kobo's Wi-Fi, for ssh.conf
PC_IP ?= $(shell ip -4 -o addr show wlp4s0 2>/dev/null | awk '{sub("/.*","",$$4); print $$4}')

install: hellobro bin/fbink
	test -d $(KOBO)/.adds || { echo "Kobo not mounted at $(KOBO)"; exit 1; }
	mkdir -p $(KOBO)/.adds/hellobro
	cp hellobro hellobro.sh bin/fbink $(KOBO)/.adds/hellobro/
	cp nm/hellobro $(KOBO)/.adds/nm/hellobro
	@# a first ssh.conf pointing at hellobro-server on this PC; kept if it's there
	test -f $(KOBO)/.adds/hellobro/ssh.conf || \
		printf '# name  user@host[:port]\nhellobro-server  bro@%s:2222\n' "$(PC_IP)" > $(KOBO)/.adds/hellobro/ssh.conf
	sync

log:
	cat $(KOBO)/.adds/hellobro/hellobro.log

# The SSH test server (server/), for the PC: cd server && ./hellobro-server
server:
	cd server && go build -o hellobro-server .

# Host-side tests: the app's (no cgo, so no ARM toolchain) and the server's.
test:
	CGO_ENABLED=0 go test ./...
	cd server && go test ./...

clean:
	rm -f hellobro hellobro-nocgo

.PHONY: install log clean server test
