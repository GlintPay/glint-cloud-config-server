# glint-cloud-config-server

![Build, deploy status](https://github.com/GlintPay/glint-cloud-config-server/actions/workflows/main.yaml/badge.svg)

This is a Golang implementation of the [Spring Cloud Config Server](https://docs.spring.io/spring-cloud-config/docs/current/reference/html/#_spring_cloud_config_server), an HTTP resource-based API for external configuration. We have used this in Docker form via https://github.com/hyness/spring-cloud-config-server for over a year with no problems.

The aims for our project are:
* Complete **compatibility** with the existing Server API
* We have had to reimplement **client-side** functionality - that Spring Boot clients get for free - in library form for Golang-only clients. We would like to shift this to the server, so that a much wider range of clients can get full value.
* **Quicker** server / container startup
* New capabilities of our choice and design

---

### Docker image:

    docker pull glintpay/glint-cloud-config-server

[Docker Hub](https://hub.docker.com/repository/docker/glintpay/glint-cloud-config-server)

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

