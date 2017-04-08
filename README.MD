## envoyds

ykevinc/envoyds is a Golang implementation of [`Lyft/Discovery`](https://github.com/lyft/discovery), a discovery service provides the REST interface for querying and registering service in microservice infrastructure. It is to be pluginable as the service discovery solution for [`Envoy`](https://github.com/lyft/envoy).

## Discovery service settings

The settings are controlled by [`envoyds.conf`](https://github.com/ykevinc/envoyds/blob/master/envoyds/envoyds.conf) file. To loaded a different file than one from the current workspace, provide the file path as first command line argument. (i.e. envoyds `[<optional-config-path>]`)

"envoyds.environment" = "dev"

"envoyds.port" = 8000

"envoyds.redis.host" = "localhost"

"envoyds.redis.port" = 6379

## How to build

1. [`glide`](https://glide.sh) is used to manage Go dependencies. Please make sure `glide` is in your PATH before you attempt to build.
1. cd envoyds
2. go build envoyds.go

## How to run

1. Redis is running
2. envoyds `[<optional-config-path>]`

## API example usages

curl -X POST "http://localhost:8000/v1/registration/test?ip=123.124.125.126&service_repo_name=v&port=100&revision=44&tags=\{\"az\":\"c\"\}"

curl -X GET "http://localhost:8000/v1/registration/test"

curl -X DELETE "http://localhost:8000/v1/registration/test/123.124.125.126"

curl -X DELETE "http://localhost:8000/v1/registration/test/123.124.125.126/100"

curl -X POST "http://localhost:8000/v1/loadbalancing/test/123.124.125.126/100?load_balancing_weight=3"


## Improvements to original lyft/discovery

1. It allows unique ip/port registration/removal [`issues/22`](https://github.com/lyft/discovery/issues/22)
2. Second index querying is done by querying indexes instead of whole list scan [`query_secondary_index`](https://github.com/lyft/discovery/blob/f1e2804d361c54a97078fd0fb239550d70f1c94b/app/services/query.py#L138)

## Integration tests

1. cd envoyds
2. go test envoyds_test.go


## Reference

https://github.com/lyft/discovery

https://github.com/lyft/envoy

https://lyft.github.io/envoy/docs/intro/arch_overview/service_discovery.html