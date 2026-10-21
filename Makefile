include .env
export

RUN_ID ?= $(shell date -u +%Y%m%dT%H%M%SZ)
RUN_ID := $(RUN_ID)
LANGS ?= java go rust
HOST_UID := $(shell id -u)
HOST_GID := $(shell id -g)
export RUN_ID HOST_UID HOST_GID

COMPOSE := docker compose

.PHONY: help build bench batch server gateway dashboard down clean

help:
	@echo "make bench       build, then run batch and server benchmarks for LANGS ($(LANGS))"
	@echo "make batch       generate BATCH_COUNT primes of BATCH_BITS in each language"
	@echo "make server      run USERS virtual users against each language's HTTP server"
	@echo "make gateway     run GATEWAY_USERS virtual users against /aggregate, which calls the mock downstream"
	@echo "make dashboard   serve the dashboard on http://localhost:8080 (DASHBOARD_PORT to change)"
	@echo "make down        stop everything"
	@echo "make clean       delete CSV results"
	@echo "Override anything from .env, for example: make bench JAVA_VERSION=25 LANGS=java"

build:
	$(COMPOSE) build $(LANGS)
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
	@for lang in $(LANGS); do \
		$(COMPOSE) up -d $$lang && \
		$(COMPOSE) run --rm -e TARGET=http://$$lang:8080 -e SCENARIO=gateway -e MOCK_URL=http://mock:8080 \
			-e USERS=$(GATEWAY_USERS) -e THINK_TIME=$(GATEWAY_THINK_TIME) loadgen; \
		status=$$?; $(COMPOSE) stop $$lang; [ $$status -eq 0 ] || { $(COMPOSE) stop mock; exit $$status; }; \
	done
	@$(COMPOSE) stop mock

dashboard:
	$(COMPOSE) up -d --build dashboard
	@echo "Dashboard on http://localhost:$${DASHBOARD_PORT:-8080}"

down:
	$(COMPOSE) --profile tools down --remove-orphans

clean:
	rm -f results/*.csv

results:
	mkdir -p results
