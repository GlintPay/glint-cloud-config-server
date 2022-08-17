# glint-cloud-config-server

![Build, deploy status](https://github.com/GlintPay/glint-cloud-config-server/actions/workflows/main.yaml/badge.svg)

[![codecov](https://codecov.io/gh/GlintPay/glint-cloud-config-server/branch/master/graph/badge.svg?token=PBV9Z53I17)](https://codecov.io/gh/GlintPay/glint-cloud-config-server)

This is a Golang implementation of the [Spring Cloud Config Server](https://docs.spring.io/spring-cloud-config/docs/current/reference/html/#_spring_cloud_config_server), an HTTP resource-based API for external configuration. We have used this in Docker form via https://github.com/hyness/spring-cloud-config-server for over a year with no problems.

The aims for our project are:
* Complete compatibility with the existing Server API
* We have had to reimplement client-side functionality - that Spring Boot clients get for free - in library form for Golang-only clients. By shifting this to the server, a wider range of clients can get full value.
* Quicker server / container startup
* New capabilities of our choice and design

---

## Server Usage:

### Docker image:

    docker pull glintpay/glint-cloud-config-server

[Docker Hub](https://hub.docker.com/r/glintpay/glint-cloud-config-server)

### Docker run:

    docker run -p 8888:80 \
        -v /here/conf/:/conf \
        -v /here/ssh/:/ssh \
        -e APP_CONFIG_FILE_YML_PATH=/conf/application.yml \
        glintpay/glint-cloud-config-server

### Example configuration file:

    server:
      port: 8888  # defaults to 80
    
    defaults:
      flattenHierarchicalConfig: true
      flattenedIndexedLists: true
      resolvePropertySources: false
    
    prometheus:
      path: /metrics  # ignored if not set
    
    tracing:
      enabled: true
      endpoint: opentelemetry-traces:4318
      samplerFraction: 0.5  # Sample only 50% of traces
    
    file:
      order: 0
      path: /config-dir
       
    git:
      order: 1
      uri: git@github.com:Org/repo.git
      knownHostsFile: /ssh/github_known_hosts
      privateKey: |
      -----BEGIN RSA PRIVATE KEY-----
      ...
      -----END RSA PRIVATE KEY-----
    
      basedir: /tmp/cloud-config
      timeout: 30000
      clone-on-start: true
      force-pull: false
      show-progress: false  # log clones and pulls
      refreshRate: 0        # default = 0 secs => Fetch updated configuration from the Git repo every time it is requested

### Testing:

One application, multiple ordered profiles, main Git branch:

    http "localhost:8888/myapp/production-usa,production-base?resolve=true&flatten=true"

One application, one profile, custom label (labelled Git branch):

    http "localhost:8888/myapp/testing-uk/refactor?resolve=true&flatten=true"

Inject, override custom properties:

    http "localhost:8888/myapp/production-uk?resolve=true&flatten=true" baseUrl=http://uk-test bypass=true


---

## Client Usage:

tbc

---

## Concepts:

### Application:

The primary discriminator for the application or client you are looking to configure, e.g. `accounts`

Multiple names can be supplied to represent groupings of applications, as a comma-separated list, e.g. `accounts,finance-shared,security-aware`

Ordering occurs from right to left, i.e. the more specific first. Thus `security-aware` is overridden by `finance-shared`, and in turn by `accounts`.

### Profile:

Use profiles to group cross-application configuration by environment, data centre etc.

Multiple names can be supplied to represent groupings, as a comma-separated list, e.g. `production-uk,production,uk-datacentre`

Ordering occurs from right to left, i.e. the more specific first. Thus `uk-datacentre` is overridden by `production`, and in turn by `production-uk`.

### Label:

Use this to switch competing configuration sets by version, by time (e.g. commit hash), or to pick up a refactoring branch.

This is a simple text value. If left blank, will default to the main Git branch, or equivalent "latest" for the backend in question.

### Backend:

A repository for configuration files. Currently supported:

* [Git](https://github.com/GlintPay/glint-cloud-config-server/tree/master/backend/git)
* [File](https://github.com/GlintPay/glint-cloud-config-server/tree/master/backend/file)
* Kubernetes `ConfigMap` (planned)

Configurations are aggregated across all non-`disabled` repositories, ordered (if necessary) by the backend's configured `order` value.

### Load:

Acquisition of the configurations for the applications / profiles / labels specified, across the available and enabled backends.

By default, a list of Spring Cloud Config Server-compatible `PropertySource`s are available:

```bash
▶ http "localhost:8888/accounts/prod-us,prod/"
HTTP/1.1 200 OK
Content-Type: application/json
Date: Tue, 16 Aug 2022 16:22:38 GMT
Transfer-Encoding: chunked

{
    "label": "",
    "name": "accounts",
    "profiles": [
        "prod-us",
        "prod"
    ],
    "propertySources": [
        {
            "name": "git@github.com:Org/config.git/accounts-prod-us.yml",
            "source": {
                "foo": "bar",
            }
        },
        {
            "name": "git@github.com:Org/config.git/application-prod-us.yml",
            "source": {
                "mysql": {
                  "host": "prod-us-mysql.xxx"
                },
                "postgres": {
                  "host": "prod-us-pg.xxx"
                }
            }
        },
        {
            "name": "git@github.com:Org/config.git/application.yml",
            "source": {
                "mysql": {
                  "host": "test-mysql.xxx"
                },
                "postgres": {
                  "host": "test-pg.xxx"
                }
                [...]
```

The ordering of the `PropertySource`s is non-deterministic.

### Flattening:

Flattening of hierarchical data structures is enabled either at request time, via `flatten=true`, or by setting the application configuration:

```yaml
defaults:
  flattenHierarchicalConfig: true
```

Which produces:
```bash
▶ http "localhost:8888/accounts/prod-us,prod/?flatten=true"
HTTP/1.1 200 OK
Content-Type: application/json
Date: Tue, 16 Aug 2022 16:22:38 GMT
Transfer-Encoding: chunked

{
    "label": "",
    "name": "accounts",
    "profiles": [
        "prod-us",
        "prod"
    ],
    "propertySources": [
        {
            "name": "git@github.com:Org/config.git/accounts-prod-us.yml",
            "source": {
                "foo": "bar",
            }
        },
        {
            "name": "git@github.com:Org/config.git/application-prod-us.yml",
            "source": {
                "mysql.host": "prod-us-mysql.xxx",
                "postgres.host": "prod-us-pg.xxx",
            }
        },
        {
            "name": "git@github.com:Org/config.git/application.yml",
            "source": {
                "mysql.host": "test-mysql.xxx",
                "postgres.host": "test-pg.xxx",
                [...]
```

Lists can be further flattened via:

```yaml
defaults:
  flattenHierarchicalConfig: true
  flattenedIndexedLists: true
```

which turns:

```json
"currencies": [
    "GBP",
    "EUR",
    "USD"
],
```

into:
```json
"currencies[0]": "GBP",
"currencies[1]": "EUR",
"currencies[2]": "USD",
```

### Resolution:

The resolution phase squashes these multiple property sources into one **single** source, according to the specified application and profile precedence ordering.

It is enabled either at request time, via `resolve=true`, or by setting the application configuration:

```yaml
defaults:
  resolvePropertySources: false
```

Thus `http://localhost:8888/service/env/?resolve=true` applies the following:

    service-env.yaml > service.yaml > application-env.yaml > application.yaml

The precise ordering applied can be seen via HTTP headers when resolution is enabled:

```
HTTP/1.1 200 OK
Content-Type: application/json
Date: Tue, 16 Aug 2022 12:00:12 GMT
Transfer-Encoding: chunked
X-Resolution-Label: 
X-Resolution-Name: service
X-Resolution-Precedencedisplaymessage: service-env.yaml > service.yaml > application-env.yaml > application.yaml
X-Resolution-Profiles: env
X-Resolution-Version: 281cd74427f6c5b87d7f8ff109a1e4e678b1b7cf

{
  [... json ...]
```

This can be combined with flattening. If flattening is disabled, a single hierarchical structure is returned.


----

## Options

tbc

----

## Observability

An OpenTelemetry exporter is available using the OTLP/HTTP protocol, if configured and enabled, e.g. via:

```yaml
tracing:
  enabled: true
  endpoint: http://opentelemetry-traces:4318
  samplerFraction: 0.5  # Sample only 50% of traces
```

This allows instrumentation of HTTP requests with some additional observability of backend events.

----

## Features

* **Property resolution** via `${propertyName}` syntax, e.g.

    ```
    serviceName: myservice
    [...]
    mysql:
      dbName: ${propertyName}_db
    ```
  
* Support for **Go templates**, including [Sprig functions](https://masterminds.github.io/sprig/), e.g.

    ```gotemplate
    mysql:
      dbName: "{{{ dashToUnderscore (first .Applications) }}}_db"
    ```

* **Property injection** - client-side:

  Send a JSON structure of configuration property name / values to the REST endpoint via the `PATCH` verb.

  Property names prefixed with `^` will be applied before any externally loaded properties (i.e. lowest precendence). Else, properties will be applied after (i.e. highest precendence)

* **Property injection** - server-side:

  tbc

----

## Missing features

* Configurable Helm chart
* Spring Cloud Bus-style event notifications