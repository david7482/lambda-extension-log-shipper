build:
	GOOS=linux GOARCH=amd64 go build -o bin/extensions/lambda-extension-log-shipper -ldflags '-s -w' .
	GOOS=linux GOARCH=amd64 go build -o bin/examples/bootstrap -ldflags '-s -w' ./examples/hello-lambda-extension.go

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
	cd bin/ && zip -r lambda-extension-log-shipper.zip extensions/
	cd bin/examples && zip ../hello-lambda-extension.zip bootstrap

deploy-layer-%: package
	aws lambda publish-layer-version --layer-name "lambda-extension-log-shipper" --region $* --zip-file  "fileb://bin/lambda-extension-log-shipper.zip"

deploy-examples-%: package
	aws lambda update-function-code --function-name hello-lambda-extension --region $* --zip-file "fileb://bin/hello-lambda-extension.zip"
	aws lambda list-layer-versions --layer-name lambda-extension-log-shipper --max-items 1 | \
	jq -r '.LayerVersions[0].LayerVersionArn' | \
	xargs -I{} aws lambda update-function-configuration --function-name hello-lambda-extension --region $* --layers {}

clean:
	@rm -rf bin/