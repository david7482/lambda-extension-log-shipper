# lambda-extension-log-shipper

This project is a Lambda layer aims to ship your Lambda logs **directly** to any destination. This log shipper works like a 
sidecar of the Lambda function (just like fluntd). It would listen to your Lambda function logs via Lambda Logs API, so you 
could ship the logs to a custom destination without CloudWatch Logs (and save cost !)


Current supported forwarders:

* [newrelic](./forwardservice/forwarders/newrelic)
* [stdout](./forwardservice/forwarders/stdout)

Other forwarder could be added easily; check [Contribute](#contribute).

## Usage

Build the release package (`bin/extension.zip`):
```bash
$ make package
```

Publish to Lambda layer using the `extension.zip` and get the produced layer arn.
```bash
$ aws lambda publish-layer-version --layer-name "lambda-extension-log-shipper" --region <region> --zip-file  "fileb://bin/extension.zip"
```

Add the Lambda layer to the Lambda function
```bash
$ aws lambda update-function-configuration --region <region> --function-name <lambda-function-name> --layers <layer-arn>
```

### Disabling logging to CloudWatch Logs 

To disable logging to CloudWatch Logs for the Lambda function, you can add this policy to the Lambda execution role to remove 
access to CloudWatch Logs. Logs are no longer delivered to CloudWatch Logs for functions using this role, but are still 
streamed to subscribed extensions. You are no longer billed for CloudWatch logging for these Lambda functions.

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Deny",
            "Action": [
                "logs:CreateLogGroup",
                "logs:CreateLogStream",
                "logs:PutLogEvents"
            ],
            "Resource": [
                "arn:aws:logs:*:*:*"
            ]
        }
    ]
}
```

### Supported runtimes

This log shipper currently supports the following [Lambda runtimes](https://docs.aws.amazon.com/lambda/latest/dg/using-extensions.html):

* .NET Core 3.1 (C#/PowerShell) (`dotnetcore3.1`)
* Custom runtime (`provided`)
* Custom runtime on Amazon Linux 2 (`provided.al2`)
* Java 11 (Corretto) (`java11`)
* Java 8 (Corretto) (`java8.al2`)
* Node.js 12.x (`nodejs12.x`)
* Node.js 10.x (`nodejs10.x`)
* Python 3.8 (`python3.8`)
* Python 3.7 (`python3.7`)
* Ruby 2.7 (`ruby2.7`)
* Ruby 2.5 (`ruby2.5`)

## Configuration

All configurations are via environment variables. You need to add these environment variables to your Lambda function.
The followings are general configurations. Check the README of each forwarder for its specific configurations.

|Env variable |  Default Value |Description |
|---|---|---|
|LS_LOG_LEVEL|info|The level of the internal logger|
|LS_LOG_TIMEFORMAT|2006-01-02T15:04:05.000Z07:00|The time format of the internal logger|


## How it works

This project uses the [AWS Lambda Logs API](https://docs.aws.amazon.com/lambda/latest/dg/runtimes-logs-api.html) to 
register itself as a sidecar of the running Lambda function. It starts an internal http server to listen to Lambda logs
(include `platform` and `function` logs), which being aggregated in-memory and transferred to all the enabled log forwarders.

## Contribute

To add a new forwarder, just need to follow the 2 steps:

1. Implement the new forwarder in a standalone package under `forwardservice\forwarders`.  The new forwarder needs to follow
the `forwardservice.Forwarder` interface.
2. Register the new forwarder into `main.forwarders` array so that main() would initialize it properly.

## License

This project is licensed under the Apache License 2.0. See the LICENSE file.
