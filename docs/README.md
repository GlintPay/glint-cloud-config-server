# glint-cloud-config-server

![Build, deploy status](https://github.com/GlintPay/glint-cloud-config-server/actions/workflows/main.yaml/badge.svg)

This is a Golang implementation of the [Spring Cloud Config Server](https://docs.spring.io/spring-cloud-config/docs/current/reference/html/#_spring_cloud_config_server), an HTTP resource-based API for external configuration. We have used this in Docker form via https://github.com/hyness/spring-cloud-config-server for over a year with no problems.

The aims for our project are:
* Complete **compatibility** with the existing Server API
* We have had to reimplement **client-side** functionality - that Spring Boot clients get for free - in library form for Golang-only clients. We would like to shift this to the server, so that a much wider range of clients can get full value.
* **Quicker** server / container startup
* New capabilities of our choice and design

---

Docker run:

    docker pull glintpay/glint-cloud-config-server

[Docker Hub](https://hub.docker.com/repository/docker/glintpay/glint-cloud-config-server)

Local build & run:

    make install
    APP_CONFIG_FILE_YML_PATH=../glint-cloud-config-server-deploy/docker-build/application.yml gak
