# NewRelic forwarder

This forwarder use [NewRelic Log API](https://docs.newrelic.com/docs/logs/log-management/log-api/introduction-log-api) 
to ship Lambda logs to NewRelic. To use this forwarder, you must first obtain a NewRelic API Key.

## Configuration

|Env variable |  Default Value |Description |
|---|---|---|
|LS_NEWRELIC_ENABLE|false|Enable the newrelic forwarder|
|LS_NEWRELIC_LICENSE_KEY|""|The NewRelic licence key to ingest the logs|