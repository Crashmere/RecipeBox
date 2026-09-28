.PHONY: build  linux
build:
	npm --prefix web run build
	go build -trimpath -o bin/recipebox ./cmd/recipebox

linux:
	npm --prefix web run build
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o bin/recipebox-linux-amd64 ./cmd/recipebox

.PHONY: release deploy portal rollback releases
release:
	bash deploy/release.sh build
deploy:
	bash deploy/release.sh deploy
portal:
	bash deploy/release.sh portal
rollback:
	bash deploy/release.sh rollback $(COMMIT)
releases:
	bash deploy/release.sh list
