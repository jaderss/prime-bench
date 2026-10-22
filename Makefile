include .env
export

RUN_ID ?= $(shell date -u +%Y%m%dT%H%M%SZ)
RUN_ID := $(RUN_ID)
LANGS ?= java go rust
GATEWAY_LANGS ?= java go rust spring spring-native
HOST_UID := $(shell id -u)
HOST_GID := $(shell id -g)
export RUN_ID HOST_UID HOST_GID

COMPOSE := docker compose
KUBECTL := kubectl --context docker-desktop
K8S_VARS := JAVA_VERSION GO_VERSION RUST_VERSION SPRING_JAVA_VERSION SERVER_BITS ROUNDS APP_CPUS APP_MEMORY_MB HEAP_PERCENT JVM_GC JAVA_OPTS \
	MOCK_CPUS MOCK_LATENCY MOCK_JITTER MOCK_TRANSACTIONS MOCK_ERROR_RATE
K8S_RENDER := sed $(foreach v,$(K8S_VARS),-e 's|$${$(v)}|$($(v))|g')

.PHONY: help build bench batch server gateway dashboard down clean k8s-up k8s-bench k8s-down

help:
	@echo "make bench       build, then run batch and server benchmarks for LANGS ($(LANGS))"
	@echo "make batch       generate BATCH_COUNT primes of BATCH_BITS in each language"
	@echo "make server      run USERS virtual users against each language's HTTP server"
	@echo "make gateway     run GATEWAY_USERS virtual users against /aggregate in GATEWAY_LANGS ($(GATEWAY_LANGS))"
	@echo "make k8s-bench   run server and gateway on Docker Desktop Kubernetes (context docker-desktop only)"
	@echo "make k8s-down    delete the prime-bench namespace"
	@echo "make dashboard   serve the dashboard on http://localhost:8080 (DASHBOARD_PORT to change)"
	@echo "make down        stop everything"
	@echo "make clean       delete CSV results"
	@echo "Override anything from .env, for example: make bench JAVA_VERSION=25 LANGS=java"

build:
	$(COMPOSE) build $(sort $(LANGS) $(GATEWAY_LANGS))
	$(COMPOSE) --profile tools build loadgen mock

bench: build batch server gateway

batch: | results
	@for lang in $(LANGS); do \
		$(COMPOSE) run --rm -e MODE=batch -e BITS=$(BATCH_BITS) $$lang || exit 1; \
	done

server: | results
	@for lang in $(LANGS); do \
		$(COMPOSE) up -d $$lang && \
		$(COMPOSE) run --rm -e TARGET=http://$$lang:8080 loadgen; \
		status=$$?; $(COMPOSE) stop $$lang; [ $$status -eq 0 ] || exit $$status; \
	done

gateway: | results
	@$(COMPOSE) up -d mock
	@for lang in $(GATEWAY_LANGS); do \
		$(COMPOSE) up -d $$lang && \
		$(COMPOSE) run --rm -e TARGET=http://$$lang:8080 -e SCENARIO=gateway -e MOCK_URL=http://mock:8080 \
			-e USERS=$(GATEWAY_USERS) -e THINK_TIME=$(GATEWAY_THINK_TIME) loadgen; \
		status=$$?; $(COMPOSE) stop $$lang; [ $$status -eq 0 ] || { $(COMPOSE) stop mock; exit $$status; }; \
	done
	@$(COMPOSE) stop mock

K8S_MANIFESTS := k8s/mock.yaml k8s/java.yaml k8s/go.yaml k8s/rust.yaml k8s/spring.yaml k8s/spring-native.yaml
K8S_IMAGES := prime-bench/java:$(JAVA_VERSION) prime-bench/go:$(GO_VERSION) prime-bench/rust:$(RUST_VERSION) \
	prime-bench/spring:java$(SPRING_JAVA_VERSION) prime-bench/spring-native:java$(SPRING_JAVA_VERSION) \
	prime-bench/mock:go$(GO_VERSION) prime-bench/loadgen:go$(GO_VERSION)

k8s-up: build
	$(KUBECTL) apply -f k8s/namespace.yaml
	scripts/k8s-load.sh $(K8S_IMAGES)
	@for f in $(K8S_MANIFESTS); do cat $$f; echo ---; done | $(K8S_RENDER) | $(KUBECTL) -n prime-bench apply -f -
	$(KUBECTL) -n prime-bench rollout status deployment/mock --timeout=120s

k8s-bench: k8s-up | results
	@for lang in $(LANGS); do scripts/k8s-run.sh $$lang prime || exit 1; done
	@for lang in $(GATEWAY_LANGS); do scripts/k8s-run.sh $$lang gateway || exit 1; done

k8s-down:
	$(KUBECTL) delete namespace prime-bench --ignore-not-found

dashboard:
	$(COMPOSE) up -d --build dashboard
	@echo "Dashboard on http://localhost:$${DASHBOARD_PORT:-8080}"

down:
	$(COMPOSE) --profile tools down --remove-orphans

clean:
	rm -f results/*.csv

results:
	mkdir -p results
