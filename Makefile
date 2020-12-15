build:
	GOOS=linux GOARCH=amd64 go build -o bin/extensions/lambda-extension-log-shipper -ldflags '-s -w' .

package: build
	cd bin/ && zip -r extension.zip extensions/

deploy-%:
	aws lambda publish-layer-version --layer-name "lambda-extension-log-shipper" --region $* --zip-file  "fileb://bin/extension.zip"

clean:
	@rm -rf bin/