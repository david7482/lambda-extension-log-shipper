build:
	GOOS=linux GOARCH=amd64 go build -o bin/extensions/lambda-extension-log-shipper -ldflags '-s -w' .

test:
	go vet ./...
	go test -race -cover -coverprofile cover.out ./...

lint:
	@if [ ! -f ./bin/golangci-lint ]; then \
		curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s "v1.34.1"; \
	fi;
	@echo "golangci-lint checking..."
	@./bin/golangci-lint -v run ./...

mock:
	@which mockgen > /dev/null || (echo "No mockgen installed."; exit 1)
	@echo "generating mocks..."
	@go generate ./...

package: build
	cd bin/ && zip -r extension.zip extensions/

deploy-%: package
	aws lambda publish-layer-version --layer-name "lambda-extension-log-shipper" --region $* --zip-file  "fileb://bin/extension.zip"

clean:
	@rm -rf bin/